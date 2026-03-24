# Story 1.6: Sandbox Run with TTY and Lifecycle

Status: review

## Story

As a developer,
I want `sandbox run` to launch my sandbox in interactive TTY mode with tini as PID 1,
so that I can interact with the agent and cleanly stop it with Ctrl+C.

## Acceptance Criteria

1. **Given** an image exists for the current config
   **When** the developer runs `sandbox run`
   **Then** the container launches in TTY mode (`-it`) with tini as the entrypoint and the configured agent as the foreground process

2. **Given** no image exists for the current config
   **When** the developer runs `sandbox run`
   **Then** the image is automatically built first, then the container launches (FR12)

3. **Given** a running sandbox session
   **When** the developer presses Ctrl+C
   **Then** tini forwards SIGTERM, the agent terminates, and the container exits cleanly with no orphaned containers or dangling networks (NFR14)

4. **Given** the config specifies `agent: claude-code`
   **When** the sandbox starts
   **Then** the entrypoint execs into `claude --dangerously-skip-permissions`

5. **Given** the config specifies `agent: gemini-cli`
   **When** the sandbox starts
   **Then** the entrypoint execs into `gemini`

## Tasks / Subtasks

- [x] Task 1: Implement minimal `scripts/entrypoint.sh` (AC: 4, 5)
  - [x] Read agent from `SANDBOX_AGENT` environment variable
  - [x] Map `claude-code` to `exec claude --dangerously-skip-permissions`
  - [x] Map `gemini-cli` to `exec gemini`
  - [x] Die with error for unknown agent values
  - [x] Follow `set -euo pipefail` convention
  - [x] Use `exec` to replace shell process (tini -> entrypoint -> exec agent)

- [x] Task 2: Implement `cmd_run()` in sandbox.sh (AC: 1, 2)
  - [x] Call `parse_config` to load configuration
  - [x] Call `cmd_build` to ensure image exists (auto-build if needed, covers FR12)
  - [x] Assemble `docker run` flags: `-it`, `--rm`, `-e SANDBOX_AGENT=${CFG_AGENT}`
  - [x] Execute `docker run` with `IMAGE_TAG` in foreground
  - [x] Print info message before launch: `"starting sandbox: ${IMAGE_TAG}"`
  - [x] Remove the current `info "not yet implemented"` stub

- [x] Task 3: Implement `--rm` and container cleanup (AC: 3)
  - [x] Use `--rm` flag on `docker run` to auto-remove container on exit
  - [x] Tini is already PID 1 in the Dockerfile (ENTRYPOINT ["tini", "--"]) -- no changes needed there
  - [x] Signal forwarding is handled by tini -- SIGTERM propagates to entrypoint -> exec'd agent
  - [x] Verify no orphaned containers or networks remain after Ctrl+C

- [x] Task 4: Add tests for `cmd_run()` (AC: 1, 2, 3, 4, 5)
  - [x] Test: `sandbox run` calls docker run with `-it` and `--rm` flags
  - [x] Test: `sandbox run` passes `SANDBOX_AGENT` env var to container
  - [x] Test: `sandbox run` with no existing image triggers build first, then run
  - [x] Test: `sandbox run` with existing image skips build, goes straight to run
  - [x] Test: `sandbox run -f custom/config.yaml` uses custom config path
  - [x] Test: docker run receives correct IMAGE_TAG

- [x] Task 5: Add tests for `scripts/entrypoint.sh` (AC: 4, 5)
  - [x] Test: `SANDBOX_AGENT=claude-code` execs `claude --dangerously-skip-permissions`
  - [x] Test: `SANDBOX_AGENT=gemini-cli` execs `gemini`
  - [x] Test: unknown `SANDBOX_AGENT` value exits with error
  - [x] Test: unset `SANDBOX_AGENT` exits with error

## Dev Notes

### Architecture Compliance

