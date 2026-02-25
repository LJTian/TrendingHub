package config

import (
	"log"
	"os"
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
}

func Load() *Config {
	cfg := &Config{
		AppPort:         getEnv("APP_PORT", "9000"),
		WebRoot:         getEnv("WEB_ROOT", ""),
		PostgresDSN:     getEnv("POSTGRES_DSN", "host=localhost user=trendinghub password=trendinghub dbname=trendinghub port=5432 sslmode=disable TimeZone=UTC"),
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6380"),
		QWeatherAPIHost: getEnv("QWEATHER_API_HOST", ""),
		QWeatherAPIKey:  getEnv("QWEATHER_API_KEY", ""),
		BasicAuthUser:   getEnv("APP_BASIC_USER", ""),
		BasicAuthPass:   getEnv("APP_BASIC_PASS", ""),
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
