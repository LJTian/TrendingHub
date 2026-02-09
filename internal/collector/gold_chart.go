package collector

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

// GoldPriceFetcher 从外部 API 拉取黄金价格。
// 默认使用 data-asg.goldprice.org 的免费 JSON 接口，
// 也可以通过环境变量 GOLD_API_URL 覆盖。
type GoldPriceFetcher struct{}

func (g *GoldPriceFetcher) Name() string {
	return "gold_price"
}

// 对应 https://data-asg.goldprice.org/dbXRates/USD 的响应结构
type goldAPIResp struct {
	TS    int64 `json:"ts"`
	TSJ   int64 `json:"tsj"`
	Date  string `json:"date"`
	Items []struct {
		Curr     string  `json:"curr"`
		XAUPrice float64 `json:"xauPrice"`
	} `json:"items"`
}

func (g *GoldPriceFetcher) Fetch() ([]NewsItem, error) {
	apiURL := os.Getenv("GOLD_API_URL")
	if apiURL == "" {
		apiURL = "https://data-asg.goldprice.org/dbXRates/USD"
	}

	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(apiURL)
	if err != nil {
		log.Printf("fetch gold price failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	var data goldAPIResp
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		log.Printf("decode gold price response failed: %v", err)
		return nil, err
	}

	// 取第一条黄金价格
	if len(data.Items) == 0 {
		log.Printf("gold price response has no items")
		return nil, nil
	}
	price := data.Items[0].XAUPrice

	// 使用接口返回的时间戳，如果解析失败则退回当前时间
	t := time.Now()
	if data.TSJ != 0 {
		t = time.UnixMilli(data.TSJ)
	}

	item := NewsItem{
		Title:       "黄金价格（XAU/USD）",
		URL:         apiURL,
		Source:      "gold",
		PublishedAt: t,
		HotScore:    price,
		RawData: map[string]any{
			"price": price,
			"ts":    data.TSJ,
		},
	}

	return []NewsItem{item}, nil
}

