package config

import (
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	// Test with missing config file (should use defaults)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify defaults
	if cfg.Port != 5000 {
		t.Errorf("expected default port 5000, got %d", cfg.Port)
	}
	if cfg.Host != "localhost" {
		t.Errorf("expected default host 'localhost', got %q", cfg.Host)
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
			cfg:  Config{Port: 8080, Host: "0.0.0.0", LogLevel: "debug"},
		},
		{
			name:    "invalid port - too low",
			cfg:     Config{Port: 0, Host: "localhost", LogLevel: "info"},
			wantErr: true,
		},
		{
			name:    "invalid port - too high",
			cfg:     Config{Port: 70000, Host: "localhost", LogLevel: "info"},
			wantErr: true,
		},
		{
			name:    "empty host",
			cfg:     Config{Port: 8080, Host: "", LogLevel: "info"},
			wantErr: true,
		},
		{
			name:    "invalid log level",
			cfg:     Config{Port: 8080, Host: "localhost", LogLevel: "trace"},
			wantErr: true,
		},
		{
			name: "case insensitive log level",
			cfg:  Config{Port: 8080, Host: "localhost", LogLevel: "INFO"},
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
				if tt.cfg.LogLevel != "info" {
					t.Errorf("expected normalized log_level 'info', got %q", tt.cfg.LogLevel)
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
	// Set environment variables
	t.Setenv("LOCALREGISTRY_PORT", "9000")
	t.Setenv("LOCALREGISTRY_HOST", "127.0.0.1")
	t.Setenv("LOCALREGISTRY_LOG_LEVEL", "DEBUG")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify environment overrides
	if cfg.Port != 9000 {
		t.Errorf("expected port 9000 from env, got %d", cfg.Port)
	}
	if cfg.Host != "127.0.0.1" {
		t.Errorf("expected host '127.0.0.1' from env, got %q", cfg.Host)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log_level 'debug' (normalized) from env, got %q", cfg.LogLevel)
	}
}
