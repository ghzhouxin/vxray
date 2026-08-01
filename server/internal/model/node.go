package model

import (
	"time"

	"v2ray-server/pkg/types"
)

type Node struct {
	ID             uint            `json:"id" gorm:"primaryKey"`
	SubscriptionID uint            `json:"subscription_id" gorm:"index"`
	Name           string          `json:"name" gorm:"not null;index"`
	Protocol       string          `json:"protocol" gorm:"not null;index"`
	Address        string          `json:"address" gorm:"not null"`
	Port           int             `json:"port" gorm:"not null"`
	RawURL         string          `json:"raw_url" gorm:"type:text"`
	RawConfig      types.Map       `json:"-" gorm:"type:text"`
	Transport      types.Transport `json:"transport" gorm:"type:text"`
	Latency        int64           `json:"latency" gorm:"index"`
	LatencyRank    int             `json:"-" gorm:"index"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

func (n *Node) IdentityKey() string {
	parsed := &types.ParsedNode{Address: n.Address, Port: n.Port, Protocol: n.Protocol, RawConfig: n.RawConfig}
	return parsed.IdentityKey()
}

type NodeFilter struct {
	SubscriptionID  uint
	Protocol        string
	Keyword         string
	LatencyStatuses []string
	Cursor          string
	Limit           int
	ExcludeID       uint
}
