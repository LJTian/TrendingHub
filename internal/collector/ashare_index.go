package collector

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AShareIndexFetcher 从东方财富拉取三大指数（置顶）+ 自选股。自选股来源：GetStockCodes 若非空则用其返回值（如从 DB 读），否则用环境变量 ASHARE_STOCK_CODES
type AShareIndexFetcher struct {
	// GetStockCodes 返回自选股代码列表，由调用方注入（如从 Store.ListAShareStockCodes）
	GetStockCodes func() []string
}

func (a *AShareIndexFetcher) Name() string {
	return "ashare_index"
}

const eastMoneyStockGetURL = "https://push2.eastmoney.com/api/qt/stock/get"

// 三大指数：上证 1.000001，深证成指 0.399001，创业板指 0.399006
var indexSecIDs = []struct {
	SecID string
	Name  string
}{
	{"1.000001", "上证指数"},
	{"0.399001", "深证成指"},
	{"0.399006", "创业板指"},
}

func (a *AShareIndexFetcher) Fetch() ([]NewsItem, error) {
	log.Println("fetch A-share (East Money)...")

	// 1. 三大指数置顶
	results := a.fetchIndices()
	// 2. 自选股：优先从 GetStockCodes（如 DB）取，否则从环境变量取；并行请求
	var codes []string
	if a.GetStockCodes != nil {
		codes = a.GetStockCodes()
	}
	if len(codes) == 0 {
		codes = getOptionalStockCodes()
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, code := range codes {
		code := code
		wg.Add(1)
		go func() {
			defer wg.Done()
			item := a.fetchOneStock(code)
			if item != nil {
				mu.Lock()
				results = append(results, *item)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return results, nil
}

func getOptionalStockCodes() []string {
	raw := os.Getenv("ASHARE_STOCK_CODES")
	if raw == "" {
		return nil
	}
	var out []string
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// code 为 6 位股票代码，如 600519。返回东方财富 secid：沪 1.xxxxxx，深 0.xxxxxx
func codeToSecID(code string) string {
	if len(code) < 1 {
		return ""
	}
	switch code[0] {
	case '6', '9':
		return "1." + code
	default:
		return "0." + code
	}
}

func (a *AShareIndexFetcher) fetchOneStock(code string) *NewsItem {
	if code == "" {
		return nil
	}
	secID := codeToSecID(code)
	if secID == "" {
		return nil
	}
	client := &http.Client{Timeout: 10 * time.Second}
	// f43: 最新价（分），f170: 涨跌幅（百分比 * 100），f58: 名称
	params := url.Values{"secid": {secID}, "fields": {"f43,f58,f170"}}
	u := eastMoneyStockGetURL + "?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("fetch A-share stock %s: %v", code, err)
		return nil
	}
	body, err := readLimit(resp.Body, 256*1024)
	resp.Body.Close()
	if err != nil {
		return nil
	}
	var payload struct {
		Data *struct {
			F43  float64 `json:"f43"`  // 最新价（分）
			F58  string  `json:"f58"`  // 名称
			F170 float64 `json:"f170"` // 涨跌幅（百分比 * 100）
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.Data == nil || payload.Data.F58 == "" {
		return nil
	}
	d := payload.Data
	itemURL := "https://quote.eastmoney.com/sz" + code + ".html"
	if len(code) >= 1 && (code[0] == '6' || code[0] == '9') {
		itemURL = "https://quote.eastmoney.com/sh" + code + ".html"
	}
	// 东方财富接口个股 f43 单位为分，除以 100 得到元；f170 为涨跌幅（百分比 * 100）
	price := d.F43 / 100
	pct := d.F170 / 100
	changeStr := strconv.FormatFloat(pct, 'f', 2, 64)
	desc := d.F58 + " " + strconv.FormatFloat(price, 'f', 2, 64) + " " + changeStr + "%"
	now := time.Now()
	// 每次采集使用带时间戳的 URL，使存储层插入新行而非更新同一条，从而保留历史用于分时图
	itemURL = itemURL + "?t=" + strconv.FormatInt(now.UnixMilli(), 10)
	return &NewsItem{
		Title:       d.F58,
		URL:         itemURL,
		Source:      "ashare",
		Description: desc + "，数据来自东方财富，仅供参考。",
		PublishedAt: now,
		HotScore:    price,
		RawData: map[string]any{
			"price":  price,
			"change": changeStr,
		},
	}
}

// fetchIndices 拉取三大指数（东方财富 qt/stock/get），并行请求
func (a *AShareIndexFetcher) fetchIndices() []NewsItem {
	now := time.Now()
	results := make([]*NewsItem, len(indexSecIDs))
	var wg sync.WaitGroup
	for i := range indexSecIDs {
		i := i
		secID := indexSecIDs[i].SecID
		name := indexSecIDs[i].Name
		wg.Add(1)
		go func() {
			defer wg.Done()
			item := a.fetchOneIndex(secID, name, now)
			if item != nil {
				results[i] = item
			}
		}()
	}
	wg.Wait()
	out := make([]NewsItem, 0, len(indexSecIDs))
	for _, p := range results {
		if p != nil {
			out = append(out, *p)
		}
	}
	return out
}

func (a *AShareIndexFetcher) fetchOneIndex(secID, indexName string, now time.Time) *NewsItem {
	client := &http.Client{Timeout: 10 * time.Second}
	// f43: 最新点位（×100），f170: 涨跌幅（百分比 * 100），f58: 名称
	params := url.Values{"secid": {secID}, "fields": {"f43,f58,f170"}}
	u := eastMoneyStockGetURL + "?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("fetch index %s: %v", indexName, err)
		return nil
	}
	body, err := readLimit(resp.Body, 256*1024)
	resp.Body.Close()
	if err != nil {
		log.Printf("fetch index %s read: %v", indexName, err)
		return nil
	}
	var payload struct {
		Data *struct {
			F43  float64 `json:"f43"`  // 最新价（×100）
			F58  string  `json:"f58"`  // 名称
			F170 float64 `json:"f170"` // 涨跌幅（百分比 * 100）
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.Data == nil {
		return nil
	}
	d := payload.Data
	name := d.F58
	if name == "" {
		name = indexName
	}
	itemURL := "https://quote.eastmoney.com/sh000001.html"
	if secID == "0.399001" {
		itemURL = "https://quote.eastmoney.com/sz399001.html"
	} else if secID == "0.399006" {
		itemURL = "https://quote.eastmoney.com/sz399006.html"
	}
	// 东方财富接口对指数返回的 f43 为实际点位的 100 倍，需除以 100；f170 为涨跌幅（百分比 * 100）
	price := d.F43 / 100
	pct := d.F170 / 100
	changeStr := strconv.FormatFloat(pct, 'f', 2, 64)
	desc := name + " " + strconv.FormatFloat(price, 'f', 2, 64) + " " + changeStr + "%"
	// 每次采集使用带时间戳的 URL，使存储层插入新行而非更新同一条，从而保留历史用于分时图
	itemURL = itemURL + "?t=" + strconv.FormatInt(now.UnixMilli(), 10)
	return &NewsItem{
		Title:       name,
		URL:         itemURL,
		Source:      "ashare",
		Description: desc + "，数据来自东方财富，仅供参考。",
		PublishedAt: now,
		HotScore:    price,
		RawData: map[string]any{
			"price":  price,
			"change": changeStr,
		},
	}
}

func readLimit(r io.Reader, n int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, n))
}
