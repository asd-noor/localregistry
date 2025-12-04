#!/usr/bin/env bash
set -euo pipefail

PATCH_TRAEFIK=${PATCH_TRAEFIK:-0}
install_dir="${INSTALL_DIR:-$(pwd)}"

# If installing to a different directory, copy project files
if [[ "$install_dir" != "$PWD" ]]; then
    mkdir -p "$install_dir"
    # Copy only necessary files (adjust patterns as needed)
    cp -v Makefile docker-compose.yaml traefik.yaml local-registry "$install_dir/"
    echo "Copied files to $install_dir"
    if [[ $PATCH_TRAEFIK -ne 0 ]]; then
        # Update the default values of REGISTRY_API_URL and REGISTRY_UI_URL in the copied Makefile
        if command -v sed >/dev/null 2>&1; then
            if [[ -n ${REGISTRY_API_URL:-} ]]; then
                sed -i "s|^REGISTRY_API_URL[[:space:]]*?=.*|REGISTRY_API_URL ?= ${REGISTRY_API_URL}|" "$install_dir/Makefile"
            fi
            if [[ -n ${REGISTRY_UI_URL:-} ]]; then
                sed -i "s|^REGISTRY_UI_URL[[:space:]]*?=.*|REGISTRY_UI_URL ?= ${REGISTRY_UI_URL}|" "$install_dir/Makefile"
            fi
            echo "Patched Makefile with custom REGISTRY_API_URL and REGISTRY_UI_URL"
        else
            echo "Warning: sed not found, skipping Makefile patching" >&2
        fi
    fi
fi

# Export all registry environment variables to shell profiles (idempotent)
declare -a export_lines=(
    "export REGISTRY_COMPOSE_DIR=\"${install_dir}\""
    "export REGISTRY_DATA_SOURCE=\"\${REGISTRY_DATA_SOURCE:-${install_dir}/data}\""
    "export REGISTRY_API_URL=\"\${REGISTRY_API_URL:-${REGISTRY_API_URL:-localhost}}\""
    "export REGISTRY_API_PORT=\"\${REGISTRY_API_PORT:-${REGISTRY_API_PORT:-50000}}\""
    "export REGISTRY_UI_URL=\"\${REGISTRY_UI_URL:-${REGISTRY_UI_URL:-localhost}}\""
    "export REGISTRY_UI_PORT=\"\${REGISTRY_UI_PORT:-${REGISTRY_UI_PORT:-49159}}\""
    "export REGISTRY_API_CONT_VER=\"\${REGISTRY_API_CONT_VER:-${REGISTRY_API_CONT_VER:-latest}}\""
    "export REGISTRY_UI_CONT_VER=\"\${REGISTRY_UI_CONT_VER:-${REGISTRY_UI_CONT_VER:-latest}}\""
)

for rcfile in "$HOME/.zshenv" "$HOME/.bashrc"; do
    if [[ -f "$rcfile" ]]; then
        for export_line in "${export_lines[@]}"; do
            var_name=$(echo "$export_line" | sed -n 's/^export \([A-Z_]*\)=.*/\1/p')
            if ! grep -qF "export ${var_name}=" "$rcfile"; then
                echo "$export_line" >>"$rcfile"
                echo "Added ${var_name} to $rcfile"
            else
                echo "${var_name} already present in $rcfile"
            fi
        done
    fi
done

# Install helper script to user bin
mkdir -p "${HOME}/.local/bin"
cp -v local-registry "${HOME}/.local/bin/local-registry"
chmod +x "${HOME}/.local/bin/local-registry"

echo "✓ Installed local-registry to ${HOME}/.local/bin"
echo "  Ensure ${HOME}/.local/bin is in your \$PATH"

printf "For bash completions, add %s in your in your bashrc\n" 'eval "$(local-registry completion-bash)"'
printf "For zsh completions, add %s in your in your zshrc\n" 'eval "$(local-registry completion-zsh)"'
