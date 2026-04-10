# Story 8.1: BMAD Multi-Repo Mounts and Agent Instructions

Status: done

## Story

As a developer,
I want to configure multiple repositories that get auto-mounted with generated agent instructions,
so that the agent can work across multiple repos in a unified workspace.

## Acceptance Criteria

1. **Given** a config with `bmad_repos: [/Users/manuel/repos/frontend, /Users/manuel/repos/api]`
   **When** the sandbox launches
   **Then** each repo is mounted at `/workspace/repos/<basename>` (e.g., `/workspace/repos/frontend`, `/workspace/repos/api`)

2. **Given** `bmad_repos` is configured
   **When** the system generates agent instructions
   **Then** a CLAUDE.md (or GEMINI.md) is generated from Go template with the repo list and instructions for git operations within `repos/`, mounted into the container at the agent's home directory path (`/home/sandbox/CLAUDE.md` or `/home/sandbox/GEMINI.md`)

3. **Given** two repo paths resolve to the same basename (e.g., `/Users/manuel/repos/client` and `/Users/manuel/work/client`)
   **When** the developer runs `asbox run`
   **Then** the CLI exits with code 1: `"error: bmad_repos basename collision — 'client' resolves from both /Users/manuel/repos/client and /Users/manuel/work/client. Rename one directory or use symlinks to disambiguate."`

4. **Given** a bmad_repos path that doesn't exist on the host
   **When** the developer runs `asbox run`
   **Then** the CLI exits with code 1: `"error: bmad_repos path '/Users/manuel/repos/missing' not found. Check bmad_repos in .asbox/config.yaml"`

5. **Given** `bmad_repos` is not configured (empty or absent)
   **When** the sandbox launches
   **Then** no additional mounts or instruction files are created; the build-time agent instructions file is used as-is

## Tasks / Subtasks

