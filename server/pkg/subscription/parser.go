package subscription

import (
	"encoding/json"
	"fmt"
	"html"
	"net"
	"net/url"
	"strconv"
	"strings"

	"v2ray-server/pkg/types"
	"v2ray-server/pkg/utils"
)

const (
	NetworkTCP                = "tcp"
	NetworkWS                 = "ws"
	NetworkGRPC               = "grpc"
	NetworkHTTPUpgrade        = "httpupgrade"
	NetworkXHTTP              = "xhttp"
	NetworkRaw                = "raw" // xray-core v25+ alias for tcp
	SecurityTLS               = "tls"
	SecurityReality           = "reality"
	SecurityNone              = "none"
	DefaultPort               = 443
	DefaultSecurity           = "auto"
	DefaultRealityFingerprint = "chrome" // xray-core requires fingerprint for REALITY
	FlowXTLSRprxVision        = "xtls-rprx-vision"
)

func Parse(nodeURL string) (*types.ParsedNode, error) {
	nodeURL = utils.CleanNodeURL(nodeURL)
	nodeURL = strings.ReplaceAll(nodeURL, "&amp%3B", "&") // fix double-encoded & separators
	nodeURL = html.UnescapeString(nodeURL)                // decode &amp; -> &, &lt; -> <, etc.
	nodeURL = encodeHashInQueryValues(nodeURL)            // fix unencoded # in path values (e.g. path=/foo#bar?ed=512&...)
	var node *types.ParsedNode
	var err error
	switch {
	case strings.HasPrefix(nodeURL, types.PrefixVMess):
		node, err = parseVMess(nodeURL)
	case strings.HasPrefix(nodeURL, types.PrefixVLESS):
		node, err = parseVLESS(nodeURL)
	case strings.HasPrefix(nodeURL, types.PrefixTrojan):
		node, err = parseTrojan(nodeURL)
	case strings.HasPrefix(nodeURL, types.PrefixSS):
		node, err = parseSS(nodeURL)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", nodeURL)
	}
	if err != nil {
		return nil, err
	}
	node.RawURL = nodeURL
	return node, nil
}

// encodeHashInQueryValues fixes unencoded # characters in query string values.
// Some subscription URLs have path values containing # (e.g. path=/foo#bar?ed=512&security=tls),
// which causes url.Parse to treat the first # as the fragment delimiter, losing all
// subsequent parameters. This function encodes # as %23 in the query portion, preserving
// only the last # as the real fragment delimiter.
func encodeHashInQueryValues(nodeURL string) string {
	lastHash := strings.LastIndex(nodeURL, "#")
	if lastHash == -1 {
		return nodeURL // no fragment at all
	}

	beforeFragment := nodeURL[:lastHash]
	fragment := nodeURL[lastHash:]

	// Only fix if there are extra # before the real fragment
	if !strings.Contains(beforeFragment, "#") {
		return nodeURL
	}

	// Encode # in the query part (after ?), or in the path part (before ?)
	queryStart := strings.Index(beforeFragment, "?")
	if queryStart == -1 {
		beforeFragment = strings.ReplaceAll(beforeFragment, "#", "%23")
	} else {
		beforeQuery := beforeFragment[:queryStart]
		query := beforeFragment[queryStart:]
		query = strings.ReplaceAll(query, "#", "%23")
		beforeFragment = beforeQuery + query
	}

	return beforeFragment + fragment
}

func defaultString(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// firstNonEmpty 返回第一个非空字符串，用于多别名参数取值（如 pbk/publicKey）。
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// validateFlow 过滤非法 flow 值，xray-core 只允许空、xtls-rprx-vision、xtls-rprx-vision-udp443。
// 参考 xray-core infra/conf/vless.go VLessOutboundConfig.Build()。
func validateFlow(flow string) string {
	switch flow {
	case "", FlowXTLSRprxVision, FlowXTLSRprxVision + "-udp443":
		return flow
	}
	return ""
}

func normalizePort(port int) int {
	if port == 0 {
		return DefaultPort
	}
	return port
}

func portFromAny(v any) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case string:
		result, _ := strconv.Atoi(val)
		return result
	default:
		return 0
	}
}

