package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Test with missing config file (should use embedded defaults)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify embedded defaults
	if cfg.Registry.Host != "localhost" {
		t.Errorf("expected default registry.host 'localhost', got %q", cfg.Registry.Host)
	}
	if cfg.Registry.Port != 5000 {
		t.Errorf("expected default registry.port 5000, got %d", cfg.Registry.Port)
	}
	if cfg.Registry.Username != "" {
		t.Errorf("expected default registry.username '', got %q", cfg.Registry.Username)
	}
	if cfg.Registry.Password != "" {
		t.Errorf("expected default registry.password '', got %q", cfg.Registry.Password)
	}
	if cfg.Registry.Insecure != false {
		t.Errorf("expected default registry.insecure false, got %v", cfg.Registry.Insecure)
	}
	if cfg.Registry.Timeout != 30*time.Second {
		t.Errorf("expected default registry.timeout 30s, got %v", cfg.Registry.Timeout)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default log_level 'info', got %q", cfg.LogLevel)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Registry: RegistryConfig{
					Host:     "localhost",
					Port:     5000,
					Timeout:  30 * time.Second,
					Insecure: false,
				},
				LogLevel: "info",
			},
		},
		{
			name: "valid config with auth",
			cfg: Config{
				Registry: RegistryConfig{
					Host:     "registry.example.com",
					Port:     5001,
					Username: "admin",
					Password: "secret",
					Timeout:  60 * time.Second,
				},
				LogLevel: "debug",
			},
		},
		{
			name: "empty registry host",
			cfg: Config{
				Registry: RegistryConfig{
					Host:    "",
					Port:    5000,
					Timeout: 30 * time.Second,
				},
				LogLevel: "info",
			},
			wantErr: true,
		},
		{
			name: "zero registry port",
			cfg: Config{
				Registry: RegistryConfig{
					Host:    "localhost",
					Port:    0,
					Timeout: 30 * time.Second,
				},
				LogLevel: "info",
			},
			wantErr: true,
		},
		{
			name: "negative timeout",
			cfg: Config{
				Registry: RegistryConfig{
					Host:    "localhost",
					Port:    5000,
					Timeout: -1 * time.Second,
				},
				LogLevel: "info",
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			cfg: Config{
				Registry: RegistryConfig{
					Host:    "localhost",
					Port:    5000,
					Timeout: 30 * time.Second,
				},
				LogLevel: "trace",
			},
			wantErr: true,
		},
		{
			name: "case insensitive log level",
			cfg: Config{
				Registry: RegistryConfig{
					Host:    "localhost",
					Port:    5000,
					Timeout: 30 * time.Second,
				},
				LogLevel: "DEBUG",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			// Verify normalization for valid case-insensitive test
			if !tt.wantErr && tt.name == "case insensitive log level" {
				if tt.cfg.LogLevel != "debug" {
					t.Errorf("expected normalized log_level 'debug', got %q", tt.cfg.LogLevel)
				}
			}
		})
	}
}

func TestConfigFilePath(t *testing.T) {
	path, err := ConfigFilePath()
	if err != nil {
		t.Fatalf("ConfigFilePath failed: %v", err)
	}

	// Verify path is absolute
	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got %q", path)
	}
	// Verify it ends with the correct filename
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("expected path to end with 'config.yaml', got %q", path)
	}
	// Verify it contains the localregistry directory
	dir := filepath.Dir(path)
	if filepath.Base(dir) != "localregistry" {
		t.Errorf("expected parent directory to be 'localregistry', got %q", filepath.Base(dir))
	}
}

func TestLoadWithEnvOverride(t *testing.T) {
	// Set environment variables using nested key format (LR_REGISTRY_*)
	t.Setenv("LR_REGISTRY_HOST", "registry.example.com")
	t.Setenv("LR_REGISTRY_PORT", "5001")
	t.Setenv("LR_REGISTRY_USERNAME", "testuser")
	t.Setenv("LR_REGISTRY_PASSWORD", "testpass")
	t.Setenv("LR_REGISTRY_INSECURE", "true")
	t.Setenv("LR_REGISTRY_TIMEOUT", "60s")
	t.Setenv("LR_LOG_LEVEL", "DEBUG")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify environment overrides
	if cfg.Registry.Host != "registry.example.com" {
		t.Errorf("expected registry.host 'registry.example.com' from env, got %q", cfg.Registry.Host)
	}
	if cfg.Registry.Port != 5001 {
		t.Errorf("expected registry.port 5001 from env, got %d", cfg.Registry.Port)
	}
	if cfg.Registry.Username != "testuser" {
		t.Errorf("expected registry.username 'testuser' from env, got %q", cfg.Registry.Username)
	}
	if cfg.Registry.Password != "testpass" {
		t.Errorf("expected registry.password 'testpass' from env, got %q", cfg.Registry.Password)
	}
	if cfg.Registry.Insecure != true {
		t.Errorf("expected registry.insecure true from env, got %v", cfg.Registry.Insecure)
	}
	if cfg.Registry.Timeout != 60*time.Second {
		t.Errorf("expected registry.timeout 60s from env, got %v", cfg.Registry.Timeout)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log_level 'debug' (normalized) from env, got %q", cfg.LogLevel)
	}
}

func TestDefaultConfig(t *testing.T) {
	defaultCfg := DefaultConfig()
	if len(defaultCfg) == 0 {
		t.Error("expected non-empty default config bytes")
	}
}
