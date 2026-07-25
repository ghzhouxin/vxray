package subscription

import (
	"fmt"
	"net/url"

	"v2ray-server/pkg/types"
)

func parseTrojan(nodeURL string) (*types.ParsedNode, error) {
	u, err := url.Parse(nodeURL)
	if err != nil {
		return nil, fmt.Errorf("trojan url parse failed: %w", err)
	}

	password := ""
	if u.User != nil {
		password = u.User.Username()
	}
	if password == "" {
		return nil, fmt.Errorf("trojan: empty password")
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("trojan: empty host")
	}
	port := normalizePort(portFromAny(u.Port()))
	name := defaultString(u.Fragment, host)
	query := u.Query()

	streamSettings := types.Map{}
	network := normalizeNetwork(defaultString(query.Get("type"), NetworkTCP))
	if network == NetworkTCP && query.Get("ws") == "1" {
		network = NetworkWS
	}
	streamSettings["network"] = network
	switch security := query.Get("security"); {
	case security != "" && security != SecurityNone:
		streamSettings["security"] = security
		applySecuritySettings(&streamSettings, security, query)
	case security == "":
		streamSettings["security"] = SecurityTLS // Trojan defaults to TLS
		applySecuritySettings(&streamSettings, SecurityTLS, query)
	}
	applyNetworkSettingsFromValues(&streamSettings, network, query.Get)

	return newParsedNode(
		name,
		types.ProtocolTrojan,
		host,
		port,
		types.Map{"password": password},
		buildServerOutbound(
			types.ProtocolTrojan,
			types.Map{"address": host, "port": port, "password": password},
			streamSettings,
		),
	), nil
}
