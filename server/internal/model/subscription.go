package model

import "time"

type Subscription struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	Name           string     `json:"name" gorm:"not null"`
	URL            string     `json:"url" gorm:"not null"`
	ContentHash    string     `json:"-"`
	LastSyncAt     *time.Time `json:"last_sync_at"`
	LastSyncStatus string     `json:"last_sync_status"`
	NodeCount      int        `json:"node_count" gorm:"column:node_count"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
