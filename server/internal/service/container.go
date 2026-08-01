package service

import (
	"context"
	"errors"
	"fmt"
	"os"

	"v2ray-server/internal/config"
	"v2ray-server/internal/constants"
	"v2ray-server/internal/repository"
	"v2ray-server/pkg/geo"
	"v2ray-server/pkg/proxy"
	"v2ray-server/pkg/xray"

	"gorm.io/gorm"
)

type geoConfig struct{ cfg *config.State }

func (c geoConfig) GeoIPPath() string { return c.cfg.SystemMeta().Paths.GeoIP }
func (c geoConfig) GeoSitePath() string {
	return c.cfg.SystemMeta().Paths.GeoSite
}
func (c geoConfig) GeoDir() string { return c.cfg.SystemMeta().Paths.GeoDir }
func (c geoConfig) GeoUpdateURL() (string, string, bool) {
	return c.cfg.GeoUpdateURL()
}

type Container struct {
	Config       *config.State
	Log          *LogService
	Subscription *SubscriptionService
	Xray         *XrayService
	Tun          *TunService
	Node         *NodeService
	Proxy        *proxy.Manager
	Geo          *geo.Manager
	ctx          context.Context
	cancel       context.CancelFunc
}

func Init(db *gorm.DB, cfg *config.State) (*Container, error) {
	if db == nil {
		return nil, fmt.Errorf("service init: nil database")
	}
	if cfg == nil {
		return nil, fmt.Errorf("service init: nil config")
	}

	nodeRepo := repository.NewNodeRepository(db)

	logSvc := NewLogService(db)
	subSvc := NewSubscriptionService(db, logSvc, nodeRepo)
	xraySvc := NewXrayService(nodeRepo, cfg, logSvc)
	paths := cfg.SystemMeta().Paths
	rootXray := xray.NewRootXray(
		xrayRuntimeConfig{cfg: cfg},
		paths.TunPidPath,
		paths.TunLogPath,
	)
	tunSvc := NewTunService(xraySvc.GetManager(), rootXray, cfg, logSvc)
	xraySvc.SetTunHandlers(tunSvc.IsEnabled, tunSvc.Disable, tunSvc.RestartRootXray)
	nodeSvc := NewNodeService(xraySvc, nodeRepo, cfg, logSvc)
	proxyMgr := proxy.NewManager(proxy.Options{
		HTTPPort:  constants.ProxyHTTPPort,
		SOCKSPort: constants.ProxySOCKSPort,
	})
	geoMgr := geo.NewManager(geoConfig{cfg: cfg})

	xraySvc.GetManager().SetLogCallback(func(level, message string) {
		_ = logSvc.LogLevel(constants.TagXray, level, message, nil)
	})
	xraySvc.GetManager().SetCrashCallback(xraySvc.OnCrash)
	rootXray.SetLogCallback(func(level, message string) {
		_ = logSvc.LogLevel(constants.TagTun, level, message, nil)
	})

	// 清理上次残留的 root xray 进程
	tunSvc.CleanupStale()

	// 迁移空 Transport 节点：补齐重构前持久化的节点，避免 Xray 构建无效 outbound
	if err := nodeSvc.MigrateEmptyTransportNodes(); err != nil {
		_ = logSvc.Error(constants.TagSubscription, "Transport 迁移失败", map[string]any{"error": err.Error()})
	}
	// 迁移后重建活跃节点 outbound，修复 stale config 文件
	if err := nodeSvc.RestoreActiveOutbound(); err != nil {
		_ = logSvc.Error(constants.TagXray, "重建活跃节点 outbound 失败", map[string]any{"error": err.Error()})
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &Container{
		Config:       cfg,
		Log:          logSvc,
		Subscription: subSvc,
		Xray:         xraySvc,
		Tun:          tunSvc,
		Node:         nodeSvc,
		Proxy:        proxyMgr,
		Geo:          geoMgr,
		ctx:          ctx,
		cancel:       cancel,
	}

	// 默认启动 xray（无节点时用 freedom outbound，有活跃节点则自动连接）
	go func() {
		if err := xraySvc.Start(); err != nil {
			_ = logSvc.Error(constants.TagXray, "vxray 启动时自动启动 xray 失败", map[string]any{"error": err.Error()})
		}
	}()

	return c, nil
}

func (c *Container) EnsureGeoFiles() {
	if c.geoFilesExist() {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				_ = c.Log.Error(constants.TagGeo, "Geo 下载 panic", map[string]any{"error": fmt.Sprintf("%v", r)})
			}
		}()
		_ = c.Log.Info(constants.TagGeo, "Geo 文件不存在，开始异步下载", nil)
		if err := c.Geo.DownloadAll(c.ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			_ = c.Log.Error(constants.TagGeo, "Geo 文件下载失败", map[string]any{"error": err.Error()})
			return
		}
		_ = c.Log.Info(constants.TagGeo, "Geo 文件下载完成", nil)
	}()
}

func (c *Container) geoFilesExist() bool {
	_, ipErr := os.Stat(c.Config.SystemMeta().Paths.GeoIP)
	_, siteErr := os.Stat(c.Config.SystemMeta().Paths.GeoSite)
	return ipErr == nil && siteErr == nil
}

func (c *Container) Close() error {
	if c == nil || c.Xray == nil {
		return nil
	}

	if c.cancel != nil {
		c.cancel()
	}

	// TUN 模式优先关闭：非交互式 kill root xray，不弹窗，不重启用户态 xray
	if c.Tun != nil {
		c.Tun.Shutdown()
	}

	if err := c.Xray.Stop(); err != nil {
		return fmt.Errorf("stop xray: %w", err)
	}

	return nil
}
