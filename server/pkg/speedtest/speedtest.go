package speedtest

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	NodeID  uint   `json:"node_id"`
	Latency int64  `json:"latency"`
	Error   string `json:"error,omitempty"`
}

type Progress struct {
	Total     int    `json:"total"`
	Completed int    `json:"completed"`
	Success   int    `json:"success"`
	Failed    int    `json:"failed"`
	NodeID    uint   `json:"node_id,omitempty"`
	Latency   int64  `json:"latency,omitempty"`
	ErrMsg    string `json:"error,omitempty"`
	Testing   bool   `json:"testing"`
}

type Node struct {
	ID             uint
	OutboundConfig types.Map
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
			func() {
				defer func() { _ = recover() }()
				instance.Close()
			}()
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

// dialTCPWithRetry 对目标地址做最多两次 TCP 拨号，返回成功的连接（调用者负责关闭）。
// 第一次超时 800ms；若为超时类失败则重试一次（超时 500ms）。
// 快速失败（connection refused 等）不重试，因为端口明确关闭。
func dialTCPWithRetry(target string) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", target, 800*time.Millisecond)
	if err == nil {
		return conn, nil
	}
	if !isTimeoutError(err) {
		return nil, err
	}
	return net.DialTimeout("tcp", target, 500*time.Millisecond)
}

// isTimeoutError 判断是否为超时类错误（值得重试）。
func isTimeoutError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded)
}

// checkTLS 对已建立的 TCP 连接做 TLS 握手检测，返回握手错误。
// 函数返回时 conn（含底层 TCP）保证已关闭，含 panic 路径。
func checkTLS(conn net.Conn, sni string, timeout time.Duration) error {
	tlsConn := tls.Client(conn, &tls.Config{
		ServerName:         sni,
		InsecureSkipVerify: true,
	})
	defer tlsConn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return tlsConn.HandshakeContext(ctx)
}

// tcpPrescreen 对节点做 TCP+TLS 连通性预筛，通过预筛的节点发送到 passedChan。
// 不可达节点直接标记失败并发送进度，不进入阶段2。
// 对 TLS 节点额外做 TLS 握手检测，快速过滤 TCP 可达但 TLS 不可达的节点。
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
		tlsTimeout      = 1500 * time.Millisecond
	)
	concurrency := st.cfg.Concurrency() * prescreenFactor
	if concurrency > maxPrescreen {
		concurrency = maxPrescreen
	}

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

			addr, port := extractServerAddr(n.OutboundConfig)
			if addr == "" || port == 0 {
				passedChan <- n
				return
			}

			target := fmt.Sprintf("%s:%d", addr, port)
			conn, err := dialTCPWithRetry(target)
			if err != nil {
				sendPrescreenFail(progressChan, n, done, succ, fail, total, fmt.Sprintf("tcp prescreen: %v", err))
				return
			}

			// TCP 可达后，对 TLS 节点额外做握手检测。
			// checkTLS 内部 defer 保证 conn 在所有路径（含 panic）都被关闭，
			// 避免 passedChan 阻塞时 fd 泄漏。
			if sni := extractTLSServerName(n.OutboundConfig); sni != "" {
				if handshakeErr := checkTLS(conn, sni, tlsTimeout); handshakeErr != nil {
					sendPrescreenFail(progressChan, n, done, succ, fail, total, fmt.Sprintf("tls prescreen: %v", handshakeErr))
					return
				}
			} else {
				conn.Close()
			}

			passedChan <- n
		}(node)
	}

	wg.Wait()
}

