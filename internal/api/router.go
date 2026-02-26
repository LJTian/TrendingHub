package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/LJTian/TrendingHub/internal/config"
	"github.com/LJTian/TrendingHub/internal/storage"
	"github.com/gin-gonic/gin"
)

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
}

type Server struct {
	store          *storage.Store
	qWeatherHost   string
	qWeatherAPIKey string
}

func NewServer(store *storage.Store, cfg *config.Config) *Server {
	return &Server{
		store:          store,
		qWeatherHost:   cfg.QWeatherAPIHost,
		qWeatherAPIKey: cfg.QWeatherAPIKey,
	}
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

		v1.GET("/ashare/stocks", s.listAshareStocks)
		v1.POST("/ashare/stocks", s.addAshareStock)
		v1.DELETE("/ashare/stocks/:code", s.removeAshareStock)
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
		if s.qWeatherHost == "" || s.qWeatherAPIKey == "" {
			log.Printf("weather: QWeather config missing, skip fetch for %s", city)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		data, err := FetchWeatherFromQWeather(ctx, city, s.qWeatherAPIKey, s.qWeatherHost)
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

// ========== A 股自选股（Web 添加，存数据库） ==========

func (s *Server) listAshareStocks(c *gin.Context) {
	codes := s.store.ListAShareStockCodes()
	c.JSON(http.StatusOK, gin.H{"code": "ok", "data": codes})
}

func (s *Server) addAshareStock(c *gin.Context) {
	var body struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "missing code"})
		return
	}
	normalized := storage.NormalizeStockCode(body.Code)
	if normalized == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "invalid code, need 6-digit stock code"})
		return
	}
	if err := s.store.AddAShareStockCode(normalized); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "ok", "data": normalized, "message": "stock added"})
}

func (s *Server) removeAshareStock(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "missing code"})
		return
	}
	normalized := storage.NormalizeStockCode(code)
	if normalized == "" {
		normalized = strings.TrimSpace(code)
	}
	if err := s.store.RemoveAShareStockCode(normalized); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "ok", "message": "stock removed"})
}

// ======== QWeather 适配：从和风天气获取实况+3日预报，并转换为 wttr.in 的结构 ========

// QWeather 城市查询响应
type qWeatherGeoResponse struct {
	Code     string `json:"code"`
	Location []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Adm2    string `json:"adm2"`
		Adm1    string `json:"adm1"`
		Country string `json:"country"`
		Lat     string `json:"lat"`
		Lon     string `json:"lon"`
	} `json:"location"`
}

// QWeather 实况天气响应
type qWeatherNowResponse struct {
	Code string `json:"code"`
	Now  struct {
		Temp      string `json:"temp"`
		FeelsLike string `json:"feelsLike"`
		Humidity  string `json:"humidity"`
		Text      string `json:"text"`
		Icon      string `json:"icon"`
		WindSpeed string `json:"windSpeed"`
		WindDir   string `json:"windDir"`
		UVIndex   string `json:"uvIndex"`
	} `json:"now"`
}

// QWeather 3 日预报响应
type qWeatherDailyResponse struct {
	Code  string `json:"code"`
	Daily []struct {
		FxDate   string `json:"fxDate"`
		TempMax  string `json:"tempMax"`
		TempMin  string `json:"tempMin"`
		Sunrise  string `json:"sunrise"`
		Sunset   string `json:"sunset"`
		TextDay  string `json:"textDay"`
		IconDay  string `json:"iconDay"`
		WindDirD string `json:"windDirDay"`
		WindDirN string `json:"windDirNight"`
	} `json:"daily"`
}

type wttrCondition struct {
	TempC          string `json:"temp_C"`
	FeelsLikeC     string `json:"FeelsLikeC"`
	Humidity       string `json:"humidity"`
	WeatherDesc    []struct {
		Value string `json:"value"`
	} `json:"weatherDesc"`
	WeatherCode    string `json:"weatherCode"`
	WindspeedKmph  string `json:"windspeedKmph"`
	Winddir16Point string `json:"winddir16Point"`
	UVIndex        string `json:"uvIndex"`
}

type wttrDay struct {
	Date     string `json:"date"`
	MaxtempC string `json:"maxtempC"`
	MintempC string `json:"mintempC"`
	Astronomy []struct {
		Sunrise string `json:"sunrise"`
		Sunset  string `json:"sunset"`
	} `json:"astronomy"`
	Hourly []struct {
		Time        string `json:"time"`
		WeatherCode string `json:"weatherCode"`
		WeatherDesc []struct {
			Value string `json:"value"`
		} `json:"weatherDesc"`
	} `json:"hourly"`
}

type wttrResponse struct {
	CurrentCondition []wttrCondition `json:"current_condition"`
	NearestArea      []struct {
		AreaName []struct {
			Value string `json:"value"`
		} `json:"areaName"`
	} `json:"nearest_area"`
	Weather []wttrDay `json:"weather"`
}

