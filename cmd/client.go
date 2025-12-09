package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"localregistry/internal/api"
)

var (
	registryHost string
	registryPort int
	username     string
	password     string
	insecure     bool
	timeout      time.Duration
)

func newClient() (*api.Client, error) {
	address := fmt.Sprintf("%s:%d", registryHost, registryPort)
	cfg := api.ClientConfig{
		Address:  address,
		Username: username,
		Password: password,
		Insecure: insecure,
		Timeout:  timeout,
	}

	slog.Debug("creating api client", "address", address, "insecure", insecure, "timeout", timeout)

	client, err := api.NewClient(cfg)
	if err != nil {
		slog.Error("failed to create api client", "error", err)
		return nil, err
	}

	slog.Debug("api client created successfully")
	return client, nil
}

func newContext() context.Context {
	return context.Background()
}

func exitOnError(err error) {
	if err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}
