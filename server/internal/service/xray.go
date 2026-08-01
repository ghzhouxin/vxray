package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"v2ray-server/internal/config"
	"v2ray-server/internal/constants"
	"v2ray-server/internal/model"
	"v2ray-server/internal/repository"
	"v2ray-server/pkg/speedtest"
	"v2ray-server/pkg/types"
	"v2ray-server/pkg/xray"

	"gorm.io/gorm"
)

type XrayService struct {
	manager      *xray.Manager
	nodeRepo     *repository.NodeRepository
	cfg          *config.State
	logSvc       *LogService
	logger       *TaggedLogger
	tunChecker   func() bool
	tunDisabler  func() error
	tunRestarter func() error
	crashMu      sync.Mutex
	lastCrashAt  time.Time
}

const crashRestartWindow = 60 * time.Second

type xrayRuntimeConfig struct{ cfg *config.State }

func (c xrayRuntimeConfig) XrayBinary() string     { return c.cfg.SystemMeta().Xray.Binary }
func (c xrayRuntimeConfig) XrayConfigPath() string { return c.cfg.SystemMeta().Paths.XrayConfigPath }
func (c xrayRuntimeConfig) GeoDir() string         { return c.cfg.SystemMeta().Paths.GeoDir }

func NewXrayService(nodeRepo *repository.NodeRepository, cfg *config.State, logger *LogService) *XrayService {
	return &XrayService{
		manager:  xray.NewManager(xrayRuntimeConfig{cfg: cfg}),
		nodeRepo: nodeRepo,
		cfg:      cfg,
		logSvc:   logger,
		logger:   logger.NewTaggedLogger(constants.TagXray),
	}
}

// SetTunHandlers 注入 TUN 操作函数，避免循环依赖。
//   - checker: 返回 TUN 是否开启
//   - disabler: 关闭 TUN（用于 Stop 时先关 TUN）
//   - restarter: 切换节点时重启 root xray
func (s *XrayService) SetTunHandlers(checker func() bool, disabler, restarter func() error) {
	s.tunChecker = checker
	s.tunDisabler = disabler
	s.tunRestarter = restarter
}

// OnCrash 是用户态 xray 崩溃时的回调。
// 60s 内首次崩溃自动重启一次；短时间再崩则放弃（防循环），仅记 error。
// 这与 TUN supervisor 的 crashBudget 同模式，补齐状态生命周期。
func (s *XrayService) OnCrash() {
	s.crashMu.Lock()
	last := s.lastCrashAt
	now := time.Now()
	s.lastCrashAt = now
	s.crashMu.Unlock()

	if s.isTunEnabled() {
		// TUN 模式下用户态 xray 不在运行，崩溃回调不应触发；记录后返回。
		s.logger.Error("xray 意外退出（TUN 模式）", nil)
		return
	}

	if !last.IsZero() && now.Sub(last) < crashRestartWindow {
		s.logger.Error("xray 短时间再次崩溃，放弃自动重启", map[string]any{
			"elapsed_ms": now.Sub(last).Milliseconds(),
		})
		return
	}

	s.logger.Info("xray 意外退出，自动重启", nil)
	if err := s.manager.Start(); err != nil {
		s.logger.Error("xray 自动重启失败", map[string]any{"error": err.Error()})
	}
}

func (s *XrayService) isTunEnabled() bool {
	if s.tunChecker == nil {
		return false
	}
	return s.tunChecker()
}

// Start 启动 xray。TUN 已开启时为 no-op（root xray 已在提供 socks/http + tun）。
func (s *XrayService) Start() error {
	if s.isTunEnabled() {
		return nil
	}
	return s.logSvc.RunOperation(constants.TagXray, "启动 Xray", nil, s.manager.Start)
}

// Stop 停止 xray。TUN 开启时先关闭 TUN（root xray 同时被停），再停用户态 xray。
func (s *XrayService) Stop() error {
	if s.isTunEnabled() && s.tunDisabler != nil {
		if err := s.tunDisabler(); err != nil {
			return fmt.Errorf("disable tun before stop xray: %w", err)
		}
	}
	return s.logSvc.RunOperation(constants.TagXray, "停止 Xray", nil, s.manager.Stop)
}

// Restart 重启 xray 以应用新配置。TUN 开启时走 tunRestarter（重启 root xray），
// 否则 ForceStart 用户态 xray（未运行时等价于 Start）。
func (s *XrayService) Restart() error {
	if s.isTunEnabled() && s.tunRestarter != nil {
		return s.logSvc.RunOperation(constants.TagXray, "重启 root Xray", nil, s.tunRestarter)
	}
	return s.logSvc.RunOperation(constants.TagXray, "重启 Xray", nil, s.manager.ForceStart)
}

