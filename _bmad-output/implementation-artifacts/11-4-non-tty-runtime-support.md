# Story 11.4: Non-TTY Runtime Support

Status: done

## Story

As a developer,
I want asbox to detect whether it's running in a terminal or in a CI/CD pipeline,
so that sandbox sessions work correctly in both interactive and non-interactive contexts.

## Acceptance Criteria

1. **Given** a developer runs `asbox run` in an interactive terminal
   **When** the container is started
   **Then** Docker is invoked with `-it` flags (interactive + TTY allocated) and the agent session is fully interactive

2. **Given** a developer runs `asbox run` in a non-TTY context (e.g., CI/CD pipeline, piped input)
   **When** the container is started
   **Then** Docker is invoked with `-i` only (interactive, no TTY) -- no "the input device is not a TTY" error

3. **Given** a developer pipes input to `asbox run` (e.g., `echo "test" | asbox run`)
   **When** the container is started
   **Then** the piped input is forwarded to the container's stdin without TTY allocation errors

4. **Given** the TTY detection logic
   **When** inspected
   **Then** it uses `term.IsTerminal(int(os.Stdin.Fd()))` from `golang.org/x/term` to detect terminal presence -- not environment variable heuristics

## Tasks / Subtasks

- [x] Task 1: Add `golang.org/x/term` dependency (AC: #4)
  - [x] 1.1 Run `go get golang.org/x/term` to add the dependency to `go.mod` and `go.sum`
- [x] Task 2: Add `AllocTTY` field to `RunOptions` struct (AC: #1, #2)
  - [x] 2.1 Add `AllocTTY bool` field to `RunOptions` in `internal/docker/run.go:11`
  - [x] 2.2 Update `runCmdArgs()` to conditionally emit `-it` (when `AllocTTY` is true) or `-i` (when false)
- [x] Task 3: Add TTY detection in `cmd/run.go` (AC: #1, #2, #3, #4)
  - [x] 3.1 Import `golang.org/x/term` and `os` (os already imported)
  - [x] 3.2 Detect terminal: `isTTY := term.IsTerminal(int(os.Stdin.Fd()))`
  - [x] 3.3 Pass `AllocTTY: isTTY` when constructing `docker.RunOptions` at line 130
- [x] Task 4: Update existing tests in `internal/docker/run_test.go` (AC: #1, #2)
  - [x] 4.1 Update `TestRunCmdArgs_basicFlags` to pass `AllocTTY: true` and continue asserting `-it`
  - [x] 4.2 Update `TestRunCmdArgs_fullOptions` to pass `AllocTTY: true` and continue asserting `-it`
  - [x] 4.3 Other tests (`noContainerName`, `envVars`, `mounts`, `emptyEnvValue`) don't assert on `-it` position -- add `AllocTTY: true` to keep existing behavior
- [x] Task 5: Add new tests for non-TTY mode in `internal/docker/run_test.go` (AC: #2, #3)
  - [x] 5.1 Add `TestRunCmdArgs_noTTY` asserting `-i` is present and `-it` is NOT when `AllocTTY: false`
  - [x] 5.2 Add `TestRunCmdArgs_noTTY_fullOptions` with mounts/env/name and `AllocTTY: false`

## Dev Notes

### Implementation Location

All Go changes are in three files:
- `internal/docker/run.go` -- struct change + conditional flag logic
- `internal/docker/run_test.go` -- update existing tests + add new tests
- `cmd/run.go` -- TTY detection before `RunContainer` call

Plus `go.mod`/`go.sum` changes from `go get golang.org/x/term`.

### RunOptions Struct Change

Add `AllocTTY` to the existing struct at `internal/docker/run.go:11`:

```go
type RunOptions struct {
    ImageRef      string
    ContainerName string
    EnvVars       map[string]string
    Mounts        []string
    AllocTTY      bool      // true = -it, false = -i only
    Stdin         io.Reader
    Stdout        io.Writer
    Stderr        io.Writer
}
```

### Conditional Flag Logic

Replace the hardcoded `-it` at `internal/docker/run.go:23`:

```go
func runCmdArgs(opts RunOptions) []string {
    var args []string
    if opts.AllocTTY {
        args = []string{"run", "-it", "--rm",
    } else {
        args = []string{"run", "-i", "--rm",
    }
    // ... rest unchanged
```

Keep `--rm` always present -- containers should be cleaned up in both modes.

### TTY Detection Call Site

In `cmd/run.go`, before constructing `RunOptions` (around line 130):

```go
isTTY := term.IsTerminal(int(os.Stdin.Fd()))
```

Then pass it:

```go
opts := docker.RunOptions{
    ImageRef:      imageRef,
    ContainerName: containerName,
    EnvVars:       envVars,
    Mounts:        mountFlags,
    AllocTTY:      isTTY,
    Stdin:         os.Stdin,
    Stdout:        cmd.OutOrStdout(),
    Stderr:        cmd.ErrOrStderr(),
}
```

Import `golang.org/x/term` and cast `os.Stdin.Fd()` to `int` since `term.IsTerminal` expects `int` and `Fd()` returns `uintptr`.

### Why `os.Stdin.Fd()` Not `os.Stdout.Fd()`

The TTY check must be against stdin because that's what determines whether Docker can allocate a pseudo-terminal for interactive input. When stdin is piped (`echo "test" | asbox run`), stdin is not a terminal even if stdout still is. Checking stdin matches Docker's own behavior -- `docker run -it` fails with "the input device is not a TTY" when stdin is not a terminal.

### Dependency Choice

Use `golang.org/x/term` (not `golang.org/x/sys/unix` directly):
- `golang.org/x/term` provides `IsTerminal(fd int) bool` -- a clean, cross-platform API
- Works on macOS (darwin) and Linux
- `golang.org/x/sys` is already an indirect dependency at `v0.42.0` -- `x/term` wraps it
- Do NOT use `isatty` heuristics, `$TERM` env var checks, or `CI` env var checks

### What `-i` vs `-it` Means

- `-i` (`--interactive`): keeps stdin open even if not attached. The container can still receive input.
- `-t` (`--tty`): allocates a pseudo-TTY. Required for interactive terminal sessions but fails when stdin is a pipe/file.
- In CI/CD: `-i` alone lets the container run, receive piped commands, and exit. `-it` would error with "the input device is not a TTY".

### Test Updates Required

**Existing tests that hardcode `-it` expectations** (must be updated):

1. `TestRunCmdArgs_basicFlags` (`internal/docker/run_test.go:9`) -- line 20 asserts `args[1] != "-it"`. Update to set `AllocTTY: true` and keep assertion.

2. `TestRunCmdArgs_fullOptions` (`internal/docker/run_test.go:147`) -- line 163 checks list includes `"-it"`. Update to set `AllocTTY: true` and keep assertion.

**Existing tests that don't assert on `-it` position** (safe, but should set `AllocTTY: true` for consistency):

3. `TestRunCmdArgs_noContainerName` (line 55)
4. `TestRunCmdArgs_envVars` (line 67)
5. `TestRunCmdArgs_mounts` (line 100)
6. `TestRunCmdArgs_emptyEnvValue` (line 125)

**New tests to add:**

- `TestRunCmdArgs_noTTY`: `AllocTTY: false` -- assert args contain `"-i"` and `"--rm"` at positions 1-2, and do NOT contain `"-it"` anywhere
- `TestRunCmdArgs_noTTY_fullOptions`: full options with `AllocTTY: false` -- assert `-i` present, `-it` absent, all other flags correct

### What NOT To Change

- `embed/entrypoint.sh` -- the entrypoint does not need to know about TTY mode; Docker handles pseudo-TTY allocation
- `embed/Dockerfile.tmpl` -- no build-time changes
- `cmd/root.go` / `cmd/root_test.go` -- no new error types or exit codes
- `internal/config/` -- no config changes; TTY detection is runtime-only, not configurable
- No `--tty` or `--no-tty` CLI flag -- detection is automatic. Users who need to force behavior can use standard Docker practices (e.g., `script -c "asbox run"` to force TTY)

### Error Pattern

No new error types needed. The only failure mode is `docker run` itself failing, which already returns `RunError`. If stdin is not a terminal and `-t` were passed, Docker would fail with "the input device is not a TTY" -- this story prevents that by detecting the condition beforehand.

### Integration Test Considerations

Existing integration tests in `integration/` use `testcontainers-go`, not the `docker.RunContainer()` path directly. They should not be affected by this change. No integration test changes needed.

Binary invocation tests (if any exist for `asbox run`) might need review -- they would be running in a non-TTY context (Go test subprocess), so they would exercise the `-i` path. This is correct behavior.

### Project Structure Notes

- All changes within existing package boundaries -- `internal/docker/` and `cmd/`
- No new packages, no new files, no new error types
- Single new dependency: `golang.org/x/term`
- `golang.org/x/sys` is already an indirect dependency (used by x/term internally)

### Previous Story Intelligence (11-3)

**Key learnings from story 11-3:**
- Story 11-3 was scoped tightly to `internal/config/` only -- clean separation
- Table-driven tests with explicit assertions on both valid and invalid paths
- Reused existing error types (`ConfigError`) rather than creating new ones
- Validation functions follow `validate<Thing>()` pattern
- Full project test suite must stay green: `go test ./...`

**Git patterns from recent commits:**
- `0dae1e1`: `feat: ENV key/value validation and Dockerfile injection hardening (story 11-3)`
- `4ba3efe`: `feat: SDK version, package name, and project name sanitization (story 11-2)`
- Convention: `feat:` prefix for new capabilities

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 11, Story 11-4]
- [Source: _bmad-output/planning-artifacts/architecture.md - Container Lifecycle, CLI Interface]
- [Source: _bmad-output/planning-artifacts/prd.md - NFR11 (non-TTY runtime), FR11 (TTY mode)]
- [Source: internal/docker/run.go - RunOptions struct (lines 11-19), runCmdArgs() with hardcoded -it (line 23), RunContainer() (lines 48-68)]
- [Source: internal/docker/run_test.go - TestRunCmdArgs_basicFlags (line 20: -it assertion), TestRunCmdArgs_fullOptions (line 163: -it check)]
- [Source: cmd/run.go - RunContainer call with RunOptions (lines 130-140), os.Stdin passed at line 135]
- [Source: go.mod - golang.org/x/sys v0.42.0 indirect (line 67), no golang.org/x/term present]

## Dev Agent Record

### Agent Model Used

Codex GPT-5

### Implementation Plan

- Add `golang.org/x/term`, then replace the hardcoded Docker `-it` behavior with an explicit `AllocTTY` flag in `docker.RunOptions`.
- Detect TTY availability from `os.Stdin` in `cmd/run.go` so interactive sessions keep full terminal behavior while piped/non-interactive runs use `-i` only.
- Update the Docker argument tests to cover both interactive and non-TTY paths, then run formatting and full Go validation.

### Debug Log References

- Initialization: loaded BMAD config, sprint status, and story 11-4; confirmed this was the first ready-for-dev story.
- Task 1: ran `go get golang.org/x/term`, which added `golang.org/x/term` and updated `golang.org/x/sys` in module metadata.
- Tasks 2-3: added `AllocTTY` to `docker.RunOptions`, changed `runCmdArgs()` to emit `-i` by default with optional `-it`, and wired `term.IsTerminal(int(os.Stdin.Fd()))` into `cmd/run.go`.
- Tasks 4-5: updated the existing Docker argument tests to pass `AllocTTY: true` and added explicit non-TTY tests for minimal and full-option cases.
- Validation: ran `gofmt -w cmd/run.go internal/docker/run.go internal/docker/run_test.go`, `go vet ./...`, and `go test ./...` successfully.

### Review Findings

- [x] [Review][Patch] Fragile index-based mutation for TTY flag [internal/docker/run.go:24-33] — replaced index mutation with conditional variable
- [x] [Review][Patch] Test `-it` absence check uses string containment [internal/docker/run_test.go:237] — replaced with per-element iteration
- [x] [Review][Patch] Stale doc comment on RunContainer [internal/docker/run.go:52] — updated to reflect conditional TTY allocation

### Completion Notes List

- Added stdin-based TTY detection using `term.IsTerminal(int(os.Stdin.Fd()))`, so `asbox run` now selects `-it` only when stdin is an interactive terminal.
- Extended `docker.RunOptions` with `AllocTTY` and made Docker interactive flags conditional, which prevents non-TTY contexts from triggering Docker's TTY allocation error.
- Updated the Docker command argument tests to preserve interactive coverage and added two new non-TTY tests that assert `-i` is used without `-it`.
- Verified the full repository validation gates with `go vet ./...` and `go test ./...`.

### File List

- cmd/run.go (modified - detect stdin TTY state and pass `AllocTTY` into Docker run options)
- go.mod (modified - added `golang.org/x/term` requirement and updated `golang.org/x/sys`)
- go.sum (modified - added checksums for updated `golang.org/x/term` and `golang.org/x/sys`)
- internal/docker/run.go (modified - added `AllocTTY` and conditional Docker `-i`/`-it` flag handling)
- internal/docker/run_test.go (modified - updated interactive tests and added non-TTY command argument coverage)

### Change Log

- 2026-04-16: Implemented story 11-4 non-TTY runtime support with stdin TTY detection, conditional Docker interactive flags, and expanded Docker run argument tests.
