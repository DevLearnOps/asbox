#!/usr/bin/env bash
set -euo pipefail

# ============================================================================
# sandbox.sh — CLI entry point for the sandbox tool
# ============================================================================

# Constants and defaults
readonly DEFAULT_CONFIG_PATH=".sandbox/config.yaml"
readonly SANDBOX_VERSION="0.1.0"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_PATH="${DEFAULT_CONFIG_PATH}"

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
# Config globals (initialized before parsing)
# ============================================================================

CFG_AGENT=""
CFG_SDK_NODEJS=""
CFG_SDK_PYTHON=""
CFG_SDK_GO=""
CFG_PACKAGES=()
CFG_SECRETS=()
CFG_MCP=()
CFG_MOUNT_SOURCES=()
CFG_MOUNT_TARGETS=()
CFG_ENV_KEYS=()
CFG_ENV_VALUES=()

# ============================================================================
# Config parsing functions
# ============================================================================

parse_config() {
  if [[ ! -f "${CONFIG_PATH}" ]]; then
    die "config not found: ${CONFIG_PATH}" 1
  fi

  # Extract agent (required)
  CFG_AGENT="$(yq eval '.agent // ""' "${CONFIG_PATH}")"
  if [[ -z "${CFG_AGENT}" || "${CFG_AGENT}" == "null" ]]; then
    die "config missing required field: agent" 1
  fi
  if [[ "${CFG_AGENT}" != "claude-code" && "${CFG_AGENT}" != "gemini-cli" ]]; then
    die "config invalid agent: ${CFG_AGENT} (expected: claude-code, gemini-cli)" 1
  fi

  # Extract SDK versions (optional)
  CFG_SDK_NODEJS="$(yq eval '.sdks.nodejs // ""' "${CONFIG_PATH}")"
  if [[ "${CFG_SDK_NODEJS}" == "null" ]]; then CFG_SDK_NODEJS=""; fi
  CFG_SDK_PYTHON="$(yq eval '.sdks.python // ""' "${CONFIG_PATH}")"
  if [[ "${CFG_SDK_PYTHON}" == "null" ]]; then CFG_SDK_PYTHON=""; fi
  CFG_SDK_GO="$(yq eval '.sdks.go // ""' "${CONFIG_PATH}")"
  if [[ "${CFG_SDK_GO}" == "null" ]]; then CFG_SDK_GO=""; fi

  # Extract packages (optional array)
  CFG_PACKAGES=()
  local pkg_count
  pkg_count="$(yq eval '.packages | length' "${CONFIG_PATH}" 2>/dev/null || echo "0")"
  if [[ "${pkg_count}" != "null" && "${pkg_count}" -gt 0 ]]; then
    while IFS= read -r line; do
      CFG_PACKAGES+=("${line}")
    done < <(yq eval '.packages[]' "${CONFIG_PATH}")
  fi

  # Extract mounts (optional array of objects)
  CFG_MOUNT_SOURCES=()
  CFG_MOUNT_TARGETS=()
  local mount_count
  mount_count="$(yq eval '.mounts | length' "${CONFIG_PATH}" 2>/dev/null || echo "0")"
  if [[ "${mount_count}" != "null" && "${mount_count}" -gt 0 ]]; then
    local i
    for ((i = 0; i < mount_count; i++)); do
      local _src _tgt
      _src="$(yq eval ".mounts[${i}].source // \"\"" "${CONFIG_PATH}")"
      if [[ "${_src}" == "null" ]]; then _src=""; fi
      _tgt="$(yq eval ".mounts[${i}].target // \"\"" "${CONFIG_PATH}")"
      if [[ "${_tgt}" == "null" ]]; then _tgt=""; fi
      CFG_MOUNT_SOURCES+=("${_src}")
      CFG_MOUNT_TARGETS+=("${_tgt}")
    done
  fi

  # Extract secrets (optional array)
  CFG_SECRETS=()
  local secret_count
  secret_count="$(yq eval '.secrets | length' "${CONFIG_PATH}" 2>/dev/null || echo "0")"
  if [[ "${secret_count}" != "null" && "${secret_count}" -gt 0 ]]; then
    while IFS= read -r line; do
      CFG_SECRETS+=("${line}")
    done < <(yq eval '.secrets[]' "${CONFIG_PATH}")
  fi

  # Extract env (optional map — single-line values only; multiline YAML values will
  # be split across array entries and misalign keys/values)
  CFG_ENV_KEYS=()
  CFG_ENV_VALUES=()
  local env_count
  env_count="$(yq eval '.env | length' "${CONFIG_PATH}" 2>/dev/null || echo "0")"
  if [[ "${env_count}" != "null" && "${env_count}" -gt 0 ]]; then
    while IFS= read -r line; do
      CFG_ENV_KEYS+=("${line}")
    done < <(yq eval '.env | to_entries | .[].key' "${CONFIG_PATH}")
    while IFS= read -r line; do
      CFG_ENV_VALUES+=("${line}")
    done < <(yq eval '.env | to_entries | .[].value' "${CONFIG_PATH}")
  fi

  # Extract mcp (optional array)
  CFG_MCP=()
  local mcp_count
  mcp_count="$(yq eval '.mcp | length' "${CONFIG_PATH}" 2>/dev/null || echo "0")"
  if [[ "${mcp_count}" != "null" && "${mcp_count}" -gt 0 ]]; then
    while IFS= read -r line; do
      CFG_MCP+=("${line}")
    done < <(yq eval '.mcp[]' "${CONFIG_PATH}")
  fi
}

# ============================================================================
# Build functions (stub)
# ============================================================================

cmd_build() {
  parse_config
  info "not yet implemented"
}

# ============================================================================
# Run functions (stub)
# ============================================================================

cmd_run() {
  parse_config
  info "not yet implemented"
}

# ============================================================================
# Init function (stub)
# ============================================================================

cmd_init() {
  if [[ -f "${CONFIG_PATH}" ]]; then
    die "config already exists" 1
  fi
  mkdir -p "$(dirname "${CONFIG_PATH}")"
  cp "${SCRIPT_DIR}/templates/config.yaml" "${CONFIG_PATH}"
  info "created ${CONFIG_PATH}"
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
  if [[ "${#}" -eq 0 ]]; then
    show_help
    exit 0
  fi

  local command=""

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
        CONFIG_PATH="${2}"
        shift 2
        ;;
      init|build|run)
        if [[ -n "${command}" ]]; then
          die "multiple commands specified: '${command}' and '${1}'" 2
        fi
        command="${1}"
        shift
        ;;
      *)
        die "unknown command '${1}'" 2
        ;;
    esac
  done

  if [[ -z "${command}" ]]; then
    show_help
    exit 0
  fi

  case "${command}" in
    init)  cmd_init ;;
    build) cmd_build ;;
    run)   cmd_run ;;
  esac
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
