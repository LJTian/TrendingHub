package collector

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

// GitHubTrendingMock 抓取 GitHub Trending，使用页上的仓库介绍（p 标签）作为详情介绍
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
	c.SetRequestTimeout(5 * time.Second)

	results := make([]NewsItem, 0, 20)

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

		starsText := strings.TrimSpace(e.ChildText("a[href$=\"/stargazers\"]"))
		stars := parseStars(starsText)

		// 从 Trending 页抓取仓库简短描述（p 标签）
		pageDesc := strings.TrimSpace(e.ChildText("p"))
		summary := repoName
		if stars > 0 {
			summary = repoName + " · " + starsText + " stars"
		}
		desc := pageDesc
		if desc == "" {
			desc = "GitHub Trending 仓库，点击标题前往查看详情。"
		} else if !isMostlyChinese(desc) {
			// 非汉语则翻译成中文
			desc = translateToChinese(desc)
		}

		item := NewsItem{
			Title:       repoName,
			URL:         fullURL,
			Source:      "github",
			Summary:     summary,
			Description: desc,
			PublishedAt: time.Now(),
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
