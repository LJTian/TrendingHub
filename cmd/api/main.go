package main

import (
	"context"
	"log"
	"net/http"
	"path/filepath"
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

	// 启动前同步预取天气，保证首次请求有缓存
	refreshWeather(store)

	// 按数据源更新频率配置独立的采集周期
	jobs := []scheduler.FetcherJob{
		{Fetcher: &collector.BaiduHotFetcher{}, CronSpec: "*/30 * * * *"},
		{Fetcher: &collector.GoldPriceFetcher{}, CronSpec: "*/30 * * * *"},
		{Fetcher: &collector.AShareIndexFetcher{}, CronSpec: "*/30 * * * *"},
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
	if _, err := s.Cron().AddFunc("0 * * * *", func() { refreshWeather(store) }); err != nil {
		log.Printf("warn: add weather cron failed: %v", err)
	}

	// API
	r := gin.Default()
	apiServer := api.NewServer(store)
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

func refreshWeather(store *storage.Store) {
	cities, err := store.ListWeatherCities()
	if err != nil {
		log.Printf("weather: list cities error: %v", err)
		return
	}
	if len(cities) == 0 {
		return
	}
	log.Printf("weather: refreshing %d cities...", len(cities))
	for _, c := range cities {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		data, err := api.FetchWeatherFromWttr(ctx, c.City)
		cancel()
		if err != nil {
			log.Printf("weather: fetch %s error: %v", c.City, err)
			continue
		}
		if err := store.SaveWeatherCache(c.City, string(data)); err != nil {
			log.Printf("weather: cache %s error: %v", c.City, err)
			continue
		}
		log.Printf("weather: cached %s (%d bytes)", c.City, len(data))
	}
	log.Println("weather: refresh done")
}
