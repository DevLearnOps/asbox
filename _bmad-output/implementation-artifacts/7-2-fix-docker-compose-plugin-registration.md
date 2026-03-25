# Story 7.2: Fix Docker Compose Plugin Registration for Podman

Status: review

## Story

As a developer,
I want `docker compose` (as a subcommand) to work inside the sandbox,
So that the agent can run multi-service applications using standard Docker Compose commands.

## Acceptance Criteria

1. **Given** a sandbox is running, **When** the agent runs `docker compose version`, **Then** the Docker Compose version is displayed (no "daemon not running" error).
2. **Given** a sandbox with a valid docker-compose.yml, **When** the agent runs `docker compose up -d`, **Then** the services start successfully using Podman as the backend.
3. **Given** `docker compose` works, **When** the agent runs the standalone `docker-compose` command, **Then** that also works (backwards compatibility).

## Tasks / Subtasks

- [x] Task 1: Add Docker CLI plugin symlink to Dockerfile.template (AC: #1, #3)
  - [x] 1.1 After the Docker Compose v2 binary download block (line 41-45), add a `RUN` to create the CLI plugin directory and symlink: `mkdir -p /usr/local/lib/docker/cli-plugins && ln -s /usr/local/bin/docker-compose /usr/local/lib/docker/cli-plugins/docker-compose`
  - [x] 1.2 Verify the symlink target matches the download path (`/usr/local/bin/docker-compose`)
  - [x] 1.3 Verify `podman-docker` (installed via apt in the Podman block) routes `docker compose` to plugins at `/usr/local/lib/docker/cli-plugins/`
- [x] Task 2: Add Story 7.2 tests to `tests/test_sandbox.sh` (AC: #1, #2, #3)
  - [x] 2.1 Test: generated Dockerfile contains `cli-plugins` directory creation
  - [x] 2.2 Test: generated Dockerfile contains symlink from `cli-plugins/docker-compose` to `/usr/local/bin/docker-compose`
  - [x] 2.3 Test: the plugin symlink appears AFTER the docker-compose binary download in the generated Dockerfile
  - [x] 2.4 Test: no Dockerfile changes when Docker Compose is not configured (if applicable -- Compose is currently always installed)
- [x] Task 3: Verify no regressions in existing Podman/Compose tests (AC: #1, #2, #3)
  - [x] 3.1 Run full test suite and confirm all existing tests pass
  - [x] 3.2 Verify existing Story 4-2 compose fixture tests are not affected

## Dev Notes

### Root Cause

Docker Compose v2 is installed as a standalone binary at `/usr/local/bin/docker-compose`. This supports the `docker-compose` (hyphenated) invocation. However, the `docker compose` (space) subcommand requires the binary to be registered as a Docker CLI plugin at a specific path. The `podman-docker` package provides a `docker` shim that routes to Podman, and it looks for CLI plugins at `/usr/local/lib/docker/cli-plugins/`. Without the symlink, `docker compose` fails with "docker daemon not running" because the shim cannot find the plugin and falls back to trying the Docker daemon socket.

### What Already Exists

- **Dockerfile.template:41-45** -- Docker Compose v2 standalone binary download:
  ```dockerfile
  # Docker Compose v2 standalone binary (provides service-name DNS via Netavark bridge networking)
  RUN ARCH="$(uname -m)" && \
      curl -fsSL "https://github.com/docker/compose/releases/download/v2.35.1/docker-compose-linux-${ARCH}" \
        -o /usr/local/bin/docker-compose && \
      chmod +x /usr/local/bin/docker-compose
  ```
- **Dockerfile.template:27** -- `podman-docker` is installed as part of the Podman block (provides the `docker` -> `podman` shim)
- **Architecture (line 146):** "`docker compose` calls route through `podman compose`" -- the architecture assumes this works, but it requires plugin registration
- **tests/test_sandbox.sh:2910-2911** -- Existing tests verify `docker-compose` binary is in generated Dockerfile and fetched from GitHub releases
- **tests/test_sandbox.sh:2322-2367** -- Story 4-2 compose fixture tests verify `docker-compose.yml` fixture exists with correct services

### Implementation Approach

Add a single `RUN` command after the existing Docker Compose download block in `Dockerfile.template`:

```dockerfile
# Register Docker Compose as a CLI plugin (enables `docker compose` subcommand via podman-docker)
RUN mkdir -p /usr/local/lib/docker/cli-plugins && \
    ln -s /usr/local/bin/docker-compose /usr/local/lib/docker/cli-plugins/docker-compose
```

This is a minimal, targeted change. The standalone `docker-compose` binary continues to work (AC #3), and the symlink enables `docker compose` as a subcommand (AC #1, #2).

### Architecture Compliance

- Change is within the existing Docker Compose section of `Dockerfile.template` -- no structural changes
- Template processing in `sandbox.sh` is unaffected -- no new conditional blocks or placeholders
- Content hash will change because `Dockerfile.template` is a hash input -- correctly triggers rebuild
- No changes to `sandbox.sh` or `scripts/entrypoint.sh`

### File Modifications Required

| File | Change |
|------|--------|
| `Dockerfile.template` | Add `RUN mkdir -p ... && ln -s ...` after Docker Compose download block (after line 45) |
| `tests/test_sandbox.sh` | Add Story 7.2 tests verifying CLI plugin symlink in generated Dockerfile |

### Test Strategy

Follow existing Story 4-1/7-1 test patterns using `assert_contains` on `.sandbox-dockerfile` content:

1. Build with default config (Compose is always installed, not conditional)
2. Read `.sandbox-dockerfile` content
3. Assert `cli-plugins` directory creation appears in the generated Dockerfile
4. Assert symlink to `docker-compose` appears in the generated Dockerfile
5. Assert the plugin registration line appears AFTER the compose binary download (use `grep -n` ordering pattern from Story 7.1 tests)

Test naming convention: `7.2: <description>`

### Anti-Patterns to Avoid

- Do NOT replace the standalone binary with a plugin-only install -- both invocation styles must work
- Do NOT modify the Podman configuration or `containers.conf` -- the fix is purely a filesystem symlink
- Do NOT add a new conditional block (`IF_COMPOSE`) -- Docker Compose is always installed
- Do NOT modify `sandbox.sh` or `scripts/entrypoint.sh` -- this is a Dockerfile-only change
- Do NOT use `podman-compose` (Python package) instead of Docker Compose v2 -- the architecture explicitly chose Docker Compose v2 for Netavark bridge networking compatibility

### Previous Story Intelligence

From Story 7-1 (Playwright system deps):
- Changes to `Dockerfile.template` are straightforward -- add lines in the right block
- Tests use `assert_contains`/`assert_not_contains` on `.sandbox-dockerfile` content
- Use `grep -n` for ordering assertions (system deps before install)
- Full test suite was 482 assertions after 7-1
- Commit format: `feat: <description> (story 7-2)`
- Review feedback pattern: `with review fixes` in commit message

From Story 4-2 (building and running inner containers):
- Docker Compose v2 chosen for Netavark bridge networking (service-name DNS resolution)
- `docker-compose.yml` fixtures exist at `tests/fixtures/docker-compose.yml`
- Story 4-2 tests verify fixture structure but do NOT test `docker compose` subcommand invocation

### Git Conventions

- Commit format: `feat: register Docker Compose as CLI plugin for podman-docker (story 7-2)`
- Include `with review fixes` if code review feedback is incorporated

### Project Structure Notes

- `Dockerfile.template` is at project root -- human-readable template, NOT a valid Dockerfile
- `.sandbox-dockerfile` is the generated/resolved Dockerfile (used by tests, gitignored)
- Docker Compose binary is always installed (not behind a conditional block)
- The plugin path `/usr/local/lib/docker/cli-plugins/` is standard for Docker CLI plugins

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 7.2] Story requirements and acceptance criteria
- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-03-25.md] Root cause analysis and fix approach
- [Source: _bmad-output/planning-artifacts/architecture.md#Inner Container Runtime] Podman decision, Docker Compose routing expectation
- [Source: Dockerfile.template:41-45] Current Docker Compose download block
- [Source: Dockerfile.template:20-28] Podman and podman-docker installation block
- [Source: tests/test_sandbox.sh:2910-2911] Existing Docker Compose tests
- [Source: tests/test_sandbox.sh:2322-2367] Story 4-2 compose fixture tests
- [Source: _bmad-output/implementation-artifacts/7-1-fix-playwright-system-dependencies.md] Previous story learnings and test patterns

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None required — straightforward implementation with no issues encountered.

### Completion Notes List

- Added `RUN mkdir -p /usr/local/lib/docker/cli-plugins && ln -s /usr/local/bin/docker-compose /usr/local/lib/docker/cli-plugins/docker-compose` after Docker Compose v2 download block in Dockerfile.template
- This registers the standalone Docker Compose binary as a Docker CLI plugin, enabling `docker compose` (space) subcommand via podman-docker shim
- Standalone `docker-compose` (hyphenated) continues to work unchanged (backwards compatibility)
- Added 4 new tests (7.2-2.1 through 7.2-2.3 plus build exit code) verifying CLI plugin directory creation, symlink content, and ordering
- Task 2.4 confirmed N/A — Compose is always installed, no conditional block exists
- Full test suite: 486/486 passed, 0 failed (up from 482 after Story 7-1)
- No regressions in existing Story 4-2 compose fixture tests

### Change Log

- 2026-03-25: Implemented Story 7.2 — Docker Compose CLI plugin registration via symlink in Dockerfile.template, added 4 tests

### File List

- `Dockerfile.template` — Added RUN command for CLI plugin directory and symlink (lines 47-49)
- `tests/test_sandbox.sh` — Added Story 7.2 test section with 4 assertions (lines 3652-3685)
