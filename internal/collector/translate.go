package collector

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/mind1949/googletrans"
	"golang.org/x/text/language"
)

const translateMaxResponseBytes = 256 * 1024 // 256KB，翻译 API 响应较小

const (
	translateMaxLen        = 500
	translateClientTimeout = 5 * time.Second
)

// isMostlyChinese 判断文本是否主要为汉语（含 CJK 字符比例或存在中文即视为“是”）
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

// sourceLangForMyMemory 根据字符猜测源语言，供 MyMemory 使用（不支持 auto）
func sourceLangForMyMemory(s string) string {
	for _, r := range s {
		if r >= 0x3040 && r <= 0x309f || r >= 0x30a0 && r <= 0x30ff {
			return "ja"
		}
	}
	return "en"
}

// translateToChinese 将非中文介绍翻译成中文：优先 MyMemory（稳定），Google 因 TKK 常失效已跳过，失败则返回原文
func translateToChinese(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	if len(text) > translateMaxLen {
		text = text[:translateMaxLen]
	}

	// 1) 优先 MyMemory，避免因 Google 网页改版导致的 "couldn't found tkk" 拖慢整轮采集
	out := translateViaMyMemory(text)
	if out != "" {
		return out
	}

	// 2) 可选：再试 Google（若库修复可恢复；目前常失败故仅作备用）
	translated, err := googletrans.Translate(googletrans.TranslateParams{
		Src:  "auto",
		Dest: language.SimplifiedChinese.String(),
		Text: text,
	})
	if err == nil && strings.TrimSpace(translated.Text) != "" {
		return strings.TrimSpace(translated.Text)
	}
	if err != nil {
		log.Printf("translate (google): %v", err)
	}

	return text
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
