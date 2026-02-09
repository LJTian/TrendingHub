package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	ID          string            `gorm:"primaryKey;size:40" json:"id"`
	Title       string            `gorm:"size:512" json:"title"`
	URL         string            `gorm:"size:1024;uniqueIndex" json:"url"`
	Source      string            `gorm:"size:64;index" json:"source"` // 兼容老字段，代表渠道 code
	Summary     string            `gorm:"size:1024" json:"summary"`
	PublishedAt time.Time         `gorm:"index" json:"publishedAt"`
	HotScore    float64           `gorm:"index" json:"hotScore"`
	ExtraData   datatypes.JSONMap `gorm:"type:jsonb" json:"extraData"` // 存储原始扩展字段

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

// SaveBatch 保存一批新闻，已存在的根据 URL 忽略
func (s *Store) SaveBatch(items []processor.ProcessedNews) error {
	for _, it := range items {
		n := &News{
			ID:          it.ID,
			Title:       it.Title,
			URL:         it.URL,
			Source:      it.Source,
			Summary:     it.Summary,
			PublishedAt: it.PublishedAt,
			HotScore:    it.HotScore,
			ExtraData:   datatypes.JSONMap(it.RawData),
		}

		// 以 URL 作为幂等键，避免重复插入
		if err := s.DB.Where("url = ?", it.URL).FirstOrCreate(n).Error; err != nil {
			return err
		}
	}

	// 这里不做按 key 通配删除，完全依赖短 TTL 的缓存自然过期，
	// 避免使用无效的通配符删除以及增加额外的 Redis 扫描复杂度。
	return nil
}

// ListNews 按渠道与排序方式返回新闻列表，并使用 Redis 做简单缓存
// channel: 渠道 code，可为空
// sort: latest(默认) / hot
func (s *Store) ListNews(channel, sort string, limit int) ([]News, error) {
	if limit <= 0 || limit > 1000 {
		limit = 20
	}
	if sort == "" {
		sort = "latest"
	}

	ctx := context.Background()
	cacheKey := fmt.Sprintf("news:list:%s:%s:%d", channel, sort, limit)

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

	// 特殊处理黄金：默认返回当天所有数据，按时间正序
	if channel == "gold" {
		now := time.Now()
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		db = db.Where("source = ?", "gold").
			Where("published_at >= ?", startOfDay).
			Order("published_at ASC")
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
	}

	if err := db.Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}

	// 回写缓存
	if s.Redis != nil && len(list) > 0 {
		if bs, err := json.Marshal(list); err == nil {
			_ = s.Redis.Set(ctx, cacheKey, bs, 60*time.Second).Err()
		}
	}

	return list, nil
}

// ListLatest 兼容旧接口
func (s *Store) ListLatest(limit int) ([]News, error) {
	return s.ListNews("", "latest", limit)
}

