// Package config provides configuration management for localregistry.
// It reads configuration from the OS-appropriate user config directory
// (e.g., ~/.config/localregistry/config.yaml on Linux) using Viper,
// with support for environment variables and sensible defaults.
//
// Recognized environment variables:
//   - LOCALREGISTRY_PORT (integer, 1-65535)
//   - LOCALREGISTRY_HOST (string, non-empty)
//   - LOCALREGISTRY_LOG_LEVEL (string, one of: debug, info, warn, error, case-insensitive)
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config holds the application configuration.
type Config struct {
	Port     int    `mapstructure:"port"`
	Host     string `mapstructure:"host"`
	LogLevel string `mapstructure:"log_level"`
}

// Load initializes and returns the application configuration.
// It loads configuration from the OS-appropriate config directory (e.g.,
// $HOME/.config/localregistry/config.yaml on Linux), with fallback to
// defaults if the file doesn't exist.
// Environment variables prefixed with LOCALREGISTRY_ override config file values.
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults (lowest priority)
	setDefaults(v)

	// Set up config file path
	if err := setupConfigFile(v); err != nil {
		return nil, fmt.Errorf("failed to setup config file: %w", err)
	}

	// Enable environment variable binding (higher priority than config file)
	v.SetEnvPrefix("LOCALREGISTRY")
	v.AutomaticEnv()

	// Read config file (ignore if not found)
	if err := v.ReadInConfig(); err != nil {
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
// Log level is normalized to lowercase for case-insensitive comparison.
func (c *Config) Validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}
	if c.Host == "" {
		return fmt.Errorf("host must not be empty")
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

// setDefaults configures default values for all configuration options.
func setDefaults(v *viper.Viper) {
	v.SetDefault("port", 5000)
	v.SetDefault("host", "localhost")
	v.SetDefault("log_level", "info")
}

// setupConfigFile configures Viper to read from the OS-appropriate config directory.
// On Linux: $HOME/.config/localregistry/config.yaml
// On macOS: $HOME/Library/Application Support/localregistry/config.yaml
// On Windows: %AppData%/localregistry/config.yaml
func setupConfigFile(v *viper.Viper) error {
	configPath, err := configFilePath()
	if err != nil {
		return err
	}

	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

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
