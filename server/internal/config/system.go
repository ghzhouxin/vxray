package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type SystemMeta struct {
	Home   string     `json:"home"`
	Paths  PathsMeta  `json:"paths"`
	Server ServerMeta `json:"server"`
	Xray   XrayMeta   `json:"xray"`
	Web    WebMeta    `json:"web"`
	Assets AssetsMeta `json:"assets"`
}

type PathsMeta struct {
	Database       string `json:"database"`
	GeoDir         string `json:"geo_dir"`
	XrayConfigPath string `json:"xray_config_path"`
	GeoIP          string `json:"geoip"`
	GeoSite        string `json:"geosite"`
}

type ServerMeta struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type XrayMeta struct {
	Binary string `json:"binary"`
}

type WebMeta struct {
	Root string `json:"root"`
}

type AssetsMeta struct {
	GeoSources map[string]GeoSource `json:"geo_sources"`
}

type GeoSource struct {
	GeoIP   string `json:"geoip"`
	GeoSite string `json:"geosite"`
}

func loadSystemMeta() SystemMeta {
	home := resolveHome()
	system := buildSystemMeta(home)
	applySystemOverrides(&system)
	return system
}

func resolveHome() string {
	home := strings.TrimSpace(os.Getenv("VXRAY_HOME"))
	if home != "" {
		return home
	}
	userHome, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(userHome) == "" {
		userHome = "."
	}
	return filepath.Join(userHome, ".vxray")
}

func buildSystemMeta(home string) SystemMeta {
	return SystemMeta{
		Home: home,
		Paths: PathsMeta{
			Database:       filepath.Join(home, "data/vxray.db"),
			GeoDir:         filepath.Join(home, "geo"),
			XrayConfigPath: filepath.Join(home, "xray/config.json"),
			GeoIP:          filepath.Join(home, "geo/geoip.dat"),
			GeoSite:        filepath.Join(home, "geo/geosite.dat"),
		},
		Server: ServerMeta{Host: "127.0.0.1", Port: 11888},
		Xray:   XrayMeta{Binary: "xray"},
		Web:    WebMeta{Root: ""},
		Assets: AssetsMeta{
			GeoSources: map[string]GeoSource{
				"loyalsoldier": {
					GeoIP:   "https://github.mk/github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat",
					GeoSite: "https://github.mk/github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat",
				},
			},
		},
	}
}

func applySystemOverrides(system *SystemMeta) {
	if envBinary := strings.TrimSpace(os.Getenv("VXRAY_XRAY_BINARY")); envBinary != "" {
		system.Xray.Binary = envBinary
	}
	if envPort := strings.TrimSpace(os.Getenv("VXRAY_SERVER_PORT")); envPort != "" {
		port, err := strconv.Atoi(envPort)
		if err == nil && port > 0 && port <= 65535 {
			system.Server.Port = port
		}
	}
	if envWebRoot := strings.TrimSpace(os.Getenv("VXRAY_WEB_ROOT")); envWebRoot != "" {
		system.Web.Root = envWebRoot
	}
}
