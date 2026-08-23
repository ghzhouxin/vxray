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
}

// Node 携带测速 outbound 与 TCP 预筛所需的结构化字段，
// 免去从 outbound map 反向解析 addr/port/SNI。
type Node struct {
	ID       uint
	Outbound types.Map
	Addr     string // 预筛直连地址；空则跳过预筛
	Port     int
	TLSHost  string // security==tls 时为 SNI，否则空
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

// testRun 汇总一次批量测速的共享状态：每个节点完成后通过 onResult 交付结果
// （保证不丢失），进度经 progressChan 有损推送（仅供 UI 展示），
// done/succ/fail 维护汇总计数。
type testRun struct {
	progress chan<- Progress
	onResult func(Result)
	total    int
	done     atomic.Int64
	succ     atomic.Int64
	fail     atomic.Int64
}

// record 交付单节点测速结果并推送进度事件。
func (r *testRun) record(result Result) {
	if r.onResult != nil {
		r.onResult(result)
	}

	p := Progress{
		Total:     r.total,
		Completed: int(r.done.Add(1)),
		NodeID:    result.NodeID,
		Latency:   result.Latency,
		ErrMsg:    result.Error,
	}
	if result.Error == "" {
		p.Success = int(r.succ.Add(1))
		p.Failed = int(r.fail.Load())
	} else {
		p.Success = int(r.succ.Load())
		p.Failed = int(r.fail.Add(1))
	}

	if r.progress != nil {
		select {
		case r.progress <- p:
		default:
		}
	}
}

// TestNodes 并发测速所有节点，每个节点完成后调用 onResult（保证不丢失），
// 进度事件写入 progressChan（有损，仅供 UI 展示）。
func (st *SpeedTest) TestNodes(nodes []Node, progressChan chan<- Progress, onResult func(Result)) {
	defer func() {
		if progressChan != nil {
			close(progressChan)
		}
	}()

	if len(nodes) == 0 {
		return
	}
	run := &testRun{progress: progressChan, onResult: onResult, total: len(nodes)}

	// 单节点：直接测，跳过预筛（预筛握手与主测速对同一服务器重复）
	if len(nodes) == 1 {
		n := nodes[0]
		run.record(*st.TestOutbound(n.Outbound, n.ID))
		return
	}

	// 流水线：阶段1 TCP 预筛 → 阶段2 xray 测速，通过 channel 连接，
	// 阶段1通过的节点立即送入阶段2，无需等待全部预筛完成
	passedChan := make(chan Node, st.cfg.Concurrency())
	go func() {
		st.tcpPrescreen(nodes, run, passedChan)
		close(passedChan)
	}()
	st.testOutbounds(run, passedChan)
}

// tcpPrescreen 对节点做 TCP+TLS 连通性预筛，通过的送入 passedChan，失败的记入 run。
// 预筛预算 = 主测速超时 × 3/4，严格保证预筛最差耗时低于主测速超时。
func (st *SpeedTest) tcpPrescreen(nodes []Node, run *testRun, passedChan chan<- Node) {
	const (
		prescreenFactor = 2
		maxPrescreen    = 256
	)
	concurrency := min(st.cfg.Concurrency()*prescreenFactor, maxPrescreen)
	budget := st.cfg.Timeout() * 3 / 4

	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	for _, node := range nodes {
		wg.Add(1)
		go func(n Node) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := prescreenNode(n, budget); err != nil {
				run.record(Result{NodeID: n.ID, Latency: -1, Error: err.Error()})
				return
			}
			passedChan <- n
		}(node)
	}
	wg.Wait()
}

// prescreenNode 做 TCP+TLS 连通性预筛，通过返回 nil。
func prescreenNode(n Node, budget time.Duration) error {
	if n.Addr == "" || n.Port == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()

	// defer conn.Close() 统一覆盖 TLS 与非 TLS 路径，避免 fd 泄漏。
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(n.Addr, fmt.Sprintf("%d", n.Port)))
	if err != nil {
		return fmt.Errorf("tcp prescreen: %w", err)
	}
	defer conn.Close()

	if n.TLSHost != "" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: n.TLSHost, InsecureSkipVerify: true})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return fmt.Errorf("tls prescreen: %w", err)
		}
	}
	return nil
}

// testOutbounds 从 channel 读取通过预筛的节点，做完整 xray 测速，结果记入 run。
func (st *SpeedTest) testOutbounds(run *testRun, passedChan <-chan Node) {
	sem := make(chan struct{}, max(st.cfg.Concurrency(), 1))

	var wg sync.WaitGroup
	for node := range passedChan {
		n := node
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			run.record(*st.TestOutbound(n.Outbound, n.ID))
		}()
	}
	wg.Wait()
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