func newParsedNode(name, protocol, address string, port int, rawConfig, outboundConfig types.Map) *types.ParsedNode {
	if name == "" {
		name = address
	}
	return &types.ParsedNode{
		Name:           name,
		Protocol:       protocol,
		Address:        address,
		Port:           port,
		RawConfig:      rawConfig,
		OutboundConfig: outboundConfig,
	}
}

func buildVNextOutbound(protocol, address string, port int, users []types.Map, streamSettings types.Map) types.Map {
	outbound := types.Map{
		"protocol": protocol,
		"settings": types.Map{
			"vnext": []types.Map{{
				"address": address,
				"port":    port,
				"users":   users,
			}},
		},
	}
	if len(streamSettings) > 0 {
		outbound["streamSettings"] = streamSettings
	}
	return outbound
}

func buildServerOutbound(protocol string, server types.Map, streamSettings types.Map) types.Map {
	outbound := types.Map{
		"protocol": protocol,
		"settings": types.Map{"servers": []types.Map{server}},
	}
	if len(streamSettings) > 0 {
		outbound["streamSettings"] = streamSettings
	}
	return outbound
}

// VMess Parser

func parseVMess(nodeURL string) (*types.ParsedNode, error) {
	encoded := strings.TrimPrefix(nodeURL, types.PrefixVMess)
	if isVMessAEADFormat(encoded) {
		return parseVMessAEAD(encoded)
	}
	return parseVMessJSON(encoded)
}

func isVMessAEADFormat(encoded string) bool {
	// 去掉 fragment（如 #@TelegramChannel）
	if idx := strings.Index(encoded, "#"); idx != -1 {
		encoded = encoded[:idx]
	}
	// AEAD 格式: uuid@host:port?... — @ 紧跟 UUID（36 字符）之后
	// JSON 格式: base64[尾部文本] — 尾部文本可能含 @（如 @LonUp_M），但不以 @ 开头
	// 用 extractBase64 隔离前导 base64 块，若 @ 紧随其后则为 AEAD
	base64Part := extractBase64(encoded)
	return len(base64Part) < len(encoded) && encoded[len(base64Part)] == '@'
}

// extractBase64 returns the leading contiguous base64 characters from s,
// stripping trailing non-base64 text (emojis, URLs, annotations that some
// subscription providers append after the base64 payload).
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
	Port                                                                 any
	Alpn                                                                 any // string or []any (from JSON)
	TLS                                                                  any // string "tls" 或 bool true/false（见 data/nodes.txt 行 7992）
}

func parseVMessJSON(encoded string) (*types.ParsedNode, error) {
	encoded = extractBase64(encoded) // strip trailing non-base64 text (emojis, URLs, etc.)
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
		path = "/" // ws/xhttp/httpupgrade require a path; default to "/"
	}
	headerType := normalizeHeaderType(config.Type)
	applyNetworkSettingsFromValues(&streamSettings, network, func(key string) string {
		switch key {
		case "path", "serviceName": // VMess JSON 用 Path 字段同时表示 ws path 和 grpc serviceName
			return path
		case "host":
			return config.Host
		case "headerType":
			return headerType
		}
		return ""
	})
	return streamSettings
}

// isVMessTLSEnabled 处理 VMess JSON tls 字段的多种类型：
// string "tls" → true；bool true → true；其他（"", false, "none"）→ false。
// 数据样本中存在 "tls": false（见 data/nodes.txt 行 7992），Go string 字段会解析为 ""，
// 用 any 类型 + 类型断言确保正确处理 string 和 bool 两种格式。
func isVMessTLSEnabled(tls any) bool {
	switch v := tls.(type) {
	case string:
		return v == SecurityTLS
	case bool:
		return v
	}
	return false
}

