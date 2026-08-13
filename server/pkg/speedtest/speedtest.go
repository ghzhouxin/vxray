package speedtest

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"v2ray-server/pkg/types"

	xraynet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/core"
	_ "github.com/xtls/xray-core/main/distro/all"
)

var errNilOutbound = errors.New("outbound config is nil")

const (
	defaultTargetURL = "https://www.google.com/generate_204"
)

type Result struct {
	NodeID  uint
	Latency int64
	Error   string
}

type Progress struct {
	Total     int
	Completed int
	Success   int
	Failed    int
	NodeID    uint
	Latency   int64
	ErrMsg    string
	Testing   bool
}

type Node struct {
	ID       uint
	Outbound types.Map
}

type Config interface {
	TargetURL() string
	Timeout() time.Duration
	Concurrency() int
}

type SpeedTest struct {
	cfg Config
}

func New(cfg Config) *SpeedTest {
	return &SpeedTest{cfg: cfg}
}

func (st *SpeedTest) TestOutbound(outbound types.Map, nodeID uint) (result *Result) {
	var instance *core.Instance
	defer func() {
		if instance != nil {
			defer func() { _ = recover() }()
			instance.Close()
		}
	}()
	defer func() {
		if r := recover(); r != nil {
			result = &Result{NodeID: nodeID, Latency: -1, Error: fmt.Sprintf("speedtest panic: %v", r)}
		}
	}()

	if outbound == nil {
		return &Result{NodeID: nodeID, Latency: -1, Error: errNilOutbound.Error()}
	}

	var err error
	instance, err = st.createInstance(outbound)
	if err != nil {
		return &Result{NodeID: nodeID, Latency: -1, Error: err.Error()}
	}

	latency, err := st.measureDelay(instance)
	if err != nil {
		return &Result{NodeID: nodeID, Latency: -1, Error: err.Error()}
	}
	return &Result{NodeID: nodeID, Latency: latency}
}

func (st *SpeedTest) TestWithProxyAndTarget(socksPort int, targetURL string) (result *Result) {
	defer func() {
		if r := recover(); r != nil {
			result = &Result{Latency: -1, Error: fmt.Sprintf("website speedtest panic: %v", r)}
		}
	}()
	latency, err := st.measureDelayViaProxy(socksPort, targetURL)
	if err != nil {
		return &Result{Latency: -1, Error: err.Error()}
	}
	return &Result{Latency: latency}
}

func (st *SpeedTest) TestNodes(nodes []Node, progressChan chan<- Progress) {
	if len(nodes) == 0 {
		if progressChan != nil {
			close(progressChan)
		}
		return
	}

	var done, succ, fail atomic.Int64
	total := len(nodes)

	// 流水线：阶段1 TCP 预筛 → 阶段2 xray 测速
	// 通过 channel 连接两阶段，阶段1通过的节点立即送入阶段2，无需等待全部预筛完成
	passedChan := make(chan Node, st.cfg.Concurrency())
	var prescreenWG sync.WaitGroup
	prescreenWG.Add(1)
	go func() {
		defer prescreenWG.Done()
		st.tcpPrescreen(nodes, passedChan, progressChan, &done, &succ, &fail, total)
		close(passedChan)
	}()

	// 阶段2：从 channel 读取通过预筛的节点，做完整 xray 测速
	st.testOutbounds(passedChan, progressChan, &done, &succ, &fail, total)

	prescreenWG.Wait()
	if progressChan != nil {
		close(progressChan)
	}
}

