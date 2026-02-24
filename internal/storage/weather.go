package storage

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// WeatherCity 用户关注的城市列表
type WeatherCity struct {
	City      string    `gorm:"primaryKey;size:100" json:"city"`
	CreatedAt time.Time `json:"createdAt"`
}

// WeatherCache 天气缓存表，按城市缓存 wttr.in 的原始 JSON
type WeatherCache struct {
	City      string    `gorm:"primaryKey;size:100" json:"city"`
	Data      string    `gorm:"type:text" json:"data"`
	FetchedAt time.Time `gorm:"index" json:"fetchedAt"`
}

// ---------- 城市管理 ----------

// ListWeatherCities 返回所有关注的城市
func (s *Store) ListWeatherCities() ([]WeatherCity, error) {
	var cities []WeatherCity
	err := s.DB.Order("created_at ASC").Find(&cities).Error
	return cities, err
}

// AddWeatherCity 添加关注城市（已存在则忽略）
func (s *Store) AddWeatherCity(city string) error {
	c := WeatherCity{City: city, CreatedAt: time.Now()}
	return s.DB.Where("city = ?", city).FirstOrCreate(&c).Error
}

// RemoveWeatherCity 移除关注城市及其缓存
func (s *Store) RemoveWeatherCity(city string) error {
	s.DB.Where("city = ?", city).Delete(&WeatherCache{})
	return s.DB.Where("city = ?", city).Delete(&WeatherCity{}).Error
}

// ---------- 天气缓存 ----------

// GetWeatherCache 获取指定城市的天气缓存，不做过期判断
func (s *Store) GetWeatherCache(city string) (string, bool) {
	var cache WeatherCache
	silent := s.DB.Session(&gorm.Session{Logger: s.DB.Logger.LogMode(logger.Silent)})
	err := silent.Where("city = ?", city).First(&cache).Error
	if err != nil {
		return "", false
	}
	return cache.Data, true
}

// GetAllWeatherCache 获取所有关注城市的天气缓存
func (s *Store) GetAllWeatherCache() ([]WeatherCache, error) {
	var list []WeatherCache
	err := s.DB.
		Where("city IN (?)", s.DB.Model(&WeatherCity{}).Select("city")).
		Order("fetched_at DESC").
		Find(&list).Error
	return list, err
}

// SaveWeatherCache 写入或更新指定城市的天气缓存
func (s *Store) SaveWeatherCache(city string, data string) error {
	cache := WeatherCache{
		City:      city,
		Data:      data,
		FetchedAt: time.Now(),
	}
	return s.DB.Save(&cache).Error
}
