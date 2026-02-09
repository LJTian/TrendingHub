package processor

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
	"time"

	"github.com/LJTian/TrendingHub/internal/collector"
)

// ProcessedNews 是写入存储层前的统一结构
type ProcessedNews struct {
	ID          string
	Title       string
	URL         string
	Source      string
	Summary     string
	PublishedAt time.Time
	HotScore    float64
	RawData     map[string]any
}

// SimpleProcessor 做最基础的数据清洗与 ID 生成
type SimpleProcessor struct{}

func NewSimpleProcessor() *SimpleProcessor {
	return &SimpleProcessor{}
}

func (p *SimpleProcessor) Process(items []collector.NewsItem) []ProcessedNews {
	out := make([]ProcessedNews, 0, len(items))
	seen := make(map[string]struct{})

	for _, it := range items {
		id := hashURL(it.URL)
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		out = append(out, ProcessedNews{
			ID:          id,
			Title:       strings.TrimSpace(it.Title),
			URL:         it.URL,
			Source:      it.Source,
			Summary:     it.Title, // MVP: 暂用标题做摘要
			PublishedAt: it.PublishedAt,
			HotScore:    it.HotScore,
			RawData:     it.RawData,
		})
	}

	return out
}

func hashURL(url string) string {
	h := sha1.New()
	h.Write([]byte(url))
	return hex.EncodeToString(h.Sum(nil))
}

