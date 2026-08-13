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
	NodeBatchSize      = 50
)

const (
	SQLiteBusyTimeoutMs = 5000
)

const (
	ShutdownTimeout    = 5 * time.Second
	ClashTopNodesLimit = 32
	ProxyHTTPPort      = 18889
	ProxySOCKSPort     = 18888
)

const (
	TagApp          = "app"
	TagSpeedtest    = "speed"
	TagSubscription = "subs"
	TagXray         = "xray"
	TagGeo          = "geo"
	TagTun          = "tun"
)

const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

var LogLevels = []string{LevelDebug, LevelInfo, LevelWarn, LevelError}

var LogTags = []string{TagApp, TagSpeedtest, TagSubscription, TagXray, TagGeo, TagTun}

var NodeProtocols = []string{"vless", "vmess", "trojan", "shadowsocks"}
