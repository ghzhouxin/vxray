package service

import (
	"v2ray-server/internal/model"
	"v2ray-server/pkg/clash"
)

func GenerateClashConfig(nodes []*model.Node) ([]byte, error) {
	clashNodes := make([]clash.ClashNode, len(nodes))
	for i, n := range nodes {
		clashNodes[i] = n
	}
	return clash.GenerateConfig(clashNodes)
}
