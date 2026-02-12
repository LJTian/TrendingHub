package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type extractRequest struct {
	URL      string `json:"url"`
	MaxChars int    `json:"maxChars"`
}

type extractResponse struct {
	OK    bool   `json:"ok"`
	Text  string `json:"text,omitempty"`
	Error string `json:"error,omitempty"`
}

func main() {
	// 创建浏览器执行器与顶层上下文，整个进程复用一个 headless 实例
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), chromedp.DefaultExecAllocatorOptions[:]...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	// 预热浏览器，避免首个请求耗时过长
	if err := chromedp.Run(browserCtx); err != nil {
		log.Printf("warn: warmup chromedp failed: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/extract", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req extractRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, extractResponse{OK: false, Error: "invalid json"})
			return
		}
		if req.URL == "" {
			writeJSON(w, http.StatusBadRequest, extractResponse{OK: false, Error: "url is required"})
			return
		}
		if req.MaxChars <= 0 || req.MaxChars > 8000 {
			req.MaxChars = 2000
		}

		// 每个请求用独立的超时上下文，复用同一个 browserCtx
		ctx, cancel := context.WithTimeout(browserCtx, 20*time.Second)
		defer cancel()

		var text string
		err := chromedp.Run(ctx,
			chromedp.Navigate(req.URL),
			chromedp.WaitReady("body", chromedp.ByQuery),
			chromedp.Evaluate(extractJS(), &text),
		)
		if err != nil {
			log.Printf("extract error: %v (url=%s)", err, req.URL)
			writeJSON(w, http.StatusOK, extractResponse{OK: false, Error: err.Error()})
			return
		}

		text = trimWhitespace(text)
		if text == "" {
			writeJSON(w, http.StatusOK, extractResponse{OK: false, Error: "empty content"})
			return
		}

		// rune 级截断，避免中文被截断成半个字符
		rs := []rune(text)
		if len(rs) > req.MaxChars {
			text = string(rs[:req.MaxChars]) + "…"
		}

		writeJSON(w, http.StatusOK, extractResponse{OK: true, Text: text})
	})

	addr := ":" + getEnv("PORT", "4000")
	log.Printf("browser-scraper listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("http server error: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// extractJS 返回一段 JS，用于在页面中提取正文文本。
// 会优先在常见正文容器中找段落，找不到时再全页兜底。
func extractJS() string {
	return `(function () {
  function getTextFromSelector(selector) {
    var el = document.querySelector(selector);
    if (!el) return "";
    return el.innerText || "";
  }

  var selectors = [
    "article",
    "div.article-content",
    "div#article-content",
    "div#content",
    "div.main-content",
    "div.content",
    "div.article",
    ".rich_media_content"
  ];

  var text = "";
  for (var i = 0; i < selectors.length; i++) {
    text = getTextFromSelector(selectors[i]).trim();
    if (text && text.length > 200) {
      break;
    }
  }

  if (!text || text.length < 200) {
    // 兜底：遍历全页较长段落
    var nodes = Array.prototype.slice.call(document.querySelectorAll("p, div"));
    var pieces = [];
    for (var j = 0; j < nodes.length; j++) {
      var t = (nodes[j].innerText || "").trim();
      if (t.length >= 40) {
        pieces.push(t);
      }
      if (pieces.join("\\n\\n").length > 4000) break;
    }
    text = pieces.join("\\n\\n");
  }

  return (text || "").replace(/\\s+\\n/g, "\\n").trim();
})();`
}

func trimWhitespace(s string) string {
	// 简单的空白清理，避免过多连续空行
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
}

