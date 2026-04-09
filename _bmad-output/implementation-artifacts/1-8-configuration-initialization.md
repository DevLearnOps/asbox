# Story 1.8: Configuration Initialization

Status: done

## Story

As a developer,
I want to run `asbox init` to generate a starter configuration file,
So that I have a working starting point for configuring my sandbox.

## Acceptance Criteria

1. **Given** no `.asbox/config.yaml` exists in the current directory
   **When** the developer runs `asbox init`
   **Then** `.asbox/config.yaml` is created from the embedded starter template with sensible defaults and inline comments

2. **Given** `.asbox/config.yaml` already exists
   **When** the developer runs `asbox init`
   **Then** the CLI exits with code 1 and prints `"error: config already exists at .asbox/config.yaml"` to stderr

3. **Given** the developer specifies `-f custom/path/config.yaml`
   **When** they run `asbox init -f custom/path/config.yaml`
   **Then** the config is generated at the specified path, creating parent directories if needed

4. **Given** the generated config
   **When** inspecting the file
   **Then** `auto_isolate_deps` appears as a commented-out option with explanation, and `bmad_repos` appears as a commented-out example

## Tasks / Subtasks

- [x] Task 1: Update embedded starter config template (AC: #1, #4)
  - [x] Replace placeholder `embed/config.yaml` with full starter template content
  - [x] Ensure `auto_isolate_deps` is commented-out with explanation
  - [x] Add `bmad_repos` as commented-out example with explanation
  - [x] Include all config sections with sensible defaults and inline comments
- [x] Task 2: Implement `cmd/init.go` (AC: #1, #2, #3)
  - [x] Replace stub with actual logic
  - [x] Read starter config from `embed.Assets` (`config.yaml`)
  - [x] Resolve target path from `-f` flag (already available via `configFile` in `cmd/root.go`)
  - [x] Check if file already exists — return `ConfigError` if so
  - [x] Create parent directories with `os.MkdirAll`
  - [x] Write embedded content to target path
  - [x] Print success message to stdout
  - [x] Skip Docker PersistentPreRunE check (init doesn't need Docker)
- [x] Task 3: Write unit tests for init command (AC: #1, #2, #3)
  - [x] Test: creates config at default `.asbox/config.yaml` in temp dir
  - [x] Test: errors when config already exists
  - [x] Test: creates config at custom `-f` path
  - [x] Test: creates parent directories when they don't exist
  - [x] Test: generated file matches embedded template content
- [x] Task 4: Remove `templates/config.yaml` (cleanup)
  - [x] Move content to `embed/config.yaml` (the embedded version is the single source of truth)
  - [x] Delete `templates/config.yaml` and `templates/` directory if empty

### Review Findings

- [x] [Review][Patch] `os.Stat` error not properly discriminated — code checks `err == nil` but doesn't handle non-`ErrNotExist` errors (e.g. permission denied); should use `errors.Is(err, os.ErrNotExist)` [cmd/init.go:30] — fixed
- [x] [Review][Patch] AC #3 test bypasses `-f` flag parsing — `TestInit_createsConfigAtCustomPath` mutates `configFile` directly instead of passing `-f` as a CLI argument [cmd/init_test.go:64-77] — fixed
- [x] [Review][Defer] `PersistentPreRunE` override scope too broad — using `PersistentPreRunE` instead of `PreRunE` means future subcommands of `init` bypass Docker check [cmd/init.go:13-15] — deferred, pre-existing pattern

## Dev Notes

### Critical: Docker Check Must Be Skipped for Init

The `PersistentPreRunE` on `rootCmd` checks for Docker via `exec.LookPath("docker")`. The `asbox init` command must NOT require Docker — a developer should be able to generate a config before Docker is installed. Override this by setting `PersistentPreRunE: nil` (or a no-op) on the `initCmd` itself, or use Cobra's `PersistentPreRunE` override on the child command. The child command's `PersistentPreRunE` takes precedence over the parent's. Example:

```go
var initCmd = &cobra.Command{
    Use:   "init",
    Short: "Initialize a new asbox configuration",
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        return nil // init doesn't need Docker
    },
    RunE: runInit,
}
```

### File to Read from Embedded Assets

The embedded filesystem is accessed via `embed.Assets` (exported from `embed/embed.go`). Read the starter config with:

```go
data, err := asboxEmbed.Assets.ReadFile("config.yaml")
```

Import the embed package as `asboxEmbed "github.com/mcastellin/asbox/embed"` to avoid collision with Go's `embed` package.

### Target Path Resolution

The `-f` flag value is already available as `configFile` (package-level var in `cmd/root.go`, default `.asbox/config.yaml`). The path is relative to the current working directory (unlike mount paths which resolve relative to config file). Use `filepath.Abs()` to resolve for error messages.

### Error Message Format

Follow the established pattern from `config.ConfigError`:
- Exists: `"config already exists at .asbox/config.yaml"` (matches AC #2 exactly)
- Write failure: `"failed to create config at <path>: <os error>"`

### Success Message Format

Follow the established pattern (plain text to stdout, no color):
```
created .asbox/config.yaml
```

### Embedded Config Template Content

The current `embed/config.yaml` is a placeholder (`# Default asbox configuration / # Placeholder - real content in Story 1.8`). Replace with the full starter template. Use `templates/config.yaml` as the base, but ensure:

1. `auto_isolate_deps` is **commented out** (the current `templates/config.yaml` has it uncommented as `auto_isolate_deps: true` — must change to commented-out per AC #4)
2. Add a `bmad_repos` section as a **commented-out example** (missing from current template — required by AC #4)

### Project Structure Notes

- `cmd/init.go` — Replace stub (currently 19 lines, returns `ConfigError{"not implemented"}`)
- `embed/config.yaml` — Replace placeholder with full starter template
- `templates/config.yaml` — Remove after migrating content to `embed/config.yaml` (architecture specifies embedded assets as single source of truth, `templates/` dir is not in the architecture's project structure)
- No new packages needed — use existing `config.ConfigError`, `embed.Assets`, standard library `os`/`filepath`

### Existing Code Patterns to Follow

- **Error types**: Use `config.ConfigError{Msg: "..."}` — exit code 1 via `cmd/root.go` mapping
- **Output**: `fmt.Fprintln(cmd.OutOrStdout(), "created ...")` for success (testable via Cobra's output capture)
- **No business logic in cmd/**: Keep init logic simple enough that it stays in `cmd/init.go` — no need for a separate internal package for a file copy operation
- **Testing**: Use `t.TempDir()` for filesystem isolation, `errors.As()` for typed error checks, individual test functions per scenario (not table-driven — matches existing convention in this codebase)

### Previous Story Intelligence

**From Story 1-7:**
- `cmd/build_helper.go` extracted shared `ensureBuild()` — init is standalone, no shared helper needed
- `cmd/root_test.go` tests exit code mapping — add `init`-specific tests in a new `cmd/init_test.go` file
- Error type convention: each package defines errors in `errors.go` with dedicated struct. Init uses existing `config.ConfigError`
- Signal handling (130/143 suppression) is irrelevant to init

**From Story 1-2 (config parsing):**
- `configFile` var in `root.go` defaults to `.asbox/config.yaml` — init reuses this same flag
- `config.Parse()` validates that the file exists and agent field is set — init creates the file that Parse will later read
- The generated config must be parseable by `config.Parse()` without errors (validate this in tests)

### Git Intelligence

Recent commits follow pattern: `feat: implement story 1-X <description>`. All stories landed cleanly on `feat/go-rewrite` branch. No merge conflicts or CI issues observed.

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Epic 1, Story 1.8]
- [Source: _bmad-output/planning-artifacts/architecture.md — Configuration Handling, Init Command, CLI Patterns, Code Structure]
- [Source: _bmad-output/planning-artifacts/prd.md — FR9, FR9a-FR9e, FR50]
- [Source: _bmad-output/implementation-artifacts/1-7-sandbox-run-with-tty-and-lifecycle.md — Dev Notes, File List]
- [Source: cmd/init.go — Current stub implementation]
- [Source: cmd/root.go — configFile var, PersistentPreRunE Docker check, error mapping]
- [Source: embed/embed.go — Assets embed.FS declaration]
- [Source: embed/config.yaml — Current placeholder]
- [Source: templates/config.yaml — Current starter template content]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

No issues encountered during implementation.

### Completion Notes List

- Task 1: Replaced placeholder `embed/config.yaml` with full starter template based on `templates/config.yaml`. Changed `auto_isolate_deps` from uncommented to commented-out. Added `bmad_repos` as commented-out example with explanation.
- Task 2: Replaced init stub with full implementation: reads embedded config via `asboxEmbed.Assets.ReadFile`, checks for existing file (returns `ConfigError`), creates parent dirs via `os.MkdirAll`, writes file, prints success message. Added `PersistentPreRunE` no-op override so init doesn't require Docker.
- Task 3: Created `cmd/init_test.go` with 7 tests: default path creation, exists error, custom `-f` path, parent directory creation, template content match, Docker not required, success message format. Removed obsolete `TestStubCommands_returnNotImplemented` from `cmd/root_test.go`.
- Task 4: Deleted `templates/config.yaml` and `templates/` directory. No Go code referenced these files.

### Change Log

- 2026-04-09: Implemented story 1-8 — `asbox init` command with embedded config template, file existence check, parent dir creation, custom path support, and Docker check bypass. Removed `templates/` directory.

### File List

- embed/config.yaml (modified) — Full starter config template replacing placeholder
- cmd/init.go (modified) — Complete init command implementation
- cmd/init_test.go (new) — 7 unit tests for init command
- cmd/root_test.go (modified) — Removed obsolete stub test
- templates/config.yaml (deleted) — Migrated to embed/config.yaml
- templates/ (deleted) — Empty directory removed
