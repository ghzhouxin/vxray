package repository

import (
	"encoding/json"

	"v2ray-server/internal/config"
	"v2ray-server/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 编译时断言 SettingRepository 实现 config.SettingStore。
var _ config.SettingStore = (*SettingRepository)(nil)

type SettingRepository struct{ db *gorm.DB }

func NewSettingRepository(db *gorm.DB) *SettingRepository {
	return &SettingRepository{db: db}
}

func (r *SettingRepository) Get(key string, dest any) error {
	var s model.Setting
	if err := r.db.First(&s, "key = ?", key).Error; err != nil {
		return err
	}
	return json.Unmarshal([]byte(s.Value), dest)
}

func (r *SettingRepository) Set(key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	// 显式 upsert：避免 Save 在新 key 上 UPDATE 0 行的静默失败
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&model.Setting{
		Key: key, Value: string(data),
	}).Error
}
