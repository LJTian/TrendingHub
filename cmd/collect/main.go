package main

import (
	"log"

	"github.com/LJTian/TrendingHub/internal/collector"
	"github.com/LJTian/TrendingHub/internal/config"
	"github.com/LJTian/TrendingHub/internal/processor"
	"github.com/LJTian/TrendingHub/internal/scheduler"
	"github.com/LJTian/TrendingHub/internal/storage"
)

// 一个仅执行一次采集任务的命令行入口：适合手动触发采集
func main() {
	cfg := config.Load()

	store, err := storage.NewStore(cfg.PostgresDSN, cfg.RedisAddr)
	if err != nil {
		log.Fatalf("init store failed: %v", err)
	}

	// 确保各个渠道存在（与 cmd/api 保持一致）
	if _, err := store.EnsureChannel("github", "GitHub Trending", "https://github.com/trending"); err != nil {
		log.Fatalf("ensure channel github failed: %v", err)
	}
	if _, err := store.EnsureChannel("baidu", "百度热搜", "https://top.baidu.com/board?tab=realtime"); err != nil {
		log.Fatalf("ensure channel baidu failed: %v", err)
	}
	if _, err := store.EnsureChannel("gold", "金融", ""); err != nil {
		log.Fatalf("ensure channel gold failed: %v", err)
	}

	// 注册采集器
	fetchers := []collector.Fetcher{
		&collector.GitHubTrendingMock{},
		&collector.BaiduHotFetcher{},
		&collector.GoldPriceFetcher{},
		&collector.AShareIndexFetcher{},
	}

	p := processor.NewSimpleProcessor()
	s, err := scheduler.New(cfg.CronSpec, fetchers, p, store)
	if err != nil {
		log.Fatalf("init scheduler failed: %v", err)
	}

	// 只执行一轮采集任务后退出
	s.RunOnce()
}

