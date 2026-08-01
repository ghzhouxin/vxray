package dto

import "v2ray-server/internal/service"

type NodeListResponse struct {
	Items      []service.NodeInfo `json:"items"`
	NextCursor string             `json:"next_cursor"`
}

type NodeActionFilterRequest struct {
	SubscriptionID  uint     `json:"subscription_id"`
	Protocol        string   `json:"protocol"`
	Keyword         string   `json:"keyword"`
	LatencyStatuses []string `json:"latency_statuses"`
}

type NodeSpeedTestRequest struct {
	IDs    []uint                  `json:"ids"`
	Filter NodeActionFilterRequest `json:"filter"`
}
