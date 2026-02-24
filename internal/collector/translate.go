package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

const translateMaxResponseBytes = 256 * 1024

const (
	translateMaxLen        = 500
	translateClientTimeout = 20 * time.Second
)

func isMostlyChinese(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	var cjk, total int
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		total++
		if isCJK(r) {
			cjk++
		}
	}
	if total == 0 {
		return true
	}
	return cjk >= 1 && (cjk*4 >= total || cjk >= 2)
}

func isCJK(r rune) bool {
	if r >= 0x4e00 && r <= 0x9fff {
		return true
	}
	if r >= 0x3400 && r <= 0x4dbf {
		return true
	}
	if r >= 0x3000 && r <= 0x303f {
		return true
	}
	return false
}

func sourceLangForMyMemory(s string) string {
	for _, r := range s {
		if r >= 0x3040 && r <= 0x309f || r >= 0x30a0 && r <= 0x30ff {
			return "ja"
		}
	}
	return "en"
}

// translateToChinese 依次尝试 Google Translate 直接 API → MyMemory，均失败则返回原文
func translateToChinese(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	if rs := []rune(text); len(rs) > translateMaxLen {
		text = string(rs[:translateMaxLen])
	}

	if out := translateViaGoogle(text); out != "" {
		return out
	}

	if out := translateViaMyMemory(text); out != "" {
		return out
	}

	return text
}

// translateViaGoogle 使用 Google Translate 公开 API（client=gtx，无需 TKK/密钥）
func translateViaGoogle(text string) string {
	apiURL := fmt.Sprintf(
		"https://translate.googleapis.com/translate_a/single?client=gtx&sl=auto&tl=zh-CN&dt=t&q=%s",
		url.QueryEscape(text),
	)
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	client := &http.Client{Timeout: translateClientTimeout}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("translate (google-gtx): %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("translate (google-gtx): status %d", resp.StatusCode)
		return ""
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, translateMaxResponseBytes))
	if err != nil {
		return ""
	}

	// 响应格式: [[["翻译文本","原文",...],...],...]
	var raw []any
	if err := json.Unmarshal(body, &raw); err != nil {
		log.Printf("translate (google-gtx): decode error: %v", err)
		return ""
	}

	var result strings.Builder
	outer, ok := raw[0].([]any)
	if !ok {
		return ""
	}
	for _, seg := range outer {
		pair, ok := seg.([]any)
		if !ok || len(pair) < 1 {
			continue
		}
		if s, ok := pair[0].(string); ok {
			result.WriteString(s)
		}
	}

	return strings.TrimSpace(result.String())
}

func translateViaMyMemory(text string) string {
	apiURL := "https://api.mymemory.translated.net/get?langpair=" + sourceLangForMyMemory(text) + "|zh&q=" + url.QueryEscape(text)
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return ""
	}
	client := &http.Client{Timeout: translateClientTimeout}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("translate (mymemory): %v", err)
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("translate (mymemory): status %d", resp.StatusCode)
		return ""
	}
	var out struct {
		ResponseData struct {
			TranslatedText string `json:"translatedText"`
		} `json:"responseData"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, translateMaxResponseBytes)).Decode(&out); err != nil {
		return ""
	}
	return strings.TrimSpace(out.ResponseData.TranslatedText)
}
