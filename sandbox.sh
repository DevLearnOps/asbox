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
RESOLVED_DOCKERFILE=""

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
# Build functions
# ============================================================================

# Escape a string for use as a sed replacement (handles &, \, |)
sed_escape_replacement() {
  printf '%s' "$1" | sed -e 's|[\\&|]|\\&|g'
}

# Ubuntu 24.04 LTS pinned to digest for reproducible builds
readonly BASE_IMAGE="ubuntu:24.04@sha256:186072bba1b2f436cbb91ef2567abca677337cfc786c86e107d25b7072feef0c"

process_template() {
  local template_file="${SCRIPT_DIR}/Dockerfile.template"
  if [[ ! -f "${template_file}" ]]; then
    die "template not found: ${template_file}" 1
  fi

  local template
  template="$(cat "${template_file}")"

  # Step 1: Extract all conditional tag names and validate matching pairs
  local open_tags close_tags tag
  open_tags="$(echo "${template}" | grep -oE '\{\{IF_[A-Z_]+\}\}' | sed 's/[{}]//g' | sort -u || true)"
  close_tags="$(echo "${template}" | grep -oE '\{\{/IF_[A-Z_]+\}\}' | sed 's|[{}/]||g' | sort -u || true)"

  # Check every opening tag has a closing tag
  for tag in ${open_tags}; do
    if ! echo "${close_tags}" | grep -qx "${tag}"; then
      die "unmatched opening tag: {{${tag}}}" 1
    fi
  done

  # Check every closing tag has an opening tag
  for tag in ${close_tags}; do
    if ! echo "${open_tags}" | grep -qx "${tag}"; then
      die "unmatched closing tag: {{/${tag}}}" 1
    fi
  done

  # Step 2: Process conditional blocks
  # IF_NODE
  if [[ -n "${CFG_SDK_NODEJS}" ]]; then
    # Keep content, remove tag lines
    template="$(echo "${template}" | sed '/^# {{IF_NODE}}$/d; /^# {{\/IF_NODE}}$/d')"
  else
    # Strip entire block including tag lines
    template="$(echo "${template}" | sed '/^# {{IF_NODE}}$/,/^# {{\/IF_NODE}}$/d')"
  fi

  # IF_PYTHON
  if [[ -n "${CFG_SDK_PYTHON}" ]]; then
    template="$(echo "${template}" | sed '/^# {{IF_PYTHON}}$/d; /^# {{\/IF_PYTHON}}$/d')"
  else
    template="$(echo "${template}" | sed '/^# {{IF_PYTHON}}$/,/^# {{\/IF_PYTHON}}$/d')"
  fi

  # IF_GO
  if [[ -n "${CFG_SDK_GO}" ]]; then
    template="$(echo "${template}" | sed '/^# {{IF_GO}}$/d; /^# {{\/IF_GO}}$/d')"
  else
    template="$(echo "${template}" | sed '/^# {{IF_GO}}$/,/^# {{\/IF_GO}}$/d')"
  fi

  # Step 3: Substitute value placeholders
  local safe_val
  safe_val="$(sed_escape_replacement "${BASE_IMAGE}")"
  template="$(echo "${template}" | sed "s|{{BASE_IMAGE}}|${safe_val}|g")"

  if [[ -n "${CFG_SDK_NODEJS}" ]]; then
    safe_val="$(sed_escape_replacement "${CFG_SDK_NODEJS}")"
    template="$(echo "${template}" | sed "s|{{NODE_VERSION}}|${safe_val}|g")"
  fi

  if [[ -n "${CFG_SDK_PYTHON}" ]]; then
    safe_val="$(sed_escape_replacement "${CFG_SDK_PYTHON}")"
    template="$(echo "${template}" | sed "s|{{PYTHON_VERSION}}|${safe_val}|g")"
  fi

  if [[ -n "${CFG_SDK_GO}" ]]; then
    safe_val="$(sed_escape_replacement "${CFG_SDK_GO}")"
    template="$(echo "${template}" | sed "s|{{GO_VERSION}}|${safe_val}|g")"
  fi

  # Handle packages: join array with spaces, remove the RUN line if empty
  local packages_str="${CFG_PACKAGES[*]:-}"
  if [[ -n "${packages_str}" ]]; then
    safe_val="$(sed_escape_replacement "${packages_str}")"
    template="$(echo "${template}" | sed "s|{{PACKAGES}}|${safe_val}|g")"
  else
    # Remove the packages RUN block (multi-line with backslash continuation)
    template="$(echo "${template}" | sed '/{{PACKAGES}}/d')"
  fi

  # Step 4: Validate no unresolved placeholders remain
  local unresolved
  unresolved="$(echo "${template}" | grep -oE '\{\{/?[A-Z_]+\}\}' | head -1 || true)"
  if [[ -n "${unresolved}" ]]; then
    die "unresolved placeholder: ${unresolved}" 1
  fi

  RESOLVED_DOCKERFILE="${template}"
}

cmd_build() {
  parse_config
  process_template

  local dockerfile_path
  dockerfile_path="${SCRIPT_DIR}/.sandbox-dockerfile"
  echo "${RESOLVED_DOCKERFILE}" > "${dockerfile_path}"
  info "generated Dockerfile: ${dockerfile_path}"
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
