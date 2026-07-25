package config

import (
	"sync"

	"v2ray-server/internal/repository"

	"gorm.io/gorm"
)

type State struct {
	system       SystemMeta
	settings     UserSettings
	activeNodeID uint
	mu           sync.RWMutex
	settingRepo  *repository.SettingRepository
}

func LoadSystemMeta() SystemMeta {
	return loadSystemMeta()
}

func Load(db *gorm.DB) (*State, error) {
	system := loadSystemMeta()

	state := &State{
		system:      system,
		settingRepo: repository.NewSettingRepository(db),
	}

	if err := state.loadSettings(); err != nil {
		return nil, err
	}

	if err := state.ensureXrayConfig(); err != nil {
		return nil, err
	}
	return state, nil
}

func (s *State) loadSettings() error {
	s.settings = DefaultUserSettings()
	speedTestLoaded := s.loadSettingKeySilent("speedtest", &s.settings.SpeedTest)
	geoLoaded := s.loadSettingKeySilent("geo", &s.settings.Geo)
	if !speedTestLoaded || !geoLoaded {
		return s.SaveUserSettings()
	}
	return nil
}

func (s *State) loadSettingKeySilent(key string, dest any) bool {
	err := s.settingRepo.Get(key, dest)
	return err == nil
}

func (s *State) SystemMeta() SystemMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSystemMeta(s.system)
}

func (s *State) UserSettings() UserSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

func cloneSystemMeta(system SystemMeta) SystemMeta {
	cloned := system
	if system.Assets.GeoSources != nil {
		cloned.Assets.GeoSources = make(map[string]GeoSource, len(system.Assets.GeoSources))
		for name, source := range system.Assets.GeoSources {
			cloned.Assets.GeoSources[name] = source
		}
	}
	return cloned
}
