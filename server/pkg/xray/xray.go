package xray

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type Config interface {
	XrayBinary() string
	XrayConfigPath() string
	GeoDir() string
}

type LogCallback func(level, message string)

type Manager struct {
	cmd         *exec.Cmd
	running     bool
	logCallback LogCallback
	cfg         Config
	mu          sync.RWMutex
}

func NewManager(cfg Config) *Manager {
	return &Manager{cfg: cfg}
}

func (m *Manager) SetLogCallback(cb LogCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logCallback = cb
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
	defer m.mu.Unlock()

	m.stopProcess()
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

	go m.safeReadOutput(stdout)
	go m.safeReadOutput(stderr)

	return nil
}

func (m *Manager) stopProcess() {
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
		_, _ = m.cmd.Process.Wait()
		m.cmd = nil
	}
	m.running = false
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopProcess()
	return nil
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
			lower := strings.ToLower(line)
			level := "debug"
			switch {
			case strings.Contains(lower, "error"):
				level = "error"
			case strings.Contains(lower, "warn"):
				level = "warn"
			case strings.Contains(lower, "info"):
				level = "info"
			}
			m.logCallback(level, line)
		}
	}
}
