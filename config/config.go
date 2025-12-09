// Package config provides configuration management for localregistry.
// It reads configuration from the OS-appropriate user config directory
// (e.g., ~/.config/localregistry/config.yaml on Linux) using Viper,
// with support for environment variables and sensible defaults.
//
// Recognized environment variables (prefix LOCALREGISTRY_):
//   - LOCALREGISTRY_REGISTRY_URL (string)
//   - LOCALREGISTRY_REGISTRY_USERNAME (string)
//   - LOCALREGISTRY_REGISTRY_PASSWORD (string)
//   - LOCALREGISTRY_REGISTRY_INSECURE (bool)
//   - LOCALREGISTRY_REGISTRY_TIMEOUT (duration, e.g. "30s")
//   - LOCALREGISTRY_LOG_LEVEL (string, one of: debug, info, warn, error)
package config

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

//go:embed default_config.yaml
var defaultConfigYAML []byte

// RegistryConfig holds registry connection settings.
type RegistryConfig struct {
	URL      string        `mapstructure:"url"`
	Username string        `mapstructure:"username"`
	Password string        `mapstructure:"password"`
	Insecure bool          `mapstructure:"insecure"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

// Config holds the application configuration.
type Config struct {
	Registry RegistryConfig `mapstructure:"registry"`
	LogLevel string         `mapstructure:"log_level"`
}

// Load initializes and returns the application configuration.
// It loads configuration from the OS-appropriate config directory (e.g.,
// $HOME/.config/localregistry/config.yaml on Linux), with fallback to
// embedded defaults if the file doesn't exist.
// Environment variables prefixed with LOCALREGISTRY_ override config file values.
func Load() (*Config, error) {
	v := viper.New()

	// Load embedded defaults first (lowest priority)
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewReader(defaultConfigYAML)); err != nil {
		return nil, fmt.Errorf("failed to read embedded default config: %w", err)
	}

	// Set up user config file path
	configPath, err := configFilePath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine config path: %w", err)
	}

	// Merge user config file if it exists (higher priority than defaults)
	v.SetConfigFile(configPath)
	if err := v.MergeInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			// File not found is acceptable - we have defaults
		} else if os.IsNotExist(err) {
			// File or directory doesn't exist - acceptable
		} else {
			// Not a "file not found" error - this is a real error
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Enable environment variable binding (highest priority)
	v.SetEnvPrefix("LOCALREGISTRY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Unmarshal into struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// Validate ensures the configuration values are valid.
func (c *Config) Validate() error {
	if c.Registry.URL == "" {
		return fmt.Errorf("registry.url must not be empty")
	}
	if c.Registry.Timeout < 0 {
		return fmt.Errorf("registry.timeout must be non-negative")
	}

	// Normalize log level to lowercase for case-insensitive validation
	normalizedLevel := strings.ToLower(c.LogLevel)

	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[normalizedLevel] {
		return fmt.Errorf("log_level must be one of [debug, info, warn, error], got %q", c.LogLevel)
	}
	c.LogLevel = normalizedLevel
	return nil
}

// configFilePath returns the full path to the config file.
func configFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine config directory: %w", err)
	}
	return filepath.Join(configDir, "localregistry", "config.yaml"), nil
}

// ConfigFilePath returns the path to the config file that would be used.
// This is useful for debugging or displaying to users.
func ConfigFilePath() (string, error) {
	return configFilePath()
}

// DefaultConfig returns the embedded default configuration as bytes.
func DefaultConfig() []byte {
	return defaultConfigYAML
}
