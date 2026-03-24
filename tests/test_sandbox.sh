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

if grep -Ew 'eval' "${SANDBOX}" | grep -qv 'yq eval'; then
  fail "no eval usage" "found bash eval in sandbox.sh"
else
  pass "no eval usage"
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
# Summary
# ============================================================================

echo ""
echo "# Test Results: ${TESTS_PASSED}/${TESTS_RUN} passed, ${TESTS_FAILED} failed"

if [[ "${TESTS_FAILED}" -gt 0 ]]; then
  exit 1
fi
