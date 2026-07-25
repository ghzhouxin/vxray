package geo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"v2ray-server/pkg/utils"
)

type Config interface {
	GeoIPPath() string
	GeoSitePath() string
	GeoDir() string
	GeoUpdateURL() (geoIP, geoSite string, ok bool)
}

type Manager struct {
	client *http.Client
	cfg    Config
}

func NewManager(cfg Config) *Manager {
	return &Manager{cfg: cfg, client: utils.LongRunningHTTPClient()}
}

func (m *Manager) GetFileInfo() map[string]any {
	info := map[string]any{"data_dir": m.cfg.GeoDir()}
	m.addFileInfo(info, "geoip", m.cfg.GeoIPPath())
	m.addFileInfo(info, "geosite", m.cfg.GeoSitePath())
	return info
}

func (m *Manager) addFileInfo(info map[string]any, name, path string) {
	if stat, err := os.Stat(path); err == nil {
		info[name+"_exists"] = true
		info[name+"_size"] = stat.Size()
		info[name+"_modified"] = stat.ModTime()
	} else {
		info[name+"_exists"] = false
	}
}

func (m *Manager) DownloadAll(ctx context.Context) error {
	if err := m.downloadGeoIP(ctx); err != nil {
		return fmt.Errorf("download geoip: %w", err)
	}
	if err := m.downloadGeoSite(ctx); err != nil {
		return fmt.Errorf("download geosite: %w", err)
	}
	return nil
}

func (m *Manager) downloadGeoIP(ctx context.Context) error {
	return m.downloadGeoFile(ctx, m.cfg.GeoUpdateURL, true)
}

func (m *Manager) downloadGeoSite(ctx context.Context) error {
	return m.downloadGeoFile(ctx, m.cfg.GeoUpdateURL, false)
}

func (m *Manager) downloadGeoFile(ctx context.Context, getURLs func() (string, string, bool), isIP bool) error {
	geoIP, geoSite, ok := getURLs()
	if !ok {
		return fmt.Errorf("unknown geo source")
	}
	geoURL := geoIP
	savePath := m.cfg.GeoIPPath()
	if !isIP {
		geoURL = geoSite
		savePath = m.cfg.GeoSitePath()
	}
	return m.download(ctx, geoURL, savePath)
}

func (m *Manager) download(ctx context.Context, url, path string) error {
	tmpFile := path + ".tmp"
	if _, err := os.Stat(tmpFile); err == nil {
		os.Remove(tmpFile)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("write file: %w", err)
	}

	if err := os.Rename(tmpFile, path); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("rename file: %w", err)
	}

	return nil
}
