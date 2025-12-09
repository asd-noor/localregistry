package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"localregistry/internal/api"
)

var (
	registryURL string
	username    string
	password    string
	insecure    bool
	timeout     time.Duration
)

func newClient() (*api.Client, error) {
	cfg := api.ClientConfig{
		Address:  registryURL,
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
