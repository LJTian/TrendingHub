package config

import (
	"log"
	"os"
	"time"
)

type Config struct {
	AppPort string

	PostgresDSN string
	RedisAddr   string

	CronSpec string
}

func Load() *Config {
	cfg := &Config{
		AppPort:     getEnv("APP_PORT", "9000"),
		PostgresDSN: getEnv("POSTGRES_DSN", "host=localhost user=trendinghub password=trendinghub dbname=trendinghub port=5432 sslmode=disable TimeZone=UTC"),
		RedisAddr:   getEnv("REDIS_ADDR", "localhost:6380"),
		CronSpec:    getEnv("CRON_SPEC", "*/30 * * * *"),
	}

	log.Printf("config loaded: port=%s cron=%s", cfg.AppPort, cfg.CronSpec)
	return cfg
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Now returns current time, 方便后续做可测试封装
func Now() time.Time {
	return time.Now()
}

