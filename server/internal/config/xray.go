package config

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"

	"v2ray-server/pkg/types"
	"v2ray-server/pkg/utils"
)

//go:embed xray.config.default.json
var embedded embed.FS

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

func (s *State) XrayConfigContent() (string, error) {
	data, err := os.ReadFile(s.SystemMeta().Paths.XrayConfigPath)
	if errors.Is(err, fs.ErrNotExist) {
		return s.DefaultXrayConfigContent()
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *State) DefaultXrayConfigContent() (string, error) {
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

func (s *State) XrayPorts() (*XrayPorts, error) {
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
	// 浅拷贝避免污染调用方的 outbound（tag 赋值会修改 map 顶层 key）
	outbound = copyMap(outbound)
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

// copyMap 浅拷贝 map[string]any。
// UpdateXrayOutbound 只新增顶层 key（"tag"），嵌套 map 不修改，浅拷贝即可避免污染调用方的 outbound。
func copyMap(src types.Map) types.Map {
	if src == nil {
		return nil
	}
	dst := make(types.Map, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (s *State) ActiveNodeID() uint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeNodeID
}

func (s *State) SetActiveNodeID(id uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeNodeID = id
}

func (s *State) GeoUpdateURL() (geoIP, geoSite string, ok bool) {
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
