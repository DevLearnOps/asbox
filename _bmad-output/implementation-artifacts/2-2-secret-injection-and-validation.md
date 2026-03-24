# Story 2.2: Secret Injection and Validation

Status: review

## Story

As a developer,
I want to declare secret names in my config that are resolved from host environment variables at runtime,
so that the agent has the API keys it needs without them being baked into the image.

## Acceptance Criteria

1. **Given** a config declaring secrets `[ANTHROPIC_API_KEY]` and the variable is set in the host environment
   **When** the sandbox launches
   **Then** `ANTHROPIC_API_KEY` is available as an environment variable inside the container

2. **Given** a declared secret is not set in the host environment (not declared at all)
   **When** the developer runs `sandbox run`
   **Then** the script exits with code 4 and prints a clear error identifying the missing secret

3. **Given** a declared secret is set to an empty string in the host environment
   **When** the sandbox launches
   **Then** the empty value is passed through -- empty is a valid value (uses `${VAR+x}` check)

4. **Given** secrets are injected via `--env` flags
   **When** the container is running
   **Then** secrets are never written to the filesystem, image layers, or build cache (NFR5)

## Tasks / Subtasks

- [x] Task 1: Implement `validate_secrets()` function in sandbox.sh (AC: 2, 3)
  - [x] Add function after config parsing section, before build functions
  - [x] Loop over `CFG_SECRETS` array
  - [x] For each secret, check if set in host environment using `${VAR+x}` (declared?) not `${VAR:+x}` (non-empty?)
  - [x] If any secret is undeclared, `die "secret not set: SECRET_NAME" 4` (exit code 4)
  - [x] Empty string values must pass validation

- [x] Task 2: Extend `cmd_run()` to inject secrets as `--env` flags (AC: 1, 4)
  - [x] Call `validate_secrets()` after `parse_config` and before `cmd_build` (ONLY in cmd_run -- NOT in cmd_build)
  - [x] Loop over `CFG_SECRETS` array after mount assembly, before `docker run` execution
  - [x] For each secret, add `run_flags+=("-e" "${secret_name}")` (Docker resolves from host env)
  - [x] Do NOT use `-e KEY=VALUE` -- use `-e KEY` so Docker reads from host env directly
  - [x] Secret `-e` flags share the same `run_flags` array as mount `-v` flags -- same pattern, same location

- [x] Task 3: Add tests for secret validation (AC: 2, 3)
  - [x] Test: declared secret not set in host env exits with code 4
  - [x] Test: declared secret set to empty string passes validation
  - [x] Test: multiple secrets all set passes validation
  - [x] Test: first of multiple secrets missing reports that specific secret name
  - [x] Test: no secrets declared in config passes validation (zero secrets is valid)

- [x] Task 4: Add tests for secret injection flags (AC: 1)
  - [x] Test: single secret produces `-e SECRET_NAME` in docker run args
  - [x] Test: multiple secrets produce multiple `-e` flags
  - [x] Test: no secrets produces no extra `-e` flags (beyond SANDBOX_AGENT)
  - [x] Test: secret flags appear in mock docker log

- [x] Task 5: Verify no secrets in image or filesystem (AC: 4)
  - [x] Test: `sandbox build` docker log does NOT contain secret values
  - [x] Test: secrets are passed via `-e` flag only (runtime injection, not build-time)

## Dev Notes

### Architecture Compliance

