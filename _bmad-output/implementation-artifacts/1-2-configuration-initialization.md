# Story 1.2: Configuration Initialization

Status: review

## Story

As a developer,
I want to run `sandbox init` to generate a starter `.sandbox/config.yaml` with sensible defaults,
so that I have a working starting point for configuring my sandbox.

## Acceptance Criteria

1. **Given** no `.sandbox/config.yaml` exists in the current directory
   **When** the developer runs `sandbox init`
   **Then** a `.sandbox/config.yaml` is created with default agent (claude-code), Node.js SDK, common packages, and inline comments explaining each option

2. **Given** a `.sandbox/config.yaml` already exists
   **When** the developer runs `sandbox init`
   **Then** the script exits with code 1 and prints "error: config already exists" to stderr without overwriting

3. **Given** the developer specifies `-f custom/path/config.yaml`
   **When** they run `sandbox init -f custom/path/config.yaml`
   **Then** the config is generated at the specified path

## Tasks / Subtasks

- [x] Task 1: Fix config_path propagation in parse_args (AC: 3)
  - [x] Promote `config_path` from local in `parse_args()` to a script-level variable so commands can access it
  - [x] Ensure `-f` flag works regardless of position (before or after command): `sandbox -f path init` and `sandbox init -f path` (currently `-f` is only parsed before the command, and the loop exits on command match — handle both orders)
  - [x] Pass `config_path` to `cmd_init` (and later to `cmd_build`/`cmd_run`)

- [x] Task 2: Create the starter config template (AC: 1)
  - [x] Replace the placeholder `templates/config.yaml` with a full starter config
  - [x] Include inline YAML comments (`#`) explaining each option
  - [x] Default values: `agent: claude-code`, `sdks: { nodejs: "22" }`, common packages `[build-essential, curl, wget, git, jq]` (git included per spec), empty `mounts`, `secrets`, `env`, `mcp` sections
  - [x] Follow config YAML conventions: `lower_snake_case` keys, no nesting beyond two levels, mount entries use `source`/`target` keys

- [x] Task 3: Implement cmd_init (AC: 1, 2, 3)
  - [x] Check if config file already exists at `config_path` — if yes, `die "config already exists" 1` (do NOT overwrite)
  - [x] Create parent directories (`mkdir -p`) for the config path
  - [x] Copy `templates/config.yaml` to `config_path`
  - [x] Use the script's own location to resolve `templates/config.yaml` path (do NOT assume CWD — use `SCRIPT_DIR` pattern)
  - [x] Print success message: `info "created ${config_path}"`

- [x] Task 4: Update tests (AC: 1, 2, 3)
  - [x] Test: `sandbox init` creates `.sandbox/config.yaml` with expected content
  - [x] Test: `sandbox init` when config exists exits code 1 with error message
  - [x] Test: `sandbox init -f custom/path.yaml` creates at custom path
  - [x] Test: `sandbox -f custom/path.yaml init` also works (flag before command)
  - [x] Test: generated config is valid YAML (parseable by yq)
  - [x] Test: generated config contains expected defaults (agent, sdks, packages)

## Dev Notes

### Architecture Compliance

- **Single file**: All logic stays in `sandbox.sh` — no external modules
- **Script organization**: `cmd_init()` goes in the "Init function" section (between Run and Command dispatch sections)
- **Template file**: `templates/config.yaml` is the source of truth for the starter config content — `cmd_init` copies this file, it does NOT generate YAML programmatically
- **Exit codes**: 1 for "config already exists" (general error), NOT 2 (usage error)
- **Output format**: `error: config already exists` to stderr (via `die`), success info to stdout (via `info`)
- **No color codes, no spinners** — plain text only

### Critical: config_path Propagation Bug

The current `parse_args()` in sandbox.sh (line 115) declares `config_path` as a local variable. This means `cmd_init()`, `cmd_build()`, and `cmd_run()` cannot access the user's `-f` override. Fix this by:

1. Declaring `CONFIG_PATH` as a script-level variable (initialized to `DEFAULT_CONFIG_PATH`)
2. Having `-f` update `CONFIG_PATH` instead of a local
3. Having commands read from `CONFIG_PATH`

Also note: the current argument parser handles `-f` only before the command token. If a user runs `sandbox init -f path`, the parser hits `init`, calls `cmd_init`, and exits — never seeing `-f`. Fix by collecting all flags first, then dispatching the command, OR by continuing to parse after the command token.

### SCRIPT_DIR Pattern

To locate `templates/config.yaml` relative to the script (not CWD), add near the top constants section:

```bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
```

Then in `cmd_init`: `cp "${SCRIPT_DIR}/templates/config.yaml" "${CONFIG_PATH}"`

### Starter Config Content

The `templates/config.yaml` should contain all configuration sections defined in the architecture:

