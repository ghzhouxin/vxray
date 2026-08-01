package model

import (
	"time"

	"v2ray-server/pkg/types"
)

type Node struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	SubscriptionID uint      `json:"subscription_id" gorm:"index"`
	Name           string    `json:"name" gorm:"not null;index"`
	Protocol       string    `json:"protocol" gorm:"not null;index"`
	Address        string    `json:"address" gorm:"not null"`
	Port           int       `json:"port" gorm:"not null"`
	RawURL         string    `json:"raw_url" gorm:"type:text"`
	RawConfig      types.Map `json:"-" gorm:"type:text"`
	OutboundConfig types.Map `json:"outbound_config" gorm:"type:text"`
	Latency        int64     `json:"latency" gorm:"index"`
	LatencyRank    int       `json:"-" gorm:"index"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (n *Node) IdentityKey() string {
	parsed := &types.ParsedNode{Address: n.Address, Port: n.Port, Protocol: n.Protocol, RawConfig: n.RawConfig, OutboundConfig: n.OutboundConfig}
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

// Getter methods for clash.ClashNode interface.
// 保留 Get 前缀：Go 不允许导出字段与同名方法共存。
func (n *Node) GetName() string              { return n.Name }
func (n *Node) GetProtocol() string          { return n.Protocol }
func (n *Node) GetAddress() string           { return n.Address }
func (n *Node) GetPort() int                 { return n.Port }
func (n *Node) GetRawConfig() types.Map      { return n.RawConfig }
func (n *Node) GetOutboundConfig() types.Map { return n.OutboundConfig }
