# Story 4.2: Inner Container Building and Orchestration

Status: done

## Story

As a developer,
I want the agent to build Docker images and run multi-service applications with Docker Compose,
so that the agent can test full-stack applications inside the sandbox.

## Acceptance Criteria

1. **Given** a sandbox is running with Podman available, **When** the agent runs `docker build -t myapp .` with a valid Dockerfile, **Then** the image builds successfully using Podman's rootless build.

2. **Given** the agent has a docker-compose.yml defining multiple services, **When** the agent runs `docker compose up`, **Then** all services start and can communicate via aardvark-dns name resolution.

3. **Given** an inner container exposes port 3000, **When** the agent runs `curl localhost:3000` from the sandbox, **Then** the request reaches the inner container's application (FR28).

4. **Given** inner containers are running, **When** attempting to reach their ports from outside the sandbox, **Then** the ports are not reachable — inner containers are on a private network bridge (NFR3, FR34, FR36).

5. **Given** Docker Compose v2 is installed, **When** the agent runs `docker compose version`, **Then** the version is displayed (plugin registered at `/usr/local/lib/docker/cli-plugins/docker-compose`).

## Tasks / Subtasks

- [x] Task 1: Integration test — docker build inside sandbox (AC: #1)
  - [x] Write a minimal Dockerfile (e.g., `FROM alpine:latest\nRUN echo built\nCMD ["echo","hello"]`) as a string in the test
  - [x] Exec `docker build -t testapp .` inside the sandbox container as user `sandbox`
  - [x] Verify exit code 0 and image appears in `docker images`
- [x] Task 2: Integration test — docker compose multi-service with DNS (AC: #2)
  - [x] Create a docker-compose.yml with two services (e.g., web server + curl client) inside the container
  - [x] Exec `docker compose up -d` as user `sandbox`
  - [x] Verify both services start (`docker compose ps` shows running)
  - [x] Verify aardvark-dns name resolution: one service can reach the other by service name
- [x] Task 3: Integration test — inner container port reachability from sandbox (AC: #3)
  - [x] Run an inner container exposing port 3000 (e.g., `python3 -m http.server 3000` or `nc -l -p 3000`)
  - [x] Exec `curl -s localhost:3000` from the sandbox and verify response
- [x] Task 4: Integration test — inner container ports NOT reachable from outside (AC: #4)
  - [x] With inner container running on port 3000, attempt to reach it from the host/test runner
  - [x] Verify connection is refused or times out — private network bridge isolation
- [x] Task 5: Integration test — docker compose version (AC: #5)
  - [x] Already partially covered in `podman_test.go` (`docker_compose_version` subtest)
  - [x] Add verification that plugin path `/usr/local/lib/docker/cli-plugins/docker-compose` exists

## Dev Notes

### Architecture Constraints

- **No code changes to Go CLI expected** — this story validates that the infrastructure from stories 1.5 and 4.1 works correctly for inner container operations. All work is integration tests.
- **Podman is the inner runtime** — all `docker` commands route through `podman-docker` alias. Agent is unaware it's using Podman.
- **vfs storage driver** — image builds will be slower (full filesystem copy per layer). This is by design, not a bug. Do not attempt to switch to overlay2.
- **Rootless Podman** — no `--privileged` on the outer container in production. However, integration tests use `Privileged: true` in `HostConfigModifier` to allow nested container operations in CI/Docker Desktop environments.
- **aardvark-dns** — provides service name DNS resolution for Podman networks, making `docker compose` service-to-service communication work.
- **netavark** — networking backend for Podman, configured in `/etc/containers/containers.conf`.

### Testing Approach

- **All tests go in `integration/inner_container_test.go`** — this file is already listed in the architecture but does not exist yet. Create it.
- **Use `startTestContainerWithEntrypoint()`** from `integration/podman_test.go` — this starts the real entrypoint so the Podman socket is initialized. Do NOT use `startTestContainer()` as it overrides the entrypoint and Podman won't start.
- **Use `execAsUser(ctx, t, container, "sandbox", cmd)`** for all commands — tests must run as the unprivileged `sandbox` user, not root.
- **Use `t.Parallel()` for independent subtests** within each test function.
- **Use `testing.Short()` skip pattern** at the top of each test function.
- **Share one container per test function** — building images is slow; start one container with entrypoint, then run multiple subtests against it.
- **Write temp files inside container** via `docker exec` / heredoc rather than volume mounts — keeps tests self-contained.

### File Structure

- **New file:** `integration/inner_container_test.go` — all tests for this story
- **No other files should be created or modified**

### Docker Compose Plugin Location

Docker Compose v2 is installed at two paths (from `embed/Dockerfile.tmpl` lines 72-76):
- `/usr/local/bin/docker-compose` — standalone binary
- `/usr/local/lib/docker/cli-plugins/docker-compose` — CLI plugin symlink

The plugin symlink enables `docker compose` (space, not hyphen) syntax via Podman's docker alias.

### Previous Story Intelligence (4.1)

**Critical bugs fixed in 4.1 that affect this story:**
- `DOCKER_HOST` must be exported before any docker/compose commands work
- `XDG_RUNTIME_DIR` must be set for Podman socket operations
- `podman system migrate` runs at startup for rootless initialization
- Socket readiness wait loop prevents race conditions — entrypoint waits up to 30s for socket

**Entrypoint function order (already correct):**
`align_uid_gid` -> `chown_volumes` -> `merge_mcp_config` -> `start_podman_socket` -> `set_testcontainers_socket` -> `start_healthcheck_poller` -> `exec`

**Integration test patterns from 4.1:**
- `startTestContainerWithEntrypoint()` uses `Privileged: true` and waits for socket at `/run/user/1000/podman/podman.sock`
- `execAsUser()` sources `/etc/profile.d/sandbox-env.sh` to pick up DOCKER_HOST and other env vars
- Parallel subtests within shared container work well

**Security opts added in 4.1:** `--security-opt seccomp=unconfined` and `--security-opt apparmor=unconfined` in `internal/docker/run.go` for rootless Podman compatibility.

### Testing Inner Container Port Isolation (AC #4)

The outer container does NOT expose ports to the host. Inner containers run on Podman's private bridge network. To test isolation:
- Option A: Inspect the outer container from the test runner — verify no published ports map to inner container ports.
- Option B: Use the Docker API from the test to attempt a connection to the outer container's IP on port 3000 — verify it fails.

The simpler approach is Option A — inspect the testcontainers container and verify no port mappings exist for inner container ports.

### Git Intelligence

Recent commits show a consistent pattern:
- One commit per story with `feat: implement story X-Y description` format
- Co-authored with Claude Opus 4.6
- Tests are written alongside implementation code
- Go 1.25.0, testcontainers-go v0.41.0, Docker client v28.5.2

### Project Structure Notes

- Alignment: `integration/inner_container_test.go` matches the architecture's defined test structure
- Package: `package integration` (same as all other integration test files)
- Imports: reuse `startTestContainerWithEntrypoint`, `execAsUser`, `execInContainer` from `integration_test.go` and `podman_test.go`

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Epic 4, Story 4.2]
- [Source: _bmad-output/planning-artifacts/architecture.md — Inner Container Runtime Decision]
- [Source: _bmad-output/planning-artifacts/architecture.md — Entrypoint Startup Sequence]
- [Source: _bmad-output/planning-artifacts/architecture.md — Integration Test Suite: inner_container_test.go]
- [Source: _bmad-output/planning-artifacts/prd.md — FR26, FR27, FR28, FR34, FR36, NFR3]
- [Source: _bmad-output/implementation-artifacts/4-1-podman-installation-and-docker-alias.md — Dev Notes]
- [Source: embed/Dockerfile.tmpl lines 60-76 — Podman + Docker Compose installation]
- [Source: integration/podman_test.go — startTestContainerWithEntrypoint(), existing docker compose version test]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- Discovered missing `iproute2` and `iptables` packages via debug test tracing netavark's "No such file or directory" error during custom network container start
- Alpine busybox does not include `httpd`; switched to `nc`-based HTTP server for port reachability test
- Confirmed slirp4netns handles default network port mapping while netavark handles custom bridge networks (compose)

### Completion Notes List

- Task 1: TestDockerBuild — writes minimal Dockerfile via heredoc, builds with `docker build -t testapp .` as sandbox user, verifies image in `docker images` output
- Task 2: TestDockerComposeMultiService — creates two-service compose (web + client alpine containers), verifies compose up/ps/dns resolution via `nslookup web` from client container through aardvark-dns
- Task 3: TestInnerContainerPorts/reachable_from_sandbox — runs nc-based HTTP server in alpine container with `-p 3000:3000`, curls from sandbox with retry loop
- Task 4: TestInnerContainerPorts/not_reachable_from_outside — inspects outer container via Docker API, verifies zero published port bindings despite inner container port mapping
- Task 5: TestDockerComposePluginPath — verifies `/usr/local/lib/docker/cli-plugins/docker-compose` symlink exists
- Bug fix: Added `iproute2` and `iptables` to Dockerfile template — netavark requires both for custom bridge network setup (compose networks, service DNS)

### File List

- integration/inner_container_test.go (new) — all 5 integration tests for this story
- embed/Dockerfile.tmpl (modified) — added iproute2 and iptables packages for netavark custom network support

### Change Log

- 2026-04-09: Implemented all 5 tasks for story 4.2. Created integration/inner_container_test.go with TestDockerBuild, TestDockerComposeMultiService, TestInnerContainerPorts, TestDockerComposePluginPath. Fixed missing iproute2/iptables in Dockerfile template that blocked netavark custom networks.

### Review Findings

- [x] [Review][Defer] nc-based HTTP server has race window between connections [integration/inner_container_test.go:121-124] — deferred, pre-existing pattern; BusyBox nc exits after each connection, leaving a brief unbound window before the while-loop restarts it; retry loop mitigates but fragile in slow CI
- [x] [Review][Defer] Docker Compose binary and apt packages are unpinned [embed/Dockerfile.tmpl:61,72] — deferred, pre-existing pattern; all packages in this RUN block use latest versions, consistent with existing project convention
- [x] [Review][Defer] No context timeout on integration tests [integration/inner_container_test.go] — deferred, pre-existing pattern; all integration test files use context.Background() with no deadline, consistent across the test suite
