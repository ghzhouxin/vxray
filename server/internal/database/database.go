package database

import (
	"fmt"

	"v2ray-server/internal/constants"
	"v2ray-server/internal/model"

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
	if err := prepareNodeIndexes(db); err != nil {
		return nil, err
	}
	if err := prepareLogIndexes(db); err != nil {
		return nil, err
	}
	return db, nil
}

func prepareNodeIndexes(db *gorm.DB) error {
	const latencyRankExpr = `CASE WHEN latency >= 1 THEN 0 WHEN latency = 0 OR latency IS NULL THEN 1 ELSE 2 END`
	if err := db.Exec(`UPDATE nodes SET latency_rank = ` + latencyRankExpr + ` WHERE latency_rank IS NULL OR latency_rank <> ` + latencyRankExpr).Error; err != nil {
		return fmt.Errorf("update nodes latency_rank: %w", err)
	}
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_nodes_latency_rank_latency_id ON nodes(latency_rank, latency, id)`,
		`CREATE INDEX IF NOT EXISTS idx_nodes_name_id ON nodes(name, id)`,
		`CREATE INDEX IF NOT EXISTS idx_nodes_protocol_name_id ON nodes(protocol, name, id)`,
		`CREATE INDEX IF NOT EXISTS idx_nodes_created_id ON nodes(created_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_nodes_address_port ON nodes(address, port)`,
	}
	for _, stmt := range indexes {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("create node index failed: %s: %w", stmt, err)
		}
	}
	return nil
}

func prepareLogIndexes(db *gorm.DB) error {
	if err := db.Exec(`UPDATE logs SET updated_at = created_at WHERE updated_at IS NULL OR updated_at = '' OR updated_at = '0001-01-01 00:00:00+00:00' OR updated_at = '0001-01-01T00:00:00Z'`).Error; err != nil {
		return fmt.Errorf("backfill logs updated_at: %w", err)
	}
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_logs_updated_id ON logs(updated_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_operation_id ON logs(operation_id)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_tag_updated ON logs(tag, updated_at DESC)`,
	}
	for _, stmt := range indexes {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("create log index failed: %s: %w", stmt, err)
		}
	}
	return nil
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