// buildVMessTLSSettings 构造 VMess JSON 的 TLS 配置，补充 vcn/pcs。
// xray-core v26 拒绝 allowInsecure，推荐用 vcn/pcs 替代证书验证。
func buildVMessTLSSettings(config vmessJSONConfig) types.Map {
	tlsSettings := buildTLSSettings(resolveVMessSNI(config), config.Fp, config.Alpn)
	applyTLSCertVerification(tlsSettings, config.Vcn, config.Pcs)
	return tlsSettings
}

// resolveVMessSNI resolves SNI for VMess JSON config: sni → host → add.
// Mirrors resolveSNI(query) used by VLESS/Trojan URL formats, where SNI
// falls back to peer/host when absent. Without this, xray-core receives an
// empty serverName and TLS handshake fails on SNI-checked servers.
func resolveVMessSNI(config vmessJSONConfig) string {
	return firstNonEmpty(config.Sni, config.Host, config.Add)
}

// normalizeHeaderType converts non-standard VMess JSON "type" values
// ("---", "none", "") to "" (no header obfuscation). Only "http" is preserved.
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
	// In AEAD URL format, "encryption" is the cipher (not "security"),
	// while "security" refers to stream security (TLS/Reality). This differs
	// from VMess JSON where "scy"/"security" is the cipher.
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

func normalizeNetwork(network string) string {
	if network == NetworkRaw {
		return NetworkTCP // xray-core v25+ renamed tcp to raw; normalize for internal processing
	}
	return network
}

// applyNetworkSettingsFromValues applies transport settings by network type.
// Not handled (xray-core v26 removed): http/h2/h3 (HTTP transport), quic.
// KCP transport still works but its header & seed are removed by xray-core v26.
func applyNetworkSettingsFromValues(streamSettings *types.Map, network string, get func(key string) string) {
	network = normalizeNetwork(network)
	switch network {
	case NetworkWS:
		path := firstNonEmpty(get("path"), get("wspath")) // wspath 是 legacy 参数
		(*streamSettings)["wsSettings"] = buildWSSettings(path, get("host"))
	case NetworkGRPC:
		authority := firstNonEmpty(get("authority"), get("host")) // 部分 URL 用 host= 作为 gRPC authority
		(*streamSettings)["grpcSettings"] = buildGRPCSettings(get("serviceName"), authority, get("mode"))
	case NetworkXHTTP:
		(*streamSettings)["xhttpSettings"] = buildXHTTPSettingsFromValues(get("path"), get("host"), get("mode"), get("extra"))
	case NetworkHTTPUpgrade:
		// httpupgrade 复用 ws 的 path+host 结构（xray-core 两者字段一致）
		(*streamSettings)["httpupgradeSettings"] = buildWSSettings(get("path"), get("host"))
	case NetworkTCP:
		if get("headerType") == "http" {
			(*streamSettings)["tcpSettings"] = buildTCPSettings(get("path"), get("host"))
		}
	}
}

// VLESS Parser

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

func applySecuritySettings(streamSettings *types.Map, security string, query url.Values) {
	switch security {
	case SecurityTLS:
		(*streamSettings)["tlsSettings"] = buildTLSSettingsFromQuery(query)
	case SecurityReality:
		(*streamSettings)["realitySettings"] = buildRealitySettings(query)
	}
}

// buildStreamSettingsFromQuery 从 URL query 构造 streamSettings，被 VLESS/VMess AEAD 共用。
func buildStreamSettingsFromQuery(query url.Values) types.Map {
	streamSettings := types.Map{}
	if security := query.Get("security"); security != "" && security != SecurityNone {
		streamSettings["security"] = security
		applySecuritySettings(&streamSettings, security, query)
	}
	network := normalizeNetwork(defaultString(query.Get("type"), NetworkTCP))
	streamSettings["network"] = network
	applyNetworkSettingsFromValues(&streamSettings, network, query.Get)
	return streamSettings
}

// Trojan Parser

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
		network = NetworkWS // legacy ws=1 format
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

