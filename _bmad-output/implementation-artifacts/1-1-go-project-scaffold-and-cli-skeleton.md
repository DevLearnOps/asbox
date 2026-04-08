# Story 1.1: Go Project Scaffold and CLI Skeleton

Status: done

## Story

As a developer,
I want to run `asbox` and see usage help, and have the CLI validate that Docker is installed,
so that I know the tool is working and will fail clearly if prerequisites are missing.

## Acceptance Criteria

1. **Given** a developer has the `asbox` binary on their PATH
   **When** they run `asbox --help`
   **Then** usage information is displayed showing available commands (init, build, run) and flags (-f, --help, --version)

2. **Given** Docker is not installed on the host
   **When** the developer runs any asbox command
   **Then** the CLI exits with code 3 and prints `"error: docker not found. Install Docker Engine 20.10+ or Docker Desktop"` to stderr

3. **Given** the developer runs `asbox build` or `asbox run` before these commands are implemented
   **When** the stub command executes
   **Then** the CLI exits with code 1 and prints `"error: not implemented"` to stderr

4. **Given** any error occurs during execution
   **When** the CLI exits
   **Then** it returns the appropriate exit code: 1 (config/general), 2 (usage), 3 (dependency), 4 (secret) -- mapped from typed error types in `cmd/root.go`

5. **Given** the asbox binary is built with `CGO_ENABLED=0`
   **When** inspecting the binary
   **Then** it is statically linked with no external runtime dependencies

## Tasks / Subtasks