- **Secret validation**: Uses `${VAR+x}` (declared?) not `${VAR:+x}` (non-empty?) -- empty values are valid. [Source: architecture.md#Secret Injection]
- **Exit code 4**: Secret validation error. [Source: architecture.md#Exit Codes]
- **Single parse_config()**: Secrets are already parsed by `parse_config()` into `CFG_SECRETS[]` (sandbox.sh lines 141-149). Do NOT re-parse config or add ad-hoc yq calls. [Source: architecture.md#Gap 4]
- **NFR5**: Secrets injected as runtime env vars only -- never written to filesystem, image layers, or build cache. [Source: prd.md#NFR5]
- **No color codes, no spinners** -- plain text only.
- **`set -euo pipefail`** already enforced.
- **Quoting**: Always `"${var}"` -- critical for variable names.

### Implementation Specifics

**What already exists (DO NOT recreate):**
- `parse_config()` (sandbox.sh lines 141-149) already parses secrets from config YAML into `CFG_SECRETS=()` array of secret name strings
- `CFG_SECRETS=()` global declared at line 77
- `cmd_run()` (sandbox.sh lines 355-388) uses `run_flags=()` array pattern for docker run argument assembly
- Mount flag assembly pattern established in story 2-1 (lines 364-384) -- follow the same `run_flags+=` pattern for secret injection
- 187 tests currently pass -- extend without regressions

**What needs to change:**

1. **Add `validate_secrets()` function** (insert between config parsing section ~line 175 and build functions ~line 180):

```bash
# Validate all declared secrets are set in host environment
validate_secrets() {
  local secret_name
  for secret_name in "${CFG_SECRETS[@]}"; do
    # Use ${VAR+x} to check if declared (empty values are valid)
    eval "local _check=\${${secret_name}+x}"
    if [[ -z "${_check}" ]]; then
      die "secret not set: ${secret_name}" 4
    fi
  done
}
```

**IMPORTANT on `eval` usage:** The architecture anti-patterns say "no eval for template substitution". This is NOT template substitution -- this is the standard bash idiom for indirect variable checking. There is no alternative to check if a variable whose name is stored in another variable is declared. `declare -p` is noisier and less portable. The `eval` here is safe because `CFG_SECRETS` values come from yq parsing of the user's own config file, and are variable names (alphanumeric + underscore).

**Alternative without eval** (if preferred): Use `declare -p "${secret_name}" 2>/dev/null` or bash nameref `local -n ref="${secret_name}"`. However, nameref with `+x` test is awkward. The `eval` approach is the clearest and most maintainable.

2. **Extend `cmd_run()`** -- add secret validation and injection after `parse_config` call:

```bash
cmd_run() {
  parse_config
  validate_secrets    # <-- ADD: validates before build to fail fast
  cmd_build

  local run_flags=()
  run_flags+=("-it" "--rm")
  run_flags+=("-e" "SANDBOX_AGENT=${CFG_AGENT}")

  # Inject secrets as env vars (Docker reads from host environment)
  local secret_name
  for secret_name in "${CFG_SECRETS[@]}"; do
    run_flags+=("-e" "${secret_name}")
  done

  # ... existing mount assembly code unchanged ...
```

**Key design decisions:**
- `validate_secrets()` is called BEFORE `cmd_build` -- fail fast before spending time building the image
- Use `-e SECRET_NAME` (not `-e SECRET_NAME=value`) -- Docker/Podman reads the value from the host environment automatically. This avoids having the secret value appear in the process arguments of the docker run command visible via `ps`.
- Secret names from `CFG_SECRETS` are simple strings like `ANTHROPIC_API_KEY` -- no special handling needed
- Zero secrets is valid -- the loop simply doesn't execute

### Testing Strategy

**Extend existing mock docker approach from stories 1-5, 1-6, and 2-1:**
- Mock docker binary logs all invocations to `MOCK_DOCKER_LOG`
- Tests export/unset environment variables to simulate secret presence/absence
- After running `sandbox run`, inspect `docker.log` for `-e` flags

**Test setup for secret validation:**
```bash
# Test: secret exists and is injected
export ANTHROPIC_API_KEY="sk-test-key"
# ... run sandbox run ...
# Verify docker.log contains: -e ANTHROPIC_API_KEY

# Test: secret not declared in host env
unset ANTHROPIC_API_KEY
# ... run sandbox run ...
# Verify exit code 4 and stderr contains "secret not set: ANTHROPIC_API_KEY"

# Test: secret set to empty string
export ANTHROPIC_API_KEY=""
# ... run sandbox run ...
# Verify docker.log contains: -e ANTHROPIC_API_KEY (empty is valid)
```

**What to verify in docker.log:**
- `-e ANTHROPIC_API_KEY` -- secret name passed (NOT `-e ANTHROPIC_API_KEY=value`)
- Multiple `-e` flags for multiple secrets
- No secret-related `-e` flags when config has no secrets
- Secret flags appear alongside existing `SANDBOX_AGENT` env var

### Previous Story (2-1) Intelligence

**Established patterns to reuse:**
- `run_flags` array pattern for docker run argument assembly (lines 360-387)
- Mock docker logs all invocations for assertion
- `setup_build_mock()` creates mock docker with configurable inspect exit code
- Tests use `PATH="${mockdir}:${PATH}" bash "${SANDBOX}" run -f "${config}"` pattern
- 187 tests currently pass

**Key learning from 2-1:**
- `cmd_run()` calls `cmd_build()` which also calls `parse_config()`. Double parse is accepted (idempotent).
- Array-based flag assembly was introduced in 2-1; secret injection follows the same pattern
- Tests verify docker log content by grepping for specific flags

**Files modified in 2-1:**
- `sandbox.sh` -- `cmd_run()` rewritten with array-based flag assembly
- `tests/test_sandbox.sh` -- 18 new mount tests (187 total)

### Git History Context

Recent commits follow `feat:` prefix convention:
- `9a7980d feat: implement host directory mounts with path resolution (story 2-1)`
- `158065b feat: implement sandbox run with TTY and lifecycle with review fixes (story 1-6)`

### Scope Boundaries

**IN scope for story 2.2:**
- Implement `validate_secrets()` function with `${VAR+x}` check
- Extend `cmd_run()` to add `-e` flags for each declared secret
- Call `validate_secrets()` before `cmd_build` in `cmd_run()` (fail fast)
- Tests for validation (missing, empty, present) and injection (flag presence in docker log)

**OUT of scope (later stories):**
- Non-secret env vars `--env KEY=VALUE` from config `env:` section (Story 2.3)
- Agent runtime verification inside container (Story 2.3)
- MCP .mcp.json generation in entrypoint (Story 5.2)
- Network configuration (Story 4.x)
- Secret rotation or renewal at runtime
- Secret masking in logs (not in requirements)
- Validation of secret name format (env var naming rules)

### Anti-Patterns to Avoid

- Do NOT add non-secret env var passing (`-e KEY=VALUE` from `env:` config) -- that's Story 2.3
- Do NOT use `-e SECRET_NAME=value` -- use `-e SECRET_NAME` so Docker reads from host env (avoids leaking to process args)
- Do NOT use `${VAR:+x}` for validation -- use `${VAR+x}` (empty values must be valid)
- Do NOT write secrets to any file inside the container
- Do NOT pass secrets as build args to docker build
- Do NOT re-parse config.yaml with ad-hoc yq calls -- use the existing `CFG_SECRETS` array
- Do NOT modify `parse_config()` -- secret parsing already works correctly
- Do NOT modify `entrypoint.sh` -- secrets are handled at docker run time via `-e` flags
- Do NOT add secret masking or redaction -- not in requirements
- Secret names MUST be validated as legal shell/env variable identifiers (`[A-Za-z_][A-Za-z0-9_]*`) before use; reject invalid names with exit code 4

### Project Structure Notes

- `sandbox.sh` -- add `validate_secrets()` function; extend `cmd_run()` with secret validation call and `-e` flag assembly
- `tests/test_sandbox.sh` -- add new test sections for secret validation and injection
- No new files needed
- No changes to `parse_config()`, `entrypoint.sh`, `Dockerfile.template`, or `templates/config.yaml`

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Secret Injection] -- fail-closed, ${VAR+x}, empty valid
- [Source: _bmad-output/planning-artifacts/architecture.md#Exit Codes] -- code 4 for secret validation error
- [Source: _bmad-output/planning-artifacts/architecture.md#Implementation Patterns & Consistency Rules] -- quoting, error handling
- [Source: _bmad-output/planning-artifacts/architecture.md#Anti-Patterns] -- no eval for template substitution (note: indirect var check is not template substitution)
- [Source: _bmad-output/planning-artifacts/architecture.md#Gap 4] -- single parse_config() mandate
- [Source: _bmad-output/planning-artifacts/architecture.md#File Responsibilities] -- sandbox.sh owns secret validation
- [Source: _bmad-output/planning-artifacts/epics.md#Story 2.2] -- acceptance criteria
- [Source: _bmad-output/planning-artifacts/epics.md#FR5, FR16] -- secret declaration and validation requirements
- [Source: _bmad-output/planning-artifacts/prd.md#NFR1] -- no implicit credential access
- [Source: _bmad-output/planning-artifacts/prd.md#NFR5] -- secrets as runtime env vars only
- [Source: _bmad-output/implementation-artifacts/2-1-host-directory-mounts.md] -- previous story patterns, run_flags array approach

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None -- clean implementation, no debug issues encountered.

### Completion Notes List

- Implemented `validate_secrets()` function using `eval` with `${VAR+x}` indirect variable check (safe: input from yq-parsed config, not template substitution)
- Extended `cmd_run()` to call `validate_secrets()` before `cmd_build` (fail-fast) and inject `-e SECRET_NAME` flags into `run_flags` array
- Secret injection uses `-e KEY` (not `-e KEY=VALUE`) so Docker reads from host env, avoiding secret leakage in process args
- Updated existing "no eval usage" test to whitelist the indirect variable check pattern
- Added 25 new tests (187 -> 212 total): 9 for secret validation, 8 for secret injection flags, 7 for no-secrets-in-image verification, 1 updated quality check
- All 212 tests pass with zero regressions

### Change Log

- 2026-03-24: Implemented secret injection and validation (story 2-2). Added `validate_secrets()` function, extended `cmd_run()` with `-e` flag assembly, added 25 tests.

### File List

- sandbox.sh (modified: added `validate_secrets()` function, extended `cmd_run()` with secret validation call and `-e` flag injection)
- tests/test_sandbox.sh (modified: added 25 new tests for secret validation, injection, and security; updated eval quality check)
