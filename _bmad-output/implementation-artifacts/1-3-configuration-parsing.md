# Story 1.3: Configuration Parsing

Status: review

## Story

As a developer,
I want sandbox to parse my config.yaml and extract all settings (SDKs, packages, env vars, agent),
so that the build and run steps can use my configuration correctly.

## Acceptance Criteria

1. **Given** a valid config.yaml with SDK versions, packages, env vars, and agent selection
   **When** sandbox parses the config via `parse_config()`
   **Then** all values are extracted correctly and available for downstream use (build and run)

2. **Given** a config.yaml with missing required fields
   **When** sandbox parses the config
   **Then** the script exits with code 1 and prints a clear error identifying the missing field

3. **Given** the developer passes `-f path/to/config.yaml`
   **When** sandbox parses the config
   **Then** it reads from the specified path instead of `.sandbox/config.yaml`

## Tasks / Subtasks

- [x] Task 1: Implement `parse_config()` function (AC: 1, 3)
  - [x] Create `parse_config()` in the "Config parsing functions" section of sandbox.sh (replacing the stub at line 68-72)
  - [x] Accept no arguments -- read from script-level `CONFIG_PATH` variable
  - [x] Verify config file exists at `CONFIG_PATH` -- if not, `die "config not found: ${CONFIG_PATH}" 1`
  - [x] Extract `agent` (string) -- store in `CFG_AGENT`
  - [x] Extract `sdks` (map) -- store enabled SDK versions in `CFG_SDK_NODEJS`, `CFG_SDK_PYTHON`, `CFG_SDK_GO` (empty string if not set)
  - [x] Extract `packages` (list) -- store in bash array `CFG_PACKAGES`
  - [x] Extract `mounts` (list of source/target maps) -- store in parallel arrays `CFG_MOUNT_SOURCES` and `CFG_MOUNT_TARGETS`
  - [x] Extract `secrets` (list of strings) -- store in bash array `CFG_SECRETS`
  - [x] Extract `env` (map) -- store in parallel arrays `CFG_ENV_KEYS` and `CFG_ENV_VALUES`
  - [x] Extract `mcp` (list of strings) -- store in bash array `CFG_MCP`
  - [x] Use `yq` for all YAML extraction -- no manual parsing of YAML
  - [x] Initialize all `CFG_*` variables as script-level globals (empty defaults) before parsing

- [x] Task 2: Validate required fields (AC: 2)
  - [x] Validate `agent` is present and non-empty -- `die "config missing required field: agent" 1`
  - [x] Validate `agent` value is one of: `claude-code`, `gemini-cli` -- `die "config invalid agent: ${CFG_AGENT} (expected: claude-code, gemini-cli)" 1`
  - [x] All other fields are optional -- commented-out or absent sections should result in empty arrays/strings, not errors

- [x] Task 3: Wire `parse_config()` into `cmd_build` and `cmd_run` (AC: 1, 3)
  - [x] Add `parse_config` call at the start of `cmd_build()` (before any build logic)
  - [x] Add `parse_config` call at the start of `cmd_run()` (before any run logic)
  - [x] Do NOT call `parse_config` from `cmd_init` -- init creates the config, it doesn't parse it

- [x] Task 4: Add tests for parse_config (AC: 1, 2, 3)
  - [x] Test: parse_config extracts agent correctly from a valid config
  - [x] Test: parse_config extracts SDK versions (nodejs, python, go) correctly
  - [x] Test: parse_config extracts packages list correctly
  - [x] Test: parse_config extracts mounts (source/target pairs) correctly
  - [x] Test: parse_config extracts secrets list correctly
  - [x] Test: parse_config extracts env key/value pairs correctly
  - [x] Test: parse_config extracts MCP server list correctly
  - [x] Test: parse_config handles optional/missing sections gracefully (empty arrays)
  - [x] Test: missing config file exits code 1 with "config not found" error
  - [x] Test: missing agent field exits code 1 with clear error
  - [x] Test: invalid agent value exits code 1 with clear error
  - [x] Test: parse_config works with `-f` custom path
  - [x] Test: parse_config works with the default starter config from `sandbox init`

## Dev Notes

### Architecture Compliance

