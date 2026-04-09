#!/usr/bin/env bash
set -euo pipefail
# Healthcheck poller: polls podman healthcheck run every 10s with fault tolerance

run_healthchecks() {
    local containers
    containers="$(podman ps --filter health=starting --filter health=healthy --format '{{.ID}}' 2>/dev/null || true)"
    for cid in ${containers}; do
        podman healthcheck run "${cid}" >/dev/null 2>&1 || true
    done
}

# Trap SIGTERM/SIGINT for clean shutdown
trap 'exit 0' TERM INT

# Restart loop for fault tolerance — trap ensures clean exit on signal
while true; do
    run_healthchecks || true
    sleep 10
done
