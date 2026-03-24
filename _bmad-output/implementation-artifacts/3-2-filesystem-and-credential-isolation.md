# Story 3.2: Filesystem and Credential Isolation

Status: review

## Story

As a developer,
I want the sandbox to restrict filesystem access to declared mounts only and prevent access to host credentials,
so that the agent cannot accidentally read or modify anything outside the project scope.

## Acceptance Criteria

1. **Given** a config declares only `[{source: ".", target: "/workspace"}]` as a mount
   **When** the agent attempts to access paths outside `/workspace` (e.g., `/home/user/.ssh`, `/home/user/.aws`)
   **Then** those paths do not exist inside the container -- no host home directory, SSH keys, or cloud credentials are accessible (NFR1, NFR6)

2. **Given** a sandbox is running
   **When** the agent attempts any operation outside the declared boundaries
   **Then** the system returns standard CLI error codes (file not found, permission denied) -- not sandbox-specific errors (FR37)

3. **Given** the container is launched
   **When** inspecting the environment
   **Then** only explicitly declared secrets are present as environment variables -- no host credentials leak through

## Tasks / Subtasks

- [x] Task 1: Verify container isolation is inherent (AC: #1, #2)
  - [x] 1.1 Confirm that Docker container isolation already prevents host filesystem access by default -- paths like `/home/user/.ssh` and `/home/user/.aws` do not exist inside the container because Docker containers have their own filesystem root (the image)
  - [x] 1.2 Confirm that only explicitly declared mounts (`-v` flags) bridge host paths into the container -- no implicit home directory, `.ssh`, `.aws`, or other host paths are mounted
  - [x] 1.3 Confirm that standard CLI error codes are returned when accessing non-existent paths (ENOENT / "No such file or directory") -- these are standard Linux errors, not sandbox-specific
- [x] Task 2: Verify environment variable isolation (AC: #3)
  - [x] 2.1 Confirm that `docker run` with explicit `-e` flags does NOT inherit the host's full environment -- only variables passed via `-e KEY` or `-e KEY=VALUE` are available inside the container
  - [x] 2.2 Confirm that `cmd_run()` in sandbox.sh only passes: `SANDBOX_AGENT`, declared secrets (via `-e SECRET_NAME`), and declared env vars (via `-e KEY=VALUE`) -- no `--env-file` or `-e` with host env passthrough
- [x] Task 3: Write verification tests (AC: #1, #2, #3)
  - [x] 3.1 Test: `cmd_run()` docker run command does NOT contain `--privileged` flag
  - [x] 3.2 Test: `cmd_run()` docker run command does NOT mount Docker socket (`/var/run/docker.sock`)
  - [x] 3.3 Test: `cmd_run()` docker run command does NOT contain `--env-file` flag
  - [x] 3.4 Test: `cmd_run()` docker run command contains ONLY the expected `-e` flags (SANDBOX_AGENT + declared secrets + declared env vars) -- no extra environment variables
  - [x] 3.5 Test: `cmd_run()` docker run command contains ONLY the expected `-v` flags (declared mounts) -- no extra volume mounts
  - [x] 3.6 Test: with zero mounts configured, docker run command has zero `-v` flags
  - [x] 3.7 Test: with zero secrets and zero env vars, docker run has only `-e SANDBOX_AGENT=<agent>` -- no other env flags
- [x] Task 4: Document isolation guarantees in test comments (AC: #1, #2, #3)
  - [x] 4.1 Add a test section header explaining that filesystem and credential isolation is inherent to Docker's containerization model
  - [x] 4.2 Document in test comments that the verification tests confirm sandbox.sh does not accidentally break this isolation (no `--privileged`, no host socket, no env leakage)

## Dev Notes

### Core Insight: Isolation Is Already Provided by Docker

This story's requirements (FR32, FR33, NFR1, NFR6) are **inherently satisfied by Docker's containerization model**. A Docker container:
- Has its own filesystem root (the image) -- host paths do not exist unless explicitly mounted via `-v`
- Does not inherit the host's environment variables -- only variables passed via `-e` are available
- Runs in its own PID, network, and mount namespaces

The sandbox does NOT need to add any new isolation mechanisms. The implementation task is to **verify and test** that `sandbox.sh` does not accidentally break Docker's built-in isolation by:
1. Adding `--privileged` (would weaken namespace isolation)
2. Mounting the Docker socket (would give container access to host Docker)
3. Using `--env-file` or passing extra `-e` flags beyond declared secrets/env vars
4. Adding implicit mounts (home directory, `.ssh`, `.aws`, etc.)

### What NOT to Change

- **sandbox.sh** -- the `cmd_run()` function already correctly assembles only declared mounts, secrets, and env vars. No code changes needed.
- **Dockerfile.template** -- already runs as non-root `sandbox` user. No changes needed.
- **scripts/entrypoint.sh** -- no changes needed.
- **scripts/git-wrapper.sh** -- no changes needed (covered by Story 3-1).

### What TO Do

Write **negative assertion tests** that verify `cmd_run()` output does NOT contain isolation-breaking flags. These tests serve as regression guards -- if someone later adds `--privileged` or an implicit mount, the tests catch it.

### Test Pattern

Follow the established test pattern from Stories 2-1 through 3-1. The existing tests capture the `docker run` command output by mocking `docker` with a script that echoes its arguments. Apply the same pattern here.

Current test infrastructure (from Story 3-1 analysis):
- **304 test assertions** currently passing
- Test helpers: `assert_exit_code`, `assert_contains`, `assert_not_contains`
- Docker mock pattern: creates a mock `docker` script that captures arguments, then verifies flags in the captured output
- Existing mount tests are around lines 1600-1800
- Existing env var tests are around lines 1200-1500

Example test structure:

```bash
# --- Filesystem and Credential Isolation Verification ---

# Test: docker run does NOT contain --privileged
output="$(cmd_run_capture ...)"
assert_not_contains "${output}" "--privileged" "docker run must not use --privileged flag"

# Test: docker run does NOT mount Docker socket
assert_not_contains "${output}" "/var/run/docker.sock" "docker run must not mount Docker socket"

# Test: docker run does NOT contain --env-file
assert_not_contains "${output}" "--env-file" "docker run must not use --env-file"

# Test: with no secrets and no env vars, only SANDBOX_AGENT env var present
# (use a config with empty secrets and env_vars arrays)
output_minimal="$(cmd_run_capture_minimal ...)"
env_count=$(echo "${output_minimal}" | grep -c '\-e ' || true)
assert_equals "1" "${env_count}" "only SANDBOX_AGENT env var when no secrets/env configured"

# Test: with no mounts, zero -v flags
output_no_mounts="$(cmd_run_capture_no_mounts ...)"
assert_not_contains "${output_no_mounts}" " -v " "no volume mounts when none configured"
```

### Previous Story Intelligence

From Story 3-1 (git push blocking):
- 304 test assertions currently passing
- Clean implementation with no debugging needed
- Git wrapper tests established the pattern for testing isolation boundaries
- The "negative assertion" pattern (testing what should NOT happen) was used for verifying `git stash push` passes through

From Story 2-1 (host directory mounts):
- Mount flag assembly is in `cmd_run()` around lines 404-424 of sandbox.sh
- Relative paths resolve against config file directory
- `-w` flag set to first mount target
- Tests verify exact `-v` flag format

From Story 2-2 (secret injection):
- Secret validation uses `${VAR+x}` check (declared, not non-empty)
- Secrets passed as `-e SECRET_NAME` (Docker reads from host env)
- Tests verify `-e` flag format for secrets vs env vars

### Git Recent Commits

```
5f87361 feat: implement git push blocking with review fixes (story 3-1)
b936ee9 feat: implement git wrapper passthrough and CLI toolchain with review fixes (story 2-4)
322198e feat: implement env var injection with validation and review fixes (story 2-3)
80b8cd1 feat: implement secret injection and validation with review fixes (story 2-2)
```

All Epic 2 stories and Story 3-1 are done. This is the second and final story in Epic 3.

### Architecture Compliance

- **FR32:** System prevents agent access to host filesystem beyond explicitly mounted paths -- satisfied by Docker containerization (no host paths exist unless mounted)
- **FR33:** System prevents agent access to host credentials, SSH keys, and cloud tokens not explicitly declared as secrets -- satisfied by Docker env isolation (no host env inherited) and no implicit mounts
- **FR37:** System returns standard CLI error codes when agent attempts operations outside the boundary -- satisfied by standard Linux ENOENT/EPERM errors for non-existent paths
- **NFR1:** No host credential, SSH key, or cloud token is accessible unless explicitly declared -- verified by testing no extra `-e` flags or mounts
- **NFR4:** Sandbox container runs without `--privileged` and without host Docker socket mount -- verified by negative assertion tests
- **NFR5:** Secrets injected as runtime env vars only, never written to filesystem -- already implemented in Story 2-2
- **NFR6:** Mounted paths limited to those explicitly declared in config -- verified by testing only declared `-v` flags present

### Threat Model Context

Per architecture doc: the security model protects against **accidental leakage from AI agents**, not deliberate adversarial exfiltration. The isolation tests verify that sandbox.sh does not accidentally weaken Docker's built-in isolation. The agent sees standard Linux errors (file not found, permission denied) when it tries to access paths that don't exist -- it doesn't know it's in a sandbox.

### Project Structure Notes

- No new files needed -- only `tests/test_sandbox.sh` is modified
- All tests follow existing patterns and use existing test helpers
- Test section should be placed after the git push blocking tests (Story 3-1) to maintain epic ordering

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 3, Story 3.2]
- [Source: _bmad-output/planning-artifacts/architecture.md - Isolation Mechanisms section, Threat Model Boundaries]
- [Source: _bmad-output/planning-artifacts/prd.md - FR32, FR33, FR37, NFR1, NFR4, NFR5, NFR6]
- [Source: _bmad-output/planning-artifacts/architecture.md - Project Structure & Boundaries, Host vs Container Boundary]
- [Source: _bmad-output/implementation-artifacts/3-1-git-push-blocking.md - Previous story patterns, test baseline of 304 assertions]
- [Source: sandbox.sh - cmd_run() lines 379-428, mount and env flag assembly]
- [Source: Dockerfile.template - Non-root sandbox user setup, entrypoint configuration]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

No debugging needed — clean implementation.

### Completion Notes List

- Tasks 1 & 2: Code inspection of `cmd_run()` (sandbox.sh:379-428) confirmed Docker's built-in isolation is not broken. No `--privileged`, no Docker socket mount, no `--env-file`, no implicit mounts or env vars. Only explicitly declared `-e` and `-v` flags are used.
- Task 3: Added 12 negative assertion tests (tests 305-316) verifying: no `--privileged` flag, no Docker socket mount, no `--env-file`, correct `-e` flag count with zero/mixed config, correct `-v` flag count with zero/single mount, and exact flag verification with secrets + env vars.
- Task 4: Added comprehensive test section header documenting that isolation is inherent to Docker containerization and tests serve as regression guards.
- All 320 tests pass (304 existing + 16 new), zero failures.
- Code review fix: Added 4 guard assertions ensuring docker_run_line is non-empty before negative assertions, preventing silent false-passes if the mock breaks.

### Change Log

- 2026-03-24: Added filesystem and credential isolation verification tests (16 assertions) to tests/test_sandbox.sh
- 2026-03-24: Code review fix — added docker run capture guards to prevent silent false-passes on empty mock output

### File List

- tests/test_sandbox.sh (modified — added isolation verification test section)
