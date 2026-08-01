// Package clash 生成 Clash YAML 配置。
package clash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"v2ray-server/pkg/types"

	"gopkg.in/yaml.v3"
)

const (
	defaultPort              = 7890
	defaultSocksPort         = 7891
	defaultGeoUpdateInterval = 24
)

// defaultRules 遵循「先拒后直后代理」顺序：
// 广告 REJECT 优先于其他匹配，避免广告域名被误判为国内直连而漏屏蔽。
var defaultRules = []string{
	"GEOSITE,category-ads-all,REJECT", // 广告屏蔽
	"GEOSITE,cn,DIRECT",               // 国内域名直连
	"GEOIP,CN,DIRECT",                 // 国内 IP 直连
	"MATCH,Proxy",                     // 其余（墙外）走代理
}

// ClashNodeData 是 Clash 配置生成所需的节点数据。
type ClashNodeData struct {
	Name        string
	Protocol    string
	Address     string
	Port        int
	RawConfig   types.Map
	Transport   types.Transport
	IdentityKey string
}

type config struct {
	Port              int      `yaml:"port"`
	SocksPort         int      `yaml:"socks-port"`
	AllowLan          bool     `yaml:"allow-lan"`
	Mode              string   `yaml:"mode"`
	LogLevel          string   `yaml:"log-level"`
	GeodataMode       bool     `yaml:"geodata-mode"`
	GeoAutoUpdate     bool     `yaml:"geo-auto-update"`
	GeoUpdateInterval int      `yaml:"geo-update-interval"`
	Proxies           []any    `yaml:"proxies,omitempty"`
	ProxyGroups       []group  `yaml:"proxy-groups"`
	Rules             []string `yaml:"rules"`
}

type group struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	Proxies []string `yaml:"proxies,omitempty"`
}