- [x] Task 1: Create `internal/mount/bmad_repos.go` with `AssembleBmadRepos()` (AC: #1, #3, #4, #5)
  - [x] 1.1 Implement early return: `if len(cfg.BmadRepos) == 0 { return nil, "", nil }` — same pattern as `ScanDeps()` in `isolate_deps.go:26`
  - [x] 1.2 Validate each repo path exists via `os.Stat()` — return `&config.ConfigError` if not found, with message: `"bmad_repos path '%s' not found. Check bmad_repos in .asbox/config.yaml"`
  - [x] 1.3 Validate each repo path is a directory via `info.IsDir()` — same pattern as `mount.go:49`
  - [x] 1.4 Detect basename collisions: build `map[string]string` of basename→full path, error if duplicate with message: `"bmad_repos basename collision — '%s' resolves from both %s and %s. Rename one directory or use symlinks to disambiguate."`
  - [x] 1.5 Generate mount flags: `source:/workspace/repos/<basename>` for each valid repo
  - [x] 1.6 Render agent instructions template from `asboxEmbed.Assets` using Go `text/template` with repo data
  - [x] 1.7 Return `(mounts []string, instructionContent string, error)`

- [x] Task 2: Define template data struct in `bmad_repos.go` (AC: #2)
  - [x] 2.1 Create `BmadRepoInfo` struct with `Name string` (basename) and `ContainerPath string` (`/workspace/repos/<name>`)
  - [x] 2.2 Create `InstructionData` struct that embeds or includes fields needed by agent-instructions.md.tmpl (list of `BmadRepoInfo` entries)
  - [x] 2.3 Populate from `cfg.BmadRepos` using `filepath.Base()` for basenames

- [x] Task 3: Update `embed/agent-instructions.md.tmpl` to be a proper Go template (AC: #2, #5)
  - [x] 3.1 Preserve all existing static content (constraints, tools, working directory, best practices)
  - [x] 3.2 Add conditional `{{if .BmadRepos}}` section with multi-repo workspace instructions
  - [x] 3.3 Include `{{range .BmadRepos}}` listing each repo with its container path
  - [x] 3.4 Add git workflow instructions for repos (commit in each repo independently, push is still blocked)
  - [x] 3.5 Ensure the template renders cleanly with BOTH empty and populated BmadRepos lists

- [x] Task 4: Update `cmd/run.go` to call `AssembleBmadRepos()` and mount instruction file (AC: #1, #2, #5)
  - [x] 4.1 Add `mount.AssembleBmadRepos(cfg)` call after `mount.AssembleMounts()` (line 25), before auto_isolate_deps block (line 41)
  - [x] 4.2 Append returned mount flags to `mountFlags`
  - [x] 4.3 If instructionContent is non-empty: write to temp file via `os.CreateTemp("", "asbox-instructions-*.md")`
  - [x] 4.4 Determine container target path: `/home/sandbox/CLAUDE.md` if `cfg.Agent == "claude-code"`, `/home/sandbox/GEMINI.md` if `cfg.Agent == "gemini-cli"`
  - [x] 4.5 Add instruction file mount: `tempFile:/home/sandbox/CLAUDE.md` (or GEMINI.md) to `mountFlags`
  - [x] 4.6 Log repo count: `fmt.Fprintf(cmd.OutOrStdout(), "bmad_repos: mounting %d repositories\n", len(bmadMounts))`

- [x] Task 5: Unit tests in `internal/mount/bmad_repos_test.go` (AC: #1, #3, #4, #5)
  - [x] 5.1 Test: valid repo paths → correct mount flags with `/workspace/repos/<basename>` targets
  - [x] 5.2 Test: empty BmadRepos → nil mounts, empty instruction content, no error
  - [x] 5.3 Test: nonexistent repo path → ConfigError returned with correct message
  - [x] 5.4 Test: repo path is a file (not directory) → ConfigError returned
  - [x] 5.5 Test: basename collision → ConfigError with both paths in message
  - [x] 5.6 Test: single repo → works correctly (no collision possible)
  - [x] 5.7 Test: instruction content rendered and non-empty when repos are provided
  - [x] 5.8 Test: instruction content contains each repo basename

- [x] Task 6: Unit tests for cmd/run.go bmad_repos integration (AC: #1, #2)
  - [x] 6.1 Verify AssembleBmadRepos is called when BmadRepos is configured
  - [x] 6.2 Verify no additional mounts when BmadRepos is empty
  - [x] 6.3 Follow existing test pattern from `run_test.go` (replicate RunE logic inline)

## Dev Notes

### What Already Exists (DO NOT recreate)

- **Config struct field**: `BmadRepos []string` already exists in `internal/config/config.go:35` with `yaml:"bmad_repos"` tag
- **YAML parsing**: Already handled by the yaml tag on the struct — no parse.go changes needed
- **Path resolution**: `parse.go:158-161` already resolves each `BmadRepos` path relative to config dir with tilde expansion
- **Embed directive**: `embed/embed.go:5` already includes `agent-instructions.md.tmpl` in the embedded filesystem
- **Template file**: `embed/agent-instructions.md.tmpl` exists with static content (28 lines of sandbox instructions)
- **Starter config**: `embed/config.yaml:63-68` already has commented-out `bmad_repos` example
- **Mount package**: `internal/mount/` already exists with `mount.go` and `isolate_deps.go` — add `bmad_repos.go` alongside them

### What Must Be Implemented

1. **`internal/mount/bmad_repos.go` — NEW FILE**
   - Function: `AssembleBmadRepos(cfg *config.Config) ([]string, string, error)`
   - Import `asboxEmbed "github.com/mcastellin/asbox/embed"` for template access — this is the established pattern (see `internal/template/render.go:10`)
   - Import `text/template` for rendering agent instructions
   - Early return when `len(cfg.BmadRepos) == 0` (same pattern as `isolate_deps.go:26`)
   - Validation order: (a) path exists, (b) path is directory, (c) no basename collisions
   - Collision detection: build `map[string]string` keyed by `filepath.Base(path)`, error on second occurrence
   - Mount flag format: `"/host/path:/workspace/repos/basename"` — same `source:target` format as `mount.go:34`
   - Template rendering: load `agent-instructions.md.tmpl` from `asboxEmbed.Assets`, parse and execute with repo data
   - Return rendered template content as the second return value (instruction string)

2. **`embed/agent-instructions.md.tmpl` — MODIFY**
   - Convert from static markdown to Go `text/template`
   - Keep all existing content intact as the base
   - Add conditional bmad_repos section using `{{if .BmadRepos}}` / `{{end}}`
   - List repos with `{{range .BmadRepos}}` showing name and container path
   - Include multi-repo git workflow instructions
   - Template data type should include fields for BmadRepos list
   - **IMPORTANT**: When bmad_repos is NOT configured, the build-time file (raw COPY in Dockerfile) is used. Template syntax will be visible but is acceptable — the runtime-rendered version replaces it when bmad_repos IS active [Source: architecture.md — Gap 1: FR44 vs FR53 resolution]

3. **`cmd/run.go` — MODIFY**
   - Insert `AssembleBmadRepos()` call between `mount.AssembleMounts()` (line 25) and `if cfg.AutoIsolateDeps` (line 41)
   - Pattern: follows the same block structure as auto_isolate_deps (lines 41-58)
   - Append repo mount flags to `mountFlags`
   - If instruction content is non-empty:
     - Write to temp file (`os.CreateTemp`)
     - Determine mount target based on `cfg.Agent`: `"/home/sandbox/CLAUDE.md"` or `"/home/sandbox/GEMINI.md"`
     - Append instruction file mount to `mountFlags`
   - Log output similar to auto_isolate_deps: `"bmad_repos: mounting %d repositories\n"`

4. **`internal/mount/bmad_repos_test.go` — NEW FILE**
   - Follow table-driven test pattern from `mount_test.go` and `isolate_deps_test.go`
   - Use `t.TempDir()` for valid source paths
   - Use `errors.As(&ce, &config.ConfigError)` for error type assertions
   - Create test dirs with unique basenames to avoid collision in valid-path tests

### Anti-Patterns (DO NOT do these)

- Do NOT modify `internal/config/config.go` or `internal/config/parse.go` — config layer is COMPLETE for this story
- Do NOT create a new package or file outside `internal/mount/` for the assembly logic — architecture specifies `internal/mount/bmad_repos.go`
- Do NOT scatter `//go:embed` directives — use existing `asboxEmbed.Assets` from `embed/embed.go`
- Do NOT add bmad_repos path validation in `parse.go` — validation (path exists, is directory, collision) belongs in `mount/bmad_repos.go` at runtime, NOT at config parse time. This follows the same pattern as `mount.go` (validates path existence) and `isolate_deps.go` (validates at scan time)
- Do NOT hardcode `/workspace/repos/` paths in multiple places — use a constant
- Do NOT add bmad_repos to the content hash — it's a runtime mount configuration, not a build-time input. Same reason `host_agent_config` is not in the hash.
- Do NOT create a separate function to render the template in `cmd/run.go` — keep template rendering inside `AssembleBmadRepos()` in the mount package, following the same pattern as `template.Render()` accessing `asboxEmbed.Assets`
- Do NOT modify the Dockerfile.tmpl — the build-time COPY of agent-instructions.md.tmpl is unchanged
- Do NOT use `os.Exit(1)` in `internal/mount/` — return typed `&config.ConfigError` errors and let `cmd/` handle exit codes

### Architecture Compliance

**Error handling chain:**
- `bmad_repos.go` returns `&config.ConfigError{Msg: "..."}` for all validation errors
- `cmd/run.go` returns the error to Cobra
- `cmd/root.go` maps `ConfigError` → exit code 1
- Error message format: what failed + why + fix action (existing pattern)

**File location:** `internal/mount/bmad_repos.go` — per architecture package responsibilities: "Mount flag assembly, auto_isolate_deps scanning, bmad_repos mapping with collision detection" [Source: architecture.md — Package Responsibilities]

**Agent instruction file dual-use (FR44 vs FR53):**
- FR44: Static instruction file baked into image at build time (COPY in Dockerfile.tmpl)
- FR53: Dynamic instruction file rendered at runtime when bmad_repos is active
- Resolution: Runtime file is mounted OVER the build-time file at the same container path
- When bmad_repos is NOT configured, build-time file is used as-is
[Source: architecture.md:651-654 — Gap 1 Resolution]

**Container paths:**
- Repos mount to: `/workspace/repos/<basename>`
- Instruction file mounts to: `/home/sandbox/CLAUDE.md` (claude-code) or `/home/sandbox/GEMINI.md` (gemini-cli)
- These paths come from Dockerfile.tmpl lines 92-98 (build-time instruction file placement)

### Library and Framework Requirements

- Go `text/template` for agent instruction rendering — same as `internal/template/render.go`
- Go `path/filepath` for `Base()` and path operations
- Go `os` for `Stat()` and `CreateTemp()` (temp file in cmd/run.go only)
- `asboxEmbed "github.com/mcastellin/asbox/embed"` for template file access
- `github.com/mcastellin/asbox/internal/config` for Config struct and ConfigError type
- No new external dependencies

### Testing Standards

- Table-driven tests with descriptive scenario names: `TestAssembleBmadRepos_validPaths`, `TestAssembleBmadRepos_basenameCollision`
- `t.TempDir()` for creating real temporary directories (same pattern as `mount_test.go`)
- `errors.As()` for typed error assertions on `config.ConfigError`
- Verify mount flag format: `"/tmp/xxx:/workspace/repos/basename"`
- Verify instruction content: non-empty string containing repo basenames
- Verify collision error message contains both conflicting paths
- `go test ./...` must pass with zero failures

### Project Structure Notes

- New files: `internal/mount/bmad_repos.go`, `internal/mount/bmad_repos_test.go`
- Modified files: `embed/agent-instructions.md.tmpl`, `cmd/run.go`
- No container-side changes (embed/entrypoint.sh, embed/Dockerfile.tmpl UNTOUCHED)
- Aligns with architecture project structure: `internal/mount/bmad_repos.go` listed at architecture.md:469
- `cmd/run.go` orchestration at architecture.md:504: "mount.AssembleBmadRepos() (if configured)"

### References

- [Source: architecture.md:271-277 — BMAD Multi-Repo Workflow decision and implementation details]
- [Source: architecture.md:403 — mount package responsibilities include bmad_repos]
- [Source: architecture.md:469 — bmad_repos.go file in project structure]
- [Source: architecture.md:504 — cmd/run.go orchestration flow including AssembleBmadRepos]
- [Source: architecture.md:519 — BmadRepos []string in Config struct]
- [Source: architecture.md:629 — FR51-FR53 mapped to bmad_repos.go + agent-instructions.md.tmpl]
- [Source: architecture.md:651-654 — Gap 1: FR44 vs FR53 agent instruction dual-use resolution]
- [Source: epics.md:818-856 — Epic 8, Story 8.1 requirements and implementation notes]
- [Source: prd.md — FR51: bmad_repos config option]
- [Source: prd.md — FR52: auto-mount repos to /workspace/repos/<name>]
- [Source: prd.md — FR53: generated agent instruction file for repos]
- [Source: internal/mount/mount.go — AssembleMounts() pattern for path validation and mount flag assembly]
- [Source: internal/mount/isolate_deps.go — ScanDeps() pattern for early return and filepath operations]
- [Source: internal/template/render.go — Pattern for importing asboxEmbed and rendering Go templates]
- [Source: embed/embed.go — Centralized embed directives, Assets FS]
- [Source: cmd/run.go — Current orchestration flow, insertion point for AssembleBmadRepos]
- [Source: embed/Dockerfile.tmpl:92-98 — Build-time instruction file paths for CLAUDE.md/GEMINI.md]

### Previous Story Intelligence (Story 7.1)

- **Pattern**: New functionality added by extending existing package files or adding new files in the correct package
- **Testing**: Table-driven tests with `t.TempDir()` for filesystem operations, `errors.As()` for error type checks
- **Integration point**: `cmd/run.go` is the main assembly point — insert new feature block in the logical sequence: mounts → bmad_repos → auto_isolate_deps → agent command → build → run
- **Config pattern**: Slice fields use `len() == 0` check for early return (different from pointer `!= nil` check used for HostAgentConfig)
- **Error pattern**: `&config.ConfigError{Msg: fmt.Sprintf(...)}` with descriptive messages including config file reference
- **Review pattern**: Story 7-1 had review findings for agent-gating (`cfg.Agent == "claude-code"`) on CLAUDE_CONFIG_DIR — the instruction file mount target also depends on agent type

### Git Intelligence

- Commit format: `feat: implement story 8-1 bmad multi-repo mounts and agent instructions`
- Recent commits: single commit per story with all changes
- Go 1.25.0, no new external dependencies needed
- All tests must pass: `go test ./...`
- Files touched in recent stories match the patterns specified here (mount package, cmd/run.go, embed assets)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

No issues encountered. All tests passed on first run.

### Completion Notes List

- Implemented `AssembleBmadRepos()` in `internal/mount/bmad_repos.go` with path validation, directory check, basename collision detection, mount flag generation, and template rendering
- Defined `BmadRepoInfo` and `InstructionData` structs for template data
- Converted `embed/agent-instructions.md.tmpl` from static markdown to Go `text/template` with conditional `{{if .BmadRepos}}` section including multi-repo workspace instructions and git workflow guidance
- Updated `cmd/run.go` to call `AssembleBmadRepos()` between mount assembly and auto_isolate_deps, write rendered instructions to temp file, and mount as CLAUDE.md/GEMINI.md based on agent type
- Added 8 unit tests in `bmad_repos_test.go` covering all ACs: valid paths, empty repos, nonexistent path, file-not-dir, basename collision, single repo, instruction content rendering, multi-repo basenames
- Added 3 integration tests in `run_test.go`: nonexistent bmad_repos path returns ConfigError, configured repos produce mounts and instruction content, empty repos produce no additional mounts
- Full test suite passes: `go test ./...` — zero failures, no regressions

### File List

- `internal/mount/bmad_repos.go` (NEW) — AssembleBmadRepos function with BmadRepoInfo/InstructionData structs
- `internal/mount/bmad_repos_test.go` (NEW) — 8 unit tests for AssembleBmadRepos
- `embed/agent-instructions.md.tmpl` (MODIFIED) — converted to Go template with conditional bmad_repos section
- `cmd/run.go` (MODIFIED) — added AssembleBmadRepos call and instruction file mount logic
- `cmd/run_test.go` (MODIFIED) — added 3 bmad_repos integration tests

### Review Findings

- [x] [Review][Patch] Temp file never cleaned up — `cmd/run.go:50` creates temp file via `os.CreateTemp` but never calls `os.Remove()`. Fixed: added `defer os.Remove(tmpFile.Name())`. [cmd/run.go:50]
- [x] [Review][Patch] Agent switch has no default case — `cmd/run.go:60-66` silently drops instruction file for unrecognized agent values. Fixed: added default case returning error; removed dead `if instructionTarget != ""` guard. [cmd/run.go:60-66]
- [x] [Review][Patch] filepath.Base("/") produces malformed mount path — `bmad_repos.go:57`. Fixed: added guard rejecting degenerate basenames ("/" or "."). [internal/mount/bmad_repos.go:57]
- [x] [Review][Defer] Mount flags lack :ro read-only qualifier [cmd/run.go:68] — deferred, pre-existing pattern (no mounts in codebase use :ro)
- [x] [Review][Defer] Content hash implicitly includes bmad_repos config — deferred, pre-existing hash granularity issue affecting all runtime-only config fields
- [x] [Review][Defer] Cmd integration tests don't exercise full RunE success path [cmd/run_test.go:177] — deferred, pre-existing test pattern (all success-path tests replicate RunE logic inline)

### Change Log

- Implemented story 8-1: BMAD multi-repo mounts and agent instructions (Date: 2026-04-10)
