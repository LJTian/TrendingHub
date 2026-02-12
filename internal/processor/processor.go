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
	Description string
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

		// description 统一在后端做长度控制，最多保留约 600 个字符
		desc := truncateRunes(strings.TrimSpace(it.Description), 600)
		if desc == "" {
			// 兜底：没有提供 description 时，用标题作为简短介绍
			desc = truncateRunes(strings.TrimSpace(it.Title), 600)
		}
		out = append(out, ProcessedNews{
			ID:          id,
			Title:       strings.TrimSpace(it.Title),
			URL:         it.URL,
			Source:      it.Source,
			Description: desc,
			PublishedAt: it.PublishedAt,
			HotScore:    it.HotScore,
			RawData:     it.RawData,
		})
	}

	return out
}

// hashURL 仅用于去重与主键生成，非密码学用途；若需安全场景请改用 SHA256。
func hashURL(url string) string {
	h := sha1.New()
	h.Write([]byte(url))
	return hex.EncodeToString(h.Sum(nil))
}

// truncateRunes 按 rune 数截断字符串，避免中文被截成半个字符
func truncateRunes(s string, limit int) string {
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
	return string(rs[:limit]) + "…"
}