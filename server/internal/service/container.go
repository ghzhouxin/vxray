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

	"gorm.io/gorm"
)

type geoConfig struct{ cfg *config.State }

func (c geoConfig) GetGeoIPPath() string { return c.cfg.SystemMeta().Paths.GeoIP }
func (c geoConfig) GetGeoSitePath() string {
	return c.cfg.SystemMeta().Paths.GeoSite
}
func (c geoConfig) GetGeoDir() string { return c.cfg.SystemMeta().Paths.GeoDir }
func (c geoConfig) GetGeoUpdateURL() (string, string, bool) {
	return c.cfg.GetGeoUpdateURL()
}

type Container struct {
	Config       *config.State
	Log          *LogService
	Subscription *SubscriptionService
	Xray         *XrayService
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
	nodeSvc := NewNodeService(xraySvc, nodeRepo, cfg, logSvc)
	proxyMgr := proxy.NewManager()
	geoMgr := geo.NewManager(geoConfig{cfg: cfg})

	xraySvc.GetManager().SetLogCallback(func(level, message string) {
		_ = logSvc.LogLevel(constants.TagXray, level, message, nil)
	})

	ctx, cancel := context.WithCancel(context.Background())
	return &Container{
		Config:       cfg,
		Log:          logSvc,
		Subscription: subSvc,
		Xray:         xraySvc,
		Node:         nodeSvc,
		Proxy:        proxyMgr,
		Geo:          geoMgr,
		ctx:          ctx,
		cancel:       cancel,
	}, nil
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

	if err := c.Xray.Stop(); err != nil {
		return fmt.Errorf("stop xray: %w", err)
	}

	return nil
}
