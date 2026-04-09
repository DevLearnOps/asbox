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
    /usr/local/bin/healthcheck-poller.sh &
    HEALTHCHECK_PID=$!
    trap 'kill "${HEALTHCHECK_PID}" 2>/dev/null || true' EXIT
}

set_testcontainers_socket() {
    local socket_path="/run/user/$(id -u sandbox)/podman/podman.sock"
    export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE="${socket_path}"
}

start_podman_socket() {
    local socket_path="/run/user/$(id -u sandbox)/podman/podman.sock"
    mkdir -p "$(dirname "${socket_path}")"
    chown sandbox:sandbox "$(dirname "${socket_path}")"
    gosu sandbox podman system service --time=0 "unix://${socket_path}" &
}

# Main entrypoint sequence
align_uid_gid
chown_volumes
merge_mcp_config
set_testcontainers_socket
start_healthcheck_poller
start_podman_socket

# Exec into agent command as sandbox user
if [[ $# -gt 0 ]]; then
    exec gosu sandbox "$@"
elif [[ -n "${AGENT_CMD:-}" ]]; then
    exec gosu sandbox bash -c "${AGENT_CMD}"
else
    die "No agent command specified"
fi
