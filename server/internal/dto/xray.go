package dto

type XrayStatusResponse struct {
	Running     bool      `json:"running"`
	CurrentNode *NodeInfo `json:"current_node,omitempty"`
}

type SaveConfigRequest struct {
	Content string `json:"content"`
}
