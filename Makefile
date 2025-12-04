REGISTRY_DATA_SOURCE ?= ./data

REGISTRY_API_CONT_VER ?= latest
REGISTRY_API_PORT     ?= 50000
REGISTRY_API_URL      ?= localhost

REGISTRY_UI_CONT_VER ?= latest
REGISTRY_UI_PORT     ?= 49159
REGISTRY_UI_URL      ?= localhost

export REGISTRY_DATA_SOURCE \
       REGISTRY_API_CONT_VER REGISTRY_API_PORT REGISTRY_API_URL \
       REGISTRY_UI_CONT_VER REGISTRY_UI_PORT REGISTRY_UI_URL

INSTALL_DIR ?= $(HOME)/.localregistry

.PHONY: help start stop update run gen-traefik patch-docker-config install

help:
	@echo "Makefile commands for Local Registry service:"
	@echo "  run                 - Start the Local Registry Server in Foreground"
	@echo "  start               - Start the Local Registry Server in Background"
	@echo "  stop                - Stop the Local Registry Server"
	@echo "  update              - Pull latest container images"
	@echo "  gen-traefik         - Regenerate traefik.yaml from current env vars"
	@echo "  patch-docker-config - Add registry to Docker insecure-registries"
	@echo "  install             - Install local-registry helper and export REGISTRY_COMPOSE_DIR"

start:
	docker compose up -d

stop:
	docker compose down

update:
	docker compose pull

run:
	docker compose up

gen-traefik:
	@command -v yq >/dev/null 2>&1 || { echo "missing dependency: yq" >&2; exit 1; }
	@API_URL="$(REGISTRY_API_URL)"; \
	UI_URL="$(REGISTRY_UI_URL)"; \
	[ "$$API_URL" = "localhost" ] && API_URL="registry.localhost"; \
	[ "$$UI_URL" = "localhost" ] && UI_URL="registry-ui.localhost"; \
	export REGISTRY_API_URL="$$API_URL"; \
	export REGISTRY_UI_URL="$$UI_URL"; \
	export REGISTRY_API_PORT="$(REGISTRY_API_PORT)"; \
	export REGISTRY_UI_PORT="$(REGISTRY_UI_PORT)"; \
	yq eval -i '.http.routers.registry.rule = "Host(`" + strenv(REGISTRY_API_URL) + "`)"' traefik.yaml; \
	yq eval -i '.http.services.registry.loadBalancer.servers[0].url = "http://localhost:" + strenv(REGISTRY_API_PORT)' traefik.yaml; \
	yq eval -i '.http.routers.registry-ui.rule = "Host(`" + strenv(REGISTRY_UI_URL) + "`)"' traefik.yaml; \
	yq eval -i '.http.services."registry-ui".loadBalancer.servers[0].url = "http://localhost:" + strenv(REGISTRY_UI_PORT)' traefik.yaml
	@echo "Updated traefik.yaml"

patch-docker-config:
	@command -v jq >/dev/null 2>&1 || { echo "missing dependency: jq" >&2; exit 1; }
	@mkdir -p ~/.docker
	@entry="$(REGISTRY_API_URL):$(REGISTRY_API_PORT)"; \
	if [ -f ~/.docker/config.json ]; then \
		jq --arg entry "$$entry" '."insecure-registries" = (((."insecure-registries" // []) + [$$entry]) | unique)' ~/.docker/config.json > ~/.docker/config.tmp && mv ~/.docker/config.tmp ~/.docker/config.json; \
	else \
		printf '{"insecure-registries": ["%s"]}\n' "$$entry" > ~/.docker/config.json; \
	fi
	@echo "Patched docker configuration"

install:
	INSTALL_DIR="$(INSTALL_DIR)" ./install.sh
