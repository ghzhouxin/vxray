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

func (c geoConfig) GeoIPPath() string  { return c.cfg.SystemMeta().Paths.GeoIP }
func (c geoConfig) GeoSitePath() string { return c.cfg.SystemMeta().Paths.GeoSite }
func (c geoConfig) GeoDir() string     { return c.cfg.SystemMeta().Paths.GeoDir }
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

	paths := cfg.SystemMeta().Paths
	meta := cfg.SystemMeta().Xray
	logSvc := NewLogService(db, paths.XrayLogPath, paths.TunLogPath)
	subSvc := NewSubscriptionService(db, logSvc, nodeRepo)

	userMgr := xray.NewManager(xray.Options{
		Binary:   meta.Binary,
		AssetDir: paths.GeoDir,
		LogFile:  paths.XrayLogPath,
		PidFile:  paths.XrayPidPath,
		OnLog: func(level, line string) {
			if level == constants.LevelWarn || level == constants.LevelError {
				_ = logSvc.LogLevel(constants.TagXray, level, line, nil)
			}
		},
	})
	rootMgr := xray.NewManager(xray.Options{
		Binary:   meta.Binary,
		AssetDir: paths.GeoDir,
		AsRoot:   true,
		LogFile:  paths.TunLogPath,
		PidFile:  paths.TunPidPath,
		OnLog: func(level, line string) {
			if level == constants.LevelWarn || level == constants.LevelError {
				_ = logSvc.LogLevel(constants.TagTun, level, line, nil)
			}
		},
	})
	userMgr.CleanupStale()
	rootMgr.CleanupStale()

	xraySvc := NewXrayService(nodeRepo, cfg, logSvc, userMgr)
	tunSvc := NewTunService(rootMgr, userMgr, cfg, logSvc)
	xraySvc.SetTunHandlers(tunSvc.IsEnabled, tunSvc.Disable, tunSvc.RestartRootProcess)
	userMgr.SetCrashCallback(xraySvc.handleCrash)
	rootMgr.SetCrashCallback(tunSvc.handleRootCrash)
	nodeSvc := NewNodeService(xraySvc, nodeRepo, cfg, logSvc)
	proxyMgr := proxy.NewManager(proxy.Options{
		HTTPPort:  constants.ProxyHTTPPort,
		SOCKSPort: constants.ProxySOCKSPort,
	})
	geoMgr := geo.NewManager(geoConfig{cfg: cfg})

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

	go func() {
		if err := xraySvc.Start(); err != nil {
			_ = logSvc.Error(constants.TagXray, "vxray 启动时自动启动 xray 失败", map[string]any{"error": err.Error()})
		}
	}()

	_ = logSvc.Info(constants.TagApp, "vxray 已启动", map[string]any{"home": cfg.SystemMeta().Home})

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

	if c.Tun != nil {
		c.Tun.Shutdown()
	}

	if err := c.Xray.Stop(); err != nil {
		return fmt.Errorf("stop xray: %w", err)
	}

	return nil
}