// Shadowsocks Parser

type ssConfig struct {
	Method, Password, Host, Name, PluginOpts string
	Port                                     int
	StreamSettings                           types.Map
}

func parseSS(nodeURL string) (*types.ParsedNode, error) {
	encoded := strings.TrimPrefix(nodeURL, types.PrefixSS)
	config, err := extractSSConfig(encoded)
	if err != nil {
		return nil, err
	}
	if config.Host == "" || config.Port == 0 {
		return nil, fmt.Errorf("ss parse failed: invalid format (host=%q port=%d)", config.Host, config.Port)
	}
	if config.Method == "" {
		return nil, fmt.Errorf("ss parse failed: empty method")
	}

	outbound := types.Map{
		"address":  config.Host,
		"port":     config.Port,
		"method":   config.Method,
		"password": config.Password,
	}

	return newParsedNode(
		config.Name,
		types.ProtocolShadowsocks,
		config.Host,
		config.Port,
		types.Map{"method": config.Method, "password": config.Password},
		buildServerOutbound(types.ProtocolShadowsocks, outbound, config.StreamSettings),
	), nil
}

func extractSSConfig(encoded string) (*ssConfig, error) {
	config := &ssConfig{StreamSettings: types.Map{}}

	if idx := strings.Index(encoded, "#"); idx != -1 {
		if name, err := url.PathUnescape(encoded[idx+1:]); err == nil {
			config.Name = name
		} else {
			config.Name = encoded[idx+1:]
		}
		encoded = encoded[:idx]
	}
	if idx := strings.Index(encoded, "?"); idx != -1 {
		config.PluginOpts = encoded[idx+1:]
		encoded = encoded[:idx]
	}

	if err := parseSSServerInfo(encoded, config); err != nil {
		return nil, err
	}

	if config.PluginOpts != "" {
		applySSPluginSettings(config)
	}
	return config, nil
}

func parseSSServerInfo(encoded string, config *ssConfig) error {
	// user@server format (plaintext or base64 userinfo) — check encoded string for @
	if strings.Contains(encoded, "@") {
		return parseSSUserServerFormat(encoded, config)
	}
	// Full base64 format: decode first, then parse the decoded string
	decoded, err := utils.DecodeBase64(encoded)
	if err != nil {
		return fmt.Errorf("ss: cannot decode %q: %w", truncate(encoded, 50), err)
	}
	decodedStr := string(decoded)
	if strings.Contains(decodedStr, "@") {
		return parseSSUserServerFormat(decodedStr, config)
	}
	// Decoded but no @ — treat as method:password (no host:port)
	if setMethodPassword(config, decodedStr) {
		return nil
	}
	return fmt.Errorf("ss: invalid format: %q", truncate(decodedStr, 50))
}

// setMethodPassword extracts "method:password" from s into config.
// Returns false if s contains no colon separator.
func setMethodPassword(config *ssConfig, s string) bool {
	if !strings.Contains(s, ":") {
		return false
	}
	mp := strings.SplitN(s, ":", 2)
	config.Method, config.Password = mp[0], mp[1]
	return true
}

func parseSSUserServerFormat(encoded string, config *ssConfig) error {
	parts := strings.SplitN(encoded, "@", 2)
	if len(parts) != 2 {
		return fmt.Errorf("ss: invalid user@server format: %q", truncate(encoded, 50))
	}

	if err := parseSSUserInfo(parts[0], config); err != nil {
		return err
	}

	// Try net.SplitHostPort first (handles IPv6 [::1]:443 and normal host:port)
	if host, portStr, err := net.SplitHostPort(parts[1]); err == nil {
		config.Host = host
		config.Port = parseLeadingDigits(portStr)
		return nil
	}

	// Fallback: handle trailing text in port (e.g. "example.com:443freenettirUnited")
	// For IPv6: [::1]:443text → last : is the port separator (brackets protect the address)
	lastColon := strings.LastIndex(parts[1], ":")
	if lastColon == -1 {
		return fmt.Errorf("ss: missing port in %q", truncate(parts[1], 50))
	}
	config.Host = strings.Trim(parts[1][:lastColon], "[]")
	config.Port = parseLeadingDigits(parts[1][lastColon+1:])
	return nil
}

