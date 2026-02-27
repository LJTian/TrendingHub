package config

import (
	"os"
	"testing"
)

func TestGetEnvWithDefault(t *testing.T) {
	const key = "TEST_APP_PORT"

	// 环境变量未设置时，应该返回默认值
	_ = os.Unsetenv(key)
	if got := getEnv(key, "9000"); got != "9000" {
		t.Fatalf("getEnv(%q) = %q, want %q", key, got, "9000")
	}

	// 环境变量设置后，应优先返回环境变量
	if err := os.Setenv(key, "8080"); err != nil {
		t.Fatalf("Setenv error: %v", err)
	}
	if got := getEnv(key, "9000"); got != "8080" {
		t.Fatalf("getEnv(%q) = %q, want %q", key, got, "8080")
	}
}

func TestLoadReadsAuthAndPorts(t *testing.T) {
	// 使用专用的 env key，避免影响其它测试
	_ = os.Setenv("APP_PORT", "1234")
	_ = os.Setenv("APP_BASIC_USER", "user")
	_ = os.Setenv("APP_BASIC_PASS", "pass")
	defer func() {
		_ = os.Unsetenv("APP_PORT")
		_ = os.Unsetenv("APP_BASIC_USER")
		_ = os.Unsetenv("APP_BASIC_PASS")
	}()

	cfg := Load()
	if cfg.AppPort != "1234" {
		t.Fatalf("AppPort = %q, want %q", cfg.AppPort, "1234")
	}
	if cfg.BasicAuthUser != "user" || cfg.BasicAuthPass != "pass" {
		t.Fatalf("BasicAuthUser/Pass not loaded correctly: %+v", cfg)
	}
}

