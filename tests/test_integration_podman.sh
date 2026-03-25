#!/usr/bin/env bash
# Integration test: verify rootless Podman works inside the sandbox container.
# Requires Docker on the host. Run: bash tests/test_integration_podman.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
IMAGE_TAG="sandbox-integration-test:podman"

passed=0
failed=0

pass() { echo "ok - $1"; ((passed++)) || true; }
fail() { echo "FAIL - $1${2:+ ($2)}"; ((failed++)) || true; }

# Run a command inside the sandbox using the entrypoint's root init + privilege drop
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

echo "# Building sandbox image for integration test..."

docker build -t "${IMAGE_TAG}" -f "${PROJECT_ROOT}/.sandbox-dockerfile" "${PROJECT_ROOT}" 2>&1 | tail -5
if [[ ${PIPESTATUS[0]} -ne 0 ]]; then
  echo "FATAL: image build failed"
  exit 1
fi
echo "# Image built: ${IMAGE_TAG}"
echo ""

# ============================================================================
# Test 1: podman info succeeds
# ============================================================================

echo "# Test 1: podman info"
output="$(run_in_sandbox "podman info 2>&1")" || true
if echo "${output}" | grep -qi "host"; then
  pass "podman info succeeds"
else
  fail "podman info succeeds" "$(echo "${output}" | tail -5)"
fi

# ============================================================================
# Test 2: docker ps works (via podman-docker alias)
# ============================================================================

echo "# Test 2: docker ps"
output="$(run_in_sandbox "docker ps 2>&1")" || true
if echo "${output}" | grep -qi "CONTAINER ID"; then
  pass "docker ps works via podman-docker alias"
else
  fail "docker ps works via podman-docker alias" "$(echo "${output}" | tail -5)"
fi

# ============================================================================
# Test 3: docker run works (rootless container execution)
# ============================================================================

echo "# Test 3: docker run"
output="$(run_in_sandbox 'docker run --rm docker.io/library/alpine:3.20 echo hello-from-inner 2>&1')" || true
if echo "${output}" | grep -q "hello-from-inner"; then
  pass "docker run executes inner container"
else
  fail "docker run executes inner container" "$(echo "${output}" | tail -10)"
fi

# ============================================================================
# Test 4: docker build works (rootless image build)
# ============================================================================

echo "# Test 4: docker build"
output="$(run_in_sandbox '
  mkdir -p /tmp/tb && cd /tmp/tb
  printf "FROM docker.io/library/alpine:3.20\nRUN echo hello > /test.txt\n" > Dockerfile
  docker build -t inner-test:latest . 2>&1
  echo "BUILD_EXIT=$?"
' 2>&1)" || true
if echo "${output}" | grep -q "BUILD_EXIT=0"; then
  pass "docker build succeeds inside sandbox"
else
  fail "docker build succeeds inside sandbox" "$(echo "${output}" | tail -10)"
fi

# ============================================================================
# Test 5: inner container port forwarding works
# ============================================================================

echo "# Test 5: inner container port forwarding"
output="$(run_in_sandbox '
  docker run -d --name port-test -p 3000:3000 docker.io/library/alpine:3.20 sh -c "while true; do echo -e \"HTTP/1.1 200 OK\r\n\r\nport-test-ok\" | nc -l -p 3000; done" 2>&1
  sleep 3
  curl -s --max-time 5 http://localhost:3000 2>&1 || echo "curl failed"
  docker rm -f port-test 2>/dev/null
' 2>&1)" || true
if echo "${output}" | grep -q "port-test-ok"; then
  pass "inner container port forwarding works"
else
  fail "inner container port forwarding works" "$(echo "${output}" | tail -10)"
fi

# ============================================================================
# Summary
# ============================================================================

echo ""
echo "# Integration Test Results: $((passed + failed)) tests, ${passed} passed, ${failed} failed"

if [[ ${failed} -gt 0 ]]; then
  exit 1
fi
