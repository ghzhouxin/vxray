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
	name := firstNonEmpty(u.Fragment, host)
	query := u.Query()
	transport := buildTransport(query)
	// Trojan 默认 TLS（query 未指定 security 时）
	if transport.Security == "" {
		transport.Security = types.SecurityTLS
	}

	return newParsedNode(
		name,
		types.ProtocolTrojan,
		host,
		port,
		types.Map{"password": password},
		transport,
	), nil
}