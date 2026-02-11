package collector

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const goldMaxResponseBytes = 64 * 1024 // 64KB，黄金 API 响应很小
var goldAllowedHosts = []string{"data-asg.goldprice.org", "data-goldprice.org"}

// GoldPriceFetcher 从外部 API 拉取黄金价格（人民币/克 或 人民币/盎司，由接口决定）。
// 默认使用 data-asg.goldprice.org 的 CNY 接口（人民币/盎司），
// 可通过环境变量 GOLD_API_URL 覆盖。
type GoldPriceFetcher struct{}

func (g *GoldPriceFetcher) Name() string {
	return "gold_price"
}

// 对应 data-asg.goldprice.org/dbXRates/CNY 的响应结构
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
		apiURL = "https://data-asg.goldprice.org/dbXRates/CNY"
	} else if !isAllowedGoldAPIURL(apiURL) {
		log.Printf("fetch gold price: GOLD_API_URL host not in whitelist, ignoring")
		apiURL = "https://data-asg.goldprice.org/dbXRates/CNY"
	}

	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(apiURL)
	if err != nil {
		log.Printf("fetch gold price failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	var data goldAPIResp
	if err := json.NewDecoder(io.LimitReader(resp.Body, goldMaxResponseBytes)).Decode(&data); err != nil {
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
		Title:       "黄金价格（XAU/人民币）",
		URL:         apiURL,
		Source:      "gold",
		Summary:     "现货黄金 XAU/CNY 最新报价（元/盎司）",
		Description: "国际现货黄金（XAU）人民币（CNY）实时价格，单位元/盎司，数据来自免费行情接口，仅供参考。",
		PublishedAt: t,
		HotScore:    price,
		RawData: map[string]any{
			"price": price,
			"ts":    data.TSJ,
		},
	}

	return []NewsItem{item}, nil
}

func isAllowedGoldAPIURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme != "https" {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	for _, allowed := range goldAllowedHosts {
		if host == allowed {
			return true
		}
	}
	return false
}
