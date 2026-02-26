package storage

import (
	"strings"
	"time"
)

// AShareStock 自选股：用户通过 Web 添加的 A 股代码，采集时会拉取行情
type AShareStock struct {
	Code      string    `gorm:"primaryKey;size:16" json:"code"`
	CreatedAt time.Time `json:"createdAt"`
}

// ListAShareStockCodes 返回所有自选股代码（按添加顺序）
func (s *Store) ListAShareStockCodes() []string {
	var list []AShareStock
	if err := s.DB.Order("created_at ASC").Find(&list).Error; err != nil {
		return nil
	}
	codes := make([]string, 0, len(list))
	for _, r := range list {
		codes = append(codes, r.Code)
	}
	return codes
}

// AddAShareStockCode 添加自选股（已存在则忽略）
func (s *Store) AddAShareStockCode(code string) error {
	code = NormalizeStockCode(code)
	if code == "" {
		return nil
	}
	r := AShareStock{Code: code, CreatedAt: time.Now()}
	return s.DB.Where("code = ?", code).FirstOrCreate(&r).Error
}

// RemoveAShareStockCode 移除自选股
func (s *Store) RemoveAShareStockCode(code string) error {
	code = NormalizeStockCode(code)
	if code == "" {
		return nil
	}
	return s.DB.Where("code = ?", code).Delete(&AShareStock{}).Error
}

// NormalizeStockCode 规范为 6 位数字代码，供 API 校验使用
func NormalizeStockCode(code string) string {
	code = strings.TrimSpace(code)
	if len(code) == 0 {
		return ""
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			return ""
		}
	}
	if len(code) == 6 {
		return code
	}
	if len(code) < 6 {
		return strings.Repeat("0", 6-len(code)) + code
	}
	return ""
}
