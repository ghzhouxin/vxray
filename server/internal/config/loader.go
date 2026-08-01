package config

import (
	"sync"
)

// SettingStore 抽象设置读写,由 repository 层实现并注入。
type SettingStore interface {
	Get(key string, dest any) error
	Set(key string, value any) error
}

type State struct {
	system       SystemMeta
	settings     UserSettings
	activeNodeID uint
	mu           sync.RWMutex
	store        SettingStore
}

func LoadSystemMeta() SystemMeta {
	home := resolveHome()
	system := buildSystemMeta(home)
	applySystemOverrides(&system)
	return system
}

// Load 接收已构造的 SettingStore,不再自行创建 repository。
func Load(store SettingStore) (*State, error) {
	system := LoadSystemMeta()

	state := &State{
		system: system,
		store:  store,
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
	err := s.store.Get(key, dest)
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
