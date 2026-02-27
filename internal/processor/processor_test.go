package processor

import (
	"testing"
	"time"

	"github.com/LJTian/TrendingHub/internal/collector"
)

func TestHashURLDeterministicAndDistinct(t *testing.T) {
	url1 := "https://example.com/a"
	url2 := "https://example.com/b"

	h1a := hashURL(url1)
	h1b := hashURL(url1)
	h2 := hashURL(url2)

	if h1a != h1b {
		t.Fatalf("hashURL not deterministic: %q vs %q", h1a, h1b)
	}
	if h1a == h2 {
		t.Fatalf("hashURL should differ for different URLs: %q", h1a)
	}
}

func TestTruncateRunesHandlesChineseAndEllipsis(t *testing.T) {
	s := "你好，世界，这是一个很长的中文句子，用来测试截断逻辑。"
	out := truncateRunes(s, 5)
	if len([]rune(out)) != 6 { // 5 个字符 + 1 个省略号
		t.Fatalf("truncateRunes length = %d, want 6 (including ellipsis): %q", len([]rune(out)), out)
	}
	if out[len(out)-3:] != "…" && out[len(out)-3:] != "…" { // 简单检查末尾是否为省略号
		t.Fatalf("truncateRunes should append ellipsis: %q", out)
	}

	// limit 大于长度时不应截断
	full := truncateRunes("短文本", 10)
	if full != "短文本" {
		t.Fatalf("truncateRunes should keep original when under limit: %q", full)
	}
}

func TestSimpleProcessorDeduplicateAndFillDescription(t *testing.T) {
	p := NewSimpleProcessor()
	now := time.Now()

	items := []collector.NewsItem{
		{
			Title:       "Title 1",
			URL:         "https://example.com/1",
			Source:      "test",
			Description: "desc 1",
			PublishedAt: now,
			HotScore:    1,
		},
		{
			Title:       "Title 1 duplicate by URL",
			URL:         "https://example.com/1",
			Source:      "test",
			Description: "desc 1 dup",
			PublishedAt: now,
			HotScore:    2,
		},
		{
			Title:       "Title 2 no desc",
			URL:         "https://example.com/2",
			Source:      "test",
			Description: "",
			PublishedAt: now,
			HotScore:    3,
		},
	}

	out := p.Process(items)
	if len(out) != 2 {
		t.Fatalf("expected 2 processed items after dedupe, got %d", len(out))
	}

	// 第一条保留 Description
	if out[0].Description == "" {
		t.Fatalf("first item should keep non-empty description")
	}

	// 第二条没有 description，应使用 Title 兜底
	if out[1].Description == "" {
		t.Fatalf("second item should use title as fallback description")
	}
	if out[1].Description != "Title 2 no desc" {
		t.Fatalf("unexpected fallback description: %q", out[1].Description)
	}
}

