#!/usr/bin/env bash
set -euo pipefail

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
