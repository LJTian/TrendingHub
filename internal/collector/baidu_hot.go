package collector

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// BaiduHotFetcher 抓取百度实时热搜榜。
// 实现方式与 ourongxing/newsnow 一致：从 HTML 中提取 <!--s-data:...--> 内嵌 JSON，
// 只使用其中的 word/rawUrl/desc，省去所有详情页与浏览器采集逻辑。
type BaiduHotFetcher struct{}

func (b *BaiduHotFetcher) Name() string {
	return "baidu_hot"
}

// 结构体对应内嵌 JSON 中我们关心的字段
type baiduState struct {
	Data struct {
		Cards []struct {
			Content []struct {
				IsTop  bool   `json:"isTop"`
				Word   string `json:"word"`
				RawURL string `json:"rawUrl"`
				Desc   string `json:"desc"`
			} `json:"content"`
		} `json:"cards"`
	} `json:"data"`
}

func (b *BaiduHotFetcher) Fetch() ([]NewsItem, error) {
	log.Println("fetch Baidu Hot Search...")

	resp, err := http.Get("https://top.baidu.com/board?tab=realtime")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("baidu_hot: unexpected status %d", resp.StatusCode)
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	html := string(body)

	// 从 <!--s-data: ... --> 中提取 JSON，正则与 newsnow 源码保持一致
	re := regexp.MustCompile(`(?s)<!--s-data:(.*?)-->`)
	matches := re.FindStringSubmatch(html)
	if len(matches) < 2 {
		log.Printf("baidu_hot: failed to extract s-data JSON")
		return nil, nil
	}

	var state baiduState
	if err := json.Unmarshal([]byte(matches[1]), &state); err != nil {
		log.Printf("baidu_hot: unmarshal s-data JSON error: %v", err)
		return nil, err
	}

	if len(state.Data.Cards) == 0 || len(state.Data.Cards[0].Content) == 0 {
		return nil, nil
	}

	contents := state.Data.Cards[0].Content
	now := time.Now()
	results := make([]NewsItem, 0, len(contents))

	for idx, c := range contents {
		if c.IsTop {
			continue
		}
		title := strings.TrimSpace(c.Word)
		if title == "" {
			continue
		}

		url := strings.TrimSpace(c.RawURL)
		if url == "" {
			url = "https://top.baidu.com/board?tab=realtime"
		}

		desc := strings.TrimSpace(c.Desc)
		if desc == "" {
			desc = title
		}

		// 与 newsnow 类似，这里没有真实“指数”字段，用排序位置近似热度（越靠前越大）
		hot := float64(len(contents) - idx)

		item := NewsItem{
			Title:       title,
			URL:         url,
			Source:      "baidu",
			Description: desc,
			PublishedAt: now,
			HotScore:    hot,
			RawData: map[string]any{
				"rank": idx + 1,
			},
		}
		results = append(results, item)
	}

	if len(results) == 0 {
		log.Printf("baidu_hot: no items parsed from s-data JSON")
	}

	return results, nil
}
