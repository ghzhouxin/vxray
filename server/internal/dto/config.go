package dto

type ConfigResponse struct {
	Settings *UserSettingsDTO `json:"settings"`
	System   *SystemMetaDTO   `json:"system"`
}

type UserSettingsDTO struct {
	SpeedTest SpeedTestDTO   `json:"speedtest"`
	Geo       GeoSettingsDTO `json:"geo"`
}

type ServerDTO struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type SystemMetaDTO struct {
	Home   string       `json:"home"`
	Paths  PathsMetaDTO `json:"paths"`
	Server ServerDTO    `json:"server"`
	Xray   XrayDTO      `json:"xray"`
	Assets AssetDTO     `json:"assets"`
}

type XrayDTO struct {
	Binary string `json:"binary"`
}

type PathsMetaDTO struct {
	GeoDir         string `json:"geo_dir"`
	XrayConfigPath string `json:"xray_config_path"`
}

type SpeedTestTargetDTO struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Icon     string `json:"icon,omitempty"`
	Latency  int64  `json:"latency"`
	Error    string `json:"error,omitempty"`
	TestedAt int64  `json:"tested_at,omitempty"`
}

type SpeedTestDTO struct {
	TargetURL      string               `json:"target_url"`
	Timeout        int                  `json:"timeout"`
	Concurrency    int                  `json:"concurrency"`
	WebsiteTargets []SpeedTestTargetDTO `json:"website_targets"`
}

type AssetDTO struct {
	GeoSources map[string]GeoURLDTO `json:"geo_sources"`
}

type GeoSettingsDTO struct {
	SelectedSource string `json:"selected_source"`
}

type GeoURLDTO struct {
	GeoIP   string `json:"geoip"`
	GeoSite string `json:"geosite"`
}
