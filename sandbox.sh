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
CFG_HOST_AGENT_CONFIG_SOURCE=""
CFG_HOST_AGENT_CONFIG_TARGET=""
RESOLVED_DOCKERFILE=""
CONTENT_HASH=""
IMAGE_TAG=""

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
      if [[ "${line}" == "null" ]]; then line=""; fi
      CFG_ENV_VALUES+=("${line}")
    done < <(yq eval '.env | to_entries | .[].value' "${CONFIG_PATH}")
    if [[ ${#CFG_ENV_KEYS[@]} -ne ${#CFG_ENV_VALUES[@]} ]]; then
      die "env keys/values mismatch (multiline values not supported)" 1
    fi
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

  # Extract host_agent_config (optional object with source/target)
  CFG_HOST_AGENT_CONFIG_SOURCE="$(yq eval '.host_agent_config.source // ""' "${CONFIG_PATH}")"
  if [[ "${CFG_HOST_AGENT_CONFIG_SOURCE}" == "null" ]]; then CFG_HOST_AGENT_CONFIG_SOURCE=""; fi
  CFG_HOST_AGENT_CONFIG_TARGET="$(yq eval '.host_agent_config.target // ""' "${CONFIG_PATH}")"
  if [[ "${CFG_HOST_AGENT_CONFIG_TARGET}" == "null" ]]; then CFG_HOST_AGENT_CONFIG_TARGET=""; fi
}

# ============================================================================
# Secret validation
# ============================================================================

# Validate all declared secrets are set in host environment
validate_secrets() {
  local secret_name
  for secret_name in "${CFG_SECRETS[@]}"; do
    # Reject empty or invalid env var names (prevents eval injection)
    if [[ ! "${secret_name}" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
      die "invalid secret name: ${secret_name}" 4
    fi
    # Use ${VAR+x} to check if declared (empty values are valid)
    eval "local _check=\${${secret_name}+x}" # shellcheck disable=eval-indirect
    if [[ -z "${_check}" ]]; then
      die "secret not set: ${secret_name}" 4
    fi
  done
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

compute_content_hash() {
  local hash_cmd
  if command -v sha256sum >/dev/null 2>&1; then
    hash_cmd=(sha256sum)
  elif command -v shasum >/dev/null 2>&1; then
    hash_cmd=(shasum -a 256)
  else
    die "neither sha256sum nor shasum found" 1
  fi

  local hash_input_1="${CONFIG_PATH}"
  local hash_input_2="${SCRIPT_DIR}/Dockerfile.template"
  local hash_input_3="${SCRIPT_DIR}/scripts/entrypoint.sh"
  local hash_input_4="${SCRIPT_DIR}/scripts/git-wrapper.sh"

  for f in "${hash_input_1}" "${hash_input_2}" "${hash_input_3}" "${hash_input_4}"; do
    if [[ ! -f "${f}" ]]; then
      die "hash input file not found: ${f}" 1
    fi
  done

  local hash
  hash="$(cat "${hash_input_1}" "${hash_input_2}" "${hash_input_3}" "${hash_input_4}" \
    | "${hash_cmd[@]}" | cut -c1-12)"
  if [[ -z "${hash}" ]]; then
    die "content hash computation produced empty result" 1
  fi
  CONTENT_HASH="${hash}"
}

compute_image_tag() {
  local project_name="sandbox"
  local config_dir
  config_dir="$(dirname "${CONFIG_PATH}")"
  if [[ "$(basename "${config_dir}")" == ".sandbox" ]]; then
    project_name="$(basename "$(cd "${config_dir}/.." && pwd)" 2>/dev/null)" || project_name="sandbox"
  fi
  if [[ -z "${project_name}" ]]; then
    project_name="sandbox"
  fi
  # Sanitize for Docker tag: lowercase, strip invalid chars
  project_name="$(echo "${project_name}" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9_.-]//g')"
  if [[ -z "${project_name}" ]]; then
    project_name="sandbox"
  fi
  IMAGE_TAG="sandbox-${project_name}:${CONTENT_HASH}"
}

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

  # IF_MCP_PLAYWRIGHT
  local has_mcp_playwright=""
  for _mcp in "${CFG_MCP[@]:-}"; do
    if [[ "${_mcp}" == "playwright" ]]; then has_mcp_playwright="yes"; break; fi
  done
  if [[ -n "${has_mcp_playwright}" ]]; then
    template="$(echo "${template}" | sed '/^# {{IF_MCP_PLAYWRIGHT}}$/d; /^# {{\/IF_MCP_PLAYWRIGHT}}$/d')"
  else
    template="$(echo "${template}" | sed '/^# {{IF_MCP_PLAYWRIGHT}}$/,/^# {{\/IF_MCP_PLAYWRIGHT}}$/d')"
  fi

  # IF_AGENT_CLAUDE
  if [[ "${CFG_AGENT}" == "claude-code" ]]; then
    template="$(echo "${template}" | sed '/^# {{IF_AGENT_CLAUDE}}$/d; /^# {{\/IF_AGENT_CLAUDE}}$/d')"
  else
    template="$(echo "${template}" | sed '/^# {{IF_AGENT_CLAUDE}}$/,/^# {{\/IF_AGENT_CLAUDE}}$/d')"
  fi

  # IF_AGENT_GEMINI
  if [[ "${CFG_AGENT}" == "gemini-cli" ]]; then
    template="$(echo "${template}" | sed '/^# {{IF_AGENT_GEMINI}}$/d; /^# {{\/IF_AGENT_GEMINI}}$/d')"
  else
    template="$(echo "${template}" | sed '/^# {{IF_AGENT_GEMINI}}$/,/^# {{\/IF_AGENT_GEMINI}}$/d')"
  fi

  # Step 3: Substitute value placeholders
  local safe_val

  # Derive Ubuntu version from BASE_IMAGE
  # Handles: "ubuntu:24.04", "ubuntu:24.04@sha256:...", "registry:5000/ubuntu:24.04@sha256:..."
  local ubuntu_version
  ubuntu_version="$(echo "${BASE_IMAGE}" | sed 's/@.*//; s/.*://')"
  if [[ ! "${ubuntu_version}" =~ ^[0-9]+\.[0-9]+$ ]]; then
    die "cannot derive Ubuntu version from BASE_IMAGE '${BASE_IMAGE}' (got '${ubuntu_version}')" 1
  fi
  safe_val="$(sed_escape_replacement "${ubuntu_version}")"
  template="$(echo "${template}" | sed "s|{{UBUNTU_VERSION}}|${safe_val}|g")"

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

  # Generate MCP manifest JSON and substitute MCP version placeholders
  local mcp_manifest='{"mcpServers": {}}'
  if [[ -n "${has_mcp_playwright}" ]]; then
    local mcp_playwright_version="0.0.68"
    mcp_manifest='{"mcpServers": {"playwright": {"type": "stdio", "command": "npx", "args": ["-y", "@playwright/mcp"]}}}'
    safe_val="$(sed_escape_replacement "${mcp_playwright_version}")"
    template="$(echo "${template}" | sed "s|{{MCP_PLAYWRIGHT_VERSION}}|${safe_val}|g")"
  fi
  safe_val="$(sed_escape_replacement "${mcp_manifest}")"
  template="$(echo "${template}" | sed "s|{{MCP_MANIFEST_JSON}}|${safe_val}|g")"

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

  # Validate MCP dependencies
  local _mcp
  local _known_mcp_servers="playwright"
  for _mcp in "${CFG_MCP[@]:-}"; do
    [[ -z "${_mcp}" ]] && continue
    if ! echo "${_known_mcp_servers}" | grep -qw "${_mcp}"; then
      die "unknown mcp server '${_mcp}' (known: ${_known_mcp_servers})" 1
    fi
    if [[ "${_mcp}" == "playwright" && -z "${CFG_SDK_NODEJS}" ]]; then
      die "mcp server 'playwright' requires sdks.nodejs to be configured" 1
    fi
  done

  # Validate agent CLI dependencies
  if [[ -n "${CFG_AGENT}" && "${CFG_AGENT}" != "none" ]]; then
    if [[ "${CFG_AGENT}" == "claude-code" || "${CFG_AGENT}" == "gemini-cli" ]]; then
      if [[ -z "${CFG_SDK_NODEJS}" ]]; then
        die "agent ${CFG_AGENT} requires sdks.nodejs to be configured" 1
      fi
    fi
  fi

  process_template

  local dockerfile_path
  dockerfile_path="${SCRIPT_DIR}/.sandbox-dockerfile"
  echo "${RESOLVED_DOCKERFILE}" > "${dockerfile_path}"
  info "generated Dockerfile: ${dockerfile_path}"

  compute_content_hash
  compute_image_tag

  if docker image inspect "${IMAGE_TAG}" >/dev/null 2>&1; then
    info "image up to date: ${IMAGE_TAG}"
    return 0
  fi

  info "building image: ${IMAGE_TAG}"
  docker build -t "${IMAGE_TAG}" -f "${dockerfile_path}" "${SCRIPT_DIR}"
  info "image built: ${IMAGE_TAG}"
}

# ============================================================================
# Run functions (stub)
# ============================================================================

cmd_run() {
  parse_config
  validate_secrets
  cmd_build

  # Assemble docker run flags using array (safe for paths with spaces)
  local run_flags=()
  run_flags+=("-it" "--rm")
  run_flags+=("--device" "/dev/net/tun")
  run_flags+=("--device" "/dev/fuse")
  run_flags+=("--security-opt" "seccomp=unconfined")
  run_flags+=("--security-opt" "apparmor=unconfined")
  run_flags+=("--security-opt" "label=disable")
  run_flags+=("--cap-add" "SYS_ADMIN")
  run_flags+=("-e" "SANDBOX_AGENT=${CFG_AGENT}")

  # Inject secrets as env vars (Docker reads from host environment)
  local secret_name
  for secret_name in "${CFG_SECRETS[@]}"; do
    run_flags+=("-e" "${secret_name}")
  done

  # Inject non-secret env vars as KEY=VALUE
  local j
  for j in "${!CFG_ENV_KEYS[@]}"; do
    if [[ ! "${CFG_ENV_KEYS[$j]}" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
      die "invalid env var name: ${CFG_ENV_KEYS[$j]}" 4
    fi
    run_flags+=("-e" "${CFG_ENV_KEYS[$j]}=${CFG_ENV_VALUES[$j]}")
  done

  # Resolve mount paths relative to config file directory
  local config_dir
  config_dir="$(cd "$(dirname "${CONFIG_PATH}")" && pwd)"

  local i
  for i in "${!CFG_MOUNT_SOURCES[@]}"; do
    local src="${CFG_MOUNT_SOURCES[$i]}"
    local tgt="${CFG_MOUNT_TARGETS[$i]}"

    # Resolve source paths: tilde expansion, then relative-to-config-dir
    if [[ "${src}" == "~/"* ]]; then
      src="${HOME}/${src#\~/}"
    elif [[ "${src}" != /* ]]; then
      src="$(cd "${config_dir}/${src}" && pwd)"
    fi

    run_flags+=("-v" "${src}:${tgt}")
  done

  # Set working directory to first mount target (if any mounts exist)
  if [[ ${#CFG_MOUNT_TARGETS[@]} -gt 0 ]]; then
    run_flags+=("-w" "${CFG_MOUNT_TARGETS[0]}")
  fi

  # Mount host agent config directory (if configured)
  if [[ -n "${CFG_HOST_AGENT_CONFIG_SOURCE}" || -n "${CFG_HOST_AGENT_CONFIG_TARGET}" ]]; then
    if [[ -z "${CFG_HOST_AGENT_CONFIG_SOURCE}" || -z "${CFG_HOST_AGENT_CONFIG_TARGET}" ]]; then
      die "host_agent_config requires both source and target" 1
    fi
    if [[ "${CFG_HOST_AGENT_CONFIG_TARGET}" != /* ]]; then
      die "host_agent_config.target must be an absolute path" 1
    fi
    local hac_src="${CFG_HOST_AGENT_CONFIG_SOURCE}"
    # Apply tilde expansion
    if [[ "${hac_src}" == "~/"* ]]; then
      hac_src="${HOME}/${hac_src#\~/}"
    elif [[ "${hac_src}" != /* ]]; then
      hac_src="$(cd "${config_dir}/${hac_src}" && pwd)"
    fi
    if [[ ! -d "${hac_src}" ]]; then
      die "host_agent_config source directory not found: ${hac_src}" 1
    fi
    run_flags+=("-v" "${hac_src}:${CFG_HOST_AGENT_CONFIG_TARGET}")
    # Pass host UID/GID for cross-platform permission alignment
    run_flags+=("-e" "HOST_UID=$(id -u)" "-e" "HOST_GID=$(id -g)")
  fi

  info "starting sandbox: ${IMAGE_TAG}"
  docker run "${run_flags[@]}" "${IMAGE_TAG}"
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