// tcpPrescreen 对节点做 TCP+TLS 连通性预筛，通过预筛的节点送入 passedChan。
// 单节点预算 = 主测速超时 × 3/4，TCP 拨号与 TLS 握手共享该 ctx，
// 严格保证预筛最差耗时低于主测速超时。
func (st *SpeedTest) tcpPrescreen(
	nodes []Node,
	passedChan chan<- Node,
	progressChan chan<- Progress,
	done, succ, fail *atomic.Int64,
	total int,
) {
	const (
		prescreenFactor = 2
		maxPrescreen    = 256
	)
	concurrency := st.cfg.Concurrency() * prescreenFactor
	if concurrency > maxPrescreen {
		concurrency = maxPrescreen
	}
	budget := st.cfg.Timeout() * 3 / 4

	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	for _, node := range nodes {
		wg.Add(1)
		go func(n Node) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					sendPrescreenFail(progressChan, n, done, succ, fail, total, fmt.Sprintf("prescreen panic: %v", r))
				}
			}()

			addr, port := extractServerAddr(n.Outbound)
			if addr == "" || port == 0 {
				passedChan <- n
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), budget)
			defer cancel()

			// defer conn.Close() 统一覆盖 TLS 与非 TLS 路径，避免 fd 泄漏。
			conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(addr, fmt.Sprintf("%d", port)))
			if err != nil {
				sendPrescreenFail(progressChan, n, done, succ, fail, total, fmt.Sprintf("tcp prescreen: %v", err))
				return
			}
			defer conn.Close()

			if sni := extractTLSServerName(n.Outbound); sni != "" {
				tlsConn := tls.Client(conn, &tls.Config{ServerName: sni, InsecureSkipVerify: true})
				if err := tlsConn.HandshakeContext(ctx); err != nil {
					sendPrescreenFail(progressChan, n, done, succ, fail, total, fmt.Sprintf("tls prescreen: %v", err))
					return
				}
			}
			passedChan <- n
		}(node)
	}
	wg.Wait()
}

func sendProgress(ch chan<- Progress, p Progress) {
	if ch == nil {
		return
	}
	select {
	case ch <- p:
	default:
	}
}

func sendPrescreenFail(progressChan chan<- Progress, n Node, done, succ, fail *atomic.Int64, total int, msg string) {
	d := done.Add(1)
	f := fail.Add(1)
	sendProgress(progressChan, Progress{
		Total:     total,
		Completed: int(d),
		Success:   int(succ.Load()),
		Failed:    int(f),
		NodeID:    n.ID,
		Latency:   -1,
		ErrMsg:    msg,
	})
}

