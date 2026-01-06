package config

import (
	"fmt"
	"os"
	"time"

	"github.com/pperesbr/gokit/pkg/tunnel"
	"gopkg.in/yaml.v3"
)

// TunnelConfig defines the configuration for a network tunnel, including its name, remote host, and port mappings.
type TunnelConfig struct {
	Name        string            `yaml:"name"`
	RemoteHost  string            `yaml:"remoteHost"`
	RemotePort  int               `yaml:"remotePort"`
	LocalPort   int               `yaml:"localPort"`
	AutoRestart AutoRestartConfig `yaml:"autoRestart"`
}

// AutoRestartConfig defines settings for automatic restart functionality, including enabling and restart intervals.
type AutoRestartConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
}

// Config represents the top-level configuration that includes SSH settings and a list of network tunnel configurations.
type Config struct {
	SSH           tunnel.SSHConfig `yaml:"ssh"`
	TunnelConfigs []TunnelConfig   `yaml:"tunnels"`
}

// Load reads a configuration file from the specified path, parses it, and validates the resulting Config object.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	expanded := os.ExpandEnv(string(data))
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// Validate checks the configuration for errors such as missing fields, invalid values, or duplicate tunnel definitions.
func (c *Config) Validate() error {
	if err := c.SSH.Validate(); err != nil {
		return fmt.Errorf("ssh: %w", err)
	}

	if len(c.TunnelConfigs) == 0 {
		return fmt.Errorf("at least one tunnel is required")
	}

	names := make(map[string]bool)
	localPorts := make(map[int]bool)

	for i, t := range c.TunnelConfigs {
		if t.Name == "" {
			return fmt.Errorf("tunnels[%d].name is required", i)
		}

		if names[t.Name] {
			return fmt.Errorf("duplicate tunnel name: %s", t.Name)
		}
		names[t.Name] = true

		if t.RemoteHost == "" {
			return fmt.Errorf("tunnels[%d].remoteHost is required", i)
		}

		if t.RemotePort <= 0 {
			return fmt.Errorf("tunnels[%d].remotePort must be greater than 0", i)
		}

		if t.LocalPort <= 0 {
			return fmt.Errorf("tunnels[%d].localPort must be greater than 0", i)
		}

		if localPorts[t.LocalPort] {
			return fmt.Errorf("duplicate localPort: %d", t.LocalPort)
		}

		localPorts[t.LocalPort] = true

		if t.AutoRestart.Enabled && t.AutoRestart.Interval <= 0 {
			return fmt.Errorf("tunnels[%d].autoRestart.interval must be greater than 0 when enabled", i)
		}
	}

	return nil
}