// Status 返回 xray 是否在运行（用户态 xray 或 root xray 任一运行即为 true）。
func (s *XrayService) Status() bool {
	return s.manager.IsRunning() || s.isTunEnabled()
}

func (s *XrayService) GetManager() *xray.Manager  { return s.manager }
func (s *XrayService) GetConfig() (string, error) { return s.cfg.XrayConfigContent() }

// GetDefaultConfig 返回默认的 Xray 配置内容。
func (s *XrayService) GetDefaultConfig() (string, error) {
	return s.cfg.DefaultXrayConfigContent()
}

func (s *XrayService) SaveConfig(content string) error {
	return s.logSvc.RunOperation(constants.TagXray, "保存 Xray 配置", nil, func() error {
		var cfg map[string]any
		if err := json.Unmarshal([]byte(content), &cfg); err != nil {
			return fmt.Errorf("unmarshal xray config: %w", err)
		}
		return s.cfg.WriteXrayConfig(cfg)
	})
}

func (s *XrayService) GetActiveNode() *model.Node {
	nodeID := s.cfg.ActiveNodeID()
	if nodeID == 0 {
		return nil
	}
	node, err := s.nodeRepo.FindByID(nodeID)
	if err != nil {
		s.logger.Error("获取活动节点失败", map[string]any{"node_id": nodeID, "error": err.Error()})
		return nil
	}
	return node
}

func (s *XrayService) SetActiveNode(id uint, outbound types.Map) error {
	node, err := s.nodeRepo.FindByID(id)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Error("查询活动节点失败", map[string]any{"node_id": id, "error": err.Error()})
			return err
		}
		s.logger.Error("活动节点不存在", map[string]any{"node_id": id})
		return fmt.Errorf("active node %d not found", id)
	}
	if err := s.cfg.UpdateXrayOutbound(outbound); err != nil {
		s.logger.Error("切换活动节点失败", map[string]any{"node_id": id, "error": err.Error()})
		return err
	}
	s.cfg.SetActiveNodeID(id)

	// TUN 开启时重启 root xray（使用更新后的 config），否则强制重启用户态 xray
	if s.isTunEnabled() && s.tunRestarter != nil {
		if err := s.tunRestarter(); err != nil {
			s.logger.Error("切换活动节点失败（重启 root xray）", map[string]any{"node_id": id, "error": err.Error()})
			return err
		}
	} else {
		if err := s.manager.ForceStart(); err != nil {
			s.logger.Error("切换活动节点失败", map[string]any{"node_id": id, "error": err.Error()})
			return err
		}
	}

	s.logger.Info("活动节点已切换", map[string]any{
		"node_id":   id,
		"node_name": node.Name,
		"protocol":  node.Protocol,
	})
	return nil
}

func (s *XrayService) XrayPorts() (*config.XrayPorts, error) {
	return s.cfg.XrayPorts()
}

func (s *XrayService) SpeedTestWebsite(socksPort int) error {
	s.logger.Info("开始网站测速", nil)

	settings := s.cfg.UserSettings()
	targets := settings.SpeedTest.WebsiteTargets
	results := make([]config.SpeedTestTarget, len(targets))
	now := time.Now().Unix()

	var wg sync.WaitGroup
	var mu sync.Mutex
	st := speedtest.New(s.cfg.UserSettings()) // SpeedTest 无状态，共享一个实例即可
	for i, target := range targets {
		wg.Add(1)
		go func(idx int, t config.SpeedTestTarget) {
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					results[idx] = config.SpeedTestTarget{
						Name: t.Name, URL: t.URL, Icon: t.Icon,
						Latency: -1, Error: fmt.Sprintf("website speedtest panic: %v", r),
						TestedAt: now,
					}
					mu.Unlock()
				}
				wg.Done()
			}()
			result := st.TestWithProxyAndTarget(socksPort, t.URL)
			mu.Lock()
			results[idx] = config.SpeedTestTarget{
				Name: t.Name, URL: t.URL, Icon: t.Icon,
				Latency: result.Latency, Error: result.Error,
				TestedAt: now,
			}
			mu.Unlock()
		}(i, target)
	}
	wg.Wait()

	// 只更新 WebsiteTargets,避免覆盖并发用户改动
	if err := s.cfg.UpdateWebsiteTargets(results); err != nil {
		s.logger.Error("保存网站测速结果到 DB 失败", map[string]any{"error": err.Error()})
		return fmt.Errorf("persist website speedtest results: %w", err)
	}

	failed := 0
	for _, result := range results {
		if result.Error != "" {
			failed++
		}
	}
	s.logger.Info("网站测速完成", map[string]any{
		"total":   len(results),
		"success": len(results) - failed,
		"failed":  failed,
	})
	return nil
}
