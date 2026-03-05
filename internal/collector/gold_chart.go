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
const eastMoneyGoldSecID = "113.888"    // 已废弃：qt/stock/get 对期货返回 data=null，改用 clist 拉列表
const eastMoneyClistURL  = "https://push2.eastmoney.com/api/qt/clist/get" // 与指数/股票同源，期货用此列表接口
const eastMoneyShfeFS    = "m:113"      // 上期所（113），不加 t:1 才能拉到沪金等品种

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

	// 优先用主源；失败时先重试 data-goldprice.org（同路径、同结构），再回退东财沪金
	item, err := g.fetchGoldFromURL(apiURL)
	if item != nil {
		return []NewsItem{*item}, nil
	}
	if err != nil {
		log.Printf("fetch gold price failed: %v", err)
	}
	// 主源失败（403/非 JSON/无 items）：若当前不是 data-goldprice.org，则重试该备用域名
	if !strings.Contains(apiURL, "data-goldprice.org") {
		altURL := "https://data-goldprice.org/dbXRates/CNY"
		if item2, _ := g.fetchGoldFromURL(altURL); item2 != nil {
			log.Printf("gold price: using fallback host data-goldprice.org")
			return []NewsItem{*item2}, nil
		}
	}
	return g.fetchGoldFromEastMoney()
}

// fetchGoldFromURL 从 goldprice.org 系 URL 拉取并解析，返回单条或 nil（并可选 err）
func (g *GoldPriceFetcher) fetchGoldFromURL(apiURL string) (*NewsItem, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, goldMaxResponseBytes))
	if err != nil {
		return nil, err
	}
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil, nil
	}
	first := body[0]
	if first != '{' && first != '[' {
		return nil, nil // 非 JSON，由调用方打日志
	}
	var data goldAPIResp
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	if len(data.Items) == 0 {
		return nil, nil
	}
	pricePerOz := data.Items[0].XAUPrice
	t := time.Now()
	if data.TSJ != 0 {
		t = time.UnixMilli(data.TSJ)
	}
	itemURL := apiURL + "?t=" + strconv.FormatInt(t.UnixMilli(), 10)
	return &NewsItem{
		Title:       "黄金价格（XAU/人民币）",
		URL:         itemURL,
		Source:      "gold",
		Description: "国际现货黄金（XAU）人民币（CNY）实时价格，单位元/克（由元/盎司换算），数据来自免费行情接口，仅供参考。",
		PublishedAt: t,
		HotScore:    pricePerOz,
		RawData:     map[string]any{"price": pricePerOz, "ts": data.TSJ},
	}, nil
}

// fetchGoldFromEastMoney 用东方财富 qt/clist/get 拉取上期所期货列表（与指数/股票同域名，接口不同），从列表中解析沪金主连/沪金合约价格。
// 沪金报价为元/克，转为元/盎司存入 HotScore，与主源一致供前端 toPerGram 展示。
func (g *GoldPriceFetcher) fetchGoldFromEastMoney() ([]NewsItem, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	// fs=m:113 上期所；fields: f2=最新价(分) f3=涨跌幅 f12=代码 f14=名称 f60=昨收
	params := url.Values{
		"fs":     {eastMoneyShfeFS},
		"fields": {"f2,f3,f12,f14,f60"},
		"pn":     {"1"},
		"pz":     {"100"},
	}
	u := eastMoneyClistURL + "?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("fetch gold from East Money clist: %v", err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, err
	}
	// 接口返回的 diff 是对象 {"0":{...},"1":{...}} 不是数组
	var payload struct {
		Data *struct {
			Diff map[string]struct {
				F2  float64 `json:"f2"`  // 最新价（单位：分，需/100 得元/克）
				F3  float64 `json:"f3"`  // 涨跌幅
				F12 string  `json:"f12"` // 代码
				F14 string  `json:"f14"` // 名称
				F60 float64 `json:"f60"` // 昨收
			} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("gold East Money clist decode: %v", err)
		return nil, nil
	}
	if payload.Data == nil || len(payload.Data.Diff) == 0 {
		log.Printf("gold East Money clist: data or diff empty")
		return nil, nil
	}
	// 在列表中找沪金：优先名称含「沪金主连」或「沪金连续」，否则任意「沪金」
	type diffRow struct {
		F2  float64
		F3  float64
		F12 string
		F14 string
		F60 float64
	}
	var chosen *diffRow
	for _, d := range payload.Data.Diff {
		if !strings.Contains(d.F14, "沪金") {
			continue
		}
		row := diffRow{d.F2, d.F3, d.F12, d.F14, d.F60}
		if chosen == nil {
			chosen = &row
		}
		if strings.Contains(d.F14, "主连") || strings.Contains(d.F14, "连续") {
			chosen = &row
			break
		}
	}
	if chosen == nil {
		log.Printf("gold East Money clist: no 沪金 in list")
		return nil, nil
	}
	// 接口 f2 为「分」，除以 100 得元/克（沪金约 500～600 元/克，f2 约 5～6 万）
	pricePerGram := chosen.F2
	if pricePerGram > 10000 {
		pricePerGram = pricePerGram / 100
	}
	if pricePerGram <= 0 {
		return nil, nil
	}
	pricePerOz := pricePerGram * goldOzPerGram
	now := time.Now()
	name := chosen.F14
	if name == "" {
		name = "沪金主连"
	}
	// f3 与股票 f170 一致，为涨跌幅百分比×100
	pct := chosen.F3 / 100
	changeStr := strconv.FormatFloat(pct, 'f', 2, 64)
	itemURL := "https://quote.eastmoney.com/qihuo/au.html?t=" + strconv.FormatInt(now.UnixMilli(), 10)
	log.Printf("gold price fallback: using East Money 沪金 (%s), %.2f 元/克", name, pricePerGram)
	return []NewsItem{{
		Title:       "黄金价格（XAU/人民币）",
		URL:         itemURL,
		Source:      "gold",
		Description: name + " " + strconv.FormatFloat(pricePerGram, 'f', 2, 64) + " 元/克 " + changeStr + "%，数据来自东方财富沪金，仅供参考。",
		PublishedAt: now,
		HotScore:    pricePerOz,
		RawData:     map[string]any{"price": pricePerOz, "ts": now.UnixMilli(), "source": "eastmoney"},
	}}, nil
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
