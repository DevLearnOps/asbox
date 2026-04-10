# Story 9.1: Integration Test Infrastructure

Status: done

## Story

As a developer,
I want a testcontainers-go integration test framework with shared helpers and test fixtures,
so that I can write and run comprehensive sandbox tests.

## Acceptance Criteria

1. **Given** the integration test directory
   **When** running `go test ./integration/... -v`
   **Then** tests execute using testcontainers-go and produce clear pass/fail results

2. **Given** the test infrastructure
   **When** inspecting test fixtures
   **Then** fixtures include: a minimal `.asbox/config.yaml`, a small project directory with `package.json`, a git repo initialized with a remote, and a `docker-compose.yml` for inner container tests

3. **Given** multiple independent test cases
   **When** tests run
   **Then** they execute in parallel via `t.Parallel()` for faster feedback

## Tasks / Subtasks

- [x] Task 1: Add missing test fixtures to `integration/testdata/` (AC: #2)
  - [x] 1.1 Create `integration/testdata/config.yaml` — minimal `.asbox/config.yaml` with `agent: claude-code`, `project_name: test-project`, empty SDKs (sufficient to render a buildable Dockerfile)
  - [x] 1.2 Create `integration/testdata/project/package.json` — minimal `{"name": "test-project", "version": "1.0.0"}` for `auto_isolate_deps` testing in story 9.4
  - [x] 1.3 Create `integration/testdata/docker-compose.yml` — minimal two-service compose file (already have `tests/fixtures/docker-compose.yml` as reference, but testdata is the Go convention for test-only data)
  - [x] 1.4 Add fixture creation helper `setupGitRepoWithRemote(t *testing.T) string` in `integration/integration_test.go` — creates a temp dir with `git init`, configures user.email/user.name, creates an initial commit, adds a remote via `git remote add origin https://example.com/test.git`, returns the path. Uses `t.TempDir()` for automatic cleanup.

- [x] Task 2: Extend shared helpers in `integration/integration_test.go` (AC: #1, #3)
  - [x] 2.1 Add `buildTestImageWithConfig(t *testing.T, cfg *config.Config) string` — builds a sandbox image from a custom config (for stories 9.2-9.4 that need MCP, auto_isolate_deps, etc.). Follows the existing `buildTestImage` pattern but accepts a config parameter.
  - [x] 2.2 Add `fileExistsInContainer(ctx context.Context, t *testing.T, container testcontainers.Container, path string) bool` — runs `test -f <path>` in the container, returns true/false without failing the test. For use in assertions.
  - [x] 2.3 Add `fileContentInContainer(ctx context.Context, t *testing.T, container testcontainers.Container, path string) string` — runs `cat <path>` in the container and returns content. Fatals if path doesn't exist.
  - [x] 2.4 Add `startTestContainerWithMounts(ctx context.Context, t *testing.T, image string, mounts []testcontainers.ContainerMount) testcontainers.Container` — starts a container with bind mounts for mount verification tests in story 9.2.

- [x] Task 3: Add Makefile targets for unit vs integration tests (AC: #1)
  - [x] 3.1 Add `test-unit` target: `go test -short ./...` (runs unit tests only, skips integration)
  - [x] 3.2 Add `test-integration` target: `go test -v ./integration/...` (runs only integration tests)
  - [x] 3.3 Update existing `test` target to run both: `go test ./...`
  - [x] 3.4 Add `test-ci` target: `go vet ./... && go test -short ./... && go test -v -count=1 ./integration/...` (full CI pipeline locally)

- [x] Task 4: Create `.github/workflows/ci.yml` (AC: #1)
  - [x] 4.1 Trigger on: push to `main` and `feat/*` branches, pull requests to `main`
  - [x] 4.2 Job 1 `build-and-unit-test`: runs on all triggers — Go setup, `go vet ./...`, `go build .`, `go test -short ./...`
  - [x] 4.3 Job 2 `integration-test`: depends on job 1, runs ONLY on pull_request events (`if: github.event_name == 'pull_request'`). Runs `go test -v -count=1 ./integration/...`. Integration tests are slow (sandbox image build takes minutes) so skip on push.
  - [x] 4.4 Use `actions/setup-go@v5` with `go-version-file: 'go.mod'` to auto-detect Go version
  - [x] 4.5 Cache Go modules via `actions/cache@v4` for `~/go/pkg/mod` and `~/.cache/go-build`
  - [x] 4.6 Integration job needs no special Docker config — GitHub Actions Ubuntu runners have Docker pre-installed and support privileged containers. The `testcontainers-go` `HostConfigModifier` in `podman_test.go` sets `Privileged: true` on the test container request, which works on standard GitHub Actions runners without additional configuration.

- [x] Task 5: Verify existing infrastructure satisfies all ACs (AC: #1, #2, #3)
  - [x] 5.1 Run `go test -v ./integration/...` and confirm existing tests pass
  - [x] 5.2 Run `go test -short ./...` and confirm unit tests still pass
  - [x] 5.3 Verify `t.Parallel()` is used in all subtests across integration test files
  - [x] 5.4 Run `go vet ./...` and confirm zero issues

## Dev Notes

### What Already Exists (DO NOT recreate)

The integration test infrastructure was built organically across stories 3.1 through 8.1. Story 9.1 formalizes and completes it.

- **`integration/integration_test.go`** (140 lines) — Core helpers already in place:
  - `buildTestImage(t)` — builds sandbox image from minimal config, cleans up with `t.Cleanup`
  - `startTestContainer(ctx, t, image)` — starts container with `tail -f /dev/null`, uses `wait.ForExec`
  - `startTestContainerWithEntrypoint(ctx, t, image)` — in `podman_test.go`, starts with real entrypoint + privileged mode, waits for Podman socket
  - `execInContainer(ctx, t, container, cmd)` — runs command as root
  - `execAsUser(ctx, t, container, user, cmd)` — runs as specified user with profile sourcing
  - `TestMain(m *testing.M)` — basic test setup
  
- **`integration/isolation_test.go`** (110 lines) — Tests: git push blocking, git operations, SSH/AWS absence, git wrapper ownership
- **`integration/podman_test.go`** (236 lines) — Tests: docker version, vfs driver, compose version, DOCKER_HOST, socket, testcontainers vars
- **`integration/inner_container_test.go`** (190 lines) — Tests: docker build, compose multi-service, DNS resolution, inner port reachability

- **`tests/fixtures/`** — Existing fixtures:
  - `docker-compose.yml` — Two-service compose (web + client)
  - `Dockerfile.inner` — Minimal alpine HTTP server for inner container tests
  - `.mcp.json` — Playwright MCP server config

- **`go.mod`** — testcontainers-go v0.41.0 already a direct dependency
- **`Makefile`** — Has `test: go test ./...` target

- **Testing patterns** already established:
  - `if testing.Short() { t.Skip("skipping integration test in short mode") }` at top of each test function
  - `t.Run("subtest_name", func(t *testing.T) { t.Parallel(); ... })` for subtests
  - `t.Cleanup()` for resource cleanup (images, containers)
  - `t.Helper()` on all helper functions
  - stdlib `testing` only — NO testify/assert (testify is transitive from testcontainers but NOT used directly)
  - Error messages follow `"expected X, got Y"` format
  - Image tags use nanosecond timestamps to avoid collision: `fmt.Sprintf("%s:%d", testImageName, time.Now().UnixNano())`

### What Must Be Implemented

See Tasks section above for complete breakdown. Summary: test fixtures in `integration/testdata/`, additional helpers in `integration_test.go`, Makefile targets for unit/integration separation, and `.github/workflows/ci.yml` for CI.

### Architecture Compliance

**File locations:**
- New helpers go in `integration/integration_test.go` — per architecture, this is the shared setup file
- Test fixtures go in `integration/testdata/` — Go convention for test data
- CI workflow goes in `.github/workflows/ci.yml` — per architecture project structure
- Makefile modifications in the existing `Makefile`

**Testing framework:**
- Go's built-in `testing` package — per architecture foundation decisions
- testcontainers-go for Docker-based testing — per architecture foundation decisions
- `t.Parallel()` for parallel execution — per epics AC #3
- `testing.Short()` for separating unit/integration — established pattern

**No new Go dependencies.** All helpers use existing `testcontainers-go` and `docker/docker` packages already in go.mod.

**Anti-patterns to avoid:**
- Do NOT import testify/assert — the project uses stdlib testing only
- Do NOT add color output to test helpers — consistent with project output conventions
- Do NOT create a separate `testhelpers` package — helpers belong in `integration_test.go` (same package, unexported)
- Do NOT move existing fixtures from `tests/fixtures/` — those may be used by shell scripts; create separate `testdata/` for Go tests
- Do NOT add `//go:embed` in integration test files — use `os.ReadFile` or inline test data (embed is centralized in `embed/embed.go`)

### Library and Framework Requirements

- `github.com/testcontainers/testcontainers-go v0.41.0` — already in go.mod
- `github.com/testcontainers/testcontainers-go/wait` — already imported
- `github.com/docker/docker/api/types/container` — already imported in podman_test.go
- `github.com/docker/docker/client` — already imported in podman_test.go, inner_container_test.go
- No new external dependencies

### Testing Standards

- All helper functions must use `t.Helper()` for correct error line reporting
- All integration tests must have `if testing.Short() { t.Skip(...) }` guard
- Subtests use `t.Run()` with `t.Parallel()` where tests are independent
- Resource cleanup via `t.Cleanup()` — not `defer` alone (survives Fatal)
- Image cleanup via `exec.Command("docker", "rmi", "-f", tag)` in cleanup func
- Error format: `t.Errorf("expected X, got %d; output: %s", val, output)`

### File Structure Requirements

New and modified files:
```
integration/
├── integration_test.go       # MODIFY — add new helpers
├── testdata/                  # NEW directory
│   ├── config.yaml            # NEW — minimal asbox config fixture
│   ├── project/               # NEW directory
│   │   └── package.json       # NEW — minimal package.json fixture
│   └── docker-compose.yml     # NEW — self-contained compose fixture
├── isolation_test.go          # UNCHANGED
├── podman_test.go             # UNCHANGED
└── inner_container_test.go    # UNCHANGED
.github/
└── workflows/
    └── ci.yml                 # NEW — GitHub Actions CI workflow
Makefile                       # MODIFY — add test-unit, test-integration, test-ci targets
```

### CI Workflow Design Notes

The CI workflow must handle that integration tests:
- Require Docker (GitHub Actions Ubuntu runners have Docker pre-installed)
- Build a sandbox image (which installs Podman, SDKs, etc.) — takes several minutes
- Need privileged containers for Podman-inside-Docker tests — GitHub Actions Ubuntu runners support this natively, no special config needed. The `testcontainers-go` `HostConfigModifier` setting `Privileged: true` in `podman_test.go` works on standard runners.
- Should use `-count=1` to disable test caching (integration tests depend on Docker state)
- Integration tests run ONLY on pull_request events (too slow for every push); unit tests run on all pushes

```yaml
on:
  push:
    branches: [main, 'feat/*']
  pull_request:
    branches: [main]

jobs:
  build-and-unit-test:
    runs-on: ubuntu-latest
    steps:
      - checkout
      - setup-go (go-version-file: go.mod)
      - go vet ./...
      - go build .
      - go test -short ./...

  integration-test:
    needs: build-and-unit-test
    if: github.event_name == 'pull_request'
    runs-on: ubuntu-latest
    steps:
      - checkout
      - setup-go (go-version-file: go.mod)
      - go test -v -count=1 ./integration/...
```

### Previous Story Intelligence

The last completed story (8-1 bmad-multi-repo-mounts-and-agent-instructions) established:
- **Test pattern**: Table-driven tests with `t.TempDir()`, `errors.As()` for typed error checks
- **Integration point**: `cmd/run.go` is the orchestration hub
- **Review findings**: Always check for cleanup (temp files, containers), handle edge cases (degenerate paths)
- **All stories**: Single commit per story, `go test ./...` must pass with zero failures

Existing integration tests (built over stories 3.1-8.1) already cover:
- Git push blocking and isolation (isolation_test.go)
- Podman/Docker alias and configuration (podman_test.go)
- Inner container build, compose, port reachability (inner_container_test.go)
- Container privilege and socket mount checks (podman_test.go)

### Git Intelligence

Recent commit pattern: `feat: implement story X-Y description`
- Go 1.25.0
- All tests must pass: `go test ./...`
- Single commit per story with all changes
- `go vet ./...` must produce zero warnings

### References

- [Source: architecture.md:99-101 — Testing Framework: Go testing + testcontainers-go]
- [Source: architecture.md:128-129 — integration/ directory in project structure]
- [Source: architecture.md:332 — Test naming: TestFunctionName_scenario, table-driven preferred]
- [Source: architecture.md:482-489 — Integration test file list per architecture plan]
- [Source: architecture.md:490-492 — .github/workflows/ci.yml in project structure]
- [Source: architecture.md:638-648 — Validation: testcontainers-go integration test suite for NFR coverage]
- [Source: epics.md:869-892 — Story 9.1 requirements and implementation notes]
- [Source: epics.md:894-969 — Stories 9.2-9.4 requirements (what the infrastructure must support)]
- [Source: prd.md — NFR15: Integration test suite covers all supported use cases with parallel execution]
- [Source: integration/integration_test.go — Existing helper functions and patterns]
- [Source: integration/podman_test.go — startTestContainerWithEntrypoint helper for Podman tests]
- [Source: go.mod — testcontainers-go v0.41.0 already present]
- [Source: Makefile — Current test target]
- [Source: tests/fixtures/ — Existing test fixtures (docker-compose.yml, Dockerfile.inner, .mcp.json)]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None — clean implementation with no blockers.

### Completion Notes List

- Task 1: Created test fixtures in `integration/testdata/` (config.yaml, project/package.json, docker-compose.yml) and added `setupGitRepoWithRemote` helper using `t.TempDir()` for automatic cleanup.
- Task 2: Added four new shared helpers to `integration/integration_test.go`: `buildTestImageWithConfig` (custom config image builds), `fileExistsInContainer` (non-fatal file check), `fileContentInContainer` (file content reader with fatal on missing), `startTestContainerWithMounts` (bind mount container startup). All follow existing patterns: `t.Helper()`, `t.Cleanup()`, nanosecond image tags.
- Task 3: Added `test-unit`, `test-integration`, and `test-ci` Makefile targets. Existing `test` target preserved as-is (already runs `go test ./...`).
- Task 4: Created `.github/workflows/ci.yml` with two jobs: `build-and-unit-test` (runs on all pushes and PRs) and `integration-test` (PR-only, depends on unit tests passing). Uses `actions/setup-go@v5` with `go-version-file`, `actions/cache@v4` for Go modules.
- Task 5: All verification passed — `go vet` zero issues, unit tests pass in short mode, integration tests pass (55s), `t.Parallel()` confirmed in all subtests across all integration test files.

### Change Log

- 2026-04-10: Implemented story 9-1 — test fixtures, shared helpers, Makefile targets, and CI workflow

### File List

- integration/testdata/config.yaml (NEW)
- integration/testdata/project/package.json (NEW)
- integration/testdata/docker-compose.yml (NEW)
- integration/integration_test.go (MODIFIED — added 5 helpers: buildTestImageWithConfig, fileExistsInContainer, fileContentInContainer, startTestContainerWithMounts, setupGitRepoWithRemote)
- Makefile (MODIFIED — added test-unit, test-integration, test-ci targets)
- .github/workflows/ci.yml (NEW)

### Review Findings

- [x] [Review][Patch] `fileContentInContainer` missing `tcexec.Multiplexed()` — returns raw Docker stream with binary framing headers instead of clean content. Callers doing exact string comparison will get corrupted output. Fix: pass `tcexec.Multiplexed()` to `container.Exec`. [integration/integration_test.go:202] — fixed
- [x] [Review][Patch] `setupGitRepoWithRemote` missing `init.defaultBranch` — `git init` without `-b main` uses system default branch name, which varies across environments and causes git warnings on CI. Fix: change to `{"git", "init", "-b", "main"}`. [integration/integration_test.go:255] — fixed
- [x] [Review][Defer] `execInContainer` also missing `tcexec.Multiplexed()` — same stream framing issue, pre-existing. Current callers tolerate via `strings.Contains`. [integration/integration_test.go:108] — deferred, pre-existing
- [x] [Review][Defer] Cleanup closures capture caller's `ctx` — `startTestContainer` (pre-existing) and `startTestContainerWithMounts` both use caller's ctx in `t.Cleanup`. Not active since all callers use `context.Background()`. [integration/integration_test.go:94,237] — deferred, pre-existing
