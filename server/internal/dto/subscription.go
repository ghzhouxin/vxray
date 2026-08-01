package dto

import "time"

// SubscriptionRequest 同时用于订阅的创建与更新。
type SubscriptionRequest struct {
	Name string `json:"name" binding:"required"`
	URL  string `json:"url" binding:"required"`
}

type SubscriptionBatchUpdateRequest struct {
	IDs []uint `json:"ids"`
}

type SubscriptionDTO struct {
	ID             uint       `json:"id"`
	Name           string     `json:"name"`
	URL            string     `json:"url"`
	LastSyncAt     *time.Time `json:"last_sync_at"`
	LastSyncStatus string     `json:"last_sync_status"`
	NodeCount      int        `json:"node_count"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
