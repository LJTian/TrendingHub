package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/LJTian/TrendingHub/internal/processor"
	"github.com/redis/go-redis/v9"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// 各频道对应独立表名，写入/查询均按 source 路由到对应表
var (
	allowedSources = []string{"github", "baidu", "gold", "ashare", "x", "hackernews"}
	sourceToTable  = map[string]string{
		"github": "news_github", "baidu": "news_baidu", "gold": "news_gold",
		"ashare": "news_ashare", "x": "news_x", "hackernews": "news_hackernews",
	}
)

func newsTable(source string) string {
	if t, ok := sourceToTable[source]; ok {
		return t
	}
	return ""
}

func sortByPublishedAtDesc(list []News) {
	sort.Slice(list, func(i, j int) bool { return list[i].PublishedAt.After(list[j].PublishedAt) })
}
func sortByHotScoreDesc(list []News) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].HotScore != list[j].HotScore {
			return list[i].HotScore > list[j].HotScore
		}
		return list[i].PublishedAt.After(list[j].PublishedAt)
	})
}

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

const (
	dbConnectRetries = 10
	dbConnectDelay   = 2 * time.Second
)

func NewStore(dsn, redisAddr string) (*Store, error) {
	var db *gorm.DB
	var err error
	for i := 0; i < dbConnectRetries; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			break
		}
		if i < dbConnectRetries-1 {
			log.Printf("database not ready (attempt %d/%d): %v; retry in %v", i+1, dbConnectRetries, err, dbConnectDelay)
			time.Sleep(dbConnectDelay)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to connect after %d attempts: %w", dbConnectRetries, err)
	}

	if err := db.AutoMigrate(&Channel{}, &News{}, &WeatherCity{}, &WeatherCache{}, &AShareStock{}); err != nil {
		return nil, err
	}
	// 按频道分表：与 news 同结构，便于按 source 路由；并行建表
	var createErr error
	var createErrMu sync.Mutex
	var wg sync.WaitGroup
	for _, src := range allowedSources {
		tbl := sourceToTable[src]
		wg.Add(1)
		go func(tbl string) {
			defer wg.Done()
			if err := db.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (LIKE news INCLUDING ALL)", tbl)).Error; err != nil {
				createErrMu.Lock()
				if createErr == nil {
					createErr = fmt.Errorf("create table %s: %w", tbl, err)
				}
				createErrMu.Unlock()
			}
		}(tbl)
	}
	wg.Wait()
	if createErr != nil {
		return nil, createErr
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

// HasAshareDataForDate 判断指定日期（YYYY-MM-DD，东八区）是否已有任何 A 股数据，
// 用于在采集层决定是否需要在收盘后额外补拉一次“当天快照”。
func (s *Store) HasAshareDataForDate(date string) bool {
	if date == "" {
		now := time.Now().In(locEast8)
		date = now.Format("2006-01-02")
	}
	const dateWhere = "(published_date = ? OR (TRIM(COALESCE(published_date, '')) = '' AND to_char(published_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD') = ?))"

	var cnt int64
	if err := s.DB.Table("news_ashare").Where(dateWhere, date, date).Count(&cnt).Error; err != nil {
		log.Printf("HasAshareDataForDate(%s) error: %v", date, err)
		return false
	}
	return cnt > 0
}

// EnsureChannel 确保某个渠道存在，使用 FirstOrCreate 避免并发竞态
func (s *Store) EnsureChannel(code, name, baseURL string) (*Channel, error) {
	ch := &Channel{
		Code:    code,
		Name:    name,
		BaseURL: baseURL,
		Status:  "active",
	}
	if err := s.DB.Where("code = ?", code).FirstOrCreate(ch).Error; err != nil {
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

// SaveBatch 按频道保存到对应分表（news_github / news_baidu / news_gold / news_ashare / news_x），已存在的按 URL 更新
func (s *Store) SaveBatch(items []processor.ProcessedNews) error {
	for _, it := range items {
		tbl := newsTable(it.Source)
		if tbl == "" {
			continue
		}
		pubDate := it.PublishedAt.In(locEast8).Format("2006-01-02")
		title := toValidUTF8(it.Title)
		description := toValidUTF8(it.Description)
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

		if err := s.DB.Table(tbl).Where("url = ?", it.URL).FirstOrCreate(n).Error; err != nil {
			return err
		}
		if err := s.DB.Table(tbl).Model(n).Updates(map[string]any{
			"title":          title,
			"description":    description,
			"hot_score":      it.HotScore,
			"published_at":   it.PublishedAt,
			"published_date": pubDate,
			"extra_data":     datatypes.JSONMap(it.RawData),
		}).Error; err != nil {
			return fmt.Errorf("update %s %s: %w", tbl, it.URL, err)
		}
	}
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

	// 按频道分表查询
	dateCond := date != ""
	dateWhere := "(published_date = ? OR (TRIM(COALESCE(published_date, '')) = '' AND to_char(published_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD') = ?))"

	// 金融渠道：从 news_gold + news_ashare 合并
	if channel == "gold" {
		now := time.Now().In(locEast8)
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, locEast8)
		if date != "" {
			if t, err := time.ParseInLocation("2006-01-02", date, locEast8); err == nil {
				startOfDay = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, locEast8)
			}
		}
		var goldList, ashareList []News
		q := s.DB.Table("news_gold")
		if dateCond {
			q = q.Where(dateWhere, date, date)
		} else {
			q = q.Where("published_at >= ?", startOfDay)
		}
		q.Order("published_at ASC").Limit(500).Find(&goldList)
		aq := s.DB.Table("news_ashare")
		if dateCond {
			aq = aq.Where(dateWhere, date, date)
		} else {
			// 金融首页 / 自选股等不指定日期时：只取当天的 A 股数据，
			// 避免把前几天或盘后采集的数据混入，导致分时图在时间轴上“偏移”。
			aq = aq.Where("published_at >= ?", startOfDay)
		}
		aq.Order("published_at ASC").Limit(500).Find(&ashareList)
		list := append(goldList, ashareList...)
		if len(list) > limit {
			list = list[:limit]
		}
		// 回写缓存
		if s.Redis != nil && len(list) > 0 {
			if bs, err := json.Marshal(list); err == nil {
				_ = s.Redis.Set(ctx, cacheKey, bs, 5*time.Minute).Err()
			}
		}
		return list, nil
	}

	// 单频道：从对应分表查
	if channel != "" {
		tbl := newsTable(channel)
		if tbl != "" {
			var list []News
			db := s.DB.Table(tbl)
			if dateCond {
				db = db.Where(dateWhere, date, date)
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
			if s.Redis != nil && len(list) > 0 {
				if bs, err := json.Marshal(list); err == nil {
					_ = s.Redis.Set(ctx, cacheKey, bs, 5*time.Minute).Err()
				}
			}
			return list, nil
		}
	}

	// channel == ""：从所有分表合并后排序截断
	var list []News
	for _, tbl := range sourceToTable {
		var part []News
		db := s.DB.Table(tbl)
		if dateCond {
			db = db.Where(dateWhere, date, date)
		}
		db.Order("published_at DESC").Limit(limit * 2).Find(&part)
		list = append(list, part...)
	}
	switch sort {
	case "hot":
		sortByHotScoreDesc(list)
	default:
		sortByPublishedAtDesc(list)
	}
	if len(list) > limit {
		list = list[:limit]
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

	// 从分表取有数据的日期；channel 为空时合并所有表
	baseSQL := `SELECT DISTINCT COALESCE(NULLIF(TRIM(published_date), ''), to_char(published_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD')) AS d FROM `
	var tables []string
	if channel == "" {
		for _, t := range sourceToTable {
			tables = append(tables, t)
		}
	} else if channel == "gold" {
		tables = []string{"news_gold", "news_ashare"}
	} else if t := newsTable(channel); t != "" {
		tables = []string{t}
	}
	if len(tables) == 0 {
		if s.Redis != nil {
			_ = s.Redis.Set(ctx, cacheKey, "[]", 5*time.Minute).Err()
		}
		return []string{}, nil
	}
	var dateSetMu sync.Mutex
	dateSet := make(map[string]struct{})
	var wg sync.WaitGroup
	for _, tbl := range tables {
		tbl := tbl
		wg.Add(1)
		go func() {
			defer wg.Done()
			var rows []struct{ D string }
			if err := s.DB.Raw(baseSQL+tbl+` ORDER BY d DESC LIMIT ?`, limit).Scan(&rows).Error; err != nil {
				return
			}
			dateSetMu.Lock()
			for _, r := range rows {
				if r.D != "" {
					dateSet[r.D] = struct{}{}
				}
			}
			dateSetMu.Unlock()
		}()
	}
	wg.Wait()
	dates := make([]string, 0, len(dateSet))
	for d := range dateSet {
		dates = append(dates, d)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	if len(dates) > limit {
		dates = dates[:limit]
	}
	if s.Redis != nil && len(dates) > 0 {
		if bs, err := json.Marshal(dates); err == nil {
			_ = s.Redis.Set(ctx, cacheKey, bs, 5*time.Minute).Err()
		}
	}
	return dates, nil
}
