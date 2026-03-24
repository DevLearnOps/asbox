# Story 2.3: Agent Runtime with Project Files and BMAD Support

Status: done

## Story

As a developer,
I want the agent to launch with full terminal access and be able to interact with mounted project files, install packages, and execute BMAD workflows,
so that the agent can do real development work inside the sandbox.

## Acceptance Criteria

1. **Given** a sandbox is running with a project directory mounted
   **When** the agent executes
   **Then** it has full terminal access and can read/write files in the mounted project directory

2. **Given** the agent needs additional packages at runtime
   **When** it runs `apt-get install` or `npm install` or `pip install`
   **Then** packages install successfully (the container has internet access and package managers)

3. **Given** a project with BMAD planning artifacts mounted
   **When** the agent reads/writes to `_bmad-output/` or `docs/` directories
   **Then** changes persist in the mounted host directory

## Tasks / Subtasks

- [x] Task 1: Inject non-secret environment variables in `cmd_run()` (AC: 1)
  - [x] Loop over `CFG_ENV_KEYS` and `CFG_ENV_VALUES` arrays
  - [x] For each env var, add `run_flags+=("-e" "KEY=VALUE")` to docker run flags
  - [x] Insert after secret injection, before mount assembly
  - [x] Zero env vars is valid -- loop simply doesn't execute

- [x] Task 2: Add tests for non-secret env var injection (AC: 1)
  - [x] Test: single env var produces `-e KEY=VALUE` in docker run args
  - [x] Test: multiple env vars produce multiple `-e KEY=VALUE` flags
  - [x] Test: no env vars produces no extra `-e` flags (beyond SANDBOX_AGENT and secrets)
  - [x] Test: env var with spaces in value is passed correctly
  - [x] Test: env vars appear in mock docker log alongside secret and mount flags

- [x] Task 3: Add integration tests verifying complete cmd_run() flag assembly (AC: 1, 3)
  - [x] Test: config with mounts + secrets + env vars produces all flags in correct order
  - [x] Test: BMAD-style project directory mounted at /workspace shows correct `-v` and `-w` flags
  - [x] Test: env vars coexist with secrets without conflicts

- [x] Task 4: Verify entrypoint.sh agent runtime behavior (AC: 1, 2)
  - [x] Confirm entrypoint tests already cover agent exec routing (claude-code, gemini-cli)
  - [x] Confirm `-it` and `--rm` flags ensure full terminal access and cleanup
  - [x] No new entrypoint changes needed -- existing implementation satisfies AC

## Dev Notes

### Architecture Compliance