- **Container lifecycle**: tini -> entrypoint.sh -> exec agent. Tini handles signal forwarding (SIGTERM on Ctrl+C) and zombie reaping. Entrypoint sets up environment then `exec`s into agent command. [Source: architecture.md#Container Lifecycle & MCP]
- **Startup sequence**: `tini -> entrypoint.sh -> (setup env) -> exec agent`. The Dockerfile already has `ENTRYPOINT ["tini", "--"]` and `CMD ["/usr/local/bin/entrypoint.sh"]`. [Source: architecture.md#Entrypoint]
- **`--rm` for cleanup**: NFR14 requires no orphaned containers or dangling networks. The `--rm` flag auto-removes the container on exit. Combined with tini's signal forwarding, Ctrl+C cleanly terminates the process tree.
- **Config parsed once**: `cmd_run()` calls `parse_config()` then `cmd_build()`. But `cmd_build()` also calls `parse_config()`. To avoid double-parsing, either: (a) have `cmd_build()` skip parse_config if already called, or (b) accept the double parse since it's idempotent. Option (b) is simpler -- `parse_config()` overwrites globals, and same config produces same values.
- **Exit codes**: 0 = success, 1 = general error (docker run fails). Exit code 3 for missing docker is already handled by `check_dependencies()` in `main()`. [Source: architecture.md#Exit Codes]
- **No color codes, no spinners** -- plain text only.
- **`set -euo pipefail`** already enforced in sandbox.sh and must be in entrypoint.sh.

### Implementation Specifics

**cmd_run() implementation:**
```bash
cmd_run() {
  parse_config
  cmd_build
  info "starting sandbox: ${IMAGE_TAG}"
  docker run -it --rm \
    -e "SANDBOX_AGENT=${CFG_AGENT}" \
    "${IMAGE_TAG}"
}
```

Key design choices:
- `cmd_build` handles the full build pipeline (parse_config -> process_template -> hash -> check -> build if needed). It's idempotent -- if image exists, it prints "image up to date" and returns.
- `SANDBOX_AGENT` env var passes the agent choice into the container. Entrypoint reads it to decide which agent to exec.
- No mounts, secrets, or env vars yet -- those are stories 2.1, 2.2, and 2.3 respectively.
- No `--name` flag needed for MVP -- `--rm` handles cleanup. A named container could conflict if run twice concurrently; unnamed containers avoid this.

**entrypoint.sh implementation:**
```bash
#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${SANDBOX_AGENT:-}" ]]; then
  echo "error: SANDBOX_AGENT not set" >&2
  exit 1
fi

case "${SANDBOX_AGENT}" in
  claude-code)
    exec claude --dangerously-skip-permissions
    ;;
  gemini-cli)
    exec gemini
    ;;
  *)
    echo "error: unknown agent: ${SANDBOX_AGENT}" >&2
    exit 1
    ;;
esac
```

Key design choices:
- `${SANDBOX_AGENT:-}` prevents unset variable error under `set -u`
- `case` is cleaner than if/elif for fixed string matching
- `exec` replaces the shell process -- tini's SIGTERM goes directly to the agent process
- Error messages follow `"error: <message>"` convention to stderr

**Why SANDBOX_AGENT env var (not a CMD argument):**
The Dockerfile has `CMD ["/usr/local/bin/entrypoint.sh"]` -- the entrypoint.sh is the command run by tini. Passing agent via env var (`-e SANDBOX_AGENT=...`) is cleaner than overriding CMD with extra args, and keeps the Dockerfile.template unchanged for this story.

### Testing Strategy

**cmd_run() tests** use the same mock docker approach from story 1.5:
- Mock docker binary that logs all invocations to `docker.log`
- `setup_build_mock()` already handles mocking `docker image inspect` and `docker build`
- Extend mock to also log `docker run` invocations with all arguments
- Verify flags: check `docker.log` contains `docker run -it --rm -e SANDBOX_AGENT=claude-code`
- Verify auto-build: when mock inspect returns 1 (no image), docker.log should show both `docker build` and `docker run`

**entrypoint.sh tests** are separate from sandbox.sh tests:
- Cannot test actual `exec` (replaces process). Instead, mock the agent binary and verify it receives correct arguments.
- Create a mock `claude` or `gemini` binary that writes its args to a file, then run entrypoint.sh.
- Test in a subshell to capture exit codes without killing test runner.
- Alternative: replace `exec` with a test-detectable pattern (e.g., check that the right command WOULD be exec'd by examining script logic).

**Recommended approach for entrypoint.sh tests:**
```bash
# Mock agent binary that records invocation
mock_agent_dir="$(mktemp -d)"
cat > "${mock_agent_dir}/claude" << 'MOCK'
#!/usr/bin/env bash
echo "claude $*" > "${MOCK_AGENT_LOG}"
MOCK
chmod +x "${mock_agent_dir}/claude"

# Run entrypoint with mock in PATH
SANDBOX_AGENT=claude-code MOCK_AGENT_LOG="${tmpdir}/agent.log" \
  PATH="${mock_agent_dir}:${PATH}" \
  bash "${SCRIPT_DIR}/scripts/entrypoint.sh"

# Verify agent was called correctly
assert_contains "$(cat "${tmpdir}/agent.log")" "--dangerously-skip-permissions"
```

Note: Since entrypoint.sh uses `exec`, it replaces the process. The mock agent binary runs and exits, which terminates the entrypoint. This works for testing.

### Previous Story (1-5) Intelligence

**Established patterns to reuse:**
- `setup_build_mock()` function creates mock docker binary with configurable inspect exit code
- Mock docker logs all invocations to `MOCK_DOCKER_LOG` for assertion
- Tests use `PATH="${mockdir}:${PATH}" bash "${SANDBOX}" <command>` to inject mocks
- `assert_contains`/`assert_not_contains` for output verification
- Temp dirs with trap-based cleanup
- 133 tests currently pass -- new tests extend the suite

**Files modified in 1-5:**
- `sandbox.sh` -- Added `CONTENT_HASH`/`IMAGE_TAG` globals, `compute_content_hash()`, `compute_image_tag()`, updated `cmd_build()`
- `tests/test_sandbox.sh` -- Added 22 tests (133 total)

**Key learning from 1-5:**
- Mock docker approach works well for verifying Docker CLI invocations
- `set +e` / `set -e` around expected-failure commands for exit code capture
- Content hash and image tag functions are already available for cmd_run() to use

### Git History Context

Recent commits follow `feat:` prefix convention:
- `4e76771 feat: implement image build with content-hash caching and review fixes (story 1-5)`
- `073e1f9 feat: implement Dockerfile generation from template with review fixes (story 1-4)`

### Current sandbox.sh Structure (relevant sections)

**cmd_run() stub at lines 355-358:**
```bash
cmd_run() {
  parse_config
  info "not yet implemented"
}
```

Replace with actual implementation.

**cmd_build() at lines 325-349:**
Calls `parse_config` -> `process_template` -> writes `.sandbox-dockerfile` -> `compute_content_hash` -> `compute_image_tag` -> checks image existence -> runs `docker build` if needed.

**Config globals at lines 72-85:**
`CFG_AGENT`, `CFG_SDK_*`, `CFG_PACKAGES`, `CFG_SECRETS`, `CFG_MCP`, `CFG_MOUNT_*`, `CFG_ENV_*`, `RESOLVED_DOCKERFILE`, `CONTENT_HASH`, `IMAGE_TAG` -- all available after `parse_config` and `cmd_build`.

### Scope Boundaries

**IN scope for story 1.6:**
- `cmd_run()` with docker run -it --rm
- Pass `SANDBOX_AGENT` env var
- Auto-build via `cmd_build` (FR12)
- Minimal `entrypoint.sh` that execs agent
- Clean Ctrl+C via tini + --rm (NFR14)
- Tests for both cmd_run() and entrypoint.sh

**OUT of scope (later stories):**
- Mount flags `-v` (Story 2.1)
- Secret injection `--env` (Story 2.2)
- Non-secret env vars (Story 2.3)
- MCP .mcp.json generation in entrypoint (Story 5.2)
- Network configuration (Story 4.x)
- git-wrapper.sh implementation (Story 3.1)

### Project Structure Notes

- `sandbox.sh` -- modify `cmd_run()` (replace stub)
- `scripts/entrypoint.sh` -- replace placeholder with minimal agent exec logic
- `tests/test_sandbox.sh` -- add new test sections for cmd_run and entrypoint
- No new files needed

### Anti-Patterns to Avoid

- Do NOT add mount flags, secret injection, or env var handling -- those are stories 2.x
- Do NOT add `--name` to docker run -- unnamed + `--rm` is simpler and avoids conflicts
- Do NOT add `--network` flags -- default networking is fine for story 1.6
- Do NOT implement .mcp.json generation in entrypoint -- that's story 5.2
- Do NOT use `eval` for command construction
- Do NOT add signal trap handlers in sandbox.sh -- tini handles signals inside the container
- Do NOT use `docker stop` or `docker rm` for cleanup -- `--rm` handles it automatically
- Do NOT double-quote inside the docker run array if using direct execution (not eval)

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Container Lifecycle & MCP]
- [Source: _bmad-output/planning-artifacts/architecture.md#Entrypoint]
- [Source: _bmad-output/planning-artifacts/architecture.md#Exit Codes]
- [Source: _bmad-output/planning-artifacts/architecture.md#Bash Coding Conventions]
- [Source: _bmad-output/planning-artifacts/architecture.md#Anti-Patterns]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 1.6]
- [Source: _bmad-output/planning-artifacts/epics.md#FR11, FR12, FR14]
- [Source: _bmad-output/planning-artifacts/prd.md#NFR14]
- [Source: _bmad-output/implementation-artifacts/1-5-image-build-with-content-hash-caching.md]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

### Completion Notes List

- Implemented `scripts/entrypoint.sh` with SANDBOX_AGENT env var routing: claude-code -> `exec claude --dangerously-skip-permissions`, gemini-cli -> `exec gemini`, with error handling for unknown/unset values
- Replaced `cmd_run()` stub in `sandbox.sh` with full implementation: parse_config -> cmd_build (auto-build) -> docker run -it --rm -e SANDBOX_AGENT
- Container cleanup via `--rm` flag; signal forwarding via tini (already configured in Dockerfile)
- Updated 3 existing tests that relied on the old "not yet implemented" stub behavior
- Added 20 new cmd_run() tests: flag verification (-it, --rm), SANDBOX_AGENT passing (claude-code, gemini-cli), auto-build when no image, skip-build when image exists, custom config via -f, IMAGE_TAG format
- Added 7 new entrypoint.sh tests: claude-code exec, gemini-cli exec, unknown agent error, unset agent error
- All 165 tests pass (was 133), 0 failures, 0 regressions

### File List

- `scripts/entrypoint.sh` (modified) - Replaced placeholder with agent exec logic
- `sandbox.sh` (modified) - Replaced cmd_run() stub with docker run implementation
- `tests/test_sandbox.sh` (modified) - Updated 3 existing tests, added 27 new tests

### Change Log

- 2026-03-24: Implemented sandbox run with TTY and lifecycle (Story 1.6) - entrypoint.sh agent routing, cmd_run() with auto-build and docker run, 32 new/updated tests
