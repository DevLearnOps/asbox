# Story 4.1: Podman Installation and Docker Alias

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want Podman installed inside the sandbox with Docker aliased to it,
So that the agent can use familiar `docker` commands without knowing it's using Podman.

## Acceptance Criteria

1. **Given** a sandbox image is built
   **When** the agent runs `docker --version` inside the container
   **Then** Podman responds via the `podman-docker` alias package

2. **Given** the Dockerfile template
   **When** inspecting the Podman installation
   **Then** Podman 5.x is installed from upstream Kubic/OBS repository with `vfs` storage driver, `netavark` networking, `aardvark-dns`, and `file` events logger

3. **Given** the sandbox container is running
   **When** the entrypoint starts the Podman API socket
   **Then** `$DOCKER_HOST` points to `$XDG_RUNTIME_DIR/podman/podman.sock`, socket owned by sandbox user only

4. **Given** the sandbox container is running
   **When** inspecting the outer container's runtime flags
   **Then** it was NOT launched with `--privileged` and does NOT mount the host Docker socket (NFR4)

5. **Given** Testcontainers is used inside the sandbox
   **When** inspecting the environment
   **Then** Ryuk disabled, socket override configured, localhost host override set

## Tasks / Subtasks

- [x] Task 1: Fix `embed/entrypoint.sh` — missing `DOCKER_HOST` export and Podman readiness (AC: #3, #5)
  - [x] Add `XDG_RUNTIME_DIR` export: `export XDG_RUNTIME_DIR="/run/user/$(id -u sandbox)"` before `start_podman_socket`
  - [x] Add `DOCKER_HOST` export in `start_podman_socket()`: `export DOCKER_HOST="unix://${socket_path}"` after creating the socket directory
  - [x] Add `podman system migrate` call before socket start (required for rootless Podman initialization on first run)
  - [x] Add `docker.sock` symlink: `ln -sf "${socket_path}" "$(dirname "${socket_path}")/../docker.sock"` for Docker Compose v2 standalone compatibility
  - [x] Add socket readiness wait loop (poll up to 5 seconds for socket to exist before proceeding to exec)
  - [x] Reorder entrypoint main sequence: `align_uid_gid` -> `chown_volumes` -> `merge_mcp_config` -> `start_podman_socket` -> `set_testcontainers_socket` -> `start_healthcheck_poller` -> exec agent

- [x] Task 2: Verify `/etc/subuid` and `/etc/subgid` for rootless Podman (AC: #2)
  - [x] Check if `embed/Dockerfile.tmpl` sets up subordinate UID/GID ranges for the sandbox user — rootless Podman requires entries in `/etc/subuid` and `/etc/subgid` (e.g., `sandbox:100000:65536`)
  - [x] If missing, add: `RUN echo "sandbox:100000:65536" >> /etc/subuid && echo "sandbox:100000:65536" >> /etc/subgid` after user creation (line 14)
  - [x] Verify `uidmap` and `slirp4netns` packages are installed (already present at `Dockerfile.tmpl` line 61)

- [x] Task 3: Write integration tests for Podman and Docker alias (AC: #1, #2, #3, #5)
  - [x] Create `integration/podman_test.go` using testcontainers-go (same pattern as `integration/isolation_test.go`)
  - [x] Test: exec `docker --version` -> output contains "podman" (AC #1)
  - [x] Test: exec `docker info --format '{{.Driver}}'` -> output contains "vfs" (AC #2 — storage driver)
  - [x] Test: exec `docker compose version` -> returns version string (Docker Compose plugin works)
  - [x] Test: exec `printenv DOCKER_HOST` -> contains `podman/podman.sock` (AC #3)
  - [x] Test: exec `stat` on the Podman socket path -> exists and owned by sandbox user (AC #3)
  - [x] Test: exec `printenv TESTCONTAINERS_RYUK_DISABLED` -> "true" (AC #5)
  - [x] Test: exec `printenv TESTCONTAINERS_HOST_OVERRIDE` -> "localhost" (AC #5)
  - [x] Test: exec `printenv TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE` -> contains `podman.sock` (AC #5)
  - [x] Use `t.Parallel()` for all tests per architecture standards

- [x] Task 4: Verify outer container has no privileged mode or Docker socket mount (AC: #4)
  - [x] Add test in `integration/podman_test.go`: inspect test container via Docker API to confirm `Privileged` is false
  - [x] Add test: inspect test container mounts to confirm no `/var/run/docker.sock` mount
  - [x] Verify `internal/docker/run.go` line 23 does not include `--privileged` in args assembly (already confirmed — document as assertion)

- [x] Task 5: Validate Podman can actually run containers (smoke test) (AC: #1)
  - [x] Add integration test: exec `docker run --rm alpine:latest echo hello` inside the sandbox -> output contains "hello"
  - [x] This validates the full stack: `podman-docker` alias -> Podman socket -> rootless container execution
  - [x] Note: this test will be slow (pulls alpine image inside the sandbox) — consider using `testing.Short()` skip or separate build tag

## Dev Notes

### This Is a Validation + Bug Fix Story

The Podman installation and Docker Compose setup already exist from Story 1.5 (`embed/Dockerfile.tmpl` lines 60-76). The Podman API socket startup exists in `embed/entrypoint.sh` (lines 97-102). This story:

1. **Fixes bugs**: `embed/entrypoint.sh` is missing `DOCKER_HOST` export, `XDG_RUNTIME_DIR` export, `podman system migrate`, docker.sock symlink, and socket readiness wait — all present in the legacy `scripts/entrypoint.sh` (lines 67-95) but not ported to the Go rewrite's entrypoint
2. **Validates**: the complete Podman stack works end-to-end via integration tests
3. **May need to add**: subordinate UID/GID ranges (`/etc/subuid`, `/etc/subgid`) if missing — rootless Podman requires these

### Critical Bug: Missing DOCKER_HOST in embed/entrypoint.sh

The legacy `scripts/entrypoint.sh` (lines 78-80) exports `DOCKER_HOST` so Docker Compose and Docker SDK clients can find the Podman socket:
```bash
export DOCKER_HOST="unix://${XDG_RUNTIME_DIR}/podman/podman.sock"
```

The Go rewrite's `embed/entrypoint.sh` does NOT set `DOCKER_HOST`. Without it:
- `docker compose up` will fail (looks for `/var/run/docker.sock` by default)
- Docker SDK clients won't find the Podman socket
- Any tool expecting the standard Docker socket path will fail

The legacy entrypoint also has these features missing from the Go rewrite entrypoint:
- `XDG_RUNTIME_DIR` export (legacy line 69)
- `podman system migrate` (legacy line 71) — required for rootless initialization
- `podman info` health check (legacy line 72)
- `docker.sock` symlink (legacy line 81) — for tools that hardcode the socket path
- Socket readiness wait loop (legacy lines 94+) — prevents race condition where agent starts before socket is ready

### Entrypoint Function Reordering Required

Current order in `embed/entrypoint.sh` (lines 104-110):
```
align_uid_gid -> chown_volumes -> merge_mcp_config -> set_testcontainers_socket -> start_healthcheck_poller -> start_podman_socket -> exec
```

**Problem**: `set_testcontainers_socket` runs BEFORE `start_podman_socket`. The `TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE` path is constructed correctly, but `DOCKER_HOST` should be exported in `start_podman_socket` before `set_testcontainers_socket` runs. The healthcheck poller also doesn't need to start before the socket.

**Required order**:
```
align_uid_gid -> chown_volumes -> merge_mcp_config -> start_podman_socket -> set_testcontainers_socket -> start_healthcheck_poller -> exec
```

### Rootless Podman Requirements

Rootless Podman needs:
1. `/etc/subuid` and `/etc/subgid` entries for the sandbox user (subordinate UID/GID ranges)
2. `uidmap` and `slirp4netns` packages (already installed: `Dockerfile.tmpl` line 61)
3. `newuidmap`/`newgidmap` setuid binaries (provided by `uidmap` package)

Check if `useradd -m -u 1000 -g sandbox -s /bin/bash sandbox` (line 13) automatically created `/etc/subuid` and `/etc/subgid` entries. On Ubuntu 24.04, `useradd` auto-allocates subordinate ranges if `SUB_UID_MIN`/`SUB_UID_MAX` are set in `/etc/login.defs`. If entries are not present, add them manually in the Dockerfile template.

### Integration Test Structure

Follow the same pattern established in Story 3.1:
- `integration/integration_test.go` — shared test helpers (already exists, created in Story 3.1)
- `integration/isolation_test.go` — existing isolation tests (Story 3.1)
- `integration/podman_test.go` — NEW: Podman/Docker alias tests for this story

Use the existing `startTestContainer()` helper from `integration/integration_test.go`. Tests should exec commands inside the running test container and verify output.

**Important**: Integration tests start the container with the entrypoint overridden to `tail -f /dev/null` (see Story 3.1 review finding). For Podman socket tests, the container must run the actual entrypoint so the socket is started. You may need a separate test container setup that uses the real entrypoint, or manually call the socket setup steps inside the container.

**testcontainers-go** is already in `go.mod` as an indirect dependency (v0.41.0) from Story 3.1.

### No Privileged Mode Verification (AC #4)

`internal/docker/run.go` line 23 assembles args as: `["run", "-it", "--rm", ...]`. There is no `--privileged` flag and no `-v /var/run/docker.sock:/var/run/docker.sock` mount. This is correct by design. The integration test should verify this at the container level via Docker inspect API.

### vfs Storage Driver — By Design, Not a Bug

The architecture explicitly documents: "`vfs` storage driver performs a full filesystem copy per layer — inner image pulls and builds will be slower and use more disk than overlay2. Acceptable for a development tool where agents pull a handful of images." This is configured at `embed/Dockerfile.tmpl` line 69. Do NOT change it to overlay2 (which would require privileged mode or fuse-overlayfs in nested containers).

### Docker Compose Installation

Docker Compose v2 is installed as a standalone binary at `/usr/local/bin/docker-compose` and symlinked as a CLI plugin at `/usr/local/lib/docker/cli-plugins/docker-compose` (Dockerfile.tmpl lines 72-76). The `docker compose` command routes through `podman compose` due to `podman-docker`. Verify this works with the Podman socket after `DOCKER_HOST` is set.

### Testcontainers Environment Variables

Already set in `embed/Dockerfile.tmpl` (lines 104-105):
```
ENV TESTCONTAINERS_RYUK_DISABLED=true
ENV TESTCONTAINERS_HOST_OVERRIDE=localhost
```

`TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE` is set at runtime in `embed/entrypoint.sh` function `set_testcontainers_socket()` (line 94) because the socket path depends on the sandbox user's UID (which may change at runtime via UID/GID alignment).

### Project Structure Notes

- **Modified**: `embed/entrypoint.sh` (add DOCKER_HOST, XDG_RUNTIME_DIR, podman migrate, socket wait, reorder functions)
- **Possibly Modified**: `embed/Dockerfile.tmpl` (add /etc/subuid, /etc/subgid if not auto-created by useradd)
- **Created**: `integration/podman_test.go` (Podman/Docker alias integration tests)
- No new `internal/` packages
- No changes to `cmd/` layer

### Previous Story Intelligence

**From Story 3.1 (done) — most recent:**
- Integration test infrastructure created: `integration/integration_test.go` (shared helpers) and `integration/isolation_test.go`
- testcontainers-go added as dependency (v0.41.0 indirect)
- Pattern: build test sandbox image -> start container -> exec commands -> assert output
- Review findings: tests run as root not sandbox user — use `su -c` or `gosu` to exec as sandbox user where needed
- Review findings: parallel subtests may race on shared temp dirs — use unique temp dirs per test
- Review findings: `startTestContainer` overrides entrypoint with `tail -f /dev/null` — Podman socket won't be started in that mode

**From Story 1.5 (done):**
- Podman installation block in `embed/Dockerfile.tmpl` lines 60-76
- `embed/entrypoint.sh` created with Podman socket start at lines 97-102
- Docker Compose installed and symlinked as CLI plugin
- All embedded scripts COPY'd and chmod'd at lines 16-19

**Legacy reference** (`scripts/entrypoint.sh` lines 67-95): Contains the complete Podman initialization sequence that was NOT fully ported to the Go rewrite's `embed/entrypoint.sh`. This is the primary source of bugs this story must fix.

### Git Intelligence

Recent commits follow `feat: implement story X-Y <description>` pattern on `feat/go-rewrite` branch. All previous stories (1.1-1.8, 2.1, 2.2, 3.1) completed. Key context:
- `embed/entrypoint.sh` last modified in Story 1.5 commit (2f1ed91) — Podman socket startup added then, but incomplete
- `embed/Dockerfile.tmpl` lines 60-76 — Podman packages, config, and Docker Compose already present
- `integration/integration_test.go` created in Story 3.1 (db7d08b) — reuse this infrastructure

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Epic 4, Story 4.1, lines 645-678]
- [Source: _bmad-output/planning-artifacts/architecture.md — Inner Container Runtime, lines 169-178]
- [Source: _bmad-output/planning-artifacts/architecture.md — Network Isolation, lines 201-204]
- [Source: _bmad-output/planning-artifacts/architecture.md — Container Lifecycle, Entrypoint Startup Sequence, lines 218-231]
- [Source: _bmad-output/planning-artifacts/architecture.md — Testcontainers compatibility, line 177]
- [Source: _bmad-output/planning-artifacts/prd.md — FR26, FR27, FR28, FR34, FR35, FR36, NFR3, NFR4, NFR7]
- [Source: embed/Dockerfile.tmpl — Podman installation (lines 60-76), Testcontainers env (lines 104-105)]
- [Source: embed/entrypoint.sh — Podman socket start (lines 97-102), main sequence (lines 104-110)]
- [Source: scripts/entrypoint.sh — Legacy Podman init with DOCKER_HOST (lines 67-95) — reference for missing features]
- [Source: internal/docker/run.go — No --privileged, no Docker socket mount (lines 22-38)]
- [Source: _bmad-output/implementation-artifacts/3-1-git-push-blocking-and-filesystem-isolation.md — integration test patterns and review findings]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None — no blockers or debug issues encountered.

### Completion Notes List

- **Task 1**: Fixed `embed/entrypoint.sh` — added `XDG_RUNTIME_DIR` export, `DOCKER_HOST` export, `podman system migrate` call, `docker.sock` symlink, socket readiness wait loop (5s poll), and reordered main sequence to: `align_uid_gid` -> `chown_volumes` -> `merge_mcp_config` -> `start_podman_socket` -> `set_testcontainers_socket` -> `start_healthcheck_poller` -> exec. All features ported from legacy `scripts/entrypoint.sh`.
- **Task 2**: Added explicit `/etc/subuid` and `/etc/subgid` entries (`sandbox:100000:65536`) to `embed/Dockerfile.tmpl` after user creation. Confirmed `uidmap` and `slirp4netns` already installed.
- **Task 3**: Created `integration/podman_test.go` with 8 subtests covering: docker --version returns podman, vfs storage driver, docker compose version, DOCKER_HOST set, socket exists and owned by sandbox, TESTCONTAINERS_RYUK_DISABLED, HOST_OVERRIDE, DOCKER_SOCKET_OVERRIDE. Uses new `startTestContainerWithEntrypoint()` helper that runs the real entrypoint with socket readiness wait.
- **Task 4**: Added `TestContainerNotPrivileged` with 2 subtests: inspects container via Docker API to verify `Privileged=false` and no `docker.sock` mount. Confirmed `internal/docker/run.go` does not include `--privileged`.
- **Task 5**: Added `TestPodmanRunContainer` smoke test that runs `docker run --rm alpine:latest echo hello` inside the sandbox, validating the full Podman stack end-to-end.

### File List

- `embed/entrypoint.sh` — modified (DOCKER_HOST, XDG_RUNTIME_DIR, podman migrate, socket wait, reorder)
- `embed/Dockerfile.tmpl` — modified (added /etc/subuid, /etc/subgid entries)
- `integration/podman_test.go` — created (Podman/Docker alias integration tests)
- `go.mod` — modified (docker/docker promoted to direct dependency)
- `go.sum` — modified (updated checksums)

### Review Findings

- [x] [Review][Decision] Rootless Podman may need `--security-opt` on outer container — added `--security-opt seccomp=unconfined` and `--security-opt apparmor=unconfined` to `internal/docker/run.go`
- [x] [Review][Patch] Socket readiness loop silently continues on timeout — added warning log after loop [embed/entrypoint.sh]
- [x] [Review][Patch] Hardcoded UID 1000 in test wait strategy and stat assertion — extracted to `podmanSocketPath` constant with documenting comment [integration/podman_test.go]
- [x] [Review][Patch] Unsafe type assertion in TestContainerNotPrivileged — converted to comma-ok pattern [integration/podman_test.go]
- [x] [Review][Patch] `podman system migrate` errors fully silenced — now logs stderr output [embed/entrypoint.sh]
- [x] [Review][Defer] `docker/docker` +incompatible in main require block [go.mod] — deferred, pre-existing; test-only dep could be isolated
- [x] [Review][Defer] Background Podman PID not tracked for cleanup [embed/entrypoint.sh:110] — deferred, pre-existing; tini reaps orphans
- [x] [Review][Defer] `AGENT_CMD` injection via shell expansion [embed/entrypoint.sh:140] — deferred, pre-existing pattern

### Change Log

- 2026-04-09: Story implementation complete — fixed entrypoint.sh Podman initialization bugs, added subuid/subgid entries, created comprehensive integration tests (11 tests across 3 test functions)
