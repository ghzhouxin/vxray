package subscription

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"v2ray-server/pkg/types"
	"v2ray-server/pkg/utils"
)

func parseVMess(nodeURL string) (*types.ParsedNode, error) {
	encoded := strings.TrimPrefix(nodeURL, types.PrefixVMess)
	if isVMessAEADFormat(encoded) {
		return parseVMessAEAD(encoded)
	}
	return parseVMessJSON(encoded)
}

func isVMessAEADFormat(encoded string) bool {
	if idx := strings.Index(encoded, "#"); idx != -1 {
		encoded = encoded[:idx]
	}
	// AEAD: uuid@host:port，JSON: base64[尾部]，用 extractBase64 隔离前导 base64 块后判断 @ 位置
	base64Part := extractBase64(encoded)
	return len(base64Part) < len(encoded) && encoded[len(base64Part)] == '@'
}

// extractBase64 隔离前导 base64 块，剥离订阅源尾部追加的非 base64 文本
func extractBase64(s string) string {
	if i := strings.IndexFunc(s, func(r rune) bool { return !isBase64Rune(r) }); i != -1 {
		return s[:i]
	}
	return s
}

func isBase64Rune(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
		r == '+' || r == '/' || r == '=' || r == '-' || r == '_'
}

type vmessJSONConfig struct {
	Ps          string `json:"ps,omitempty"`
	Add         string `json:"add,omitempty"`
	ID          string `json:"id,omitempty"`
	Scy         string `json:"scy,omitempty"`
	Net         string `json:"net,omitempty"`
	Type        string `json:"type,omitempty"`
	Host        string `json:"host,omitempty"`
	Path        string `json:"path,omitempty"`
	Sni         string `json:"sni,omitempty"`
	Fp          string `json:"fp,omitempty"`
	Security    string `json:"security,omitempty"`
	ServiceName string `json:"servicename,omitempty"`
	Authority   string `json:"authority,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Extra       string `json:"extra,omitempty"`
	Port        any    `json:"port,omitempty"`
	Alpn        any    `json:"alpn,omitempty"` // string 或 []any
	TLS         any    `json:"tls,omitempty"`  // string "tls" 或 bool
	ECH         string `json:"ech,omitempty"`
	VCN         string `json:"vcn,omitempty"`
	PCS         string `json:"pcs,omitempty"`
}

func parseVMessJSON(encoded string) (*types.ParsedNode, error) {
	encoded = extractBase64(encoded)
	decoded, err := utils.DecodeBase64(encoded)
	if err != nil {
		return nil, fmt.Errorf("vmess base64 decode failed: %w", err)
	}

	var config vmessJSONConfig
	if err := json.Unmarshal(decoded, &config); err != nil {
		return nil, fmt.Errorf("vmess json parse failed: %w", err)
	}

	port := normalizePort(portFromAny(config.Port))
	if config.Add == "" {
		return nil, fmt.Errorf("vmess: empty host")
	}
	security := firstNonEmpty(config.Scy, config.Security, DefaultSecurity)

	return newParsedNode(
		config.Ps,
		types.ProtocolVMess,
		config.Add,
		port,
		types.Map{"uuid": config.ID, "security": security},
		buildVMessTransport(config),
	), nil
}

// buildVMessTransport 从 vmess JSON 构造 Transport。
func buildVMessTransport(config vmessJSONConfig) types.Transport {
	transport := types.Transport{Network: normalizeNetwork(config.Net)}

	if isVMessTLSEnabled(config.TLS) {
		transport.Security = types.SecurityTLS
		sni := firstNonEmpty(config.Sni, config.Host, config.Add)
		transport.TLS = &types.TLSConfig{
			ServerName:           sni,
			Fingerprint:          config.Fp,
			ALPN:                 normalizeALPN(config.Alpn),
			ECHConfigList:        config.ECH,
			VerifyPeerCertByName: config.VCN,
			PinnedPeerCertSha256: config.PCS,
		}
	}

	headerType := normalizeHeaderType(config.Type)

	switch transport.Network {
	case types.NetworkWS:
		transport.WebSocket = buildWebSocket(config.Path, config.Host)
	case types.NetworkGRPC:
		serviceName := firstNonEmpty(config.ServiceName, config.Path) // 兼容旧版 v2rayN 用 Path 表示 serviceName
		transport.GRPC = buildGRPC(serviceName, config.Authority, config.Mode == "multi")
	case types.NetworkXHTTP:
		transport.XHTTP = buildXHTTP(config.Path, config.Host, config.Mode, config.Extra, "", "")
	case types.NetworkHTTPUpgrade:
		transport.WebSocket = buildWebSocket(config.Path, config.Host)
	case types.NetworkTCP:
		transport.TCP = buildTCP(headerType, config.Path, config.Host)
	}
	return transport
}

// isVMessTLSEnabled 处理 VMess JSON tls 字段（string "tls" 或 bool）
func isVMessTLSEnabled(tls any) bool {
	switch v := tls.(type) {
	case string:
		return v == types.SecurityTLS
	case bool:
		return v
	}
	return false
}

// normalizeHeaderType 仅保留 "http"，其他值（"---"/"none"/""）转为 ""
func normalizeHeaderType(t string) string {
	if t == "http" {
		return t
	}
	return ""
}

func parseVMessAEAD(encoded string) (*types.ParsedNode, error) {
	u, err := url.Parse(types.PrefixVMess + encoded)
	if err != nil {
		return nil, fmt.Errorf("vmess aead url parse failed: %w", err)
	}

	uuid := ""
	if u.User != nil {
		uuid = u.User.Username()
	}
	if uuid == "" {
		return nil, fmt.Errorf("vmess: empty uuid")
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("vmess: empty host")
	}
	port := normalizePort(portFromAny(u.Port()))
	name := firstNonEmpty(u.Fragment, host)
	query := u.Query()
	// AEAD: encryption 是 cipher，security 是 stream security（TLS/Reality），区别于 JSON 的 scy/security
	cipher := firstNonEmpty(query.Get("encryption"), DefaultSecurity)

	return newParsedNode(
		name,
		types.ProtocolVMess,
		host,
		port,
		types.Map{"uuid": uuid, "security": cipher},
		buildTransport(query),
	), nil
}
