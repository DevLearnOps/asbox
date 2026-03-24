#!/usr/bin/env bash
set -euo pipefail

# ============================================================================
# Test suite for sandbox.sh
# Simple TAP-like test runner — no external dependencies
# ============================================================================

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
SANDBOX="${PROJECT_ROOT}/sandbox.sh"

# Build a minimal system path with only essential binaries (bash, env, core utils)
# We create a temp dir with symlinks to avoid picking up docker/yq from the same dirs
MOCK_SYSBIN="$(mktemp -d)"
for bin in bash env cat echo grep sed awk chmod mkdir rm cp dirname pwd cut basename shasum sha256sum head tail sort uniq wc tr tee mktemp ln printf test true false touch date read; do
  real_path="$(command -v "${bin}" 2>/dev/null || true)"
  if [[ -n "${real_path}" ]]; then
    ln -sf "${real_path}" "${MOCK_SYSBIN}/${bin}"
  fi
done
SYSTEM_PATH="${MOCK_SYSBIN}"

cleanup_sysbin() { rm -rf "${MOCK_SYSBIN}"; }
trap cleanup_sysbin EXIT

TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

pass() {
  TESTS_RUN=$((TESTS_RUN + 1))
  TESTS_PASSED=$((TESTS_PASSED + 1))
  echo "ok ${TESTS_RUN} - ${1}"
}

fail() {
  TESTS_RUN=$((TESTS_RUN + 1))
  TESTS_FAILED=$((TESTS_FAILED + 1))
  echo "not ok ${TESTS_RUN} - ${1}"
  if [[ -n "${2:-}" ]]; then
    echo "  # ${2}"
  fi
}

assert_exit_code() {
  local expected="${1}"
  local actual="${2}"
  local name="${3}"
  if [[ "${actual}" -eq "${expected}" ]]; then
    pass "${name}"
  else
    fail "${name}" "expected exit ${expected}, got ${actual}"
  fi
}

assert_contains() {
  local haystack="${1}"
  local needle="${2}"
  local name="${3}"
  if [[ "${haystack}" == *"${needle}"* ]]; then
    pass "${name}"
  else
    fail "${name}" "output did not contain: ${needle}"
  fi
}

assert_not_contains() {
  local haystack="${1}"
  local needle="${2}"
  local name="${3}"
  if [[ "${haystack}" == *"${needle}"* ]]; then
    fail "${name}" "output unexpectedly contained: ${needle}"
  else
    pass "${name}"
  fi
}

# Helper: create a mock docker binary for build tests
# Usage: setup_build_mock <tmpdir> [inspect_exit_code]
# Sets MOCK_DOCKER_LOG and MOCK_DOCKER_DIR
setup_build_mock() {
  local target_dir="${1}"
  local inspect_exit="${2:-1}"
  MOCK_DOCKER_DIR="${target_dir}"
  MOCK_DOCKER_LOG="${target_dir}/docker.log"
  cat > "${target_dir}/docker" <<MOCK
#!/usr/bin/env bash
echo "docker \$*" >> "${MOCK_DOCKER_LOG}"
if [[ "\$1" == "image" && "\$2" == "inspect" ]]; then
  exit ${inspect_exit}
fi
exit 0
MOCK
  chmod +x "${target_dir}/docker"
}

# ============================================================================
# AC 1: Help output
# ============================================================================

echo "# AC 1: Help output"

output="$(bash "${SANDBOX}" --help 2>&1)"
exit_code=$?
assert_exit_code 0 "${exit_code}" "--help exits with code 0"
assert_contains "${output}" "Usage: sandbox <command> [options]" "--help shows usage line"
assert_contains "${output}" "init" "--help shows init command"
assert_contains "${output}" "build" "--help shows build command"
assert_contains "${output}" "run" "--help shows run command"
assert_contains "${output}" "-f <path>" "--help shows -f flag"
assert_contains "${output}" "--help" "--help shows --help flag"

# No args should also show help
output="$(bash "${SANDBOX}" 2>&1)"
exit_code=$?
assert_exit_code 0 "${exit_code}" "no args exits with code 0"
assert_contains "${output}" "Usage: sandbox" "no args shows usage"

# ============================================================================
# AC 2: Docker not found
# ============================================================================

echo "# AC 2: Docker not found"

# Create a mock environment where docker is not on PATH
tmpbin="$(mktemp -d)"
# Create fake yq that reports v4
cat > "${tmpbin}/yq" <<'MOCK'
#!/usr/bin/env bash
if [[ "${1:-}" == "--version" ]]; then
  echo "yq (https://github.com/mikefarah/yq/) version v4.44.1"
else
  exit 0
fi
MOCK
chmod +x "${tmpbin}/yq"

stderr_output="$(PATH="${tmpbin}:${SYSTEM_PATH}" bash "${SANDBOX}" init 2>&1 1>/dev/null || true)"
exit_code_output="$(PATH="${tmpbin}:${SYSTEM_PATH}" bash "${SANDBOX}" init 2>/dev/null; echo "EXIT:$?" )"
# The above won't work cleanly since exit terminates. Let's do it properly:
set +e
output_all="$(PATH="${tmpbin}:${SYSTEM_PATH}" bash "${SANDBOX}" init 2>&1)"
exit_code=$?
set -e

assert_exit_code 3 "${exit_code}" "missing docker exits with code 3"
assert_contains "${output_all}" "error: docker not found" "missing docker prints error message"

rm -rf "${tmpbin}"

# ============================================================================
# AC 3: yq not found / below v4
# ============================================================================

echo "# AC 3: yq not found or below v4"

# Case 1: yq not found at all
tmpbin="$(mktemp -d)"
# Create fake docker
cat > "${tmpbin}/docker" <<'MOCK'
#!/usr/bin/env bash
exit 0
MOCK
chmod +x "${tmpbin}/docker"

set +e
output_all="$(PATH="${tmpbin}:${SYSTEM_PATH}" bash "${SANDBOX}" init 2>&1)"
exit_code=$?
set -e

assert_exit_code 3 "${exit_code}" "missing yq exits with code 3"
assert_contains "${output_all}" "error: yq not found" "missing yq prints error message"

# Case 2: yq present but v3
cat > "${tmpbin}/yq" <<'MOCK'
#!/usr/bin/env bash
if [[ "${1:-}" == "--version" ]]; then
  echo "yq version 3.4.1"
else
  exit 0
fi
MOCK
chmod +x "${tmpbin}/yq"

set +e
output_all="$(PATH="${tmpbin}:${SYSTEM_PATH}" bash "${SANDBOX}" init 2>&1)"
exit_code=$?
set -e

assert_exit_code 3 "${exit_code}" "yq v3 exits with code 3"
assert_contains "${output_all}" "yq version 3.x detected" "yq v3 prints version error"

rm -rf "${tmpbin}"

# ============================================================================
# AC 4: Bash version check
# ============================================================================

echo "# AC 4: Bash version check"

# We can't easily test bash version < 4 from bash 4+, but we can verify
# check_bash_version function exists and passes on our current bash
set +e
output_all="$(bash "${SANDBOX}" --help 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "bash version check passes on current bash (${BASH_VERSINFO[0]}.${BASH_VERSINFO[1]})"

# Verify the check_bash_version function exists in the script
if grep -q 'check_bash_version' "${SANDBOX}"; then
  pass "check_bash_version function exists in sandbox.sh"
else
  fail "check_bash_version function exists in sandbox.sh"
fi

if grep -q 'BASH_VERSINFO\[0\]' "${SANDBOX}"; then
  pass "bash version check uses BASH_VERSINFO[0]"
else
  fail "bash version check uses BASH_VERSINFO[0]"
fi

# ============================================================================
# Additional: Command stubs and error handling
# ============================================================================

echo "# Additional: command stubs and error handling"

# run command requires config (no stub anymore)
set +e
output_all="$(bash "${SANDBOX}" run 2>&1)"
exit_code=$?
set -e
assert_exit_code 1 "${exit_code}" "'run' without config exits with code 1"
assert_contains "${output_all}" "config not found" "'run' without config prints config error"

# Unknown command
set +e
output_all="$(bash "${SANDBOX}" foobar 2>&1)"
exit_code=$?
set -e
assert_exit_code 2 "${exit_code}" "unknown command exits with code 2"
assert_contains "${output_all}" "error: unknown command 'foobar'" "unknown command prints error"

# -f without argument
set +e
output_all="$(bash "${SANDBOX}" -f 2>&1)"
exit_code=$?
set -e
assert_exit_code 2 "${exit_code}" "-f without arg exits with code 2"