func GenerateConfig(nodes []ClashNodeData) ([]byte, error) {
	proxies := make([]any, 0, len(nodes))
	proxyNames := make([]string, 0, len(nodes))
	// 预占代理组名，避免节点名与组名冲突导致 Clash 校验失败。
	seen := map[string]struct{}{"Proxy": {}}

	for _, node := range nodes {
		proxy, err := nodeToProxy(node)
		if err != nil {
			continue
		}
		name := uniqueName(node.Name, node.IdentityKey, seen)
		proxy["name"] = name
		proxies = append(proxies, proxy)
		proxyNames = append(proxyNames, name)
	}

	cfg := &config{
		Port:              defaultPort,
		SocksPort:         defaultSocksPort,
		AllowLan:          false,
		Mode:              "rule",
		LogLevel:          "info",
		GeodataMode:       true,
		GeoAutoUpdate:     true,
		GeoUpdateInterval: defaultGeoUpdateInterval,
		Proxies:           proxies,
		ProxyGroups: []group{
			{
				Name:    "Proxy",
				Type:    "select",
				Proxies: proxyNames,
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
				"uuid":    raw["uuid"],
				"cipher":  raw["security"],
				"alterId": 0,
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
			if pe, ok := raw["packetEncoding"]; ok {
				m["packet-encoding"] = pe
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

func nodeToProxy(node ClashNodeData) (map[string]any, error) {
	b, ok := protocolBuilders[node.Protocol]
	if !ok {
		return nil, fmt.Errorf("unsupported protocol: %s", node.Protocol)
	}
	proxy := map[string]any{
		"type":   b.clashType,
		"server": node.Address,
		"port":   node.Port,
		"udp":    true,
	}
	for k, v := range b.fields(node.RawConfig) {
		proxy[k] = v
	}
	applyTransport(proxy, node.Transport, b.clashType)
	return proxy, nil
}

// uniqueName 返回与 seen 中已有名称不冲突的唯一名称。
// 无冲突时使用原始名称；冲突时追加 identityKey 的短哈希，保证确定性且与节点顺序无关。
func uniqueName(base, identityKey string, seen map[string]struct{}) string {
	if _, ok := seen[base]; !ok {
		seen[base] = struct{}{}
		return base
	}
	sum := sha256.Sum256([]byte(identityKey))
	name := base + "-" + hex.EncodeToString(sum[:4])
	seen[name] = struct{}{}
	return name
}

// applyTransport 从 Transport 直接构建 Clash proxy 的传输层字段。
func applyTransport(proxy map[string]any, t types.Transport, clashType string) {
	sniKey := "servername"
	if clashType == "trojan" {
		sniKey = "sni"
	}
	applySecurity(proxy, t, sniKey)
	applyNetwork(proxy, t)
}

func applySecurity(proxy map[string]any, t types.Transport, sniKey string) {
	switch t.Security {
	case types.SecurityTLS:
		proxy["tls"] = true
		if t.TLS != nil {
			if t.TLS.ServerName != "" {
				proxy[sniKey] = t.TLS.ServerName
			}
			if t.TLS.Fingerprint != "" {
				proxy["client-fingerprint"] = t.TLS.Fingerprint
			}
			if len(t.TLS.ALPN) > 0 {
				proxy["alpn"] = t.TLS.ALPN
			}
		}
	case types.SecurityReality:
		proxy["tls"] = true
		if t.Reality != nil {
			proxy["reality-opts"] = buildRealityOpts(t.Reality)
			if t.Reality.ServerName != "" {
				proxy[sniKey] = t.Reality.ServerName
			}
			if t.Reality.Fingerprint != "" {
				proxy["client-fingerprint"] = t.Reality.Fingerprint
			}
		}
	}
}

func applyNetwork(proxy map[string]any, t types.Transport) {
	proxy["network"] = t.Network
	switch t.Network {
	case types.NetworkWS, types.NetworkHTTPUpgrade:
		if t.WebSocket != nil {
			proxy["ws-opts"] = buildWSOptsFromTransport(t.WebSocket)
		}
	case types.NetworkGRPC:
		if t.GRPC != nil {
			proxy["grpc-opts"] = buildGRPCOpts(t.GRPC)
		}
	case types.NetworkXHTTP:
		if t.XHTTP != nil {
			proxy["xhttp-opts"] = buildXHTTPOpts(t.XHTTP)
		}
	default: // 空值或未知 network 归一化为 tcp
		proxy["network"] = types.NetworkTCP
		if t.TCP != nil && t.TCP.HeaderType == "http" {
			proxy["http-opts"] = buildHTTPOptsFromTCP(t.TCP)
		}
	}
}

func buildWSOptsFromTransport(ws *types.WebSocketConfig) map[string]any {
	opts := map[string]any{}
	if ws.Path != "" {
		opts["path"] = ws.Path
	}
	if ws.Host != "" {
		opts["headers"] = map[string]string{"Host": ws.Host}
	}
	return opts
}

func buildHTTPOptsFromTCP(tcp *types.TCPConfig) map[string]any {
	opts := map[string]any{}
	if tcp.Path != "" {
		opts["path"] = []string{tcp.Path}
	}
	if tcp.Host != "" {
		opts["headers"] = map[string][]string{"Host": {tcp.Host}}
	}
	return opts
}

func buildRealityOpts(r *types.RealityConfig) map[string]any {
	opts := map[string]any{}
	if r.PublicKey != "" {
		opts["public-key"] = r.PublicKey
	}
	if r.ShortID != "" {
		opts["short-id"] = r.ShortID
	}
	return opts
}

// buildGRPCOpts 构造 Clash 的 grpc-opts 字段。
// multiMode=true → grpc-mode: multi，否则 gun（Clash 默认）。
func buildGRPCOpts(g *types.GRPCConfig) map[string]any {
	opts := map[string]any{}
	if g.MultiMode {
		opts["grpc-mode"] = "multi"
	} else {
		opts["grpc-mode"] = "gun"
	}
	if g.ServiceName != "" {
		opts["grpc-service-name"] = g.ServiceName
	}
	return opts
}

func buildXHTTPOpts(x *types.XHTTPConfig) map[string]any {
	opts := map[string]any{}
	if x.Path != "" {
		opts["path"] = x.Path
	}
	if x.Host != "" {
		opts["host"] = x.Host
	}
	if x.Mode != "" {
		opts["mode"] = x.Mode
	}
	return opts
}
