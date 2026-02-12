package collector

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

const xTrendsURL = "https://trends24.in/"
const xTrendsMaxItems = 50
const xTrendsMaxBodyBytes = 2 << 20 // 2MB，防止超大 HTML 导致 DoS

// XTrendsFetcher 抓取 X (Twitter) 热搜，数据来自 trends24.in（全球榜）
type XTrendsFetcher struct{}

func (x *XTrendsFetcher) Name() string {
	return "x_trends"
}

func (x *XTrendsFetcher) Fetch() ([]NewsItem, error) {
	log.Println("fetch X (Twitter) trends...")

	list := x.fetchWithColly()
	if len(list) == 0 {
		list = x.fetchWithHTTP()
	}
	if len(list) == 0 {
		list = x.fetchFromGetdaytrends()
	}

	if len(list) == 0 {
		log.Printf("fetch X trends got 0 items")
		return nil, nil
	}

	if len(list) > xTrendsMaxItems {
		list = list[:xTrendsMaxItems]
	}

	now := time.Now()
	results := make([]NewsItem, 0, len(list))
	for i, t := range list {
		hotScore := float64(xTrendsMaxItems - i)
		if hotScore < 1 {
			hotScore = 1
		}
		results = append(results, NewsItem{
			Title:       t.title,
			URL:         t.url,
			Source:      "x",
			Description: "X (Twitter) 热搜话题，点击在 X 上搜索。",
			PublishedAt: now,
			HotScore:    hotScore,
			RawData:     map[string]any{"rank": i + 1},
		})
	}
	return results, nil
}

type xTrend struct {
	title string
	url   string
}

func (x *XTrendsFetcher) fetchWithColly() []xTrend {
	c := colly.NewCollector(
		colly.AllowedDomains("trends24.in", "www.trends24.in"),
		colly.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)
	c.SetRequestTimeout(15 * time.Second)

	var list []xTrend
	seen := make(map[string]bool)

	c.OnHTML("a.trend-link[href*='twitter.com/search'], a[href*='twitter.com/search']", func(e *colly.HTMLElement) {
		href, _ := e.DOM.Attr("href")
		href = strings.TrimSpace(href)
		if href == "" || seen[href] {
			return
		}
		title := strings.TrimSpace(e.DOM.Text())
		if title == "" {
			return
		}
		url := toXSearchURL(href)
		seen[href] = true
		list = append(list, xTrend{title: title, url: url})
	})

	if err := c.Visit(xTrendsURL); err != nil {
		log.Printf("fetch X trends (colly): %v", err)
		return nil
	}
	return list
}

// fetchWithHTTP 备用：直接 GET 后用正则从 HTML 中提取 trend 链接
func (x *XTrendsFetcher) fetchWithHTTP() []xTrend {
	body, err := x.httpGet(xTrendsURL)
	if err != nil {
		return nil
	}
	list := x.parseTrendLinks(body)
	if len(list) > 0 {
		return list
	}
	// 若全球榜无结果，尝试美国区
	bodyUS, err := x.httpGet("https://trends24.in/united-states/")
	if err != nil {
		return nil
	}
	return x.parseTrendLinks(bodyUS)
}

func (x *XTrendsFetcher) httpGet(url string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("fetch X trends (http): %v", err)
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, xTrendsMaxBodyBytes))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// parseTrendLinks 从 HTML 中解析出所有 twitter.com/search 链接及标题（多种格式兼容）
func (x *XTrendsFetcher) parseTrendLinks(html string) []xTrend {
	seen := make(map[string]bool)
	var list []xTrend

	// 格式1: <a href="https://twitter.com/search?q=..." ...>标题</a>（class 可有可无、顺序任意）
	re1 := regexp.MustCompile(`<a\s+[^>]*href="(https://twitter\.com/search\?q=[^"]+)"[^>]*>([^<]+)</a>`)
	for _, m := range re1.FindAllStringSubmatch(html, -1) {
		if len(m) != 3 {
			continue
		}
		href := m[1]
		title := strings.TrimSpace(m[2])
		if title == "" || len(title) > 200 || seen[href] {
			continue
		}
		seen[href] = true
		list = append(list, xTrend{title: title, url: toXSearchURL(href)})
	}

	// 格式2: 仅有 href，用 q= 后的值解码作为标题（兜底）
	if len(list) == 0 {
		re2 := regexp.MustCompile(`href="(https://twitter\.com/search\?q=([^"]+))"`)
		for _, m := range re2.FindAllStringSubmatch(html, -1) {
			if len(m) < 3 || seen[m[1]] {
				continue
			}
			href := m[1]
			seen[href] = true
			title := m[2]
			if dec, err := url.QueryUnescape(title); err == nil && dec != "" {
				title = dec
			}
			if len(title) > 200 {
				title = title[:200]
			}
			list = append(list, xTrend{title: title, url: toXSearchURL(href)})
		}
	}
	return list
}

func toXSearchURL(twitterSearchURL string) string {
	if strings.Contains(twitterSearchURL, "twitter.com") {
		return "https://x.com/search?" + strings.TrimPrefix(twitterSearchURL, "https://twitter.com/search?")
	}
	return twitterSearchURL
}

// fetchFromGetdaytrends 备用：从 getdaytrends.com 解析全球榜，链接形如 /trend/话题名/
func (x *XTrendsFetcher) fetchFromGetdaytrends() []xTrend {
	body, err := x.httpGet("https://getdaytrends.com/")
	if err != nil {
		return nil
	}
	// 匹配 <a href="https://getdaytrends.com/trend/XXX/"> 或 /trend/XXX/ ，取链接文本或路径最后一段为标题
	re := regexp.MustCompile(`<a\s+href="https://getdaytrends\.com/trend/([^"]+)/?"[^>]*>([^<]+)</a>`)
	seen := make(map[string]bool)
	var list []xTrend
	for _, m := range re.FindAllStringSubmatch(body, -1) {
		if len(m) < 3 {
			continue
		}
		pathPart := m[1]           // URL 编码的话题名
		linkText := strings.TrimSpace(m[2])
		if linkText == "" || len(linkText) > 200 {
			continue
		}
		title := linkText
		if dec, err := url.QueryUnescape(pathPart); err == nil && dec != "" {
			title = dec
		}
		// 去重：同一标题只保留一次
		if seen[title] {
			continue
		}
		seen[title] = true
		xURL := "https://x.com/search?q=" + url.QueryEscape(title)
		list = append(list, xTrend{title: title, url: xURL})
	}
	return list
}
