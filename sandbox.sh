#!/usr/bin/env bash
set -euo pipefail

# ============================================================================
# sandbox.sh — CLI entry point for the sandbox tool
# ============================================================================

# Constants and defaults
readonly DEFAULT_CONFIG_PATH=".sandbox/config.yaml"
readonly SANDBOX_VERSION="0.1.0"

# ============================================================================
# Utility functions
# ============================================================================

die() {
  local message="${1}"
  local code="${2:-1}"
  echo "error: ${message}" >&2
  exit "${code}"
}

info() {
  echo "${1}"
}

warn() {
  echo "warning: ${1}" >&2
}

# ============================================================================
# Dependency checking
# ============================================================================

check_bash_version() {
  if [[ "${BASH_VERSINFO[0]}" -lt 4 ]]; then
    die "bash 4+ required (current: ${BASH_VERSINFO[0]}.${BASH_VERSINFO[1]}) -- on macOS run: brew install bash" 3
  fi
}

check_dependencies() {
  if ! command -v docker >/dev/null 2>&1; then
    die "docker not found -- install Docker Desktop or Docker Engine (https://docs.docker.com/get-docker/)" 3
  fi

  if ! command -v yq >/dev/null 2>&1; then
    die "yq not found -- install yq v4+ (https://github.com/mikefarah/yq/#install)" 3
  fi

  local yq_version_output
  yq_version_output="$(yq --version 2>&1)"
  local yq_major
  if [[ "${yq_version_output}" =~ v([0-9]+) ]]; then
    yq_major="${BASH_REMATCH[1]}"
  elif [[ "${yq_version_output}" =~ version[[:space:]]+([0-9]+)\.[0-9]+ ]]; then
    yq_major="${BASH_REMATCH[1]}"
  else
    die "unable to parse yq version from: ${yq_version_output}" 3
  fi

  if [[ "${yq_major}" -lt 4 ]]; then
    die "yq version ${yq_major}.x detected -- sandbox requires yq v4+ (https://github.com/mikefarah/yq/#install)" 3
  fi
}

# ============================================================================
# Config parsing functions (stub)
# ============================================================================

# Implemented in a later story

# ============================================================================
# Build functions (stub)
# ============================================================================

cmd_build() {
  info "not yet implemented"
}

# ============================================================================
# Run functions (stub)
# ============================================================================

cmd_run() {
  info "not yet implemented"
}

# ============================================================================
# Init function (stub)
# ============================================================================

cmd_init() {
  info "not yet implemented"
}

# ============================================================================
# Command dispatch
# ============================================================================

show_help() {
  cat <<'HELPTEXT'
Usage: sandbox <command> [options]

Commands:
  init    Generate a starter .sandbox/config.yaml
  build   Build the sandbox container image
  run     Launch an interactive sandbox session

Options:
  -f <path>   Use specified config file (default: .sandbox/config.yaml)
  --help      Show this help message
HELPTEXT
}

parse_args() {
  local config_path="${DEFAULT_CONFIG_PATH}"

  if [[ "${#}" -eq 0 ]]; then
    show_help
    exit 0
  fi

  while [[ "${#}" -gt 0 ]]; do
    case "${1}" in
      --help)
        show_help
        exit 0
        ;;
      -f)
        if [[ "${#}" -lt 2 ]]; then
          die "option -f requires an argument" 2
        fi
        config_path="${2}"
        shift 2
        ;;
      init)
        cmd_init
        exit 0
        ;;
      build)
        cmd_build
        exit 0
        ;;
      run)
        cmd_run
        exit 0
        ;;
      *)
        die "unknown command '${1}'" 2
        ;;
    esac
  done
}

# ============================================================================
# Main entry point
# ============================================================================

main() {
  check_bash_version
  check_dependencies
  parse_args "$@"
}

main "$@"
