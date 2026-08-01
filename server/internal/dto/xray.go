package dto

import "v2ray-server/internal/service"

type XrayStatusResponse struct {
	Running     bool              `json:"running"`
	CurrentNode *service.NodeInfo `json:"current_node,omitempty"`
}

type SaveConfigRequest struct {
	Content string `json:"content"`
}
