package xray

import "strings"

// classifyLogLevel 按 xray 日志的级别标记精确匹配。
// 无标记返回 ""（调用方过滤后不入库），"Failed to start:" 是 xray 启动失败的固定输出。
func classifyLogLevel(line string) string {
	switch {
	case strings.Contains(line, "[Error]"):
		return "error"
	case strings.Contains(line, "[Warning]"):
		return "warn"
	case strings.Contains(line, "[Info]"):
		return "info"
	case strings.Contains(line, "[Debug]"):
		return "debug"
	case strings.Contains(line, "Failed to start:"):
		return "error"
	default:
		return ""
	}
}
