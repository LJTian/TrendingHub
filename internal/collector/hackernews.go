package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	hnBaseURL             = "https://hacker-news.firebaseio.com/v0"
	hnMaxItems            = 30
	hnMaxResponseBytes    = 1 << 20 // 1MB
	hnConcurrency         = 10
	hnClientTimeout       = 10 * time.Second
	hnItemClientTimeout   = 5 * time.Second
)

// HackerNewsFetcher 通过官方 Firebase API 抓取 Hacker News 热门故事
type HackerNewsFetcher struct{}

func (h *HackerNewsFetcher) Name() string {
	return "hackernews_top"
}

type hnItem struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Score       int    `json:"score"`
	Descendants int    `json:"descendants"`
	By          string `json:"by"`
	Time        int64  `json:"time"`
	Type        string `json:"type"`
}

func (h *HackerNewsFetcher) Fetch() ([]NewsItem, error) {
	log.Println("fetch Hacker News Top Stories...")

	client := &http.Client{Timeout: hnClientTimeout}

	resp, err := client.Get(hnBaseURL + "/topstories.json")
	if err != nil {
		return nil, fmt.Errorf("hackernews: fetch top stories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hackernews: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, hnMaxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("hackernews: read top stories: %w", err)
	}

	var ids []int
	if err := json.Unmarshal(body, &ids); err != nil {
		return nil, fmt.Errorf("hackernews: unmarshal top stories: %w", err)
	}

	if len(ids) > hnMaxItems {
		ids = ids[:hnMaxItems]
	}

	type indexedItem struct {
		idx  int
		item hnItem
	}

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		sem     = make(chan struct{}, hnConcurrency)
		items   = make([]indexedItem, 0, len(ids))
	)

	itemClient := &http.Client{Timeout: hnItemClientTimeout}

	for i, id := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx, id int) {
			defer wg.Done()
			defer func() { <-sem }()

			it, err := fetchHNItem(itemClient, id)
			if err != nil {
				log.Printf("hackernews: fetch item %d: %v", id, err)
				return
			}
			if it.Title == "" || it.Type != "story" {
				return
			}

			mu.Lock()
			items = append(items, indexedItem{idx: idx, item: it})
			mu.Unlock()
		}(i, id)
	}
	wg.Wait()

	// 并发翻译标题，避免串行调用外部 API 导致整体超时
	type translatedResult struct {
		idx        int
		item       hnItem
		rank       int
		translated string
	}

	var (
		twg     sync.WaitGroup
		tmu     sync.Mutex
		tsem    = make(chan struct{}, 3)
		tItems  = make([]translatedResult, 0, len(items))
	)

	for _, ii := range items {
		twg.Add(1)
		tsem <- struct{}{}
		go func(ii indexedItem) {
			defer twg.Done()
			defer func() { <-tsem }()

			translated := ii.item.Title
			if !isMostlyChinese(translated) {
				translated = translateToChinese(translated)
			}

			tmu.Lock()
			tItems = append(tItems, translatedResult{
				idx:        ii.idx,
				item:       ii.item,
				rank:       ii.idx + 1,
				translated: translated,
			})
			tmu.Unlock()
		}(ii)
	}
	twg.Wait()

	results := make([]NewsItem, 0, len(tItems))
	for _, tr := range tItems {
		it := tr.item

		itemURL := it.URL
		if itemURL == "" {
			itemURL = fmt.Sprintf("https://news.ycombinator.com/item?id=%d", it.ID)
		}

		results = append(results, NewsItem{
			Title:       tr.translated,
			URL:         itemURL,
			Source:      "hackernews",
			Description: tr.translated,
			PublishedAt: time.Unix(it.Time, 0),
			HotScore:    float64(it.Score),
			RawData: map[string]any{
				"hn_id":          it.ID,
				"original_title": it.Title,
				"author":         it.By,
				"comments":       it.Descendants,
				"score":          it.Score,
				"rank":           tr.rank,
			},
		})
	}

	if len(results) == 0 {
		log.Println("hackernews: no items fetched")
	}

	return results, nil
}

func fetchHNItem(client *http.Client, id int) (hnItem, error) {
	url := fmt.Sprintf("%s/item/%d.json", hnBaseURL, id)
	resp, err := client.Get(url)
	if err != nil {
		return hnItem{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return hnItem{}, fmt.Errorf("status %d", resp.StatusCode)
	}

	var it hnItem
	if err := json.NewDecoder(io.LimitReader(resp.Body, hnMaxResponseBytes)).Decode(&it); err != nil {
		return hnItem{}, err
	}
	return it, nil
}