- [x] Task 1: Initialize Go module and project structure (AC: #1, #5)
  - [x] Create `go.mod` with module name `github.com/mcastellin/asbox`
  - [x] Run `go get github.com/spf13/cobra` to add Cobra dependency
  - [x] Create directory structure: `cmd/`, `internal/config/`, `internal/template/`, `internal/docker/`, `internal/hash/`, `internal/mount/`, `embed/`
  - [x] Create `main.go` with single line `cmd.Execute()`
- [x] Task 2: Implement typed error types (AC: #4)
  - [x] Create `internal/config/errors.go` with `ConfigError{Field, Msg string}`
  - [x] Create `internal/docker/errors.go` with `DependencyError{Msg string}` and `TemplateError{Msg string}`
  - [x] Each error type must implement `Error() string` with format: what failed + why + fix action
- [x] Task 3: Implement root command with error-to-exit-code mapping (AC: #1, #4)
  - [x] Create `cmd/root.go` with root Cobra command
  - [x] Add `-f` persistent flag (string, default `.asbox/config.yaml`)
  - [x] Add `--version` flag
  - [x] Implement `Execute()` function that maps error types to exit codes via type switch: `ConfigError`/`TemplateError` -> 1, Cobra usage errors -> 2, `DependencyError` -> 3, `SecretError` -> 4
  - [x] All errors printed to stderr with `fmt.Fprintf(os.Stderr, "error: %s\n", err)`
- [x] Task 4: Implement Docker dependency check (AC: #2)
  - [x] In root command `PersistentPreRunE`, call `exec.LookPath("docker")`
  - [x] If docker not found, return `DependencyError{Msg: "docker not found. Install Docker Engine 20.10+ or Docker Desktop"}`
- [x] Task 5: Implement stub subcommands (AC: #3)
  - [x] Create `cmd/init.go` with `init` subcommand returning `ConfigError{Msg: "not implemented"}`
  - [x] Create `cmd/build.go` with `build` subcommand returning `ConfigError{Msg: "not implemented"}`
  - [x] Create `cmd/run.go` with `run` subcommand returning `ConfigError{Msg: "not implemented"}`
- [x] Task 6: Create embed package with placeholder assets (AC: #1)
  - [x] Create `embed/embed.go` with `//go:embed` directives for all assets
  - [x] Create placeholder files: `Dockerfile.tmpl`, `entrypoint.sh`, `git-wrapper.sh`, `healthcheck-poller.sh`, `agent-instructions.md.tmpl`, `config.yaml`
  - [x] Export embedded FS as `var Assets embed.FS`
- [x] Task 7: Write tests (AC: #1, #2, #3, #4)
  - [x] Unit test: `--help` output contains "init", "build", "run", "-f"
  - [x] Unit test: Docker not found returns exit code 3 with correct error message
  - [x] Unit test: Stub commands return exit code 1 with "not implemented"
  - [x] Unit test: Error-to-exit-code mapping for all typed errors
  - [x] Verify build with `CGO_ENABLED=0 go build` produces static binary
- [x] Task 8: Verify the build (AC: #5)
  - [x] Run `CGO_ENABLED=0 go build -o asbox .`
  - [x] Run `go vet ./...`
  - [x] Run `go test ./...`

## Dev Notes

### Architecture Compliance

- **`main.go`**: Single line only — `cmd.Execute()`. No business logic, no imports beyond `cmd` package.
- **`cmd/` layer**: Thin — only Cobra commands, flag parsing, and exit code mapping. No business logic.
- **Error types defined in owning packages**: `ConfigError` in `internal/config/`, `DependencyError` in the package that detects it. No centralized errors package.
- **No `os.Exit()` in `internal/` packages**: Return typed errors, let `cmd/root.go` handle exit codes.
- **Dependency direction**: `cmd/` -> `internal/*` -> standard library. Internal packages do NOT import each other.

### Error-to-Exit-Code Mapping (cmd/root.go)

```go
// In root command Execute(), after running command:
switch err.(type) {
case *config.ConfigError:
    os.Exit(1)
case *template.TemplateError:
    os.Exit(1)
case *docker.DependencyError:
    os.Exit(3)
case *config.SecretError:
    os.Exit(4)
default:
    os.Exit(1) // general/unknown errors
}
// Cobra handles exit code 2 for usage errors automatically
```

### Output Conventions

- **Errors**: `fmt.Fprintf(os.Stderr, "error: %s\n", msg)` — lowercase, no trailing punctuation
- **No color codes, spinners, or progress bars**
- **Error messages include**: what failed + why + fix action
- **Success output**: plain text to stdout, no prefix

### Go Code Conventions

- **Formatting**: `gofmt` is law, `go vet` must pass
- **File naming**: `snake_case.go` — `root.go`, `embed.go`, `errors.go`
- **Test naming**: `TestFunctionName_scenario` — `TestExecute_dockerNotFound`, `TestExecute_helpOutput`
- **Table-driven tests** preferred for multiple scenarios
- **Variable naming**: `camelCase` — `configPath`, `imageName`

### Embed Package Structure

```go
// embed/embed.go
package embed

import "embed"

//go:embed Dockerfile.tmpl entrypoint.sh git-wrapper.sh healthcheck-poller.sh agent-instructions.md.tmpl config.yaml
var Assets embed.FS
```

All `//go:embed` directives centralized in this single file. Placeholder content is fine for this story — real content comes in Stories 1.3-1.5 and 1.8.

### DependencyError Placement

`DependencyError` is used for Docker-not-found detection. Since it relates to external dependencies (not config or template), place it in `internal/docker/errors.go`. The `cmd/root.go` imports `internal/docker` for the type switch.

### Key Anti-Patterns to Avoid

- Do NOT create `internal/utils/` or `internal/common/` packages
- Do NOT scatter `//go:embed` across multiple files
- Do NOT add color codes or spinners to output
- Do NOT use `os.Exit()` inside `internal/` packages
- Do NOT use `interface{}` or `any` as function parameters
- Do NOT print errors to stdout — always stderr

### Project Structure Notes

This story creates the Go project from scratch. The repository currently contains the old bash-based `sandbox.sh` implementation and supporting scripts (`scripts/`, `templates/`, `Dockerfile.template`, `.sandbox-dockerfile`). The new Go project structure lives alongside these files initially. The old files are NOT modified or removed in this story.

Expected new files after this story:
```
asbox/                       # (or project root if preferred)
├── main.go
├── go.mod
├── go.sum
├── cmd/
│   ├── root.go
│   ├── init.go
│   ├── build.go
│   └── run.go
├── internal/
│   ├── config/
│   │   └── errors.go        # ConfigError, SecretError
│   ├── template/
│   ├── docker/
│   │   └── errors.go        # DependencyError
│   ├── hash/
│   └── mount/
└── embed/
    ├── embed.go
    ├── Dockerfile.tmpl       # placeholder
    ├── entrypoint.sh         # placeholder
    ├── git-wrapper.sh        # placeholder
    ├── healthcheck-poller.sh # placeholder
    ├── agent-instructions.md.tmpl  # placeholder
    └── config.yaml           # placeholder
```

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Story 1.1: Go Project Scaffold and CLI Skeleton]
- [Source: _bmad-output/planning-artifacts/architecture.md — Error Handling Strategy section]
- [Source: _bmad-output/planning-artifacts/architecture.md — Exit Codes table]
- [Source: _bmad-output/planning-artifacts/architecture.md — Go Code Conventions section]
- [Source: _bmad-output/planning-artifacts/architecture.md — Go Project Organization section]
- [Source: _bmad-output/planning-artifacts/architecture.md — Complete Project Directory Structure]
- [Source: _bmad-output/planning-artifacts/architecture.md — Anti-Patterns section]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None - clean implementation with no blocking issues.

### Completion Notes List

- Initialized Go module `github.com/mcastellin/asbox` with Cobra dependency
- Created typed error types: `ConfigError`, `SecretError` (in `internal/config/`), `DependencyError`, `TemplateError` (in `internal/docker/`)
- Implemented root command with `SilenceErrors`/`SilenceUsage` to control error output format
- Extracted `exitCode()` helper function for testability (avoids `os.Exit` in tests)
- Docker dependency check via `exec.LookPath("docker")` in `PersistentPreRunE`
- Stub subcommands (init, build, run) return `ConfigError{Msg: "not implemented"}`
- Embed package with 6 placeholder asset files and `//go:embed` directive
- 15 tests across 4 packages, all passing: error types, exit code mapping, help output, Docker check, stub commands, embedded assets
- Static binary verified: `CGO_ENABLED=0 go build` produces binary with no dynamic dependencies

### Change Log

- 2026-04-08: Story implementation complete — Go project scaffold, CLI skeleton, error types, tests, and embedded assets

### File List

- go.mod (new)
- go.sum (new)
- main.go (new)
- cmd/root.go (new)
- cmd/root_test.go (new)
- cmd/init.go (new)
- cmd/build.go (new)
- cmd/run.go (new)
- internal/config/errors.go (new)
- internal/config/errors_test.go (new)
- internal/docker/errors.go (new)
- internal/docker/errors_test.go (new)
- internal/template/errors.go (new)
- internal/template/errors_test.go (new)
- internal/hash/.gitkeep (new)
- internal/mount/.gitkeep (new)
- embed/embed.go (new)
- embed/embed_test.go (new)
- embed/Dockerfile.tmpl (new)
- embed/entrypoint.sh (new)
- embed/git-wrapper.sh (new)
- embed/healthcheck-poller.sh (new)
- embed/agent-instructions.md.tmpl (new)
- embed/config.yaml (new)

### Review Findings

- [x] [Review][Decision] Placeholder module path `github.com/user/asbox` — Fixed: updated to `github.com/mcastellin/asbox`
- [x] [Review][Decision] Exit code 2 (usage error) never produced — Fixed: added `usageError` type via `SetFlagErrorFunc`, `exitCode()` returns 2
- [x] [Review][Decision] TemplateError defined in `internal/docker/` not `internal/template/` — Fixed: moved to `internal/template/errors.go`
- [x] [Review][Patch] `go.mod` marks cobra as `// indirect` when it's a direct dependency — Fixed via `go mod tidy`
- [x] [Review][Patch] Redundant `defer os.Setenv` after `t.Setenv` — Fixed: removed redundant defer
- [x] [Review][Patch] `TestStubCommands` fails in environments without Docker — Fixed: tests use helper that isolates rootCmd state
- [x] [Review][Patch] `rootCmd` mutated in tests without cleanup — Fixed: added `newRootCmd()` helper with output capture
- [x] [Review][Patch] No test for `--version` flag in help output — Fixed: added `--version` to help assertions
- [x] [Review][Patch] `exitCode()` uses type-switch for typed errors — wrapped errors silently map to exit 1. Fixed: replaced type-switch with `errors.As` for all error types [cmd/root.go:50]
- [x] [Review][Defer] Docker check in `PersistentPreRunE` runs on `--help` and non-docker commands like `init` [cmd/root.go:35] — deferred, scope for future story when `init` becomes real
- [x] [Review][Defer] Test helper `newRootCmd()` mutates package-level `rootCmd` instead of creating a fresh command tree [cmd/root_test.go:17] — deferred, works today but fragile for parallel tests
