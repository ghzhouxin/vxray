package constants

import "time"

const (
	LatencyTimeout  = -1
	LatencyUntested = 0
	LatencyMinValid = 1
)

const (
	LatencyStatusPending   = "pending"
	LatencyStatusAvailable = "available"
	LatencyStatusTimeout   = "timeout"
)

func LatencyStatus(latency int64) string {
	switch {
	case latency >= 1:
		return LatencyStatusAvailable
	case latency == -1:
		return LatencyStatusTimeout
	default:
		return LatencyStatusPending
	}
}

const (
	DefaultLogPageSize = 50
	NodeBatchSize      = 100
)

const (
	SQLiteBusyTimeoutMs = 5000
)

const (
	ShutdownTimeout    = 5 * time.Second
	ClashTopNodesLimit = 50
	ProxyHTTPPort      = 18889
	ProxySOCKSPort     = 18888
)

const (
	TagSpeedtest    = "speed"
	TagSubscription = "subs"
	TagXray         = "xray"
	TagGeo          = "geo"
	TagSystem       = "system"
)

var LogLevels = []string{"debug", "info", "warn", "error"}

var LogTags = []string{TagSpeedtest, TagSubscription, TagXray, TagGeo, TagSystem}

var NodeProtocols = []string{"vless", "vmess", "trojan", "shadowsocks"}
