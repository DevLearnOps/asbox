# Story 4.4: Agent CLI Installation

Status: review

## Story

As a developer,
I want the sandbox image to include the configured AI agent CLI (Claude Code or Gemini CLI),
so that the entrypoint can exec into the agent without relying on manual installation or runtime downloads.

## Background

The entrypoint (`scripts/entrypoint.sh`, lines 26-44) already routes to `claude --dangerously-skip-permissions` or `gemini` based on `SANDBOX_AGENT`, and validates the binary exists with `command -v`. However, no story has addressed actually installing these CLIs into the image. Currently the sandbox would fail at launch with "error: claude not found in PATH" or "error: gemini not found in PATH".

## Acceptance Criteria

1. **Given** a config with `agent: claude-code` and `sdks.nodejs` configured
   **When** the sandbox image is built
   **Then** the Claude Code CLI (`@anthropic-ai/claude-code`) is installed globally via npm and `claude --version` succeeds inside the container

2. **Given** a config with `agent: gemini-cli` and `sdks.nodejs` configured
   **When** the sandbox image is built
   **Then** the Gemini CLI (`@google/gemini-cli`) is installed globally via npm and `gemini --version` succeeds inside the container

3. **Given** a config with `agent: claude-code` but no `sdks.nodejs` configured
   **When** the developer runs `sandbox build`
   **Then** the build fails with a clear error: "agent claude-code requires sdks.nodejs to be configured"

4. **Given** the agent CLI is installed at build time
   **When** inspecting the installed version
   **Then** no specific version is pinned -- `npm install -g` installs the latest available version (the image content-hash will trigger a rebuild when the Dockerfile.template changes, but the agent CLI version floats to latest on each build)

5. **Given** the sandbox container starts with the agent CLI installed
   **When** the entrypoint runs the existing `command -v` check
   **Then** the check passes and the agent launches successfully

## Tasks / Subtasks

