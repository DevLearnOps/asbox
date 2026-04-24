# Story 16.1: Configurable Agent Instructions Extension

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want to configure a path to a project-local markdown file that gets appended to the agent's instruction file,
so that my project-specific constraints and conventions are visible to whichever agent I launch — without forking asbox or duplicating a `CLAUDE.md` / `GEMINI.md` / `AGENTS.md` per agent.

## Acceptance Criteria

1. **Given** a developer sets `agent_instructions: ./AGENT_INSTRUCTIONS.md` in `.asbox/config.yaml` and the file exists
   **When** they run `asbox run`
   **Then** the rendered agent instruction file mounted into the container contains the extension file's contents under a trailing `## Project-Specific Instructions` section, verbatim (no transformation of the extension body)

2. **Given** a developer sets `agent_instructions` and installs both `claude` and `gemini`
   **When** they launch with `-a claude` and then separately with `-a gemini`
   **Then** both runs produce an instruction file with the same project-specific section — the mount target differs per agent via `agentInstructionTarget()` (`/home/sandbox/CLAUDE.md` vs `/home/sandbox/GEMINI.md`) but the template output is agent-agnostic

3. **Given** `agent_instructions` is set but the file does not exist at launch
   **When** the CLI runs
   **Then** it exits with code 1 and prints exactly `error: agent_instructions path '<resolved-path>' not found. Check agent_instructions in .asbox/config.yaml` to stderr — no temp file written, no partial mount, no `docker run` invocation

4. **Given** `agent_instructions` is set and the file exists but is not readable (permissions denied, non-regular-file, or any other read error that is not `os.IsNotExist`)
   **When** the CLI runs
   **Then** it exits with code 1 and prints exactly `error: agent_instructions path '<resolved-path>' is not readable: <underlying-err>. Check agent_instructions in .asbox/config.yaml` to stderr

5. **Given** `agent_instructions` is unset or the empty string
   **When** the CLI runs with `bmad_repos` also unset
   **Then** behavior is byte-identical to today — no runtime instruction file is generated, no mount is added, and the build-time `CLAUDE.md` / `GEMINI.md` / `AGENTS.md` baked into the image is used as-is

6. **Given** `agent_instructions` is set and `bmad_repos` is empty
   **When** the CLI runs
   **Then** the instruction file is still rendered, written to a temp file, and mounted at `agentInstructionTarget(cfg.DefaultAgent)` — the mount is no longer gated solely on `len(bmadMounts) > 0`. The `bmad_repos: mounting N repositories` log line is NOT emitted (because there are no repo mounts)

7. **Given** `agent_instructions` is set and `bmad_repos` is also non-empty
   **When** the CLI runs
   **Then** the rendered file contains BOTH the `## Multi-Repo Workspace` section (from `bmad_repos`) AND the trailing `## Project-Specific Instructions` section (from `agent_instructions`), in that order, with no leading blank line between the extension heading and the `Best Practices` section that precedes it

8. **Given** the `agent_instructions` value is a relative path (e.g. `./AGENT_INSTRUCTIONS.md`, `../shared/agent-notes.md`, `~/notes.md`)
   **When** the CLI resolves it
   **Then** it is resolved relative to the directory containing the config file (same helper as `mounts` and `bmad_repos` — `resolvePath(configDir, ...)` in `internal/config/parse.go`), with tilde expansion for `~/` prefixes. Absolute paths pass through unchanged

## Tasks / Subtasks

