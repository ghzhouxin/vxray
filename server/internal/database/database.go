package database

import (
	"fmt"

	"v2ray-server/internal/constants"
	"v2ray-server/internal/model"
	"v2ray-server/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Init(path string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("%s?_busy_timeout=%d", path, constants.SQLiteBusyTimeoutMs)), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)

	if err := db.AutoMigrate(&model.Subscription{}, &model.Node{}, &model.Log{}, &model.Setting{}); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}
	if err := repository.PrepareNodeIndexes(db); err != nil {
		return nil, err
	}
	if err := repository.PrepareLogIndexes(db); err != nil {
		return nil, err
	}
	return db, nil
}

func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql db: %w", err)
	}

	if sqlDB == nil {
		return nil
	}

	return sqlDB.Close()
}
