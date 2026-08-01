package proxy

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

type ProxySettings struct {
	HTTPHost  string
	HTTPPort  int
	HTTPSHost string
	HTTPSPort int
	SOCKSHost string
	SOCKSPort int
}

type Options struct {
	HTTPPort  int
	SOCKSPort int
}

type Manager struct {
	mu       sync.RWMutex
	enabled  bool
	settings ProxySettings
}

func NewManager(opts Options) *Manager {
	return &Manager{
		settings: ProxySettings{
			HTTPHost:  "127.0.0.1",
			HTTPPort:  opts.HTTPPort,
			HTTPSHost: "127.0.0.1",
			HTTPSPort: opts.HTTPPort, // HTTPS 与 HTTP 同端口,简化配置
			SOCKSHost: "127.0.0.1",
			SOCKSPort: opts.SOCKSPort,
		},
	}
}

func (m *Manager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

func (m *Manager) Toggle(enable bool) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		m.mu.RLock()
		settings := m.settings
		m.mu.RUnlock()

		if err := m.setMacOSProxy(settings, enable); err != nil {
			return "", err
		}

		m.mu.Lock()
		m.enabled = enable
		m.mu.Unlock()

		if enable {
			return "system proxy enabled", nil
		}
		return "system proxy disabled", nil
	default:
		return "", fmt.Errorf("system proxy only supported on macOS")
	}
}

func (m *Manager) setMacOSProxy(settings ProxySettings, enable bool) error {
	services, err := m.getNetworkServices()
	if err != nil {
		return err
	}

	state := "off"
	if enable {
		state = "on"
	}

	var errs []error
	for _, s := range services {
		if enable {
			runProxyCmd(&errs, "setwebproxy", s, settings.HTTPHost, strconv.Itoa(settings.HTTPPort))
			runProxyCmd(&errs, "setsecurewebproxy", s, settings.HTTPSHost, strconv.Itoa(settings.HTTPSPort))
			runProxyCmd(&errs, "setsocksfirewallproxy", s, settings.SOCKSHost, strconv.Itoa(settings.SOCKSPort))
		}
		runProxyCmd(&errs, "setwebproxystate", s, state)
		runProxyCmd(&errs, "setsecurewebproxystate", s, state)
		runProxyCmd(&errs, "setsocksfirewallproxystate", s, state)
	}
	return errors.Join(errs...)
}

func runProxyCmd(errs *[]error, args ...string) {
	if err := exec.Command("networksetup", args...).Run(); err != nil {
		*errs = append(*errs, fmt.Errorf("networksetup %s: %w", strings.Join(args, " "), err))
	}
}

func (m *Manager) getNetworkServices() ([]string, error) {
	output, err := exec.Command("networksetup", "-listallnetworkservices").Output()
	if err != nil {
		return nil, err
	}

	var services []string
	for i, line := range strings.Split(string(output), "\n") {
		if i > 0 && line != "" && !strings.HasPrefix(line, "*") {
			services = append(services, line)
		}
	}
	return services, nil
}
