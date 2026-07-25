package subscription

const (
	NetworkTCP                = "tcp"
	NetworkWS                 = "ws"
	NetworkGRPC               = "grpc"
	NetworkHTTPUpgrade        = "httpupgrade"
	NetworkXHTTP              = "xhttp"
	NetworkRaw                = "raw" // xray-core v25+ alias for tcp
	SecurityTLS               = "tls"
	SecurityReality           = "reality"
	SecurityNone              = "none"
	DefaultPort               = 443
	DefaultSecurity           = "auto"
	DefaultRealityFingerprint = "chrome" // xray-core requires fingerprint for REALITY
	FlowXTLSRprxVision        = "xtls-rprx-vision"
)
