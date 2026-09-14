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
// `sudo -n xray run -c <config>` 会 fork 出两个进程：
//   - sudo 前端：vxray 直系子进程，euid=0（setuid）但 ruid 是普通用户；
//   - 真实 xray：sudo 子进程，ruid=0，跑 TUN 配置、占用端口。
//
// 向 sudo 前端发信号只会让 sudo 退出、孤儿化真实 xray（端口仍被占）。因此停止与
// 监控只针对真实 xray pid（启动时按 ruid 捕获一次），孤儿清理按 config 路径 rediscover。

// captureRootPID 捕获真实 root xray pid；未捕获到返回 -1。
func captureRootPID(configPath string) int {
	if pids := rootPids(configPath); len(pids) == 1 {
		return pids[0]
	}
	return -1
}

// rootPids 返回匹配 config 的真实 root xray pid。
// 用 pgrep -U（ruid 匹配）而非 -u（euid 匹配）：sudo 前端 euid=0（setuid）会被
// -u 误中，-U 只命中 ruid=0 的真实 xray。
func rootPids(configPath string) []int {
	out, err := exec.Command("pgrep", "-U", "0", "-f", "xray run -c "+configPath).Output()
	if err != nil {
		return nil // pgrep 无匹配时 exit 1，非错误
	}
	var pids []int
	for _, s := range strings.Fields(string(out)) {
		if pid, err := strconv.Atoi(s); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

// rootAlive 用 sudo kill -0 探测 root 进程存活（普通用户 kill(0) 对 uid 0 进程返回 EPERM，不可靠）。
func rootAlive(pid int) bool {
	return killRoot(pid, "-0") == nil
}

// killRoot 以 root 身份对 pid 发信号。
func killRoot(pid int, sig string) error {
	return exec.Command("sudo", "-n", "/bin/kill", sig, strconv.Itoa(pid)).Run()
}

// stopRoot 停止 root xray：以真实 xray pid 消亡为成功标准，再收尾 sudo 前端。
func (m *Manager) stopRoot(proc *process.Process) error {
	pid := m.rootPID
	if pid <= 0 {
		return reapFrontend(proc, stopTimeout) // 未捕获真实 pid，退化为杀前端
	}
	if rootAlive(pid) && !signalRoot(pid, "-TERM", rootTermWait) && !signalRoot(pid, "-9", rootKillWait) {
		_ = reapFrontend(proc, reapWait)
		return fmt.Errorf("root xray %d 在 SIGKILL 后仍存活", pid)
	}
	return reapFrontend(proc, reapWait)
}

// signalRoot 发信号并等待真实 pid 消亡：成功标准是 pid 消失，而非 kill 命令返回 0。
func signalRoot(pid int, sig string, wait time.Duration) bool {
	_ = killRoot(pid, sig)
	return waitRootGone(pid, wait)
}

// reapFrontend 收尾 sudo 前端（proc 为 nil 或已退出时为无害空操作）。
func reapFrontend(proc *process.Process, timeout time.Duration) error {
	if proc == nil {
		return nil
	}
	return proc.Stop(timeout)
}

// waitRootGone 轮询直到 root pid 消失，返回是否在超时内消失。
func waitRootGone(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for rootAlive(pid) {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(200 * time.Millisecond)
	}
	return true
}

// cleanupStaleRoot 清理上次会话残留的 root xray：按 config 路径 rediscover 真实 pid，
// TERM → 等待 → KILL 兜底，再收尾残留 sudo 前端。不依赖 pidFile——孤儿场景下 pidFile
// 可能已被上一会话 monitor 删除。
func (m *Manager) cleanupStaleRoot(configPath string) {
	for _, pid := range rootPids(configPath) {
		if !signalRoot(pid, "-TERM", staleWait) {
			_ = killRoot(pid, "-9")
		}
	}
	m.reapStaleFrontend()
}

// reapStaleFrontend 收尾残留 sudo 前端（pidFile 记录其 pid）。
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