package collector

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const maxResponseBodyBytes = 1 << 20 // 1MB，防止超大响应导致 DoS

// AShareIndexFetcher 从新浪财经拉取 A 股主要指数（上证、深证、创业板）
type AShareIndexFetcher struct{}

func (a *AShareIndexFetcher) Name() string {
	return "ashare_index"
}

// 新浪指数接口（HTTPS，多码用逗号分隔）
const ashareIndexURL = "https://hq.sinajs.cn/list=s_sh000001,s_sz399001,s_sz399006"

func (a *AShareIndexFetcher) Fetch() ([]NewsItem, error) {
	log.Println("fetch A-share index...")

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, ashareIndexURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "http://finance.sina.com.cn/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("fetch A-share index failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("fetch A-share index status %d", resp.StatusCode)
		return nil, nil
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		log.Printf("fetch A-share index read body: %v", err)
		return nil, err
	}
	// 新浪接口通常返回 GBK，需解码为 UTF-8
	decoder := transform.NewReader(bytes.NewReader(bodyBytes), simplifiedchinese.GBK.NewDecoder())
	decoded, err := io.ReadAll(decoder)
	if err != nil {
		// 若 GBK 解码失败（如已是 UTF-8），则按原字节解析
		decoded = bodyBytes
	}
	body := string(decoded)
	// 格式: var hq_str_s_sh000001="上证指数,3094.668,-128.073,-3.97,436653,5458126";
	lines := strings.Split(body, ";")
	now := time.Now()
	var results []NewsItem
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 解析变量名中的代码，如 hq_str_s_sh000001 -> s_sh000001
		codeStart := strings.Index(line, "hq_str_")
		if codeStart < 0 {
			continue
		}
		codeStart += len("hq_str_")
		eq := strings.Index(line[codeStart:], "=\"")
		if eq < 0 {
			continue
		}
		code := line[codeStart : codeStart+eq]
		content := strings.TrimSuffix(line[codeStart+eq+2:], "\"")
		parts := strings.Split(content, ",")
		if len(parts) < 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		priceStr := strings.TrimSpace(parts[1])
		if name == "" || priceStr == "" {
			continue
		}
		price, err := strconv.ParseFloat(priceStr, 64)
		if err != nil {
			continue
		}
		changeStr := ""
		if len(parts) >= 4 {
			changeStr = strings.TrimSpace(parts[3]) // 涨跌幅%
		}
		desc := name + " " + priceStr
		if changeStr != "" {
			desc += " " + changeStr + "%"
		}
		item := NewsItem{
			Title:       name,
			URL:         "https://finance.sina.com.cn/realstock/index/" + code + ".html",
			Source:      "ashare",
			Description: desc + "，A 股指数实时行情，数据来自新浪财经，仅供参考。",
			PublishedAt: now,
			HotScore:    price,
			RawData: map[string]any{
				"price":  price,
				"change": changeStr,
			},
		}
		results = append(results, item)
	}
	if len(results) == 0 {
		preview := body
		if len(preview) > 300 {
			preview = preview[:300] + "..."
		}
		log.Printf("fetch A-share index got 0 items, body preview: %s", preview)
	}
	return results, nil
}
