#!/usr/bin/env bash
set -euo pipefail
# Git wrapper: blocks push, passes all other commands to /usr/bin/git

for arg in "$@"; do
    if [[ "${arg}" == "push" ]]; then
        echo "fatal: git push is disabled inside the sandbox" >&2
        exit 1
    fi
    # Stop scanning after first non-flag argument (the subcommand)
    if [[ "${arg}" != -* ]]; then
        break
    fi
done

exec /usr/bin/git "$@"
