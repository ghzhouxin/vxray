package subscription

import (
	"net/url"

	"v2ray-server/pkg/types"
)

// buildTransport 从 URL query 构造 Transport（VLESS/VMess AEAD/Trojan/SS 共用）。
func buildTransport(query url.Values) types.Transport {
	transport := types.Transport{Network: normalizeNetwork(query.Get("type"))}

	switch security := query.Get("security"); security {
	case types.SecurityTLS:
		transport.Security = types.SecurityTLS
		transport.TLS = buildTLS(query)
	case types.SecurityReality:
		transport.Security = types.SecurityReality
		transport.Reality = buildReality(query)
	}

	applyNetworkSettings(&transport, query)
	return transport
}

// applyNetworkSettings 按 network 类型填充 Transport 子结构。
func applyNetworkSettings(transport *types.Transport, query url.Values) {
	switch transport.Network {
	case types.NetworkWS:
		path := mergeEarlyData(firstNonEmpty(query.Get("path"), query.Get("wspath")), query.Get("ed"))
		transport.WebSocket = buildWebSocket(path, query.Get("host"))
	case types.NetworkGRPC:
		authority := firstNonEmpty(query.Get("authority"), query.Get("host"))
		multiMode := query.Get("mode") == "multi"
		transport.GRPC = buildGRPC(query.Get("serviceName"), authority, multiMode)
	case types.NetworkXHTTP:
		transport.XHTTP = buildXHTTP(query.Get("path"), query.Get("host"), query.Get("mode"), query.Get("extra"), query.Get("x_padding_bytes"), query.Get("xPaddingKey"))
	case types.NetworkHTTPUpgrade:
		path := mergeEarlyData(query.Get("path"), query.Get("ed"))
		transport.WebSocket = buildWebSocket(path, query.Get("host"))
	case types.NetworkTCP:
		transport.TCP = buildTCP(query.Get("headerType"), query.Get("path"), query.Get("host"))
	}
}