# ============================================================================
# Additional: File structure validation
# ============================================================================

echo "# Additional: project structure"

for f in sandbox.sh scripts/entrypoint.sh scripts/git-wrapper.sh templates/config.yaml Dockerfile.template; do
  if [[ -f "${PROJECT_ROOT}/${f}" ]]; then
    pass "file exists: ${f}"
  else
    fail "file exists: ${f}"
  fi
done

if [[ -x "${PROJECT_ROOT}/sandbox.sh" ]]; then
  pass "sandbox.sh is executable"
else
  fail "sandbox.sh is executable"
fi

# ============================================================================
# Additional: Output goes to correct streams
# ============================================================================

echo "# Additional: output stream correctness"

# Help goes to stdout
stdout_output="$(bash "${SANDBOX}" --help 2>/dev/null)"
stderr_output="$(bash "${SANDBOX}" --help 2>&1 1>/dev/null)"
assert_contains "${stdout_output}" "Usage:" "help output goes to stdout"
assert_not_contains "${stderr_output}" "Usage:" "help output does NOT go to stderr"

# Error goes to stderr
tmpbin="$(mktemp -d)"
cat > "${tmpbin}/yq" <<'MOCK'
#!/usr/bin/env bash
echo "yq (https://github.com/mikefarah/yq/) version v4.44.1"
MOCK
chmod +x "${tmpbin}/yq"

set +e
stdout_output="$(PATH="${tmpbin}:${SYSTEM_PATH}" bash "${SANDBOX}" init 2>/dev/null)"
stderr_output="$(PATH="${tmpbin}:${SYSTEM_PATH}" bash "${SANDBOX}" init 2>&1 1>/dev/null)"
set -e
assert_contains "${stderr_output}" "error:" "error output goes to stderr"
assert_not_contains "${stdout_output}" "error:" "error output does NOT go to stdout"
rm -rf "${tmpbin}"

# ============================================================================
# Additional: Script quality checks
# ============================================================================

echo "# Additional: script quality checks"

if head -1 "${SANDBOX}" | grep -q '#!/usr/bin/env bash'; then
  pass "shebang is #!/usr/bin/env bash"
else
  fail "shebang is #!/usr/bin/env bash"
fi

if grep -q 'set -euo pipefail' "${SANDBOX}"; then
  pass "set -euo pipefail is present"
else
  fail "set -euo pipefail is present"
fi

# Find lines that start with eval (command position), excluding yq eval and marked exceptions
eval_lines="$(grep -n '^[[:space:]]*eval ' "${SANDBOX}" | grep -v '# shellcheck disable=eval-indirect' || true)"
if [[ -n "${eval_lines}" ]]; then
  fail "no eval usage (except marked indirect var check)" "found unexpected bash eval: ${eval_lines}"
else
  pass "no eval usage (except marked indirect var check)"
fi

# ============================================================================
# AC: sandbox init
# ============================================================================

echo "# AC: sandbox init"

# Test: sandbox init creates .sandbox/config.yaml with expected content
tmpdir="$(mktemp -d)"
set +e
output_all="$(cd "${tmpdir}" && bash "${SANDBOX}" init 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "sandbox init exits with code 0"
assert_contains "${output_all}" "created .sandbox/config.yaml" "sandbox init prints success message"
if [[ -f "${tmpdir}/.sandbox/config.yaml" ]]; then
  pass "sandbox init creates .sandbox/config.yaml"
else
  fail "sandbox init creates .sandbox/config.yaml"
fi
rm -rf "${tmpdir}"

# Test: sandbox init when config exists exits code 1 with error message
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/.sandbox"
touch "${tmpdir}/.sandbox/config.yaml"
set +e
output_all="$(cd "${tmpdir}" && bash "${SANDBOX}" init 2>&1)"
exit_code=$?
set -e
assert_exit_code 1 "${exit_code}" "sandbox init with existing config exits code 1"
assert_contains "${output_all}" "error: config already exists" "sandbox init with existing config prints error"
rm -rf "${tmpdir}"

# Test: sandbox init -f custom/path.yaml creates at custom path
tmpdir="$(mktemp -d)"
set +e
output_all="$(cd "${tmpdir}" && bash "${SANDBOX}" init -f custom/path.yaml 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "sandbox init -f custom/path.yaml exits with code 0"
if [[ -f "${tmpdir}/custom/path.yaml" ]]; then
  pass "sandbox init -f creates config at custom path"
else
  fail "sandbox init -f creates config at custom path"
fi
rm -rf "${tmpdir}"

# Test: sandbox -f custom/path.yaml init also works (flag before command)
tmpdir="$(mktemp -d)"
set +e
output_all="$(cd "${tmpdir}" && bash "${SANDBOX}" -f custom/before.yaml init 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "sandbox -f path init exits with code 0"
if [[ -f "${tmpdir}/custom/before.yaml" ]]; then
  pass "sandbox -f before command creates config at custom path"
else
  fail "sandbox -f before command creates config at custom path"
fi
rm -rf "${tmpdir}"

# Test: generated config is valid YAML (parseable by yq)
tmpdir="$(mktemp -d)"
output_all="$(cd "${tmpdir}" && bash "${SANDBOX}" init 2>&1)"
set +e
yq_output="$(yq eval '.' "${tmpdir}/.sandbox/config.yaml" 2>&1)"
yq_exit=$?
set -e
assert_exit_code 0 "${yq_exit}" "generated config is valid YAML (yq parses it)"
rm -rf "${tmpdir}"

# Test: generated config contains expected defaults (agent, sdks, packages)
tmpdir="$(mktemp -d)"
output_all="$(cd "${tmpdir}" && bash "${SANDBOX}" init 2>&1)"
config_content="$(cat "${tmpdir}/.sandbox/config.yaml")"
assert_contains "${config_content}" "agent: claude-code" "generated config has agent: claude-code"
assert_contains "${config_content}" "nodejs:" "generated config has nodejs SDK"
assert_contains "${config_content}" "build-essential" "generated config has build-essential package"
assert_contains "${config_content}" "curl" "generated config has curl package"
assert_contains "${config_content}" "wget" "generated config has wget package"
assert_contains "${config_content}" "git" "generated config has git package"
assert_contains "${config_content}" "jq" "generated config has jq package"
rm -rf "${tmpdir}"

# ============================================================================
# Setup: mock docker for all build tests
# ============================================================================

BUILD_MOCK_DIR="$(mktemp -d)"
setup_build_mock "${BUILD_MOCK_DIR}" 1
BUILD_PATH="${BUILD_MOCK_DIR}:${PATH}"
cleanup_build_mock() { rm -rf "${BUILD_MOCK_DIR}"; cleanup_sysbin; }
trap cleanup_build_mock EXIT

# ============================================================================
# AC: parse_config — valid config extraction
# ============================================================================

echo "# AC: parse_config — valid config extraction"

# Test: parse_config extracts agent correctly
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "parse_config extracts agent correctly (exit 0)"
assert_contains "${output_all}" "image built: sandbox-" "parse_config succeeds then build completes"
rm -rf "${tmpdir}"

# Test: parse_config extracts agent gemini-cli
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: gemini-cli
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "parse_config accepts gemini-cli agent"
rm -rf "${tmpdir}"

# Test: parse_config extracts SDK versions
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
sdks:
  nodejs: "22"
  python: "3.12"
  go: "1.22"
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "parse_config extracts SDK versions correctly"
rm -rf "${tmpdir}"

# Test: parse_config extracts packages list
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
packages:
  - curl
  - wget
  - git
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "parse_config extracts packages list correctly"
rm -rf "${tmpdir}"

# Test: parse_config extracts mounts (source/target pairs)
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
mounts:
  - source: "."
    target: "/workspace"
  - source: "./data"
    target: "/data"
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "parse_config extracts mounts correctly"
rm -rf "${tmpdir}"

# Test: parse_config extracts secrets list
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
secrets:
  - ANTHROPIC_API_KEY
  - GITHUB_TOKEN
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "parse_config extracts secrets list correctly"
rm -rf "${tmpdir}"

# Test: parse_config extracts env key/value pairs
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
env:
  NODE_ENV: development
  DEBUG: "true"
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "parse_config extracts env key/value pairs correctly"
rm -rf "${tmpdir}"

# Test: parse_config extracts MCP server list
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
mcp:
  - playwright
  - filesystem
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "parse_config extracts MCP server list correctly"
rm -rf "${tmpdir}"

