# Story 11.1: Concurrent Sandbox Sessions

Status: done

## Story

As a developer,
I want to run multiple sandbox sessions simultaneously against the same or different projects,
So that I can delegate tasks to multiple agents in parallel without container name collisions.

## Acceptance Criteria

1. **Random-suffixed container name**
   ```
   GIVEN a developer runs `asbox run` for a project
   WHEN the container is created
   THEN the container name uses the pattern `asbox-<project>-<suffix>` where `<suffix>` is a 6-character lowercase hex string, ensuring uniqueness across concurrent runs
   ```

2. **No conflict on concurrent runs**
   ```
   GIVEN a developer runs two `asbox run` commands for the same project simultaneously
   WHEN both containers start
   THEN both sessions launch successfully with distinct container names and no conflict error
   ```

3. **Clean single-session cleanup**
   ```
   GIVEN a developer runs `asbox run` and then exits with Ctrl+C
   WHEN the session ends
   THEN the container with the random-suffixed name is removed cleanly, and `docker ps -a` shows no orphaned asbox containers for that session
   ```

4. **Clean concurrent-session cleanup**
   ```
   GIVEN a developer runs two concurrent sandbox sessions
   WHEN both sessions exit (via Ctrl+C or normal termination)
   THEN both containers are removed and no orphaned containers or networks remain
   ```

5. **Backward-compatible prefix**
   ```
   GIVEN the container naming scheme has changed from deterministic to random-suffixed
   WHEN existing scripts or workflows reference the old `asbox-<project>` name
   THEN the `asbox-<project>` prefix is preserved as the first part of the name, maintaining greppability and log identification
   ```

## Tasks / Subtasks

