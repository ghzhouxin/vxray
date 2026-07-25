package dto

import (
	"time"

	"v2ray-server/pkg/types"
)

type NodeInfo struct {
	ID             uint      `json:"id"`
	SubscriptionID uint      `json:"subscription_id"`
	Name           string    `json:"name"`
	Protocol       string    `json:"protocol"`
	ProtocolLabel  string    `json:"protocol_label"`
	Address        string    `json:"address"`
	Port           int       `json:"port"`
	RawURL         string    `json:"raw_url"`
	OutboundConfig types.Map `json:"outbound_config"`
	Latency        int64     `json:"latency"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type NodeListResponse struct {
	Items      []NodeInfo `json:"items"`
	NextCursor string     `json:"next_cursor"`
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
