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

// TunService 持有 root 与 user 两个 Manager 实例：
// root 实例跑 TUN 配置，user 实例在 TUN 关闭/回滚时恢复代理。
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
		return nil // 幂等：已开启直接成功
	}
	if !s.casState(TunDisabled, TunTransitioning) {
		return ErrTunBusy
	}

	paths := s.cfg.SystemMeta().Paths
	rollback := func(reason string, err error) error {
		s.state.Store(int32(TunDisabled))
		wrapped := fmt.Errorf("%s: %w", reason, err)
		s.logger.Error("开启 TUN 失败", map[string]any{"error": err.Error()})
		if s.root.Running() {
			if stopErr := s.root.Stop(); stopErr != nil {
				s.logger.Error("回滚停止 root xray 失败", map[string]any{"error": stopErr.Error()})
			}
		}
		if !s.user.Running() {
			if restartErr := s.user.Start(paths.XrayConfigPath); restartErr != nil {
				s.logger.Error("回滚重启用户态 xray 失败", map[string]any{"error": restartErr.Error()})
			}
		}
		return wrapped
	}

	if err := xray.InjectTunInbound(paths.XrayConfigPath, paths.TunConfigPath); err != nil {
		return rollback("inject tun config", err)
	}
	if s.user.Running() {
		if err := s.user.Stop(); err != nil {
			return rollback("stop user xray", err)
		}
	}
	if err := s.root.Start(paths.TunConfigPath); err != nil {
		return rollback("start root xray", err)
	}

	s.state.Store(int32(TunEnabled))
	s.logger.Info("TUN 模式已开启", nil)
	return nil
}

func (s *TunService) Disable() error {
	if s.Status() == TunDisabled {
		return nil // 幂等：已关闭直接成功
	}
	if !s.casState(TunEnabled, TunTransitioning) {
		return ErrTunBusy
	}

	paths := s.cfg.SystemMeta().Paths
	if err := s.root.Stop(); err != nil {
		// root 仍存活，状态回滚为 Enabled
		s.state.Store(int32(TunEnabled))
		return fmt.Errorf("stop root xray: %w", err)
	}

	s.state.Store(int32(TunDisabled))

	// root.Stop() 成功后等 socks 端口释放再启动 user xray：
	// 端口还可能被垂死的 root xray 短暂持有，此时 bind 会失败
	paths = s.cfg.SystemMeta().Paths
	ports, _ := s.cfg.XrayPorts()
	if ports != nil && !xray.WaitPortClosed(ports.SOCKSPort, 3*time.Second) {
		s.logger.Info("socks 端口未及时释放，继续启动用户态 xray", nil)
	}

	// root 已停、TUN 事实已失效；user 启动失败只记日志
	if err := s.user.Start(paths.XrayConfigPath); err != nil {
		s.logger.Error("重启用户态 xray 失败", map[string]any{"error": err.Error()})
		return fmt.Errorf("restart user xray: %w", err)
	}

	s.logger.Info("TUN 模式已关闭", nil)
	return nil
}

// handleRootCrash 由 root Manager 崩溃回调触发：清状态并恢复用户态代理。
func (s *TunService) handleRootCrash() {
	if !s.casState(TunEnabled, TunDisabled) {
		return
	}
	s.logger.Error("root xray 意外退出，回退用户态模式", nil)
	paths := s.cfg.SystemMeta().Paths
	if err := s.user.Start(paths.XrayConfigPath); err != nil {
		s.logger.Error("崩溃回退后重启用户态 xray 失败", map[string]any{"error": err.Error()})
	}
}

func (s *TunService) Shutdown() {
	// 无条件尝试停止 root：Enable 进行中应用退出时也要卸载 root 副作用
	if err := s.root.Stop(); err != nil {
		s.logger.Error("关闭 TUN root xray 失败", map[string]any{"error": err.Error()})
	}
	s.state.Store(int32(TunDisabled))
}

func (s *TunService) RestartRootProcess() error {
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
