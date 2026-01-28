package agent

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the agent configuration.
type Config struct {
	Relay   RelayConfig   `yaml:"relay"`
	Proxy   ProxyConfig   `yaml:"proxy"`
	Backend BackendConfig `yaml:"backend"`
	Auth    AuthConfig    `yaml:"auth"`
	Tunnel  TunnelConfig  `yaml:"tunnel"`
}

// RelayConfig specifies the relay server websocket endpoint.
type RelayConfig struct {
	URL string `yaml:"url"`
}

// ProxyConfig controls the residential proxy settings.
type ProxyConfig struct {
	URL             string        `yaml:"url"`
	VerifyRouting   bool          `yaml:"verify_routing"`
	HealthTimeout   time.Duration `yaml:"health_timeout"`
	RecheckInterval time.Duration `yaml:"recheck_interval"`
}

// BackendConfig specifies the local backend target.
type BackendConfig struct {
	TargetURL string `yaml:"target_url"`
}

// AuthConfig holds the shared secret for hmac authentication.
type AuthConfig struct {
	SharedSecret string `yaml:"shared_secret"`
}

// TunnelConfig controls reconnection and keepalive behaviour.
type TunnelConfig struct {
	ReconnectDelay    time.Duration `yaml:"reconnect_delay"`
	MaxReconnectDelay time.Duration `yaml:"max_reconnect_delay"`
	PingInterval      time.Duration `yaml:"ping_interval"`
}

// LoadConfig reads and parses an agent configuration file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	cfg := &Config{
		Backend: BackendConfig{TargetURL: "http://127.0.0.1:8080"},
		Proxy: ProxyConfig{
			VerifyRouting:   true,
			HealthTimeout:   10 * time.Second,
			RecheckInterval: 5 * time.Minute,
		},
		Tunnel: TunnelConfig{
			ReconnectDelay:    2 * time.Second,
			MaxReconnectDelay: 60 * time.Second,
			PingInterval:      15 * time.Second,
		},
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Relay.URL == "" {
		return nil, fmt.Errorf("relay.url is required")
	}
	if cfg.Auth.SharedSecret == "" {
		return nil, fmt.Errorf("auth.shared_secret is required")
	}
	return cfg, nil
}
