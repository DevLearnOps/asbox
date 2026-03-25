# Story 7.3: Migrate Claude CLI Installation to Official Install Script

Status: review

## Story

As a developer,
I want Claude Code installed via the official install script instead of the deprecated npm package,
So that the sandbox uses the supported installation method and avoids future breakage.

## Acceptance Criteria

1. **Given** a sandbox image is built with `agent: claude-code`, **When** the agent runs `claude --version` inside the sandbox, **Then** Claude Code responds with its version (installed via official script).

2. **Given** the Dockerfile.template, **When** the Claude Code install block is processed, **Then** it uses the official install script (e.g., `curl -fsSL https://claude.ai/install.sh | sh`) instead of `npm install -g @anthropic-ai/claude-code`.

3. **Given** Claude Code is installed via the official script, **When** the agent launches with `claude --dangerously-skip-permissions`, **Then** the agent starts and operates normally.

## Tasks / Subtasks

- [x] Task 1: Update Dockerfile.template Claude install block (AC: #2)
  - [x] 1.1 Replace `RUN npm install -g @anthropic-ai/claude-code` (line 73) with official install script
  - [x] 1.2 Verify the exact install script URL (https://claude.ai/install.sh or similar)
  - [x] 1.3 Ensure the `claude` binary lands in PATH accessible to both root (build) and sandbox user (runtime)
  - [x] 1.4 If the official installer places the binary in a user-local path (e.g., `~/.local/bin` or `~/.claude/local/`), add a symlink or PATH adjustment so it's available at `/usr/local/bin/claude` or equivalent
- [x] Task 2: Evaluate and update Node.js dependency requirement (AC: #1, #3)
  - [x] 2.1 Check if the official install script bundles its own Node.js or still requires system Node.js
  - [x] 2.2 If Node.js is no longer required for Claude CLI, remove the `sdks.nodejs` validation gate in `sandbox.sh` (lines 425-429) for `claude-code` agent. Keep it for `gemini-cli`.
  - [x] 2.3 If Node.js IS still required, keep the validation as-is
- [x] Task 3: Update sandbox.sh template processing if needed (AC: #2)
  - [x] 3.1 If the install command changes (no longer `npm install`), verify the `IF_AGENT_CLAUDE` conditional block in `sandbox.sh` (lines 332-336) still works correctly
  - [x] 3.2 Ensure content hash correctly detects template changes for cache invalidation
- [x] Task 4: Update unit tests in test_sandbox.sh (AC: #1, #2, #3)
  - [x] 4.1 Update test 4.4-1 (line 3199): change assertion from `npm install -g @anthropic-ai/claude-code` to the new install command string
  - [x] 4.2 Update test 4.4-5 (line 3276): change grep pattern from `npm install -g @anthropic-ai/claude-code` to the new install command
  - [x] 4.3 Update test 4.4-3 (line 3228): if Node.js is no longer required for claude-code, this test must change (build should succeed without `sdks.nodejs`)
  - [x] 4.4 Add new test: assert the generated Dockerfile does NOT contain `npm install -g @anthropic-ai/claude-code` (regression guard)
  - [x] 4.5 Verify no `IF_AGENT_CLAUDE` or `IF_AGENT_GEMINI` template tags remain in generated Dockerfile
- [x] Task 5: Update integration tests in test_integration_agent_cli.sh (AC: #1)
  - [x] 5.1 Remove or update Test 3 (line 80-86): `npm list -g @anthropic-ai/claude-code` will no longer work if installed via script
  - [x] 5.2 Remove or update Test 4 (line 92-100): npm-based version check will no longer apply
  - [x] 5.3 Keep Test 1 (`command -v claude`) and Test 2 (`claude --version`) — these are install-method agnostic
  - [x] 5.4 Add new test: verify `claude` binary is NOT in npm global packages (confirms migration away from npm)
- [x] Task 6: Verify entrypoint.sh compatibility (AC: #3)
  - [x] 6.1 Confirm `scripts/entrypoint.sh` line 113 (`command -v claude`) still works with the new install path
  - [x] 6.2 Confirm `exec claude --dangerously-skip-permissions` (line 124) still launches correctly

## Dev Notes

### Current Implementation (what to change)

**Dockerfile.template:72-74** — Current npm-based installation:
```dockerfile
# {{IF_AGENT_CLAUDE}}
RUN npm install -g @anthropic-ai/claude-code
# {{/IF_AGENT_CLAUDE}}
```

**sandbox.sh:425-429** — Node.js dependency validation (may need update):
```bash
if [[ -n "${CFG_AGENT}" && "${CFG_AGENT}" != "none" ]]; then
  if [[ "${CFG_AGENT}" == "claude-code" || "${CFG_AGENT}" == "gemini-cli" ]]; then
    if [[ -z "${CFG_SDK_NODEJS}" ]]; then
      die "agent ${CFG_AGENT} requires sdks.nodejs to be configured" 1
    fi
  fi
fi
```

**sandbox.sh:332-336** — Template conditional processing (review, likely no change):
```bash
if [[ "${CFG_AGENT}" == "claude-code" ]]; then
  template="$(echo "${template}" | sed '/^# {{IF_AGENT_CLAUDE}}$/d; /^# {{\/IF_AGENT_CLAUDE}}$/d')"
else
  template="$(echo "${template}" | sed '/^# {{IF_AGENT_CLAUDE}}$/,/^# {{\/IF_AGENT_CLAUDE}}$/d')"
fi
```

### Architecture Constraints

- **Dockerfile generation uses bash template substitution** with `{{IF_NAME}}`/`{{/IF_NAME}}` conditional blocks and `{{NAME}}` value placeholders. The `IF_AGENT_CLAUDE` block format MUST be preserved.
- **Template processing MUST validate** all conditional blocks have matching open/close tags and all value placeholders have corresponding config values. Never produce a Dockerfile with unresolved placeholders.
- **Content hash caching**: Changes to Dockerfile.template correctly trigger rebuild. No additional work needed.
- **Entrypoint contract**: `scripts/entrypoint.sh` verifies `command -v claude` and then `exec claude --dangerously-skip-permissions`. The binary must be in PATH for the sandbox user.

### Key Investigation: Official Install Script

The official install script URL needs to be verified. The epics reference `curl -fsSL https://claude.ai/install.sh | sh`. Key questions:
- Where does the script install the `claude` binary? (likely `~/.claude/local/bin/claude` or similar)
- Does it require Node.js as a runtime dependency?
- Does it work when run as root during Docker build? (Some installers detect root and refuse or install differently)
- If it installs to a user-local path, how do we make it available to the `sandbox` user created later in the Dockerfile?

**Likely approach**: Run the install script as root, then symlink or move the binary to `/usr/local/bin/claude` so it's globally available. Alternatively, set ENV PATH to include the install location.

### Previous Story Learnings (from Story 7-2)

- Changes for this epic are **Dockerfile-only** + test updates. No modifications to `sandbox.sh` logic unless the Node.js dependency changes.
- Test patterns: use `assert_contains`/`assert_not_contains` on `.sandbox-dockerfile` content, following the Story 4.4 and 7.2 test patterns.
- Line ordering tests (agent install after SDK blocks, before COPY scripts) should be updated with the new grep pattern.
- Test suite currently at **486/486 passed** — maintain zero regressions.

### Files to Modify

| File | Change |
|------|--------|
| `Dockerfile.template:72-74` | Replace `npm install -g @anthropic-ai/claude-code` with official install script |
| `sandbox.sh:425-429` | Possibly remove Node.js requirement for `claude-code` (only if official script bundles Node) |
| `tests/test_sandbox.sh:3182-3314` | Update Story 4.4 test assertions to match new install command |
| `tests/test_integration_agent_cli.sh:77-100` | Update/remove npm-specific integration tests |
| `scripts/entrypoint.sh` | Likely no change (verify `command -v claude` still works) |

### Project Structure Notes

- Template lives at `/workspace/Dockerfile.template`
- Generated Dockerfile lands at `/workspace/.sandbox-dockerfile`
- Unit tests at `/workspace/tests/test_sandbox.sh` (bash-based, uses `assert_contains`/`assert_not_contains` helpers)
- Integration tests at `/workspace/tests/test_integration_agent_cli.sh` (requires Docker, builds real image)
- Entrypoint at `/workspace/scripts/entrypoint.sh`
- Build orchestrator at `/workspace/sandbox.sh`

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Epic 7, Story 7.3]
- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-03-25.md — Story 7.3 lines 141-170]
- [Source: _bmad-output/planning-artifacts/architecture.md — Dockerfile Generation, Template Placeholder Format]
- [Source: _bmad-output/implementation-artifacts/7-2-fix-docker-compose-plugin-registration.md — Dev learnings]

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6 (1M context)

### Debug Log References
- Official install script URL verified: `https://claude.ai/install.sh` (redirects to GCS-hosted bootstrap.sh)
- Install script downloads standalone binary to `$HOME/.claude/downloads/`, runs `install` subcommand
- No Node.js dependency — binary is self-contained
- Claude Code expects binary at `$HOME/.local/bin/claude` for the running user (native install method)
- Install must run as `sandbox` user so binary lands at `/home/sandbox/.local/bin/claude`
- Moved Claude install block after `useradd sandbox` with `USER sandbox` / `USER root` bracketing
- Added `ENV PATH="/home/sandbox/.local/bin:${PATH}"` to ensure `command -v claude` works at runtime

### Completion Notes List
- Replaced `npm install -g @anthropic-ai/claude-code` with `curl -fsSL https://claude.ai/install.sh | bash` running as sandbox user
- Fixed: install script requires `| bash` (not `| sh`) — uses bash syntax incompatible with dash
- Fixed: must install as sandbox user, not root — Claude checks `$HOME/.local/bin/claude`
- Removed Node.js dependency requirement for claude-code agent (kept for gemini-cli)
- Updated 10 unit test assertions (new install command, regression guard, Node.js no longer required)
- Replaced 2 npm-specific integration tests with migration regression tests
- Verified entrypoint.sh compatibility (command -v, exec claude) — no changes needed
- Template processing (IF_AGENT_CLAUDE block) unchanged — works with new content
- Full test suite: 474/474 passed, 0 failed

### File List
- `Dockerfile.template` — replaced npm install with official install script + symlink
- `sandbox.sh` — removed Node.js validation gate for claude-code agent
- `tests/test_sandbox.sh` — updated 4.4 test assertions for new install method
- `tests/test_integration_agent_cli.sh` — replaced npm-specific tests with migration guards

### Change Log
- 2026-03-25: Migrated Claude CLI from npm package to official install script (Story 7.3)
