package xray

import (
	"fmt"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

const sudoersFilePath = "/etc/sudoers.d/vxray"

// sudoersRuleReady 检查当前用户是否能免密 sudo 执行指定的 xray 启动命令。
// 用 `sudo -n -l` 列出全部权限，检查是否存在 NOPASSWD 行同时匹配 binary 和 configPath。
// 不能用 `sudo -n -l <cmd>`：macOS 上 admin 用户对任何命令都有 sudo 权限（需密码），
// 该命令即使无 NOPASSWD 规则也会列出 <cmd>，导致误判为已就绪。
func sudoersRuleReady(binary, configPath string) bool {
	cmd := exec.Command("sudo", "-n", "-l")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	output := string(out)
	return strings.Contains(output, "NOPASSWD") &&
		strings.Contains(output, binary) &&
		strings.Contains(output, configPath)
}

// ensureSudoersRule 确保sudoers 规则已安装。
// 若规则已存在（sudoersRuleReady 返回 true），直接返回。
// 否则用 osascript 提权写入 /etc/sudoers.d/vxray（这是系统内唯一的密码弹窗）。
//
// 规则格式：
//
//	<user> ALL=(ALL) NOPASSWD: <binary> run -c <configPath>, /bin/kill
//
// 精确匹配二进制路径和参数，避免提权风险。
func ensureSudoersRule(binary, configPath string) error {
	if sudoersRuleReady(binary, configPath) {
		return nil
	}

	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("get current user: %w", err)
	}
	username := u.Username

	// sudoers 要求绝对路径；binary 可能是裸名（靠 PATH 查找），用 LookPath 解析
	absBinary, err := resolveBinaryPath(binary)
	if err != nil {
		return fmt.Errorf("resolve binary path: %w", err)
	}
	absConfig, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}

	// env_keep 确保 XRAY_LOCATION_ASSET 透过 sudo 的 env_reset 传递给 xray，
	// 否则 xray 找不到 geoip.dat/geosite.dat 会启动即崩。
	rule := fmt.Sprintf("Defaults env_keep += \"XRAY_LOCATION_ASSET\"\n%s ALL=(ALL) NOPASSWD: %s run -c %s, /bin/kill\n",
		username, absBinary, absConfig)

	// osascript 提权写入 sudoers 文件；tee 追加，chmod 0440 是 sudoers 要求
	shellCmd := fmt.Sprintf(
		"echo %s | sudo tee %s > /dev/null && sudo chmod 0440 %s",
		shellQuoteSudoers(rule), sudoersFilePath, sudoersFilePath,
	)
	script := fmt.Sprintf(`do shell script %q with administrator privileges`, shellCmd)
	cmd := exec.Command("osascript", "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("install sudoers rule (user may have cancelled): %w (output: %s)",
			err, strings.TrimSpace(string(out)))
	}

	// 验证规则生效
	if !sudoersRuleReady(absBinary, absConfig) {
		return fmt.Errorf("sudoers rule installed but not effective; check %s", sudoersFilePath)
	}
	return nil
}

// shellQuoteSudoers 对字符串加单引号，用于 shell 安全传递。
func shellQuoteSudoers(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
