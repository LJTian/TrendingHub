package collector

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

// GitHubTrendingMock 最初是示例，现在实现一个简单的 GitHub Trending 抓取
type GitHubTrendingMock struct{}

func (g *GitHubTrendingMock) Name() string {
	return "github_trending"
}

func (g *GitHubTrendingMock) Fetch() ([]NewsItem, error) {
	log.Println("fetch GitHub Trending...")

	c := colly.NewCollector(
		colly.AllowedDomains("github.com"),
		colly.UserAgent("TrendingHubBot/1.0"),
	)
	// 避免长时间阻塞，设置较短超时
	c.SetRequestTimeout(5 * time.Second)

	results := make([]NewsItem, 0, 20)

	// GitHub Trending 页面结构可能变动，此处实现为“尽力而为”的解析
	c.OnHTML("article.Box-row", func(e *colly.HTMLElement) {
		titleSel := e.DOM.Find("h2 a")
		if titleSel.Length() == 0 {
			return
		}

		repoName := strings.TrimSpace(titleSel.Text())
		href, exists := titleSel.Attr("href")
		if !exists {
			return
		}

		fullURL := "https://github.com" + strings.TrimSpace(href)

		// star 数
		starsText := strings.TrimSpace(e.ChildText("a[href$=\"/stargazers\"]"))
		stars := parseStars(starsText)

		// 简单用 star 数作为 hotScore
		item := NewsItem{
			Title:       repoName,
			URL:         fullURL,
			Source:      "github",
			PublishedAt: time.Now(), // Trending 没有明确发布时间，使用当前时间
			HotScore:    float64(stars),
			RawData: map[string]any{
				"stars": stars,
			},
		}
		results = append(results, item)
	})

	if err := c.Visit("https://github.com/trending"); err != nil {
		log.Printf("fetch GitHub Trending failed: %v", err)
		return nil, err
	}

	// 如果因为网络或页面结构变化导致解析不到任何条目，直接返回空结果
	if len(results) == 0 {
		log.Printf("fetch GitHub Trending got 0 items")
		return nil, nil
	}

	return results, nil
}

// parseStars 将 GitHub Trending 中“12.3k”之类的文本解析为整数
func parseStars(text string) int {
	text = strings.ReplaceAll(text, ",", "")
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}

	multiplier := 1.0
	if strings.HasSuffix(text, "k") || strings.HasSuffix(text, "K") {
		multiplier = 1000
		text = strings.TrimSuffix(strings.TrimSuffix(text, "k"), "K")
	}

	f, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil {
		return 0
	}
	return int(f * multiplier)
}
