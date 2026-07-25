package config

import "time"

type UserSettings struct {
	SpeedTest SpeedTestSettings `json:"speedtest"`
	Geo       GeoSettings       `json:"geo"`
}

type SpeedTestTarget struct {
	Name string `json:"name"`
	URL  string `json:"url"`
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
	defaultConcurrency = 20
)

func DefaultUserSettings() UserSettings {
	return UserSettings{
		SpeedTest: SpeedTestSettings{
			TargetURL:   "https://www.google.com/generate_204",
			Timeout:     int(defaultTimeout / time.Millisecond),
			Concurrency: defaultConcurrency,
			WebsiteTargets: []SpeedTestTarget{
				{Name: "Google", URL: "https://www.google.com/generate_204"},
				{Name: "YouTube", URL: "https://www.youtube.com/generate_204"},
				{Name: "GitHub", URL: "https://github.com/favicon.ico"},
				{Name: "Cloudflare", URL: "https://1.1.1.1/cdn-cgi/trace"},
				{Name: "Wikipedia", URL: "https://en.wikipedia.org/favicon.ico"},
				{Name: "Telegram", URL: "https://telegram.org/favicon.ico"},
			},
		},
		Geo: GeoSettings{SelectedSource: "loyalsoldier"},
	}
}

func (s UserSettings) GetTargetURL() string {
	return s.SpeedTest.TargetURL
}

func (s UserSettings) GetTimeout() time.Duration {
	if s.SpeedTest.Timeout > 0 {
		return time.Duration(s.SpeedTest.Timeout) * time.Millisecond
	}
	return defaultTimeout
}

func (s UserSettings) GetConcurrency() int {
	if s.SpeedTest.Concurrency > 0 {
		return s.SpeedTest.Concurrency
	}
	return defaultConcurrency
}

func (s *State) SaveUserSettings() error {
	s.mu.RLock()
	speedTest := s.settings.SpeedTest
	geo := s.settings.Geo
	s.mu.RUnlock()
	if err := s.settingRepo.Set("speedtest", speedTest); err != nil {
		return err
	}
	return s.settingRepo.Set("geo", geo)
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