- [x] **Task 1: Add `AgentInstructions` field to `Config` struct** (AC: #1, #5, #8)
  - [x] 1.1 In `internal/config/config.go`, add `AgentInstructions string \`yaml:"agent_instructions"\`` to the `Config` struct — place it immediately after `BmadRepos []string` (line 52) so related multi-instruction fields stay co-located
  - [x] 1.2 No changes to `parse.go` validation logic — empty string is a valid value (means "not configured"). No type validation needed (it's a string; any YAML scalar parses)
  - [x] 1.3 In `internal/config/parse.go`, add `cfg.AgentInstructions = resolvePath(configDir, cfg.AgentInstructions)` immediately after the `bmad_repos` resolution block (lines 204-207). Skip the call when the field is empty to preserve "unset means unset" semantics: `if cfg.AgentInstructions != "" { cfg.AgentInstructions = resolvePath(configDir, cfg.AgentInstructions) }`. This matches the existing `mounts` and `bmad_repos` resolution pattern — path resolution at parse time, existence validation at runtime

- [x] **Task 2: Rename `internal/mount/bmad_repos.go` → `internal/mount/agent_instructions.go` and broaden the function** (AC: #1, #3, #4, #5, #6, #7)
  - [x] 2.1 Rename file via `git mv internal/mount/bmad_repos.go internal/mount/agent_instructions.go` (preserves git blame/history — do NOT copy+delete)
  - [x] 2.2 Rename function `AssembleBmadRepos` → `AssembleAgentInstructions`. Signature stays `func AssembleAgentInstructions(cfg *config.Config) ([]string, string, error)` — the first return is still the bmad_repos mount flags (callers like the `auto_isolate_deps` scan still depend on this), the second is the rendered instruction content
  - [x] 2.3 Broaden the early-return guard at the top of the function: `if len(cfg.BmadRepos) == 0 && cfg.AgentInstructions == "" { return nil, "", nil }` — both unset is the only case that short-circuits
  - [x] 2.4 Keep the existing bmad_repos validation + mount assembly (lines 35-76 in the current file) unchanged. It runs when `len(cfg.BmadRepos) > 0`. When `cfg.BmadRepos` is empty but `cfg.AgentInstructions` is set, the `mounts` slice stays empty and the `repos` slice stays empty — the `for _, repoPath := range cfg.BmadRepos` loop is a no-op
  - [x] 2.5 After the bmad_repos loop, before template rendering, read the extension file IF `cfg.AgentInstructions != ""`:
    ```go
    var projectExtension string
    if cfg.AgentInstructions != "" {
        data, err := os.ReadFile(cfg.AgentInstructions)
        if err != nil {
            if os.IsNotExist(err) {
                return nil, "", &config.ConfigError{
                    Msg: fmt.Sprintf("agent_instructions path '%s' not found. Check agent_instructions in .asbox/config.yaml", cfg.AgentInstructions),
                }
            }
            return nil, "", &config.ConfigError{
                Msg: fmt.Sprintf("agent_instructions path '%s' is not readable: %s. Check agent_instructions in .asbox/config.yaml", cfg.AgentInstructions, err),
            }
        }
        projectExtension = string(data)
    }
    ```
    Note: the path in `cfg.AgentInstructions` is already absolute (resolved in Task 1.3) — no re-resolution here
  - [x] 2.6 Extend `InstructionData` struct (line 22-25) with `ProjectExtension string`. Final shape:
    ```go
    type InstructionData struct {
        BmadRepos        []BmadRepoInfo
        ProjectExtension string
    }
    ```
  - [x] 2.7 Pass `projectExtension` into the template data when calling `tmpl.Execute`: `InstructionData{BmadRepos: repos, ProjectExtension: projectExtension}`
  - [x] 2.8 Update the top-level doc comment on `AssembleAgentInstructions` to describe BOTH responsibilities: bmad_repos mount assembly (when configured) AND project-extension file read + template rendering (when configured). Do NOT leave the comment saying "bmad_repos" only

- [x] **Task 3: Rename `internal/mount/bmad_repos_test.go` → `internal/mount/agent_instructions_test.go` and extend tests** (AC: #1, #3, #4, #5, #6, #7, #8)
  - [x] 3.1 `git mv internal/mount/bmad_repos_test.go internal/mount/agent_instructions_test.go` — preserve history
  - [x] 3.2 Rename all existing test function identifiers from `TestAssembleBmadRepos_*` → `TestAssembleAgentInstructions_*`. The body of each existing test stays unchanged — they all exercise the bmad_repos-only path, which is preserved byte-identically (regression guard for AC #5 and AC #7's bmad_repos half)
  - [x] 3.3 Add `TestAssembleAgentInstructions_extensionFileAppended` (AC #1): write a temp file with known content `"## Project Rules\n\nAlways run `go fmt`.\n"`, set `cfg.AgentInstructions` to its absolute path, assert returned instruction content contains `## Project-Specific Instructions`, contains the exact file body verbatim, and contains NO `## Multi-Repo Workspace` section (bmad_repos empty)
  - [x] 3.4 Add `TestAssembleAgentInstructions_extensionMissingFile_returnsConfigError` (AC #3): set `cfg.AgentInstructions = "/nonexistent/path/agent-instructions.md"`, assert `errors.As(err, &ce)` with `*config.ConfigError`, assert error message is exactly `agent_instructions path '/nonexistent/path/agent-instructions.md' not found. Check agent_instructions in .asbox/config.yaml`
  - [x] 3.5 Add `TestAssembleAgentInstructions_extensionUnreadableFile_returnsConfigError` (AC #4): create a file, `os.Chmod` it to `0o000`, `t.Cleanup(func() { os.Chmod(path, 0o644) })` so TempDir cleanup works on Linux, set `cfg.AgentInstructions` to it, assert ConfigError with `is not readable` substring. Skip the test on non-Linux platforms with `if runtime.GOOS != "linux" { t.Skip(...) }` — macOS chmod 000 on a file does not prevent `os.ReadFile` by the file owner, and Windows permission semantics differ entirely. (Story 8-1 ran on darwin/linux and this edge case was not covered — explicit skip is the clean fix.)
  - [x] 3.6 Add `TestAssembleAgentInstructions_extensionOnlyNoBmadRepos` (AC #6): set ONLY `cfg.AgentInstructions` (leave `cfg.BmadRepos` nil), assert `mounts == nil`, assert `content != ""`, assert content contains `## Project-Specific Instructions` and does NOT contain `## Multi-Repo Workspace`
  - [x] 3.7 Add `TestAssembleAgentInstructions_bothBmadReposAndExtension` (AC #7): set both `cfg.BmadRepos` (two temp dirs) AND `cfg.AgentInstructions`. Assert `len(mounts) == 2`, assert content contains BOTH `## Multi-Repo Workspace` AND `## Project-Specific Instructions`, and assert `## Project-Specific Instructions` appears AFTER `## Multi-Repo Workspace` in the rendered output (use `strings.Index` and compare)
  - [x] 3.8 Add `TestAssembleAgentInstructions_extensionBodyVerbatim` (AC #1 detail): write a fixture file containing markdown with its own headings, lists, and code fences (e.g. `"# Our Conventions\n\n- Always X\n- Never Y\n\n\`\`\`bash\nmake test\n\`\`\`\n"`) and assert the rendered output contains the EXACT body string (no template processing, no escaping, no extra blank lines injected into the body itself)

- [x] **Task 4: Update `embed/agent-instructions.md.tmpl` with the trailing `{{- if .ProjectExtension}}` block** (AC: #1, #7)
  - [x] 4.1 Append a new block AFTER the final `- Run tests inside the sandbox...` line of the Best Practices section (line 67). The block must use `{{-` trim markers to prevent leading/trailing blank lines. Exact content to append:
    ```gotemplate
    {{- if .ProjectExtension}}

    ## Project-Specific Instructions

    {{.ProjectExtension}}
    {{- end}}
    ```
  - [x] 4.2 Manual template-render smoke check: with `ProjectExtension = ""`, the rendered output must end exactly at the final `...before signaling completion.` line with a trailing newline — NO dangling `## Project-Specific Instructions` heading, NO blank-line noise. With `ProjectExtension = "body"`, the heading + body appear with one blank line separating them from Best Practices. Verified by the tests in Task 3
  - [x] 4.3 Do NOT wrap the existing `{{- if .BmadRepos}}` block. The two conditional sections are independent. Verify by rendering all four combinations (both unset / bmad only / extension only / both set) in the tests

- [x] **Task 5: Wire `AssembleAgentInstructions` into `cmd/run.go`** (AC: #1, #3, #4, #5, #6)
  - [x] 5.1 Rename the call site at `cmd/run.go:77`: `mount.AssembleBmadRepos(cfg)` → `mount.AssembleAgentInstructions(cfg)`. Signature unchanged — same return tuple, same handling
  - [x] 5.2 The `if len(bmadMounts) > 0 { ... mountFlags = append...; fmt.Fprintf(... "bmad_repos: mounting %d repositories\n" ...) }` block (lines 81-87) stays gated on `len(bmadMounts) > 0`. Do NOT broaden this — the log is about repo mounts, not about the instruction file. When `agent_instructions` is set but `bmad_repos` is empty, this log line must NOT print (AC #6)
  - [x] 5.3 The `if instructionContent != ""` block (lines 88-105) is ALREADY correct. It triggers whenever EITHER bmad_repos OR the extension produces content. No code change needed here — the temp file is written, `agentInstructionTarget(cfg.DefaultAgent)` resolves the mount target, and the mount flag is appended to `mountFlags`. Verify by inspection; no edit required
  - [x] 5.4 Update the comment at line 76 from `// Mount BMAD multi-repo directories and generate agent instructions` → `// Mount BMAD multi-repo directories and/or render project-specific agent instructions`
  - [x] 5.5 No changes to the `auto_isolate_deps` block (lines 107-126). It references `cfg.BmadRepos` directly, which is unchanged semantically
  - [x] 5.6 No changes to `agentInstructionTarget()` (lines 331-342) — target-per-agent mapping is already correct for claude/gemini/codex. The error-message prefix `bmad_repos: unsupported agent %q` at line 340 is now slightly inaccurate (could also fire for agent_instructions). Change it to `instruction file: unsupported agent %q for instruction file mount` to de-couple the phrasing from `bmad_repos`

- [x] **Task 6: Update `cmd/run_test.go` bmad_repos tests to reference the renamed function** (AC: #5, #6, #7)
  - [x] 6.1 Update all `mount.AssembleBmadRepos(cfg)` references → `mount.AssembleAgentInstructions(cfg)`. Current call sites: `cmd/run_test.go:705`, `cmd/run_test.go:737`
  - [x] 6.2 Rename test functions that reference bmad_repos in their name for clarity. Keep body unchanged (they still exercise the bmad_repos path):
    - `TestRun_bmadReposNonexistentPath_returnsConfigError` — stays, this is a bmad_repos path-validation test
    - `TestRun_bmadReposConfigured_assembleBmadReposCalled` → `TestRun_bmadReposConfigured_assembleAgentInstructionsCalled`
    - `TestRun_bmadReposEmpty_noAdditionalMounts` — stays, tests the both-unset short-circuit
  - [x] 6.3 Add a new binary-invocation test `TestRun_agentInstructionsConfigured_extensionMountCreated` (AC #6): write a fixture markdown file inside the temp config dir, write a config that sets `agent_instructions: ./fixture.md` and leaves `bmad_repos` empty, parse the config, call `mount.AssembleAgentInstructions(cfg)`, assert `mounts == nil` (no bmad_repos entries), assert `content != ""` and contains `## Project-Specific Instructions` + the fixture body
  - [x] 6.4 Add `TestRun_agentInstructionsMissingFile_returnsConfigError` (AC #3): write a config pointing to a non-existent path, run the full `run` command via `r := newRootCmd(); r.run("run")`, assert `errors.As(err, &ce)` with `*config.ConfigError` and `exitCode(err) == 1`
  - [x] 6.5 Update `TestAgentInstructionTarget_unknown` (line 853-858) assertion if you tightened the error message in Task 5.6 — the test currently just checks `err == nil` non-nil, so a message change does NOT require a test update

- [x] **Task 7: Update `embed/config.yaml` starter with a commented-out example** (AC: #1)
  - [x] 7.1 After the existing `# bmad_repos:` commented block (currently lines 60-64), add:
    ```yaml

    # Optional: append project-specific instructions to the generated agent
    # instruction file (CLAUDE.md / GEMINI.md / AGENTS.md). Path is resolved
    # relative to this config file. Fail-closed on missing/unreadable file.
    # agent_instructions: ../AGENT_INSTRUCTIONS.md
    ```
  - [x] 7.2 The example intentionally uses `../AGENT_INSTRUCTIONS.md` (project root, since config lives in `.asbox/`) to match the most common placement. Do NOT use a path that suggests the file must be inside `.asbox/`

- [x] **Task 8: Verify integration test coverage — DO NOT add an integration test unless there is a gap** (AC: #1, #6)
  - [x] 8.1 Read `integration/bmad_repos_test.go` (already exercises the instruction-file mount path via `bmad_repos`) and confirm the rename `AssembleBmadRepos` → `AssembleAgentInstructions` is only in the Go package call sites — the integration test uses the binary via `exec.Command`, so it does NOT reference the Go function and needs NO edit
  - [x] 8.2 Assess: do the unit tests in Task 3 + binary-invocation tests in Task 6 fully cover AC #1-#8? Yes — the instruction-file mount path is already exercised end-to-end by the existing `TestBmadRepos_*` integration tests (they run `asbox run -f <path>` and assert the `bmad_repos: mounting N repositories` log). The agent_instructions-only path is covered by the unit + binary tests. NO new integration test required. Record this assessment in the dev-agent completion notes
  - [x] 8.3 If `go test ./...` with `-tags integration` was running locally before the change, re-run it after the change to confirm no regression. If integration tests require Docker and are not in the developer's default loop, document in completion notes that unit + binary tests pass and integration tests were NOT re-run locally

- [x] **Task 9: Build, lint, and full test sweep**
  - [x] 9.1 `go build ./...` — must succeed
  - [x] 9.2 `go vet ./...` — must pass
  - [x] 9.3 `gofmt -l .` — must print nothing (no formatting diffs)
  - [x] 9.4 `go test ./...` — must pass with zero failures, including renamed `internal/mount/agent_instructions_test.go` and updated `cmd/run_test.go`
  - [x] 9.5 Optional but recommended: `go test -run TestAssembleAgentInstructions -v ./internal/mount/...` to visually scan the new test output and confirm all 6 new + 8 renamed tests ran

### Review Findings

_Code review 2026-04-24. Three parallel layers (Blind Hunter, Edge Case Hunter, Acceptance Auditor) produced 1 patch, 2 defer, ~25 dismissed as noise._

- [x] [Review][Patch] Stale `bmad_repos:` prefix in temp-file error wrappers [cmd/run.go:91, 96] — fixed 2026-04-24: renamed prefix from `bmad_repos:` to `instruction file:` in both `CreateTemp` and `WriteString` error wrappers. Build, vet, gofmt, and unit tests pass.
- [x] [Review][Defer] Pre-existing `~/` expansion silently swallows `os.UserHomeDir` error [internal/config/parse.go:214-224] — deferred, pre-existing. When `HOME` is unset, `resolvePath` silently skips tilde expansion, joins `~/foo` with `configDir`, and surfaces a confusing "not found" later. Affects `mounts`, `bmad_repos`, and now `agent_instructions` equally; not introduced by this story.
- [x] [Review][Defer] Extension file pointing to a directory produces a generic "is not readable" error [internal/mount/agent_instructions.go:82-95] — deferred, minor quality-of-life. `os.ReadFile` on a directory returns `"is not readable: read <path>: is a directory"` via the existing error branch — functionally fail-closed, but inconsistent with `bmad_repos`'s dedicated `IsDir` guard. Could add an `os.Stat`+`info.IsDir()` guard for a clearer diagnostic; not required by any AC.

## Dev Notes

### Why This Story Exists

Today asbox generates one `CLAUDE.md` / `GEMINI.md` / `AGENTS.md` per agent from a single embedded template. Users cannot inject project-specific instructions — things like "our API uses X convention", "never touch `deploy/prod/`", "commits must use conventional-commits format" — without forking asbox or dropping a `CLAUDE.md` at the project root. The project-root file trick works for one agent and silently breaks when the user switches runtime via `--agent`. This violates DRY.

This story ships one new config field (`agent_instructions`) that points to a project-local markdown file. Its contents get appended as a trailing `## Project-Specific Instructions` section to whichever agent instruction file is rendered — uniform across claude/gemini/codex, via the shared template. One extension file, one source of truth, every agent sees it.

**Triggering sprint-change:** `_bmad-output/planning-artifacts/sprint-change-proposal-2026-04-23.md` approved 2026-04-23. Architecture updated (new decision section at `architecture.md:285-304`, new Requirements-to-Structure Mapping row at `architecture.md:743`). PRD updated (new FR68, Configuration Surface bullet, Runtime behavior bullet, example YAML). Epics updated (new Epic 16 + Story 16.1 block).

### What Already Exists (DO NOT Recreate)

- **Template rendering infrastructure**: `internal/mount/bmad_repos.go:78-92` already reads `agent-instructions.md.tmpl` from `asboxEmbed.Assets`, parses via `text/template`, and executes with `InstructionData`. Extend the struct; reuse the rendering code path
- **Temp-file write + instruction-target mount**: `cmd/run.go:88-105` already creates a temp file via `os.CreateTemp("", "asbox-instructions-*.md")`, writes `instructionContent`, calls `defer os.Remove(tmpFile.Name())`, resolves the container target via `agentInstructionTarget(cfg.DefaultAgent)`, and appends the mount flag. This ENTIRE block is unchanged — it already triggers on `instructionContent != ""`, which now includes the extension-only case
- **Agent-instruction target per agent**: `cmd/run.go:331-342` maps claude/gemini/codex to their respective mount targets (`/home/sandbox/CLAUDE.md`, `/home/sandbox/GEMINI.md`, `/home/sandbox/.codex/AGENTS.md`). No changes needed — extension is agent-agnostic by design
- **Path resolution helper**: `internal/config/parse.go:214-224` `resolvePath(baseDir, p string) string` handles tilde expansion and relative-to-absolute. Used by `mounts` and `bmad_repos` today; `agent_instructions` plugs into the same helper
- **`ConfigError` + exit-code mapping**: `internal/config/errors.go` defines `ConfigError{Field, Msg}`. `cmd/root.go:66` already maps `*config.ConfigError` to exit code 1. No changes to error-type surface or exit-code table
- **Embed directive**: `embed/embed.go` already includes `agent-instructions.md.tmpl` in the embedded FS. No `//go:embed` edits needed

### What Must Be Implemented (Deliverable Surface)

1. **`internal/config/config.go`** — add `AgentInstructions string` field to `Config` (one line)
2. **`internal/config/parse.go`** — add conditional `resolvePath` call for `AgentInstructions` (three lines)
3. **`internal/mount/bmad_repos.go` → `internal/mount/agent_instructions.go`** — rename file; rename function; broaden early-return guard; add file-read + error-translate block; extend `InstructionData` struct; pass `ProjectExtension` into template data
4. **`internal/mount/bmad_repos_test.go` → `internal/mount/agent_instructions_test.go`** — rename file + function identifiers; add 6 new tests
5. **`embed/agent-instructions.md.tmpl`** — append 5-line trailing `{{- if .ProjectExtension}}` block
6. **`cmd/run.go`** — rename one call site; update one comment; optionally tighten error-message phrasing in `agentInstructionTarget` (Task 5.6)
7. **`cmd/run_test.go`** — rename two call-site references + one test-function identifier; add 2 new binary-invocation tests
8. **`embed/config.yaml`** — append 4-line commented-out starter example

**Net Go LOC added:** ~80. **Net LOC removed:** 0 (pure extension). **File renames:** 2.

### Anti-Patterns (DO NOT Do These)

- **Do NOT add validation to `parse.go`** beyond the `resolvePath` call. Field emptiness is valid (means "not configured"). Existence/readability validation belongs in `internal/mount/agent_instructions.go` at runtime — same pattern as `mounts` and `bmad_repos` (validated in `AssembleMounts`/`AssembleAgentInstructions`, not at parse time)
- **Do NOT take a `configPath` argument in `AssembleAgentInstructions`**. The path in `cfg.AgentInstructions` is resolved at parse time (Task 1.3) — the mount function just reads the already-absolute path. This keeps the signature symmetric with `AssembleBmadRepos`'s original shape and matches how `AssembleMounts` works. (The sprint-change-proposal-2026-04-23 at section 4.9 originally suggested `AssembleAgentInstructions(cfg, configPath)` — resolving in parse.go is the cleaner, more consistent alternative and matches the rest of the codebase.)
- **Do NOT add `agent_instructions` to the content-hash inputs** (`internal/hash/`). The extension is read at runtime, not baked into the image. Bumping the extension file does NOT trigger a rebuild — matches `bmad_repos` and `host_agent_config` precedent
- **Do NOT pre-process / template / sanitize the extension body**. Verbatim passthrough. The user owns the extension's markdown structure. If they put `{{- something -}}` in their file, it is NOT evaluated — Go template only processes the outer `agent-instructions.md.tmpl`, and `{{.ProjectExtension}}` is a plain string interpolation that does NOT re-parse the inserted content as a template
- **Do NOT rename `BmadRepoInfo` or `InstructionData`**. These are stable shapes used by tests. Only EXTEND `InstructionData` with the new `ProjectExtension` field
- **Do NOT break the existing bmad_repos-only path**. All 8 existing `TestAssembleBmadRepos_*` tests (renamed to `TestAssembleAgentInstructions_*`) must pass with their bodies unchanged. This is the primary regression guard for AC #5
- **Do NOT widen the "bmad_repos: mounting N repositories" log to the extension-only case**. That log is about repo mounts; it must stay gated on `len(bmadMounts) > 0`. AC #6 is explicit: extension-only runs produce an instruction mount WITHOUT the bmad_repos log
- **Do NOT use `os.Exit` inside `internal/mount/`**. Return typed `&config.ConfigError` — the cmd layer maps to exit codes
- **Do NOT add a new integration test** just because the story is new. The existing `integration/bmad_repos_test.go` already exercises the instruction-file-mount path end-to-end via the binary invocation. The new extension-only path is a trivial variation covered sufficiently by unit + binary tests (Task 8.2's assessment)
- **Do NOT use `git add` + `git rm` for the file renames**. Use `git mv` explicitly (Task 2.1 and 3.1) so the diff shows as a rename in review and `git blame` stays intact across the refactor

### Architecture Compliance

**Error handling chain:**
- `agent_instructions.go` returns `&config.ConfigError{Msg: ...}` with the exact messages specified in AC #3 and AC #4
- `cmd/run.go` returns the error to Cobra unchanged
- `cmd/root.go:66` maps `*config.ConfigError` → exit code 1 (already wired — no edits)
- Error-message format: `what failed + why + fix action` (per CLAUDE.md). Both messages end with `Check agent_instructions in .asbox/config.yaml` as the fix action
- Uses `errors.Is`/`errors.As` for error comparison (CLAUDE.md — never bare `==`). `os.IsNotExist(err)` is the one sanctioned non-`errors.Is` check; it is the canonical Go stdlib idiom and remains acceptable [Source: existing pattern in `internal/mount/bmad_repos.go:42` and `internal/mount/mount.go:22`]

**Exit codes:**
- No new error types. `ConfigError` already maps to 1. No changes to `cmd/root.go:50-71` (`exitCode` function) or its test table in `cmd/root_test.go`. Verify the existing `TestExitCode_configError` case (if present) still passes

**Agent registry:**
- Untouched. `agent_instructions` is agent-agnostic. No `AgentConfigRegistry` entry. No `agentCommand` / `agentInstructionTarget` changes besides the optional Task 5.6 error-message phrasing tweak

**File organization:**
- New field sits with other config fields in `internal/config/config.go` (CLAUDE.md: "Error types defined per owning package" — this is a config field, not an error, so it lives with Config)
- Mount assembly logic stays in `internal/mount/` (CLAUDE.md: "All embedded assets in `embed/` with `//go:embed` directives in `embed/embed.go`" — unchanged)
- Import alias `asboxEmbed` for the project's `embed` package is already used at `internal/mount/bmad_repos.go:10` — keep it

**Template rendering:**
- Uses `text/template`, NOT `html/template`. The instruction file is consumed by an LLM, not an HTML renderer — no escaping desired (verbatim passthrough per AC #1)
- Whitespace trim markers (`{{-` `-}}`) are essential on the new block — without them, a blank line appears before `## Project-Specific Instructions` when `BmadRepos` is NOT set (because the preceding `{{- end}}` for the bmad_repos block is absent from that code path). Task 4.1 specifies the markers explicitly

**Fail-closed policy:**
- Missing/unreadable `agent_instructions` → exit 1. Rationale: the user explicitly named a file; a typo silently dropping the project conventions is worse than a hard failure. Matches `bmad_repos` (FR52) — opposite of `host_agent_config` (silent-skip because it's optional OAuth convenience). [Source: architecture.md:289]

### Decision Points The Dev Agent Will Hit

1. **Path resolution location (parse.go vs AssembleAgentInstructions)**: Sprint-change-proposal-2026-04-23 section 4.9 suggested `AssembleAgentInstructions(cfg, configPath)` with resolution inside the mount function. This story selects the cleaner alternative: resolve in `parse.go` alongside `mounts` and `bmad_repos`, keep the mount function signature symmetric with the original `AssembleBmadRepos(cfg)`. Rationale: consistent with existing codebase (two call sites of `resolvePath` in parse.go already), avoids double-handling, and keeps the mount function shape stable. **Follow Task 1.3 + Task 2.x as specified.**
2. **Test skip for unreadable-file case (Task 3.5)**: `os.Chmod(path, 0o000)` on macOS does NOT prevent the file owner from reading the file. Linux does enforce this. Cross-platform testing for the "unreadable" branch requires either `runtime.GOOS == "linux"` skip or a different error-injection technique. Simplest approach: skip on non-linux, document in the test comment. **Use `t.Skip` with `runtime.GOOS` check — do NOT try to invent a portable unreadable-file fixture.**
3. **Template trim markers**: Without `{{-`, the rendered output has a blank line between `Best Practices` and `## Project-Specific Instructions`. With `{{-`, the blank line is consumed. AC #7 doesn't prescribe exact blank-line count, but the rendered markdown must be clean (no double blank lines, no dangling whitespace). **The Task 4.1 block with `{{- if .ProjectExtension}}` and `{{- end}}` produces the correct output. Verify by running the test in Task 3.8.**
4. **Reading the extension as text vs bytes**: `os.ReadFile` returns `[]byte`. Convert to `string` for the template. The template writes it directly via `{{.ProjectExtension}}`. If the file contains non-UTF8 bytes, Go's template engine writes them as-is — no corruption. **Use `string(data)`; no UTF-8 validation needed.**
5. **Empty extension file**: If the user sets `agent_instructions` to an existing but empty file, `projectExtension == ""` and the `{{- if .ProjectExtension}}` block does NOT render. This is correct behavior — an empty file contributes nothing, same as unset. No special-case handling needed. (This case is NOT covered by an explicit test — add one if you want; it's not required for AC coverage.)

### Previous Story Intelligence (14.2 — Code Exploration Tools, merged 2026-04-20 via commit `30c33ae`)

14.2 established:
- **Commit message format:** `feat: <short description> (story N-M)`. For this story: `feat: configurable agent instructions extension (story 16-1)`
- **File-scope discipline:** 14.2 touched exactly the files in its "File-Change Summary" table. Do the same for this story — the File-Change Summary below is the contract
- **Rename via `git mv`:** Not specifically 14.2, but the codebase norm. Preserves blame and shows as rename in `git log --follow`

### Previous Story Intelligence (8.1 — Bmad Multi-Repo Mounts, merged earlier in Epic 8)

8.1 built `bmad_repos.go` from scratch. This story is its first significant modification. Critical learnings from 8.1's review findings (captured in the story's `### Review Findings` section):

- **Temp file cleanup**: `cmd/run.go:93` has `defer os.Remove(tmpFile.Name())` — this is the cleanup pattern for the instruction temp file. It still works correctly when extension content is the sole source (no-op for the non-instruction path). **Do NOT touch the defer.**
- **Agent switch default case**: Story 8.1 originally had no default case in `agentInstructionTarget`; the review required adding one. The current code at `cmd/run.go:339-341` has `default: return "", fmt.Errorf(...)`. **Verify this default case still fires correctly — unknown agent now errors cleanly regardless of which source produced the instruction content.** Optional Task 5.6 tweaks the error-message phrasing to not mention `bmad_repos` specifically
- **Degenerate basename guard**: `bmad_repos.go:57-62` guards against `filepath.Base("/") == "/"` and `filepath.Base(".") == "."`. Preserved by rename. **Do not remove.** Does not apply to `agent_instructions` (which is a file path, not a directory path), but the guard is in the bmad_repos loop, which is unchanged
- **No `:ro` read-only qualifier on mounts**: Deferred issue in story 8.1. The instruction-file mount does NOT use `:ro` today. Do NOT add it as part of this story (scope discipline — that's a separate read-only-mount initiative, if it ever happens)

### Git Intelligence (Recent Commits)

```
44a00a0 docs(correct-course): add configuration to specify custom agent instructions
45e8f5a chore: bump codex version for GPT 5.5 access
56757bf chore: bump codex version
35b42e5 fix(security): update codex to latest version to patch axios vulnerability
30c33ae feat: pre-installed code exploration tools (story 14-2)
```

- **`44a00a0`** is the merge of the sprint-change-proposal-2026-04-23 document itself (PRD + architecture + epics + sprint-status edits). This story implements the code behind that docs change
- **`30c33ae`** (14.2) is the immediate predecessor in sprint flow and the reference for commit-message + file-scope conventions
- **No recent changes to `internal/mount/` or the instruction template** since story 8.1 merged — the code you are modifying is stable. Read the 8.1 review findings (embedded in `_bmad-output/implementation-artifacts/8-1-bmad-multi-repo-mounts-and-agent-instructions.md` under `### Review Findings`) before starting Task 2

### File-Change Summary (Contract)

| File | Change | Why |
|---|---|---|
| `internal/config/config.go` | Edit — add `AgentInstructions string` field | Task 1 (AC #1, #5, #8) |
| `internal/config/parse.go` | Edit — add `resolvePath` call for the new field | Task 1.3 (AC #8) |
| `internal/mount/bmad_repos.go` → `internal/mount/agent_instructions.go` | Rename (git mv) + broaden function + extend struct | Tasks 2 (AC #1, #3, #4, #5, #6, #7) |
| `internal/mount/bmad_repos_test.go` → `internal/mount/agent_instructions_test.go` | Rename (git mv) + extend with 6 new tests | Task 3 (all ACs) |
| `embed/agent-instructions.md.tmpl` | Edit — append `{{- if .ProjectExtension}}` block | Task 4 (AC #1, #7) |
| `cmd/run.go` | Edit — rename call site + comment; optional phrasing tweak | Task 5 (AC #6) |
| `cmd/run_test.go` | Edit — rename references + add 2 new tests | Task 6 (AC #3, #6) |
| `embed/config.yaml` | Edit — append commented-out starter example | Task 7 |
| `_bmad-output/implementation-artifacts/sprint-status.yaml` | Edit (by workflow) — `16-1-*` → `ready-for-dev`, then `review` after code-review, then `done`; `epic-16` → `in-progress` | Workflow automation |

**No changes to:** `cmd/root.go`, `cmd/build.go`, `cmd/init.go`, `internal/docker/`, `internal/hash/`, `internal/template/`, `internal/gitfetch/`, `embed/embed.go`, `embed/Dockerfile.tmpl`, `embed/entrypoint.sh`, `embed/git-wrapper.sh`, `embed/healthcheck-poller.sh`, `integration/*`, `go.mod`, `go.sum`. **If you find yourself editing any of these, stop and re-read the scope.**

### Exit Code Impact

None. No new error types. `ConfigError` already maps to exit code 1. No edits to `cmd/root.go`'s `exitCode()` function or its test table in `cmd/root_test.go`.

### Content Hash Impact

None. The extension file is read at runtime — not baked into the image. Bumping the extension content does NOT change the rendered `Dockerfile.tmpl` or the content of `embed/agent-instructions.md.tmpl`, so the content hash is unchanged. Users do NOT need to `asbox build` after editing the extension — the next `asbox run` picks it up.

The TEMPLATE edit in Task 4 (appending the `{{- if .ProjectExtension}}` block) DOES change the embedded asset bytes, which IS a hash input. The next `asbox build` after this story ships WILL rebuild the image. This is correct — the rendered in-image fallback `CLAUDE.md` (copied during `docker build`) will have the new trailing block shape (rendered with `ProjectExtension == ""`, so the block itself is absent in the fallback — same as today in practice).

### CLAUDE.md Compliance Checklist

- [ ] **Error handling:** Uses `errors.Is()`/`errors.As()` for error comparison in tests. Uses `os.IsNotExist(err)` for file-not-found branch (canonical Go idiom, matches existing `internal/mount/*.go` files). No bare `==` or type switches on errors
- [ ] **Error type registry:** No new error types — reuses `config.ConfigError`. No changes to `exitCode()` in `cmd/root.go` or its test table in `cmd/root_test.go`
- [ ] **Error message format:** Both new error messages follow `what failed + why + fix action`: `agent_instructions path 'X' not found. Check agent_instructions in .asbox/config.yaml` and `agent_instructions path 'X' is not readable: <err>. Check agent_instructions in .asbox/config.yaml`
- [ ] **Testing:** Table-driven NOT used — each new test is a single scenario function, matching the existing style of `bmad_repos_test.go` (individual functions per scenario). Stdlib `testing` only — no testify. `t.TempDir()` for all temp dirs. `t.Cleanup()` for the chmod-restore in Task 3.5 (no `defer`)
- [ ] **Code organization:** `AgentInstructions` field in `Config` (not centralized). Mount assembly + validation in `internal/mount/agent_instructions.go`. Template in `embed/`. No new packages, no `utils` / `helpers`
- [ ] **Agent registry invariant:** Untouched. Extension is agent-agnostic
- [ ] **Import alias:** `asboxEmbed` stays as the import alias for `github.com/mcastellin/asbox/embed` in the renamed `agent_instructions.go` (existing pattern — preserve)
- [ ] **gofmt / go vet:** Must pass with zero output (Task 9)

### Source Hints for Fast Navigation

| Artifact | Path | Relevant Lines |
|---|---|:---:|
| Sprint change proposal (authoritative spec) | `_bmad-output/planning-artifacts/sprint-change-proposal-2026-04-23.md` | 1-329 |
| PRD FR68 | `_bmad-output/planning-artifacts/prd.md` | 490 |
| PRD Configuration Surface bullet | `_bmad-output/planning-artifacts/prd.md` | 248 |
| PRD Runtime behavior bullet | `_bmad-output/planning-artifacts/prd.md` | 367 |
| Architecture decision | `_bmad-output/planning-artifacts/architecture.md` | 285-304 |
| Architecture FR68 row | `_bmad-output/planning-artifacts/architecture.md` | 743 |
| Epics FR68 | `_bmad-output/planning-artifacts/epics.md` | 93 |
| Epics Story 16.1 full block | `_bmad-output/planning-artifacts/epics.md` | 1836-1902 |
| Current `AssembleBmadRepos` | `internal/mount/bmad_repos.go` | 30-95 |
| Current `InstructionData` struct | `internal/mount/bmad_repos.go` | 22-25 |
| Current instruction template | `embed/agent-instructions.md.tmpl` | 1-68 |
| Template `{{if .BmadRepos}}` block | `embed/agent-instructions.md.tmpl` | 46-60 |
| Config struct with `BmadRepos` | `internal/config/config.go` | 40-53 |
| parse.go `resolvePath` helper | `internal/config/parse.go` | 214-224 |
| parse.go `BmadRepos` resolution | `internal/config/parse.go` | 204-207 |
| `AssembleBmadRepos` call site in cmd | `cmd/run.go` | 77-105 |
| `agentInstructionTarget` | `cmd/run.go` | 331-342 |
| Exit code mapping | `cmd/root.go` | 50-71 |
| Story 8.1 (predecessor, review findings) | `_bmad-output/implementation-artifacts/8-1-bmad-multi-repo-mounts-and-agent-instructions.md` | 256-266 |
| Starter config | `embed/config.yaml` | 60-65 (bmad_repos block — insert new block after) |
| Existing bmad_repos integration test | `integration/bmad_repos_test.go` | 1-94 |

### References

- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-04-23.md — Authoritative spec; all scope, rationale, and ACs derive from this document]
- [Source: _bmad-output/planning-artifacts/prd.md#FR68 — Functional requirement definition]
- [Source: _bmad-output/planning-artifacts/architecture.md#Project-Specific Agent Instructions Extension — Architecture decision: template integration, fail-closed policy, path resolution, content-hash impact, decoupling from bmad_repos]
- [Source: _bmad-output/planning-artifacts/epics.md#Story-16.1 — User story + full acceptance criteria + implementation notes]
- [Source: internal/mount/bmad_repos.go — Current `AssembleBmadRepos` function; target of rename + broaden]
- [Source: internal/config/parse.go:204-207 — Reference pattern for `BmadRepos` path resolution — mirror for `AgentInstructions`]
- [Source: cmd/run.go:76-105 — Call site for `AssembleBmadRepos`; temp file + mount-target handling stays byte-identical]
- [Source: embed/agent-instructions.md.tmpl — Template file; target of trailing block append]
- [Source: _bmad-output/implementation-artifacts/8-1-bmad-multi-repo-mounts-and-agent-instructions.md#Review-Findings — Prior review learnings: temp-file defer-remove, default-case agent switch, degenerate-basename guard, deferred :ro discussion]
- [Source: CLAUDE.md — Project conventions: errors.Is/As, error types per-package, stdlib testing only, no testify, t.TempDir, t.Cleanup]

## Dev Agent Record

### Agent Model Used

GPT-5 Codex

### Debug Log References

- 2026-04-24: Resumed broken-session implementation from story start; audited existing partial changes against ACs before editing.
- 2026-04-24: Added explicit parser coverage for `agent_instructions` relative path resolution and `~/` expansion to close AC #8 coverage.
- 2026-04-24: Non-escalated `go test ./...` failed only because sandboxed Docker socket access was denied; escalated rerun passed.

### Implementation Plan

- Extend config parsing with `agent_instructions` as a resolved-but-not-validated string field, matching existing `mounts` and `bmad_repos` path semantics.
- Rename and broaden BMAD instruction assembly so `bmad_repos`, `agent_instructions`, or both can render the shared instruction template while preserving bmad repo mount behavior.
- Append project-specific markdown through the existing `text/template` flow without preprocessing the extension body.
- Cover bmad-only, extension-only, missing/unreadable extension, mixed bmad+extension, and path-resolution cases with focused unit and command tests.

### Completion Notes List

- Implemented `agent_instructions` config support with parse-time path resolution relative to the config file directory and tilde expansion.
- Renamed `AssembleBmadRepos` to `AssembleAgentInstructions`, preserving bmad repo mount flags while allowing extension-only instruction rendering and fail-closed missing/unreadable file errors.
- Updated the embedded agent instruction template with a trailing `## Project-Specific Instructions` block that inserts the extension body verbatim.
- Wired the run command to call the renamed assembler, kept the `bmad_repos: mounting N repositories` log gated on actual repo mounts, and updated the unknown-agent instruction target error wording.
- Added unit and command tests for extension-only rendering, missing/unreadable extension failures, combined bmad+extension output order, verbatim body insertion, and `agent_instructions` path resolution.
- Reviewed `integration/bmad_repos_test.go`; it invokes the binary and does not reference the renamed Go function, so no integration test edit was needed. Existing integration coverage still exercises instruction-file mount plumbing via `bmad_repos`; extension-only behavior is covered by unit and command tests.
- Validation: `go build ./...` passed; `go vet ./...` passed; `gofmt -l .` produced no output; `go test -run TestAssembleAgentInstructions -v ./internal/mount` passed with 13 passing and 1 platform skip on darwin; `go test ./...` passed with Docker access.

### File List

- `_bmad-output/implementation-artifacts/16-1-configurable-agent-instructions-extension.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`
- `cmd/run.go`
- `cmd/run_test.go`
- `embed/agent-instructions.md.tmpl`
- `embed/config.yaml`
- `internal/config/config.go`
- `internal/config/parse.go`
- `internal/config/parse_test.go`
- `internal/mount/agent_instructions.go` (renamed from `internal/mount/bmad_repos.go`)
- `internal/mount/agent_instructions_test.go` (renamed from `internal/mount/bmad_repos_test.go`)

### Change Log

- 2026-04-24: Implemented configurable project-specific agent instructions extension and moved story to review.
