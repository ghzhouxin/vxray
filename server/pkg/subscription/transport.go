package subscription

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"v2ray-server/pkg/types"
)

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
	if mode == "multi" {
		settings["multiMode"] = true
	}
	return settings
}

func buildXHTTPSettingsFromValues(path, host, mode, extra, xPaddingBytes string) types.Map {
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
	// XHTTP 高级配置（v2rayN+ 格式）：xray-core 用 URL 参数覆盖 extra 的 Host/Path/Mode
	if extra != "" {
		var extraMap types.Map
		if json.Unmarshal([]byte(extra), &extraMap) == nil {
			settings["extra"] = extraMap
		}
	}
	if pb := parseXPaddingBytes(xPaddingBytes); pb != nil {
		settings["xPaddingBytes"] = pb
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

// mergeEarlyData 把顶层 query 的 ed 参数合并到 wsSettings.path 的 query 中。
// xray-core WebSocketConfig.Build() 从 path 解析 ed（early data），
// 顶层 query 的 ed 会被忽略，导致 early data 失效。
// 若 path 已含 ed 则不覆盖。参考 xray-core infra/conf/transport_method.go WebSocketConfig.Build()。
func mergeEarlyData(path, ed string) string {
	if ed == "" || path == "" {
		return path
	}
	if u, err := url.Parse(path); err == nil {
		q := u.Query()
		if q.Get("ed") != "" {
			return path // path 已含 ed，不覆盖
		}
		q.Set("ed", ed)
		u.RawQuery = q.Encode()
		return u.String()
	}
	// url.Parse 失败时简单追加
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
