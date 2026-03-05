package collector

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const goldMaxResponseBytes = 64 * 1024  // 64KB，黄金 API 响应很小
const goldOzPerGram = 31.1034768        // 1 盎司 = 31.1034768 克，前端用 HotScore/此值得到元/克
const eastMoneyGoldSecID = "113.888"    // 东方财富 沪金主连（上期所），备用数据源；与 ashare 共用 eastMoneyStockGetURL

var goldAllowedHosts = []string{"data-asg.goldprice.org", "data-goldprice.org"}

// GoldPriceFetcher 从外部 API 拉取黄金价格，存储为人民币/盎司；前端展示时按 1 盎司=31.1034768 克换算为元/克。
// 优先使用 data-asg.goldprice.org 的 CNY 接口（GOLD_API_URL）；失败时复用东方财富 qt/stock/get 接口拉取沪金主连作为备用。
type GoldPriceFetcher struct{}

func (g *GoldPriceFetcher) Name() string {
	return "gold_price"
}

// 对应 data-asg.goldprice.org/dbXRates/CNY 的响应结构
type goldAPIResp struct {
	TS    int64  `json:"ts"`
	TSJ   int64  `json:"tsj"`
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, goldMaxResponseBytes))
	if err != nil {
		log.Printf("fetch gold price: read body failed: %v", err)
		return nil, err
	}
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		log.Printf("decode gold price response failed: empty response")
		return g.fetchGoldFromEastMoney()
	}
	// API 可能返回 HTML/错误页（如 403、5xx 或反爬），首字符非 { 或 [ 则不是 JSON
	first := body[0]
	if first != '{' && first != '[' {
		preview := string(body)
		if len(preview) > 300 {
			preview = preview[:300] + "..."
		}
		log.Printf("decode gold price response failed: not JSON (first byte %q), status=%d, preview: %s", first, resp.StatusCode, preview)
		return g.fetchGoldFromEastMoney()
	}

	var data goldAPIResp
	if err := json.Unmarshal(body, &data); err != nil {
		log.Printf("decode gold price response failed: %v", err)
		return g.fetchGoldFromEastMoney()
	}

	// 取第一条黄金价格
	if len(data.Items) == 0 {
		log.Printf("gold price response has no items")
		return g.fetchGoldFromEastMoney()
	}
	pricePerOz := data.Items[0].XAUPrice

	// 使用接口返回的时间戳，如果解析失败则退回当前时间
	t := time.Now()
	if data.TSJ != 0 {
		t = time.UnixMilli(data.TSJ)
	}

	// 每次采集使用带时间戳的 URL，使存储层插入新行而非更新同一条，从而保留历史用于折线图
	itemURL := apiURL + "?t=" + strconv.FormatInt(t.UnixMilli(), 10)

	item := NewsItem{
		Title:       "黄金价格（XAU/人民币）",
		URL:         itemURL,
		Source:      "gold",
		Description: "国际现货黄金（XAU）人民币（CNY）实时价格，单位元/克（由元/盎司换算），数据来自免费行情接口，仅供参考。",
		PublishedAt: t,
		HotScore:    pricePerOz, // 存元/盎司，前端按 1 盎司=31.1034768 克换算为元/克展示
		RawData: map[string]any{
			"price": pricePerOz,
			"ts":    data.TSJ,
		},
	}

	return []NewsItem{item}, nil
}

// fetchGoldFromEastMoney 复用与 A 股相同的东方财富 push2 qt/stock/get 接口，拉取沪金主连（或 GOLD_EASTMONEY_SECID 指定合约）作为金价备用源。
// 沪金报价一般为元/克，转为元/盎司存入 HotScore，与主源一致供前端 toPerGram 展示。
func (g *GoldPriceFetcher) fetchGoldFromEastMoney() ([]NewsItem, error) {
	secID := os.Getenv("GOLD_EASTMONEY_SECID")
	if secID == "" {
		secID = eastMoneyGoldSecID
	}
	client := &http.Client{Timeout: 10 * time.Second}
	params := url.Values{"secid": {secID}, "fields": {"f43,f58,f60,f170"}}
	u := eastMoneyStockGetURL + "?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("fetch gold from East Money failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		log.Printf("fetch gold from East Money read: %v", err)
		return nil, err
	}
	var payload struct {
		Data *struct {
			F43  float64 `json:"f43"`  // 最新价（沪金通常为元/克；若为分的接口则需/100）
			F58  string  `json:"f58"`  // 名称
			F60  float64 `json:"f60"`  // 昨收
			F170 float64 `json:"f170"` // 涨跌幅（百分比*100）
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.Data == nil {
		log.Printf("decode gold East Money response failed: %v", err)
		return nil, nil
	}
	d := payload.Data
	// 沪金期货接口 f43 常见为元/克；若数值很大（如 5 位数）则可能是「分」需除以 100
	pricePerGram := d.F43
	if pricePerGram > 10000 {
		pricePerGram = pricePerGram / 100
	}
	pricePerOz := pricePerGram * goldOzPerGram
	now := time.Now()
	name := d.F58
	if name == "" {
		name = "沪金主连"
	}
	itemURL := "https://quote.eastmoney.com/qihuo/au.html?t=" + strconv.FormatInt(now.UnixMilli(), 10)
	pct := d.F170 / 100
	changeStr := strconv.FormatFloat(pct, 'f', 2, 64)
	item := NewsItem{
		Title:       "黄金价格（XAU/人民币）",
		URL:         itemURL,
		Source:      "gold",
		Description: name + " " + strconv.FormatFloat(pricePerGram, 'f', 2, 64) + " 元/克 " + changeStr + "%，数据来自东方财富沪金，仅供参考。",
		PublishedAt: now,
		HotScore:    pricePerOz,
		RawData: map[string]any{
			"price": pricePerOz,
			"ts":    now.UnixMilli(),
			"source": "eastmoney",
		},
	}
	log.Printf("gold price fallback: using East Money 沪金, %.2f 元/克", pricePerGram)
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
