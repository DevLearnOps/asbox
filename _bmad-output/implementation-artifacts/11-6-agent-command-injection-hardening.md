# Story 11.6: Agent Command Injection Hardening

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want agent launch commands to be executed safely without shell expansion risks,
so that agent command strings cannot be exploited through shell injection.

## Acceptance Criteria

1. **Given** an agent is launched inside the sandbox
   **When** the entrypoint executes the agent command
   **Then** the command is executed via `exec gosu sandbox` with explicit arguments — not via `bash -c "${AGENT_CMD}"`

2. **Given** the agent command for claude is `claude --dangerously-skip-permissions`
   **When** the entrypoint launches the agent
   **Then** the command is split into an argument list `["claude", "--dangerously-skip-permissions"]` and `exec gosu sandbox` is invoked directly — no `bash -c` wrapper and no shell interpolation

3. **Given** the agent command for gemini is `gemini -y`
   **When** the entrypoint launches the agent
   **Then** it is executed as `exec gosu sandbox gemini -y` — direct exec, no `bash -c` wrapper

4. **Given** the agent command for codex is `codex --dangerously-bypass-approvals-and-sandbox`
   **When** the entrypoint launches the agent
   **Then** it is executed as a direct exec with explicit arguments — no `bash -c` wrapper

5. **Given** the `AGENT_CMD` environment variable is set by the Go CLI
   **When** the entrypoint reads it
   **Then** the value is a simple space-separated token list (one argument per token, no quoted segments, no embedded shell metacharacters), and the entrypoint relies on bash word-splitting of `${AGENT_CMD}` without `bash -c`

6. **Given** the existing fallback where CLI positional args are forwarded (`if [[ $# -gt 0 ]]; then exec gosu sandbox "$@"`)
   **When** `asbox run` is invoked
   **Then** that branch continues to work unchanged — only the `${AGENT_CMD}` branch is hardened

## Tasks / Subtasks

