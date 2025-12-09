package cmd

import (
	"context"
	"fmt"
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
	return api.NewClient(cfg)
}

func newContext() context.Context {
	return context.Background()
}

func exitOnError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
