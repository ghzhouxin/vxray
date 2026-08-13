package xray

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"v2ray-server/pkg/process"
)

const (
	stopTimeout   = 3 * time.Second
	startupWait   = 3 * time.Second
	staleKillWait = 2 * time.Second
)

type LogCallback func(level, line string)

// Options 构造期固定实例身份：user 与 root 是同一 Manager 的两个不同配置实例。
// root 模式 spawn `sudo -n xray run -c <config>`：sudo 会 fork，直接子进程是
// sudo monitor（用户态），xray 是其下 uid 0 的孙进程——pidFile 记 sudo pid，
// SIGTERM 经 monitor 转发生效，强杀需 `sudo -n /bin/kill`（sudoers 需授权）。
type Options struct {
	Binary   string
	AssetDir string
	AsRoot   bool
	LogFile  string // 实例独立日志文件
	PidFile  string // 非空时跟踪 pid（孤儿清理）
	OnLog    LogCallback // 每行日志回调（已分级）
}

type Manager struct {
	opts       Options
	proc       *process.Process
	configPath string // root 模式停止时按真实 pid 杀，需配置路径
	mu         sync.Mutex
	onCrash    func()
	stopping   atomic.Bool // true = 主动停止或启动失败，不算崩溃
}

func NewManager(opts Options) *Manager {
	return &Manager{opts: opts}
}

// SetCrashCallback 注入崩溃回调，须在 Start 前调用（service 层依赖 manager，构造顺序相反）。
func (m *Manager) SetCrashCallback(cb func()) { m.onCrash = cb }

func (m *Manager) Start(configPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.proc != nil && m.proc.Running() {
		return fmt.Errorf("xray already running (pid %d)", m.proc.PID())
	}

	bin, args, err := m.command(configPath)
	if err != nil {
		return err
	}

	var env []string
	if m.opts.AssetDir != "" {
		env = append(env, "XRAY_LOCATION_ASSET="+m.opts.AssetDir)
	}

	var onLine process.LogCallback
	if m.opts.OnLog != nil {
		cb := m.opts.OnLog
		onLine = func(line string) {
			cb(classifyLogLevel(line), line)
		}
	}

	proc := process.New(process.Options{
		Binary:  bin,
		Args:    args,
		Env:     env,
		LogFile: m.opts.LogFile,
		OnLine:  onLine,
	})

	if err := proc.Start(); err != nil {
		return err
	}
	m.proc = proc
	m.configPath = configPath
	m.stopping.Store(false)

	if err := m.awaitStartup(proc, configPath); err != nil {
		m.stopping.Store(true) // 启动失败不算崩溃
		_ = proc.Stop(stopTimeout)
		return err
	}

	if m.opts.PidFile != "" {
		_ = os.MkdirAll(filepath.Dir(m.opts.PidFile), 0755)
		_ = os.WriteFile(m.opts.PidFile, []byte(strconv.Itoa(proc.PID())), 0644)
	}

	go m.monitor(proc)
	return nil
}

func (m *Manager) command(configPath string) (string, []string, error) {
	if !m.opts.AsRoot {
		return m.opts.Binary, []string{"run", "-c", configPath}, nil
	}
	xrayPath, err := exec.LookPath(m.opts.Binary)
	if err != nil {
		return "", nil, fmt.Errorf("resolve xray binary: %w", err)
	}
	return "sudo", []string{"-n", xrayPath, "run", "-c", configPath}, nil
}

