package service

import (
	"fmt"

	"v2ray-server/internal/constants"
	"v2ray-server/internal/model"
	"v2ray-server/pkg/types"

	"gopkg.in/yaml.v3"
)

type ClashConfig struct {
	Port        int          `yaml:"port"`
	SocksPort   int          `yaml:"socks-port"`
	AllowLan    bool         `yaml:"allow-lan"`
	Mode        string       `yaml:"mode"`
	LogLevel    string       `yaml:"log-level"`
	Proxies     []any        `yaml:"proxies,omitempty"`
	ProxyGroups []ClashGroup `yaml:"proxy-groups"`
	Rules       []string     `yaml:"rules"`
}

type ClashGroup struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Proxies  []string `yaml:"proxies,omitempty"`
	URL      string   `yaml:"url,omitempty"`
	Interval int      `yaml:"interval,omitempty"`
}

func GenerateClashConfig(nodes []*model.Node) ([]byte, error) {
	proxies := make([]any, 0, len(nodes))
	proxyNames := make([]string, 0, len(nodes))

	for _, node := range nodes {
		proxy, err := nodeToClashProxy(node)
		if err != nil {
			continue
		}
		proxies = append(proxies, proxy)
		proxyNames = append(proxyNames, node.Name)
	}

	config := &ClashConfig{
		Port:      constants.ClashPort,
		SocksPort: constants.ClashSocksPort,
		AllowLan:  false,
		Mode:      "rule",
		LogLevel:  "info",
		Proxies:   proxies,
		ProxyGroups: []ClashGroup{
			{
				Name:     "Auto",
				Type:     "url-test",
				Proxies:  proxyNames,
				URL:      constants.ClashAutoTestURL,
				Interval: constants.ClashAutoTestInterval,
			},
			{
				Name:    "Proxy",
				Type:    "select",
				Proxies: append([]string{"Auto"}, proxyNames...),
			},
		},
		Rules: constants.ClashRules,
	}

	return yaml.Marshal(config)
}

func nodeToClashProxy(node *model.Node) (map[string]any, error) {
	switch node.Protocol {
	case types.ProtocolVMess:
		return buildClashVMess(node)
	case types.ProtocolVLESS:
		return buildClashVLESS(node)
	case types.ProtocolTrojan:
		return buildClashTrojan(node)
	case types.ProtocolShadowsocks, types.ProtocolShadowsocksR:
		return buildClashSS(node)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", node.Protocol)
	}
}

func buildClashVMess(node *model.Node) (map[string]any, error) {
	proxy := map[string]any{
		"name":   node.Name,
		"type":   "vmess",
		"server": node.Address,
		"port":   node.Port,
		"uuid":   node.RawConfig["uuid"],
		"cipher": node.RawConfig["security"],
	}
	applyClashTransport(proxy, node.OutboundConfig)
	return proxy, nil
}

func buildClashVLESS(node *model.Node) (map[string]any, error) {
	proxy := map[string]any{
		"name":   node.Name,
		"type":   "vless",
		"server": node.Address,
		"port":   node.Port,
		"uuid":   node.RawConfig["uuid"],
	}
	if flow, ok := node.RawConfig["flow"]; ok {
		proxy["flow"] = flow
	}
	applyClashTransport(proxy, node.OutboundConfig)
	return proxy, nil
}

func buildClashTrojan(node *model.Node) (map[string]any, error) {
	proxy := map[string]any{
		"name":     node.Name,
		"type":     "trojan",
		"server":   node.Address,
		"port":     node.Port,
		"password": node.RawConfig["password"],
	}
	applyClashTransport(proxy, node.OutboundConfig)
	return proxy, nil
}

func buildClashSS(node *model.Node) (map[string]any, error) {
	proxy := map[string]any{
		"name":     node.Name,
		"type":     "ss",
		"server":   node.Address,
		"port":     node.Port,
		"cipher":   node.RawConfig["method"],
		"password": node.RawConfig["password"],
	}
	applyClashTransport(proxy, node.OutboundConfig)
	return proxy, nil
}

func applyClashTransport(proxy map[string]any, outbound types.Map) {
	streamSettings, ok := outbound["streamSettings"].(types.Map)
	if !ok {
		return
	}

	if security, ok := streamSettings["security"]; ok {
		switch security {
		case "tls":
			proxy["tls"] = true
			if tlsSettings, ok := streamSettings["tlsSettings"].(types.Map); ok {
				copyIfPresent(proxy, tlsSettings, "serverName", "servername")
				copyIfPresent(proxy, tlsSettings, "fingerprint", "client-fingerprint")
				copyIfPresent(proxy, tlsSettings, "alpn", "alpn")
			}
		case "reality":
			proxy["tls"] = true
			if realitySettings, ok := streamSettings["realitySettings"].(types.Map); ok {
				proxy["reality-opts"] = buildClashRealityOpts(realitySettings)
				copyIfPresent(proxy, realitySettings, "fingerprint", "client-fingerprint")
			}
		}
	}

	network, ok := streamSettings["network"].(string)
	if !ok {
		return
	}
	switch network {
	case "ws", "httpupgrade":
		proxy["network"] = "ws" // Clash 用 ws-opts 表示 httpupgrade
		settingsKey := network + "Settings"
		settings, _ := streamSettings[settingsKey].(types.Map)
		if opts := buildClashWSOpts(settings); opts != nil {
			proxy["ws-opts"] = opts
		}
	case "grpc":
		proxy["network"] = "grpc"
		if grpcSettings, ok := streamSettings["grpcSettings"].(types.Map); ok {
			opts := map[string]any{}
			copyIfPresent(opts, grpcSettings, "serviceName", "grpc-service-name")
			proxy["grpc-opts"] = opts
		}
	}
}

// copyIfPresent copies src[srcKey] to dst[dstKey] if srcKey exists in src.
func copyIfPresent(dst map[string]any, src types.Map, srcKey, dstKey string) {
	if v, ok := src[srcKey]; ok {
		dst[dstKey] = v
	}
}

// buildClashWSOpts 构造 Clash 的 ws-opts 字段。
// wsSettings 的 headers 已是 map；httpupgradeSettings 的 host 需转为 {"Host": hostStr}。
func buildClashWSOpts(settings types.Map) map[string]any {
	if len(settings) == 0 {
		return nil
	}
	opts := map[string]any{}
	if path, ok := settings["path"]; ok {
		opts["path"] = path
	}
	if headers, ok := settings["headers"]; ok {
		opts["headers"] = headers
	} else if hostStr, ok := settings["host"].(string); ok {
		opts["headers"] = map[string]string{"Host": hostStr}
	}
	return opts
}

func buildClashRealityOpts(realitySettings types.Map) map[string]any {
	opts := map[string]any{}
	copyIfPresent(opts, realitySettings, "publicKey", "public-key")
	copyIfPresent(opts, realitySettings, "shortId", "short-id")
	return opts
}
