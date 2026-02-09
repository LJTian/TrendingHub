package collector

import (
	"log"
	"strconv"
	"strings"
	"time"

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

		item := NewsItem{
			Title:       title,
			URL:         relLink,
			Source:      "baidu",
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

