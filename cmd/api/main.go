package main

import (
	"log"

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
	if _, err := store.EnsureChannel("gold", "黄金价格", ""); err != nil {
		log.Fatalf("ensure channel gold failed: %v", err)
	}

	// 注册采集器
	fetchers := []collector.Fetcher{
		&collector.GitHubTrendingMock{},
		&collector.BaiduHotFetcher{},
		&collector.GoldPriceFetcher{},
	}

	// Processor & Scheduler
	p := processor.NewSimpleProcessor()
	s, err := scheduler.New(cfg.CronSpec, fetchers, p, store)
	if err != nil {
		log.Fatalf("init scheduler failed: %v", err)
	}
	s.Start()

	// API
	r := gin.Default()
	apiServer := api.NewServer(store)
	apiServer.RegisterRoutes(r)

	addr := ":" + cfg.AppPort
	log.Printf("starting api server at %s ...", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server exit: %v", err)
	}
}

