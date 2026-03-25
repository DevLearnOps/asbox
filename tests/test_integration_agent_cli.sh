#!/usr/bin/env bash
# Integration test: verify agent CLI is installed and discoverable inside the sandbox.
# Requires Docker on the host and a pre-built sandbox image.
# Run: bash tests/test_integration_agent_cli.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
IMAGE_TAG="sandbox-integration-test:agent-cli"

passed=0
failed=0

pass() { echo "ok - $1"; ((passed++)) || true; }
fail() { echo "FAIL - $1${2:+ ($2)}"; ((failed++)) || true; }

# Run a command inside the sandbox as the sandbox user
run_in_sandbox() {
  local cmd="$1"
  timeout 120 docker run --rm \
    --device /dev/net/tun \
    --device /dev/fuse \
    --security-opt seccomp=unconfined \
    --security-opt apparmor=unconfined \
    --security-opt label=disable \
    --cap-add SYS_ADMIN \
    -e SANDBOX_AGENT=test \
    "${IMAGE_TAG}" bash -c "
      # Replicate entrypoint root init
      mount --make-rshared / 2>/dev/null || true
      grep ' /proc/' /proc/self/mountinfo | awk '{print \$5}' | sort -r | while read -r mp; do
        umount \"\$mp\" 2>/dev/null || true
      done
      exec runuser -u sandbox -- bash -c '${cmd}'
    " 2>&1
}

# ============================================================================
# Build the test image
# ============================================================================

echo "# Building sandbox image for agent CLI integration test..."

docker build -t "${IMAGE_TAG}" -f "${PROJECT_ROOT}/.sandbox-dockerfile" "${PROJECT_ROOT}" 2>&1 | tail -5
if [[ ${PIPESTATUS[0]} -ne 0 ]]; then
  echo "FATAL: image build failed"
  exit 1
fi
echo "# Image built: ${IMAGE_TAG}"
echo ""

# ============================================================================
# Test 1: claude binary exists in PATH (AC #5 — command -v check passes)
# ============================================================================

echo "# Test 1: command -v claude"
output="$(run_in_sandbox "command -v claude 2>&1")" || true
if echo "${output}" | grep -q "claude"; then
  pass "claude binary found in PATH"
else
  fail "claude binary found in PATH" "$(echo "${output}" | tail -5)"
fi

# ============================================================================
# Test 2: claude --version succeeds (AC #1 — CLI is functional)
# ============================================================================

echo "# Test 2: claude --version"
output="$(run_in_sandbox "claude --version 2>&1")" || true
if [[ $? -eq 0 ]] && echo "${output}" | grep -qiE "[0-9]+\.[0-9]+"; then
  pass "claude --version returns a version string"
else
  fail "claude --version returns a version string" "$(echo "${output}" | tail -5)"
fi

# ============================================================================
# Test 3: claude is installed globally via npm (AC #1 — installed via npm install -g)
# ============================================================================

echo "# Test 3: claude installed via npm global"
output="$(run_in_sandbox "npm list -g @anthropic-ai/claude-code 2>&1")" || true
if echo "${output}" | grep -q "@anthropic-ai/claude-code"; then
  pass "claude-code is in npm global packages"
else
  fail "claude-code is in npm global packages" "$(echo "${output}" | tail -5)"
fi

# ============================================================================
# Test 4: no version pinning — installed version is latest (AC #4)
# ============================================================================

echo "# Test 4: no pinned version (informational)"
output="$(run_in_sandbox "npm list -g @anthropic-ai/claude-code 2>/dev/null")" || true
version="$(echo "${output}" | grep -o '@anthropic-ai/claude-code@[^ ]*' || true)"
if [[ -n "${version}" ]]; then
  echo "# Installed: ${version}"
  pass "version info printed (manual verification)"
else
  fail "version info printed (manual verification)" "npm list returned no version"
fi

# ============================================================================
# Summary
# ============================================================================

echo ""
echo "# Integration Test Results: $((passed + failed)) tests, ${passed} passed, ${failed} failed"

if [[ ${failed} -gt 0 ]]; then
  exit 1
fi