func sendPrescreenFail(progressChan chan<- Progress, n Node, done, succ, fail *atomic.Int64, total int, msg string) {
	d := done.Add(1)
	f := fail.Add(1)
	if progressChan != nil {
		progressChan <- Progress{
			Total:     total,
			Completed: int(d),
			Success:   int(succ.Load()),
			Failed:    int(f),
			NodeID:    n.ID,
			Latency:   -1,
			ErrMsg:    msg,
		}
	}
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
					if progressChan != nil {
						progressChan <- Progress{
							Total: total, Completed: int(d),
							Success: int(succ.Load()), Failed: int(f),
							NodeID: n.ID, Latency: -1,
							ErrMsg: fmt.Sprintf("speedtest panic: %v", r),
						}
					}
				}
				if acquired {
					<-sem
				}
				wg.Done()
			}()
			sem <- struct{}{}
			acquired = true

			if progressChan != nil {
				progressChan <- Progress{
					Total:     total,
					Completed: int(done.Load()),
					Success:   int(succ.Load()),
					Failed:    int(fail.Load()),
					NodeID:    n.ID,
					Testing:   true,
				}
			}

			result := st.TestOutbound(n.OutboundConfig, n.ID)

			d := done.Add(1)
			if result.Error == "" {
				s := succ.Add(1)
				finished = true
				if progressChan != nil {
					progressChan <- Progress{
						Total: total, Completed: int(d), Success: int(s),
						Failed: int(fail.Load()), NodeID: result.NodeID,
						Latency: result.Latency, Testing: false,
					}
				}
			} else {
				f := fail.Add(1)
				finished = true
				if progressChan != nil {
					progressChan <- Progress{
						Total: total, Completed: int(d),
						Success: int(succ.Load()), Failed: int(f),
						NodeID:  result.NodeID,
						Latency: -1, ErrMsg: result.Error, Testing: false,
					}
				}
			}
		}()
	}

	wg.Wait()
}

// extractServerAddr 从 outbound config 提取服务器地址和端口。
// 支持 VLESS/VMess (vnext[0]) 和 Trojan/SS (servers[0]) 两种结构。
// 注意：用 map[string]any 而非 types.Map 断言，因为 DB 反序列化后
// 嵌套值类型为 map[string]any（unnamed type），types.Map 断言会失败。
func extractServerAddr(outbound types.Map) (string, int) {
	settings, ok := outbound["settings"].(map[string]any)
	if !ok {
		return "", 0
	}
	if vnext, ok := settings["vnext"].([]any); ok && len(vnext) > 0 {
		first, ok := vnext[0].(map[string]any)
		if !ok {
			return "", 0
		}
		addr, _ := first["address"].(string)
		return addr, toInt(first["port"])
	}
	if servers, ok := settings["servers"].([]any); ok && len(servers) > 0 {
		first, ok := servers[0].(map[string]any)
		if !ok {
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
	streamSettings, ok := outbound["streamSettings"].(map[string]any)
	if !ok {
		return ""
	}
	security, _ := streamSettings["security"].(string)
	if security != "tls" {
		return ""
	}
	tlsSettings, ok := streamSettings["tlsSettings"].(map[string]any)
	if !ok {
		return ""
	}
	sni, _ := tlsSettings["serverName"].(string)
	return sni
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

func (st *SpeedTest) measureDelay(instance *core.Instance) (int64, error) {
	timeout := st.cfg.Timeout()
	tr := &http.Transport{
		TLSHandshakeTimeout: timeout,
		DisableKeepAlives:   true,
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

func (st *SpeedTest) measureDelayViaProxy(socksPort int, targetURL string) (int64, error) {
	timeout := st.cfg.Timeout()
	proxyURL := fmt.Sprintf("socks5://127.0.0.1:%d", socksPort)
	tr := &http.Transport{
		TLSHandshakeTimeout: timeout,
		DisableKeepAlives:   true,
		Proxy:               func(*http.Request) (*url.URL, error) { return url.Parse(proxyURL) },
	}
	defer tr.CloseIdleConnections()
	if targetURL == "" {
		return st.measureWithClient(tr, timeout, st.getTargetURL())
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

	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return -1, fmt.Errorf("bad status: %d", resp.StatusCode)
	}
	return time.Since(start).Milliseconds(), nil
}
