package api

import (
	"v2ray-server/internal/config"
	"v2ray-server/internal/dto"
)

func toSpeedTestTargetDTOs(targets []config.SpeedTestTarget) []dto.SpeedTestTargetDTO {
	result := make([]dto.SpeedTestTargetDTO, len(targets))
	for i, t := range targets {
		result[i] = dto.SpeedTestTargetDTO{Name: t.Name, URL: t.URL}
	}
	return result
}

func toSpeedTestTargets(targets []dto.SpeedTestTargetDTO) []config.SpeedTestTarget {
	result := make([]config.SpeedTestTarget, len(targets))
	for i, t := range targets {
		result[i] = config.SpeedTestTarget{Name: t.Name, URL: t.URL}
	}
	return result
}

func toUserSettingsDTO(settings config.UserSettings) *dto.UserSettingsDTO {
	return &dto.UserSettingsDTO{
		SpeedTest: dto.SpeedTestDTO{
			TargetURL:      settings.SpeedTest.TargetURL,
			Timeout:        settings.SpeedTest.Timeout,
			Concurrency:    settings.SpeedTest.Concurrency,
			WebsiteTargets: toSpeedTestTargetDTOs(settings.SpeedTest.WebsiteTargets),
		},
		Geo: dto.GeoSettingsDTO{SelectedSource: settings.Geo.SelectedSource},
	}
}

func toUserSettings(d dto.UserSettingsDTO) config.UserSettings {
	return config.UserSettings{
		SpeedTest: config.SpeedTestSettings{
			TargetURL:      d.SpeedTest.TargetURL,
			Timeout:        d.SpeedTest.Timeout,
			Concurrency:    d.SpeedTest.Concurrency,
			WebsiteTargets: toSpeedTestTargets(d.SpeedTest.WebsiteTargets),
		},
		Geo: config.GeoSettings{SelectedSource: d.Geo.SelectedSource},
	}
}

func toSystemMetaDTO(system config.SystemMeta) *dto.SystemMetaDTO {
	sources := make(map[string]dto.GeoURLDTO, len(system.Assets.GeoSources))
	for k, v := range system.Assets.GeoSources {
		sources[k] = dto.GeoURLDTO{GeoIP: v.GeoIP, GeoSite: v.GeoSite}
	}
	return &dto.SystemMetaDTO{
		Home: system.Home,
		Paths: dto.PathsMetaDTO{
			GeoDir:         system.Paths.GeoDir,
			XrayConfigPath: system.Paths.XrayConfigPath,
		},
		Server: dto.ServerDTO{Host: system.Server.Host, Port: system.Server.Port},
		Xray:   dto.XrayDTO{Binary: system.Xray.Binary},
		Assets: dto.AssetDTO{GeoSources: sources},
	}
}
