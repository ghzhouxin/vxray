package subscription

import (
	"fmt"
	"net/url"

	"v2ray-server/pkg/types"
)

func parseVLESS(nodeURL string) (*types.ParsedNode, error) {
	u, err := url.Parse(nodeURL)
	if err != nil {
		return nil, fmt.Errorf("vless url parse failed: %w", err)
	}

	uuid := ""
	if u.User != nil {
		uuid = u.User.Username()
	}
	if uuid == "" {
		return nil, fmt.Errorf("vless: empty uuid")
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("vless: empty host")
	}
	port := normalizePort(portFromAny(u.Port()))
	name := firstNonEmpty(u.Fragment, host)
	query := u.Query()

	rawConfig := types.Map{"uuid": uuid}
	if flow := validateFlow(query.Get("flow")); flow != "" {
		rawConfig["flow"] = flow
	}
	if pe := firstNonEmpty(query.Get("packetEncoding"), query.Get("packet-encoding")); pe != "" {
		rawConfig["packetEncoding"] = pe
	}

	return newParsedNode(
		name,
		types.ProtocolVLESS,
		host,
		port,
		rawConfig,
		buildTransport(query),
	), nil
}

// validateFlow 仅允许 xtls-rprx-vision[-udp443]，xray-core VLessOutboundConfig.Build() 的约束。
func validateFlow(flow string) string {
	switch flow {
	case "", FlowXTLSRprxVision, FlowXTLSRprxVision + "-udp443":
		return flow
	}
	return ""
}
