package subscription

import (
	"v2ray-server/pkg/types"
)

// buildVNextOutbound 构造 VMess/VLESS 的 vnext 结构
func buildVNextOutbound(protocol, address string, port int, users []types.Map, streamSettings types.Map) types.Map {
	outbound := types.Map{
		"protocol": protocol,
		"settings": types.Map{
			"vnext": []types.Map{{
				"address": address,
				"port":    port,
				"users":   users,
			}},
		},
	}
	if len(streamSettings) > 0 {
		outbound["streamSettings"] = streamSettings
	}
	return outbound
}

// buildServerOutbound 构造 Trojan/SS 的 servers 结构
func buildServerOutbound(protocol string, server types.Map, streamSettings types.Map) types.Map {
	outbound := types.Map{
		"protocol": protocol,
		"settings": types.Map{"servers": []types.Map{server}},
	}
	if len(streamSettings) > 0 {
		outbound["streamSettings"] = streamSettings
	}
	return outbound
}
