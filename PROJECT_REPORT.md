# Local Registry - Project Report

## Overview

Welcome to **localregistry**, a sophisticated Go-based command-line tool designed to simplify your interactions with Docker registries during development. Think of it as your friendly neighborhood registry manager—it helps you run a local Docker registry container and provides both a powerful CLI and an interactive Terminal User Interface (TUI) to manage container images effortlessly.

If you've ever found yourself wrestling with Docker registries in local Kubernetes clusters like k3d or kind, this tool is your new best friend. It handles all the heavy lifting: starting registry containers, mirroring images from Docker Hub or other registries, managing tags, and even running garbage collection to keep your disk space in check.

---

## What Does It Do?

At its core, **localregistry** does three main things:

### 1. Registry Server Management
It manages a local Docker registry container (the official `registry:3` image from Docker Hub) right on your machine. You can:
- **Start** a registry container with sensible defaults
- **Stop** and **restart** the container
- **Check status** and get detailed information about the running container
- **Run garbage collection** to reclaim disk space from deleted images

### 2. Image Operations
It provides a complete toolkit for working with container images:
- **Add/Mirror images** from any source (Docker Hub, GHCR, your local Docker daemon) into your local registry using Skopeo under the hood
- **Inspect images** to see detailed metadata (layers, architecture, tags, digests)
- **Copy images** between registries
- **List repositories** and **tags** in your registry
- **Delete images, tags, or entire repositories** with optional automatic garbage collection

### 3. Interactive Terminal UI
For those who prefer a visual, interactive experience, it offers a full-featured TUI built with Bubble Tea that lets you:
- Browse repositories and tags visually
- View image details including layers and sizes
- Add new images through an interactive workflow
- Delete tags or repositories with confirmation prompts
- View container logs in real-time
- Manage the registry server (start/stop/restart) without leaving the interface
- Run garbage collection with visual feedback

---

## How Does It Work?

Let's peek under the hood to understand the architecture and how everything fits together.

### Architecture Overview

The project follows a clean, modular architecture with clear separation of concerns:

```
localregistry/
├── main.go                    # Entry point: config loading, logging setup
├── cmd/                       # CLI commands (Cobra)
│   ├── root.go               # Root command, config binding
│   ├── catalog.go            # List repositories
│   ├── tags.go               # List tags for a repo
│   ├── server.go             # Server management (start/stop/restart/gc/info)
│   ├── add.go                # Add/mirror images, inspect, copy
│   ├── delete.go             # Delete repos/images/tags
│   ├── tui.go                # Launch TUI
│   ├── manifest.go           # Manifest operations
│   ├── ping.go               # Registry connectivity check
│   └── client.go             # Shared client initialization
├── config/                    # Configuration management (Viper)
│   ├── config.go             # Config loading, validation
│   └── default_config.yaml   # Embedded defaults
├── internal/
│   ├── api/                  # Docker Registry HTTP API V2 client
│   │   ├── client.go         # HTTP client for registry API
│   │   ├── manifest.go       # Manifest operations
│   │   └── errors.go         # API error handling
│   ├── registry/             # Docker and Skopeo integrations
│   │   ├── registry.go       # Registry container lifecycle
│   │   ├── docker.go         # Docker SDK wrapper
│   │   └── skopeo.go         # Skopeo wrapper for image operations
│   └── tui/                  # Terminal User Interface
│       ├── tui.go            # Bubble Tea TUI implementation
│       └── styles.go         # Lipgloss styling
```

### Core Components

#### 1. Configuration System (config/)
The configuration system uses **Viper** and follows a hierarchical precedence model:

**Precedence order (highest to lowest):**
1. Command-line flags (e.g., `--host localhost --port 5000`)
2. Environment variables with `LR_` prefix (e.g., `LR_REGISTRY_HOST=localhost`)
3. User config file at `~/.config/localregistry/config.yaml`
4. Embedded defaults from `default_config.yaml`

**Default configuration:**
```yaml
registry:
  host: localhost
  port: 5000
  datadir: "$XDG_DATA_HOME/local-registry"  # Defaults to ~/.local/share/local-registry
  username: ""
  password: ""
  insecure: false
  timeout: 30s
log_level: info
```

The config is loaded once at startup in `main.go:14` and passed to all commands, ensuring consistent settings throughout the application.

#### 2. CLI Commands (cmd/)
Built with **spf13/cobra**, each command is self-contained:

- **`root.go`**: Defines the root command, persistent flags (host, port, credentials, timeout), and applies configuration in `PersistentPreRunE`
- **`server.go`**: Manages the local registry container lifecycle using the Docker SDK
- **`catalog.go`** and **`tags.go`**: Simple wrappers around the Registry API client
- **`add.go`**: The most sophisticated command—uses Skopeo to copy images with automatic source detection (local Docker daemon vs. remote registry)
- **`delete.go`**: Multi-level deletion (tag, image, repository) with optional garbage collection
- **`tui.go`**: Launches the interactive interface with full features

#### 3. Docker Registry API Client (internal/api/)
A lightweight HTTP client implementing the [Docker Registry HTTP API V2](https://docs.docker.com/registry/spec/api/):

**Key methods in `client.go`:**
- `Ping()`: Verifies registry connectivity and API version
- `ListAllRepositories()`: Paginated catalog retrieval
- `ListAllTags()`: Paginated tag listing for a repository
- `GetManifest()` / `DeleteManifestByTag()`: Manifest operations
- `GetBlob()` / `DeleteBlob()`: Blob layer operations

The client handles:
- Automatic HTTP/HTTPS schema selection based on the `insecure` flag
- Basic authentication
- Pagination for large datasets
- Proper error handling with registry-specific error responses

**Example flow for listing repositories:**
```
User runs: localregistry catalog
    ↓
cmd/catalog.go creates API client
    ↓
client.ListAllRepositories() makes GET /v2/_catalog
    ↓
Handles pagination if n > 100
    ↓
Returns []string of repository names
    ↓
Printed to stdout
```

#### 4. Registry Container Management (internal/registry/)
This package provides two critical integrations:

**Docker SDK Integration (`docker.go` + `registry.go`):**
Wraps the official `github.com/docker/docker` SDK to manage the registry container:

- **Container lifecycle**: Start/Stop/Restart operations
- **Image management**: Pulls `registry:3` image if not present
- **Container inspection**: Status, detailed info, port bindings, mounts
- **Exec operations**: Run commands inside the container for garbage collection
- **Repository deletion**: Direct filesystem operations via `docker exec`

**Start flow (`registry.go:100`):**
```go
1. Check if container already exists (by name)
2. If exists and running: return success
3. If exists but stopped: start it
4. If doesn't exist: pull image → create container → start
5. Configure port binding (5000:5000), restart policy, etc.
```

**Skopeo Integration (`skopeo.go`):**
Wraps the `skopeo` CLI tool for advanced image operations:

- **Copy**: Mirror images between registries, Docker daemon, OCI layouts
- **Inspect**: Retrieve detailed image metadata without pulling
- **Multi-architecture support**: Copy all platform images with `--all`
- **Flexible authentication**: Separate source/destination credentials
- **TLS control**: Per-operation TLS verification

**Why Skopeo?** Unlike Docker's `docker pull` + `docker tag` + `docker push`, Skopeo can copy images directly between registries without storing them locally, and it preserves all manifests in multi-arch images.

#### 5. Terminal User Interface (internal/tui/)
Built with **Bubble Tea** (Elm-inspired TUI framework) and **Lipgloss** (styling):

The TUI implements a state machine with multiple views:
- **Main menu**: Repository browser with server status footer
- **Tag list**: Shows all tags for a selected repository
- **Image details**: Displays layers, digests, sizes
- **Add image**: Interactive form for image source and target
- **Logs viewer**: Live container log stream
- **Confirmation dialogs**: For destructive operations

**State management:**
The TUI maintains state for the current view, selected items, API client, Docker client, and Skopeo instance. User interactions (key presses) trigger state transitions and API calls.

---

## How to Use It

### Installation

**Prerequisites:**
- Go 1.25.5 or later
- Docker running on your machine
- Skopeo installed (for image mirroring operations)

**Build from source:**
```bash
git clone <repository-url>
cd localregistry
make build    # Builds to ./bin/localregistry
# or
go build -o localregistry
```

### Quick Start

**1. Start the local registry:**
```bash
localregistry server start
```
This starts a registry container at `localhost:5000` with default settings.

**2. Add an image from Docker Hub:**
```bash
localregistry add alpine:latest
```
This mirrors Alpine Linux from Docker Hub to your local registry.

**3. Verify it's there:**
```bash
localregistry catalog
# Output: alpine

localregistry tags alpine
# Output: latest
```

**4. Try the interactive TUI:**
```bash
localregistry tui
```
Navigate with arrow keys, press Enter to select, 'q' to quit.

### Configuration

**Create a config file (optional):**
```bash
mkdir -p ~/.config/localregistry
cat > ~/.config/localregistry/config.yaml <<EOF
registry:
  host: localhost
  port: 5000
  insecure: true
  timeout: 60s
log_level: debug
EOF
```

**Or use environment variables:**
```bash
export LR_REGISTRY_HOST=localhost
export LR_REGISTRY_PORT=5000
export LR_LOG_LEVEL=debug
localregistry catalog
```

**Or use CLI flags:**
```bash
localregistry --host localhost --port 5000 catalog
```

### Common Use Cases

#### Use Case 1: Develop with k3d
```bash
# Start registry
localregistry server start

# Add images you need
localregistry add nginx:alpine
localregistry add postgres:15

# Create k3d cluster with registry
k3d cluster create mycluster --registry-use localhost:5000

# Deploy using local images
kubectl create deployment nginx --image=localhost:5000/nginx:alpine
```

#### Use Case 2: Mirror Multi-Architecture Images
```bash
# Copy all architectures (amd64, arm64, etc.)
localregistry add --all docker.io/library/golang:1.21

# Inspect to verify
localregistry inspect golang:1.21
```

#### Use Case 3: Clean Up Old Images
```bash
# Delete specific tags
localregistry delete tag myapp v1.0.0

# Delete entire repository
localregistry delete repo old-project

# Run garbage collection to reclaim space
localregistry server gc --delete-untagged
```

#### Use Case 4: Copy Between Registries
```bash
# Copy from local registry to GHCR
localregistry copy \
  docker://localhost:5000/myapp:v1.0 \
  docker://ghcr.io/myorg/myapp:v1.0 \
  --dest-creds username:token
```

### CLI Command Reference

**Server Management:**
```bash
localregistry server start              # Start registry container
localregistry server stop               # Stop registry
localregistry server restart            # Restart registry
localregistry server status             # Show status (running/stopped)
localregistry server info               # Detailed container info
localregistry server gc                 # Run garbage collection
```

**Image Operations:**
```bash
localregistry add <source> [target]     # Add/mirror an image
localregistry add alpine                # From Docker Hub (docker.io/library/alpine)
localregistry add myimage:latest        # From local Docker daemon (auto-detected)
localregistry add ghcr.io/user/app      # From GHCR
localregistry add --all nginx:latest    # Copy all architectures

localregistry inspect <image>           # Show image details
localregistry copy <src> <dest>         # Copy between registries
```

**Registry Queries:**
```bash
localregistry catalog                   # List all repositories
localregistry tags <repo>               # List tags for a repository
localregistry ping                      # Check registry connectivity
```

**Deletion:**
```bash
localregistry delete tag <repo> <tag>   # Delete specific tag
localregistry delete image <repo> [tags...] # Delete tags (all if none specified)
localregistry delete repo <repo>        # Delete entire repository
# Add --gc flag to run garbage collection after deletion
```

**Interactive TUI:**
```bash
localregistry tui                       # Launch interactive interface
```

**Configuration:**
```bash
localregistry config                    # Show current configuration
localregistry version                   # Show version info
```

### Advanced Configuration

**Custom registry port:**
```bash
localregistry server start --port 5001
localregistry --port 5001 catalog
```

**With authentication:**
```bash
# Start with credentials
localregistry server start --user admin --pass secret

# Use credentials in commands
localregistry --user admin --pass secret catalog
```

**Skip TLS verification (for self-signed certs):**
```bash
localregistry --insecure add myregistry.local/image:tag
```

**Increase timeout for slow networks:**
```bash
localregistry --timeout 120s add large-image:tag
```

---

## Architecture Deep Dive

### Data Flow: Adding an Image

Let's trace what happens when you run `localregistry add alpine:latest`:

1. **Command Parsing** (`cmd/add.go:46`)
   - Cobra parses arguments: source = "alpine:latest"
   - No target specified, so target is extracted: "alpine:latest"
   - Destination constructed: `localhost:5000/alpine:latest`

2. **Source Detection** (`cmd/add.go:145`)
   - `determineSourceRef()` checks if "alpine" exists in local Docker daemon
   - If not found locally, prepends `docker.io/library/` → `docker://docker.io/library/alpine:latest`

3. **Skopeo Copy** (`internal/registry/skopeo.go`)
   - Builds Skopeo command: `skopeo copy docker://docker.io/library/alpine:latest docker://localhost:5000/alpine:latest`
   - Adds flags: `--dest-tls-verify=false` if insecure
   - Executes command, captures output
   - Returns error if exit code != 0

4. **Result**
   - Image layers streamed from Docker Hub to local registry
   - No local disk usage (Skopeo works registry-to-registry)
   - Output: "Successfully added localhost:5000/alpine:latest"

### Data Flow: Deleting a Repository

When you run `localregistry delete repo myapp --gc`:

1. **List Tags** (`cmd/delete.go:47`)
   - API client calls `GET /v2/myapp/tags/list`
   - Returns all tags: `["v1.0", "v1.1", "latest"]`

2. **Delete Manifests** (`cmd/delete.go:56`)
   - For each tag, get manifest digest via `GET /v2/myapp/manifests/<tag>` with `Accept: application/vnd.docker.distribution.manifest.v2+json`
   - Delete via `DELETE /v2/myapp/manifests/<digest>`
   - Marks manifest for deletion (doesn't free space yet)

3. **Remove Directory** (`cmd/delete.go:78`)
   - Uses Docker SDK to exec into registry container
   - Runs `rm -rf /var/lib/registry/docker/registry/v2/repositories/myapp`
   - Removes repository metadata from filesystem

4. **Garbage Collection** (`cmd/delete.go:84`)
   - Stops registry container temporarily
   - Execs `registry garbage-collect /etc/docker/registry/config.yml --delete-untagged`
   - Deletes unreferenced blobs and layers
   - Restarts registry
   - Frees actual disk space

### Docker SDK Integration Details

The `internal/registry/registry.go` uses the Docker SDK to manage containers:

**Container Creation** (`registry.go:120`):
```go
// Simplified pseudo-code
containerConfig := &container.Config{
    Image: "registry:3",
    ExposedPorts: nat.PortSet{"5000/tcp": struct{}{}},
}

hostConfig := &container.HostConfig{
    PortBindings: nat.PortMap{
        "5000/tcp": []nat.PortBinding{{HostPort: "5000"}},
    },
    RestartPolicy: container.RestartPolicy{Name: "always"},
}

resp, err := docker.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "registry")
docker.ContainerStart(ctx, resp.ID, container.StartOptions{})
```

This creates a container with:
- Automatic restart on Docker daemon restart
- Port 5000 exposed to host
- Default storage in container's filesystem (or custom volume if configured)

---

## Technology Stack

**Languages & Frameworks:**
- **Go 1.25.5**: Main language
- **Cobra**: CLI framework with subcommands, flags, help text
- **Viper**: Configuration management with multiple sources
- **Bubble Tea**: Elm-inspired TUI framework for the interactive mode
- **Lipgloss**: Terminal styling and layout
- **Bubbles**: Reusable TUI components (lists, viewports, spinners)

**External Dependencies:**
- **Docker SDK** (`github.com/docker/docker`): Container lifecycle management
- **Docker Go Connections** (`github.com/docker/go-connections`): Port binding utilities
- **Skopeo** (external CLI): Image copy/inspect operations
- **Standard Registry** (`registry:3` Docker image): The actual registry server

**Development Tools:**
- **slog**: Structured logging with JSON output
- **Context**: Cancellation and timeout propagation
- **Standard library**: Extensive use of `net/http`, `encoding/json`, `os/exec`

---

## Design Decisions

### Why Skopeo Instead of Docker SDK for Image Operations?

**Skopeo advantages:**
1. **Registry-to-registry copying**: No need to pull images locally, saving disk space
2. **Multi-arch preservation**: Copies all manifests in a manifest list (amd64, arm64, etc.)
3. **Format flexibility**: Supports OCI, Docker v2s1, v2s2 formats
4. **Lightweight**: Just needs the binary, no daemon required

**Trade-off:** Requires Skopeo to be installed separately. This is documented and Skopeo is widely available in package managers.

### Why Embedded Default Config?

Using `//go:embed` to bundle `default_config.yaml` ensures:
- **Zero-config startup**: Works immediately after building
- **Consistent defaults**: No environment-specific configuration drift
- **Single binary**: No need to ship separate config files

### Why Both CLI and TUI?

**CLI for automation:**
- Scriptable (CI/CD pipelines, automation scripts)
- Composable with Unix tools (`localregistry catalog | grep myapp`)
- Fast for single operations

**TUI for exploration:**
- Visual discovery of repositories and tags
- Real-time feedback for long operations
- Safer deletion (confirmation prompts)
- Better for learning the registry contents

---

## Common Workflows

### Development Workflow
```bash
# Morning: Start registry
localregistry server start

# Add base images
localregistry add golang:1.21-alpine
localregistry add postgres:15-alpine

# Work on your app, build images
docker build -t localhost:5000/myapp:dev .
docker push localhost:5000/myapp:dev

# Test in k3d
k3d cluster create dev --registry-use localhost:5000
kubectl create deployment myapp --image=localhost:5000/myapp:dev

# Evening: Check what's stored
localregistry tui  # Browse visually

# Cleanup old tags
localregistry delete tag myapp old-dev-tag
localregistry server gc
```

### CI/CD Integration
```bash
#!/bin/bash
# ci-pipeline.sh

# Ensure registry is running
localregistry server status || localregistry server start

# Mirror dependencies to local registry (faster pulls in CI)
localregistry add --quiet alpine:latest
localregistry add --quiet node:18-alpine

# Run tests using local images
docker run --rm localhost:5000/node:18-alpine npm test

# Cleanup
localregistry server gc --delete-untagged
```

---

## Troubleshooting

### Registry Won't Start
```bash
# Check Docker is running
docker ps

# Check port availability
lsof -i :5000

# View registry logs
localregistry tui  # Navigate to logs view
# Or with Docker:
docker logs registry
```

### Skopeo Not Found
```bash
# Install Skopeo
# macOS:
brew install skopeo

# Ubuntu/Debian:
sudo apt install skopeo

# Fedora:
sudo dnf install skopeo
```

### TLS Verification Errors
If you see certificate errors:
```bash
# Use insecure flag
localregistry --insecure add myimage:tag

# Or set in config
echo "registry:
  insecure: true" > ~/.config/localregistry/config.yaml
```

### Out of Disk Space
```bash
# Run garbage collection
localregistry server gc --delete-untagged

# Check registry size
docker exec registry du -sh /var/lib/registry

# Delete unused repositories
localregistry delete repo old-project-1
localregistry delete repo old-project-2
localregistry server gc
```

---

## Performance Characteristics

**Image mirroring speed:**
- Limited by network bandwidth to source registry
- No local storage overhead (direct registry-to-registry)
- Multi-arch images take longer (multiple manifests + layers)

**API operations:**
- Catalog/tags listing: O(n) with pagination, typically <100ms for small registries
- Manifest operations: Single HTTP request, <50ms local
- Deletion: Fast API operation, but disk space not freed until GC

**Garbage collection:**
- Requires registry stop (brief downtime)
- Duration scales with registry size
- Recommendation: Run during low-traffic periods or maintenance windows

---

## Future Enhancements

Based on the codebase structure, potential additions could include:

1. **Registry replication**: Sync between multiple registries
2. **Image signing**: Integration with Cosign/Notary for image signing
3. **Storage backend configuration**: S3, Azure Blob, GCS support
4. **Web UI**: Complement the TUI with a browser-based interface
5. **Metrics/monitoring**: Prometheus exporter for registry metrics
6. **Image vulnerability scanning**: Integration with Trivy or Grype
7. **Webhook support**: Trigger actions on image push/delete
8. **LDAP/OAuth authentication**: Enterprise auth integration

---

## Conclusion

**localregistry** is a thoughtfully designed tool that bridges the gap between Docker's complexity and developer productivity. Whether you're running local Kubernetes clusters, testing container images, or managing a development registry, it provides the right tool for the job—from scriptable CLI commands to an intuitive TUI.

The modular architecture makes it maintainable and extensible, while the use of industry-standard tools (Docker SDK, Skopeo) ensures reliability. The configuration system is flexible enough for both casual users (zero-config) and power users (fine-grained control).

If you're developing containerized applications, especially with local Kubernetes distributions, this tool will save you time and frustration. Give it a try, and enjoy having a local registry that just works.

---

**Project maintained by:** [Your name/org]  
**License:** [License type - check LICENSE file]  
**Go version:** 1.25.5  
**Registry version:** Docker Registry 3.x
