package subscription

import (
	"encoding/json"
	"net/url"

	"v2ray-server/pkg/types"
)

func applySecuritySettings(streamSettings *types.Map, security string, query url.Values) {
	switch security {
	case SecurityTLS:
		(*streamSettings)["tlsSettings"] = buildTLSSettingsFromQuery(query)
	case SecurityReality:
		(*streamSettings)["realitySettings"] = buildRealitySettings(query)
	}
}

// buildStreamSettingsFromQuery 从 URL query 构造 streamSettings（VLESS/VMess AEAD 共用）
func buildStreamSettingsFromQuery(query url.Values) types.Map {
	streamSettings := types.Map{}
	if security := query.Get("security"); security != "" && security != SecurityNone {
		streamSettings["security"] = security
		applySecuritySettings(&streamSettings, security, query)
	}
	network := normalizeNetwork(defaultString(query.Get("type"), NetworkTCP))
	streamSettings["network"] = network
	applyNetworkSettingsFromValues(&streamSettings, network, query.Get)
	if fm := buildFinalMask(query.Get("fm")); fm != nil {
		streamSettings["finalmask"] = fm
	}
	return streamSettings
}

// applyNetworkSettingsFromValues 按 network 类型构造 transport settings
func applyNetworkSettingsFromValues(streamSettings *types.Map, network string, get func(key string) string) {
	network = normalizeNetwork(network)
	switch network {
	case NetworkWS:
		path := firstNonEmpty(get("path"), get("wspath"))
		path = mergeEarlyData(path, get("ed"))
		(*streamSettings)["wsSettings"] = buildWSSettings(path, get("host"))
	case NetworkGRPC:
		authority := firstNonEmpty(get("authority"), get("host"))
		(*streamSettings)["grpcSettings"] = buildGRPCSettings(get("serviceName"), authority, get("mode"))
	case NetworkXHTTP:
		(*streamSettings)["xhttpSettings"] = buildXHTTPSettingsFromValues(get("path"), get("host"), get("mode"), get("extra"), get("x_padding_bytes"))
	case NetworkHTTPUpgrade:
		path := mergeEarlyData(get("path"), get("ed"))
		(*streamSettings)["httpupgradeSettings"] = buildWSSettings(path, get("host"))
	case NetworkTCP:
		if get("headerType") == "http" {
			(*streamSettings)["tcpSettings"] = buildTCPSettings(get("path"), get("host"))
		}
	}
}

// buildFinalMask 解析 fm URL 参数为 xray-core StreamConfig.finalmask。
// fm 是 URL 编码的 JSON 对象（如 {"tcp":[...]}），采用透传策略，不校验内部结构。
// 参考 xray-core infra/conf/transport_finalmask.go。
func buildFinalMask(fm string) types.Map {
	if fm == "" {
		return nil
	}
	var result types.Map
	if err := json.Unmarshal([]byte(fm), &result); err != nil {
		return nil
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
