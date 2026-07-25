package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

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
	manager  *xray.Manager
	nodeRepo *repository.NodeRepository
	cfg      *config.State
	logSvc   *LogService
	logger   *TaggedLogger
}

type xrayRuntimeConfig struct{ cfg *config.State }

func (c xrayRuntimeConfig) GetXrayBinary() string     { return c.cfg.SystemMeta().Xray.Binary }
func (c xrayRuntimeConfig) GetXrayConfigPath() string { return c.cfg.SystemMeta().Paths.XrayConfigPath }
func (c xrayRuntimeConfig) GetGeoDir() string         { return c.cfg.SystemMeta().Paths.GeoDir }

func NewXrayService(nodeRepo *repository.NodeRepository, cfg *config.State, logger *LogService) *XrayService {
	return &XrayService{
		manager:  xray.NewManager(xrayRuntimeConfig{cfg: cfg}),
		nodeRepo: nodeRepo,
		cfg:      cfg,
		logSvc:   logger,
		logger:   logger.NewTaggedLogger(constants.TagXray),
	}
}

func (s *XrayService) Start() error {
	return s.logSvc.RunOperation(constants.TagXray, "启动 Xray", nil, s.manager.Start)
}

func (s *XrayService) Stop() error {
	return s.logSvc.RunOperation(constants.TagXray, "停止 Xray", nil, s.manager.Stop)
}
func (s *XrayService) Status() bool               { return s.manager.IsRunning() }
func (s *XrayService) GetManager() *xray.Manager  { return s.manager }
func (s *XrayService) GetConfig() (string, error) { return s.cfg.GetXrayConfigContent() }

func (s *XrayService) SaveConfig(content string) error {
	return s.logSvc.RunOperation(constants.TagSystem, "保存 Xray 配置", nil, func() error {
		var cfg map[string]any
		if err := json.Unmarshal([]byte(content), &cfg); err != nil {
			return fmt.Errorf("unmarshal xray config: %w", err)
		}
		return s.cfg.WriteXrayConfig(cfg)
	})
}

func (s *XrayService) GetActiveNode() *model.Node {
	nodeID := s.cfg.GetActiveNodeID()
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
	}
	if err := s.cfg.UpdateXrayOutbound(outbound); err != nil {
		s.logger.Error("切换活动节点失败", map[string]any{"node_id": id, "error": err.Error()})
		return err
	}
	s.cfg.SetActiveNodeID(id)
	if err := s.manager.ForceStart(); err != nil {
		s.logger.Error("切换活动节点失败", map[string]any{"node_id": id, "error": err.Error()})
		return err
	}
	detail := map[string]any{"node_id": id}
	if node != nil {
		detail["node_name"] = node.Name
		detail["protocol"] = node.Protocol
	}
	s.logger.Info("活动节点已切换", detail)
	return nil
}

func (s *XrayService) GetXrayPorts() (*config.XrayPorts, error) {
	return s.cfg.GetXrayPorts()
}

func (s *XrayService) SpeedTestWebsite(socksPort int) ([]config.WebsiteSpeedTestResult, error) {
	s.logger.Info("开始网站测速", nil)

	st := speedtest.New(s.cfg.UserSettings())
	targets := s.cfg.UserSettings().SpeedTest.WebsiteTargets
	results := make([]config.WebsiteSpeedTestResult, len(targets))

	var wg sync.WaitGroup
	var mu sync.Mutex
	for i, target := range targets {
		wg.Add(1)
		go func(idx int, t config.SpeedTestTarget) {
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					results[idx] = config.WebsiteSpeedTestResult{
						Name: t.Name, URL: t.URL,
						Latency: -1, Error: fmt.Sprintf("website speedtest panic: %v", r),
					}
					mu.Unlock()
				}
				wg.Done()
			}()
			result := st.TestWithProxyAndTarget(socksPort, t.URL)
			mu.Lock()
			results[idx] = config.WebsiteSpeedTestResult{
				Name: t.Name, URL: t.URL,
				Latency: result.Latency, Error: result.Error,
			}
			mu.Unlock()
		}(i, target)
	}
	wg.Wait()
	failed := 0
	for _, result := range results {
		if result.Error != "" {
			failed++
		}
	}
	if err := s.cfg.SaveWebsiteSpeedTestResults(results); err != nil {
		s.logger.Error("保存网站测速结果失败", map[string]any{"error": err.Error()})
	}
	s.logger.Info("网站测速完成", map[string]any{
		"total":   len(results),
		"success": len(results) - failed,
		"failed":  failed,
	})
	return results, nil
}
