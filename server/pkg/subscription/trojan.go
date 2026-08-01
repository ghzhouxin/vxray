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

	// 兼容 legacy ws=1 标志：覆盖 type 缺失或 raw 时的传输类型
	if query.Get("ws") == "1" {
		if t := query.Get("type"); t == "" || t == types.NetworkRaw {
			query.Set("type", types.NetworkWS)
		}
	}
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