- [x] Task 1: Add agent CLI installation to Dockerfile.template (AC: #1, #2)
  - [x] 1.1 Add conditional block `# {{IF_AGENT_CLAUDE}}` / `# {{/IF_AGENT_CLAUDE}}` containing `RUN npm install -g @anthropic-ai/claude-code`
  - [x] 1.2 Add conditional block `# {{IF_AGENT_GEMINI}}` / `# {{/IF_AGENT_GEMINI}}` containing `RUN npm install -g @google/gemini-cli`
  - [x] 1.3 Place agent blocks after all SDK blocks (after line ~64, the `{{IF_GO}}` block) but before the MCP block (line ~66, `{{IF_MCP_PLAYWRIGHT}}`)
- [x] Task 2: Add template marker expansion in sandbox.sh `process_template()` (AC: #1, #2)
  - [x] 2.1 Read `CFG_AGENT` (already parsed into global var at line ~71)
  - [x] 2.2 Add IF_AGENT_CLAUDE conditional processing: keep block when `CFG_AGENT == "claude-code"`, strip otherwise
  - [x] 2.3 Add IF_AGENT_GEMINI conditional processing: keep block when `CFG_AGENT == "gemini-cli"`, strip otherwise
  - [x] 2.4 Ensure tag validation in step 1 of `process_template()` (lines 270-287) recognizes the new tags
- [x] Task 3: Add Node.js dependency validation in `cmd_build()` (AC: #3)
  - [x] 3.1 After existing MCP dependency check (lines 386-390), add validation that `CFG_SDK_NODEJS` is set when `CFG_AGENT` is `claude-code` or `gemini-cli`
  - [x] 3.2 Error message format: `die "agent <name> requires sdks.nodejs to be configured" 1`
- [x] Task 4: Write tests in tests/test_sandbox.sh (AC: #1, #2, #3, #5)
  - [x] 4.1 Test: when agent is `claude-code` and Node.js configured, generated Dockerfile contains `npm install -g @anthropic-ai/claude-code`
  - [x] 4.2 Test: when agent is `gemini-cli` and Node.js configured, generated Dockerfile contains `npm install -g @google/gemini-cli`
  - [x] 4.3 Test: when agent is `claude-code` and no `sdks.nodejs`, build exits with error containing "requires sdks.nodejs"
  - [x] 4.4 Test: when agent is `gemini-cli` and no `sdks.nodejs`, build exits with error containing "requires sdks.nodejs"
  - [x] 4.5 Test: agent installation block appears after SDK blocks but before isolation scripts in generated Dockerfile
  - [x] 4.6 Test: no cross-contamination — selecting one agent excludes the other's install lines
- [x] Task 5: Write integration test in tests/test_integration_agent_cli.sh (AC: #1, #4, #5)
  - [x] 5.1 Build real sandbox image and verify `command -v claude` succeeds inside container
  - [x] 5.2 Verify `claude --version` returns a version string
  - [x] 5.3 Verify `@anthropic-ai/claude-code` appears in npm global packages
  - [x] 5.4 Print installed version info for manual verification (AC #4 — no pinning)

## Dev Notes

### Exact Files to Modify

1. **Dockerfile.template** -- Add two new conditional blocks for agent CLI installation
2. **sandbox.sh** -- Add conditional processing in `process_template()` (~line 291+) and validation in `cmd_build()` (~line 386+)
3. **tests/test_sandbox.sh** -- Add new test section at end of file (after story 4-3 tests at line ~2832)
4. **tests/test_integration_agent_cli.sh** -- New integration test verifying agent CLI is installed and functional inside a real container

### What Does NOT Change

- `scripts/entrypoint.sh` -- Already handles agent routing and `command -v` checks (lines 26-44). No changes needed.
- `templates/config.yaml` -- Already has `agent: claude-code` field. No changes needed.
- `scripts/git-wrapper.sh` -- Unrelated.

### Conditional Block Pattern to Follow

Follow the exact pattern from existing blocks in `process_template()` (sandbox.sh lines 291-322):

```bash
# IF_AGENT_CLAUDE
if [[ "${CFG_AGENT}" == "claude-code" ]]; then
  template="$(echo "${template}" | sed '/^# {{IF_AGENT_CLAUDE}}$/d; /^# {{\/IF_AGENT_CLAUDE}}$/d')"
else
  template="$(echo "${template}" | sed '/^# {{IF_AGENT_CLAUDE}}$/,/^# {{\/IF_AGENT_CLAUDE}}$/d')"
fi
```

Note: Existing blocks test `-n "${CFG_SDK_NODEJS}"` (non-empty), but agent blocks should test string equality (`== "claude-code"`) since `CFG_AGENT` holds a name, not a version.

### Dockerfile.template Block Placement

Insert agent blocks between the SDK installations and the MCP block. Current ordering:
1. Lines 49-53: `{{IF_NODE}}` (Node.js SDK)
2. Lines 55-59: `{{IF_PYTHON}}` (Python SDK)
3. Lines 61-64: `{{IF_GO}}` (Go SDK)
4. **NEW: `{{IF_AGENT_CLAUDE}}` and `{{IF_AGENT_GEMINI}}` go here**
5. Lines 66-68: `{{IF_MCP_PLAYWRIGHT}}` (MCP servers)
6. Lines 70-71: MCP manifest
7. Lines 73-76: Isolation scripts COPY
8. Lines 78+: Non-root user setup

### Validation Pattern to Follow

Follow the existing MCP dependency validation in `cmd_build()` (sandbox.sh lines 386-390):

```bash
# Existing pattern:
if [[ "${_mcp}" == "playwright" && -z "${CFG_SDK_NODEJS}" ]]; then
  die "mcp server 'playwright' requires sdks.nodejs to be configured" 1
fi

# New pattern (add after MCP validation):
if [[ -n "${CFG_AGENT}" && "${CFG_AGENT}" != "none" ]]; then
  if [[ "${CFG_AGENT}" == "claude-code" || "${CFG_AGENT}" == "gemini-cli" ]]; then
    if [[ -z "${CFG_SDK_NODEJS}" ]]; then
      die "agent ${CFG_AGENT} requires sdks.nodejs to be configured" 1
    fi
  fi
fi
```

### Test Pattern to Follow

From story 4-3, use the established test pattern:

```bash
# Dockerfile content capture
dockerfile_content="$(cat "${PROJECT_ROOT}/.sandbox-dockerfile")"

# Assertion helpers
assert_contains "${dockerfile_content}" "npm install -g @anthropic-ai/claude-code" "description"
assert_not_contains "${dockerfile_content}" "@google/gemini-cli" "description"

# Ordering verification with line numbers
agent_line="$(echo "${dockerfile_content}" | grep -n "npm install -g @anthropic-ai/claude-code" | head -1 | cut -d: -f1)"
copy_line="$(echo "${dockerfile_content}" | grep -n "COPY scripts/entrypoint.sh" | head -1 | cut -d: -f1)"
if [[ "${agent_line}" -lt "${copy_line}" ]]; then pass "..."; else fail "..."; fi
```

Use `setup_build_mock` (lines 82-96) and `BUILD_PATH` for controlled test execution.

### NPM Package Names (Verified)

- Claude Code CLI: `@anthropic-ai/claude-code` (installs `claude` binary)
- Gemini CLI: `@google/gemini-cli` (installs `gemini` binary)

Both are npm packages requiring Node.js. Install with `npm install -g <package>`.

### Project Structure Notes

- All changes align with the existing project structure and conventions
- No new files needed -- only modifications to existing files
- Naming convention follows established `CFG_AGENT` global variable pattern (already declared at sandbox.sh line ~71)

### References

- [Source: Dockerfile.template lines 49-68] Existing conditional block patterns for SDKs and MCP
- [Source: sandbox.sh lines 261-381] process_template() function with conditional processing
- [Source: sandbox.sh lines 383-412] cmd_build() function with MCP dependency validation
- [Source: sandbox.sh lines 71-85] Global CFG_* variables including CFG_AGENT
- [Source: scripts/entrypoint.sh lines 26-44] Agent routing and command -v checks
- [Source: tests/test_sandbox.sh lines 82-96] setup_build_mock() pattern
- [Source: _bmad-output/planning-artifacts/architecture.md] Template placeholder format and conditional block conventions
- [Source: npmjs.com/@google/gemini-cli] Verified Gemini CLI package name

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None required.

### Completion Notes List

- Added IF_AGENT_CLAUDE and IF_AGENT_GEMINI conditional blocks to Dockerfile.template between SDK blocks and MCP block
- Added conditional processing in process_template() using string equality (`== "claude-code"` / `== "gemini-cli"`) rather than non-empty test
- Added agent dependency validation in cmd_build() after MCP validation — dies with clear error if sdks.nodejs missing
- Existing tag validation in process_template() step 1 automatically recognizes new tags (no additional code needed)
- Updated 65 existing test configs to include `sdks.nodejs: "22"` since the new validation correctly rejects builds without it
- Added 15 new test assertions covering all ACs: claude install, gemini install, missing nodejs errors, ordering, no-cross-contamination
- Fixed test 4.4-6 comment/title to accurately describe cross-contamination test (code review finding)
- Added integration test (test_integration_agent_cli.sh) verifying command -v claude, claude --version, and npm global listing inside a real container (code review finding — AC #5 intent gap)
- All 421 unit tests pass with 0 failures

### File List

- Dockerfile.template (modified) — added IF_AGENT_CLAUDE and IF_AGENT_GEMINI conditional blocks
- sandbox.sh (modified) — added agent conditional processing in process_template() and dependency validation in cmd_build()
- tests/test_sandbox.sh (modified) — added Story 4.4 test section; updated existing test configs to include sdks.nodejs; fixed test 4.4-6 comment
- tests/test_integration_agent_cli.sh (new) — integration test for agent CLI installation inside real container

### Change Log

- 2026-03-25: Implemented agent CLI installation at build time (story 4-4). Added Dockerfile template blocks, template expansion, Node.js dependency validation, and comprehensive tests.
- 2026-03-25: Code review fixes — fixed test 4.4-6 title, added integration test for AC #5.