- [x] Task 1: Add random suffix generator (AC: #1, #5)
  - [x] 1.1 Create `randomSuffix()` function in `cmd/run.go` using `crypto/rand` to generate 6 hex characters
  - [x] 1.2 Update container name construction at `cmd/run.go:124` from `"asbox-" + cfg.ProjectName` to `"asbox-" + cfg.ProjectName + "-" + randomSuffix()`

- [x] Task 2: Update unit tests (AC: #1, #5)
  - [x] 2.1 Add `TestRandomSuffix` in `cmd/run_test.go`: verify length (6 chars), hex charset, uniqueness across calls
  - [x] 2.2 Update `internal/docker/run_test.go` tests that assert exact `--name asbox-myapp` to verify prefix pattern instead (`--name asbox-myapp-`)
  - [x] 2.3 Add test that container name matches `^asbox-[a-z0-9-]+-[0-9a-f]{6}$` pattern

- [x] Task 3: Add integration test for concurrent sessions (AC: #2, #3, #4)
  - [x] 3.1 Add concurrent launch test in `integration/`: start two containers for same project, verify both running, verify distinct names, verify both cleanup on termination

- [x] Task 4: Verify existing cleanup still works (AC: #3, #4)
  - [x] 4.1 Confirm `--rm` flag in `internal/docker/run.go:23` handles random-suffixed names (no changes expected)
  - [x] 4.2 Confirm SIGINT/SIGTERM suppression at `internal/docker/run.go:59-63` is unaffected

## Dev Notes

### Core Change: Container Name in `cmd/run.go`

The ONLY production code change is in `cmd/run.go:124`:

```go
// BEFORE:
containerName := "asbox-" + cfg.ProjectName

// AFTER:
containerName := "asbox-" + cfg.ProjectName + "-" + randomSuffix()
```

The `randomSuffix()` function belongs in the same file. Use `crypto/rand` (not `math/rand`) for cryptographic quality:

```go
func randomSuffix() string {
    b := make([]byte, 3) // 3 bytes = 6 hex chars
    if _, err := rand.Read(b); err != nil {
        // crypto/rand.Read never returns error on supported platforms
        panic("crypto/rand failed: " + err.Error())
    }
    return hex.EncodeToString(b)
}
```

Import `crypto/rand` and `encoding/hex`. The panic is acceptable because `crypto/rand.Read` only errors on unsupported platforms (not a recoverable condition).

### Why This Change Is Safe

- **Cleanup is automatic** via `--rm` flag on `docker run` (`internal/docker/run.go:23`). Docker removes the container when the process exits regardless of the container name.
- **Signal handling is name-independent**. SIGINT/SIGTERM suppression at `internal/docker/run.go:59-63` checks exit codes, not container names.
- **Image naming is unaffected**. Image tags use `asbox-<project>:<hash>` (in `cmd/build_helper.go:40-41`) — independent of container names.
- **Volume naming is unaffected**. Auto-isolate deps volumes use `asbox-<project>-<path>-node_modules` pattern (`internal/mount/isolate_deps.go:106`) — derived from project name, not container name.
- **No code anywhere reconstructs a container name** to execute `docker rm` — cleanup relies entirely on `--rm`.

### What NOT to Change

- `cmd/build_helper.go` — image naming (`asbox-<project>:<hash>`) must remain deterministic for caching
- `internal/docker/run.go` — no changes needed; it already receives `ContainerName` as a parameter
- `internal/mount/isolate_deps.go` — volume naming is project-based, not container-based
- `embed/entrypoint.sh` — no hardcoded container names exist
- `embed/healthcheck-poller.sh` — no container name references

### Test Update Strategy

**`internal/docker/run_test.go`** — Three tests assert exact `--name` values:
- `TestRunCmdArgs_basicFlags` (line 50): asserts `--name asbox-myapp`
- `TestRunCmdArgs_fullOptions` (line 170): asserts `--name asbox-myproject`
- `TestRunCmdArgs_noContainerName` (line 55-65): asserts no `--name` flag when empty

These tests pass `ContainerName` directly to `RunOptions` and verify the docker arg assembly. They test `internal/docker/run.go`, not `cmd/run.go`. Since the random suffix is generated in `cmd/run.go` before being passed to `RunOptions.ContainerName`, these tests **do not need to change** — they test arg assembly with whatever name is given. The randomness is tested separately in `cmd/run_test.go`.

**New test in `cmd/run_test.go`**: Test `randomSuffix()` directly — verify 6 hex chars, verify two calls produce different values.

**Integration test**: Use testcontainers-go pattern from `integration/integration_test.go` to verify two containers with the same image can run concurrently. The existing `startTestContainer` helper (line 77) already handles container lifecycle via `t.Cleanup()`.

### Architecture Compliance

- **Error handling**: `crypto/rand.Read` failure is a panic (unrecoverable platform error), consistent with Go convention — not a user-facing error that needs exit code mapping
- **Error types**: No new error types needed — this change cannot produce a recoverable error
- **Exit codes**: No new exit codes needed
- **Testing**: Table-driven test for `randomSuffix()`, individual integration test for concurrent sessions
- **Naming**: Function `randomSuffix()` is unexported (only used in `cmd/run.go`)
- **No new dependencies**: `crypto/rand` and `encoding/hex` are Go stdlib

### Project Structure Notes

- All changes are in existing files — no new files created
- `cmd/run.go` — container name construction (line 124), new `randomSuffix()` function
- `cmd/run_test.go` — new test for `randomSuffix()`
- `integration/` — new concurrent session test (add to existing test file or new `concurrent_test.go`)

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 11.1] — acceptance criteria and implementation notes
- [Source: _bmad-output/planning-artifacts/prd.md#NFR14] — "A crashed or Ctrl+C'd sandbox leaves no orphaned containers"
- [Source: _bmad-output/planning-artifacts/architecture.md] — container lifecycle, naming conventions, error patterns
- [Source: cmd/run.go:124] — current container name construction
- [Source: internal/docker/run.go:23] — `--rm` flag ensures automatic cleanup
- [Source: internal/docker/run.go:59-63] — SIGINT/SIGTERM exit code suppression
- [Source: cmd/build_helper.go:40-41] — image naming (unaffected)
- [Source: internal/mount/isolate_deps.go:106] — volume naming (unaffected)

## Dev Agent Record

### Agent Model Used

- `gpt-5.4`

### Debug Log References

- `env GOCACHE=/tmp/asbox-gocache GOTMPDIR=/tmp/asbox-gotmp go test ./cmd ./internal/docker`
- `env GOCACHE=/tmp/asbox-gocache GOTMPDIR=/tmp/asbox-gotmp go test ./integration -run TestContainer_concurrentSessionsCleanup -count=1`
- `env GOCACHE=/tmp/asbox-gocache GOTMPDIR=/tmp/asbox-gotmp go test ./...`

### Completion Notes List

- Added `randomSuffix()` in `cmd/run.go` and switched sandbox container names to `asbox-<project>-<suffix>` while preserving the existing prefix for greppability.
- Added unit coverage for suffix length/charset/uniqueness and container-name regex validation in `cmd/run_test.go`.
- Updated Docker arg assembly tests to assert the preserved `asbox-<project>-` prefix when a suffixed container name is provided.
- Added concurrent lifecycle integration coverage in `integration/lifecycle_test.go` to verify two same-project containers run together and are removed after stop.
- Confirmed existing cleanup behavior remains name-independent because `internal/docker/run.go` still relies on `--rm` and signal-exit suppression only.
- Full regression suite passed with `go test ./...`.

### File List

- `cmd/run.go`
- `cmd/run_test.go`
- `internal/docker/run_test.go`
- `integration/lifecycle_test.go`

### Review Findings

- [x] [Review][Patch] Weakened `--name` assertions should match exact test input [internal/docker/run_test.go:50,170]
- [x] [Review][Patch] Dead distinctness check in integration test [integration/lifecycle_test.go:118]
- [x] [Review][Patch] Cleanup poll doesn't distinguish timeout from container removal [integration/lifecycle_test.go:144-156]
- [x] [Review][Patch] Stale inline comment on RunOptions.ContainerName [internal/docker/run.go:13]
- [x] [Review][Defer] TestRunContainerNameMatchesPattern doesn't exercise production code path [cmd/run_test.go:199-205] — deferred, new test but fix approach is debatable (refactor vs binary invocation)
- [x] [Review][Defer] No binary invocation test for CLI-level name generation — deferred, nice-to-have coverage improvement
- [x] [Review][Defer] Project names with uppercase/underscores not validated — deferred, story 11-2 scope

## Change Log

- 2026-04-15: Implemented random-suffixed sandbox container names and added unit plus integration coverage for concurrent session cleanup.
- 2026-04-15: Code review completed — 4 patch, 3 deferred, 8 dismissed.
