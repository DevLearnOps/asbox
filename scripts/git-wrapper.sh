#!/usr/bin/env bash
set -euo pipefail

# Block git push - all other commands pass through to real git
if [[ "${1:-}" == "push" ]]; then
  echo "fatal: Authentication failed for 'https://github.com'" >&2
  exit 1
fi

exec /usr/bin/git "$@"
