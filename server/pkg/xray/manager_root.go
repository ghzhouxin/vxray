package xray

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"v2ray-server/pkg/process"
)

const (
	rootTermWait = 5 * time.Second
	rootKillWait = 3 * time.Second
	reapWait     = 1 * time.Second
)

// —— root 模式进程管理 ——
//
// sudo 的 use_pty 会把真实 xray 放进独立会话：向 sudo 前端发 SIGTERM 只会让
// sudo 自身退出、孤儿化 xray（端口仍被占）。故 root 的停止与清理都直接对真实
// root xray pid 逐 pid `sudo -n /bin/kill`，不依赖 sudo 转发信号。

// stopRoot 停止 root xray：以真实 xray pid 全部消亡为成功标准。
func (m *Manager) stopRoot(proc *process.Process) error {
	if m.configPath == "" {
		return proc.Stop(stopTimeout)
	}

	// 优雅停止：发 TERM，超时不死则强杀，均不济则上报仍存活的真实 pid。
	if signalRootXrays(m.configPath, "-TERM") && !waitRootGone(m.configPath, rootTermWait) {
		signalRootXrays(m.configPath, "-9")
		if !waitRootGone(m.configPath, rootKillWait) {
			_ = proc.Stop(reapWait)
			return fmt.Errorf("root xray survived SIGKILL: %v", rootXrayPids(m.configPath))
		}
	}
	// 真实 xray 已死，收尾 sudo 前端
	return proc.Stop(reapWait)
}

// cleanupStaleRoot 清理上次会话残留的 root xray：按 configPath 精确匹配真实 pid
// 逐个 sudo kill，再收尾残留的 sudo 前端。不依赖 pidFile——孤儿场景下 pidFile 可能
// 已被上一会话 monitor 删除，但 root xray 仍需按 configPath 回收。
func (m *Manager) cleanupStaleRoot(configPath string) {
	if signalRootXrays(configPath, "-TERM") && !waitRootGone(configPath, staleWait) {
		signalRootXrays(configPath, "-9")
		waitRootGone(configPath, staleWait)
	}
	m.reapStaleFrontend()
}

// reapStaleFrontend 收尾残留的 sudo 前端（pidFile 记录其 pid）。与 user 模式不同，
// sudo 前端拿不到 root xray 的进程组，只需 TERM 前端自身即可。
func (m *Manager) reapStaleFrontend() {
	if m.opts.PidFile == "" {
		return
	}
	if pid := m.readPidFile(); pid > 0 && pidAlive(pid) && isXrayProcess(pid) {
		_ = syscall.Kill(pid, syscall.SIGTERM)
		waitForExit(pid, staleWait)
	}
	_ = os.Remove(m.opts.PidFile)
}

// signalRootXrays 对匹配 config 的真实 root xray pid 逐个 `sudo -n /bin/kill` 发信号，
// 返回是否存在过存活 pid。
func signalRootXrays(configPath, signal string) bool {
	pids := rootXrayPids(configPath)
	for _, pid := range pids {
		_ = exec.Command("sudo", "-n", "/bin/kill", signal, strconv.Itoa(pid)).Run()
	}
	return len(pids) > 0
}

// waitRootGone 轮询直到无匹配真实 root xray pid，返回是否在超时内消失。
func waitRootGone(configPath string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if len(rootXrayPids(configPath)) == 0 {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// rootXrayPids 返回匹配 config 路径的 root 属主 xray 真实 pid。
// pgrep -u 0 只匹配 uid 0 进程，排除 sudo 前端（普通用户 uid）。
func rootXrayPids(configPath string) []int {
	out, err := exec.Command("pgrep", "-u", "0", "-f",
		"xray run -c "+configPath).Output()
	if err != nil {
		return nil // pgrep 无匹配时 exit 1，不是错误
	}
	var pids []int
	for _, s := range strings.Fields(string(out)) {
		if pid, err := strconv.Atoi(s); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}