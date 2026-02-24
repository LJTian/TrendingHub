package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/LJTian/TrendingHub/internal/storage"
	"github.com/gin-gonic/gin"
)

// 专用 HTTP 客户端：强制 HTTP/1.1 + 15 秒超时，避免 wttr.in 的 HTTP/2 流错误
var wttrClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		TLSNextProto:      make(map[string]func(string, *tls.Conn) http.RoundTripper),
		DialContext:       (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		DisableKeepAlives: true,
	},
}

type Server struct {
	store *storage.Store
}

func NewServer(store *storage.Store) *Server {
	return &Server{store: store}
}

func (s *Server) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", s.health)

	v1 := r.Group("/api/v1")
	{
		v1.GET("/news/dates", s.listNewsDates)
		v1.GET("/news", s.listNews)

		v1.GET("/weather", s.getWeather)
		v1.GET("/weather/cities", s.listWeatherCities)
		v1.POST("/weather/cities", s.addWeatherCity)
		v1.DELETE("/weather/cities/:city", s.removeWeatherCity)
	}
}

func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ========== 天气相关 ==========

// getWeather 返回所有关注城市的天气缓存（只读 DB，不实时请求 wttr.in）
func (s *Server) getWeather(c *gin.Context) {
	list, err := s.store.GetAllWeatherCache()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to read weather cache"})
		return
	}

	type item struct {
		City      string          `json:"city"`
		FetchedAt time.Time       `json:"fetchedAt"`
		Weather   json.RawMessage `json:"weather"`
	}

	items := make([]item, 0, len(list))
	for _, w := range list {
		items = append(items, item{
			City:      w.City,
			FetchedAt: w.FetchedAt,
			Weather:   json.RawMessage(w.Data),
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": "ok", "data": items})
}

// listWeatherCities 返回关注城市列表
func (s *Server) listWeatherCities(c *gin.Context) {
	cities, err := s.store.ListWeatherCities()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	names := make([]string, len(cities))
	for i, c := range cities {
		names[i] = c.City
	}
	c.JSON(http.StatusOK, gin.H{"code": "ok", "data": names})
}

// addWeatherCity 添加关注城市，立即获取并缓存天气
func (s *Server) addWeatherCity(c *gin.Context) {
	var body struct {
		City string `json:"city"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.City == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "missing city"})
		return
	}
	city := body.City
	if len([]rune(city)) > 30 {
		city = string([]rune(city)[:30])
	}

	if err := s.store.AddWeatherCity(city); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}

	// 立即获取天气并缓存，这样前端刷新就能看到
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		data, err := FetchWeatherFromWttr(ctx, city)
		if err != nil {
			log.Printf("weather: fetch %s on add error: %v", city, err)
			return
		}
		_ = s.store.SaveWeatherCache(city, string(data))
		log.Printf("weather: cached %s on add (%d bytes)", city, len(data))
	}()

	c.JSON(http.StatusOK, gin.H{"code": "ok", "message": "city added"})
}

// removeWeatherCity 移除关注城市
func (s *Server) removeWeatherCity(c *gin.Context) {
	city := c.Param("city")
	if city == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "missing city"})
		return
	}
	if err := s.store.RemoveWeatherCity(city); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "ok", "message": "city removed"})
}

// FetchWeatherFromWttr 从 wttr.in 获取指定城市的天气 JSON，带重试
func FetchWeatherFromWttr(ctx context.Context, city string) ([]byte, error) {
	target := fmt.Sprintf("https://wttr.in/%s?format=j1&lang=zh", url.PathEscape(city))
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "TrendingHub/1.0")
		resp, err := wttrClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("wttr.in returned %d", resp.StatusCode)
			continue
		}
		if readErr != nil {
			lastErr = readErr
			continue
		}
		return body, nil
	}
	return nil, lastErr
}

// ========== 新闻相关 ==========

func (s *Server) listNews(c *gin.Context) {
	channel := c.Query("channel")
	sort := c.DefaultQuery("sort", "latest")
	if sort != "latest" && sort != "hot" {
		sort = "latest"
	}
	date := c.Query("date")
	if date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    "bad_request",
				"message": "invalid date format, expected YYYY-MM-DD",
			})
			return
		}
	}

	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}
	maxLimit := 100
	if channel == "gold" {
		maxLimit = 600
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	items, err := s.store.ListNews(channel, sort, limit, date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "internal_error",
			"message": "internal server error",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    "ok",
		"message": "success",
		"data":    items,
	})
}

func (s *Server) listNewsDates(c *gin.Context) {
	channel := c.Query("channel")
	limitStr := c.DefaultQuery("limit", "31")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 31
	}
	if limit > 365 {
		limit = 365
	}

	dates, err := s.store.ListPublishedDates(channel, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "internal_error",
			"message": "internal server error",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    "ok",
		"message": "success",
		"data":    dates,
	})
}
