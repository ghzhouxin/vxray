package subscription

import (
	"fmt"
	"strings"
)

// encodeHashInQueryValues 把最后一个 # 之前的所有 # 编码为 %23。
// 订阅源 password/path/query 可能含 #（如 trojan://pass#word@host:port?...#name），
// url.Parse 会把第一个 # 当作 fragment 分隔符，导致丢失 host 和后续参数。
func encodeHashInQueryValues(nodeURL string) string {
	lastHash := strings.LastIndex(nodeURL, "#")
	if lastHash == -1 {
		return nodeURL
	}

	beforeFragment := nodeURL[:lastHash]
	fragment := nodeURL[lastHash:]

	if !strings.Contains(beforeFragment, "#") {
		return nodeURL
	}

	// beforeFragment 中的所有 # 都应编码（userinfo/path/query 都不允许 #）
	beforeFragment = strings.ReplaceAll(beforeFragment, "#", "%23")
	return beforeFragment + fragment
}

// fixStrayPercent 把不在 %XX 序列中的裸 % 编码为 %25。
// 订阅源 fragment 可能含 %FREE_VPN02 等非标准百分号编码，Go url.Parse 会拒绝。
func fixStrayPercent(nodeURL string) string {
	var b strings.Builder
	for i := 0; i < len(nodeURL); i++ {
		if nodeURL[i] != '%' {
			b.WriteByte(nodeURL[i])
			continue
		}
		if i+2 < len(nodeURL) && isHexByte(nodeURL[i+1]) && isHexByte(nodeURL[i+2]) {
			b.WriteByte('%')
			b.WriteByte(nodeURL[i+1])
			b.WriteByte(nodeURL[i+2])
			i += 2
		} else {
			b.WriteString("%25")
		}
	}
	return b.String()
}

// fixIllegalUserinfoChars 对 authority 中的非法字符做百分号编码。
// trojan/vless 密码可能含 [ < { 等，Go url.Parse 会拒绝。有 @ 时仅编码 userinfo 部分，不影响 IPv6 主机 [::1]。
func fixIllegalUserinfoChars(nodeURL string) string {
	schemeEnd := strings.Index(nodeURL, "://")
	if schemeEnd == -1 {
		return nodeURL
	}
	start := schemeEnd + 3

	authEnd := len(nodeURL)
	for i := start; i < len(nodeURL); i++ {
		if nodeURL[i] == '/' || nodeURL[i] == '?' || nodeURL[i] == '#' {
			authEnd = i
			break
		}
	}
	authority := nodeURL[start:authEnd]

	atIdx := strings.Index(authority, "@")
	if atIdx == -1 {
		// 无 @ —— authority 全部是 host。若是合法 IPv6 字面量 [::1]:port 则保留。
		if strings.HasPrefix(authority, "[") && strings.Contains(authority, "]") {
			return nodeURL
		}
		fixed := encodeIllegalUserinfoChars(authority)
		if fixed == authority {
			return nodeURL
		}
		return nodeURL[:start] + fixed + nodeURL[authEnd:]
	}

	userinfo := authority[:atIdx]
	fixed := encodeIllegalUserinfoChars(userinfo)
	if fixed == userinfo {
		return nodeURL
	}
	return nodeURL[:start] + fixed + authority[atIdx:] + nodeURL[authEnd:]
}

// encodeIllegalUserinfoChars 把不在 RFC 3986 userinfo 合法集合内的字符百分号编码（保留已编码的 %XX）。
func encodeIllegalUserinfoChars(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isUserinfoChar(c) || (c == '%' && i+2 < len(s) && isHexByte(s[i+1]) && isHexByte(s[i+2])) {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func isUserinfoChar(c byte) bool {
	switch {
	case 'A' <= c && c <= 'Z', 'a' <= c && c <= 'z', '0' <= c && c <= '9':
		return true
	}
	switch c {
	case '-', '.', '_', '~', '!', '$', '&', '\'', '(', ')', '*', '+', ',', ';', '=', ':':
		return true
	}
	return false
}

func isHexByte(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}
