package xray

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

type Config interface {
	XrayBinary() string
	XrayConfigPath() string
	GeoDir() string
}

type LogCallback func(level, message string)

// CrashCallback 是 xray 崩溃时的回调（由监控 goroutine 调用）。
type CrashCallback func()

type Manager struct {
	cmd           *exec.Cmd
	running       bool
	logCallback   LogCallback
	crashCallback CrashCallback
	cfg           Config
	doneMonitor   chan struct{} // monitor goroutine 退出完成通知（由 monitorProcess close）
	mu            sync.RWMutex
}

func NewManager(cfg Config) *Manager {
	return &Manager{cfg: cfg}
}

func (m *Manager) SetLogCallback(cb LogCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logCallback = cb
}

// SetCrashCallback 注册崩溃回调。
func (m *Manager) SetCrashCallback(cb CrashCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.crashCallback = cb
}

func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("xray is already running")
	}

	return m.startProcess()
}

func (m *Manager) ForceStart() error {
	m.mu.Lock()
	doneMonitor := m.stopProcess()
	m.mu.Unlock()
	if doneMonitor != nil {
		<-doneMonitor
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startProcess()
}

func (m *Manager) startProcess() error {
	ctx := context.Background()
	m.cmd = exec.CommandContext(ctx, m.cfg.XrayBinary(), "run", "-c", m.cfg.XrayConfigPath())
	m.cmd.Env = append(os.Environ(), "XRAY_LOCATION_ASSET="+m.cfg.GeoDir())

	stdout, err := m.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := m.cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := m.cmd.Start(); err != nil {
		return err
	}

	m.running = true
	m.doneMonitor = make(chan struct{})

	go m.safeReadOutput(stdout)
	go m.safeReadOutput(stderr)
	go m.monitorProcess()

	return nil
}

// stopProcess 终止进程并标记 running=false。
// 调用方需持有 m.mu；返回的通道（若非 nil）必须等调用方释放 m.mu 后再等待，
// 否则 monitor goroutine 无法获取 m.mu 来结束并 close 通道，造成死锁。
func (m *Manager) stopProcess() <-chan struct{} {
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
	}
	// 标记为非崩溃退出，monitor 检测到 running=false 后直接返回
	m.running = false
	doneMonitor := m.doneMonitor
	m.doneMonitor = nil
	m.cmd = nil
	return doneMonitor
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	doneMonitor := m.stopProcess()
	m.mu.Unlock()
	if doneMonitor != nil {
		<-doneMonitor
	}
	return nil
}

// monitorProcess 通过 Wait() 阻塞等待 xray 进程退出。
// 进程退出后：若是正常 Stop（running 已被置 false）则静默返回；
// 若是意外崩溃（running 仍为 true）则触发 crashCallback。
func (m *Manager) monitorProcess() {
	defer close(m.doneMonitor)

	if m.cmd != nil && m.cmd.Process != nil {
		_, _ = m.cmd.Process.Wait()
	}

	m.mu.Lock()
	if !m.running {
		// 正常 Stop：running 已被 stopProcess 置 false
		m.mu.Unlock()
		return
	}
	// 意外崩溃
	m.running = false
	cb := m.crashCallback
	m.mu.Unlock()

	if cb != nil {
		cb()
	}
}

func (m *Manager) safeReadOutput(reader io.Reader) {
	defer func() {
		if r := recover(); r != nil && m.logCallback != nil {
			m.logCallback("error", fmt.Sprintf("xray output reader panic: %v", r))
		}
	}()
	m.readOutput(reader)
}

func (m *Manager) readOutput(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if m.logCallback != nil {
			m.logCallback(classifyLogLevel(line), line)
		}
	}
}
