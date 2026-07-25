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
	name := defaultString(u.Fragment, host)
	query := u.Query()

	rawConfig := types.Map{"uuid": uuid}
	if flow := validateFlow(query.Get("flow")); flow != "" {
		rawConfig["flow"] = flow
	}

	streamSettings := buildStreamSettingsFromQuery(query)

	return newParsedNode(
		name,
		types.ProtocolVLESS,
		host,
		port,
		rawConfig,
		buildVNextOutbound(
			types.ProtocolVLESS,
			host,
			port,
			buildVLESSUsers(uuid, query),
			streamSettings,
		),
	), nil
}

func buildVLESSUsers(uuid string, query url.Values) []types.Map {
	user := types.Map{"id": uuid, "encryption": defaultString(query.Get("encryption"), SecurityNone)}
	if flow := validateFlow(query.Get("flow")); flow != "" {
		user["flow"] = flow
	}
	return []types.Map{user}
}
