#!/usr/bin/env bash
set -euo pipefail
# Container entrypoint: UID/GID alignment, volume prep, service start, exec agent

die() {
    echo "FATAL: $*" >&2
    exit 1
}

align_uid_gid() {
    local host_uid="${HOST_UID:-}"
    local host_gid="${HOST_GID:-}"

    if [[ -z "${host_uid}" || -z "${host_gid}" ]]; then
        return 0
    fi

    local current_uid
    local current_gid
    current_uid="$(id -u sandbox)"
    current_gid="$(id -g sandbox)"

    if [[ "${current_uid}" == "${host_uid}" && "${current_gid}" == "${host_gid}" ]]; then
        return 0
    fi

    # Remove conflicting user if target UID is taken
    local conflicting_user
    conflicting_user="$(getent passwd "${host_uid}" | cut -d: -f1 || true)"
    if [[ -n "${conflicting_user}" && "${conflicting_user}" != "sandbox" ]]; then
        userdel "${conflicting_user}"
    fi

    # Remove conflicting group if target GID is taken
    local conflicting_group
    conflicting_group="$(getent group "${host_gid}" | cut -d: -f1 || true)"
    if [[ -n "${conflicting_group}" && "${conflicting_group}" != "sandbox" ]]; then
        groupdel "${conflicting_group}"
    fi

    groupmod -g "${host_gid}" sandbox
    usermod -u "${host_uid}" -g "${host_gid}" sandbox
}

chown_volumes() {
    # Chown named volume mounts for auto_isolate_deps
    local volume_paths="${AUTO_ISOLATE_VOLUME_PATHS:-}"
    if [[ -z "${volume_paths}" ]]; then
        return 0
    fi

    IFS=',' read -ra paths <<< "${volume_paths}"
    for path in "${paths[@]}"; do
        if [[ -d "${path}" ]]; then
            chown -R sandbox:sandbox "${path}"
        fi
    done
}

setup_codex_home() {
    local codex_home="/home/sandbox/.codex"
    local host_config="/opt/codex-config"

    # Only act when host config directory is mounted and non-empty
    if [[ ! -d "${host_config}" ]] || [[ -z "$(ls -A "${host_config}" 2>/dev/null)" ]]; then
        return 0
    fi

    mkdir -p "${codex_home}"

    # Symlink host config/auth files into CODEX_HOME (skip instruction files)
    shopt -s dotglob
    for f in "${host_config}"/*; do
        [[ -e "$f" ]] || continue
        local base
        base="$(basename "$f")"
        case "$base" in
            AGENTS.md|AGENTS.override.md) continue ;;
        esac
        # Don't overwrite existing files (could be bind mounts)
        if [[ ! -e "${codex_home}/$base" ]]; then
            ln -sf "$f" "${codex_home}/$base"
        fi
    done

    shopt -u dotglob

    chown -Rh sandbox:sandbox "${codex_home}"
}

merge_mcp_config() {
    local build_config="/etc/sandbox/mcp-servers.json"
    local project_config="/workspace/.mcp.json"
    local merged_config="/home/sandbox/.mcp.json"

    if [[ ! -f "${build_config}" && ! -f "${project_config}" ]]; then
        return 0
    fi

    if [[ ! -f "${build_config}" ]]; then
        cp "${project_config}" "${merged_config}"
        chown sandbox:sandbox "${merged_config}"
        return 0
    fi

    if [[ ! -f "${project_config}" ]]; then
        cp "${build_config}" "${merged_config}"
        chown sandbox:sandbox "${merged_config}"
        return 0
    fi

    # Merge: project config wins on name conflicts
    jq -s '.[0] * .[1]' "${build_config}" "${project_config}" > "${merged_config}"
    chown sandbox:sandbox "${merged_config}"
}

start_healthcheck_poller() {
    # Must run as sandbox user so podman sees the rootless containers.
    # Running as root would query root's (empty) container store.
    gosu sandbox /usr/local/bin/healthcheck-poller.sh &
    HEALTHCHECK_PID=$!
    trap 'kill "${HEALTHCHECK_PID}" 2>/dev/null || true' EXIT
}

set_testcontainers_socket() {
    local socket_path="/run/user/$(id -u sandbox)/podman/podman.sock"
    export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE="${socket_path}"
}

persist_env() {
    # Write dynamic env vars to a profile script so they are available
    # in docker exec sessions (which don't inherit entrypoint exports).
    local profile="/etc/profile.d/sandbox-env.sh"
    cat > "${profile}" <<ENVEOF
export DOCKER_HOST="${DOCKER_HOST:-}"
export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE="${TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE:-}"
export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-}"
ENVEOF
    chmod 644 "${profile}"
}

start_podman_socket() {
    export XDG_RUNTIME_DIR="/run/user/$(id -u sandbox)"
    mkdir -p "${XDG_RUNTIME_DIR}"
    chown sandbox:sandbox "${XDG_RUNTIME_DIR}"

    local socket_path="${XDG_RUNTIME_DIR}/podman/podman.sock"
    mkdir -p "$(dirname "${socket_path}")"
    chown sandbox:sandbox "$(dirname "${socket_path}")"

    # Initialize rootless Podman storage/config on first run
    gosu sandbox podman system migrate 2>&1 | grep -v "^$" >&2 || true

    # Start Podman API socket
    gosu sandbox podman system service --time=0 "unix://${socket_path}" &

    # Export DOCKER_HOST so Docker Compose and SDK clients find the Podman socket
    export DOCKER_HOST="unix://${socket_path}"

    # Create docker.sock symlink for tools that hardcode the socket path
    ln -sf "${socket_path}" "${XDG_RUNTIME_DIR}/docker.sock" 2>/dev/null || true

    # Wait for socket readiness (up to 5 seconds)
    local i
    for i in 1 2 3 4 5; do
        if [[ -S "${socket_path}" ]]; then
            break
        fi
        sleep 1
    done

    if [[ ! -S "${socket_path}" ]]; then
        echo "WARNING: Podman socket not ready after 5 seconds at ${socket_path}" >&2
    fi
}

unmask_proc() {
    # Docker masks /proc sub-paths as read-only (e.g. /proc/sys) and overlays
    # others with tmpfs. These masks prevent rootless Podman from mounting
    # /proc inside nested containers and from writing sysctl tunables like
    # /proc/sys/net/ipv4/ping_group_range. Unmounting the overlays restores
    # full /proc access, which is safe because the sandbox already runs with
    # SYS_ADMIN + seccomp=unconfined.
    mount --make-rshared / 2>/dev/null || true
    # The trailing || true guards against pipefail: grep returns 1 when
    # there are no /proc submounts, which would exit the script.
    grep ' /proc/' /proc/self/mountinfo | awk '{print $5}' | sort -r | while read -r mp; do
        umount "$mp" 2>/dev/null || true
    done || true
}

# Main entrypoint sequence
unmask_proc
align_uid_gid
chown_volumes
setup_codex_home
merge_mcp_config
start_podman_socket
set_testcontainers_socket
persist_env
start_healthcheck_poller

# Exec into agent command as sandbox user
if [[ $# -gt 0 ]]; then
    exec gosu sandbox "$@"
elif [[ -n "${AGENT_CMD:-}" ]]; then
    # Intentional word-split: AGENT_CMD is a trusted token list from the Go CLI.
    # shellcheck disable=SC2086
    exec gosu sandbox ${AGENT_CMD}
else
    die "No agent command specified"
fi