// parseLeadingDigits extracts leading digits from a port string,
// handling trailing text like "443freenettirUnited" → 443.
func parseLeadingDigits(s string) int {
	if i := strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }); i != -1 {
		s = s[:i]
	}
	port, _ := strconv.Atoi(s)
	return port
}

func parseSSUserInfo(userInfo string, config *ssConfig) error {
	// If userInfo contains ":", treat as plaintext method:password
	if setMethodPassword(config, userInfo) {
		return nil
	}

	// Otherwise try base64 decode
	decoded, err := utils.DecodeBase64(userInfo)
	if err != nil {
		// Some URLs have URL-encoded characters in userinfo (e.g. %2B for +)
		if unescaped, e := url.PathUnescape(userInfo); e == nil && unescaped != userInfo {
			decoded, err = utils.DecodeBase64(unescaped)
		}
		if err != nil {
			return fmt.Errorf("ss: cannot decode userinfo %q: %w", truncate(userInfo, 50), err)
		}
	}

	userStr := string(decoded)
	if setMethodPassword(config, userStr) {
		return nil
	}
	// No ":" means no method — xray-core rejects empty method
	return fmt.Errorf("ss: cannot determine method from %q", truncate(userStr, 50))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// parseSIP003PluginOpts 解析 SIP003 plugin 参数（; 分隔的 key=value 对）。
// 第一个部分是插件名（跳过）。映射 v2ray-plugin 参数到 xray transport 参数：
//
//	mode=websocket → type=ws
//	mode=http      → type=tcp, headerType=http
//	tls（标志）    → security=tls
//	host/path/sni  → 同名
//	mux/skip-cert-verify → 忽略
func parseSIP003PluginOpts(opts string) url.Values {
	// 部分订阅源将 ; 编码为 %3B、= 编码为 %3D，先解码再分割
	if decoded, err := url.PathUnescape(opts); err == nil {
		opts = decoded
	}
	params := url.Values{}
	for i, part := range strings.Split(opts, ";") {
		if i == 0 || part == "" {
			continue // 跳过插件名和空段
		}
		idx := strings.Index(part, "=")
		if idx == -1 {
			if part == "tls" {
				params.Set("security", SecurityTLS)
			}
			continue
		}
		key, value := part[:idx], part[idx+1:]
		switch key {
		case "mode":
			switch value {
			case "websocket":
				params.Set("type", NetworkWS)
			case "http":
				params.Set("type", NetworkTCP)
				params.Set("headerType", "http")
			}
		case "host", "path", "sni":
			params.Set(key, value)
		}
	}
	return params
}

func applySSPluginSettings(config *ssConfig) {
	config.StreamSettings = buildStreamSettingsFromQuery(parseSIP003PluginOpts(config.PluginOpts))
}

// Common Settings Builders

func buildTLSSettings(sni, fp string, alpn any) types.Map {
	tlsSettings := types.Map{}
	if sni != "" {
		tlsSettings["serverName"] = sni
	}
	if fp != "" {
		tlsSettings["fingerprint"] = fp
	}
	if alpnList := normalizeALPN(alpn); len(alpnList) > 0 {
		tlsSettings["alpn"] = alpnList
	}
	return tlsSettings
}

// normalizeALPN handles alpn from query params (string) or VMess JSON (string, []any, or []string).
func normalizeALPN(v any) []string {
	var raw []string
	switch a := v.(type) {
	case string:
		if a == "" {
			return nil
		}
		for _, p := range strings.Split(a, ",") {
			raw = append(raw, strings.TrimSpace(p))
		}
	case []any:
		for _, item := range a {
			if s, ok := item.(string); ok {
				raw = append(raw, s)
			}
		}
	case []string:
		raw = a
	default:
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, s := range raw {
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

// resolveSNI resolves server name with fallback chain: sni → peer → host.
// VMess JSON uses resolveVMessSNI instead (sni → host → add, no peer concept).
func resolveSNI(query url.Values) string {
	return firstNonEmpty(query.Get("sni"), query.Get("peer"), query.Get("host"))
}

// buildTLSSettingsFromQuery builds TLS settings from URL query params.
// Note: allowInsecure is intentionally NOT parsed — xray-core v26 rejects it
// via PrintRemovedFeatureError. Use pinnedPeerCertSha256/verifyPeerCertByName instead.
func buildTLSSettingsFromQuery(query url.Values) types.Map {
	tlsSettings := buildTLSSettings(resolveSNI(query), query.Get("fp"), query.Get("alpn"))
	if ech := query.Get("ech"); ech != "" {
		tlsSettings["echConfigList"] = ech
	}
	applyTLSCertVerification(tlsSettings, query.Get("vcn"), query.Get("pcs"))
	return tlsSettings
}

// applyTLSCertVerification 设置证书固定字段（vcn/pcs）。
// xray-core v26 拒绝 allowInsecure，推荐用 vcn/pcs 替代证书验证。
func applyTLSCertVerification(tlsSettings types.Map, vcn, pcs string) {
	if vcn != "" {
		tlsSettings["verifyPeerCertByName"] = vcn
	}
	if pcs != "" {
		tlsSettings["pinnedPeerCertSha256"] = pcs
	}
}

func buildRealitySettings(query url.Values) types.Map {
	realitySettings := types.Map{}
	if sni := resolveSNI(query); sni != "" {
		realitySettings["serverName"] = sni
	}
	if v := firstNonEmpty(query.Get("publicKey"), query.Get("pbk")); v != "" {
		realitySettings["publicKey"] = v
	}
	if v := firstNonEmpty(query.Get("shortId"), query.Get("sid")); v != "" {
		realitySettings["shortId"] = v
	}
	if v := firstNonEmpty(query.Get("fingerprint"), query.Get("fp")); v != "" {
		realitySettings["fingerprint"] = v
	} else {
		realitySettings["fingerprint"] = DefaultRealityFingerprint
	}
	if v := firstNonEmpty(query.Get("spiderX"), query.Get("spx")); v != "" {
		realitySettings["spiderX"] = v
	}
	return realitySettings
}

func buildWSSettings(path, host string) types.Map {
	settings := types.Map{}
	if path != "" {
		settings["path"] = path
	}
	if host != "" {
		settings["host"] = host
	}
	return settings
}

func buildGRPCSettings(serviceName, authority, mode string) types.Map {
	settings := types.Map{}
	if serviceName != "" {
		settings["serviceName"] = serviceName
	}
	if authority != "" {
		settings["authority"] = authority
	}
	// mode "multi" enables multiMode; "gun"/"auto" use default (multiMode=false)
	if mode == "multi" {
		settings["multiMode"] = true
	}
	return settings
}

func buildXHTTPSettingsFromValues(path, host, mode, extra string) types.Map {
	settings := types.Map{}
	if mode != "" {
		settings["mode"] = mode
	}
	if path != "" {
		settings["path"] = path
	}
	if host != "" {
		settings["host"] = host
	}
	// Parse extra JSON (XHTTP advanced config from v2rayN+ format)
	if extra != "" {
		var extraMap types.Map
		if json.Unmarshal([]byte(extra), &extraMap) == nil {
			settings["extra"] = extraMap // nested sub-object; xray-core overrides extra's Host/Path/Mode with URL params
		}
	}
	return settings
}

func buildTCPSettings(path, host string) types.Map {
	request := types.Map{}
	if path != "" {
		request["path"] = []string{path}
	}
	if host != "" {
		request["headers"] = types.Map{"Host": []string{host}}
	}
	return types.Map{
		"header": types.Map{"type": "http", "request": request},
	}
}
