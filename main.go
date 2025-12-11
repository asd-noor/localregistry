// Package main is the entry point for the local-registry CLI.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"local-registry/cmd"
	"local-registry/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		Level:     cfg.SlogLevel(),
	}))
	slog.SetDefault(logger)

	slog.Debug("configuration loaded", "log_level", cfg.LogLevel)

	cmd.Execute(cfg)
}
