package main

import (
	"fmt"
	"log/slog"
	"os"

	"localregistry/cmd"
	"localregistry/config"
)

func main() {
	// Load configuration first to get log level
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger with configured level
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		Level:     cfg.SlogLevel(),
	}))
	slog.SetDefault(logger)

	slog.Debug("configuration loaded", "log_level", cfg.LogLevel)

	cmd.Execute()
}
