package xray

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// defaultTunInbound 返回默认的 TUN inbound 配置。
// name: 固定 utun100，避免与系统 utun0-N 冲突（xray-core 要求 utunN 格式，N∈[10,1024]）。
// autoSystemRoutingTable: 接管全部 IPv4/IPv6 流量，xray-core 用 protected routes 实现，不会路由环路。
// mtu/gateway/dns 省略：使用 xray-core 默认值 (1500 / 169.254.10.1/30 / 无)。
// 需要 xray-core 26.7.28+（支持 autoSystemRoutingTable）。
func defaultTunInbound() map[string]any {
	return map[string]any{
		"tag":      "tun-in",
		"protocol": "tun",
		"settings": map[string]any{
			"name":                   "utun100",
			"autoSystemRoutingTable": []string{"0.0.0.0/0", "::/0"},
		},
		"sniffing": map[string]any{
			"enabled":      true,
			"destOverride": []string{"http", "tls"},
		},
	}
}

// InjectTunInbound 读取 srcConfigPath 的 xray config，注入 tun inbound，写到 dstConfigPath。
// 若已有 protocol=="tun" 的 inbound 则用默认 tun inbound 替换首个，并丢弃其余 tun inbound，
// 否则追加到 inbounds 末尾。inbounds 缺失或非数组时视为空。
func InjectTunInbound(srcConfigPath, dstConfigPath string) error {
	data, err := os.ReadFile(srcConfigPath)
	if err != nil {
		return fmt.Errorf("read source config: %w", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}

	inbounds, _ := cfg["inbounds"].([]any) // 缺失/非数组视为空
	tunInbound := defaultTunInbound()

	newInbounds := make([]any, 0, len(inbounds)+1)
	replaced := false
	for _, ib := range inbounds {
		ibMap, ok := ib.(map[string]any)
		if !ok {
			newInbounds = append(newInbounds, ib)
			continue
		}
		if protocol, _ := ibMap["protocol"].(string); protocol == "tun" {
			// 仅保留首个 tun inbound（替换为默认配置），后续 tun inbound 丢弃避免重复
			if !replaced {
				newInbounds = append(newInbounds, tunInbound)
				replaced = true
			}
			continue
		}
		newInbounds = append(newInbounds, ib)
	}
	if !replaced {
		newInbounds = append(newInbounds, tunInbound)
	}
	cfg["inbounds"] = newInbounds

	if err := os.MkdirAll(filepath.Dir(dstConfigPath), 0o755); err != nil {
		return fmt.Errorf("create dst dir: %w", err)
	}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(dstConfigPath, out, 0o644); err != nil {
		return fmt.Errorf("write dst config: %w", err)
	}
	return nil
}
