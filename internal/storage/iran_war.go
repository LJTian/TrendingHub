package storage

import "time"

// IranWarCostSnapshot 记录 Iran War Cost Tracker 的快照，用于缓存和后续折线图展示
type IranWarCostSnapshot struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// 估算总成本（作战 + 离散事件）
	Total float64 `json:"total"`
	// 作战成本累计（按三阶段日成本积分）
	OpsTotal float64 `json:"opsTotal"`
	// 离散事件成本累计（TOTAL DISCRETE COSTS）
	DiscreteTotal float64 `json:"discreteTotal"`
	// 当前阶段日/时/秒成本
	PerSecond float64 `json:"perSecond"`
	PerHour   float64 `json:"perHour"`
	PerDay    float64 `json:"perDay"`
	Phase     string  `gorm:"size:64" json:"phase"`
	// Upstream 抓取时间（通常与 CreatedAt 接近，但保留原始值便于对齐）
	FetchedAt time.Time `gorm:"index" json:"fetchedAt"`
}

// SaveIranWarSnapshot 保存一条成本快照
func (s *Store) SaveIranWarSnapshot(snap *IranWarCostSnapshot) error {
	if snap == nil {
		return nil
	}
	return s.DB.Create(snap).Error
}

// ListIranWarSnapshots 返回最近 N 条时间倒序的成本快照，便于折线图展示
func (s *Store) ListIranWarSnapshots(limit int) ([]IranWarCostSnapshot, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	var list []IranWarCostSnapshot
	if err := s.DB.Order("fetched_at DESC").Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}
