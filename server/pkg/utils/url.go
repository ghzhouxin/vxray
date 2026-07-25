package utils

import (
	"strings"

	"v2ray-server/pkg/types"
)

// urlSuffixes lists protocol-name residue that may appear appended to node URLs
// (e.g. "vmess://base64==vmess"). Short suffixes like "ss"/"ssr" are excluded
// to avoid false-matching valid base64 payloads ending in those characters.
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

// removeInvalidSuffixes strips trailing protocol-name residue (e.g. "vmess://base64==vmess")
// from the base part of a node URL. The fragment (after "#") is excluded by the caller.
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
