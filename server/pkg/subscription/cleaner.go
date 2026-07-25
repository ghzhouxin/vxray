package subscription

import (
	"strings"

	"v2ray-server/pkg/types"
	"v2ray-server/pkg/utils"
)

func CleanContent(content string) []string {
	content = strings.ReplaceAll(content, "\x00", "") // strip null bytes from corrupted data files
	content = tryBase64Decode(content)
	urls := extractURLs(content)
	urls = filterValidURLs(urls)
	urls = removeDuplicates(urls)
	return urls
}

func tryBase64Decode(content string) string {
	content = strings.TrimSpace(content)

	if containsNodeURLs(content) {
		return content
	}

	decoded, err := utils.DecodeBase64(content)
	if err != nil {
		return content
	}

	decodedStr := string(decoded)
	if containsNodeURLs(decodedStr) {
		return decodedStr
	}

	return content
}

func containsNodeURLs(content string) bool {
	for _, p := range types.ProtocolPrefixes {
		if strings.Contains(content, p) {
			return true
		}
	}
	return false
}

func extractURLs(content string) []string {
	var urls []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		extracted := extractURLsFromLine(line)
		urls = append(urls, extracted...)
	}

	return urls
}

func extractURLsFromLine(line string) []string {
	var urls []string
	remaining := line

	for len(remaining) > 0 {
		start, end := findNextURLBounds(remaining)
		if start == -1 {
			break
		}
		urls = append(urls, utils.CleanNodeURL(remaining[start:end]))
		remaining = remaining[end:]
	}

	return urls
}

// findNextURLBounds 找出 s 中下一个 URL 的 [start, end) 字节偏移。
// URL 从协议前缀延伸到空白、下一个协议前缀或字符串末尾；# 触发包含剩余部分（fragment 属于 URL）
func findNextURLBounds(s string) (start, end int) {
	start, protocol := findEarliestProtocol(s)
	if start == -1 {
		return -1, -1
	}
	end = start + len(protocol)
	for end < len(s) {
		switch {
		case strings.IndexByte(" \t\n\r", s[end]) != -1:
			return start, end
		case s[end] == '#':
			return start, len(s) // fragment is part of the URL
		case startsWithProtocol(s[end:]):
			return start, end
		}
		end++
	}
	return start, end
}

func findEarliestProtocol(s string) (idx int, protocol string) {
	idx = -1
	for _, p := range types.ProtocolPrefixes {
		if i := strings.Index(s, p); i != -1 && (idx == -1 || i < idx) {
			idx, protocol = i, p
		}
	}
	return idx, protocol
}

func startsWithProtocol(s string) bool {
	for _, p := range types.ProtocolPrefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func filterValidURLs(urls []string) []string {
	var result []string
	for _, url := range urls {
		if strings.HasPrefix(url, types.PrefixSSR) {
			continue // xray-core does not support SSR; skip silently
		}
		if utils.IsValidNodeURL(url) {
			result = append(result, url)
		}
	}
	return result
}

func removeDuplicates(urls []string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, url := range urls {
		if _, exists := seen[url]; !exists {
			seen[url] = struct{}{}
			result = append(result, url)
		}
	}
	return result
}
