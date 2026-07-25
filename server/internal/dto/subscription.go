package dto

import "time"

type SubscriptionCreateRequest struct {
	Name string `json:"name" binding:"required"`
	URL  string `json:"url" binding:"required"`
}

type SubscriptionUpdateRequest struct {
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

type BatchUpdateResult struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
}

type ParseStats struct {
	Total      int  `json:"total"`
	Success    int  `json:"success"`
	Duplicates int  `json:"duplicates"`
	Added      int  `json:"added"`
	Unchanged  bool `json:"unchanged"`
}
