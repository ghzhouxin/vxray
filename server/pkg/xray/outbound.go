package xray

import (
	"v2ray-server/pkg/types"
)

// BuildOutbound 根据 ParsedNode 的协议和 Transport 构建 xray-core outbound config。
func BuildOutbound(n *types.ParsedNode) types.Map {
	outbound := types.Map{
		"protocol": n.Protocol,
		"settings": buildSettings(n),
	}
	if ss := buildStreamSettings(n.Transport); ss != nil {
		outbound["streamSettings"] = ss
	}
	return outbound
}

func buildSettings(n *types.ParsedNode) types.Map {
	switch n.Protocol {
	case types.ProtocolVMess, types.ProtocolVLESS:
		return buildVNextSettings(n)
	case types.ProtocolTrojan, types.ProtocolShadowsocks:
		return buildServerSettings(n)
	}
	return types.Map{}
}

func buildVNextSettings(n *types.ParsedNode) types.Map {
	user := types.Map{"id": n.RawConfig["uuid"]}
	if n.Protocol == types.ProtocolVMess {
		user["security"] = n.RawConfig["security"]
	} else {
		// VLESS
		user["encryption"] = "none"
		if flow, ok := n.RawConfig["flow"]; ok {
			user["flow"] = flow
		}
		if pe, ok := n.RawConfig["packetEncoding"]; ok {
			user["packetEncoding"] = pe
		}
	}
	return types.Map{
		"vnext": []types.Map{{
			"address": n.Address,
			"port":    n.Port,
			"users":   []types.Map{user},
		}},
	}
}

func buildServerSettings(n *types.ParsedNode) types.Map {
	server := types.Map{"address": n.Address, "port": n.Port}
	if n.Protocol == types.ProtocolTrojan {
		server["password"] = n.RawConfig["password"]
	} else {
		// Shadowsocks
		server["method"] = n.RawConfig["method"]
		server["password"] = n.RawConfig["password"]
	}
	return types.Map{"servers": []types.Map{server}}
}

func buildStreamSettings(t types.Transport) types.Map {
	// 空 Network 意味着 Transport 未初始化，不添加 streamSettings
	// 避免 xray-core 因 {"network":""} 拒绝启动
	if t.Network == "" {
		return nil
	}
	ss := types.Map{"network": t.Network}

	switch t.Security {
	case types.SecurityTLS:
		ss["security"] = types.SecurityTLS
		if t.TLS != nil {
			ss["tlsSettings"] = buildTLSSettings(t.TLS)
		} else {
			// security=tls 但无 TLS 配置：提供空 tlsSettings，让 xray-core 用默认值
			ss["tlsSettings"] = types.Map{}
		}
	case types.SecurityReality:
		ss["security"] = types.SecurityReality
		if t.Reality != nil {
			ss["realitySettings"] = buildRealitySettings(t.Reality)
		}
	}

	fillStreamSettings(ss, t)
	return ss
}

func buildTLSSettings(cfg *types.TLSConfig) types.Map {
	m := types.Map{}
	if cfg.ServerName != "" {
		m["serverName"] = cfg.ServerName
	}
	if cfg.Fingerprint != "" {
		m["fingerprint"] = cfg.Fingerprint
	}
	if len(cfg.ALPN) > 0 {
		m["alpn"] = cfg.ALPN
	}
	if cfg.ECHConfigList != "" {
		m["echConfigList"] = cfg.ECHConfigList
	}
	if cfg.VerifyPeerCertByName != "" {
		m["verifyPeerCertByName"] = cfg.VerifyPeerCertByName
	}
	if cfg.PinnedPeerCertSha256 != "" {
		m["pinnedPeerCertSha256"] = cfg.PinnedPeerCertSha256
	}
	return m
}

func buildRealitySettings(cfg *types.RealityConfig) types.Map {
	m := types.Map{}
	if cfg.ServerName != "" {
		m["serverName"] = cfg.ServerName
	}
	if cfg.PublicKey != "" {
		m["publicKey"] = cfg.PublicKey
	}
	if cfg.ShortID != "" {
		m["shortId"] = cfg.ShortID
	}
	if cfg.Fingerprint != "" {
		m["fingerprint"] = cfg.Fingerprint
	}
	if cfg.SpiderX != "" {
		m["spiderX"] = cfg.SpiderX
	}
	if cfg.Mldsa65Verify != "" {
		m["mldsa65Verify"] = cfg.Mldsa65Verify
	}
	return m
}

func fillStreamSettings(ss types.Map, t types.Transport) {
	switch t.Network {
	case types.NetworkWS, types.NetworkHTTPUpgrade:
		if t.WebSocket != nil {
			ws := types.Map{}
			if t.WebSocket.Path != "" {
				ws["path"] = t.WebSocket.Path
			}
			if t.WebSocket.Host != "" {
				ws["host"] = t.WebSocket.Host // 顶层 host（xray v26 格式）
			}
			if len(ws) > 0 {
				ss["wsSettings"] = ws
			}
		}
	case types.NetworkGRPC:
		if t.GRPC != nil {
			grpc := types.Map{}
			if t.GRPC.ServiceName != "" {
				grpc["serviceName"] = t.GRPC.ServiceName
			}
			if t.GRPC.Authority != "" {
				grpc["authority"] = t.GRPC.Authority
			}
			if t.GRPC.MultiMode {
				grpc["multiMode"] = true
			}
			if len(grpc) > 0 {
				ss["grpcSettings"] = grpc
			}
		}
	case types.NetworkXHTTP:
		if t.XHTTP != nil {
			xh := types.Map{}
			if t.XHTTP.Mode != "" {
				xh["mode"] = t.XHTTP.Mode
			}
			if t.XHTTP.Path != "" {
				xh["path"] = t.XHTTP.Path
			}
			if t.XHTTP.Host != "" {
				xh["host"] = t.XHTTP.Host
			}
			if t.XHTTP.Extra != nil {
				xh["extra"] = t.XHTTP.Extra
			}
			if t.XHTTP.XPaddingBytes != nil {
				xh["xPaddingBytes"] = t.XHTTP.XPaddingBytes
			}
			if t.XHTTP.XPaddingKey != "" {
				xh["xPaddingKey"] = t.XHTTP.XPaddingKey
			}
			if len(xh) > 0 {
				ss["xhttpSettings"] = xh
			}
		}
	case types.NetworkTCP:
		if t.TCP != nil && t.TCP.HeaderType == "http" {
			request := types.Map{}
			if t.TCP.Path != "" {
				request["path"] = []string{t.TCP.Path}
			}
			if t.TCP.Host != "" {
				request["headers"] = types.Map{"Host": []string{t.TCP.Host}}
			}
			ss["tcpSettings"] = types.Map{
				"header": types.Map{"type": "http", "request": request},
			}
		}
	}
}
