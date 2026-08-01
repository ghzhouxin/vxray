package dto

type ConsoleSnapshot struct {
	NodeSummary   NodeSummaryDTO    `json:"node_summary"`
	Runtime       ConsoleRuntimeDTO `json:"runtime"`
	Subscriptions []SubscriptionDTO `json:"subscriptions"`
	Protocols     []ProtocolOption  `json:"protocols"`
	Logs          ConsoleLogsDTO    `json:"logs"`
}

type ProtocolOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type NodeSummaryDTO struct {
	All       int64 `json:"all"`
	Available int64 `json:"available"`
	Pending   int64 `json:"pending"`
	Timeout   int64 `json:"timeout"`
}

type ConsoleRuntimeDTO struct {
	Running     bool      `json:"running"`
	Proxy       ProxyDTO  `json:"proxy"`
	Ports       PortsDTO  `json:"ports"`
	CurrentNode *NodeInfo `json:"current_node,omitempty"`
}

type PortsDTO struct {
	HTTP  int `json:"http"`
	SOCKS int `json:"socks"`
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
