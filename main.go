package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"localregistry/internal/api"
	"localregistry/internal/tui"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	var (
		registryURL string
		username    string
		password    string
		insecure    bool
		showVersion bool
	)

	flag.StringVar(&registryURL, "url", envOrDefault("REGISTRY_URL", "http://localhost:5000"), "Registry URL")
	flag.StringVar(&username, "user", os.Getenv("REGISTRY_USER"), "Registry username for basic auth")
	flag.StringVar(&password, "pass", os.Getenv("REGISTRY_PASS"), "Registry password for basic auth")
	flag.BoolVar(&insecure, "insecure", false, "Skip TLS certificate verification")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.Parse()

	if showVersion {
		fmt.Printf("localregistry %s (%s)\n", version, commit)
		return
	}

	cfg := api.ClientConfig{
		Address:  registryURL,
		Username: username,
		Password: password,
		Insecure: insecure,
	}

	client, err := api.NewClient(cfg)
	if err != nil {
		log.Fatalf("failed to create registry client: %v", err)
	}

	if err := tui.Run(client); err != nil {
		log.Fatalf("tui error: %v", err)
	}
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