- **Single parse_config() function**: This is the MOST critical architectural mandate. Both `cmd_build()` and `cmd_run()` MUST consume config through this one function. No ad-hoc `yq` calls anywhere else in sandbox.sh. [Source: architecture.md#Gap 4: Config parse duplication risk]
- **Script-level globals for output**: Use `CFG_*` prefix for all parsed config values. These are script-level variables (not local) so all downstream functions can read them.
- **Config is parsed at build time AND run time**: Same `parse_config()` function, invoked from both `cmd_build()` and `cmd_run()`. This prevents the two parse paths from diverging. [Source: architecture.md#Cross-Cutting Concerns]
- **Exit code 1**: Config errors use exit code 1 (general error), not 2 (usage error) or 3 (dependency error). [Source: architecture.md#Exit Codes]
- **No color codes, no spinners** -- plain text only.
- **`set -euo pipefail`** is already enforced at script top. `parse_config()` must handle unset yq values without triggering `set -u` failures -- use `${var:-}` or explicit null checks.

### yq Usage Patterns

All yq calls must use the mikefarah/yq v4+ syntax. Key patterns:

```bash
# Extract scalar value (returns "null" if missing)
yq eval '.agent' "${CONFIG_PATH}"

# Extract scalar with default (returns empty string if missing)
yq eval '.agent // ""' "${CONFIG_PATH}"

# Extract array as newline-separated values
yq eval '.packages[]' "${CONFIG_PATH}"

# Check if key exists
yq eval 'has("agent")' "${CONFIG_PATH}"

# Extract map key
yq eval '.sdks.nodejs // ""' "${CONFIG_PATH}"

# Count array items (0 if missing)
yq eval '.mounts | length' "${CONFIG_PATH}"

# Extract from array of objects
yq eval '.mounts[0].source' "${CONFIG_PATH}"
```

**Critical**: yq returns the literal string `"null"` for missing keys (not an empty string). Always check for both `""` and `"null"` when determining if a value is absent, or use the `// ""` default operator.

### Variable Storage Design

Script-level globals initialized before `parse_config()` is called:

```bash
# Scalar values
CFG_AGENT=""

# SDK versions (empty = not configured)
CFG_SDK_NODEJS=""
CFG_SDK_PYTHON=""
CFG_SDK_GO=""

# Arrays (bash indexed arrays)
CFG_PACKAGES=()
CFG_SECRETS=()
CFG_MCP=()

# Parallel arrays for structured data
CFG_MOUNT_SOURCES=()
CFG_MOUNT_TARGETS=()
CFG_ENV_KEYS=()
CFG_ENV_VALUES=()
```

Use `declare -a` for arrays if needed for clarity. These variables are consumed by:
- **Build path**: SDK versions drive template conditional blocks; packages go into apt-get; MCP servers go into npm install
- **Run path**: Mounts become `-v` flags; secrets become `--env` flags; env vars become `--env` flags; agent determines the exec command

### Handling Optional Sections

The starter config from `sandbox init` has optional sections commented out (mounts, secrets, env, mcp). When these are absent from the YAML:
- yq returns `null` for missing keys
- Arrays should be empty `()`, not error
- The `// ""` operator in yq prevents null propagation
- For arrays, check `yq eval '.key | length' file` before iterating -- if 0 or null, skip

Example for optional array:
```bash
local count
count="$(yq eval '.packages | length' "${CONFIG_PATH}" 2>/dev/null || echo "0")"
if [[ "${count}" != "null" && "${count}" -gt 0 ]]; then
  # read array items
fi
```

### Previous Story (1-2) Intelligence

**Established patterns to follow:**
- `die()` for errors (message + exit code), `info()` for success, `warn()` for warnings
- Section headers use `# ===...===` comment blocks
- Function naming: `cmd_init`, `cmd_build`, `cmd_run` for commands; `check_dependencies`, `parse_args` for internals
- `SCRIPT_DIR` and `CONFIG_PATH` are script-level variables already defined at line 11-12
- `-f` flag handling already works in any position (before or after command)

**Files created/modified in 1-2:**
- `sandbox.sh` (modified) -- Added SCRIPT_DIR, CONFIG_PATH globals; refactored parse_args for flag-anywhere support; implemented cmd_init
- `templates/config.yaml` (modified) -- Full starter config with defaults and inline comments
- `tests/test_sandbox.sh` (modified) -- 16 new init tests (54 total)

**Key learnings from 1-2:**
- yq version parsing needed special handling for different yq distributions
- Test suite uses a simple bash testing framework (no external deps)
- All tests use temp directories for isolation
- Tests for config parsing should create purpose-built YAML files in temp dirs rather than relying on `sandbox init` output

### Git History Context

Recent commits show sequential story implementation:
- `654ee5e feat: implement sandbox init with config generation (story 1-2)`
- `72a7ddc feat: add CLI skeleton with dependency validation (story 1-1)`

Code patterns established: functions go in their section between the `# ===` comment blocks; tests append to the bottom of test_sandbox.sh before the Summary section.

### Current sandbox.sh Structure (179 lines)

The config parsing stub is at lines 68-72:
```bash
# ============================================================================
# Config parsing functions (stub)
# ============================================================================

# Implemented in a later story
```

Replace this entire stub section with the `parse_config()` implementation. Initialize `CFG_*` globals just before this section (after the Utility functions section).

### Testing Strategy

Tests should create minimal YAML config files in temp directories with exactly the fields needed for each test case. Pattern:

```bash
tmpdir="$(mktemp -d)"
cat > "${tmpdir}/config.yaml" <<'YAML'
agent: claude-code
sdks:
  nodejs: "22"
packages:
  - curl
YAML

# Source parse_config or call sandbox build/run with -f
set +e
output_all="$(bash "${SANDBOX}" build -f "${tmpdir}/config.yaml" 2>&1)"
exit_code=$?
set -e
# ... assertions ...
rm -rf "${tmpdir}"
```

Since `cmd_build` currently just prints "not yet implemented" after calling `parse_config`, the test can verify that parsing succeeded (exit code 0) and then inspect the global variables. However, bash subshells lose variables -- so testing globals requires either:
1. Adding a debug/dump mode to sandbox.sh (not recommended -- adds non-production code)
2. Testing indirectly by verifying `cmd_build` doesn't crash with valid config (exit 0) and crashes with invalid config (exit 1)
3. Creating a test helper that sources sandbox.sh functions

**Recommended approach**: Option 2 (indirect testing). Verify that `sandbox build -f valid.yaml` exits 0 (parse succeeded, then prints "not yet implemented") and `sandbox build -f invalid.yaml` exits 1 with the expected error. This tests parse_config end-to-end without coupling tests to internal variable names.

### Anti-Patterns to Avoid

- Do NOT parse YAML manually with `grep`, `sed`, or `awk` -- use yq for everything
- Do NOT create a second config parser or ad-hoc yq calls outside `parse_config()`
- Do NOT use `eval` for any purpose
- Do NOT put parse_config output on stdout -- use script-level variables
- Do NOT make `parse_config()` accept arguments -- it reads from `CONFIG_PATH`
- Do NOT validate fields that are optional (mounts, secrets, env, mcp, sdks) -- only `agent` is required
- Do NOT use `local` for `CFG_*` variables -- they must be script-level for downstream functions

### Project Structure Notes

- All changes go in `sandbox.sh` (single file mandate) and `tests/test_sandbox.sh`
- No new files needed for this story
- Config parsing section replaces the stub between lines 68-72
- CFG_* globals should be initialized between Utility functions and Config parsing sections

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Config YAML Conventions]
- [Source: _bmad-output/planning-artifacts/architecture.md#Gap 4: Config parse duplication risk]
- [Source: _bmad-output/planning-artifacts/architecture.md#Cross-Cutting Concerns — Configuration parsing and data flow direction]
- [Source: _bmad-output/planning-artifacts/architecture.md#Implementation Patterns & Consistency Rules]
- [Source: _bmad-output/planning-artifacts/architecture.md#Exit Codes]
- [Source: _bmad-output/planning-artifacts/architecture.md#Anti-Patterns]
- [Source: _bmad-output/planning-artifacts/architecture.md#Script Organization (sandbox.sh)]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 1.3]
- [Source: _bmad-output/planning-artifacts/epics.md#FR1-FR7 — config options]
- [Source: _bmad-output/implementation-artifacts/1-2-configuration-initialization.md]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None — implementation was straightforward with no blocking issues.

### Completion Notes List

- Implemented `parse_config()` function using yq v4+ for all YAML extraction
- CFG_* globals initialized as script-level variables with empty defaults before the parsing section
- Agent validation: required, must be `claude-code` or `gemini-cli`, exits code 1 on failure
- All other fields (sdks, packages, mounts, secrets, env, mcp) are optional — missing sections produce empty arrays/strings
- Parallel arrays used for structured data (mounts: source/target, env: keys/values)
- Wired `parse_config` into `cmd_build()` and `cmd_run()` — NOT `cmd_init`
- Updated existing `eval` detection test to exclude `yq eval` (false positive)
- Added 22 new tests covering all ACs: valid extraction, error cases, custom path, starter config compatibility
- All 77 tests pass (55 existing + 22 new), 0 regressions

### Change Log

- 2026-03-24: Implemented parse_config() with full YAML extraction and validation; added 22 tests (77 total)

### File List

- sandbox.sh (modified) — Added CFG_* globals, parse_config() function, wired into cmd_build/cmd_run
- tests/test_sandbox.sh (modified) — Added 22 parse_config tests, fixed eval detection for yq eval
