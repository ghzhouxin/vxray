package service

import (
	"errors"
	"fmt"
	"sync/atomic"
	"v2ray-server/internal/config"
	"v2ray-server/internal/constants"
	"v2ray-server/pkg/xray"
)

// TunState 表示 TUN 模式的状态。
type TunState int32

const (
	TunDisabled TunState = iota
	TunTransitioning
	TunEnabled
)

func (s TunState) String() string {
	switch s {
	case TunDisabled:
		return "disabled"
	case TunTransitioning:
		return "transitioning"
	case TunEnabled:
		return "enabled"
	default:
		return "unknown"
	}
}

// ErrTunBusy 表示 TUN 正在切换状态，拒绝重复操作。
var ErrTunBusy = errors.New("tun mode is transitioning, please wait")

type TunService struct {
	manager  *xray.Manager  // 用户态 xray
	rootXray *xray.RootXray // root xray（TUN 模式）
	cfg      *config.State
	logSvc   *LogService
	logger   *TaggedLogger
	state    atomic.Int32
}

func NewTunService(manager *xray.Manager, rootXray *xray.RootXray, cfg *config.State, logSvc *LogService) *TunService {
	s := &TunService{
		manager:  manager,
		rootXray: rootXray,
		cfg:      cfg,
		logSvc:   logSvc,
		logger:   logSvc.NewTaggedLogger(constants.TagTun),
	}
	s.state.Store(int32(TunDisabled))
	// 注册崩溃回调：root xray 非预期退出时恢复用户态 xray
	rootXray.SetCrashCallback(s.onRootXrayCrash)
	return s
}

// onRootXrayCrash 在 root xray 非预期退出时被调用，恢复用户态 xray 并重置状态。
func (s *TunService) onRootXrayCrash() {
	s.logger.Error("root xray 意外退出，正在恢复用户态 xray", nil)
	if err := s.manager.Start(); err != nil {
		s.logger.Error("恢复用户态 xray 失败", map[string]any{"error": err.Error()})
	}
	s.state.Store(int32(TunDisabled))
}

// Enable 开启 TUN 模式：停止用户态 xray，osascript 提权启动 supervisor（root xray）。
// 这是 TUN 会话内唯一的密码弹窗。supervisor 启动后，后续操作经 IPC 下发，不再弹窗。
func (s *TunService) Enable() error {
	if !s.casState(TunDisabled, TunTransitioning) {
		return ErrTunBusy
	}
	// panic 安全：确保状态不卡在 Transitioning
	defer func() {
		if r := recover(); r != nil {
			_ = s.rootXray.Stop()
			s.state.Store(int32(TunDisabled))
			s.logger.Error("Enable panic", map[string]any{"error": fmt.Sprintf("%v", r)})
		}
	}()

	paths := s.cfg.SystemMeta().Paths

	// 失败时回滚：恢复用户态 xray（若之前在运行），状态置 Disabled
	rollback := func(reason string, err error) error {
		s.state.Store(int32(TunDisabled))
		wrapped := fmt.Errorf("%s: %w", reason, err)
		s.logger.Error("开启 TUN 失败", map[string]any{"error": err.Error()})
		if !s.manager.IsRunning() {
			if restartErr := s.manager.Start(); restartErr != nil {
				s.logger.Error("回滚重启用户态 xray 失败", map[string]any{"error": restartErr.Error()})
			}
		}
		return wrapped
	}

	// 1. 注入 tun inbound 到临时配置
	if err := xray.InjectTunInbound(paths.XrayConfigPath, paths.TunConfigPath); err != nil {
		return rollback("inject tun config", err)
	}

	// 2. 停止用户态 xray（root xray 会接管 socks/http 端口）
	if s.manager.IsRunning() {
		if err := s.manager.Stop(); err != nil {
			return rollback("stop user xray", err)
		}
	}

	// 3. 启动 root xray（首次会弹 osascript 安装 sudoers 规则，之后永久免密）
	if err := s.rootXray.Start(paths.TunConfigPath); err != nil {
		return rollback("start root xray", err)
	}

	s.state.Store(int32(TunEnabled))
	s.logger.Info("TUN 模式已开启", nil)
	return nil
}

// Disable 关闭 TUN 模式：sudo -n kill root xray，无弹窗。
func (s *TunService) Disable() error {
	if !s.casState(TunEnabled, TunTransitioning) {
		return ErrTunBusy
	}
	defer func() {
		if r := recover(); r != nil {
			s.state.Store(int32(TunDisabled))
			s.logger.Error("Disable panic", map[string]any{"error": fmt.Sprintf("%v", r)})
		}
	}()

	// 1. 停止 root xray（sudo -n kill，无弹窗）
	if err := s.rootXray.Stop(); err != nil {
		s.logger.Error("停止 root xray 失败", map[string]any{"error": err.Error()})
		// 即便失败也继续：root xray 可能已死，仍需恢复用户态 xray
	}

	// 2. 重启用户态 xray 恢复代理服务
	if err := s.manager.Start(); err != nil {
		s.state.Store(int32(TunDisabled))
		s.logger.Error("重启用户态 xray 失败", map[string]any{"error": err.Error()})
		return fmt.Errorf("restart user xray: %w", err)
	}

	s.state.Store(int32(TunDisabled))
	s.logger.Info("TUN 模式已关闭", nil)
	return nil
}

// Shutdown 用于 vxray 关闭：sudo -n kill root xray，无弹窗，不重启用户态 xray。
func (s *TunService) Shutdown() {
	if s.Status() != TunEnabled {
		return
	}
	_ = s.rootXray.Stop()
	s.state.Store(int32(TunDisabled))
}

// RestartRootXray 重新生成 tun-config（基于最新 xray config）并重启 root xray。
// 用于 TUN 开启时切换节点/测速：config 已更新，需重启 xray 使其生效。无密码弹窗。
func (s *TunService) RestartRootXray() error {
	paths := s.cfg.SystemMeta().Paths
	if err := xray.InjectTunInbound(paths.XrayConfigPath, paths.TunConfigPath); err != nil {
		return fmt.Errorf("reinject tun config: %w", err)
	}
	if err := s.rootXray.Restart(paths.TunConfigPath); err != nil {
		return fmt.Errorf("restart root xray: %w", err)
	}
	s.logger.Info("root xray 已重启（节点切换）", nil)
	return nil
}

// Status 返回当前 TUN 状态。
func (s *TunService) Status() TunState {
	return TunState(s.state.Load())
}

// IsEnabled 便捷方法。
func (s *TunService) IsEnabled() bool {
	return s.Status() == TunEnabled
}

// CleanupStale 启动时清理残留的 root xray 进程。
func (s *TunService) CleanupStale() {
	if err := s.rootXray.CleanupStale(); err != nil {
		s.logger.Error("清理残留 root xray 失败", map[string]any{"error": err.Error()})
	}
}

// casState 原子 CAS 状态转换，仅当当前状态等于 old 时才设为 new。
func (s *TunService) casState(old, new TunState) bool {
	return s.state.CompareAndSwap(int32(old), int32(new))
}