# Test: parse_config handles optional/missing sections gracefully
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "parse_config handles missing optional sections (exit 0)"
rm -rf "${tmpdir}"

# ============================================================================
# AC: parse_config — error cases
# ============================================================================

echo "# AC: parse_config — error cases"

# Test: missing config file exits code 1
tmpdir="$(mktemp -d)"
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/nonexistent.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 1 "${exit_code}" "missing config file exits code 1"
assert_contains "${output_all}" "config not found" "missing config prints 'config not found' error"
rm -rf "${tmpdir}"

# Test: missing agent field exits code 1
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
sdks:
  nodejs: "22"
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 1 "${exit_code}" "missing agent field exits code 1"
assert_contains "${output_all}" "config missing required field: agent" "missing agent prints clear error"
rm -rf "${tmpdir}"

# Test: invalid agent value exits code 1
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: invalid-agent
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 1 "${exit_code}" "invalid agent value exits code 1"
assert_contains "${output_all}" "config invalid agent: invalid-agent" "invalid agent prints clear error"
rm -rf "${tmpdir}"

# ============================================================================
# AC: parse_config — custom path via -f
# ============================================================================

echo "# AC: parse_config — custom path"

# Test: parse_config works with -f custom path
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/custom"
cat > "${tmpdir}/custom/my-config.yaml" <<'YAML'
agent: claude-code
packages:
  - vim
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/custom/my-config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "parse_config works with -f custom path"
rm -rf "${tmpdir}"

# Test: parse_config works with -f flag before command
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: gemini-cli
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" -f "${tmpdir}/config.yaml" run 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "parse_config works with -f before command (run)"
rm -rf "${tmpdir}"

# ============================================================================
# AC: parse_config — starter config from sandbox init
# ============================================================================

echo "# AC: parse_config — starter config compatibility"

# Test: parse_config works with the default starter config from sandbox init
tmpdir="$(mktemp -d)"
# First create the config via init
output_all="$(cd "${tmpdir}" && bash "${SANDBOX}" init 2>&1)"
# Then verify build can parse it
set +e
output_all="$(cd "${tmpdir}" && PATH="${BUILD_PATH}" bash "${SANDBOX}" build 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "parse_config works with default starter config from sandbox init"
assert_contains "${output_all}" "image built: sandbox-" "build continues after parsing starter config"
rm -rf "${tmpdir}"

# Test: run also works with starter config (parses config and triggers build+run)
tmpdir="$(mktemp -d)"
output_all="$(cd "${tmpdir}" && bash "${SANDBOX}" init 2>&1)"
set +e
output_all="$(cd "${tmpdir}" && PATH="${BUILD_PATH}" bash "${SANDBOX}" run 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "parse_config works with starter config via run command"
assert_contains "${output_all}" "starting sandbox:" "run prints starting sandbox message"
rm -rf "${tmpdir}"

# ============================================================================
# AC: parse_config — full config with all sections
# ============================================================================

echo "# AC: parse_config — full config"

# Test: parse_config handles a complete config with all sections populated
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
sdks:
  nodejs: "22"
  python: "3.12"
  go: "1.22"
packages:
  - build-essential
  - curl
  - wget
mounts:
  - source: "."
    target: "/workspace"
secrets:
  - ANTHROPIC_API_KEY
env:
  NODE_ENV: development
mcp:
  - playwright
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "parse_config handles full config with all sections"
rm -rf "${tmpdir}"

# ============================================================================
# AC: process_template — Node.js + Python enabled, Go stripped
# ============================================================================

echo "# AC: process_template — Node.js + Python enabled, Go stripped"

rm -f "${PROJECT_ROOT}/.sandbox-dockerfile"
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
sdks:
  nodejs: "22"
  python: "3.12"
packages:
  - build-essential
  - curl
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "template with Node.js+Python exits code 0"

dockerfile_content="$(cat "${PROJECT_ROOT}/.sandbox-dockerfile")"
assert_contains "${dockerfile_content}" "setup_22.x" "resolved Dockerfile contains Node.js 22 install"
assert_contains "${dockerfile_content}" "python3.12" "resolved Dockerfile contains Python 3.12 install"
assert_not_contains "${dockerfile_content}" "IF_NODE" "resolved Dockerfile has no IF_NODE tags"
assert_not_contains "${dockerfile_content}" "IF_PYTHON" "resolved Dockerfile has no IF_PYTHON tags"
assert_not_contains "${dockerfile_content}" "IF_GO" "resolved Dockerfile has no IF_GO block"
assert_not_contains "${dockerfile_content}" "go.dev" "resolved Dockerfile has no Go install"
rm -rf "${tmpdir}"

# ============================================================================
# AC: process_template — no SDKs strips all conditional blocks
# ============================================================================

echo "# AC: process_template — no SDKs strips all conditional blocks"

rm -f "${PROJECT_ROOT}/.sandbox-dockerfile"
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "template with no SDKs exits code 0"

dockerfile_content="$(cat "${PROJECT_ROOT}/.sandbox-dockerfile")"
assert_not_contains "${dockerfile_content}" "nodejs" "no SDKs: no Node.js install"
assert_not_contains "${dockerfile_content}" "python" "no SDKs: no Python install"
assert_not_contains "${dockerfile_content}" "go.dev" "no SDKs: no Go install"
assert_not_contains "${dockerfile_content}" "IF_" "no SDKs: no conditional tags remain"
assert_contains "${dockerfile_content}" "tini" "no SDKs: base tooling (tini) remains"
assert_contains "${dockerfile_content}" "ENTRYPOINT" "no SDKs: ENTRYPOINT remains"
rm -rf "${tmpdir}"

# ============================================================================
# AC: process_template — all three SDKs enabled
# ============================================================================

echo "# AC: process_template — all three SDKs enabled"

rm -f "${PROJECT_ROOT}/.sandbox-dockerfile"
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
sdks:
  nodejs: "20"
  python: "3.11"
  go: "1.22"
packages:
  - git
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "template with all SDKs exits code 0"

dockerfile_content="$(cat "${PROJECT_ROOT}/.sandbox-dockerfile")"
assert_contains "${dockerfile_content}" "setup_20.x" "all SDKs: Node.js 20 install present"
assert_contains "${dockerfile_content}" "python3.11" "all SDKs: Python 3.11 install present"
assert_contains "${dockerfile_content}" "go1.22.linux-amd64" "all SDKs: Go 1.22 install present"
assert_not_contains "${dockerfile_content}" "IF_" "all SDKs: no conditional tags remain"
rm -rf "${tmpdir}"

# ============================================================================
# AC: process_template — packages substitution
# ============================================================================

echo "# AC: process_template — packages substitution"

rm -f "${PROJECT_ROOT}/.sandbox-dockerfile"
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
packages:
  - build-essential
  - curl
  - wget
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "packages substitution exits code 0"

dockerfile_content="$(cat "${PROJECT_ROOT}/.sandbox-dockerfile")"
assert_contains "${dockerfile_content}" "build-essential curl wget" "packages list substituted into apt-get install"
rm -rf "${tmpdir}"

# ============================================================================
# AC: process_template — empty packages produces valid Dockerfile
# ============================================================================

echo "# AC: process_template — empty packages"

rm -f "${PROJECT_ROOT}/.sandbox-dockerfile"
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "empty packages exits code 0"

dockerfile_content="$(cat "${PROJECT_ROOT}/.sandbox-dockerfile")"
assert_not_contains "${dockerfile_content}" "{{PACKAGES}}" "empty packages: no PACKAGES placeholder remains"
assert_not_contains "${dockerfile_content}" "{{" "empty packages: no unresolved placeholders"
rm -rf "${tmpdir}"

# ============================================================================
# AC: process_template — mismatched tags error
# ============================================================================

echo "# AC: process_template — mismatched tags error"

tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
# Create a broken template with unmatched opening tag
orig_template="${PROJECT_ROOT}/Dockerfile.template"
cp "${orig_template}" "${orig_template}.bak"
restore_template() { cp "${orig_template}.bak" "${orig_template}"; rm -f "${orig_template}.bak"; }
trap restore_template EXIT
cat > "${orig_template}" <<'TMPL'
FROM {{BASE_IMAGE}}
# {{IF_NODE}}
RUN echo "node"
TMPL
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
restore_template
trap - EXIT

assert_exit_code 1 "${exit_code}" "mismatched tag exits code 1"
assert_contains "${output_all}" "unmatched" "mismatched tag prints unmatched error"
assert_contains "${output_all}" "IF_NODE" "mismatched tag names the unmatched tag"
rm -rf "${tmpdir}"

