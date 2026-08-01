package service

import (
	"v2ray-server/internal/model"
	"v2ray-server/pkg/clash"
)

func GenerateClashConfig(nodes []*model.Node) ([]byte, error) {
	clashNodes := make([]clash.ClashNodeData, len(nodes))
	for i, n := range nodes {
		clashNodes[i] = clash.ClashNodeData{
			Name:        n.Name,
			Protocol:    n.Protocol,
			Address:     n.Address,
			Port:        n.Port,
			RawConfig:   n.RawConfig,
			Transport:   n.Transport,
			IdentityKey: n.IdentityKey(),
		}
	}
	return clash.GenerateConfig(clashNodes)
}
