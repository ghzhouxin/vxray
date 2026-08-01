package dto

import "v2ray-server/internal/service"

type ConsoleSnapshot struct {
	NodeSummary   service.NodeSummary      `json:"node_summary"`
	Runtime       ConsoleRuntimeDTO        `json:"runtime"`
	Subscriptions []SubscriptionDTO        `json:"subscriptions"`
	Protocols     []service.ProtocolOption `json:"protocols"`
	Logs          ConsoleLogsDTO           `json:"logs"`
}

type ConsoleRuntimeDTO struct {
	Running     bool              `json:"running"`
	TunEnabled  bool              `json:"tun_enabled"`
	TunState    string            `json:"tun_state"`
	Proxy       ProxyDTO          `json:"proxy"`
	CurrentNode *service.NodeInfo `json:"current_node,omitempty"`
}

type ProxyDTO struct {
	Enabled   bool `json:"enabled"`
	HTTPPort  int  `json:"http_port"`
	SOCKSPort int  `json:"socks_port"`
}

type ConsoleLogsDTO struct {
	Items      []LogDTO `json:"items"`
	Tags       []string `json:"tags"`
	Levels     []string `json:"levels"`
	NextCursor string   `json:"next_cursor"`
	HasMore    bool     `json:"has_more"`
}
