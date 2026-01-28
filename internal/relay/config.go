package relay

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the relay server configuration.
type Config struct {
	Listen ListenConfig `yaml:"listen"`
	TLS    TLSConfig    `yaml:"tls"`
	Auth   AuthConfig   `yaml:"auth"`
	Tunnel TunnelConfig `yaml:"tunnel"`
}

// ListenConfig specifies the address to bind on.
type ListenConfig struct {
	Addr string `yaml:"addr"`
}

// TLSConfig controls tls certificate settings.
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// AuthConfig holds the shared secret for hmac authentication.
type AuthConfig struct {
	SharedSecret string `yaml:"shared_secret"`
}

// TunnelConfig controls tunnel behaviour.
type TunnelConfig struct {
	Path           string        `yaml:"path"`
	PingInterval   time.Duration `yaml:"ping_interval"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
}

// LoadConfig reads and parses a relay configuration file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	cfg := &Config{
		Listen: ListenConfig{Addr: ":8080"},
		Tunnel: TunnelConfig{
			Path:           "/_tunnel/ws",
			PingInterval:   15 * time.Second,
			RequestTimeout: 60 * time.Second,
		},
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Auth.SharedSecret == "" {
		return nil, fmt.Errorf("auth.shared_secret is required")
	}
	return cfg, nil
}
