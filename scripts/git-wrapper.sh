#!/usr/bin/env bash
set -euo pipefail
# Transparent passthrough to real git - push blocking added in Story 3-1
exec /usr/bin/git "$@"
