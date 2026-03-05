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

// fetchGoldFromEastMoney 复用与 A 股相同的东方财富 push2 qt/stock/get 接口，拉取沪金主连（或 GOLD_EASTMONEY_SECID 指定合约）作为金价备用源。
// 沪金报价一般为元/克，转为元/盎司存入 HotScore，与主源一致供前端 toPerGram 展示。
func (g *GoldPriceFetcher) fetchGoldFromEastMoney() ([]NewsItem, error) {
	secIDs := []string{os.Getenv("GOLD_EASTMONEY_SECID")}
	if secIDs[0] == "" {
		secIDs[0] = eastMoneyGoldSecID
	}
	// 若默认 113.888 返回 data=null，可尝试 116.888（上期所另一编码）
	secIDs = append(secIDs, "116.888")
	client := &http.Client{Timeout: 10 * time.Second}
	for _, secID := range secIDs {
		if secID == "" {
			continue
		}
		item, ok := g.tryEastMoneySecID(client, secID)
		if ok && item != nil {
			return []NewsItem{*item}, nil
		}
	}
	return nil, nil
}

func (g *GoldPriceFetcher) tryEastMoneySecID(client *http.Client, secID string) (*NewsItem, bool) {
	params := url.Values{"secid": {secID}, "fields": {"f43,f58,f60,f170"}}
	u := eastMoneyStockGetURL + "?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("fetch gold from East Money (secid=%s): %v", secID, err)
		return nil, false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, false
	}
	var payload struct {
		Data *struct {
			F43  float64 `json:"f43"`
			F58  string  `json:"f58"`
			F60  float64 `json:"f60"`
			F170 float64 `json:"f170"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("gold East Money (secid=%s): json decode error: %v", secID, err)
		return nil, false
	}
	if payload.Data == nil {
		preview := string(body)
		if len(preview) > 400 {
			preview = preview[:400] + "..."
		}
		log.Printf("gold East Money (secid=%s): data is null, status=%d, body: %s", secID, resp.StatusCode, preview)
		return nil, false
	}
	d := payload.Data
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
	log.Printf("gold price fallback: using East Money 沪金 (secid=%s), %.2f 元/克", secID, pricePerGram)
	return &NewsItem{
		Title:       "黄金价格（XAU/人民币）",
		URL:         itemURL,
		Source:      "gold",
		Description: name + " " + strconv.FormatFloat(pricePerGram, 'f', 2, 64) + " 元/克 " + changeStr + "%，数据来自东方财富沪金，仅供参考。",
		PublishedAt: now,
		HotScore:    pricePerOz,
		RawData:     map[string]any{"price": pricePerOz, "ts": now.UnixMilli(), "source": "eastmoney"},
	}, true
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
