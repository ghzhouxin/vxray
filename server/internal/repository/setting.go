package repository

import (
	"encoding/json"

	"v2ray-server/internal/model"

	"gorm.io/gorm"
)

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
	return r.db.Save(&model.Setting{
		Key: key, Value: string(data),
	}).Error
}
