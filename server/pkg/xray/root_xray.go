package xray

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RootXray 管理 root 权限的 xray 子进程（TUN 模式）。
//
// 通过 sudoers NOPASSWD 规则（一次性安装）免密 sudo 启停 xray。
// 替代旧 SudoManager + SupervisorClient：无 osascript 弹窗、无 IPC、无 supervisor 子进程。
//
// 生命周期跟随 vxray 会话：vxray 退出时调用 Stop() 杀 root xray。
// 崩溃检测通过轮询 kill -0 <pid>（sudo fork 的进程非 vxray 子进程，无法用 cmd.Wait）。
type RootXray struct {
	cfg     Config
	pidPath string
	logPath string

	mu            sync.Mutex
	pid           int
	stopInFlight  bool
	logCallback   LogCallback
	crashCallback CrashCallback
	stopTail      chan struct{}
	tailDone      chan struct{}
	stopMonitor   chan struct{}
	monitorDone   chan struct{}
}

// NewRootXray 构造 RootXray。pidPath/logPath 显式传入，避免 pkg/xray → internal/config 循环依赖。
func NewRootXray(cfg Config, pidPath, logPath string) *RootXray {
	return &RootXray{
		cfg:     cfg,
		pidPath: pidPath,
		logPath: logPath,
	}
}

func (r *RootXray) SetLogCallback(cb LogCallback) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logCallback = cb
}

func (r *RootXray) SetCrashCallback(cb CrashCallback) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.crashCallback = cb
}

// IsRunning 返回 root xray 是否在运行。
func (r *RootXray) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pid > 0 && r.pidAlive(r.pid)
}

// root xray 启停相关常量
const (
	rootLivenessWait      = 1 * time.Second        // 启动后存活检查等待
	rootSigtermWait       = 2 * time.Second        // SIGTERM 后等待退出的超时
	rootCrashPollInterval = 500 * time.Millisecond // 崩溃检测轮询间隔
	rootLogPollInterval   = 100 * time.Millisecond // 日志 tail 轮询间隔
)

// Start 确保 sudoers 规则已安装（首次会弹窗），然后用 sudo -n 启动 root xray。
// configPath 是 TUN 配置文件路径（已注入 tun inbound）。
func (r *RootXray) Start(configPath string) error {
	r.mu.Lock()
	if r.pid > 0 {
		r.mu.Unlock()
		return fmt.Errorf("root xray already running (pid %d)", r.pid)
	}
	r.mu.Unlock()

	binary := r.cfg.XrayBinary()
	geo := r.cfg.GeoDir()

	// 解析二进制绝对路径：config 中可能是裸名 "xray"（靠 PATH 查找），
	// 需用 exec.LookPath 解析（和 exec.Command 一致），不能用 filepath.Abs
	// （会把裸名解析成 <CWD>/xray，导致 sudo: command not found）。
	resolvedBinary, err := resolveBinaryPath(binary)
	if err != nil {
		return fmt.Errorf("resolve xray binary %q: %w", binary, err)
	}
	absConfig, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("resolve config: %w", err)
	}

	// 1. 确保 sudoers 规则（首次会弹 osascript，之后永久免密）
	if err := ensureSudoersRule(resolvedBinary, absConfig); err != nil {
		return fmt.Errorf("ensure sudoers: %w", err)
	}

	// 3. 清理残留的日志/PID 文件：旧 supervisor 代码可能留下 root 所有的 tun.log，
	//    导致用户态 shell 的 > 重定向因权限不足失败，xray 根本无法启动。
	//    目录由用户所有，可删除其中的 root 文件。
	_ = os.Remove(r.logPath)
	_ = os.Remove(r.pidPath)

	// 4. sudo -n 启动 xray，重定向 stdout/stderr 到日志文件，后台运行
	//    用 shell 的 > 重定向和 & 后台；通过 $! 捕获 PID 写入 pid 文件
	shellCmd := fmt.Sprintf(
		`XRAY_LOCATION_ASSET=%s sudo -n %s run -c %s > %s 2>&1 & echo $! > %s`,
		shellQuoteSudoers(geo), shellQuoteSudoers(resolvedBinary),
		shellQuoteSudoers(absConfig), shellQuoteSudoers(r.logPath),
		shellQuoteSudoers(r.pidPath),
	)
	cmd := exec.Command("sh", "-c", shellCmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("spawn root xray: %w", err)
	}

	// 4. 读取 PID
	pid, err := r.readPid()
	if err != nil {
		return fmt.Errorf("read pid: %w", err)
	}

	// 5. 存活检查（1 秒）
	time.Sleep(rootLivenessWait)
	if !r.pidAlive(pid) {
		tail := r.readLogTail(20)
		return fmt.Errorf("root xray died within 1s of startup; log tail:\n%s", tail)
	}

	// 6. 启动日志 tail 和崩溃监控
	r.mu.Lock()
	r.pid = pid
	r.stopInFlight = false
	r.stopTail = make(chan struct{})
	r.tailDone = make(chan struct{})
	r.stopMonitor = make(chan struct{})
	r.monitorDone = make(chan struct{})
	r.mu.Unlock()

	go r.tailLog()
	go r.monitorCrash()

	return nil
}

