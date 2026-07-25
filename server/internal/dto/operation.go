package dto

type OperationProgress struct {
	Type      string     `json:"type"`
	Status    string     `json:"status"`
	Stage     string     `json:"stage"`
	Total     int        `json:"total"`
	Completed int        `json:"completed"`
	Success   int        `json:"success"`
	Failed    int        `json:"failed"`
	NodeID    uint       `json:"node_id,omitempty"`
	Latency   int64      `json:"latency,omitempty"`
	Error     string     `json:"error,omitempty"`
	Message   string     `json:"message,omitempty"`
	Testing   bool       `json:"testing"`
	Nodes     []NodeInfo `json:"nodes,omitempty"`
}

type NodeSpeedTestStatus struct {
	Running    bool               `json:"running"`
	Progress   *OperationProgress `json:"progress,omitempty"`
	Nodes      []NodeInfo         `json:"nodes,omitempty"`
	StartedAt  string             `json:"started_at,omitempty"`
	FinishedAt string             `json:"finished_at,omitempty"`
	Error      string             `json:"error,omitempty"`
}