# ============================================================================
# AC: process_template — unresolved placeholder error
# ============================================================================

echo "# AC: process_template — unresolved placeholder error"

tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
# Create a template with an unknown placeholder
orig_template="${PROJECT_ROOT}/Dockerfile.template"
cp "${orig_template}" "${orig_template}.bak"
restore_template() { cp "${orig_template}.bak" "${orig_template}"; rm -f "${orig_template}.bak"; }
trap restore_template EXIT
cat > "${orig_template}" <<'TMPL'
FROM {{BASE_IMAGE}}
RUN echo {{UNKNOWN_THING}}
TMPL
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
restore_template
trap - EXIT

assert_exit_code 1 "${exit_code}" "unresolved placeholder exits code 1"
assert_contains "${output_all}" "unresolved placeholder" "unresolved placeholder prints error message"
assert_contains "${output_all}" "UNKNOWN_THING" "unresolved placeholder names the placeholder"
rm -rf "${tmpdir}"

# ============================================================================
# AC: process_template — resolved Dockerfile contains no {{ markers
# ============================================================================

echo "# AC: process_template — no {{ markers in resolved Dockerfile"

rm -f "${PROJECT_ROOT}/.sandbox-dockerfile"
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
sdks:
  nodejs: "22"
  python: "3.12"
  go: "1.22"
packages:
  - build-essential
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "full config for marker check exits code 0"

dockerfile_content="$(cat "${PROJECT_ROOT}/.sandbox-dockerfile")"
if echo "${dockerfile_content}" | grep -q '{{'; then
  fail "resolved Dockerfile contains no {{ markers" "found {{ in resolved Dockerfile"
else
  pass "resolved Dockerfile contains no {{ markers"
fi
rm -rf "${tmpdir}"

# ============================================================================
# AC: process_template — end-to-end via sandbox build -f
# ============================================================================

echo "# AC: process_template — end-to-end via sandbox build -f"

rm -f "${PROJECT_ROOT}/.sandbox-dockerfile"
tmpdir="$(mktemp -d)"
# Use sandbox init to get a starter config, then build with it
output_all="$(cd "${tmpdir}" && bash "${SANDBOX}" init 2>&1)"
set +e
output_all="$(cd "${tmpdir}" && PATH="${BUILD_PATH}" bash "${SANDBOX}" build 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "end-to-end build with starter config exits code 0"
assert_contains "${output_all}" "generated Dockerfile" "end-to-end build prints generated Dockerfile message"
assert_contains "${output_all}" "image built: sandbox-" "end-to-end build completes with image built message"
if [[ -f "${PROJECT_ROOT}/.sandbox-dockerfile" ]]; then
  pass "end-to-end build creates .sandbox-dockerfile"
else
  fail "end-to-end build creates .sandbox-dockerfile"
fi
rm -rf "${tmpdir}"

# ============================================================================
# AC: compute_content_hash — deterministic and sensitive to changes
# ============================================================================

echo "# AC: compute_content_hash — deterministic and sensitive to changes"

# Test: compute_content_hash produces consistent hash (via sandbox build output)
# Run build twice with identical files — both should produce the same image tag
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output1="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit1=$?
output2="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit2=$?
set -e
assert_exit_code 0 "${exit1}" "first build for hash consistency exits 0"
assert_exit_code 0 "${exit2}" "second build for hash consistency exits 0"
tag1="$(echo "${output1}" | grep "image built:" | sed 's/.*image built: //')"
# Second run hits "image up to date" since mock inspect now finds it
# But our global mock always returns exit 1 for inspect, so both runs build
tag2="$(echo "${output2}" | grep "image built:" | sed 's/.*image built: //')"
if [[ "${tag1}" == "${tag2}" && -n "${tag1}" ]]; then
  pass "compute_content_hash produces consistent hash for same files"
else
  fail "compute_content_hash produces consistent hash for same files" "tag1=${tag1} tag2=${tag2}"
fi

# Verify hash part is exactly 12 chars
hash_part="$(echo "${tag1}" | sed 's/.*://')"
if [[ "${#hash_part}" -eq 12 ]]; then
  pass "content hash is exactly 12 characters"
else
  fail "content hash is exactly 12 characters" "length=${#hash_part}"
fi
rm -rf "${tmpdir}"

# Test: compute_content_hash produces different hash when config changes
# Build with one config, modify it, build again — tags should differ
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output1="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
set -e
tag1="$(echo "${output1}" | grep "image built:" | sed 's/.*image built: //')"
# Now modify the config
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: gemini-cli
YAML
set +e
output2="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
set -e
tag2="$(echo "${output2}" | grep "image built:" | sed 's/.*image built: //')"
if [[ "${tag1}" != "${tag2}" && -n "${tag1}" && -n "${tag2}" ]]; then
  pass "compute_content_hash produces different hash when config changes"
else
  fail "compute_content_hash produces different hash when config changes" "tag1=${tag1} tag2=${tag2}"
fi
rm -rf "${tmpdir}"

# Test: compute_content_hash dies if a hash input file is missing
# Copy sandbox.sh to a temp dir that lacks scripts/git-wrapper.sh
tmpdir="$(mktemp -d)"
cp "${SANDBOX}" "${tmpdir}/sandbox.sh"
chmod +x "${tmpdir}/sandbox.sh"
cp "${PROJECT_ROOT}/Dockerfile.template" "${tmpdir}/"
mkdir -p "${tmpdir}/scripts" "${tmpdir}/templates"
cp "${PROJECT_ROOT}/scripts/entrypoint.sh" "${tmpdir}/scripts/"
cp "${PROJECT_ROOT}/templates/config.yaml" "${tmpdir}/templates/"
# Deliberately do NOT copy scripts/git-wrapper.sh
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${tmpdir}/sandbox.sh" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 1 "${exit_code}" "compute_content_hash dies if hash input file missing"
assert_contains "${output_all}" "hash input file not found" "missing hash input prints error"
rm -rf "${tmpdir}"

# Test: content hash includes all four specified files and no others
# Verify by building, changing each file, and checking that the tag changes
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_base="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
set -e
base_tag="$(echo "${output_base}" | grep "image built:" | sed 's/.*image built: //')"

for change_file in Dockerfile.template scripts/entrypoint.sh scripts/git-wrapper.sh; do
  target="${PROJECT_ROOT}/${change_file}"
  orig="$(cat "${target}")"
  echo "# review-test-modification" >> "${target}"
  set +e
  output_mod="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
  set -e
  echo "${orig}" > "${target}"
  mod_tag="$(echo "${output_mod}" | grep "image built:" | sed 's/.*image built: //')"
  if [[ "${base_tag}" != "${mod_tag}" && -n "${mod_tag}" ]]; then
    pass "content hash changes when ${change_file} changes"
  else
    fail "content hash changes when ${change_file} changes" "base=${base_tag} mod=${mod_tag}"
  fi
done

rm -rf "${tmpdir}"

# ============================================================================
# AC: compute_image_tag — correct format
# ============================================================================

echo "# AC: compute_image_tag — correct format"

# Test: compute_image_tag produces correct format sandbox-<project>:<hash>
# When config is at /some/path/myproject/.sandbox/config.yaml, project = myproject
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/myproject/.sandbox"
cat > "${tmpdir}/myproject/.sandbox/config.yaml" <<'YAML'
agent: claude-code
YAML

set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/myproject/.sandbox/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "build with .sandbox/ config exits code 0"

# Extract image tag from output
if echo "${output_all}" | grep -q "image built: sandbox-myproject:"; then
  pass "compute_image_tag produces sandbox-<project>:<hash> format"
else
  fail "compute_image_tag produces sandbox-<project>:<hash> format" "output: ${output_all}"
fi

# Verify hash part is 12 chars
tag_line="$(echo "${output_all}" | grep "image built:" | head -1)"
hash_part="$(echo "${tag_line}" | sed 's/.*://')"
if [[ "${#hash_part}" -eq 12 ]]; then
  pass "image tag hash part is 12 characters"
else
  fail "image tag hash part is 12 characters" "hash_part=${hash_part} length=${#hash_part}"
fi

rm -rf "${tmpdir}"

