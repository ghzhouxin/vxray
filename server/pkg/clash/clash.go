// Package clash 生成 Clash YAML 配置。
package clash

import (
	"fmt"

	"v2ray-server/pkg/types"

	"gopkg.in/yaml.v3"
)

const (
	defaultPort             = 7890
	defaultSocksPort        = 7891
	defaultAutoTestURL      = "https://www.gstatic.com/generate_204"
	defaultAutoTestInterval = 300
)

var defaultRules = []string{
	"GEOIP,CN,DIRECT",
	"MATCH,Proxy",
}

// ClashNode 由 internal/model.Node 实现。
// 保留 Get 前缀：Go 不允许导出字段与同名方法共存。
type ClashNode interface {
	GetName() string
	GetProtocol() string
	GetAddress() string
	GetPort() int
	GetRawConfig() types.Map
	GetOutboundConfig() types.Map
}

type config struct {
	Port        int      `yaml:"port"`
	SocksPort   int      `yaml:"socks-port"`
	AllowLan    bool     `yaml:"allow-lan"`
	Mode        string   `yaml:"mode"`
	LogLevel    string   `yaml:"log-level"`
	Proxies     []any    `yaml:"proxies,omitempty"`
	ProxyGroups []group  `yaml:"proxy-groups"`
	Rules       []string `yaml:"rules"`
}

type group struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Proxies  []string `yaml:"proxies,omitempty"`
	URL      string   `yaml:"url,omitempty"`
	Interval int      `yaml:"interval,omitempty"`
}

func GenerateConfig(nodes []ClashNode) ([]byte, error) {
	proxies := make([]any, 0, len(nodes))
	proxyNames := make([]string, 0, len(nodes))

	for _, node := range nodes {
		proxy, err := nodeToProxy(node)
		if err != nil {
			continue
		}
		proxies = append(proxies, proxy)
		proxyNames = append(proxyNames, node.GetName())
	}

	cfg := &config{
		Port:      defaultPort,
		SocksPort: defaultSocksPort,
		AllowLan:  false,
		Mode:      "rule",
		LogLevel:  "info",
		Proxies:   proxies,
		ProxyGroups: []group{
			{
				Name:     "Auto",
				Type:     "url-test",
				Proxies:  proxyNames,
				URL:      defaultAutoTestURL,
				Interval: defaultAutoTestInterval,
			},
			{
				Name:    "Proxy",
				Type:    "select",
				Proxies: append([]string{"Auto"}, proxyNames...),
			},
		},
		Rules: defaultRules,
	}

	return yaml.Marshal(cfg)
}

// protocolBuilder 描述每种协议到 Clash proxy 字段的映射。
type protocolBuilder struct {
	clashType string
	fields    func(raw types.Map) map[string]any
}

var protocolBuilders = map[string]protocolBuilder{
	types.ProtocolVMess: {
		clashType: "vmess",
		fields: func(raw types.Map) map[string]any {
			return map[string]any{
				"uuid":   raw["uuid"],
				"cipher": raw["security"],
			}
		},
	},
	types.ProtocolVLESS: {
		clashType: "vless",
		fields: func(raw types.Map) map[string]any {
			m := map[string]any{"uuid": raw["uuid"]}
			if flow, ok := raw["flow"]; ok {
				m["flow"] = flow
			}
			return m
		},
	},
	types.ProtocolTrojan: {
		clashType: "trojan",
		fields: func(raw types.Map) map[string]any {
			return map[string]any{"password": raw["password"]}
		},
	},
	types.ProtocolShadowsocks: {
		clashType: "ss",
		fields: func(raw types.Map) map[string]any {
			return map[string]any{
				"cipher":   raw["method"],
				"password": raw["password"],
			}
		},
	},
}

func nodeToProxy(node ClashNode) (map[string]any, error) {
	b, ok := protocolBuilders[node.GetProtocol()]
	if !ok {
		return nil, fmt.Errorf("unsupported protocol: %s", node.GetProtocol())
	}
	proxy := map[string]any{
		"name":   node.GetName(),
		"type":   b.clashType,
		"server": node.GetAddress(),
		"port":   node.GetPort(),
	}
	for k, v := range b.fields(node.GetRawConfig()) {
		proxy[k] = v
	}
	applyTransport(proxy, node.GetOutboundConfig())
	return proxy, nil
}

func applyTransport(proxy map[string]any, outbound types.Map) {
	streamSettings, ok := outbound["streamSettings"].(types.Map)
	if !ok {
		return
	}
	applyStreamSecurity(proxy, streamSettings)
	applyStreamNetwork(proxy, streamSettings)
}

// applyStreamSecurity 处理 TLS / Reality 安全层设置。
func applyStreamSecurity(proxy map[string]any, streamSettings types.Map) {
	security, ok := streamSettings["security"]
	if !ok {
		return
	}
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
			proxy["reality-opts"] = buildRealityOpts(realitySettings)
			copyIfPresent(proxy, realitySettings, "fingerprint", "client-fingerprint")
		}
	}
}

// applyStreamNetwork 处理 ws/httpupgrade/grpc 传输层设置。
func applyStreamNetwork(proxy map[string]any, streamSettings types.Map) {
	network, ok := streamSettings["network"].(string)
	if !ok {
		return
	}
	switch network {
	case "ws", "httpupgrade":
		proxy["network"] = "ws" // Clash 用 ws-opts 表示 httpupgrade
		settingsKey := network + "Settings"
		settings, _ := streamSettings[settingsKey].(types.Map)
		if opts := buildWSOpts(settings); opts != nil {
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

func copyIfPresent(dst map[string]any, src types.Map, srcKey, dstKey string) {
	if v, ok := src[srcKey]; ok {
		dst[dstKey] = v
	}
}

// buildWSOpts 构造 Clash 的 ws-opts 字段。
// wsSettings 的 headers 已是 map；httpupgradeSettings 的 host 需转为 {"Host": hostStr}。
func buildWSOpts(settings types.Map) map[string]any {
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

func buildRealityOpts(realitySettings types.Map) map[string]any {
	opts := map[string]any{}
	copyIfPresent(opts, realitySettings, "publicKey", "public-key")
	copyIfPresent(opts, realitySettings, "shortId", "short-id")
	return opts
}
