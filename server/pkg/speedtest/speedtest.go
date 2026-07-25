package speedtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"v2ray-server/pkg/httpclient"
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
	GetTargetURL() string
	GetTimeout() time.Duration
	GetConcurrency() int
}

type SpeedTest struct {
	cfg    Config
	client *http.Client
}

func New(cfg Config) *SpeedTest {
	return &SpeedTest{cfg: cfg, client: httpclient.Default()}
}

func (st *SpeedTest) TestOutbound(outbound types.Map, nodeID uint) (result *Result) {
	var instance *core.Instance
	defer func() {
		if instance != nil {
			instance.Close()
		}
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
		return
	}

	concurrency := st.getConcurrency()
	results := make([]Result, len(nodes))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	progress := Progress{Total: len(nodes)}

	for i, node := range nodes {
		wg.Add(1)
		go func(idx int, n Node) {
			acquired := false
			completed := false
			defer func() {
				if r := recover(); r != nil && !completed {
					res := Result{NodeID: n.ID, Latency: -1, Error: fmt.Sprintf("speedtest panic: %v", r)}
					results[idx] = res
					mu.Lock()
					applyResultProgress(&progress, res)
					progCopy := progress
					mu.Unlock()
					if progressChan != nil {
						progressChan <- progCopy
					}
				}
				if acquired {
					<-sem
				}
				wg.Done()
			}()
			sem <- struct{}{}
			acquired = true

			mu.Lock()
			progress.NodeID = n.ID
			progress.Latency = 0
			progress.ErrMsg = ""
			progress.Testing = true
			progCopy := progress
			mu.Unlock()
			if progressChan != nil {
				progressChan <- progCopy
			}

			result := st.TestOutbound(n.OutboundConfig, n.ID)

			results[idx] = *result

			mu.Lock()
			applyResultProgress(&progress, *result)
			completed = true
			progCopy = progress
			mu.Unlock()
			if progressChan != nil {
				progressChan <- progCopy
			}
		}(i, node)
	}

	wg.Wait()
	if progressChan != nil {
		close(progressChan)
	}
}

func applyResultProgress(progress *Progress, result Result) {
	progress.Completed++
	if result.Error == "" {
		progress.Success++
	} else {
		progress.Failed++
	}
	progress.NodeID = result.NodeID
	progress.Latency = result.Latency
	progress.ErrMsg = result.Error
	progress.Testing = false
}

func (st *SpeedTest) getTargetURL() string {
	if url := st.cfg.GetTargetURL(); url != "" {
		return url
	}
	return defaultTargetURL
}

func (st *SpeedTest) getTimeout() time.Duration {
	return st.cfg.GetTimeout()
}

func (st *SpeedTest) getConcurrency() int {
	return st.cfg.GetConcurrency()
}

func (st *SpeedTest) createInstance(outbound types.Map) (*core.Instance, error) {
	port, err := getAvailablePort()
	if err != nil {
		return nil, fmt.Errorf("get available port: %w", err)
	}

	cfg := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"inbounds": []any{
			map[string]any{
				"port":     port,
				"protocol": "socks",
				"settings": map[string]any{"auth": "noauth", "udp": true},
			},
		},
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
	timeout := st.getTimeout()
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
	return st.doMeasureWithTransport(tr, timeout)
}

func (st *SpeedTest) measureDelayViaProxy(socksPort int, targetURL string) (int64, error) {
	timeout := st.getTimeout()
	proxyURL := fmt.Sprintf("socks5://127.0.0.1:%d", socksPort)
	tr := &http.Transport{
		TLSHandshakeTimeout: timeout,
		DisableKeepAlives:   true,
		Proxy:               func(*http.Request) (*url.URL, error) { return url.Parse(proxyURL) },
	}
	if targetURL == "" {
		return st.doMeasureWithTransport(tr, timeout)
	}
	return st.doMeasureWithTransportAndTarget(tr, timeout, targetURL)
}

func (st *SpeedTest) doMeasureWithTransport(tr *http.Transport, timeout time.Duration) (int64, error) {
	return st.doMeasureWithTransportAndTarget(tr, timeout, st.getTargetURL())
}

func (st *SpeedTest) doMeasureWithTransportAndTarget(tr *http.Transport, timeout time.Duration, targetURL string) (int64, error) {
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

func getAvailablePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	addr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected listener address type: %T", l.Addr())
	}
	return addr.Port, nil
}
