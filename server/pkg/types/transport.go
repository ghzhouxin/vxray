package types

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// Transport 描述节点的传输层配置，独立于 xray/clash 的具体格式。
// subscription 填充此结构，xray/clash 转换器各自读取所需字段。
type Transport struct {
	Network   string // tcp, ws, httpupgrade, grpc, xhttp
	Security  string // tls, reality, ""（none）
	TLS       *TLSConfig
	Reality   *RealityConfig
	WebSocket *WebSocketConfig
	GRPC      *GRPCConfig
	XHTTP     *XHTTPConfig
	TCP       *TCPConfig
}

type TLSConfig struct {
	ServerName           string   `json:"serverName,omitempty"`
	Fingerprint          string   `json:"fingerprint,omitempty"`
	ALPN                 []string `json:"alpn,omitempty"`
	ECHConfigList        string   `json:"echConfigList,omitempty"`        // ech
	VerifyPeerCertByName string   `json:"verifyPeerCertByName,omitempty"` // vcn
	PinnedPeerCertSha256 string   `json:"pinnedPeerCertSha256,omitempty"` // pcs
}

type RealityConfig struct {
	ServerName    string `json:"serverName,omitempty"`
	PublicKey     string `json:"publicKey,omitempty"`
	ShortID       string `json:"shortId,omitempty"`
	Fingerprint   string `json:"fingerprint,omitempty"`
	SpiderX       string `json:"spiderX,omitempty"`       // spx
	Mldsa65Verify string `json:"mldsa65Verify,omitempty"` // pqv
}

type WebSocketConfig struct {
	Path string `json:"path,omitempty"`
	Host string `json:"host,omitempty"` // 顶层 host（xray v26 格式）
}

type GRPCConfig struct {
	ServiceName string `json:"serviceName,omitempty"`
	Authority   string `json:"authority,omitempty"`
	MultiMode   bool   `json:"multiMode,omitempty"`
}

type XHTTPConfig struct {
	Path          string `json:"path,omitempty"`
	Host          string `json:"host,omitempty"`
	Mode          string `json:"mode,omitempty"`
	Extra         Map    `json:"extra,omitempty"`
	XPaddingBytes Map    `json:"xPaddingBytes,omitempty"` // Int32Range {min, max}
	XPaddingKey   string `json:"xPaddingKey,omitempty"`   // 默认 "x_padding"
}

type TCPConfig struct {
	HeaderType string `json:"headerType,omitempty"` // "http" 或 ""
	Path       string `json:"path,omitempty"`
	Host       string `json:"host,omitempty"`
}

// Value 实现 driver.Valuer，JSON 序列化存 DB。
func (t Transport) Value() (driver.Value, error) {
	return json.Marshal(t)
}

// Scan 实现 sql.Scanner，从 DB JSON 反序列化。
func (t *Transport) Scan(value any) error {
	if value == nil {
		*t = Transport{}
		return nil
	}
	data, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(data, t)
}
