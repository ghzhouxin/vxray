package config

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"v2ray-server/pkg/types"
	"v2ray-server/pkg/utils"
)

//go:embed xray.config.default.json
var embedded embed.FS

type WebsiteSpeedTestResult struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Latency int64  `json:"latency"`
	Error   string `json:"error,omitempty"`
}

func (s *State) ReadXrayConfig() (map[string]any, error) {
	var cfg map[string]any
	if err := utils.ReadJSON(s.SystemMeta().Paths.XrayConfigPath, &cfg); err != nil {
		return nil, fmt.Errorf("read xray config: %w", err)
	}
	return cfg, nil
}

func (s *State) WriteXrayConfig(cfg any) error {
	if err := utils.WriteJSON(s.SystemMeta().Paths.XrayConfigPath, cfg); err != nil {
		return fmt.Errorf("write xray config: %w", err)
	}
	return nil
}

func (s *State) UpdateXrayConfig(updater func(map[string]any) error) error {
	cfg, err := s.ReadXrayConfig()
	if err != nil {
		return err
	}
	if err := updater(cfg); err != nil {
		return err
	}
	return s.WriteXrayConfig(cfg)
}

func (s *State) GetXrayConfigContent() (string, error) {
	data, err := os.ReadFile(s.SystemMeta().Paths.XrayConfigPath)
	if os.IsNotExist(err) {
		return s.GetDefaultXrayConfigContent()
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *State) GetDefaultXrayConfigContent() (string, error) {
	cfg, err := loadDefaultXrayConfig()
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal default xray config: %w", err)
	}
	return string(data), nil
}

func loadDefaultXrayConfig() (map[string]any, error) {
	data, err := embedded.ReadFile("xray.config.default.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded xray config: %w", err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

type XrayPorts struct {
	HTTPPort  int
	SOCKSPort int
}

func (s *State) GetXrayPorts() (*XrayPorts, error) {
	cfg, err := s.ReadXrayConfig()
	if err != nil {
		return nil, err
	}
	ports := &XrayPorts{}
	inbounds, ok := cfg["inbounds"].([]any)
	if !ok {
		return ports, nil
	}
	for _, ib := range inbounds {
		inbound, ok := ib.(map[string]any)
		if !ok {
			continue
		}
		protocol, ok := inbound["protocol"].(string)
		if !ok {
			continue
		}
		port, ok := inbound["port"].(float64)
		if !ok {
			continue
		}
		switch protocol {
		case "http":
			ports.HTTPPort = int(port)
		case "socks":
			ports.SOCKSPort = int(port)
		}
	}
	return ports, nil
}

func (s *State) UpdateXrayOutbound(outbound types.Map) error {
	// 深拷贝避免污染调用方的 outbound（normalizeWSSettings 和 tag 赋值会修改 map）
	outbound = deepCopyMap(outbound)
	normalizeWSSettings(outbound)
	return s.UpdateXrayConfig(func(cfg map[string]any) error {
		outbounds, ok := cfg["outbounds"].([]any)
		if !ok {
			defaultCfg, err := loadDefaultXrayConfig()
			if err != nil {
				return err
			}
			outbounds, _ = defaultCfg["outbounds"].([]any)
		}

		if outbounds == nil {
			outbounds = []any{}
		}

		outbound["tag"] = "proxy"

		newOutbounds := make([]any, 0, len(outbounds)+1)
		newOutbounds = append(newOutbounds, outbound)
		for _, ob := range outbounds {
			if obMap, ok := ob.(map[string]any); ok {
				if tag, ok := obMap["tag"].(string); !ok || tag != "proxy" {
					newOutbounds = append(newOutbounds, ob)
				}
			} else {
				newOutbounds = append(newOutbounds, ob)
			}
		}

		cfg["outbounds"] = newOutbounds
		return nil
	})
}

// deepCopyMap 通过 JSON 序列化实现 map[string]any 的深拷贝，避免共享底层引用。
func deepCopyMap(src types.Map) types.Map {
	if src == nil {
		return nil
	}
	data, err := json.Marshal(src)
	if err == nil {
		var dst types.Map
		if err := json.Unmarshal(data, &dst); err == nil {
			return dst
		}
	}
	// JSON 失败时回退到浅拷贝
	dst := make(types.Map, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// normalizeWSSettings migrates deprecated wsSettings.headers.Host to the
// top-level host field. xray-core v26 auto-migrates this in Build() with a
// deprecation warning; normalizing here eliminates the warning and protects
// against future removal. Idempotent: no-op for already-new-format data.
func normalizeWSSettings(outbound types.Map) {
	streamSettings, ok := outbound["streamSettings"].(types.Map)
	if !ok {
		return
	}
	wsSettings, ok := streamSettings["wsSettings"].(types.Map)
	if !ok {
		return
	}
	headers, ok := wsSettings["headers"].(types.Map)
	if !ok {
		return
	}
	if host, ok := popHeaderHost(headers); ok {
		if existingHost, exists := wsSettings["host"]; !exists || existingHost == "" {
			wsSettings["host"] = host
		}
	}
	if len(headers) == 0 {
		delete(wsSettings, "headers")
	}
}

// popHeaderHost finds and removes the Host header (case-insensitive) from wsSettings.headers,
// returning its value. xray-core v26 deprecates headers.Host in favor of top-level host field.
func popHeaderHost(headers types.Map) (string, bool) {
	for k, v := range headers {
		if strings.ToLower(k) == "host" {
			if hostStr, ok := v.(string); ok {
				delete(headers, k)
				return hostStr, true
			}
		}
	}
	return "", false
}

func (s *State) GetActiveNodeID() uint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeNodeID
}

func (s *State) SetActiveNodeID(id uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeNodeID = id
}

func (s *State) GetGeoUpdateURL() (geoIP, geoSite string, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sourceName := s.settings.Geo.SelectedSource
	if sourceName == "" {
		sourceName = "loyalsoldier"
	}
	source, exists := s.system.Assets.GeoSources[sourceName]
	if !exists {
		return "", "", false
	}
	return source.GeoIP, source.GeoSite, true
}

func (s *State) SaveWebsiteSpeedTestResults(results []WebsiteSpeedTestResult) error {
	data := append([]WebsiteSpeedTestResult(nil), results...)
	return utils.WriteJSON(s.websiteSpeedTestCachePath(), data)
}

func (s *State) GetWebsiteSpeedTestResults() ([]WebsiteSpeedTestResult, error) {
	var results []WebsiteSpeedTestResult
	if err := utils.ReadJSON(s.websiteSpeedTestCachePath(), &results); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read speedtest cache: %w", err)
	}
	return append([]WebsiteSpeedTestResult(nil), results...), nil
}

func (s *State) ensureXrayConfig() error {
	system := s.SystemMeta()
	if _, err := exec.LookPath(system.Xray.Binary); err != nil {
		return fmt.Errorf("xray binary not found: %w", err)
	}

	path := system.Paths.XrayConfigPath
	data, err := embedded.ReadFile("xray.config.default.json")
	if err != nil {
		return fmt.Errorf("read embedded xray config: %w", err)
	}
	return utils.EnsureFile(path, data)
}

func (s *State) websiteSpeedTestCachePath() string {
	return filepath.Join(s.SystemMeta().Home, "speedtest.cache.json")
}
