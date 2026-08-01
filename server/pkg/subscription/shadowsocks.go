package subscription

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"v2ray-server/pkg/types"
	"v2ray-server/pkg/utils"
)

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
	if strings.Contains(encoded, "@") {
		return parseSSUserServerFormat(encoded, config)
	}
	decoded, err := utils.DecodeBase64(encoded)
	if err != nil {
		return fmt.Errorf("ss: cannot decode %q: %w", truncate(encoded, 50), err)
	}
	if !utf8.Valid(decoded) {
		return fmt.Errorf("ss: content is not valid base64-encoded text: %q", truncate(encoded, 50))
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

	if host, portStr, err := net.SplitHostPort(parts[1]); err == nil {
		config.Host = host
		config.Port = parseLeadingDigits(portStr)
		return nil
	}

	// 兼容尾部带杂质的端口（如 "example.com:443freenettirUnited"）；
	// IPv6 [::1]:443text 的最后一个 : 是端口分隔符（方括号保护地址）
	lastColon := strings.LastIndex(parts[1], ":")
	if lastColon == -1 {
		return fmt.Errorf("ss: missing port in %q", truncate(parts[1], 50))
	}
	config.Host = strings.Trim(parts[1][:lastColon], "[]")
	config.Port = parseLeadingDigits(parts[1][lastColon+1:])
	return nil
}

// parseLeadingDigits 处理端口串尾部杂质（如 "443freenettirUnited" → 443）
func parseLeadingDigits(s string) int {
	if i := strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }); i != -1 {
		s = s[:i]
	}
	port, _ := strconv.Atoi(s)
	return port
}

func parseSSUserInfo(userInfo string, config *ssConfig) error {
	if setMethodPassword(config, userInfo) {
		return nil
	}

	decoded, err := utils.DecodeBase64(userInfo)
	if err != nil {
		// 部分订阅源在 userinfo 中使用 URL 编码（如 %2B 表示 +）
		if unescaped, e := url.PathUnescape(userInfo); e == nil && unescaped != userInfo {
			decoded, err = utils.DecodeBase64(unescaped)
		}
		if err != nil {
			return fmt.Errorf("ss: cannot decode userinfo %q: %w", truncate(userInfo, 50), err)
		}
	}

	// base64 解码可能将非 base64 文本（如 "aes-128-gcm"）解码为二进制乱码。
	// SS method:password 始终是文本，非 UTF-8 说明输入不是 base64 编码的文本。
	if !utf8.Valid(decoded) {
		return fmt.Errorf("ss: userinfo is not valid base64-encoded text: %q", truncate(userInfo, 50))
	}

	userStr := string(decoded)
	if setMethodPassword(config, userStr) {
		return nil
	}
	return fmt.Errorf("ss: cannot determine method from %q", truncate(userStr, 50))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// parseSIP003PluginOpts 解析 SIP003 plugin 参数（; 分隔的 key=value 对，首段为插件名跳过）
// v2ray-plugin 映射：mode=websocket→type=ws、mode=http→type=tcp+headerType=http、tls→security=tls
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
