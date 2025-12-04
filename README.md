# Local Registry Stack

Local Docker Registry packaged with Traefik routing and a modern UI. This document covers:

- Running the stack via Docker Compose.
- Making Docker trust the insecure registry endpoint.
- Integrating with local Kubernetes clusters (kind, k3d).
- Connecting additional Docker Compose stacks.
- Optional Traefik generation workflow.

## Prerequisites

- Docker 20.10+
- `docker compose` plugin
- [`yq`](https://github.com/mikefarah/yq) and [`jq`](https://stedolan.github.io/jq/) available on your shell `$PATH`
- For Kubernetes integration: `kubectl`, plus `kind` or `k3d`

## Installation

The `install` target copies the stack to a target directory, installs the `local-registry` helper script to `~/.local/bin`, and exports `REGISTRY_COMPOSE_DIR` to your shell profile.

### Quick install (current directory)

```bash
make install
```

This will:
- Install `local-registry` to `~/.local/bin/local-registry`
- Export `REGISTRY_COMPOSE_DIR` to `~/.zshenv` and `~/.bashrc` (idempotent)
- Ensure `~/.local/bin` is in your `$PATH`

### Install to custom directory

```bash
make install INSTALL_DIR=/opt/local-registry
```

Copies all project files to `/opt/local-registry` and installs the helper script.

### Install with custom registry URLs

To override the default `localhost` URLs and patch the Makefile defaults:

```bash
PATCH_TRAEFIK=1 \
REGISTRY_API_URL=registry.example.com \
REGISTRY_UI_URL=ui.example.com \
make install INSTALL_DIR=/opt/local-registry
```

When `PATCH_TRAEFIK=1`, the installer updates `REGISTRY_API_URL` and `REGISTRY_UI_URL` in the copied Makefile to the provided values.

### Post-installation

1. **Verify installation:**
   ```bash
   local-registry help
   ```

2. **Enable shell completions** (optional):
   ```bash
   # Bash
   eval "$(local-registry completion-bash)"
   
   # Zsh
   eval "$(local-registry completion-zsh)"
   ```
   Add the `eval` line to your shell profile for persistence.

3. **Start the registry:**
   ```bash
   local-registry start-server
   # or navigate to REGISTRY_COMPOSE_DIR and run:
   make start
   ```

## Configuration Surface

Environment-driven knobs live in the top of the `Makefile`:

```makefile
REGISTRY_API_CONT_VER ?= 3.0.0
REGISTRY_API_PORT     ?= 50000
REGISTRY_API_URL      ?= registry.localhost

REGISTRY_UI_CONT_VER  ?= 2.0.0
REGISTRY_UI_PORT      ?= 49159
REGISTRY_UI_URL       ?= registry-ui.localhost

REGISTRY_DATA_SOURCE  ?= ./data
```

Key behaviors:

- `REGISTRY_API_URL` / `REGISTRY_API_PORT` drive the registry endpoint exposed to clients.
- `REGISTRY_UI_URL` / `REGISTRY_UI_PORT` determine the UI host/port and Traefik routing rules.
- `REGISTRY_DATA_SOURCE` controls the Docker volume or bind mount feeding `/var/lib/registry` (default `./data`).
- Compose pulls the matching image tags derived from `REGISTRY_API_CONT_VER` and `REGISTRY_UI_CONT_VER`.

Run `make gen-traefik` whenever you tweak any of the host/port values to rewrite `traefik.yaml` appropriately.

## Bootstrapping the Stack

```bash
make patch-docker-config   # Add registry.localhost:50000 to insecure registries
make start                 # docker compose up -d
# or: make run (foreground)
```

### What happens

- Registry starts with deletion enabled and CORS headers set for the UI origin.
- UI points at the registry via an internal proxy (`NGINX_PROXY_PASS_URL=http://registry:5000`).
- `PULL_URL` surfaces the correct CLI endpoint (e.g. `registry.localhost:50000`).

## Validating

1. Visit `http://registry-ui.localhost` – you should see catalog results (empty if unused).
2. Push an image, e.g.:
   ```bash
   docker tag alpine registry.localhost:50000/demo/alpine:latest
   docker push registry.localhost:50000/demo/alpine:latest
   ```
3. Refresh the UI; the image should appear and be deletable (`DELETE_IMAGES=true`).

## Integrating with kind

kind nodes run inside Docker and need two considerations:

1. **Trust the registry**

   After the stack is up, inject the registry endpoint into kind nodes:

   ```bash
   cat <<EOF | kind create cluster --name local-registry --config -
   kind: Cluster
   apiVersion: kind.x-k8s.io/v1alpha4
   containerdConfigPatches:
     - |-
       [plugins."io.containerd.grpc.v1.cri".registry]
         [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
           [plugins."io.containerd.grpc.v1.cri".registry.mirrors."registry.localhost:${REGISTRY_API_PORT}"]
             endpoint = ["http://host.docker.internal:${REGISTRY_API_PORT}"]
   EOF
   ```

   For Linux hosts lacking `host.docker.internal`, swap with the bridge IP (often `172.17.0.1`).

2. **Use the registry inside the cluster**

   Add a `docker-registry` secret or use image pull specs like:

   ```yaml
   image: registry.localhost:50000/demo/alpine:latest
   imagePullPolicy: IfNotPresent
   ```

   Since we run without TLS, ensure workloads use plain HTTP (`imagePullSecrets` unnecessary unless you enable auth).

### Existing kind clusters

Patch running clusters by logging onto each node and editing `/etc/containerd/certs.d`:

```bash
for node in $(kind get nodes --name local-registry); do
  docker exec "$node" /bin/sh -c '
    mkdir -p /etc/containerd/certs.d/registry.localhost:${REGISTRY_API_PORT} &&
    cat <<EOF >/etc/containerd/certs.d/registry.localhost:${REGISTRY_API_PORT}/hosts.toml
server = "http://registry.localhost:${REGISTRY_API_PORT}"
[host."http://host.docker.internal:${REGISTRY_API_PORT}"]
  capabilities = ["pull", "resolve"]
EOF
  '
done
```

> Replace `host.docker.internal` with the bridge IP on Linux if needed.

## Integrating with k3d

k3d builds on k3s and provides native flags for registries:

```bash
k3d registry create localregistry --port ${REGISTRY_API_PORT}
k3d cluster create dev \
  --registry-use k3d-localregistry:${REGISTRY_API_PORT} \
  --api-port 6550 \
  --servers 1 --agents 1
```

- `k3d registry create` spins a managed registry container. Instead, point it at the existing stack by providing a registry config file:

```bash
> cat <<EOF >k3d-registry.yaml
mirrors:
  "registry.localhost:${REGISTRY_API_PORT}":
    endpoint:
      - "http://host.docker.internal:${REGISTRY_API_PORT}"
EOF

> k3d cluster create dev --registry-config k3d-registry.yaml
```

- For Linux hosts, replace `host.docker.internal` with the Docker bridge IP.

Once the cluster is live, reference images via `registry.localhost:50000/...` in workloads or Helm charts.

## Reusing the registry across Docker Compose stacks

Any Compose application can target the registry by adding:

```yaml
services:
  backend:
    build: .
    image: registry.localhost:${REGISTRY_API_PORT}/my-app/backend:latest
    push: true
```

Before pushing from CI-like flows, log in (optional if you enable auth) and ensure the daemon trusts the registry (`make patch-docker-config`). When Traefik is fronting multiple services, create distinct routers/services in `traefik.yaml` and rerun `make gen-traefik`.

### Registry helper script

`local-registry` consolidates registry operations:

```bash
./local-registry add ghcr.io/myorg/service:latest
./local-registry add ghcr.io/myorg/service:latest demo/service:edge
./local-registry delete-tag demo/service latest
./local-registry delete-repo demo/service
```

Environment knobs:

- `REGISTRY_API_URL` / `REGISTRY_API_PORT` / `REGISTRY_DATA_SOURCE` – override registry endpoint and local data root (defaults match the Makefile).
- `SKOPEO_DEBUG=true` – enable verbose `skopeo` logging.

The `add` command uses `skopeo copy --dest-tls-verify=false`; delete commands hit the v2 API and require `jq` for tag enumeration.

#### Shell completions

```bash
# Bash
eval "$(./local-registry completion-bash)"
# or persist
local-registry completion-bash | sudo tee /etc/bash_completion.d/local-registry > /dev/null

# Zsh
eval "$(./local-registry completion-zsh)"
```

Add the `eval` line to your shell profile for automatic setup.

## Troubleshooting

- **CORS errors**: confirm `REGISTRY_HTTP_HEADERS_*` env vars are present on the registry service and that `REGISTRY_URL` in the UI includes the port.
- **Catalog 404**: re-run `make gen-traefik` after changing hostnames/ports.
- **Docker push fails with TLS error**: rerun `make patch-docker-config` and restart Docker.
- **kind image pulls fail**: check `/etc/containerd/certs.d` inside nodes for the correct endpoint mapping.

## Housekeeping

- `make stop` takes down the compose stack.
- `make update` pulls new container images.
- To upgrade versions, adjust `REGISTRY_API_CONT_VER` / `REGISTRY_UI_CONT_VER`, regenerate Traefik, and `make update && make start`.

Happy shipping!