# Test: compute_image_tag falls back to "sandbox" for non-standard config path
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(PATH="${BUILD_PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "build with flat config path exits code 0"
if echo "${output_all}" | grep -q "image built: sandbox-sandbox:"; then
  pass "compute_image_tag falls back to 'sandbox' for non-standard config path"
else
  fail "compute_image_tag falls back to 'sandbox' for non-standard config path" "output: ${output_all}"
fi
rm -rf "${tmpdir}"

# ============================================================================
# AC: cmd_build — skips build when image exists
# ============================================================================

echo "# AC: cmd_build — skips build when image exists"

# Test: cmd_build skips build when image already exists (mock docker image inspect to succeed)
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
# Mock docker where inspect succeeds (image exists)
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "build with existing image exits code 0"
assert_contains "${output_all}" "image up to date:" "build skips when image exists"
assert_not_contains "${output_all}" "building image:" "build does NOT start docker build when image exists"

# Verify docker build was NOT called
if grep -q "docker build" "${tmpdir}/mockbin/docker.log" 2>/dev/null; then
  fail "docker build is not called when image exists"
else
  pass "docker build is not called when image exists"
fi

rm -rf "${tmpdir}"

# ============================================================================
# AC: cmd_build — calls docker build when image does not exist
# ============================================================================

echo "# AC: cmd_build — calls docker build when image does not exist"

# Test: cmd_build calls docker build when image does not exist (mock docker image inspect to fail)
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
# Mock docker where inspect fails (image does not exist)
setup_build_mock "${tmpdir}/mockbin" 1
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "build without existing image exits code 0"
assert_contains "${output_all}" "building image:" "build prints building message"
assert_contains "${output_all}" "image built:" "build prints image built message"

# Verify docker build WAS called with correct args
if grep -q "docker build" "${tmpdir}/mockbin/docker.log" 2>/dev/null; then
  pass "docker build is called when image does not exist"
else
  fail "docker build is called when image does not exist"
fi

# Verify docker build args contain the tag and dockerfile
docker_build_line="$(grep "docker build" "${tmpdir}/mockbin/docker.log")"
assert_contains "${docker_build_line}" "-t sandbox-" "docker build includes -t with sandbox- prefix tag"
assert_contains "${docker_build_line}" ".sandbox-dockerfile" "docker build references .sandbox-dockerfile"

rm -rf "${tmpdir}"

# BUILD_MOCK_DIR cleanup is handled by the EXIT trap

# ============================================================================
# AC: cmd_run — calls docker run with correct flags
# ============================================================================

echo "# AC: cmd_run — calls docker run with correct flags"

# Test: sandbox run calls docker run with -it and --rm flags
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "sandbox run exits code 0"
assert_contains "${output_all}" "starting sandbox:" "sandbox run prints starting message"

docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "-it" "docker run includes -it flag"
assert_contains "${docker_run_line}" "--rm" "docker run includes --rm flag"
rm -rf "${tmpdir}"

# Test: sandbox run passes SANDBOX_AGENT env var to container
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
set -e
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "SANDBOX_AGENT=claude-code" "docker run passes SANDBOX_AGENT=claude-code"
rm -rf "${tmpdir}"

# Test: sandbox run passes SANDBOX_AGENT=gemini-cli for gemini config
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: gemini-cli
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
set -e
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "SANDBOX_AGENT=gemini-cli" "docker run passes SANDBOX_AGENT=gemini-cli"
rm -rf "${tmpdir}"

# Test: sandbox run with no existing image triggers build first, then run
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 1
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "sandbox run with no image exits code 0"
assert_contains "${output_all}" "building image:" "sandbox run triggers build when no image"
assert_contains "${output_all}" "starting sandbox:" "sandbox run starts after build"

# Verify both docker build and docker run were called
if grep -q "docker build" "${tmpdir}/mockbin/docker.log" 2>/dev/null; then
  pass "sandbox run triggers docker build when image missing"
else
  fail "sandbox run triggers docker build when image missing"
fi
if grep -q "docker run" "${tmpdir}/mockbin/docker.log" 2>/dev/null; then
  pass "sandbox run calls docker run after build"
else
  fail "sandbox run calls docker run after build"
fi
rm -rf "${tmpdir}"

# Test: sandbox run with existing image skips build, goes straight to run
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "sandbox run with existing image exits code 0"
assert_contains "${output_all}" "image up to date:" "sandbox run skips build when image exists"
assert_not_contains "${output_all}" "building image:" "sandbox run does NOT build when image exists"

# Verify docker build was NOT called but docker run WAS called
if grep -q "docker build" "${tmpdir}/mockbin/docker.log" 2>/dev/null; then
  fail "docker build is NOT called when image exists (run)"
else
  pass "docker build is NOT called when image exists (run)"
fi
if grep -q "docker run" "${tmpdir}/mockbin/docker.log" 2>/dev/null; then
  pass "docker run IS called when image exists"
else
  fail "docker run IS called when image exists"
fi
rm -rf "${tmpdir}"

# Test: sandbox run -f custom/config.yaml uses custom config path
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin" "${tmpdir}/custom"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/custom/config.yaml" <<'YAML'
agent: gemini-cli
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/custom/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "sandbox run -f custom/config.yaml exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "SANDBOX_AGENT=gemini-cli" "sandbox run -f uses custom config for agent"
rm -rf "${tmpdir}"

# Test: docker run receives correct IMAGE_TAG
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
set -e
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "sandbox-" "docker run receives image tag with sandbox- prefix"
# Verify the tag has the hash part (12 chars after colon)
if echo "${docker_run_line}" | grep -qE 'sandbox-[a-z0-9._-]+:[a-f0-9]{12}'; then
  pass "docker run receives IMAGE_TAG in correct format"
else
  fail "docker run receives IMAGE_TAG in correct format" "line: ${docker_run_line}"
fi
rm -rf "${tmpdir}"

# ============================================================================
# AC: cmd_run — mount flag assembly and path resolution
# ============================================================================

echo "# AC: cmd_run — mount flag assembly and path resolution"

# Test: single mount {source: ".", target: "/workspace"} produces -v /abs/path:/workspace flag
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin" "${tmpdir}/project/.sandbox"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/project/.sandbox/config.yaml" <<'YAML'
agent: claude-code
mounts:
  - source: ".."
    target: "/workspace"
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/project/.sandbox/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "sandbox run with single mount exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "-v ${tmpdir}/project:/workspace" "single mount produces -v with resolved absolute path"
rm -rf "${tmpdir}"

# Test: relative source path resolved against config file directory, not $PWD
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin" "${tmpdir}/myproject/subdir/.sandbox"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/myproject/subdir/.sandbox/config.yaml" <<'YAML'
agent: claude-code
mounts:
  - source: ".."
    target: "/workspace"
YAML
set +e
# Run from a DIFFERENT directory than where the config lives
output_all="$(cd "${tmpdir}" && PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/myproject/subdir/.sandbox/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "sandbox run with relative mount from different PWD exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
# ".." relative to config dir (.sandbox/) should resolve to subdir/, not to $PWD
assert_contains "${docker_run_line}" "-v ${tmpdir}/myproject/subdir:/workspace" "relative path resolves against config dir, not PWD"
rm -rf "${tmpdir}"

# Test: multiple mounts produce multiple -v flags in docker run
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin" "${tmpdir}/project" "${tmpdir}/shared"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<YAML
agent: claude-code
mounts:
  - source: "${tmpdir}/project"
    target: "/workspace"
  - source: "${tmpdir}/shared"
    target: "/shared"
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "sandbox run with multiple mounts exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "-v ${tmpdir}/project:/workspace" "first mount -v flag present"
assert_contains "${docker_run_line}" "-v ${tmpdir}/shared:/shared" "second mount -v flag present"
rm -rf "${tmpdir}"

# Test: -w flag is set to first mount's target path
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin" "${tmpdir}/project"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<YAML
agent: claude-code
mounts:
  - source: "${tmpdir}/project"
    target: "/workspace"
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
set -e
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "-w /workspace" "docker run includes -w set to first mount target"
rm -rf "${tmpdir}"

# Test: sandbox run with no mounts in config produces no -v flags
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "sandbox run with no mounts exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_not_contains "${docker_run_line}" "-v " "no -v flags when config has no mounts"
assert_not_contains "${docker_run_line}" "-w " "no -w flag when config has no mounts"
rm -rf "${tmpdir}"

# Test: absolute source paths are passed through unchanged
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin" "${tmpdir}/abs-data"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<YAML
agent: claude-code
mounts:
  - source: "${tmpdir}/abs-data"
    target: "/data"
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
set -e
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "-v ${tmpdir}/abs-data:/data" "absolute source path passed through unchanged"
rm -rf "${tmpdir}"

# ============================================================================
# AC: cmd_run — end-to-end mount behavior
# ============================================================================

echo "# AC: cmd_run — end-to-end mount behavior"

# Test: config with mounts uncommented passes parse + build + run without errors
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin" "${tmpdir}/project/.sandbox"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/project/.sandbox/config.yaml" <<'YAML'
agent: claude-code
sdks:
  nodejs: "22"
mounts:
  - source: ".."
    target: "/workspace"
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/project/.sandbox/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "e2e: config with mounts passes parse+build+run"
assert_contains "${output_all}" "starting sandbox:" "e2e: sandbox starts successfully with mounts"
rm -rf "${tmpdir}"

# Test: verify mount flags appear in mock docker log with correct resolved paths
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin" "${tmpdir}/workspace/.sandbox" "${tmpdir}/data"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/workspace/.sandbox/config.yaml" <<YAML
agent: claude-code
mounts:
  - source: ".."
    target: "/workspace"
  - source: "${tmpdir}/data"
    target: "/data"
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/workspace/.sandbox/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "e2e: multi-mount config exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
# Relative ".." from .sandbox/ should resolve to workspace/
assert_contains "${docker_run_line}" "-v ${tmpdir}/workspace:/workspace" "e2e: relative mount resolved correctly in docker log"
assert_contains "${docker_run_line}" "-v ${tmpdir}/data:/data" "e2e: absolute mount present in docker log"
assert_contains "${docker_run_line}" "-w /workspace" "e2e: working directory set to first mount target"
rm -rf "${tmpdir}"

# ============================================================================
# AC: validate_secrets — secret validation (AC 2, 3)
# ============================================================================

echo "# AC: validate_secrets — secret validation"

# Test: declared secret not set in host env exits with code 4
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
secrets:
  - ANTHROPIC_API_KEY
YAML
set +e
output_all="$(unset ANTHROPIC_API_KEY && PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 4 "${exit_code}" "secret not set in host env exits code 4"
assert_contains "${output_all}" "secret not set: ANTHROPIC_API_KEY" "secret not set reports specific secret name"
rm -rf "${tmpdir}"

# Test: declared secret set to empty string passes validation
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
secrets:
  - ANTHROPIC_API_KEY
YAML
set +e
output_all="$(ANTHROPIC_API_KEY="" PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "secret set to empty string passes validation (exit 0)"
assert_contains "${output_all}" "starting sandbox:" "secret set to empty string allows sandbox to start"
rm -rf "${tmpdir}"

# Test: multiple secrets all set passes validation
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
secrets:
  - TEST_SECRET_ONE
  - TEST_SECRET_TWO
YAML
set +e
output_all="$(TEST_SECRET_ONE=val1 TEST_SECRET_TWO=val2 PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "multiple secrets all set passes validation"
rm -rf "${tmpdir}"

# Test: first of multiple secrets missing reports that specific secret name
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
secrets:
  - MISSING_SECRET_X
  - TEST_SECRET_TWO
YAML
set +e
output_all="$(unset MISSING_SECRET_X && TEST_SECRET_TWO=val2 PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 4 "${exit_code}" "first missing secret exits code 4"
assert_contains "${output_all}" "secret not set: MISSING_SECRET_X" "first missing secret reports correct name"
rm -rf "${tmpdir}"

# Test: no secrets declared in config passes validation (zero secrets is valid)
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "no secrets declared passes validation"
assert_contains "${output_all}" "starting sandbox:" "no secrets declared allows sandbox to start"
rm -rf "${tmpdir}"

# ============================================================================
# AC: secret injection — -e flags in docker run (AC 1)
# ============================================================================

echo "# AC: secret injection — -e flags in docker run"

# Test: single secret produces -e SECRET_NAME in docker run args
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
secrets:
  - ANTHROPIC_API_KEY
YAML
set +e
output_all="$(ANTHROPIC_API_KEY=sk-test PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "single secret injection exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "-e ANTHROPIC_API_KEY" "single secret produces -e SECRET_NAME in docker run"
rm -rf "${tmpdir}"

# Test: multiple secrets produce multiple -e flags
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
secrets:
  - TEST_SECRET_A
  - TEST_SECRET_B
YAML
set +e
output_all="$(TEST_SECRET_A=val1 TEST_SECRET_B=val2 PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "multiple secret injection exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "-e TEST_SECRET_A" "multiple secrets: first -e flag present"
assert_contains "${docker_run_line}" "-e TEST_SECRET_B" "multiple secrets: second -e flag present"
rm -rf "${tmpdir}"

# Test: no secrets produces no extra -e flags (beyond SANDBOX_AGENT)
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "no secrets: run exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
# Count -e occurrences: should only have SANDBOX_AGENT
e_count="$(echo "${docker_run_line}" | grep -o ' -e ' | wc -l | tr -d ' ')"
if [[ "${e_count}" -eq 1 ]]; then
  pass "no secrets: only SANDBOX_AGENT -e flag present (count: ${e_count})"
else
  fail "no secrets: only SANDBOX_AGENT -e flag present" "expected 1, got ${e_count}"
fi
rm -rf "${tmpdir}"

# Test: secret flags appear in mock docker log
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
secrets:
  - MY_SECRET_KEY
YAML
set +e
output_all="$(MY_SECRET_KEY=testvalue PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "secret flags in docker log: exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "-e MY_SECRET_KEY" "secret flag appears in mock docker log"
rm -rf "${tmpdir}"

# ============================================================================
# AC: secrets not in image or filesystem (AC 4)
# ============================================================================

echo "# AC: secrets not in image or filesystem"

# Test: sandbox build docker log does NOT contain secret values
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 1
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
secrets:
  - BUILD_TEST_SECRET
YAML
set +e
output_all="$(BUILD_TEST_SECRET=supersecretvalue PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "build with secret config exits code 0"
build_log="$(cat "${tmpdir}/mockbin/docker.log" 2>/dev/null || true)"
assert_not_contains "${build_log}" "supersecretvalue" "build docker log does NOT contain secret values"
assert_not_contains "${build_log}" "BUILD_TEST_SECRET" "build docker log does NOT contain secret names as args"
rm -rf "${tmpdir}"

# Test: secrets are passed via -e flag only (runtime injection, not build-time)
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
secrets:
  - RUNTIME_SECRET
YAML
set +e
output_all="$(RUNTIME_SECRET=myvalue PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "runtime secret injection exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "-e RUNTIME_SECRET" "secret injected via -e flag at runtime"
# Verify the secret VALUE is not in the docker log (only the name via -e KEY, not -e KEY=VALUE)
assert_not_contains "${docker_run_line}" "myvalue" "secret value not exposed in docker run args"
docker_build_line="$(grep "docker build" "${tmpdir}/mockbin/docker.log" || true)"
assert_not_contains "${docker_build_line}" "RUNTIME_SECRET" "secret not passed during docker build"
rm -rf "${tmpdir}"

# ============================================================================
# AC: non-secret env var injection — -e KEY=VALUE flags in docker run (AC 1)
# ============================================================================

echo "# AC: non-secret env var injection — -e KEY=VALUE flags in docker run"

# Test: single env var produces -e KEY=VALUE in docker run args
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
env:
  NODE_ENV: development
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "single env var injection exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "-e NODE_ENV=development" "single env var produces -e KEY=VALUE in docker run"
rm -rf "${tmpdir}"

# Test: multiple env vars produce multiple -e KEY=VALUE flags
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
env:
  NODE_ENV: development
  DEBUG: "true"
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "multiple env var injection exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "-e NODE_ENV=development" "multiple env vars: first -e KEY=VALUE flag present"
assert_contains "${docker_run_line}" "-e DEBUG=true" "multiple env vars: second -e KEY=VALUE flag present"
rm -rf "${tmpdir}"

# Test: no env vars produces no extra -e flags (beyond SANDBOX_AGENT and secrets)
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "no env vars: run exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
# Count -e occurrences: should only have SANDBOX_AGENT
e_count="$(echo "${docker_run_line}" | grep -o ' -e ' | wc -l | tr -d ' ')"
if [[ "${e_count}" -eq 1 ]]; then
  pass "no env vars: only SANDBOX_AGENT -e flag present (count: ${e_count})"
else
  fail "no env vars: only SANDBOX_AGENT -e flag present" "expected 1, got ${e_count}"
fi
rm -rf "${tmpdir}"

# Test: env var with spaces in value is passed correctly
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<YAML
agent: claude-code
env:
  MY_MESSAGE: hello world
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "env var with spaces exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "-e MY_MESSAGE=hello world" "env var with spaces in value is passed correctly"
rm -rf "${tmpdir}"

# Test: env vars appear in mock docker log alongside secret and mount flags
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin" "${tmpdir}/project"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<YAML
agent: claude-code
secrets:
  - MY_SECRET
env:
  APP_ENV: staging
mounts:
  - source: ./project
    target: /workspace
YAML
set +e
output_all="$(MY_SECRET=secret123 PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "env vars alongside secrets and mounts exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "-e SANDBOX_AGENT=claude-code" "SANDBOX_AGENT present alongside env vars"
assert_contains "${docker_run_line}" "-e MY_SECRET" "secret flag present alongside env vars"
assert_contains "${docker_run_line}" "-e APP_ENV=staging" "env var flag present alongside secrets"
assert_contains "${docker_run_line}" "-v " "mount flag present alongside env vars"
rm -rf "${tmpdir}"

# ============================================================================
# Integration: complete cmd_run() flag assembly (AC 1, 3)
# ============================================================================

echo "# Integration: complete cmd_run() flag assembly"

# Test: config with mounts + secrets + env vars produces all flags in correct order
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin" "${tmpdir}/src"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<YAML
agent: claude-code
secrets:
  - API_KEY
  - DB_PASSWORD
env:
  NODE_ENV: production
  LOG_LEVEL: debug
mounts:
  - source: ./src
    target: /workspace
YAML
set +e
output_all="$(API_KEY=key123 DB_PASSWORD=pass456 PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "integration: mounts + secrets + env vars exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "-it" "integration: -it flag present"
assert_contains "${docker_run_line}" "--rm" "integration: --rm flag present"
assert_contains "${docker_run_line}" "-e SANDBOX_AGENT=claude-code" "integration: SANDBOX_AGENT present"
assert_contains "${docker_run_line}" "-e API_KEY" "integration: first secret present"
assert_contains "${docker_run_line}" "-e DB_PASSWORD" "integration: second secret present"
assert_contains "${docker_run_line}" "-e NODE_ENV=production" "integration: first env var present"
assert_contains "${docker_run_line}" "-e LOG_LEVEL=debug" "integration: second env var present"
resolved_src="$(cd "${tmpdir}/src" && pwd)"
assert_contains "${docker_run_line}" "-v ${resolved_src}:/workspace" "integration: mount flag with resolved path"
assert_contains "${docker_run_line}" "-w /workspace" "integration: working directory flag present"
rm -rf "${tmpdir}"

# Test: BMAD-style project directory mounted at /workspace shows correct -v and -w flags
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin" "${tmpdir}/my-project/_bmad-output" "${tmpdir}/my-project/docs"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<YAML
agent: claude-code
env:
  BMAD_PROJECT: my-project
mounts:
  - source: ./my-project
    target: /workspace
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "BMAD project mount exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
resolved_project="$(cd "${tmpdir}/my-project" && pwd)"
assert_contains "${docker_run_line}" "-v ${resolved_project}:/workspace" "BMAD project: correct -v mount flag"
assert_contains "${docker_run_line}" "-w /workspace" "BMAD project: correct -w working directory"
assert_contains "${docker_run_line}" "-e BMAD_PROJECT=my-project" "BMAD project: env var present"
rm -rf "${tmpdir}"

# Test: env vars coexist with secrets without conflicts
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<YAML
agent: claude-code
secrets:
  - SECRET_TOKEN
env:
  PUBLIC_URL: https://example.com
  APP_NAME: test-app
YAML
set +e
output_all="$(SECRET_TOKEN=tok123 PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "env vars coexist with secrets exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
# Verify secret uses -e KEY (no value) and env vars use -e KEY=VALUE
assert_contains "${docker_run_line}" "-e SECRET_TOKEN" "coexist: secret flag present"
assert_not_contains "${docker_run_line}" "-e SECRET_TOKEN=" "coexist: secret does NOT have value in flag"
assert_contains "${docker_run_line}" "-e PUBLIC_URL=https://example.com" "coexist: first env var with value"
assert_contains "${docker_run_line}" "-e APP_NAME=test-app" "coexist: second env var with value"
rm -rf "${tmpdir}"

# ============================================================================
# Review fixes: env var validation and edge cases
# ============================================================================

echo "# Review fixes: env var validation and edge cases"

# Test: invalid env var name is rejected with exit code 4
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
env:
  INVALID-KEY: value
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 4 "${exit_code}" "invalid env var name exits code 4"
assert_contains "${output_all}" "invalid env var name" "invalid env var name error message shown"
rm -rf "${tmpdir}"

# Test: env var name with equals sign is rejected
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
env:
  FOO=BAR: value
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 4 "${exit_code}" "env var name with equals rejected with exit code 4"
rm -rf "${tmpdir}"

# Test: valid env var names with underscores and numbers pass validation
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
env:
  _PRIVATE: secret
  NODE_ENV_2: test
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "valid env var names with underscores and numbers pass"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "-e _PRIVATE=secret" "underscore-prefixed env var passes validation"
assert_contains "${docker_run_line}" "-e NODE_ENV_2=test" "alphanumeric env var passes validation"
rm -rf "${tmpdir}"

# Test: YAML null value (env: { FOO: }) produces empty string, not literal "null"
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
env:
  FOO:
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "null env value exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "-e FOO=" "null env value produces empty value, not literal null"
assert_not_contains "${docker_run_line}" "-e FOO=null" "null env value is not literal string null"
rm -rf "${tmpdir}"

# Test: empty env map (env: {}) produces no extra -e flags
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
env: {}
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "empty env map exits code 0"
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
e_count="$(echo "${docker_run_line}" | grep -o ' -e ' | wc -l | tr -d ' ')"
if [[ "${e_count}" -eq 1 ]]; then
  pass "empty env map: only SANDBOX_AGENT -e flag present (count: ${e_count})"
else
  fail "empty env map: only SANDBOX_AGENT -e flag present" "expected 1, got ${e_count}"
fi
rm -rf "${tmpdir}"

# ============================================================================
# AC: entrypoint.sh — agent mapping and error handling
# ============================================================================

echo "# AC: entrypoint.sh — agent mapping and error handling"

ENTRYPOINT="${PROJECT_ROOT}/scripts/entrypoint.sh"

# Test: SANDBOX_AGENT=claude-code execs claude --dangerously-skip-permissions
tmpdir="$(mktemp -d)"
mock_agent_dir="$(mktemp -d)"
cat > "${mock_agent_dir}/claude" <<MOCK
#!/usr/bin/env bash
echo "claude \$*" > "${tmpdir}/agent.log"
MOCK
chmod +x "${mock_agent_dir}/claude"

set +e
SANDBOX_AGENT=claude-code PATH="${mock_agent_dir}:${SYSTEM_PATH}" bash "${ENTRYPOINT}" 2>&1
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "entrypoint with claude-code exits code 0"
if [[ -f "${tmpdir}/agent.log" ]]; then
  agent_output="$(cat "${tmpdir}/agent.log")"
  assert_contains "${agent_output}" "--dangerously-skip-permissions" "entrypoint execs claude with --dangerously-skip-permissions"
else
  fail "entrypoint execs claude with --dangerously-skip-permissions" "agent.log not created"
fi
rm -rf "${tmpdir}" "${mock_agent_dir}"

# Test: SANDBOX_AGENT=gemini-cli execs gemini
tmpdir="$(mktemp -d)"
mock_agent_dir="$(mktemp -d)"
cat > "${mock_agent_dir}/gemini" <<MOCK
#!/usr/bin/env bash
echo "gemini \$*" > "${tmpdir}/agent.log"
MOCK
chmod +x "${mock_agent_dir}/gemini"

set +e
SANDBOX_AGENT=gemini-cli PATH="${mock_agent_dir}:${SYSTEM_PATH}" bash "${ENTRYPOINT}" 2>&1
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "entrypoint with gemini-cli exits code 0"
if [[ -f "${tmpdir}/agent.log" ]]; then
  agent_output="$(cat "${tmpdir}/agent.log")"
  assert_contains "${agent_output}" "gemini" "entrypoint execs gemini"
else
  fail "entrypoint execs gemini" "agent.log not created"
fi
rm -rf "${tmpdir}" "${mock_agent_dir}"

# Test: unknown SANDBOX_AGENT value exits with error
set +e
output_all="$(SANDBOX_AGENT=unknown-agent PATH="${SYSTEM_PATH}" bash "${ENTRYPOINT}" 2>&1)"
exit_code=$?
set -e
assert_exit_code 1 "${exit_code}" "entrypoint with unknown agent exits code 1"
assert_contains "${output_all}" "error: unknown agent: unknown-agent" "entrypoint prints unknown agent error"

# Test: unset SANDBOX_AGENT exits with error
set +e
output_all="$(unset SANDBOX_AGENT && PATH="${SYSTEM_PATH}" bash "${ENTRYPOINT}" 2>&1)"
exit_code=$?
set -e
assert_exit_code 1 "${exit_code}" "entrypoint with unset SANDBOX_AGENT exits code 1"
assert_contains "${output_all}" "error: SANDBOX_AGENT not set" "entrypoint prints SANDBOX_AGENT not set error"

# Test: SANDBOX_AGENT=claude-code with claude not in PATH exits with error
empty_bin_dir="$(mktemp -d)"
set +e
output_all="$(SANDBOX_AGENT=claude-code PATH="${empty_bin_dir}:${SYSTEM_PATH}" bash "${ENTRYPOINT}" 2>&1)"
exit_code=$?
set -e
assert_exit_code 1 "${exit_code}" "entrypoint with claude-code but no claude binary exits code 1"
assert_contains "${output_all}" "error: claude not found in PATH" "entrypoint prints claude not found error"
rm -rf "${empty_bin_dir}"

# Test: SANDBOX_AGENT=gemini-cli with gemini not in PATH exits with error
empty_bin_dir="$(mktemp -d)"
set +e
output_all="$(SANDBOX_AGENT=gemini-cli PATH="${empty_bin_dir}:${SYSTEM_PATH}" bash "${ENTRYPOINT}" 2>&1)"
exit_code=$?
set -e
assert_exit_code 1 "${exit_code}" "entrypoint with gemini-cli but no gemini binary exits code 1"
assert_contains "${output_all}" "error: gemini not found in PATH" "entrypoint prints gemini not found error"
rm -rf "${empty_bin_dir}"

# ============================================================================
# AC: git-wrapper.sh — transparent passthrough to real git
# ============================================================================

echo "# AC: git-wrapper.sh — transparent passthrough to real git"

GIT_WRAPPER="${PROJECT_ROOT}/scripts/git-wrapper.sh"

# Test: git-wrapper.sh forwards all arguments to /usr/bin/git
tmpdir="$(mktemp -d)"
mock_git="${tmpdir}/git"
cat > "${mock_git}" <<'MOCK'
#!/usr/bin/env bash
echo "ARGS:$*"
MOCK
chmod +x "${mock_git}"
# The wrapper uses exec /usr/bin/git "$@", so we replace /usr/bin/git path in a copy
wrapper_copy="${tmpdir}/git-wrapper-test.sh"
sed "s|/usr/bin/git|${mock_git}|g" "${GIT_WRAPPER}" > "${wrapper_copy}"
chmod +x "${wrapper_copy}"

set +e
output_all="$(bash "${wrapper_copy}" add -A -- file1.txt file2.txt 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "git wrapper exits 0 when git succeeds"
assert_contains "${output_all}" "ARGS:add -A -- file1.txt file2.txt" "git wrapper forwards all arguments to real git"

# Test: git-wrapper.sh preserves exit codes from real git
mock_git_fail="${tmpdir}/git-fail"
cat > "${mock_git_fail}" <<'MOCK'
#!/usr/bin/env bash
exit 128
MOCK
chmod +x "${mock_git_fail}"
wrapper_fail="${tmpdir}/git-wrapper-fail.sh"
sed "s|/usr/bin/git|${mock_git_fail}|g" "${GIT_WRAPPER}" > "${wrapper_fail}"
chmod +x "${wrapper_fail}"

set +e
output_all="$(bash "${wrapper_fail}" status 2>&1)"
exit_code=$?
set -e
assert_exit_code 128 "${exit_code}" "git wrapper preserves non-zero exit code from real git"

# Test: git-wrapper.sh passes stdin through (for commit messages)
mock_git_stdin="${tmpdir}/git-stdin"
cat > "${mock_git_stdin}" <<'MOCK'
#!/usr/bin/env bash
echo "STDIN:$(cat)"
MOCK
chmod +x "${mock_git_stdin}"
wrapper_stdin="${tmpdir}/git-wrapper-stdin.sh"
sed "s|/usr/bin/git|${mock_git_stdin}|g" "${GIT_WRAPPER}" > "${wrapper_stdin}"
chmod +x "${wrapper_stdin}"

set +e
output_all="$(echo "test commit message" | bash "${wrapper_stdin}" commit -F - 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "git wrapper with stdin exits 0"
assert_contains "${output_all}" "STDIN:test commit message" "git wrapper passes stdin through to real git"

# Test: common git operations all pass through (add, commit, log, diff, branch, checkout, merge, commit --amend)
mock_git_ops="${tmpdir}/git-ops"
cat > "${mock_git_ops}" <<'MOCK'
#!/usr/bin/env bash
echo "OP:$*"
MOCK
chmod +x "${mock_git_ops}"
wrapper_ops="${tmpdir}/git-wrapper-ops.sh"
sed "s|/usr/bin/git|${mock_git_ops}|g" "${GIT_WRAPPER}" > "${wrapper_ops}"
chmod +x "${wrapper_ops}"

for op in add commit log diff branch checkout merge push; do
  set +e
  output_all="$(bash "${wrapper_ops}" "${op}" 2>&1)"
  exit_code=$?
  set -e
  assert_exit_code 0 "${exit_code}" "git wrapper passes through '${op}' operation"
  assert_contains "${output_all}" "OP:${op}" "git wrapper forwards '${op}' to real git"
done

# Test: git commit --amend passes through
set +e
output_all="$(bash "${wrapper_ops}" commit --amend 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "git wrapper passes through 'commit --amend' operation"
assert_contains "${output_all}" "OP:commit --amend" "git wrapper forwards 'commit --amend' to real git"

rm -rf "${tmpdir}"

# ============================================================================
# AC: Dockerfile.template includes git, curl, wget, dnsutils in base packages
# ============================================================================

echo "# AC: Dockerfile.template includes git, curl, wget, dnsutils in base packages"

DOCKERFILE_TEMPLATE="${PROJECT_ROOT}/Dockerfile.template"
template_content="$(cat "${DOCKERFILE_TEMPLATE}")"

# Extract the "Common CLI tools" apt-get install block to test against
base_pkg_block="$(sed -n '/^# Common CLI tools/,/rm -rf/p' "${DOCKERFILE_TEMPLATE}")"

# Test: Dockerfile.template base apt-get install includes git
assert_contains "${base_pkg_block}" "git" "Dockerfile.template base packages include git"

# Test: Dockerfile.template base apt-get install includes curl
assert_contains "${base_pkg_block}" "curl" "Dockerfile.template base packages include curl"

# Test: Dockerfile.template base apt-get install includes wget
assert_contains "${base_pkg_block}" "wget" "Dockerfile.template base packages include wget"

# Test: Dockerfile.template base apt-get install includes dnsutils
assert_contains "${base_pkg_block}" "dnsutils" "Dockerfile.template base packages include dnsutils"

# Test: resolved Dockerfile contains expected base package installation lines
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 1
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
set -e
# The build process creates a resolved Dockerfile — verify docker build was invoked
docker_build_line="$(grep "docker build" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_build_line}" "docker build" "docker build command was invoked"
rm -rf "${tmpdir}"

# Test: no network isolation flags in docker run command (internet unrestricted by default)
tmpdir="$(mktemp -d)"
mkdir -p "${tmpdir}/mockbin"
setup_build_mock "${tmpdir}/mockbin" 0
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(PATH="${tmpdir}/mockbin:${PATH}" bash "${SANDBOX}" run -f "${tmpdir}/config.yaml" 2>&1)"
set -e
docker_run_line="$(grep "docker run" "${tmpdir}/mockbin/docker.log" || true)"
assert_contains "${docker_run_line}" "docker run" "docker run command was invoked"
assert_not_contains "${docker_run_line}" "--network=none" "docker run does not include --network=none flag"
assert_not_contains "${docker_run_line}" "--network none" "docker run does not include --network none flag"
assert_not_contains "${docker_run_line}" "--net=none" "docker run does not include --net=none flag"
rm -rf "${tmpdir}"

# ============================================================================
# Summary
# ============================================================================

echo ""
echo "# Test Results: ${TESTS_PASSED}/${TESTS_RUN} passed, ${TESTS_FAILED} failed"

if [[ "${TESTS_FAILED}" -gt 0 ]]; then
  exit 1
fi
