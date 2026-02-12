package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/LJTian/TrendingHub/internal/processor"
	"github.com/redis/go-redis/v9"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Channel 描述一个数据源，例如 weibo / zhihu / github
type Channel struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	Code    string `gorm:"size:64;uniqueIndex" json:"code"` // 例如: github, zhihu
	Name    string `gorm:"size:128" json:"name"`
	BaseURL string `gorm:"size:256" json:"baseUrl"`
	Status  string `gorm:"size:32;index" json:"status"` // active / disabled

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type News struct {
	ID     string `gorm:"primaryKey;size:40" json:"id"`
	Title  string `gorm:"size:512" json:"title"`
	URL    string `gorm:"size:1024;uniqueIndex" json:"url"`
	Source string `gorm:"size:64;index" json:"source"`
	// 只保留一段介绍文案；长度控制在约 200 个字符（在 processor 中按 rune 截断）
	Description   string            `gorm:"size:600" json:"description"` // 详细介绍，悬停显示
	PublishedAt   time.Time         `gorm:"index" json:"publishedAt"`
	PublishedDate string            `gorm:"size:10;index" json:"publishedDate"` // 日期 YYYY-MM-DD，用于按日期展示
	HotScore      float64           `gorm:"index" json:"hotScore"`
	ExtraData     datatypes.JSONMap `gorm:"type:jsonb" json:"extraData"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Store struct {
	DB    *gorm.DB
	Redis *redis.Client
}

func NewStore(dsn, redisAddr string) (*Store, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&Channel{}, &News{}); err != nil {
		return nil, err
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("warn: redis ping failed: %v", err)
	}

	return &Store{DB: db, Redis: rdb}, nil
}

// EnsureChannel 确保某个渠道存在
func (s *Store) EnsureChannel(code, name, baseURL string) (*Channel, error) {
	ch := &Channel{}
	if err := s.DB.Where("code = ?", code).First(ch).Error; err == nil {
		return ch, nil
	}

	ch = &Channel{
		Code:    code,
		Name:    name,
		BaseURL: baseURL,
		Status:  "active",
	}
	if err := s.DB.Create(ch).Error; err != nil {
		return nil, err
	}
	return ch, nil
}

// 东八区，用于日期展示与筛选
var locEast8 *time.Location

func init() {
	locEast8, _ = time.LoadLocation("Asia/Shanghai")
	if locEast8 == nil {
		locEast8 = time.FixedZone("CST", 8*3600)
	}
}

// toValidUTF8 将字符串规范为合法 UTF-8，避免 PostgreSQL invalid byte sequence 错误（如百度等源可能含 GBK/混编）
func toValidUTF8(s string) string {
	return strings.ToValidUTF8(s, "\uFFFD")
}

// truncateRunesDB 按 rune 数截断字符串，确保不会超过数据库字段长度（例如 varchar(600)）。
// 这是对上游 Processor 的双保险，防止外部服务返回异常长文本导致入库失败。
func truncateRunesDB(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= limit {
		return s
	}
	return string(rs[:limit])
}

// SaveBatch 保存一批新闻，已存在的根据 URL 忽略
func (s *Store) SaveBatch(items []processor.ProcessedNews) error {
	for _, it := range items {
		pubDate := it.PublishedAt.In(locEast8).Format("2006-01-02")
		title := toValidUTF8(it.Title)
		description := toValidUTF8(it.Description)
		// 再次做长度保护，确保不会超过 varchar(600) 的限制
		description = truncateRunesDB(description, 600)
		n := &News{
			ID:            it.ID,
			Title:         title,
			URL:           it.URL,
			Source:        it.Source,
			Description:   description,
			PublishedAt:   it.PublishedAt,
			PublishedDate: pubDate,
			HotScore:      it.HotScore,
			ExtraData:     datatypes.JSONMap(it.RawData),
		}

		// 以 URL 作为幂等键，避免重复插入；已存在时更新摘要/详细介绍等便于悬停显示
		if err := s.DB.Where("url = ?", it.URL).FirstOrCreate(n).Error; err != nil {
			return err
		}
		_ = s.DB.Model(n).Updates(map[string]any{
			"title":          title,
			"description":    description,
			"hot_score":      it.HotScore,
			"published_at":   it.PublishedAt,
			"published_date": pubDate,
		}).Error
	}

	// 这里不做按 key 通配删除，完全依赖短 TTL 的缓存自然过期，
	// 避免使用无效的通配符删除以及增加额外的 Redis 扫描复杂度。
	return nil
}

// ListNews 按渠道、排序与可选日期返回新闻列表，并使用 Redis 做简单缓存
// channel: 渠道 code，可为空
// sort: latest(默认) / hot
// date: 可选，格式 2006-01-02，指定则只返回该日期的数据
func (s *Store) ListNews(channel, sort string, limit int, date string) ([]News, error) {
	if limit <= 0 || limit > 1000 {
		limit = 20
	}
	if sort == "" {
		sort = "latest"
	}

	ctx := context.Background()
	cacheKey := fmt.Sprintf("news:list:%s:%s:%d:%s", channel, sort, limit, date)

	// L2: Redis 缓存
	if s.Redis != nil {
		if bs, err := s.Redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var cached []News
			if err := json.Unmarshal(bs, &cached); err == nil {
				return cached, nil
			}
		}
	}

	// DB 兜底
	var list []News
	db := s.DB.Model(&News{})

	// 按日期筛选：只展示指定日期的数据（东八区日期；兼容 published_date 为空的旧数据）
	if date != "" {
		db = db.Where("(published_date = ? OR (TRIM(COALESCE(published_date, '')) = '' AND to_char(published_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD') = ?))", date, date)
	}

	// 金融渠道：黄金（当天） + A 股指数（最新），合并后总条数不超过 limit；“当天”按东八区
	if channel == "gold" {
		now := time.Now().In(locEast8)
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, locEast8)
		if date != "" {
			if t, err := time.ParseInLocation("2006-01-02", date, locEast8); err == nil {
				startOfDay = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, locEast8)
			}
		}

		var goldList []News
		q := s.DB.Model(&News{}).Where("source = ?", "gold")
		if date != "" {
			q = q.Where("(published_date = ? OR (TRIM(COALESCE(published_date, '')) = '' AND to_char(published_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD') = ?))", date, date)
		} else {
			q = q.Where("published_at >= ?", startOfDay)
		}
		q.Order("published_at ASC").Limit(500).Find(&goldList)

		var ashareList []News
		aq := s.DB.Model(&News{}).Where("source = ?", "ashare")
		if date != "" {
			aq = aq.Where("(published_date = ? OR (TRIM(COALESCE(published_date, '')) = '' AND to_char(published_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD') = ?))", date, date)
		}
		aq.Order("published_at DESC").Limit(20).Find(&ashareList)

		list = append(goldList, ashareList...)
		if len(list) > limit {
			list = list[:limit]
		}
	} else {
		if channel != "" {
			db = db.Where("source = ?", channel)
		}
		switch sort {
		case "hot":
			db = db.Order("hot_score DESC").Order("published_at DESC")
		default:
			db = db.Order("published_at DESC")
		}
		if err := db.Limit(limit).Find(&list).Error; err != nil {
			return nil, err
		}
	}

	// 回写缓存（5 分钟，减轻每天首次打开时的 DB 压力）
	const listCacheTTL = 5 * time.Minute
	if s.Redis != nil && len(list) > 0 {
		if bs, err := json.Marshal(list); err == nil {
			_ = s.Redis.Set(ctx, cacheKey, bs, listCacheTTL).Err()
		}
	}

	return list, nil
}

// ListLatest 兼容旧接口
func (s *Store) ListLatest(limit int) ([]News, error) {
	return s.ListNews("", "latest", limit, "")
}

// ListPublishedDates 返回有数据的日期列表（倒序）。兼容旧数据：published_date 为空时用 published_at 的日期；结果缓存 5 分钟
func (s *Store) ListPublishedDates(channel string, limit int) ([]string, error) {
	if limit <= 0 || limit > 365 {
		limit = 31
	}
	ctx := context.Background()
	cacheKey := fmt.Sprintf("news:dates:%s:%d", channel, limit)
	if s.Redis != nil {
		if bs, err := s.Redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var cached []string
			if err := json.Unmarshal(bs, &cached); err == nil {
				return cached, nil
			}
		}
	}

	// 使用 COALESCE：有 published_date 用其值，否则用 published_at 的日期（东八区），保证历史数据也出现在日期列表
	sql := `SELECT DISTINCT COALESCE(NULLIF(TRIM(published_date), ''), to_char(published_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD')) AS d FROM news`
	args := []interface{}{}
	if channel != "" {
		sql += ` WHERE source = ?`
		args = append(args, channel)
	}
	sql += ` ORDER BY d DESC LIMIT ?`
	args = append(args, limit)

	var rows []struct{ D string }
	if err := s.DB.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	dates := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.D != "" {
			dates = append(dates, r.D)
		}
	}
	if s.Redis != nil && len(dates) > 0 {
		if bs, err := json.Marshal(dates); err == nil {
			_ = s.Redis.Set(ctx, cacheKey, bs, 5*time.Minute).Err()
		}
	}
	return dates, nil
}
