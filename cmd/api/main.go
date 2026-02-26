package main

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/LJTian/TrendingHub/internal/api"
	"github.com/LJTian/TrendingHub/internal/collector"
	"github.com/LJTian/TrendingHub/internal/config"
	"github.com/LJTian/TrendingHub/internal/processor"
	"github.com/LJTian/TrendingHub/internal/scheduler"
	"github.com/LJTian/TrendingHub/internal/storage"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	store, err := storage.NewStore(cfg.PostgresDSN, cfg.RedisAddr)
	if err != nil {
		log.Fatalf("init store failed: %v", err)
	}

	// 确保各个渠道存在
	if _, err := store.EnsureChannel("github", "GitHub Trending", "https://github.com/trending"); err != nil {
		log.Fatalf("ensure channel github failed: %v", err)
	}
	if _, err := store.EnsureChannel("baidu", "百度热搜", "https://top.baidu.com/board?tab=realtime"); err != nil {
		log.Fatalf("ensure channel baidu failed: %v", err)
	}
	if _, err := store.EnsureChannel("gold", "金融", ""); err != nil {
		log.Fatalf("ensure channel gold failed: %v", err)
	}
	if _, err := store.EnsureChannel("hackernews", "Hacker News", "https://news.ycombinator.com"); err != nil {
		log.Fatalf("ensure channel hackernews failed: %v", err)
	}

	// 确保默认城市"北京"存在
	if err := store.AddWeatherCity("北京"); err != nil {
		log.Printf("warn: ensure default weather city: %v", err)
	}

	// 启动时在后台预取天气，不阻塞主流程；首次请求若未命中缓存可稍后刷新
	go refreshWeather(store, cfg.QWeatherAPIKey, cfg.QWeatherAPIHost)

	// 按数据源更新频率配置独立的采集周期；A 股自选股从数据库读取
	jobs := []scheduler.FetcherJob{
		{Fetcher: &collector.BaiduHotFetcher{}, CronSpec: "*/30 * * * *"},
		{Fetcher: &collector.GoldPriceFetcher{}, CronSpec: "*/30 * * * *"},
		// A 股指数 + 自选股：提高频率到每 3 分钟一次，以获得更平滑的分时折线；
		// 收盘后仅在“当天尚无任何 A 股数据”时允许再拉一次，用当前价回填当天快照。
		{
			Fetcher: &collector.AShareIndexFetcher{
				GetStockCodes: func() []string { return store.ListAShareStockCodes() },
				HasTodayData: func(now time.Time) bool {
					// 使用东八区日期与存储层保持一致
					loc := time.FixedZone("CST", 8*60*60)
					date := now.In(loc).Format("2006-01-02")
					return store.HasAshareDataForDate(date)
				},
			},
			CronSpec: "*/3 * * * *",
		},
		{Fetcher: &collector.HackerNewsFetcher{}, CronSpec: "0 * * * *"},
		{Fetcher: &collector.GitHubTrendingMock{}, CronSpec: "0 */2 * * *"},
	}

	p := processor.NewSimpleProcessor()
	s, err := scheduler.New(jobs, p, store)
	if err != nil {
		log.Fatalf("init scheduler failed: %v", err)
	}
	s.Start()

	// 天气定时刷新：每小时从数据库读取城市列表并全量获取
	if _, err := s.Cron().AddFunc("0 * * * *", func() { refreshWeather(store, cfg.QWeatherAPIKey, cfg.QWeatherAPIHost) }); err != nil {
		log.Printf("warn: add weather cron failed: %v", err)
	}

	// API
	r := gin.Default()
	// 若配置了全局访问密码，则启用 Basic Auth 保护（/health 仍然免认证）
	if cfg.BasicAuthUser != "" && cfg.BasicAuthPass != "" {
		r.Use(basicAuthMiddleware(cfg.BasicAuthUser, cfg.BasicAuthPass))
	}

	apiServer := api.NewServer(store, cfg)
	apiServer.RegisterRoutes(r)

	// 若配置了前端目录，则托管 SPA 静态文件并做 fallback
	if cfg.WebRoot != "" {
		assetsDir := filepath.Join(cfg.WebRoot, "assets")
		indexFile := filepath.Join(cfg.WebRoot, "index.html")
		r.Static("/assets", assetsDir)
		r.NoRoute(func(c *gin.Context) {
			if c.Request.Method != http.MethodGet {
				c.Status(http.StatusNotFound)
				return
			}
			// SPA：未匹配 API 的 GET 均返回 index.html
			c.File(indexFile)
		})
	}
	addr := ":" + cfg.AppPort
	log.Printf("starting api server at %s ...", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server exit: %v", err)
	}
}

func refreshWeather(store *storage.Store, apiKey, apiHost string) {
	if apiKey == "" || apiHost == "" {
		log.Printf("weather: skip refresh, QWeather not configured")
		return
	}
	cities, err := store.ListWeatherCities()
	if err != nil {
		log.Printf("weather: list cities error: %v", err)
		return
	}
	if len(cities) == 0 {
		return
	}
	log.Printf("weather: refreshing %d cities...", len(cities))
	var wg sync.WaitGroup
	for _, c := range cities {
		city := c.City
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			data, err := api.FetchWeatherFromQWeather(ctx, city, apiKey, apiHost)
			if err != nil {
				log.Printf("weather: fetch %s error: %v", city, err)
				return
			}
			if err := store.SaveWeatherCache(city, string(data)); err != nil {
				log.Printf("weather: cache %s error: %v", city, err)
				return
			}
			log.Printf("weather: cached %s (%d bytes)", city, len(data))
		}()
	}
	wg.Wait()
	log.Println("weather: refresh done")
}

// basicAuthMiddleware 为整个站点增加一个简单的 Basic Auth 访问密码。
// 仅当配置了 APP_BASIC_USER / APP_BASIC_PASS 时启用。
// /health 不做认证，便于健康检查。
func basicAuthMiddleware(user, pass string) gin.HandlerFunc {
	const realm = "Restricted"
	uBytes := []byte(user)
	pBytes := []byte(pass)

	return func(c *gin.Context) {
		if c.Request.URL.Path == "/health" {
			c.Next()
			return
		}
		u, p, ok := c.Request.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(u), uBytes) != 1 ||
			subtle.ConstantTimeCompare([]byte(p), pBytes) != 1 {
			c.Header("WWW-Authenticate", `Basic realm="`+realm+`"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	}
}
