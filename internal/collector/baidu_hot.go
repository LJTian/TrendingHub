package collector

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

// BaiduHotFetcher 抓取百度实时热搜榜
type BaiduHotFetcher struct{}

func (b *BaiduHotFetcher) Name() string {
	return "baidu_hot"
}

func (b *BaiduHotFetcher) Fetch() ([]NewsItem, error) {
	log.Println("fetch Baidu Hot Search...")

	c := colly.NewCollector(
		colly.AllowedDomains("top.baidu.com"),
		colly.UserAgent("TrendingHubBot/1.0"),
	)
	c.SetRequestTimeout(5 * time.Second)

	results := make([]NewsItem, 0, 50)
	now := time.Now()

	// 页面结构可能调整，此处基于当前的 DOM 结构做“尽力而为”的解析
	c.OnHTML("div.category-wrap_iQLoo", func(e *colly.HTMLElement) {
		title := strings.TrimSpace(e.ChildText("div.c-single-text-ellipsis"))
		if title == "" {
			return
		}

		relLink := ""
		if href := e.ChildAttr("a", "href"); href != "" {
			if strings.HasPrefix(href, "http") {
				relLink = href
			} else {
				relLink = "https://top.baidu.com" + href
			}
		} else {
			relLink = "https://top.baidu.com/board?tab=realtime"
		}

		heatText := strings.TrimSpace(e.ChildText("div.hot-index_1Bl1a"))
		heat := parseInt(heatText)

		// 抓取红框内的“介绍”内容：优先匹配介绍/描述段落（百度页面结构可能变化）
		desc := strings.TrimSpace(e.ChildText("div[class*='content']"))
		if desc == "" {
			desc = strings.TrimSpace(e.ChildText("div[class*='Content']"))
		}
		if desc == "" {
			desc = strings.TrimSpace(e.ChildText("div[class*='desc']"))
		}
		if desc == "" {
			desc = strings.TrimSpace(e.ChildText("div[class*='abstract']"))
		}
		if desc == "" {
			desc = strings.TrimSpace(e.ChildText("div[class*='intro']"))
		}
		if desc == "" {
			desc = strings.TrimSpace(e.ChildText("div[class*='summary']"))
		}
		if desc == "" {
			desc = strings.TrimSpace(e.ChildText("div[class*='detail']"))
		}
		if desc == "" {
			desc = strings.TrimSpace(e.ChildText("p"))
		}
		if desc == "" {
			desc = strings.TrimSpace(e.ChildText("span[class*='content']"))
		}
		if desc == "" {
			desc = strings.TrimSpace(e.ChildText("span[class*='desc']"))
		}
		// 红框介绍常在标题下方：取标题所在块之后的第一个长文本块
		if desc == "" {
			desc = firstLongTextAfter(e, "div.c-single-text-ellipsis", 20, title, heatText)
		}
		// 兜底：从当前块内取非标题、非热度的最长段落（红框式介绍文案）
		if desc == "" {
			desc = fallbackBaiduDesc(e, title, heatText)
		}
		// 去掉“查看更多”等链接文案，只保留正文
		desc = cleanBaiduDesc(desc)

		summary := desc
		if len(summary) > 120 {
			summary = summary[:120] + "…"
		}
		if summary == "" {
			summary = title
		}

		item := NewsItem{
			Title:       title,
			URL:         relLink,
			Source:      "baidu",
			Summary:     summary,
			Description: desc,
			PublishedAt: now,
			HotScore:    float64(heat),
			RawData: map[string]any{
				"heat": heatText,
			},
		}
		results = append(results, item)
	})

	if err := c.Visit("https://top.baidu.com/board?tab=realtime"); err != nil {
		log.Printf("fetch Baidu Hot Search failed: %v", err)
		return nil, err
	}

	if len(results) == 0 {
		log.Printf("fetch Baidu Hot Search got 0 items")
	}

	return results, nil
}

// cleanBaiduDesc 去掉简介中的“查看更多”等链接文案，只保留正文
func cleanBaiduDesc(s string) string {
	s = strings.TrimSpace(s)
	for _, cut := range []string{"[查看更多>]", "[查看更多&gt;]", "查看更多", "…[查看更多>]", "…[查看更多&gt;]"} {
		if idx := strings.Index(s, cut); idx != -1 {
			s = strings.TrimSpace(s[:idx])
		}
	}
	return s
}

// firstLongTextAfter 从 selector 匹配到的元素之后的兄弟节点中取第一个足够长的文本（用于红框介绍）
func firstLongTextAfter(e *colly.HTMLElement, selector string, minLen int, exclude ...string) string {
	sel := e.DOM.Find(selector).First()
	for i := 0; i < 10; i++ {
		sel = sel.Next()
		if sel.Length() == 0 {
			break
		}
		t := strings.TrimSpace(sel.Text())
		if len(t) < minLen {
			continue
		}
		skip := false
		for _, ex := range exclude {
			if ex != "" && t == ex {
				skip = true
				break
			}
		}
		if !skip {
			return t
		}
	}
	return ""
}

// fallbackBaiduDesc 从当前条目内找“红框”式介绍：非标题、非热度的最长段落（优先像介绍文案的）
func fallbackBaiduDesc(e *colly.HTMLElement, title, heatText string) string {
	var best string
	minLen := 20 // 介绍至少有一定长度

	e.DOM.Find("div, p, span").Each(func(i int, s *goquery.Selection) {
		t := strings.TrimSpace(s.Text())
		if t == "" || t == title || t == heatText || len(t) < minLen {
			return
		}
		// 排除纯数字（热度）
		if _, err := strconv.Atoi(strings.TrimSpace(strings.ReplaceAll(t, ",", ""))); err == nil && len(t) < 30 {
			return
		}
		// 优先保留更长、更像介绍段落的（含逗号/句号等）
		if len(t) > len(best) {
			best = t
		}
	})
	return best
}

func parseInt(s string) int {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	if s == "" {
		return 0
	}
	// 去掉可能的“万”等单位，只保留数字部分
	end := 0
	for ; end < len(s); end++ {
		if s[end] < '0' || s[end] > '9' {
			break
		}
	}
	numPart := s[:end]
	n, err := strconv.Atoi(numPart)
	if err != nil {
		return 0
	}
	return n
}
