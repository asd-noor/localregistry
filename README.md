# local-registry

A CLI tool for managing a local Docker registry for development with k3d, kind, and similar tools.

## Features

- **Interactive TUI** for browsing repositories, tags, and managing images
- **CLI commands** for scripting and automation
- Add images from Docker Hub, other registries, or local Docker daemon
- View and delete tags, manifests, and repositories
- Manage the registry container (start/stop/restart)
- Stream container logs
- Run garbage collection
- XDG-compliant configuration and logging

## Requirements

- Go 1.21+
- Docker
- [skopeo](https://github.com/containers/skopeo) (for image operations)

## Installation

```bash
# Clone and build
git clone https://github.com/yourusername/local-registry.git
cd local-registry
make build

# Or install directly to $GOPATH/bin
make install
```

## Quick Start

```bash
# Start the registry container
local-registry server start

# Launch the TUI (default when no command given)
local-registry

# Add an image from Docker Hub
local-registry add alpine:latest

# Add a local Docker image
local-registry add myapp:dev

# List repositories
local-registry catalog
```

## Usage

### TUI (Interactive Mode)

Run `local-registry` without arguments to launch the TUI:

```bash
local-registry
```

#### Keybindings

| Key | Action |
|-----|--------|
| `j/k` or `arrows` | Navigate |
| `Enter` | Select |
| `Esc` | Go back |
| `a` | Add image |
| `d` | Delete tag |
| `i` | Image details |
| `l` | View logs |
| `s` | Server actions |
| `g` | Garbage collection |
| `r` | Refresh |
| `?` | Help |
| `q` | Quit |

### CLI Commands

#### Registry Operations

```bash
# Check connectivity
local-registry ping

# List all repositories
local-registry catalog

# List tags for a repository
local-registry tags alpine

# View manifest
local-registry manifest get alpine:latest

# Delete a tag
local-registry delete alpine:latest
```

#### Image Operations

```bash
# Add image from Docker Hub
local-registry add nginx:alpine

# Add from another registry
local-registry add ghcr.io/org/app:v1.0

# Add with custom target name
local-registry add alpine:latest myalpine:latest

# Add multi-arch image
local-registry add --all golang:1.21

# Inspect an image
local-registry inspect myapp:latest
```

#### Server Management

```bash
# Start the registry container
local-registry server start

# Stop the registry
local-registry server stop

# Restart the registry
local-registry server restart

# Check status
local-registry server status

# View detailed info
local-registry server info

# View logs
local-registry server logs
local-registry server logs -f  # follow
```

### Global Flags

```
-H, --host string        Registry host (default "localhost")
-p, --port int           Registry port (default 5000)
-k, --insecure           Skip TLS verification
    --user string        Username for basic auth
    --pass string        Password for basic auth
    --timeout duration   Request timeout (default 30s)
```

## Configuration

Configuration is loaded from (in order of precedence):
1. Command-line flags
2. Environment variables (`LR_REGISTRY_HOST`, `LR_REGISTRY_PORT`, etc.)
3. Config file
4. Embedded defaults

### Config File Location

```
~/.config/local-registry/config.yaml
```

### Example Configuration

```yaml
registry:
  host: localhost
  port: 5000
  container_name: local-registry
  data_dir: "$XDG_DATA_HOME/local-registry/data"
  insecure: false
  timeout: 30s

log_level: info
log_dir: "$XDG_DATA_HOME/local-registry/logs"
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `LR_REGISTRY_HOST` | Registry hostname |
| `LR_REGISTRY_PORT` | Registry port |
| `LR_REGISTRY_USERNAME` | Basic auth username |
| `LR_REGISTRY_PASSWORD` | Basic auth password |
| `LR_REGISTRY_INSECURE` | Skip TLS verification |
| `LR_LOG_LEVEL` | Log level (debug, info, warn, error) |

## Using with k3d

```bash
# Create k3d cluster with local registry
k3d cluster create dev --registry-use local-registry:5000

# Add images and use in deployments
local-registry add nginx:alpine
kubectl run nginx --image=localhost:5000/nginx:alpine
```

## Using with kind

```bash
# Create kind cluster with registry config
cat <<EOF | kind create cluster --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:5000"]
    endpoint = ["http://local-registry:5000"]
EOF

# Connect registry to kind network
docker network connect kind local-registry
```

## Building

```bash
# Production build with version info
make build

# Development build (faster)
make dev

# Custom version
make build VERSION=1.0.0

# Install to $GOPATH/bin
make install

# Check embedded version
./bin/local-registry version
```

## TODO

- TUI Improvement

## License

MIT License - see [LICENSE](LICENSE) for details.
