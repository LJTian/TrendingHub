package collector

import "time"

// NewsItem 统一采集后的基础结构
type NewsItem struct {
	Title       string
	URL         string
	Source      string
	PublishedAt time.Time
	HotScore    float64
	RawData     map[string]any
}

// Fetcher 抽象每一个数据源
type Fetcher interface {
	Name() string
	Fetch() ([]NewsItem, error)
}

