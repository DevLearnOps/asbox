#!/usr/bin/env bash
# Integration test: verify Docker Compose works inside the sandbox (both subcommand and standalone).
# Requires Docker on the host and a pre-built .sandbox-dockerfile.
# Run: bash tests/test_integration_compose.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
IMAGE_TAG="sandbox-integration-test:compose"

passed=0
failed=0

pass() { echo "ok - $1"; ((passed++)) || true; }
fail() { echo "FAIL - $1${2:+ ($2)}"; ((failed++)) || true; }

# Run a command inside the sandbox using the entrypoint's root init + privilege drop.
# Optional second arg: extra docker run flags (e.g. volume mounts).
# Optional third arg: HOST_UID to test UID remapping (default: no remapping).
run_in_sandbox() {
  local cmd="$1"
  local extra_flags="${2:-}"
  local host_uid="${3:-}"
  local uid_remap=""
  if [[ -n "${host_uid}" ]]; then
    uid_remap="
      current_uid=\$(id -u sandbox)
      if [[ \"${host_uid}\" != \"\$current_uid\" ]]; then
        usermod -u ${host_uid} sandbox
        groupmod -g ${host_uid} sandbox 2>/dev/null || true
        chown -R sandbox:sandbox /home/sandbox
        mkdir -p /run/user/${host_uid}
        chown sandbox:sandbox /run/user/${host_uid}
        chmod 700 /run/user/${host_uid}
      fi
    "
  fi
  # shellcheck disable=SC2086
  timeout 120 docker run --rm \
    --device /dev/net/tun \
    --device /dev/fuse \
    --security-opt seccomp=unconfined \
    --security-opt apparmor=unconfined \
    --security-opt label=disable \
    --cap-add SYS_ADMIN \
    -e SANDBOX_AGENT=test \
    ${extra_flags} \
    "${IMAGE_TAG}" bash -c "
      # Replicate entrypoint root init
      mount --make-rshared / 2>/dev/null || true
      grep ' /proc/' /proc/self/mountinfo | awk '{print \$5}' | sort -r | while read -r mp; do
        umount \"\$mp\" 2>/dev/null || true
      done
      ${uid_remap}
      # Ensure XDG_RUNTIME_DIR exists (must run as root since /run/user is root-owned)
      runtime_dir=\"/run/user/\$(id -u sandbox)\"
      mkdir -p \"\$runtime_dir\"
      chown sandbox:sandbox \"\$runtime_dir\"
      chmod 700 \"\$runtime_dir\"
      exec runuser -u sandbox -- bash -c '${cmd}'
    " 2>&1
}

# ============================================================================
# Build the test image
# ============================================================================

echo "# Building sandbox image for compose integration test..."

docker build -t "${IMAGE_TAG}" -f "${PROJECT_ROOT}/.sandbox-dockerfile" "${PROJECT_ROOT}" 2>&1 | tail -5
if [[ ${PIPESTATUS[0]} -ne 0 ]]; then
  echo "FATAL: image build failed"
  exit 1
fi
echo "# Image built: ${IMAGE_TAG}"
echo ""

# ============================================================================
# Test 1: docker compose version (subcommand via CLI plugin)
# ============================================================================

echo "# Test 1: docker compose version"
output="$(run_in_sandbox "docker compose version 2>&1")" || true
if echo "${output}" | grep -qi "docker compose version"; then
  pass "docker compose version works (CLI plugin)"
else
  fail "docker compose version works (CLI plugin)" "$(echo "${output}" | tail -5)"
fi

# ============================================================================
# Test 2: docker-compose version (standalone binary, backwards compat)
# ============================================================================

echo "# Test 2: docker-compose version"
output="$(run_in_sandbox "docker-compose version 2>&1")" || true
if echo "${output}" | grep -qi "docker compose version"; then
  pass "docker-compose version works (standalone binary)"
else
  fail "docker-compose version works (standalone binary)" "$(echo "${output}" | tail -5)"
fi

# ============================================================================
# Test 3: docker compose up -d with test fixtures
# ============================================================================

echo "# Test 3: docker compose up -d"
output="$(run_in_sandbox '
  export XDG_RUNTIME_DIR="/run/user/$(id -u)"
  export DOCKER_HOST="unix://${XDG_RUNTIME_DIR}/podman/podman.sock"
  podman system migrate 2>/dev/null || true
  mkdir -p "${XDG_RUNTIME_DIR}/podman" 2>/dev/null || true
  podman system service --time=0 "unix://${XDG_RUNTIME_DIR}/podman/podman.sock" &
  ln -sf "${XDG_RUNTIME_DIR}/podman/podman.sock" "${XDG_RUNTIME_DIR}/docker.sock" 2>/dev/null || true
  sleep 2

  cd /fixtures
  docker compose up -d 2>&1
  echo "COMPOSE_UP_EXIT=$?"

  sleep 5
  docker compose ps 2>&1
  docker compose down 2>&1
' "-v ${PROJECT_ROOT}/tests/fixtures:/fixtures:ro")" || true
if echo "${output}" | grep -q "COMPOSE_UP_EXIT=0"; then
  pass "docker compose up -d starts services"
else
  fail "docker compose up -d starts services" "$(echo "${output}" | tail -15)"
fi

# ============================================================================
# Test 4: docker compose works after UID remapping (HOST_UID)
# ============================================================================

echo "# Test 4: docker compose version after UID remap to 501"
output="$(run_in_sandbox "docker compose version 2>&1" "" "501")" || true
if echo "${output}" | grep -qi "docker compose version"; then
  pass "docker compose version works after UID remap"
else
  fail "docker compose version works after UID remap" "$(echo "${output}" | tail -5)"
fi

# ============================================================================
# Test 5: docker compose up -d after UID remapping
# ============================================================================

echo "# Test 5: docker compose up -d after UID remap to 501"
output="$(run_in_sandbox '
  export XDG_RUNTIME_DIR="/run/user/$(id -u)"
  export DOCKER_HOST="unix://${XDG_RUNTIME_DIR}/podman/podman.sock"
  podman system migrate 2>/dev/null || true
  mkdir -p "${XDG_RUNTIME_DIR}/podman" 2>/dev/null || true
  podman system service --time=0 "unix://${XDG_RUNTIME_DIR}/podman/podman.sock" &
  ln -sf "${XDG_RUNTIME_DIR}/podman/podman.sock" "${XDG_RUNTIME_DIR}/docker.sock" 2>/dev/null || true
  sleep 2

  cd /fixtures
  docker compose up -d 2>&1
  echo "COMPOSE_UP_EXIT=$?"

  sleep 5
  docker compose ps 2>&1
  docker compose down 2>&1
' "-v ${PROJECT_ROOT}/tests/fixtures:/fixtures:ro" "501")" || true
if echo "${output}" | grep -q "COMPOSE_UP_EXIT=0"; then
  pass "docker compose up -d works after UID remap"
else
  fail "docker compose up -d works after UID remap" "$(echo "${output}" | tail -15)"
fi

# ============================================================================
# Test 6: service-name DNS resolution between compose containers
# ============================================================================

echo "# Test 6: service-name DNS resolution (aardvark-dns via netavark)"
output="$(run_in_sandbox '
  export XDG_RUNTIME_DIR="/run/user/$(id -u)"
  export DOCKER_HOST="unix://${XDG_RUNTIME_DIR}/podman/podman.sock"
  podman system migrate 2>/dev/null || true
  mkdir -p "${XDG_RUNTIME_DIR}/podman" 2>/dev/null || true
  podman system service --time=0 "unix://${XDG_RUNTIME_DIR}/podman/podman.sock" &
  ln -sf "${XDG_RUNTIME_DIR}/podman/podman.sock" "${XDG_RUNTIME_DIR}/docker.sock" 2>/dev/null || true
  sleep 2

  cd /fixtures
  docker compose up -d 2>&1
  sleep 5

  # Test DNS resolution: client container resolves "web" service name
  docker compose exec client nslookup web 2>&1
  echo "DNS_RESOLVE_EXIT=$?"

  docker compose down 2>&1
' "-v ${PROJECT_ROOT}/tests/fixtures:/fixtures:ro")" || true
if echo "${output}" | grep -q "DNS_RESOLVE_EXIT=0"; then
  pass "service-name DNS resolution works between compose services"
else
  fail "service-name DNS resolution works between compose services" "$(echo "${output}" | tail -15)"
fi

# ============================================================================
# Summary
# ============================================================================

echo ""
echo "# Integration Test Results: $((passed + failed)) tests, ${passed} passed, ${failed} failed"

if [[ ${failed} -gt 0 ]]; then
  exit 1
fi
