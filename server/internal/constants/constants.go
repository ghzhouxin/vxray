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

const (
	DefaultLogPageSize = 50
	NodeBatchSize      = 100
)

// 数据库常量
const (
	SQLiteBusyTimeoutMs = 5000
)

// 运行时常量
const (
	ShutdownTimeout    = 5 * time.Second
	ClashTopNodesLimit = 50
	ProxyHTTPPort      = 18889
	ProxySOCKSPort     = 18888
)

// 日志标签常量
const (
	TagSpeedtest    = "speed"
	TagSubscription = "subs"
	TagXray         = "xray"
	TagGeo          = "geo"
	TagSystem       = "system" // 默认标签，非重要日志使用
)

var LogLevels = []string{"debug", "info", "warn", "error"}

var LogTags = []string{TagSpeedtest, TagSubscription, TagXray, TagGeo, TagSystem}

var NodeProtocols = []string{"vless", "vmess", "trojan", "shadowsocks"}

// Clash 配置常量
const (
	ClashPort             = 7890
	ClashSocksPort        = 7891
	ClashAutoTestURL      = "https://www.gstatic.com/generate_204"
	ClashAutoTestInterval = 300
)

var ClashRules = []string{
	"GEOIP,CN,DIRECT",
	"MATCH,Proxy",
}
