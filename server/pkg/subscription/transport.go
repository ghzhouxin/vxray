package subscription

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"v2ray-server/pkg/types"
)

func buildWebSocket(path, host string) *types.WebSocketConfig {
	if path == "" {
		path = "/" // WS/HTTPUpgrade 必须有 path，默认 "/"
	}
	return &types.WebSocketConfig{Path: path, Host: host}
}

func buildGRPC(serviceName, authority string, multiMode bool) *types.GRPCConfig {
	if serviceName == "" && authority == "" && !multiMode {
		return nil
	}
	return &types.GRPCConfig{ServiceName: serviceName, Authority: authority, MultiMode: multiMode}
}

func buildXHTTP(path, host, mode, extra, xPaddingBytes, xPaddingKey string) *types.XHTTPConfig {
	if path == "" {
		path = "/" // XHTTP 必须有 path，默认 "/"
	}
	cfg := &types.XHTTPConfig{Path: path}
	if host != "" {
		cfg.Host = host
	}
	if mode != "" {
		cfg.Mode = mode
	}
	if extra != "" {
		var extraMap types.Map
		if json.Unmarshal([]byte(extra), &extraMap) == nil {
			cfg.Extra = extraMap
		}
	}
	if pb := parseXPaddingBytes(xPaddingBytes); pb != nil {
		cfg.XPaddingBytes = pb
	}
	if xPaddingKey != "" {
		cfg.XPaddingKey = xPaddingKey
	}
	return cfg
}

func buildTCP(headerType, path, host string) *types.TCPConfig {
	if headerType != "http" {
		return nil
	}
	return &types.TCPConfig{HeaderType: headerType, Path: path, Host: host}
}

// mergeEarlyData 把顶层 query 的 ed 参数合并到 path 的 query 中。
// xray-core WebSocketConfig.Build() 从 path 解析 ed（early data），
// 顶层 query 的 ed 会被忽略，导致 early data 失效。
// 若 path 已含 ed 则不覆盖；path 为空时默认 "/" 再合并。
// 参考 xray-core infra/conf/transport_method.go WebSocketConfig.Build()。
func mergeEarlyData(path, ed string) string {
	if ed == "" {
		return path
	}
	if path == "" {
		path = "/"
	}
	if u, err := url.Parse(path); err == nil {
		q := u.Query()
		if q.Get("ed") != "" {
			return path
		}
		q.Set("ed", ed)
		u.RawQuery = q.Encode()
		return u.String()
	}
	if strings.Contains(path, "?") {
		return path + "&ed=" + ed
	}
	return path + "?ed=" + ed
}

// parseXPaddingBytes 解析 x_padding_bytes URL 参数为 xray-core Int32Range。
// 格式 "N-M" → {min:N, max:M}，"N" → {min:N, max:N}，"0"/空/非法 → nil。
// xray-core SplitHTTPConfig.xPaddingBytes <=0 会报错，故 0 跳过。
func parseXPaddingBytes(s string) types.Map {
	if s == "" {
		return nil
	}
	parts := strings.SplitN(s, "-", 2)
	minVal, err := strconv.Atoi(parts[0])
	if err != nil || minVal <= 0 {
		return nil
	}
	maxVal := minVal
	if len(parts) == 2 {
		maxVal, err = strconv.Atoi(parts[1])
		if err != nil || maxVal <= 0 {
			return nil
		}
	}
	return types.Map{"min": minVal, "max": maxVal}
}
