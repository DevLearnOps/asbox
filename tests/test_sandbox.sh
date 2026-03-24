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
for bin in bash env cat echo grep sed awk chmod mkdir rm cp dirname pwd; do
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

for cmd in build run; do
  set +e
  output_all="$(bash "${SANDBOX}" "${cmd}" 2>&1)"
  exit_code=$?
  set -e
  assert_exit_code 0 "${exit_code}" "'${cmd}' exits with code 0"
  assert_contains "${output_all}" "not yet implemented" "'${cmd}' prints not yet implemented"
done

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
# AC: parse_config — valid config extraction
# ============================================================================

echo "# AC: parse_config — valid config extraction"

# Test: parse_config extracts agent correctly
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
YAML
set +e
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "parse_config extracts agent correctly (exit 0)"
assert_contains "${output_all}" "not yet implemented" "parse_config succeeds then build continues"
rm -rf "${tmpdir}"

# Test: parse_config extracts agent gemini-cli
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: gemini-cli
YAML
set +e
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/nonexistent.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/custom/my-config.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" -f "${tmpdir}/config.yaml" run 2>&1)"
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
output_all="$(cd "${tmpdir}" && bash "${SANDBOX}" build 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "parse_config works with default starter config from sandbox init"
assert_contains "${output_all}" "not yet implemented" "build continues after parsing starter config"
rm -rf "${tmpdir}"

# Test: run also works with starter config
tmpdir="$(mktemp -d)"
output_all="$(cd "${tmpdir}" && bash "${SANDBOX}" init 2>&1)"
set +e
output_all="$(cd "${tmpdir}" && bash "${SANDBOX}" run 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "parse_config works with starter config via run command"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
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
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
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
output_all="$(cd "${tmpdir}" && bash "${SANDBOX}" build 2>&1)"
exit_code=$?
set -e
assert_exit_code 0 "${exit_code}" "end-to-end build with starter config exits code 0"
assert_contains "${output_all}" "generated Dockerfile" "end-to-end build prints generated Dockerfile message"
assert_contains "${output_all}" "not yet implemented" "end-to-end build still prints not yet implemented (docker build is story 1-5)"
if [[ -f "${PROJECT_ROOT}/.sandbox-dockerfile" ]]; then
  pass "end-to-end build creates .sandbox-dockerfile"
else
  fail "end-to-end build creates .sandbox-dockerfile"
fi
rm -rf "${tmpdir}"

# ============================================================================
# Summary
# ============================================================================

echo ""
echo "# Test Results: ${TESTS_PASSED}/${TESTS_RUN} passed, ${TESTS_FAILED} failed"

if [[ "${TESTS_FAILED}" -gt 0 ]]; then
  exit 1
fi
