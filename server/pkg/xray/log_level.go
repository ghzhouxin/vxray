package xray

import "strings"

// classifyLogLevel 从 xray 日志行推断级别。
// 用于 Manager（用户态）和 SupervisorClient（root，TUN 模式）的日志回调。
func classifyLogLevel(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "error"):
		return "error"
	case strings.Contains(lower, "warn"):
		return "warn"
	case strings.Contains(lower, "info"):
		return "info"
	default:
		return "debug"
	}
}
