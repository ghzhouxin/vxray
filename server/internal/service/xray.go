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
	logger       *TaggedLogger
	tunChecker   func() bool
	tunDisabler  func() error
	tunRestarter func() error
	crashMu      sync.Mutex
	lastCrashAt  time.Time
}

const crashRestartWindow = 60 * time.Second

func NewXrayService(nodeRepo *repository.NodeRepository, cfg *config.State, logSvc *LogService, manager *xray.Manager) *XrayService {
	return &XrayService{
		manager:  manager,
		nodeRepo: nodeRepo,
		cfg:      cfg,
		logger:   logSvc.NewTaggedLogger(constants.TagXray),
	}
}

func (s *XrayService) SetTunHandlers(checker func() bool, disabler, restarter func() error) {
	s.tunChecker = checker
	s.tunDisabler = disabler
	s.tunRestarter = restarter
}

func (s *XrayService) isTunEnabled() bool {
	if s.tunChecker == nil {
		return false
	}
	return s.tunChecker()
}

func (s *XrayService) Start() error {
	if s.isTunEnabled() {
		return nil
	}
	s.logger.Info("启动 Xray", nil)
	configPath := s.cfg.SystemMeta().Paths.XrayConfigPath
	if err := s.manager.Start(configPath); err != nil {
		s.logger.Error("启动 Xray 失败", map[string]any{"error": err.Error()})
		return err
	}
	return nil
}

// handleCrash 由 user Manager 崩溃回调触发：
// 60s 窗口内首次崩溃自动重启一次，再崩只记日志（防循环）；TUN 模式下仅记日志。
func (s *XrayService) handleCrash() {
	if s.isTunEnabled() {
		s.logger.Error("用户态 xray 意外退出（TUN 模式，不自动重启）", nil)
		return
	}
	s.crashMu.Lock()
	if time.Since(s.lastCrashAt) < crashRestartWindow {
		s.crashMu.Unlock()
		s.logger.Error("用户态 xray 意外退出（窗口期内多次崩溃，放弃自动重启）", nil)
		return
	}
	s.lastCrashAt = time.Now()
	s.crashMu.Unlock()

	s.logger.Error("用户态 xray 意外退出，自动重启", nil)
	if err := s.Start(); err != nil {
		s.logger.Error("自动重启用户态 xray 失败", map[string]any{"error": err.Error()})
	}
}

func (s *XrayService) Stop() error {
	if s.isTunEnabled() && s.tunDisabler != nil {
		if err := s.tunDisabler(); err != nil {
			return fmt.Errorf("disable tun before stop xray: %w", err)
		}
	}
	s.logger.Info("停止 Xray", nil)
	if err := s.manager.Stop(); err != nil {
		s.logger.Error("停止 Xray 失败", map[string]any{"error": err.Error()})
		return err
	}
	return nil
}

func (s *XrayService) Restart() error {
	if s.isTunEnabled() && s.tunRestarter != nil {
		s.logger.Info("重启 root process", nil)
		if err := s.tunRestarter(); err != nil {
			s.logger.Error("重启 root process 失败", map[string]any{"error": err.Error()})
			return err
		}
		return nil
	}
	s.logger.Info("重启 Xray", nil)
	configPath := s.cfg.SystemMeta().Paths.XrayConfigPath
	if err := s.manager.Restart(configPath); err != nil {
		s.logger.Error("重启 Xray 失败", map[string]any{"error": err.Error()})
		return err
	}
	return nil
}

func (s *XrayService) Status() bool {
	return s.manager.Running() || s.isTunEnabled()
}

func (s *XrayService) GetConfig() (string, error) { return s.cfg.XrayConfigContent() }

func (s *XrayService) GetDefaultConfig() (string, error) {
	return s.cfg.DefaultXrayConfigContent()
}

func (s *XrayService) SaveConfig(content string) error {
	s.logger.Info("保存 Xray 配置", nil)
	var cfg map[string]any
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		return fmt.Errorf("unmarshal xray config: %w", err)
	}
	if err := s.cfg.WriteXrayConfig(cfg); err != nil {
		s.logger.Error("保存 Xray 配置失败", map[string]any{"error": err.Error()})
		return err
	}
	return nil
}

func (s *XrayService) GetActiveNode() *model.Node {
	nodeID := s.cfg.ActiveNodeID()
	if nodeID == 0 {
		return nil
	}
	node, err := s.nodeRepo.FindByID(nodeID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Error("获取活动节点失败", map[string]any{"node_id": nodeID, "error": err.Error()})
		}
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
	if err := s.cfg.SetActiveNodeID(id); err != nil {
		s.logger.Error("持久化活动节点失败", map[string]any{"node_id": id, "error": err.Error()})
	}

	if s.isTunEnabled() && s.tunRestarter != nil {
		if err := s.tunRestarter(); err != nil {
			s.logger.Error("切换活动节点失败（重启 root process）", map[string]any{"node_id": id, "error": err.Error()})
			return err
		}
	} else {
		configPath := s.cfg.SystemMeta().Paths.XrayConfigPath
		if err := s.manager.Restart(configPath); err != nil {
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
	settings := s.cfg.UserSettings()
	targets := settings.SpeedTest.WebsiteTargets
	results := make([]config.SpeedTestTarget, len(targets))
	now := time.Now().Unix()

	st := speedtest.New(s.cfg.UserSettings())
	const maxConcurrency = 2
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	for i, target := range targets {
		sem <- struct{}{}
		wg.Add(1)
		go func(idx int, t config.SpeedTestTarget) {
			defer func() {
				if r := recover(); r != nil {
					results[idx] = config.SpeedTestTarget{
						Name: t.Name, URL: t.URL, Icon: t.Icon,
						Latency: -1, Error: fmt.Sprintf("website speedtest panic: %v", r),
						TestedAt: now,
					}
				}
				<-sem
				wg.Done()
			}()
			result := st.TestWithProxyAndTarget(socksPort, t.URL)
			results[idx] = config.SpeedTestTarget{
				Name: t.Name, URL: t.URL, Icon: t.Icon,
				Latency: result.Latency, Error: result.Error,
				TestedAt: now,
			}
		}(i, target)
	}
	wg.Wait()

	if err := s.cfg.UpdateWebsiteTargets(results); err != nil {
		s.logger.Error("保存网站测速结果到 DB 失败", map[string]any{"error": err.Error()})
		return fmt.Errorf("persist website speedtest results: %w", err)
	}

	return nil
}