```yaml
# Sandbox Configuration
# See: https://github.com/... for full documentation

# AI agent to run inside the sandbox
# Options: claude-code, gemini-cli
agent: claude-code

# SDK versions to install (comment out to skip)
sdks:
  nodejs: "22"
  # python: "3.12"
  # go: "1.22"

# Additional system packages to install
packages:
  - build-essential
  - curl
  - wget
  - git
  - jq

# Host directories to mount into the sandbox
# mounts:
#   - source: "."
#     target: "/workspace"

# Secret names resolved from host environment variables at runtime
# secrets:
#   - ANTHROPIC_API_KEY

# Non-secret environment variables
# env:
#   NODE_ENV: development

# MCP servers to pre-install
# mcp:
#   - playwright
```

### Previous Story (1-1) Intelligence

**Established patterns to follow:**
- `die()` for errors (message + exit code), `info()` for success, `warn()` for warnings
- Section headers use `# ===...===` comment blocks
- Function naming: `cmd_init`, `cmd_build`, `cmd_run` for commands; `check_dependencies`, `parse_args` for internals
- Test suite at `tests/test_sandbox.sh` — uses function-based test pattern with counters
- `BASH_VERSINFO` check runs first, then `check_dependencies`, then `parse_args`

**Files created in 1-1:**
- `sandbox.sh` — CLI entry point (165 lines, fully functional skeleton)
- `scripts/entrypoint.sh` — placeholder
- `scripts/git-wrapper.sh` — placeholder
- `templates/config.yaml` — placeholder (just a comment, needs full content)
- `Dockerfile.template` — placeholder
- `tests/test_sandbox.sh` — test suite (40 tests)

**Key learnings from 1-1:**
- yq version parsing needed special handling for different yq distributions
- Test suite uses a simple bash testing framework (no external dependencies like bats)
- All 40 tests passed on first implementation attempt

### Anti-Patterns to Avoid

- Do NOT generate YAML via `echo` or heredoc in `cmd_init` — copy the template file
- Do NOT use `eval` for path expansion
- Do NOT overwrite existing config — check first, fail with `die`
- Do NOT create directories with `755` permissions explicitly — let `mkdir -p` use umask defaults
- Do NOT assume script is run from project root — resolve template path from `SCRIPT_DIR`
- Do NOT hardcode `.sandbox/config.yaml` in `cmd_init` — always use `CONFIG_PATH`

### Testing Notes

The existing test suite in `tests/test_sandbox.sh` uses a simple bash testing pattern:
- Functions named `test_*`
- Manual assertion helpers
- Counter-based pass/fail tracking
- Tests use temporary directories for isolation

New tests should follow the same pattern. Tests for `init` should:
- Create a temp directory as working directory
- Run `sandbox init` and verify file creation
- Run `sandbox init` again and verify it fails
- Verify YAML validity with `yq eval '.' <file>`
- Verify expected keys exist in generated config

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Config YAML Conventions]
- [Source: _bmad-output/planning-artifacts/architecture.md#Script Organization (sandbox.sh)]
- [Source: _bmad-output/planning-artifacts/architecture.md#File Responsibilities — templates/config.yaml]
- [Source: _bmad-output/planning-artifacts/architecture.md#Exit Codes]
- [Source: _bmad-output/planning-artifacts/architecture.md#Anti-Patterns]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 1.2]
- [Source: _bmad-output/planning-artifacts/epics.md#FR9 — sandbox init starter config]
- [Source: _bmad-output/implementation-artifacts/1-1-cli-skeleton-and-dependency-validation.md]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- Used `${BASH_SOURCE[0]%/*}` instead of `dirname` for SCRIPT_DIR to avoid dependency on external `dirname` binary in restricted PATH test environments

### Completion Notes List

- Task 1: Refactored parse_args to collect all flags before dispatching commands. Added script-level CONFIG_PATH and SCRIPT_DIR variables. `-f` now works in any position relative to the command.
- Task 2: Replaced placeholder templates/config.yaml with full starter config including all sections (agent, sdks, packages, mounts, secrets, env, mcp) with inline comments.
- Task 3: Implemented cmd_init with existence check (die on conflict), mkdir -p for parent dirs, cp from SCRIPT_DIR-relative template path, and success info message.
- Task 4: Added 16 new tests (54 total, up from 40). Updated existing stub test to exclude init from "not yet implemented" checks. All tests pass with zero regressions.

### Change Log

- 2026-03-24: Implemented story 1-2 — config_path propagation fix, starter config template, cmd_init implementation, 16 new tests

### File List

- sandbox.sh (modified) — Added SCRIPT_DIR, CONFIG_PATH globals; refactored parse_args for flag-anywhere support; implemented cmd_init
- templates/config.yaml (modified) — Full starter config with defaults and inline comments
- tests/test_sandbox.sh (modified) — 16 new init tests, updated stub test loop