// testOutbounds 从 channel 读取节点做完整 xray 测速。
func (st *SpeedTest) testOutbounds(
	passedChan <-chan Node,
	progressChan chan<- Progress,
	done, succ, fail *atomic.Int64,
	total int,
) {
	concurrency := st.cfg.Concurrency()
	if concurrency < 1 {
		concurrency = 1
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	for node := range passedChan {
		n := node
		wg.Add(1)
		go func() {
			acquired := false
			finished := false
			defer func() {
				if r := recover(); r != nil && !finished {
					d := done.Add(1)
					f := fail.Add(1)
					sendProgress(progressChan, Progress{
						Total: total, Completed: int(d),
						Success: int(succ.Load()), Failed: int(f),
						NodeID: n.ID, Latency: -1,
						ErrMsg: fmt.Sprintf("speedtest panic: %v", r),
					})
				}
				if acquired {
					<-sem
				}
				wg.Done()
			}()
			sem <- struct{}{}
			acquired = true

			sendProgress(progressChan, Progress{
				Total:     total,
				Completed: int(done.Load()),
				Success:   int(succ.Load()),
				Failed:    int(fail.Load()),
				NodeID:    n.ID,
				Testing:   true,
			})

			result := st.TestOutbound(n.Outbound, n.ID)

			d := done.Add(1)
			finished = true
			if result.Error == "" {
				s := succ.Add(1)
				sendProgress(progressChan, Progress{
					Total: total, Completed: int(d), Success: int(s),
					Failed: int(fail.Load()), NodeID: result.NodeID,
					Latency: result.Latency, Testing: false,
				})
			} else {
				f := fail.Add(1)
				sendProgress(progressChan, Progress{
					Total: total, Completed: int(d),
					Success: int(succ.Load()), Failed: int(f),
					NodeID: result.NodeID,
					Latency: -1, ErrMsg: result.Error, Testing: false,
				})
			}
		}()
	}

	wg.Wait()
}

// extractServerAddr 从 outbound config 提取服务器地址和端口。
// 支持 VLESS/VMess (vnext[0]) 和 Trojan/SS (servers[0]) 两种结构。
func extractServerAddr(outbound types.Map) (string, int) {
	settings := asMap(outbound["settings"])
	if settings == nil {
		return "", 0
	}
	if vnext, ok := settings["vnext"].([]any); ok && len(vnext) > 0 {
		first := asMap(vnext[0])
		if first == nil {
			return "", 0
		}
		addr, _ := first["address"].(string)
		return addr, toInt(first["port"])
	}
	if servers, ok := settings["servers"].([]any); ok && len(servers) > 0 {
		first := asMap(servers[0])
		if first == nil {
			return "", 0
		}
		addr, _ := first["address"].(string)
		return addr, toInt(first["port"])
	}
	return "", 0
}

// extractTLSServerName 从 outbound config 提取 TLS SNI。
// 仅对 security=="tls" 的节点返回 SNI，其他（none/reality/空）返回空。
func extractTLSServerName(outbound types.Map) string {
	streamSettings := asMap(outbound["streamSettings"])
	if streamSettings == nil {
		return ""
	}
	security, _ := streamSettings["security"].(string)
	if security != "tls" {
		return ""
	}
	tlsSettings := asMap(streamSettings["tlsSettings"])
	if tlsSettings == nil {
		return ""
	}
	sni, _ := tlsSettings["serverName"].(string)
	return sni
}

// asMap 将 any 转为 map[string]any，兼容 types.Map（named type）和 DB 反序列化的 unnamed type。
func asMap(v any) map[string]any {
	switch m := v.(type) {
	case map[string]any:
		return m
	case types.Map:
		return map[string]any(m)
	}
	return nil
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

func (st *SpeedTest) getTargetURL() string {
	if u := st.cfg.TargetURL(); u != "" {
		return u
	}
	return defaultTargetURL
}

func (st *SpeedTest) createInstance(outbound types.Map) (*core.Instance, error) {
	cfg := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"outbounds": []any{
			outbound,
			map[string]any{"protocol": "freedom", "tag": "direct"},
		},
	}

	jsonCfg, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	instance, err := core.StartInstance("json", jsonCfg)
	if err != nil {
		return nil, fmt.Errorf("start instance: %w", err)
	}
	return instance, nil
}

// measureDelay 通过独立 xray 实例测速。
// core.Dial 包含完整代理链路（本地→节点→目标），各阶段共享 Client.Timeout 总预算。
func (st *SpeedTest) measureDelay(instance *core.Instance) (int64, error) {
	timeout := st.cfg.Timeout()
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dest, err := xraynet.ParseDestination(fmt.Sprintf("%s:%s", network, addr))
			if err != nil {
				return nil, err
			}
			return core.Dial(ctx, instance, dest)
		},
	}
	defer tr.CloseIdleConnections()
	return st.measureWithClient(tr, timeout, st.getTargetURL())
}

// measureDelayViaProxy 通过主 xray 的 socks 端口测速（网站测速用）。
func (st *SpeedTest) measureDelayViaProxy(socksPort int, targetURL string) (int64, error) {
	timeout := st.cfg.Timeout()
	parsed, err := url.Parse(fmt.Sprintf("socks5://127.0.0.1:%d", socksPort))
	if err != nil {
		return -1, err
	}
	tr := &http.Transport{Proxy: http.ProxyURL(parsed)}
	defer tr.CloseIdleConnections()
	if targetURL == "" {
		targetURL = st.getTargetURL()
	}
	return st.measureWithClient(tr, timeout, targetURL)
}

func (st *SpeedTest) measureWithClient(tr *http.Transport, timeout time.Duration, targetURL string) (int64, error) {
	client := &http.Client{Transport: tr, Timeout: timeout}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, targetURL, nil)
	if err != nil {
		return -1, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()
	// 不读 body：延迟只测 TCP+TLS+首字节，避免 body 大小差异污染延迟值

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return -1, fmt.Errorf("bad status: %d", resp.StatusCode)
	}
	return time.Since(start).Milliseconds(), nil
}
