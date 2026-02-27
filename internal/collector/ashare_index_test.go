package collector

import (
	"os"
	"testing"
	"time"
)

// helper 返回东八区时间
func mustBeijingTime(t time.Time) time.Time {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*60*60)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc)
}

func TestIsAshareMarketOpenBasicSessions(t *testing.T) {
	// 选取一个工作日（假定是周三），验证时段逻辑；不依赖具体日期的节假日。
	base := mustBeijingTime(time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)) // 2024-01-03 是周三

	openMorning := mustBeijingTime(time.Date(base.Year(), base.Month(), base.Day(), 9, 30, 0, 0, base.Location()))
	if !isAshareMarketOpen(openMorning) {
		t.Fatalf("expected market open at 09:30")
	}

	closeMorning := mustBeijingTime(time.Date(base.Year(), base.Month(), base.Day(), 11, 31, 0, 0, base.Location()))
	if isAshareMarketOpen(closeMorning) {
		t.Fatalf("expected market closed at 11:31")
	}

	openAfternoon := mustBeijingTime(time.Date(base.Year(), base.Month(), base.Day(), 13, 0, 0, 0, base.Location()))
	if !isAshareMarketOpen(openAfternoon) {
		t.Fatalf("expected market open at 13:00")
	}

	closeAfternoon := mustBeijingTime(time.Date(base.Year(), base.Month(), base.Day(), 15, 1, 0, 0, base.Location()))
	if isAshareMarketOpen(closeAfternoon) {
		t.Fatalf("expected market closed at 15:01")
	}

	// 周末必然休市
	sat := mustBeijingTime(time.Date(2024, 1, 6, 10, 0, 0, 0, time.UTC)) // 周六
	if isAshareMarketOpen(sat) {
		t.Fatalf("expected market closed on Saturday")
	}
}

func TestIsAshareTradingWeekday(t *testing.T) {
	// 周三 -> 交易日
	wed := mustBeijingTime(time.Date(2024, 1, 3, 10, 0, 0, 0, time.UTC))
	if !isAshareTradingWeekday(wed) {
		t.Fatalf("expected trading weekday on Wednesday")
	}
	// 周日 -> 非交易日
	sun := mustBeijingTime(time.Date(2024, 1, 7, 10, 0, 0, 0, time.UTC))
	if isAshareTradingWeekday(sun) {
		t.Fatalf("expected non-trading day on Sunday")
	}
}

func TestGetOptionalStockCodesFromEnv(t *testing.T) {
	envKey := "ASHARE_STOCK_CODES"
	defer os.Unsetenv(envKey)

	_ = os.Unsetenv(envKey)
	if codes := getOptionalStockCodes(); len(codes) != 0 {
		t.Fatalf("expected empty codes when env not set, got %v", codes)
	}

	if err := os.Setenv(envKey, "600519, 000858 , ,300750"); err != nil {
		t.Fatalf("Setenv error: %v", err)
	}
	codes := getOptionalStockCodes()
	if len(codes) != 3 {
		t.Fatalf("expected 3 codes, got %d (%v)", len(codes), codes)
	}
	want := []string{"600519", "000858", "300750"}
	for i, c := range want {
		if codes[i] != c {
			t.Fatalf("codes[%d]=%q, want %q", i, codes[i], c)
		}
	}
}

func TestCodeToSecID(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{"600519", "1.600519"},
		{"9XXXX", "1.9XXXX"},
		{"000858", "0.000858"},
		{"300750", "0.300750"},
		{"", ""},
	}

	for _, c := range cases {
		if got := codeToSecID(c.code); got != c.want {
			t.Fatalf("codeToSecID(%q) = %q, want %q", c.code, got, c.want)
		}
	}
}

