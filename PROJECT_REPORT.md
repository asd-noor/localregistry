# Project Report: localregistry

A command-line tool for interacting with Docker Registry HTTP API V2, featuring both CLI and TUI interfaces.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Technology Stack](#technology-stack)
- [Project Structure](#project-structure)
- [Configuration](#configuration)
- [CLI Commands](#cli-commands)
- [Usage Examples](#usage-examples)
- [Internal Packages](#internal-packages)
- [Development](#development)

## Overview

`localregistry` is a Go-based CLI application designed to interact with Docker Registry HTTP API V2. It provides comprehensive functionality for:

- Listing repositories and tags in a registry
- Retrieving and deleting manifests
- Managing blobs (image layers)
- Interactive terminal UI for registry browsing

The tool supports authentication, TLS configuration, and flexible configuration via files, environment variables, or command-line flags.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                              main.go                                │
│                         (Entry Point)                               │
└─────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                             cmd/                                    │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐            │
│  │ root.go  │  │catalog.go│  │ tags.go  │  │manifest. │            │
│  │ (Cobra)  │  │          │  │          │  │   go     │            │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                          │
│  │ blob.go  │  │ ping.go  │  │  tui.go  │                          │
│  └──────────┘  └──────────┘  └──────────┘                          │
│  ┌──────────────────────────────────────┐                          │
│  │           client.go                  │                          │
│  │  (Shared client factory & helpers)   │                          │
│  └──────────────────────────────────────┘                          │
└─────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                          config/                                    │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  config.go - Viper-based configuration management            │  │
│  │  - Embedded defaults (default_config.yaml)                   │  │
│  │  - User config file (~/.config/localregistry/config.yaml)    │  │
│  │  - Environment variables (LOCALREGISTRY_*)                   │  │
│  └──────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                          internal/                                  │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │                        api/                                 │    │
│  │  client.go   - HTTP client for Docker Registry API V2       │    │
│  │  manifest.go - Manifest operations (get/head/delete)        │    │
│  │  blob.go     - Blob operations (get/head/delete)            │    │
│  │  errors.go   - API error types and parsing                  │    │
│  └────────────────────────────────────────────────────────────┘    │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │                      registry/                              │    │
│  │  registry.go - Local registry container management          │    │
│  │  docker.go   - Docker SDK client abstraction                │    │
│  │  skopeo.go   - (Placeholder for skopeo integration)         │    │
│  └────────────────────────────────────────────────────────────┘    │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │                        tui/                                 │    │
│  │  tui.go    - Bubbletea-based terminal UI                    │    │
│  │  styles.go - Lipgloss styling definitions                   │    │
│  └────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
```

### Data Flow

1. **CLI Entry**: `main.go` invokes `cmd.Execute()` which initializes Cobra
2. **Configuration Loading**: `PersistentPreRunE` in `root.go` loads config via Viper
3. **Client Creation**: Commands use `newClient()` from `client.go` to create an API client
4. **API Calls**: The `api.Client` makes HTTP requests to the Docker Registry V2 endpoints
5. **Response Handling**: Results are printed to stdout or displayed in TUI

## Technology Stack

| Component | Technology | Version |
|-----------|------------|---------|
| Language | Go | 1.25.5 |
| CLI Framework | [spf13/cobra](https://github.com/spf13/cobra) | v1.10.2 |
| Configuration | [spf13/viper](https://github.com/spf13/viper) | v1.21.0 |
| TUI Framework | [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) | v1.3.10 |
| TUI Components | [charmbracelet/bubbles](https://github.com/charmbracelet/bubbles) | v0.21.0 |
| TUI Styling | [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) | v1.1.0 |
| Docker SDK | [docker/docker](https://github.com/docker/docker) | v28.5.2 |

## Project Structure

```
localregistry/
├── main.go                      # Application entry point
├── go.mod                       # Go module definition
├── go.sum                       # Dependency checksums
├── Makefile                     # Build automation (empty)
├── LICENSE                      # License file
├── README.md                    # User documentation (empty)
├── cmd/                         # CLI commands
│   ├── root.go                  # Root command, config loading, global flags
│   ├── client.go                # Shared client factory and helpers
│   ├── catalog.go               # List repositories command
│   ├── tags.go                  # List tags command
│   ├── manifest.go              # Manifest subcommands (get/head/delete)
│   ├── blob.go                  # Blob subcommands (get/head/delete)
│   ├── ping.go                  # Registry connectivity check
│   └── tui.go                   # Launch interactive TUI
├── config/                      # Configuration management
│   ├── config.go                # Viper-based config loading
│   ├── config_test.go           # Config tests
│   └── default_config.yaml      # Embedded default configuration
└── internal/                    # Internal packages
    ├── api/                     # Docker Registry HTTP API V2 client
    │   ├── client.go            # Client struct and catalog/tags operations
    │   ├── manifest.go          # Manifest operations
    │   ├── blob.go              # Blob operations
    │   └── errors.go            # API error types
    ├── registry/                # Docker container management
    │   ├── registry.go          # Registry container lifecycle
    │   ├── docker.go            # Docker SDK wrapper
    │   └── skopeo.go            # Skopeo integration (placeholder)
    └── tui/                     # Terminal UI
        ├── tui.go               # Bubbletea model and views
        └── styles.go            # Lipgloss style definitions
```

## Configuration

Configuration is loaded with the following precedence (highest to lowest):

1. **Command-line flags** (e.g., `--url`, `--user`, `--pass`)
2. **Environment variables** (prefix: `LOCALREGISTRY_`)
3. **User config file** (`~/.config/localregistry/config.yaml`)
4. **Embedded defaults** (`config/default_config.yaml`)

### Configuration Options

| Option | Flag | Environment Variable | Default |
|--------|------|---------------------|---------|
| Registry URL | `--url`, `-u` | `LOCALREGISTRY_REGISTRY_URL` | `http://localhost:5000` |
| Username | `--user` | `LOCALREGISTRY_REGISTRY_USERNAME` | (empty) |
| Password | `--pass` | `LOCALREGISTRY_REGISTRY_PASSWORD` | (empty) |
| Skip TLS Verify | `--insecure`, `-k` | `LOCALREGISTRY_REGISTRY_INSECURE` | `false` |
| Timeout | `--timeout` | `LOCALREGISTRY_REGISTRY_TIMEOUT` | `30s` |
| Log Level | - | `LOCALREGISTRY_LOG_LEVEL` | `info` |

### Example Config File

```yaml
# ~/.config/localregistry/config.yaml
registry:
  url: https://registry.example.com
  username: admin
  password: secret
  insecure: false
  timeout: 60s

log_level: debug
```

## CLI Commands

### Global Flags

```
--url, -u      Registry URL (default: http://localhost:5000)
--user         Username for basic auth
--pass         Password for basic auth
--insecure, -k Skip TLS certificate verification
--timeout      Request timeout (default: 30s)
```

### Commands

| Command | Description |
|---------|-------------|
| `localregistry ping` | Verify registry connectivity and V2 API support |
| `localregistry catalog` | List all repositories (aliases: `repos`, `repositories`) |
| `localregistry tags <repo>` | List all tags for a repository |
| `localregistry manifest get <repo> <ref>` | Retrieve a manifest by tag or digest |
| `localregistry manifest head <repo> <ref>` | Check manifest existence and metadata |
| `localregistry manifest delete <repo> <ref>` | Delete a manifest by tag or digest |
| `localregistry blob get <repo> <digest>` | Download a blob to stdout or file (`-o`) |
| `localregistry blob head <repo> <digest>` | Check blob existence and metadata |
| `localregistry blob delete <repo> <digest>` | Delete a blob |
| `localregistry tui` | Launch interactive terminal UI |
| `localregistry config` | Show configuration information |
| `localregistry version` | Print version information |

## Usage Examples

### Check Registry Connectivity

```bash
localregistry ping --url https://registry.example.com
```

### List All Repositories

```bash
localregistry catalog
# or
localregistry repos
```

### List Tags for a Repository

```bash
localregistry tags myapp
```

### Get Manifest Details

```bash
# By tag
localregistry manifest get myapp latest

# By digest
localregistry manifest get myapp sha256:abc123...
```

### Delete an Image Tag

```bash
localregistry manifest delete myapp v1.0.0
```

### Download a Blob Layer

```bash
# To stdout
localregistry blob get myapp sha256:abc123... > layer.tar.gz

# To file
localregistry blob get myapp sha256:abc123... -o layer.tar.gz
```

### Launch Interactive TUI

```bash
localregistry tui
```

**TUI Keybindings:**
- `↑/↓` or `k/j`: Navigate
- `Enter`: Select/drill down
- `d`: Delete selected item
- `r`: Refresh current view
- `Esc`: Go back
- `q`: Quit

### Using Environment Variables

```bash
export LOCALREGISTRY_REGISTRY_URL=https://registry.example.com
export LOCALREGISTRY_REGISTRY_USERNAME=admin
export LOCALREGISTRY_REGISTRY_PASSWORD=secret

localregistry catalog
```

## Internal Packages

### `internal/api`

HTTP client for Docker Registry HTTP API V2. Key types:

- **`Client`**: Main client struct with methods for all registry operations
- **`ClientConfig`**: Configuration for creating a client
- **`CatalogResponse`**, **`TagsResponse`**: API response types
- **`Manifest`**, **`BlobInfo`**: Data types for manifests and blobs
- **`APIError`**, **`APIErrors`**: Error types with helper functions (`IsNotFound`, `IsUnauthorized`, `IsDenied`)

Supported media types:
- `application/vnd.docker.distribution.manifest.v2+json`
- `application/vnd.docker.distribution.manifest.list.v2+json`
- `application/vnd.oci.image.manifest.v1+json`
- `application/vnd.oci.image.index.v1+json`

### `internal/registry`

Docker SDK wrapper for managing local registry containers:

- **`Registry`**: Manages registry container lifecycle (start, stop, remove, status)
- **`RegistryConfig`**: Configuration including TLS, auth, volumes
- **`DockerClient`**: Interface abstracting Docker operations for testability
- **`NewDockerClient()`**: Creates a client using environment configuration

### `internal/tui`

Bubbletea-based terminal user interface:

- **Views**: Repositories → Tags → Manifest (with delete confirmation dialog)
- **Features**: Fuzzy filtering, async loading with spinners, keyboard navigation
- **Styling**: Consistent color palette via Lipgloss

## Development

### Building

```bash
go build -o localregistry .
```

### Build with Version Information

```bash
go build -ldflags "-X localregistry/cmd.Version=v1.0.0 -X localregistry/cmd.Commit=$(git rev-parse --short HEAD)" -o localregistry .
```

### Running Tests

```bash
go test ./...
```

### Code Quality Notes

The codebase follows Go best practices:
- Clear package boundaries with `internal/` for private packages
- Interface-based design for testability (`DockerClient` interface)
- Comprehensive error handling with typed errors
- Viper for flexible configuration management
- Cobra for structured CLI with help generation

**Linting Issue**: The `main` package is missing a package comment. Add a comment above `package main` in `main.go`:

```go
// Package main is the entry point for the localregistry CLI application.
package main
```

---

*Generated by project analysis on 2025-12-09*
