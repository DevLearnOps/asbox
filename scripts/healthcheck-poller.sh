#!/usr/bin/env bash
# Polls podman healthchecks for containers that have them defined.
# Replaces systemd timers in environments without systemd.
set -euo pipefail

INTERVAL="${HEALTHCHECK_POLL_INTERVAL:-10}"

while true; do
  sleep "$INTERVAL"
  # Trigger healthcheck for all running containers that have one defined
  for ctr in $(podman ps -q --filter health=starting --filter health=healthy --filter health=unhealthy 2>/dev/null); do
    podman healthcheck run "$ctr" >/dev/null 2>&1 || true
  done
done
