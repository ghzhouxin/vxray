package api

import (
	"v2ray-server/internal/config"
	"v2ray-server/internal/dto"
	"v2ray-server/internal/model"
	"v2ray-server/internal/service"
)

// toNodeInfo converts a single node to its DTO representation.
func toNodeInfo(node *model.Node) *dto.NodeInfo {
	if node == nil {
		return nil
	}
	infos := service.ToNodeInfos([]*model.Node{node})
	if len(infos) == 0 {
		return nil
	}
	return &infos[0]
}

func toSubscriptionDTO(m model.Subscription) dto.SubscriptionDTO {
	return dto.SubscriptionDTO{
		ID:             m.ID,
		Name:           m.Name,
		URL:            m.URL,
		LastSyncAt:     m.LastSyncAt,
		LastSyncStatus: m.LastSyncStatus,
		NodeCount:      m.NodeCount,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

func toSubscriptionDTOs(items []model.Subscription) []dto.SubscriptionDTO {
	result := make([]dto.SubscriptionDTO, len(items))
	for i, item := range items {
		result[i] = toSubscriptionDTO(item)
	}
	return result
}

func toLogDTO(m model.Log) dto.LogDTO {
	return dto.LogDTO{
		ID:        m.ID,
		Message:   m.Message,
		Tag:       m.Tag,
		Detail:    m.Detail,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

func toLogDTOs(items []model.Log) []dto.LogDTO {
	result := make([]dto.LogDTO, len(items))
	for i, item := range items {
		result[i] = toLogDTO(item)
	}
	return result
}

func toSpeedTestTargetDTOs(targets []config.SpeedTestTarget) []dto.SpeedTestTargetDTO {
	result := make([]dto.SpeedTestTargetDTO, len(targets))
	for i, t := range targets {
		result[i] = dto.SpeedTestTargetDTO{
			Name:     t.Name,
			URL:      t.URL,
			Icon:     t.Icon,
			Latency:  t.Latency,
			Error:    t.Error,
			TestedAt: t.TestedAt,
		}
	}
	return result
}

func toSpeedTestTargets(targets []dto.SpeedTestTargetDTO) []config.SpeedTestTarget {
	result := make([]config.SpeedTestTarget, len(targets))
	for i, t := range targets {
		result[i] = config.SpeedTestTarget{
			Name:     t.Name,
			URL:      t.URL,
			Icon:     t.Icon,
			Latency:  t.Latency,
			Error:    t.Error,
			TestedAt: t.TestedAt,
		}
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