// Stop 用 sudo -n kill 停止 root xray。无弹窗。
func (r *RootXray) Stop() error {
	r.mu.Lock()
	pid := r.pid
	if pid == 0 {
		r.mu.Unlock()
		return nil
	}
	r.stopInFlight = true
	r.mu.Unlock()

	// SIGTERM
	_ = exec.Command("sudo", "-n", "kill", strconv.Itoa(pid)).Run()

	// 等待退出（轮询 kill -0）
	deadline := time.Now().Add(rootSigtermWait)
	for time.Now().Before(deadline) {
		if !r.pidAlive(pid) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// 仍未退出 → SIGKILL
	if r.pidAlive(pid) {
		_ = exec.Command("sudo", "-n", "kill", "-9", strconv.Itoa(pid)).Run()
		sigkillDeadline := time.Now().Add(time.Second)
		for time.Now().Before(sigkillDeadline) {
			if !r.pidAlive(pid) {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	// 停止 tail 和 monitor goroutine
	r.mu.Lock()
	stopTail := r.stopTail
	tailDone := r.tailDone
	stopMonitor := r.stopMonitor
	monitorDone := r.monitorDone
	r.stopTail = nil
	r.tailDone = nil
	r.stopMonitor = nil
	r.monitorDone = nil
	r.pid = 0
	r.mu.Unlock()

	if stopTail != nil {
		close(stopTail)
		<-tailDone
	}
	if stopMonitor != nil {
		close(stopMonitor)
		<-monitorDone
	}

	_ = os.Remove(r.pidPath)
	return nil
}

// Restart 停止并重新启动 root xray。无弹窗。
func (r *RootXray) Restart(configPath string) error {
	if err := r.Stop(); err != nil {
		return fmt.Errorf("stop: %w", err)
	}
	return r.Start(configPath)
}

// CleanupStale 在 vxray 启动时清理可能残留的 root xray 进程。
// 读 PID 文件，若进程存在则 sudo -n kill。无弹窗。
func (r *RootXray) CleanupStale() error {
	pid, err := r.readPid()
	if err != nil {
		_ = os.Remove(r.pidPath)
		return nil
	}
	if !r.pidAlive(pid) {
		_ = os.Remove(r.pidPath)
		return nil
	}
	_ = exec.Command("sudo", "-n", "kill", strconv.Itoa(pid)).Run()
	time.Sleep(500 * time.Millisecond)
	if r.pidAlive(pid) {
		_ = exec.Command("sudo", "-n", "kill", "-9", strconv.Itoa(pid)).Run()
	}
	_ = os.Remove(r.pidPath)
	return nil
}

// tailLog 持续读取日志文件新内容，按行转发到 logCallback。
func (r *RootXray) tailLog() {
	// 在启动时捕获 done 通道引用，避免 Stop() 将 r.tailDone 置 nil 后 defer 无法 close
	r.mu.Lock()
	done := r.tailDone
	r.mu.Unlock()
	defer func() {
		if done != nil {
			close(done)
		}
	}()

	f, err := os.Open(r.logPath)
	if err != nil {
		return
	}
	defer f.Close()
	// 跳到文件末尾（启动时的历史日志已在 Start 的存活检查中处理）
	_, _ = f.Seek(0, io.SeekEnd)

	reader := bufio.NewReader(f)
	lineBuf := make([]byte, 0, 64*1024)

	for {
		r.mu.Lock()
		stop := r.stopTail
		cb := r.logCallback
		r.mu.Unlock()
		if stop == nil {
			return
		}

		// 读取一行
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimRight(line, "\n")
			if cb != nil {
				cb(classifyLogLevel(line), line)
			}
			lineBuf = lineBuf[:0]
		}
		if err != nil {
			if err == io.EOF {
				// 无新数据，等待或退出
				select {
				case <-stop:
					return
				case <-time.After(rootLogPollInterval):
				}
			} else {
				return
			}
		}
	}
}

// monitorCrash 轮询 kill -0 检测 root xray 崩溃。
func (r *RootXray) monitorCrash() {
	// 在启动时捕获 done 通道引用，避免 Stop() 将 r.monitorDone 置 nil 后 defer 无法 close
	r.mu.Lock()
	done := r.monitorDone
	r.mu.Unlock()
	defer func() {
		if done != nil {
			close(done)
		}
	}()

	ticker := time.NewTicker(rootCrashPollInterval)
	defer ticker.Stop()

	for {
		r.mu.Lock()
		stop := r.stopMonitor
		pid := r.pid
		stopInFlight := r.stopInFlight
		cb := r.crashCallback
		r.mu.Unlock()

		if stop == nil {
			return
		}

		if !r.pidAlive(pid) {
			if !stopInFlight && cb != nil {
				cb()
			}
			return
		}

		select {
		case <-stop:
			return
		case <-ticker.C:
		}
	}
}

// pidAlive 用 sudo -n kill -0 检查 PID 是否存活。
func (r *RootXray) pidAlive(pid int) bool {
	cmd := exec.Command("sudo", "-n", "kill", "-0", strconv.Itoa(pid))
	return cmd.Run() == nil
}

// readPid 从 PID 文件读取 PID。
func (r *RootXray) readPid() (int, error) {
	data, err := os.ReadFile(r.pidPath)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid pid in %s: %s", r.pidPath, string(data))
	}
	return pid, nil
}

// readLogTail 读取日志文件最后 n 行。
func (r *RootXray) readLogTail(n int) string {
	f, err := os.Open(r.logPath)
	if err != nil {
		return fmt.Sprintf("(cannot read log: %v)", err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// resolveBinaryPath 将二进制名解析为绝对路径。
// config 中可能是裸名 "xray"（靠 PATH 查找），filepath.Abs 会错误地解析成 <CWD>/xray。
// 用 exec.LookPath 与 exec.Command 一致的语义解析。
func resolveBinaryPath(binary string) (string, error) {
	if filepath.IsAbs(binary) {
		return binary, nil
	}
	return exec.LookPath(binary)
}
