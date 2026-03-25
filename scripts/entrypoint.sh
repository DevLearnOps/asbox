#!/usr/bin/env bash
set -euo pipefail

# Rootful init: fix mount propagation for nested containers, then drop to sandbox user
if [[ "$(id -u)" == "0" ]]; then
  # Fix "/" not shared mount — required for Podman/crun to mount /proc in inner containers
  mount --make-rshared / 2>/dev/null || true
  # Remove Docker's read-only bind mounts on /proc sub-paths so crun can mount
  # /proc inside inner containers. These are security masks Docker applies; removing
  # them is safe because the sandbox user has no direct access (runs as non-root).
  grep " /proc/" /proc/self/mountinfo | awk '{print $5}' | sort -r | while read -r mp; do
    umount "$mp" 2>/dev/null || true
  done
  # Align sandbox user UID/GID with host user for bind mount permissions
  if [[ -n "${HOST_UID:-}" ]]; then
    if ! [[ "${HOST_UID}" =~ ^[0-9]+$ ]] || [[ "${HOST_UID}" -eq 0 ]]; then
      echo "error: HOST_UID must be a non-zero numeric value (got: ${HOST_UID})" >&2
      exit 1
    fi
    current_uid="$(id -u sandbox)"
    if [[ "${HOST_UID}" != "${current_uid}" ]]; then
      usermod -u "${HOST_UID}" sandbox
      groupmod -g "${HOST_GID:-$(id -g sandbox)}" sandbox 2>/dev/null || true
      chown -R sandbox:sandbox /home/sandbox
    fi
  fi

  # Ensure XDG_RUNTIME_DIR exists for the sandbox user (must run as root since /run/user is root-owned)
  runtime_dir="/run/user/$(id -u sandbox)"
  mkdir -p "${runtime_dir}"
  chown sandbox:sandbox "${runtime_dir}"
  chmod 700 "${runtime_dir}"

  # Re-exec this script as the sandbox user
  exec runuser -u sandbox -- "$0" "$@"
fi

# Initialize Podman rootless runtime (required before agent can use docker/podman)
if command -v podman >/dev/null 2>&1; then
  export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
  mkdir -p "${XDG_RUNTIME_DIR}" 2>/dev/null || true
  podman system migrate 2>/dev/null || true
  podman info >/dev/null 2>&1 || echo "warning: podman info failed" >&2

  # Start Podman API socket (required for Docker Compose v2 to communicate with Podman)
  mkdir -p "${XDG_RUNTIME_DIR}/podman" 2>/dev/null || true
  podman system service --time=0 "unix://${XDG_RUNTIME_DIR}/podman/podman.sock" &

  # Expose the Podman socket as DOCKER_HOST so docker-compose (standalone) can find it.
  # Also create docker.sock symlink (systemd-tmpfiles would do this but there's no systemd).
  export DOCKER_HOST="unix://${XDG_RUNTIME_DIR}/podman/podman.sock"
  ln -sf "${XDG_RUNTIME_DIR}/podman/podman.sock" "${XDG_RUNTIME_DIR}/docker.sock" 2>/dev/null || true

  # Wait briefly for the socket to become available
  for _ in 1 2 3 4 5; do
    if podman info >/dev/null 2>&1; then
      break
    fi
    sleep 0.5
  done
fi

# Generate .mcp.json from build-time manifest
MCP_MANIFEST="/etc/sandbox/mcp-servers.json"
if [[ -f "${MCP_MANIFEST}" ]]; then
  server_count="$(jq '.mcpServers | length' "${MCP_MANIFEST}" 2>/dev/null || echo 0)"
  if [[ "${server_count}" -gt 0 ]]; then
    if [[ -f ".mcp.json" ]]; then
      # Merge: project config takes precedence on name conflicts
      tmp_file="$(mktemp)"
      cp ".mcp.json" "${tmp_file}"
      jq -r '.mcpServers | keys[]' "${MCP_MANIFEST}" | while IFS= read -r server_name; do
        if jq -e --arg name "${server_name}" '.mcpServers[$name]' "${tmp_file}" >/dev/null 2>&1; then
          echo "sandbox: skipping ${server_name} (project override exists)"
        else
          jq --arg name "${server_name}" \
            --argjson config "$(jq --arg name "${server_name}" '.mcpServers[$name]' "${MCP_MANIFEST}")" \
            '.mcpServers[$name] = $config' "${tmp_file}" > "${tmp_file}.new" && mv "${tmp_file}.new" "${tmp_file}"
          echo "sandbox: added ${server_name} to .mcp.json"
        fi
      done
      mv "${tmp_file}" ".mcp.json"
    else
      cp "${MCP_MANIFEST}" ".mcp.json"
      echo "sandbox: generated .mcp.json with $(jq -r '.mcpServers | keys | join(", ")' "${MCP_MANIFEST}") servers"
    fi
  fi
fi

if [[ -z "${SANDBOX_AGENT:-}" ]]; then
  echo "error: SANDBOX_AGENT not set" >&2
  exit 1
fi

case "${SANDBOX_AGENT}" in
  claude-code)
    command -v claude >/dev/null 2>&1 || { echo "error: claude not found in PATH" >&2; exit 1; }
    exec claude --dangerously-skip-permissions
    ;;
  gemini-cli)
    command -v gemini >/dev/null 2>&1 || { echo "error: gemini not found in PATH" >&2; exit 1; }
    exec gemini
    ;;
  *)
    echo "error: unknown agent: ${SANDBOX_AGENT}" >&2
    exit 1
    ;;
esac