// awaitStartup 启动稳定期：进程退出即失败；socks 端口可连即成功；超时视为成功。
func (m *Manager) awaitStartup(proc *process.Process, configPath string) error {
	port, _ := socksPortOf(configPath)
	deadline := time.Now().Add(startupWait)
	for port <= 0 || !portDialable(port) {
		select {
		case <-proc.Exited():
			return fmt.Errorf("xray exited during startup")
		default:
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	select {
	case <-proc.Exited():
		return fmt.Errorf("xray exited during startup")
	default:
	}
	return nil
}

// monitor 监听进程退出：清理 pidFile，意外退出时触发 onCrash。
// 代际校验（m.proc == proc）防止 Restart 后旧 monitor 把新进程误判为崩溃。
func (m *Manager) monitor(proc *process.Process) {
	<-proc.Exited()
	m.removePidFileIf(proc.PID())
	m.mu.Lock()
	crash := m.proc == proc && !m.stopping.Load() && m.onCrash != nil
	m.mu.Unlock()
	if crash {
		go m.onCrash()
	}
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.proc == nil || !m.proc.Running() {
		return nil
	}
	m.stopping.Store(true)
	if m.opts.AsRoot {
		return m.stopRoot(m.proc)
	}
	return m.proc.Stop(stopTimeout)
}

func (m *Manager) Restart(configPath string) error {
	if err := m.Stop(); err != nil {
		return fmt.Errorf("stop before restart: %w", err)
	}
	return m.Start(configPath)
}

// stopRoot 停止 root xray：以真实 xray pid 死亡为唯一成功标准。
// sudo 的 use_pty 会将 xray 放入独立会话，按进程组杀永远摸不到——
// 这里用 pgrep -u 0 找真实 pid，逐 pid 精确 sudo kill。
func (m *Manager) stopRoot(proc *process.Process) error {
	if m.configPath == "" {
		return proc.Stop(stopTimeout)
	}
	pids := rootXrayPids(m.configPath)
	if len(pids) == 0 {
		// xray 已死，收尾 monitor 即可
		return proc.Stop(1 * time.Second)
	}

	// 优雅停止：SIGTERM 经 sudo 前端转发给 xray
	_ = syscall.Kill(proc.PID(), syscall.SIGTERM)

	// 等真实 xray 退出，200ms 轮询，上限 5s（安全网，正常 2-3s）
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pids = rootXrayPids(m.configPath)
		if len(pids) == 0 {
			return proc.Stop(1 * time.Second)
		}
		time.Sleep(200 * time.Millisecond)
	}

	// 强杀兜底：sudo kill -9 精确 pid，不按组杀
	pids = rootXrayPids(m.configPath)
	for _, pid := range pids {
		_ = exec.Command("sudo", "-n", "/bin/kill", "-9", strconv.Itoa(pid)).Run()
	}
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(rootXrayPids(m.configPath)) == 0 {
			return proc.Stop(1 * time.Second)
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("root xray survived SIGKILL: %v", rootXrayPids(m.configPath))
}

// rootXrayPids 返回匹配 config 路径的 root 属主 xray 真实 pid。
// pgrep -u 0 只匹配 uid 0 进程，排除 sudo 前端（uid 501）。
func rootXrayPids(configPath string) []int {
	out, err := exec.Command("pgrep", "-u", "0", "-f",
		"xray run -c "+configPath).Output()
	if err != nil {
		// pgrep 无匹配时 exit 1，不是错误
		return nil
	}
	var pids []int
	for _, s := range strings.Fields(string(out)) {
		if pid, err := strconv.Atoi(s); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

// WaitPortClosed 轮询直到端口不可连或超时，用于 Disable 后等端口释放。
func WaitPortClosed(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !portDialable(port) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return !portDialable(port)
}

func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.proc != nil && m.proc.Running()
}

// CleanupStale 清理上次会话残留的孤儿进程（vxray 异常退出时子进程无人回收）。
// root 模式：pgrep 找真实 xray pid，sudo kill 逐 pid 强杀。
func (m *Manager) CleanupStale() {
	if m.opts.PidFile == "" {
		return
	}
	data, err := os.ReadFile(m.opts.PidFile)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		_ = os.Remove(m.opts.PidFile)
		return
	}

	if m.opts.AsRoot {
		// root 模式：pgrep 找真实 xray pid，精确 sudo kill
		rootPids := rootXrayPids(m.configPath)
		for _, rp := range rootPids {
			_ = syscall.Kill(pid, syscall.SIGTERM) // monitor 转发
			if !waitForExit(rp, staleKillWait) {
				_ = exec.Command("sudo", "-n", "/bin/kill", "-9", strconv.Itoa(rp)).Run()
			}
		}
		// 收尾 monitor
		if processAlive(pid) && isXrayProcess(pid) {
			_ = syscall.Kill(pid, syscall.SIGTERM)
			waitForExit(pid, staleKillWait)
		}
		_ = os.Remove(m.opts.PidFile)
		return
	}

	// user 模式：按进程组杀
	if !processAlive(pid) || !isXrayProcess(pid) {
		_ = os.Remove(m.opts.PidFile)
		return
	}
	_ = syscall.Kill(pid, syscall.SIGTERM)
	if !waitForExit(pid, staleKillWait) {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
	_ = os.Remove(m.opts.PidFile)
}

// removePidFileIf 仅当 pidFile 内容与 pid 一致时删除，避免删掉新一轮启动写入的 pid。
func (m *Manager) removePidFileIf(pid int) {
	if m.opts.PidFile == "" {
		return
	}
	data, err := os.ReadFile(m.opts.PidFile)
	if err != nil {
		return
	}
	if strings.TrimSpace(string(data)) == strconv.Itoa(pid) {
		_ = os.Remove(m.opts.PidFile)
	}
}

func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// isXrayProcess 按 argv 判断：user 模式 "xray run -c ..."，root 模式
// sudo monitor 的 "sudo -n <path>/xray run -c ..."，两者均含 "xray run"。
func isXrayProcess(pid int) bool {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "args=").Output()
	return err == nil && strings.Contains(string(out), "xray run")
}

func waitForExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return !processAlive(pid)
}

// socksPortOf 提取配置中首个 socks inbound 端口，用于启动就绪探测。
func socksPortOf(configPath string) (int, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return 0, err
	}
	var cfg struct {
		Inbounds []struct {
			Protocol string `json:"protocol"`
			Port     any    `json:"port"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return 0, err
	}
	for _, in := range cfg.Inbounds {
		if in.Protocol != "socks" {
			continue
		}
		switch p := in.Port.(type) {
		case float64:
			return int(p), nil
		case string:
			if n, err := strconv.Atoi(p); err == nil {
				return n, nil
			}
		}
	}
	return 0, fmt.Errorf("no socks inbound in %s", configPath)
}

func portDialable(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