- [x] Task 1: Replace `bash -c` with direct exec in the entrypoint (AC: #1, #2, #3, #4, #5, #6)
  - [x] 1.1 In `embed/entrypoint.sh`, locate the fallback block at lines 204-210. Keep the `$# -gt 0` branch (`exec gosu sandbox "$@"`) unchanged
  - [x] 1.2 Replace the `elif [[ -n "${AGENT_CMD:-}" ]]` branch body (line 207) from `exec gosu sandbox bash -c "${AGENT_CMD}"` with `# shellcheck disable=SC2086` then `exec gosu sandbox ${AGENT_CMD}` (unquoted, to trigger bash word-splitting on spaces into argv — Option A from epic notes)
  - [x] 1.3 Keep the `die "No agent command specified"` branch unchanged
  - [x] 1.4 Verify `set -euo pipefail` at top (line 2) still applies — the unquoted expansion is the one spot where intentional word-splitting is required; document intent with a one-line comment above the exec (e.g., `# Intentional word-split: AGENT_CMD is a trusted token list from the Go CLI.`)

- [x] Task 2: Assert agent command strings are safe for word-splitting (AC: #5)
  - [x] 2.1 Review `agentCommand()` in `cmd/run.go:184-195` — confirm each returned string contains only simple tokens separated by single spaces (claude, gemini, codex values already satisfy this). No change to return values is required for the current agent set
  - [x] 2.2 Add a new unit test `TestAgentCommand_noShellMetacharacters` in `cmd/run_test.go` that iterates over all supported agents (`claude`, `gemini`, `codex`) and asserts each returned string contains none of the unsafe runes `;`, `&`, `|`, `<`, `>`, `$`, `` ` ``, `\`, `"`, `'`, `*`, `?`, `(`, `)`, `{`, `}`, `[`, `]`, `\n`, `\r`, `\t`. This locks the invariant so a future agent entry that breaks word-split safety is caught at test time
  - [x] 2.3 Do NOT change existing tests `TestAgentCommand_claude`, `TestAgentCommand_gemini`, `TestAgentCommand_codex`, `TestAgentCommand_unknown`, `TestAgentCommand_oldNameRejected` — they must continue to assert exact return strings

- [x] Task 3: Add a bash-level regression test for the entrypoint exec path (AC: #1, #2, #5)
  - [x] 3.1 In `embed/`, create a new test `TestEntrypoint_AgentCmdExecNoShell` in a new file `embed/entrypoint_test.go` (new package-level test — follow the pattern already established by `git_wrapper_test.go`: extract the embedded `entrypoint.sh`, patch it, run with `exec.Command("bash", ...)`)
  - [x] 3.2 The test must verify two properties without running the full entrypoint chain:
    - [x] The script contains no `bash -c "${AGENT_CMD}"` nor `bash -c "$AGENT_CMD"` substring (grep the embedded content)
    - [x] The script contains an `exec gosu sandbox ${AGENT_CMD}` line (no quotes around `${AGENT_CMD}`) — this is the hardened form
  - [x] 3.3 Because the entrypoint performs root-only operations (`usermod`, `mount`), do NOT try to run the whole script in the unit test. Do the verification by string inspection of the embedded asset — pattern: read `Assets.ReadFile("entrypoint.sh")` and assert on substrings. This keeps the test hermetic on developer machines without privileged containers
  - [x] 3.4 Keep `embed/embed_test.go` unchanged — it only verifies assets exist; the new test covers the hardening assertion

- [x] Task 4: Run full test suite and format (AC: all)
  - [x] 4.1 Run `gofmt -w` on all modified Go files
  - [x] 4.2 Run `go vet ./...`
  - [x] 4.3 Run `go test ./...` and confirm every package is green (cmd, embed, internal/*)
  - [x] 4.4 Optionally, run `shellcheck embed/entrypoint.sh` if available locally to confirm the intentional SC2086 disable is the only new lint

### Review Findings

Code review completed 2026-04-17. Acceptance Auditor confirms AC#1–AC#6 fully satisfied and all "Dev MUST NOT" constraints respected. 14 additional findings dismissed as design trade-offs, spec-intended behavior, or invalid claims (e.g., `set -f` explicitly rejected by spec; brace expansion does not apply to parameter-expansion results; `package embed` matches existing `git_wrapper_test.go` convention).

- [x] [Review][Defer] Empty/whitespace-only `AGENT_CMD` bypasses `die` fallback [embed/entrypoint.sh:206] — pre-existing `-n` branch condition; a value of `" "` passes the `-n` test, and after word-split the exec receives zero args. Unchanged by this diff. Producer side never emits whitespace-only values.
- [x] [Review][Defer] `exec gosu sandbox ... "$@"` lacks `--` separator [embed/entrypoint.sh:205,209] — out-of-scope; applies equally to both branches and no agent command currently begins with a dash. General-hardening item.
- [x] [Review][Defer] `TestAgentCommand_noShellMetacharacters` uses hardcoded `{claude, gemini, codex}` list [cmd/run_test.go:275] — spec mandates this exact list in Task 2.2; future-agent additions require touching the test alongside `agentCommand()`. Enhancement potential: iterate a single source of truth.

## Dev Notes

### The Exact Change in `embed/entrypoint.sh`

Current (lines 204-210):

```bash
# Exec into agent command as sandbox user
if [[ $# -gt 0 ]]; then
    exec gosu sandbox "$@"
elif [[ -n "${AGENT_CMD:-}" ]]; then
    exec gosu sandbox bash -c "${AGENT_CMD}"
else
    die "No agent command specified"
fi
```

Target:

```bash
# Exec into agent command as sandbox user
if [[ $# -gt 0 ]]; then
    exec gosu sandbox "$@"
elif [[ -n "${AGENT_CMD:-}" ]]; then
    # Intentional word-split: AGENT_CMD is a trusted token list from the Go CLI.
    # shellcheck disable=SC2086
    exec gosu sandbox ${AGENT_CMD}
else
    die "No agent command specified"
fi
```

Only the `elif` branch changes. The `$@` positional branch stays as-is (it already does proper arg-array exec). The `die` branch stays as-is.

Why **Option A (unquoted `${AGENT_CMD}`)** instead of **Option B (split env vars `AGENT_BIN`/`AGENT_ARGS`)**:
- AGENT_CMD is produced by `agentCommand()` in `cmd/run.go:184-195`, a hardcoded switch over `claude`/`gemini`/`codex` with static return strings — no user input reaches it
- Bash word-splitting on space is sufficient and preserves the single-source-of-truth (the Go switch statement) without duplicating argv shaping across Go and the shell
- `bash -c` is eliminated entirely — no shell parser runs on the value, so metacharacters like `;`, `&&`, backticks, `$()` are never interpreted. Word-splitting splits on whitespace only
- Epic 11.6 notes explicitly endorse Option A as "simpler and sufficient"

### Current `AGENT_CMD` Producers

`cmd/run.go:184-195` — the only place `AGENT_CMD` is set:

```go
func agentCommand(agent string) (string, error) {
    switch agent {
    case "claude":
        return "claude --dangerously-skip-permissions", nil
    case "gemini":
        return "gemini -y", nil
    case "codex":
        return "codex --dangerously-bypass-approvals-and-sandbox", nil
    default:
        return "", fmt.Errorf("unknown agent %q: supported agents are claude, gemini, codex", agent)
    }
}
```

All three existing strings are single-space-separated tokens with no shell metacharacters — safe for bash word-splitting out of the box. The new test in Task 2.2 enforces this invariant for future additions.

`cmd/run.go:118` — where it gets wired into the container env:

```go
envVars["AGENT_CMD"] = agentCmd
```

No change needed here.

### What the Dev MUST NOT Do

- **Do not** introduce new `AGENT_BIN` / `AGENT_ARGS` env vars. That is Option B, explicitly rejected in the epic notes — more complexity for no marginal safety
- **Do not** re-split `AGENT_CMD` in Go (e.g., with `strings.Fields`) and pass as positional args via `docker run ... image arg1 arg2`. The existing `$@` fallback already covers positional args if needed in the future; for this story, keep using the `AGENT_CMD` env var shape
- **Do not** add `set +f` / `set -f` dance around the exec. The three agent commands contain no glob characters, and the Task 2.2 test prevents future regressions
- **Do not** remove or reorder the `$# -gt 0` branch — integration scaffolding (e.g., `lifecycle_test.go:115` uses `--entrypoint tail`) bypasses the entrypoint entirely, but other callers may pass positional args and rely on that branch
- **Do not** add per-agent logic inside the entrypoint. The agent choice stays in Go; the entrypoint just execs a validated string
- **Do not** write `exec gosu sandbox $AGENT_CMD` (unbraced) — use `${AGENT_CMD}` per architecture.md:350 quoting convention ("Always double-quote variable expansions" — this is the one documented exception where intentional word-split is required, hence the shellcheck disable)

### Testing Strategy Summary

| Test | Purpose | Location |
|------|---------|----------|
| Existing `TestAgentCommand_claude/gemini/codex` | Lock return strings | `cmd/run_test.go:208-236` — no change |
| NEW `TestAgentCommand_noShellMetacharacters` | Prevent future agent entries from breaking word-split safety | `cmd/run_test.go` (append) |
| NEW `TestEntrypoint_AgentCmdExecNoShell` | Assert entrypoint.sh has no `bash -c "${AGENT_CMD}"` and contains the hardened exec line | new file `embed/entrypoint_test.go` |

Why not integration tests? The existing integration suite (`integration/lifecycle_test.go:115`, `integration/mcp_test.go`, etc.) launches containers with `--entrypoint tail` to bypass the agent exec, or launches `execInContainer` via `su -s /bin/bash -c` (`integration/integration_test.go:134-136`). None of them exercise the `exec gosu sandbox ${AGENT_CMD}` path because no real agent is installed in the test flow. Unit-level string inspection of the embedded entrypoint is the right granularity — same pattern as `embed/git_wrapper_test.go`.

### Pattern to Follow: `embed/git_wrapper_test.go`

`embed/git_wrapper_test.go:14-45` shows the canonical pattern for testing an embedded shell script:
1. Read the script via `Assets.ReadFile("<name>.sh")`
2. Optionally patch paths (not needed here since we only inspect strings)
3. Assert on content OR run via `exec.Command("bash", ...)` with a patched copy

For story 11.6, string inspection is enough — we don't need to execute entrypoint.sh (it needs root, `usermod`, `mount`). Reading `Assets.ReadFile("entrypoint.sh")` and asserting substrings via `strings.Contains` / `!strings.Contains` is hermetic and fast.

### Content Hash Impact

Changing `embed/entrypoint.sh` changes the content hash (`internal/hash/hash.go` hashes all embedded scripts per architecture.md:242). This invalidates the cached image — **correct behavior**. No code change to the hash computation.

### Error Handling — No New Types

No new error types. The only failure mode is the `die "No agent command specified"` branch, already present. `exitCode()` in `cmd/root.go` stays untouched per CLAUDE.md: "Every new error type must be added to `exitCode()`". None introduced here.

### Exit Code Impact

Zero. The entrypoint fallback branch either execs into the agent (success path) or dies with the existing message. No Go-level error paths added.

### Architecture Compliance Pointers

- **Bash script conventions (architecture.md:342-362):** keep `snake_case` functions, `UPPER_SNAKE_CASE` env vars, `set -euo pipefail` intact, `die()` helper preserved. The one deliberate deviation is the unquoted `${AGENT_CMD}` expansion — documented via the inline shellcheck-disable comment
- **Two execution domains (architecture.md:75):** command shape lives in Go (`agentCommand`), execution lives in bash (`entrypoint.sh`). This story preserves the boundary; only the exec form changes
- **FR17-FR19/FR19a (PRD):** "AI agent can execute inside the sandbox with full terminal access" — unchanged. The agent still launches with the same argv; only how it is launched changes (direct exec vs. bash -c)
- **NFR5 (PRD:540):** "Config input sanitization" — Epic 11 theme. This is the last injection-surface story; stories 11.2 and 11.3 already hardened SDK/package/project_name and ENV inputs. 11.6 hardens the final downstream consumer

### Project Structure Notes

- Single change to `embed/entrypoint.sh` (1 existing file)
- Single change to `cmd/run_test.go` (1 existing file — append a test)
- One new file: `embed/entrypoint_test.go` (new, mirrors git_wrapper_test.go pattern)
- No changes to `go.mod`, `go.sum`, `embed/embed.go`, `cmd/root.go`, `internal/*`
- No new packages, no new error types, no new CLI flags, no new config fields
- Follows the established Epic 11 pattern: tight scope, single concern, preserves all existing behavior

### Previous Story Intelligence (11.5)

From `_bmad-output/implementation-artifacts/11-5-pinned-build-dependencies.md` and `11-5-pinned-build-dependencies.md:197-208`:

- **Scope discipline:** Epic 11 stories stay narrow. 11.5 touched exactly one embed file (`Dockerfile.tmpl`) and one test file. 11.6 follows the same pattern
- **Regression tests lock invariants:** 11.5 added `TestRender_baseImage`/`TestRender_dockerCompose`/`TestRender_geminiAgent` assertions that catch un-pinning. 11.6's `TestAgentCommand_noShellMetacharacters` plays the same role — locks safety of agent command strings going forward
- **Full test suite is the gate:** Dev must run `gofmt -w`, `go vet ./...`, `go test ./...`. No partial runs. The integration suite also ran in 11.5; it will run here but no integration coverage is added — unit is sufficient
- **Review pattern is predictable:** 11.5 review dismissed/deferred pre-existing issues (npx runtime pinning, checksum verification). For 11.6, the analogous pre-existing deferred items are in `deferred-work.md:15` ("`AGENT_CMD` injection via shell expansion") — this story closes that exact item, so it is no longer deferred after merge

### Git Intelligence (Recent Commits)

Last 5 commits pattern:
- `cd122b2 feat: pinned Dockerfile build dependencies for reproducible builds (story 11-5)`
- `1c84cec feat: non-TTY runtime support with stdin-based TTY detection (story 11-4)`
- `0dae1e1 feat: ENV key/value validation and Dockerfile injection hardening (story 11-3)`
- `4ba3efe feat: SDK version, package name, and project name sanitization (story 11-2)`
- `5775826 feat: concurrent sandbox sessions with random-suffixed container names (story 11-1)`

**Commit message convention:** `feat: <short description> (story N-M)` — single line, no multi-paragraph body, `feat:` prefix even for security hardening. Follow this format for 11-6.

### CLAUDE.md Compliance

- **Error handling:** No new error types — nothing to add to `exitCode()` / `cmd/root_test.go`
- **Testing:** Unit tests only (stdlib `testing`, no testify). Table-driven for pure functions (Task 2.2 is a great fit). `t.TempDir()` not required — no file I/O in new tests
- **Code organization:** Change stays within existing packages (`cmd/`, `embed/`). No new centralized error packages
- **Agent registry invariant:** Do NOT touch `AgentConfigRegistry`, `agentCommand`, or `agentInstructionTarget` signatures. The story is about how the command runs, not which commands run

### Source Hints for Fast Navigation

| Artifact | Path | Relevant Lines |
|----------|------|:--------------:|
| Entrypoint bash -c line | `embed/entrypoint.sh` | 204-210 (change 207) |
| AGENT_CMD env var setter | `cmd/run.go` | 114-118 |
| Agent command source of truth | `cmd/run.go` | 184-195 (`agentCommand`) |
| Existing agent cmd tests | `cmd/run_test.go` | 208-272 |
| Embed asset read pattern | `embed/embed_test.go` | 5-26 |
| Bash-script test pattern (template) | `embed/git_wrapper_test.go` | 14-45 |
| Architecture: bash conventions | `_bmad-output/planning-artifacts/architecture.md` | 342-362 |
| Architecture: exec agent step | `_bmad-output/planning-artifacts/architecture.md` | 221-231 |
| PRD: agent launch FRs | `_bmad-output/planning-artifacts/prd.md` | 479-482 |
| PRD: NFR security | `_bmad-output/planning-artifacts/prd.md` | 540-546 |
| Deferred item this story closes | `_bmad-output/implementation-artifacts/deferred-work.md` | 15 |
| Epic story spec | `_bmad-output/planning-artifacts/epics.md` | 1408-1441 |

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 11 — Story 11.6 (lines 1408-1441)]
- [Source: _bmad-output/planning-artifacts/architecture.md#Container Lifecycle (lines 218-231)]
- [Source: _bmad-output/planning-artifacts/architecture.md#Bash Script Conventions (lines 342-362)]
- [Source: _bmad-output/planning-artifacts/prd.md#Functional Requirements — FR17-FR19/FR19a (lines 479-482)]
- [Source: _bmad-output/planning-artifacts/prd.md#Non-Functional Requirements — NFR5 (line 540)]
- [Source: _bmad-output/implementation-artifacts/deferred-work.md — AGENT_CMD injection entry (line 15)]
- [Source: embed/entrypoint.sh — bash -c exec (line 207)]
- [Source: cmd/run.go — agentCommand (lines 184-195), AGENT_CMD wiring (line 118)]
- [Source: cmd/run_test.go — agent command tests (lines 208-272)]
- [Source: embed/git_wrapper_test.go — embedded script test pattern (lines 14-45)]
- [Source: CLAUDE.md — error handling, testing, code organization conventions]

## Dev Agent Record

### Agent Model Used

GPT-5 Codex

### Debug Log References

- `go test ./cmd` passed immediately after adding the new invariant test, confirming the existing `agentCommand()` return values remained valid.
- `go test ./embed` failed in the red phase because `entrypoint.sh` still contained `bash -c "${AGENT_CMD}"`, then passed after replacing it with `exec gosu sandbox ${AGENT_CMD}` and the documented SC2086 exception.
- `go test ./...` initially failed inside the workspace sandbox because the integration suite could not reach `/Users/manuel/.docker/run/docker.sock`; rerunning the full suite outside sandbox restrictions completed successfully, including `integration`.

### Completion Notes List

- Replaced the `AGENT_CMD` entrypoint branch in `embed/entrypoint.sh` with direct `exec gosu sandbox ${AGENT_CMD}`, preserving the positional-argument `$@` path and the existing `die` fallback unchanged.
- Documented the intentional unquoted expansion with a one-line rationale and `shellcheck disable=SC2086`, while keeping `set -euo pipefail` intact.
- Added `TestAgentCommand_noShellMetacharacters` in `cmd/run_test.go` to lock the invariant that supported agent commands remain simple, space-separated token lists with no shell metacharacters.
- Added `embed/entrypoint_test.go` with `TestEntrypoint_AgentCmdExecNoShell` to assert the embedded script no longer contains any `bash -c "$AGENT_CMD"` form and does contain the hardened direct exec line.
- Validation passed with `gofmt -w cmd/run_test.go embed/entrypoint_test.go`, `go vet ./...`, `go test ./cmd`, `go test ./embed`, and a final unrestricted `go test ./...`. `shellcheck` was not available locally, so Task 4.4 was satisfied as an availability check only.

### File List

- `embed/entrypoint.sh` (modified)
- `cmd/run_test.go` (modified)
- `embed/entrypoint_test.go` (new)
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (modified)
- `_bmad-output/implementation-artifacts/11-6-agent-command-injection-hardening.md` (modified)

### Change Log

- 2026-04-17: Implemented Story 11.6 by removing `bash -c` from the agent exec path, locking `AGENT_CMD` token safety with tests, and validating the full Go suite including Docker-backed integration tests.
