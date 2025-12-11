// Package main is the entry point for the local-registry CLI.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"local-registry/cmd"
	"local-registry/config"

	"github.com/natefinch/lumberjack"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Ensure log directory exists
	if err := os.MkdirAll(cfg.LogDir, 0750); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log directory %s: %v\n", cfg.LogDir, err)
		os.Exit(1)
	}

	// Configure rotating log writer
	rotator := &lumberjack.Logger{
		Filename:   filepath.Join(cfg.LogDir, "local-registry.log"),
		MaxSize:    10,   // megabytes
		MaxBackups: 5,    // number of rotated files to keep
		MaxAge:     28,   // days
		Compress:   true, // gzip rotated files
	}

	logger := slog.New(slog.NewJSONHandler(rotator, &slog.HandlerOptions{
		AddSource: true,
		Level:     cfg.SlogLevel(),
	}))
	slog.SetDefault(logger)

	slog.Debug("configuration loaded", "log_level", cfg.LogLevel)

	cmd.Execute(cfg)
}
