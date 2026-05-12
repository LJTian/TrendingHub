package config

import (
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AppPort string
	WebRoot string // 静态前端目录，非空时由 API 服务 SPA

	PostgresDSN string
	RedisAddr   string

	// QWeather 专属 API Host（形如 https://xxx.qweatherapi.com）
	QWeatherAPIHost string
	// QWeather 的 API KEY（API Key 凭据）
	QWeatherAPIKey string
	// 整站访问的 Basic Auth 账号与密码（为空则不开启）
	BasicAuthUser string
	BasicAuthPass string

	// 各采集频道启用/禁用开关（默认全部启用；伊朗战争成本单独默认关闭）
	EnableBaiduHot       bool
	EnableGoldPrice      bool
	EnableAshare         bool
	EnableHackerNews     bool
	EnableGitHubTrending bool
	EnableProductHunt    bool

	// 各采集频道的 cron 表达式（如需集中调整周期，可修改此处）
	BaiduHotCron       string
	GoldPriceCron      string
	AshareCron         string
	HackerNewsCron     string
	GitHubTrendingCron string
	ProductHuntCron    string

	// Product Hunt 可选 API Token，当前优先使用公开 RSS feed，但保留该配置以便后续切换官方 API
	ProductHuntAPIToken string

	// 是否启用伊朗战争成本频道与抓取逻辑（默认关闭）
	EnableIranWarCost bool
}

func Load() *Config {
	// 1. 先用内置默认值 + 环境变量构建一份基础配置
	cfg := &Config{
		AppPort: getEnv("APP_PORT", "9000"),
		WebRoot: getEnv("WEB_ROOT", ""),

		PostgresDSN: getEnv("POSTGRES_DSN", "host=localhost user=trendinghub password=trendinghub dbname=trendinghub port=5432 sslmode=disable TimeZone=UTC"),
		RedisAddr:   getEnv("REDIS_ADDR", "localhost:6380"),

		QWeatherAPIHost: getEnv("QWEATHER_API_HOST", ""),
		QWeatherAPIKey:  getEnv("QWEATHER_API_KEY", ""),

		BasicAuthUser: getEnv("APP_BASIC_USER", ""),
		BasicAuthPass: getEnv("APP_BASIC_PASS", ""),

		// 采集频道：默认全部开启
		EnableBaiduHot:       true,
		EnableGoldPrice:      true,
		EnableAshare:         true,
		EnableHackerNews:     true,
		EnableGitHubTrending: true,
		EnableProductHunt:    true,

		// 采集周期：集中配置，便于统一调优
		BaiduHotCron:        "*/30 * * * *",
		GoldPriceCron:       "*/30 * * * *",
		AshareCron:          "*/3 * * * *",
		HackerNewsCron:      "0 * * * *",
		GitHubTrendingCron:  "0 */2 * * *",
		ProductHuntCron:     "*/30 * * * *",
		ProductHuntAPIToken: getEnv("PRODUCTHUNT_API_TOKEN", ""),

		// 目前默认关闭伊朗战争成本相关能力，如需启用可在此显式改为 true / 或改为从外部配置加载
		EnableIranWarCost: false,
	}

	// 2. 尝试从 config/config.yaml 读取覆盖业务相关配置（非必须）
	if err := loadFromYAML(cfg); err != nil {
		log.Printf("config: load from YAML failed, use defaults/env only: %v", err)
	}

	log.Printf("config loaded: port=%s", cfg.AppPort)
	return cfg
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// loadFromYAML 尝试从项目内的 config/config.yaml 加载配置覆盖。
// 若文件不存在则直接返回 nil，不视为错误。
func loadFromYAML(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	// 默认从当前工作目录下的 config/config.yaml 读取，便于 Docker / 本地开发统一挂载。
	path := filepath.Join("config", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	// 使用 struct tag 做字段映射，避免直接暴露内部字段名
	type yamlConfig struct {
		// 仅暴露业务常用控制项；基础设施依然主要靠 env（端口/DSN/Redis 等）
		Channels struct {
			BaiduHot       *bool `yaml:"baidu_hot"`
			GoldPrice      *bool `yaml:"gold_price"`
			Ashare         *bool `yaml:"ashare"`
			HackerNews     *bool `yaml:"hackernews"`
			GitHubTrending *bool `yaml:"github_trending"`
			ProductHunt    *bool `yaml:"producthunt"`
			IranWarCost    *bool `yaml:"iran_war_cost"`
		} `yaml:"channels"`

		Cron struct {
			BaiduHot       string `yaml:"baidu_hot"`
			GoldPrice      string `yaml:"gold_price"`
			Ashare         string `yaml:"ashare"`
			HackerNews     string `yaml:"hackernews"`
			GitHubTrending string `yaml:"github_trending"`
			ProductHunt    string `yaml:"producthunt"`
		} `yaml:"cron"`
	}

	var yc yamlConfig
	if err := yaml.Unmarshal(data, &yc); err != nil {
		return err
	}

	// 开关：仅当 YAML 中显式配置时才覆盖默认值
	if yc.Channels.BaiduHot != nil {
		cfg.EnableBaiduHot = *yc.Channels.BaiduHot
	}
	if yc.Channels.GoldPrice != nil {
		cfg.EnableGoldPrice = *yc.Channels.GoldPrice
	}
	if yc.Channels.Ashare != nil {
		cfg.EnableAshare = *yc.Channels.Ashare
	}
	if yc.Channels.HackerNews != nil {
		cfg.EnableHackerNews = *yc.Channels.HackerNews
	}
	if yc.Channels.GitHubTrending != nil {
		cfg.EnableGitHubTrending = *yc.Channels.GitHubTrending
	}
	if yc.Channels.ProductHunt != nil {
		cfg.EnableProductHunt = *yc.Channels.ProductHunt
	}
	if yc.Channels.IranWarCost != nil {
		cfg.EnableIranWarCost = *yc.Channels.IranWarCost
	}

	// Cron：非空字符串才覆盖
	if yc.Cron.BaiduHot != "" {
		cfg.BaiduHotCron = yc.Cron.BaiduHot
	}
	if yc.Cron.GoldPrice != "" {
		cfg.GoldPriceCron = yc.Cron.GoldPrice
	}
	if yc.Cron.Ashare != "" {
		cfg.AshareCron = yc.Cron.Ashare
	}
	if yc.Cron.HackerNews != "" {
		cfg.HackerNewsCron = yc.Cron.HackerNews
	}
	if yc.Cron.GitHubTrending != "" {
		cfg.GitHubTrendingCron = yc.Cron.GitHubTrending
	}
	if yc.Cron.ProductHunt != "" {
		cfg.ProductHuntCron = yc.Cron.ProductHunt
	}

	return nil
}
