package config

import "time"

type UserSettings struct {
	SpeedTest SpeedTestSettings `json:"speedtest"`
	Geo       GeoSettings       `json:"geo"`
}

type SpeedTestTarget struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Icon     string `json:"icon,omitempty"`
	Latency  int64  `json:"latency"`
	Error    string `json:"error,omitempty"`
	TestedAt int64  `json:"tested_at,omitempty"`
}

type SpeedTestSettings struct {
	TargetURL      string            `json:"target_url"`
	Timeout        int               `json:"timeout"`
	Concurrency    int               `json:"concurrency"`
	WebsiteTargets []SpeedTestTarget `json:"website_targets"`
}

type GeoSettings struct {
	SelectedSource string `json:"selected_source"`
}

const (
	defaultTimeout     = 2000 * time.Millisecond
	defaultConcurrency = 64
)

func DefaultUserSettings() UserSettings {
	return UserSettings{
		SpeedTest: SpeedTestSettings{
			TargetURL:   "https://www.google.com/generate_204",
			Timeout:     int(defaultTimeout / time.Millisecond),
			Concurrency: defaultConcurrency,
			WebsiteTargets: []SpeedTestTarget{
				{Name: "Google", URL: "https://www.google.com/generate_204", Icon: "https://www.google.com/favicon.ico"},
				{Name: "GitHub", URL: "https://github.com/robots.txt", Icon: "https://github.com/favicon.ico"},
				{Name: "OpenAI", URL: "https://chatgpt.com/cdn-cgi/trace", Icon: "https://chatgpt.com/favicon.ico"},
				{Name: "Wikipedia", URL: "https://en.wikipedia.org/favicon.ico", Icon: "https://en.wikipedia.org/favicon.ico"},
				{Name: "YouTube", URL: "https://www.youtube.com/generate_204", Icon: "https://www.youtube.com/favicon.ico"},
				{Name: "Telegram", URL: "https://web.telegram.org/favicon.ico", Icon: "https://web.telegram.org/favicon.ico"},
			},
		},
		Geo: GeoSettings{SelectedSource: "loyalsoldier"},
	}
}

func (s UserSettings) TargetURL() string {
	return s.SpeedTest.TargetURL
}

func (s UserSettings) Timeout() time.Duration {
	if s.SpeedTest.Timeout > 0 {
		return time.Duration(s.SpeedTest.Timeout) * time.Millisecond
	}
	return defaultTimeout
}

func (s UserSettings) Concurrency() int {
	if s.SpeedTest.Concurrency > 0 {
		return s.SpeedTest.Concurrency
	}
	return defaultConcurrency
}

// 以下方法让 *State 实现 speedtest.Config 接口,实时读取最新设置,
// 避免在 service 构造时固化 UserSettings 值快照导致配置变更不生效。
func (s *State) TargetURL() string      { return s.UserSettings().TargetURL() }
func (s *State) Timeout() time.Duration { return s.UserSettings().Timeout() }
func (s *State) Concurrency() int       { return s.UserSettings().Concurrency() }

func (s *State) SaveUserSettings() error {
	s.mu.RLock()
	speedTest := s.settings.SpeedTest
	geo := s.settings.Geo
	s.mu.RUnlock()
	if err := s.store.Set("speedtest", speedTest); err != nil {
		return err
	}
	return s.store.Set("geo", geo)
}

func (s *State) UpdateAndSaveSettings(next UserSettings) error {
	s.mu.RLock()
	old := s.settings
	s.mu.RUnlock()

	s.mu.Lock()
	s.settings = next
	s.mu.Unlock()

	if err := s.SaveUserSettings(); err != nil {
		s.mu.Lock()
		s.settings = old
		s.mu.Unlock()
		return err
	}
	return nil
}

// UpdateWebsiteTargets 原子更新 WebsiteTargets 并持久化,避免长测速期间被覆盖。
func (s *State) UpdateWebsiteTargets(targets []SpeedTestTarget) error {
	s.mu.Lock()
	s.settings.SpeedTest.WebsiteTargets = targets
	speedTest := s.settings.SpeedTest
	s.mu.Unlock()
	return s.store.Set("speedtest", speedTest)
}
