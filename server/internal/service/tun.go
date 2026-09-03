package service

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"v2ray-server/internal/config"
	"v2ray-server/internal/constants"
	"v2ray-server/pkg/xray"
)

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

var ErrTunBusy = errors.New("tun mode is transitioning, please wait")

// TunService 编排 root 与 user 两个 Manager：root 跑 TUN 配置，
// user 在 TUN 关闭或回滚时恢复用户态代理。
type TunService struct {
	root   *xray.Manager
	user   *xray.Manager
	cfg    *config.State
	logger *TaggedLogger
	state  atomic.Int32
}

func NewTunService(root, user *xray.Manager, cfg *config.State, logSvc *LogService) *TunService {
	return &TunService{
		root:   root,
		user:   user,
		cfg:    cfg,
		logger: logSvc.NewTaggedLogger(constants.TagTun),
	}
}

func (s *TunService) Enable() error {
	if s.Status() == TunEnabled {
		return nil
	}
	if !s.casState(TunDisabled, TunTransitioning) {
		return ErrTunBusy
	}

	paths := s.cfg.SystemMeta().Paths
	if err := xray.InjectTunInbound(paths.XrayConfigPath, paths.TunConfigPath); err != nil {
		return s.enableFailed("注入 TUN 配置失败", err)
	}
	if s.user.Running() {
		if err := s.user.Stop(); err != nil {
			return s.enableFailed("停止用户态 xray 失败", err)
		}
	}
	if err := s.root.Start(paths.TunConfigPath); err != nil {
		return s.enableFailed("启动 root xray 失败", err)
	}

	s.state.Store(int32(TunEnabled))
	s.logger.Info("TUN 模式已开启", nil)
	return nil
}

// enableFailed 开启失败回滚：停 root、恢复用户态代理，返回带原因的错误。
func (s *TunService) enableFailed(reason string, cause error) error {
	s.state.Store(int32(TunDisabled))
	s.logger.Error("开启 TUN 失败", map[string]any{"reason": reason, "error": cause.Error()})
	if s.root.Running() {
		if err := s.root.Stop(); err != nil {
			s.logger.Error("回滚停止 root xray 失败", map[string]any{"error": err.Error()})
		}
	}
	if !s.user.Running() {
		if err := s.user.Start(s.cfg.SystemMeta().Paths.XrayConfigPath); err != nil {
			s.logger.Error("回滚恢复用户态 xray 失败", map[string]any{"error": err.Error()})
		}
	}
	return fmt.Errorf("%s: %w", reason, cause)
}

func (s *TunService) Disable() error {
	if s.Status() == TunDisabled {
		return nil
	}
	if !s.casState(TunEnabled, TunTransitioning) {
		return ErrTunBusy
	}

	paths := s.cfg.SystemMeta().Paths
	if err := s.root.Stop(); err != nil {
		s.state.Store(int32(TunEnabled)) // root 仍存活，回滚状态
		return fmt.Errorf("stop root xray: %w", err)
	}
	s.state.Store(int32(TunDisabled))

	// root 已停但端口可能被垂死进程短暂持有，等释放再拉起 user xray
	s.waitSocksFree()

	if err := s.user.Start(paths.XrayConfigPath); err != nil {
		s.logger.Error("恢复用户态 xray 失败", map[string]any{"error": err.Error()})
		return fmt.Errorf("restart user xray: %w", err)
	}
	s.logger.Info("TUN 模式已关闭", nil)
	return nil
}

// waitSocksFree 等待 socks 端口释放再恢复 user xray，避免端口被垂死 root 进程
// 短暂持有导致 user xray bind 失败。
func (s *TunService) waitSocksFree() {
	ports, err := s.cfg.XrayPorts()
	if err != nil || ports.SOCKSPort == 0 {
		return
	}
	if !xray.WaitPortClosed(ports.SOCKSPort, 3*time.Second) {
		s.logger.Info("socks 端口未及时释放，继续恢复用户态 xray", nil)
	}
}

// handleRootCrash root Manager 崩溃回调：清状态并恢复用户态代理。
func (s *TunService) handleRootCrash() {
	if !s.casState(TunEnabled, TunDisabled) {
		return
	}
	s.logger.Error("root xray 意外退出，回退用户态模式", nil)
	if err := s.user.Start(s.cfg.SystemMeta().Paths.XrayConfigPath); err != nil {
		s.logger.Error("崩溃回退后恢复用户态 xray 失败", map[string]any{"error": err.Error()})
	}
}

func (s *TunService) Shutdown() {
	// 无条件尝试停止 root：Enable 进行中应用退出时也要卸载 root 副作用
	if err := s.root.Stop(); err != nil {
		s.logger.Error("关闭 TUN root xray 失败", map[string]any{"error": err.Error()})
	}
	s.state.Store(int32(TunDisabled))
}

// restartRoot 节点切换后重新注入 TUN 配置并重启 root xray。
func (s *TunService) restartRoot() error {
	paths := s.cfg.SystemMeta().Paths
	if err := xray.InjectTunInbound(paths.XrayConfigPath, paths.TunConfigPath); err != nil {
		return fmt.Errorf("reinject tun config: %w", err)
	}
	if err := s.root.Restart(paths.TunConfigPath); err != nil {
		return fmt.Errorf("restart root xray: %w", err)
	}
	s.logger.Info("root xray 已重启（节点切换）", nil)
	return nil
}

func (s *TunService) Status() TunState {
	return TunState(s.state.Load())
}

func (s *TunService) IsEnabled() bool {
	return s.Status() == TunEnabled
}

func (s *TunService) casState(old, new TunState) bool {
	return s.state.CompareAndSwap(int32(old), int32(new))
}