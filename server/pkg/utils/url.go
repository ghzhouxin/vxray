package utils

import (
	"strings"

	"v2ray-server/pkg/types"
)

// urlSuffixes 节点 URL 尾部可能追加的协议名残留（如 "vmess://base64==vmess"）
// 排除 "ss"/"ssr" 等短串，避免误匹配 base64 payload 的合法结尾
var urlSuffixes = []string{"vmess", "vless", "trojan", "🔗"}

func CleanNodeURL(url string) string {
	url = strings.TrimSpace(url)

	url = strings.TrimSuffix(url, "```")
	url = strings.TrimSuffix(url, "`")

	if idx := strings.Index(url, "#"); idx != -1 {
		basePart := url[:idx]
		fragmentPart := url[idx:]
		basePart = removeInvalidSuffixes(basePart)
		url = basePart + fragmentPart
	} else {
		url = removeInvalidSuffixes(url)
	}

	return strings.TrimSpace(url)
}

func removeInvalidSuffixes(s string) string {
	for _, suffix := range urlSuffixes {
		s = strings.TrimSuffix(s, suffix)
	}
	return s
}

func IsValidNodeURL(url string) bool {
	for _, prefix := range types.ProtocolPrefixes {
		if strings.HasPrefix(url, prefix) {
			return len(url) > len(prefix)
		}
	}
	return false
}
