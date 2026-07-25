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
	Ps, Add, ID, Scy, Net, Type, Host, Path, Sni, Fp, Security, Vcn, Pcs string
	ServiceName, Authority, Mode, Extra                                  string
	Port                                                                 any
	Alpn                                                                 any // string 或 []any
	TLS                                                                  any // string "tls" 或 bool
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
		buildVNextOutbound(
			types.ProtocolVMess,
			config.Add,
			port,
			[]types.Map{{"id": config.ID, "security": security}},
			buildVMessStreamSettings(config),
		),
	), nil
}

func buildVMessStreamSettings(config vmessJSONConfig) types.Map {
	streamSettings := types.Map{}
	network := normalizeNetwork(config.Net)
	if network != "" {
		streamSettings["network"] = network
	}
	if isVMessTLSEnabled(config.TLS) {
		streamSettings["security"] = SecurityTLS
		streamSettings["tlsSettings"] = buildVMessTLSSettings(config)
	}
	path := config.Path
	if path == "" && (network == NetworkWS || network == NetworkXHTTP || network == NetworkHTTPUpgrade) {
		path = "/"
	}
	headerType := normalizeHeaderType(config.Type)
	applyNetworkSettingsFromValues(&streamSettings, network, func(key string) string {
		switch key {
		case "path":
			return path
		case "serviceName":
			return firstNonEmpty(config.ServiceName, config.Path) // 兼容旧版 v2rayN 用 Path 表示 serviceName
		case "authority":
			return config.Authority
		case "mode":
			return config.Mode
		case "extra":
			return config.Extra
		case "host":
			return config.Host
		case "headerType":
			return headerType
		}
		return ""
	})
	return streamSettings
}

// isVMessTLSEnabled 处理 VMess JSON tls 字段（string "tls" 或 bool）
func isVMessTLSEnabled(tls any) bool {
	switch v := tls.(type) {
	case string:
		return v == SecurityTLS
	case bool:
		return v
	}
	return false
}

// buildVMessTLSSettings 补充 vcn/pcs（xray-core v26 拒绝 allowInsecure）
func buildVMessTLSSettings(config vmessJSONConfig) types.Map {
	tlsSettings := buildTLSSettings(resolveVMessSNI(config), config.Fp, config.Alpn)
	applyTLSCertVerification(tlsSettings, config.Vcn, config.Pcs)
	return tlsSettings
}

// resolveVMessSNI: sni → host → add（无 peer 概念）
func resolveVMessSNI(config vmessJSONConfig) string {
	return firstNonEmpty(config.Sni, config.Host, config.Add)
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
	name := defaultString(u.Fragment, host)
	query := u.Query()
	// AEAD: encryption 是 cipher，security 是 stream security（TLS/Reality），区别于 JSON 的 scy/security
	cipher := defaultString(query.Get("encryption"), DefaultSecurity)

	streamSettings := buildStreamSettingsFromQuery(query)

	return newParsedNode(
		name,
		types.ProtocolVMess,
		host,
		port,
		types.Map{"uuid": uuid, "security": cipher},
		buildVNextOutbound(
			types.ProtocolVMess,
			host,
			port,
			[]types.Map{{"id": uuid, "security": cipher}},
			streamSettings,
		),
	), nil
}
