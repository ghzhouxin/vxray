package repository

import (
	"fmt"

	"gorm.io/gorm"
)

func PrepareNodeIndexes(db *gorm.DB) error {
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

func PrepareLogIndexes(db *gorm.DB) error {
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_logs_updated_id ON logs(updated_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_tag_level_updated ON logs(tag, level, updated_at DESC)`,
	}
	for _, stmt := range indexes {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("create log index failed: %s: %w", stmt, err)
		}
	}
	return nil
}
