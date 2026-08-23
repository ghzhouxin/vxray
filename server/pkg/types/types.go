package types

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

type Map map[string]any

func (m Map) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

func (m *Map) Scan(value any) error {
	if value == nil {
		*m = nil
		return nil
	}
	data, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(data, m)
}

type ParsedNode struct {
	Name      string
	Protocol  string
	Address   string
	Port      int
	RawURL    string
	RawConfig Map
	Transport Transport
}

func (n *ParsedNode) IdentityKey() string {
	var buf bytes.Buffer
	buf.WriteString(n.Address)
	buf.WriteByte(':')
	buf.WriteString(strconv.Itoa(n.Port))
	buf.WriteByte(':')

	if n.RawConfig != nil {
		if uuid, ok := n.RawConfig["uuid"].(string); ok && uuid != "" {
			buf.WriteString(uuid)
		} else if pwd, ok := n.RawConfig["password"].(string); ok && pwd != "" {
			buf.WriteString(pwd)
		} else {
			buf.WriteString(n.Protocol)
		}
	} else {
		buf.WriteString(n.Protocol)
	}

	return buf.String()
}

const (
	ProtocolVMess       = "vmess"
	ProtocolVLESS       = "vless"
	ProtocolTrojan      = "trojan"
	ProtocolShadowsocks = "shadowsocks"
)

// Network / Security 传输层常量，作为 subscription/clash/xray 三方的单一真源。
const (
	NetworkTCP         = "tcp"
	NetworkWS          = "ws"
	NetworkGRPC        = "grpc"
	NetworkHTTPUpgrade = "httpupgrade"
	NetworkXHTTP       = "xhttp"
	NetworkRaw         = "raw" // xray-core v25+ alias for tcp
	SecurityTLS        = "tls"
	SecurityReality    = "reality"
)

func ProtocolLabel(protocol string) string {
	if protocol == ProtocolShadowsocks {
		return "SS"
	}
	return strings.ToUpper(protocol)
}

const (
	PrefixVMess  = "vmess://"
	PrefixVLESS  = "vless://"
	PrefixTrojan = "trojan://"
	PrefixSS     = "ss://"
	PrefixSSR    = "ssr://"
)

var ProtocolPrefixes = []string{
	PrefixVMess,
	PrefixVLESS,
	PrefixTrojan,
	PrefixSS,
	PrefixSSR,
}