- **Non-secret env vars**: `env:` config section parsed into `CFG_ENV_KEYS[]` and `CFG_ENV_VALUES[]` by `parse_config()`. Must be injected as `-e KEY=VALUE` (NOT `-e KEY` like secrets). [Source: architecture.md#File Responsibilities]
- **Single parse_config()**: Env vars are already parsed. Do NOT re-parse config or add ad-hoc yq calls. [Source: architecture.md#Gap 4]
- **Exit codes**: 0 = success, 1 = general error. [Source: architecture.md#Exit Codes]
- **No color codes, no spinners** -- plain text only.
- **`set -euo pipefail`** already enforced.
- **Quoting**: Always `"${var}"` -- critical for values with spaces.

### Implementation Specifics

**What already exists (DO NOT recreate):**
- `parse_config()` (sandbox.sh lines 91-175) already parses `env:` config section into:
  - `CFG_ENV_KEYS=()` -- array of env var names (lines 153-164)
  - `CFG_ENV_VALUES=()` -- array of env var values (lines 153-164)
- `cmd_run()` (sandbox.sh lines 375-415) already handles:
  - `parse_config` call
  - `validate_secrets` call
  - `cmd_build` call
  - Secret injection via `-e SECRET_NAME` flags (lines 386-389)
  - Mount flag assembly with path resolution (lines 391-411)
  - `-w` working directory flag (lines 408-411)
- Entrypoint.sh (scripts/entrypoint.sh) already handles agent exec routing
- 212 tests currently pass -- extend without regressions

**What needs to change:**

1. **Extend `cmd_run()`** -- add non-secret env var injection after secret injection:

```bash
  # Inject non-secret env vars as KEY=VALUE
  local j
  for j in "${!CFG_ENV_KEYS[@]}"; do
    run_flags+=("-e" "${CFG_ENV_KEYS[$j]}=${CFG_ENV_VALUES[$j]}")
  done
```

**Key design decisions:**
- Use `-e KEY=VALUE` (not `-e KEY`) because non-secret env vars are defined in the config with explicit values -- they are NOT read from the host environment
- This is distinct from secrets which use `-e KEY` so Docker reads from host env
- Insert after secret injection, before mount assembly, to keep logical grouping: env setup -> filesystem setup
- Zero env vars is valid -- the loop simply doesn't execute (same pattern as secrets and mounts)
- Values with spaces are handled correctly by the array-based `run_flags` pattern (no string splitting)

**What does NOT need to change:**
- `parse_config()` -- env parsing already works correctly
- `entrypoint.sh` -- agent routing is already implemented; env vars are automatically available to the agent process via Docker's env injection
- `Dockerfile.template` -- package managers (apt-get, npm, pip) are already available in the base image with Node.js SDK; internet access is unrestricted by default
- `templates/config.yaml` -- already has commented `env:` section

### AC 2 (Package Installation) -- No Code Changes Needed

AC 2 is satisfied by the existing container configuration:
- Ubuntu 24.04 base image has `apt-get` available
- Node.js SDK installation (configured in most sandboxes) provides `npm`
- Python SDK installation (if configured) provides `pip`
- Internet access is unrestricted (no egress filtering) -- packages can be fetched
- Container runs with sufficient privileges for package installation
- This is a design-level guarantee, not a code change -- no new tests needed for package managers (would require a real Docker build)

### AC 3 (BMAD Persistence) -- No Code Changes Needed

AC 3 is satisfied by the mount system implemented in story 2-1:
- Host directories mounted via `-v` are bidirectional by default
- Agent writes to `/workspace/_bmad-output/` persist on the host
- Agent writes to `/workspace/docs/` persist on the host
- This is Docker's default mount behavior -- no additional configuration needed
- Integration test verifies mount + env var + secret flags all work together

### Testing Strategy

**Extend existing mock docker approach from stories 2-1 and 2-2:**
- Mock docker binary logs all invocations to `MOCK_DOCKER_LOG`
- Tests create temp config files with `env:` declarations
- After running `sandbox run`, inspect `docker.log` for `-e KEY=VALUE` flags

**Test config for env var injection:**
```yaml
agent: claude-code
env:
  NODE_ENV: development
  DEBUG: "true"
```

**What to verify in docker.log:**
- `-e NODE_ENV=development` -- key=value pair passed
- `-e DEBUG=true` -- multiple env vars produce multiple flags
- `-e SANDBOX_AGENT=claude-code` -- existing env vars still present
- Secret `-e KEY` flags (if any) distinct from env `-e KEY=VALUE` flags
- Mount `-v` and `-w` flags still work alongside env vars

### Previous Story (2-2) Intelligence

**Established patterns to reuse:**
- `run_flags` array pattern for docker run argument assembly
- Mock docker logs all invocations for assertion
- `setup_build_mock()` creates mock docker with configurable inspect exit code
- Tests use `PATH="${mockdir}:${PATH}" bash "${SANDBOX}" run -f "${config}"` pattern
- Tests export/unset env vars and inspect docker log for flag presence
- 212 tests currently pass

**Key learning from 2-2:**
- Secret injection uses `-e KEY` (no value) -- env var injection MUST use `-e KEY=VALUE` (with value)
- `validate_secrets()` is called before `cmd_build` for fail-fast -- no equivalent validation needed for env vars (values come from config, not host env)
- Array-based flag assembly handles all edge cases (spaces, special chars) correctly

**Files modified in 2-2:**
- `sandbox.sh` -- added `validate_secrets()`, extended `cmd_run()` with `-e` secret flags
- `tests/test_sandbox.sh` -- 25 new tests (212 total)

### Git History Context

Recent commits follow `feat:` prefix convention:
- `80b8cd1 feat: implement secret injection and validation with review fixes (story 2-2)`
- `9a7980d feat: implement host directory mounts with path resolution (story 2-1)`

### Scope Boundaries

**IN scope for story 2.3:**
- Inject non-secret env vars from config `env:` section as `-e KEY=VALUE` flags in `cmd_run()`
- Tests for env var injection (single, multiple, zero, with spaces, alongside secrets/mounts)
- Integration test verifying complete flag assembly (mounts + secrets + env vars)

**OUT of scope (later stories):**
- Git operations inside the container (Story 2.4)
- CLI tools verification inside the container (Story 2.4)
- Internet access verification (Story 2.4)
- MCP .mcp.json generation in entrypoint (Story 5.2)
- Network configuration (Story 4.x)
- Env var name validation (not in requirements -- config is user-authored)
- Env var value masking (not in requirements)
- Env var precedence rules (Docker handles duplicates by last-wins)

### Anti-Patterns to Avoid

- Do NOT add git operations or CLI tool verification -- that's Story 2.4
- Do NOT use `-e KEY` (without value) for env vars -- use `-e KEY=VALUE` (values come from config, not host env)
- Do NOT re-parse config.yaml with ad-hoc yq calls -- use the existing `CFG_ENV_KEYS/CFG_ENV_VALUES` arrays
- Do NOT modify `parse_config()` -- env parsing already works correctly
- Do NOT modify `entrypoint.sh` -- env vars are handled at docker run time via `-e` flags
- Do NOT modify `Dockerfile.template` -- no build-time changes needed
- Do NOT add env var name validation -- not in requirements, config is user-authored
- Do NOT attempt real Docker builds in tests -- use mock docker for unit tests
- Do NOT modify the existing secret or mount injection code -- only add env var injection alongside it

### Project Structure Notes

- `sandbox.sh` -- extend `cmd_run()` to inject non-secret env vars as `-e KEY=VALUE` flags
- `tests/test_sandbox.sh` -- add new test sections for env var injection and integration
- No new files needed
- No changes to `parse_config()`, `entrypoint.sh`, `Dockerfile.template`, or `templates/config.yaml`

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#File Responsibilities] -- sandbox.sh owns env var injection via docker run flags
- [Source: _bmad-output/planning-artifacts/architecture.md#Implementation Patterns & Consistency Rules] -- quoting, error handling, array-based flag assembly
- [Source: _bmad-output/planning-artifacts/architecture.md#Anti-Patterns] -- no eval, no unquoted vars
- [Source: _bmad-output/planning-artifacts/architecture.md#Gap 4] -- single parse_config() mandate
- [Source: _bmad-output/planning-artifacts/architecture.md#Data Flow] -- config -> parse -> docker run flag assembly
- [Source: _bmad-output/planning-artifacts/epics.md#Story 2.3] -- acceptance criteria
- [Source: _bmad-output/planning-artifacts/epics.md#FR6, FR17, FR20, FR21, FR22] -- env vars, agent runtime, project files, BMAD support
- [Source: _bmad-output/planning-artifacts/prd.md#FR6] -- non-secret environment variables for agent runtime
- [Source: _bmad-output/implementation-artifacts/2-2-secret-injection-and-validation.md] -- previous story patterns, run_flags array, 212 tests
- [Source: _bmad-output/implementation-artifacts/2-1-host-directory-mounts.md] -- mount flag assembly pattern, path resolution

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None -- implementation was straightforward with no debugging needed.

### Completion Notes List

- Task 1: Added 5-line env var injection loop in `cmd_run()` after secret injection, before mount assembly. Uses `CFG_ENV_KEYS` and `CFG_ENV_VALUES` arrays populated by existing `parse_config()`. Pattern `-e KEY=VALUE` (with value) distinguishes from secrets which use `-e KEY` (no value).
- Task 2: Added 14 unit tests for env var injection: single var, multiple vars, zero vars, spaces in values, coexistence with secrets and mounts.
- Task 3: Added 19 integration tests for complete flag assembly: mounts + secrets + env vars together, BMAD-style project mount with `-v` and `-w` flags, secret/env var coexistence without conflicts.
- Task 4: Verified existing entrypoint tests cover agent exec routing (claude-code, gemini-cli) and `-it`/`--rm` flags. No changes needed.
- All 245 tests pass (212 existing + 33 new). Zero regressions.

### Change Log

- 2026-03-24: Implemented non-secret env var injection in cmd_run() and added 33 tests (story 2-3)

### File List

- sandbox.sh (modified: added env var injection loop in cmd_run, lines 391-395)
- tests/test_sandbox.sh (modified: added 33 new tests for env var injection and integration)
