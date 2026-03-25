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
  # Re-exec this script as the sandbox user
  exec runuser -u sandbox -- "$0" "$@"
fi

# Initialize Podman rootless runtime (required before agent can use docker/podman)
if command -v podman >/dev/null 2>&1; then
  export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
  mkdir -p "${XDG_RUNTIME_DIR}" 2>/dev/null || true
  podman system migrate 2>/dev/null || true
  podman info >/dev/null 2>&1 || echo "warning: podman info failed" >&2
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
