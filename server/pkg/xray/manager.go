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
	stopTimeout = 3 * time.Second
	startupWait = 3 * time.Second
	staleWait   = 2 * time.Second
)

type LogCallback func(level, line string)

// Options 固定 Manager 实例身份：user 与 root 是同一 Manager 的两个配置实例。
// root 模式经 sudo fork 起真实 xray，停止与孤儿清理按真实 pid 处理（见 manager_root.go）。
type Options struct {
	Binary   string
	AssetDir string
	AsRoot   bool
	LogFile  string      // 实例独立日志文件
	PidFile  string      // 非空时记录直系子进程 pid（孤儿清理）
	OnLog    LogCallback // 每行日志回调（已分级）
}

type Manager struct {
	opts     Options
	proc     *process.Process
	rootPID  int // root 模式：sudo fork 出的真实 xray pid；<=0 表示未捕获
	mu       sync.Mutex
	onCrash  func()
	stopping atomic.Bool // true = 主动停止或启动失败，不算崩溃
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
	m.stopping.Store(false)

	if err := m.awaitStartup(proc, configPath); err != nil {
		m.stopping.Store(true) // 启动失败不算崩溃
		_ = proc.Stop(stopTimeout)
		return err
	}
	if m.opts.AsRoot {
		m.rootPID = captureRootPID(configPath) // 真实 xray 就绪后按 ruid 捕获一次
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

// awaitStartup 启动稳定期：进程提前退出视为失败，socks 端口可连视为就绪，超时放宽。
func (m *Manager) awaitStartup(proc *process.Process, configPath string) error {
	port := socksPortOf(configPath)
	deadline := time.Now().Add(startupWait)
	for time.Now().Before(deadline) {
		if !proc.Running() {
			return fmt.Errorf("xray exited during startup")
		}
		if port > 0 && portDialable(port) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if proc.Running() {
		return nil
	}
	return fmt.Errorf("xray exited during startup")
}

// monitor 监听真实进程退出。root 模式轮询捕获的真实 xray pid（sudo 前端退出
// 不等于真实 xray 停止），user 模式直接等直系子进程退出；随后收尾并触发 onCrash。
func (m *Manager) monitor(proc *process.Process) {
	if m.opts.AsRoot {
		m.monitorRoot(proc)
		return
	}
	<-proc.Exited()
	m.finish(proc)
}

// monitorRoot root 模式：轮询真实 xray pid 存活。未捕获到 pid 时退化为等 sudo 前端退出。
func (m *Manager) monitorRoot(proc *process.Process) {
	if m.rootPID > 0 {
		for rootAlive(m.rootPID) {
			time.Sleep(500 * time.Millisecond)
		}
	} else {
		<-proc.Exited()
	}
	m.finish(proc)
}

// finish 进程退出收尾：清理 pidFile，意外退出（非主动停止）时触发 onCrash。
// 代际校验（m.proc == proc）防止 Restart 后旧 monitor 把新进程误判为崩溃。
func (m *Manager) finish(proc *process.Process) {
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

	if m.opts.AsRoot {
		m.stopping.Store(true)
		return m.stopRoot(m.proc)
	}
	if m.proc == nil || !m.proc.Running() {
		return nil
	}
	m.stopping.Store(true)
	return m.proc.Stop(stopTimeout)
}

func (m *Manager) Restart(configPath string) error {
	if err := m.Stop(); err != nil {
		return fmt.Errorf("stop before restart: %w", err)
	}
	return m.Start(configPath)
}

func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.proc != nil && m.proc.Running()
}

// CleanupStale 清理上次会话残留的孤儿进程（vxray 异常退出时子进程无人回收）。
// root 模式按 configPath 精确匹配真实 xray pid（见 manager_root.go）；user 模式
// 按 pidFile + 进程组收尾。
func (m *Manager) CleanupStale(configPath string) {
	if m.opts.AsRoot {
		m.cleanupStaleRoot(configPath)
		return
	}
	m.cleanupStaleUser()
}

func (m *Manager) cleanupStaleUser() {
	if m.opts.PidFile == "" {
		return
	}
	pid := m.readPidFile()
	if pid <= 0 || !pidAlive(pid) || !isXrayProcess(pid) {
		_ = os.Remove(m.opts.PidFile)
		return
	}
	_ = syscall.Kill(pid, syscall.SIGTERM)
	if !waitForExit(pid, staleWait) {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
	_ = os.Remove(m.opts.PidFile)
}

// readPidFile 读取 pidFile 记录的 pid；文件缺失或内容非法时返回 0（非法则删文件）。
func (m *Manager) readPidFile() int {
	if m.opts.PidFile == "" {
		return 0
	}
	data, err := os.ReadFile(m.opts.PidFile)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		_ = os.Remove(m.opts.PidFile)
		return 0
	}
	return pid
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

func pidAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// isXrayProcess 按 argv 判断：user 模式 "xray run -c ..."，root 模式 sudo 前端的
// "sudo -n <path>/xray run -c ..."，两者均含 "xray run"。
func isXrayProcess(pid int) bool {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "args=").Output()
	return err == nil && strings.Contains(string(out), "xray run")
}

func waitForExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !pidAlive(pid) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return !pidAlive(pid)
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

// socksPortOf 提取配置中首个 socks inbound 端口，用于启动就绪探测；无则返回 0。
func socksPortOf(configPath string) int {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return 0
	}
	var cfg struct {
		Inbounds []struct {
			Protocol string `json:"protocol"`
			Port     any    `json:"port"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return 0
	}
	for _, in := range cfg.Inbounds {
		if in.Protocol != "socks" {
			continue
		}
		switch p := in.Port.(type) {
		case float64:
			return int(p)
		case string:
			if n, err := strconv.Atoi(p); err == nil {
				return n
			}
		}
	}
	return 0
}

func portDialable(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}