// FetchWeatherFromQWeather 调用和风天气 Geo + Now + 3d 接口，并组装为 wttr.in 兼容结构
func FetchWeatherFromQWeather(ctx context.Context, city, apiKey, apiHost string) ([]byte, error) {
	city = strings.TrimSpace(city)
	if city == "" {
		return nil, fmt.Errorf("empty city")
	}
	if apiKey == "" || apiHost == "" {
		return nil, fmt.Errorf("qweather config missing")
	}

	base := strings.TrimRight(apiHost, "/")
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "https://" + base
	}

	// 1. 城市地理编码：city 名称 -> location ID
	geoURL := fmt.Sprintf("%s/geo/v2/city/lookup?location=%s&lang=zh", base, url.QueryEscape(city))
	geoBody, err := qweatherGetWithRetry(ctx, geoURL, apiKey)
	if err != nil {
		return nil, err
	}
	var geo qWeatherGeoResponse
	if err := json.Unmarshal(geoBody, &geo); err != nil {
		return nil, err
	}
	if geo.Code != "200" || len(geo.Location) == 0 {
		return nil, fmt.Errorf("qweather geoapi code=%s, locations=%d", geo.Code, len(geo.Location))
	}
	loc := geo.Location[0]

	// 2. 实况
	nowURL := fmt.Sprintf("%s/v7/weather/now?location=%s&lang=zh&unit=m", base, url.QueryEscape(loc.ID))
	nowBody, err := qweatherGetWithRetry(ctx, nowURL, apiKey)
	if err != nil {
		return nil, err
	}
	var now qWeatherNowResponse
	if err := json.Unmarshal(nowBody, &now); err != nil {
		return nil, err
	}
	if now.Code != "200" {
		return nil, fmt.Errorf("qweather now code=%s", now.Code)
	}

	// 3. 3 日预报
	dailyURL := fmt.Sprintf("%s/v7/weather/3d?location=%s&lang=zh&unit=m", base, url.QueryEscape(loc.ID))
	dailyBody, err := qweatherGetWithRetry(ctx, dailyURL, apiKey)
	if err != nil {
		return nil, err
	}
	var daily qWeatherDailyResponse
	if err := json.Unmarshal(dailyBody, &daily); err != nil {
		return nil, err
	}
	if daily.Code != "200" {
		return nil, fmt.Errorf("qweather 3d code=%s", daily.Code)
	}

	// 3. 组装为 wttr.in 兼容结构，方便前端复用现有类型和 UI
	resp := wttrResponse{}

	// current_condition
	resp.CurrentCondition = []wttrCondition{
		{
			TempC:      now.Now.Temp,
			FeelsLikeC: now.Now.FeelsLike,
			Humidity:   now.Now.Humidity,
			WeatherDesc: []struct {
				Value string `json:"value"`
			}{
				{Value: now.Now.Text},
			},
			WeatherCode:    now.Now.Icon,
			WindspeedKmph:  now.Now.WindSpeed,
			Winddir16Point: now.Now.WindDir,
			UVIndex:        now.Now.UVIndex,
		},
	}

	// nearest_area（仅用于展示城市名）
	resp.NearestArea = []struct {
		AreaName []struct {
			Value string `json:"value"`
		} `json:"areaName"`
	}{
		{
				AreaName: []struct {
					Value string `json:"value"`
				}{
					{Value: loc.Name},
				},
		},
	}

	// weather（三天预报）
	for _, d := range daily.Daily {
		day := wttrDay{
			Date:     d.FxDate,
			MaxtempC: d.TempMax,
			MintempC: d.TempMin,
		}
		day.Astronomy = []struct {
			Sunrise string `json:"sunrise"`
			Sunset  string `json:"sunset"`
		}{
			{Sunrise: d.Sunrise, Sunset: d.Sunset},
		}
		day.Hourly = []struct {
			Time        string `json:"time"`
			WeatherCode string `json:"weatherCode"`
			WeatherDesc []struct {
				Value string `json:"value"`
			} `json:"weatherDesc"`
		}{
			{
				Time:        "1200",
				WeatherCode: d.IconDay,
				WeatherDesc: []struct {
					Value string `json:"value"`
				}{
					{Value: d.TextDay},
				},
			},
		}
		resp.Weather = append(resp.Weather, day)
	}

	return json.Marshal(resp)
}

// httpGetWithRetry 带简单重试的 GET 请求封装，主要缓解瞬时网络问题。
// qweatherGetWithRetry：带简单重试的 QWeather 请求封装，使用 X-QW-Api-Key 头进行鉴权
func qweatherGetWithRetry(ctx context.Context, fullURL, apiKey string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if ctx.Err() != nil {
			break
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
		if err != nil {
			return nil, err
		}
		if apiKey != "" {
			req.Header.Set("X-QW-Api-Key", apiKey)
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = err
		} else {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				lastErr = fmt.Errorf("qweather status %d: %s", resp.StatusCode, string(body))
			} else if readErr != nil {
				lastErr = readErr
			} else {
				return body, nil
			}
		}
		// 简单指数退避，避免打爆服务；若上下文已取消则立即退出
		if ctx.Err() != nil {
			break
		}
		time.Sleep(time.Duration(attempt+1) * time.Second)
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
