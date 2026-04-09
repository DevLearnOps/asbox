# Story 1.6: Image Build with Content-Hash Caching

Status: done

## Story

As a developer,
I want `asbox build` to build my container image and skip rebuilds when nothing has changed,
so that I get fast iteration without unnecessary rebuilds.

## Acceptance Criteria

1. **Given** a valid config.yaml and Dockerfile template **When** the developer runs `asbox build` **Then** a Docker image is built and tagged as `asbox-<project>:<hash>` (12-char SHA256) and `asbox-<project>:latest`

2. **Given** an image already exists with the current content hash **When** the developer runs `asbox build` **Then** the build is skipped and a message indicates the image is up to date

3. **Given** the developer modifies config.yaml or any embedded script **When** they run `asbox build` **Then** a new content hash is computed and the image is rebuilt

4. **Given** the content hash inputs **When** computing the hash **Then** it includes: rendered Dockerfile (which contains the pinned base image digest) + entrypoint.sh + git-wrapper.sh + healthcheck-poller.sh + config.yaml content

5. **Given** `docker build` fails **When** the error is returned **Then** the CLI prints the Docker error output to stderr and exits with code 1

## Tasks / Subtasks

- [x] Task 1: Implement content hash computation (AC: #4)
  - [x] 1.1 Create `internal/hash/hash.go` with `Compute()` function
  - [x] 1.2 SHA256 over concatenated inputs: rendered Dockerfile + entrypoint.sh + git-wrapper.sh + healthcheck-poller.sh + config.yaml raw content
  - [x] 1.3 Return first 12 hex characters of the hash
  - [x] 1.4 Create `internal/hash/hash_test.go` with table-driven tests: determinism, different inputs produce different hashes, 12-char length

- [x] Task 2: Implement image existence check (AC: #2)
  - [x] 2.1 Add `ImageExists(imageRef string) (bool, error)` to `internal/docker/build.go`
  - [x] 2.2 Use `os/exec` to run `docker image inspect <imageRef>` — exit 0 means exists, non-zero means not found
  - [x] 2.3 Add tests for `ImageExists` (mock not needed — unit test the arg assembly; integration test actual check)

- [x] Task 3: Implement `BuildImage()` function (AC: #1, #5)
  - [x] 3.1 Add `BuildImage(opts BuildOptions) error` to `internal/docker/build.go`
  - [x] 3.2 `BuildOptions` struct: `RenderedDockerfile string`, `BuildArgs []string`, `Tags []string`, `Stdout io.Writer`, `Stderr io.Writer`
  - [x] 3.3 Write rendered Dockerfile to a temp file, pass via `-f`
  - [x] 3.4 Execute `docker build` via `os/exec` with `--build-arg` flags and `-t` for each tag
  - [x] 3.5 Stream Docker stdout/stderr to the provided writers
  - [x] 3.6 On failure, return error wrapping Docker's stderr output (exits as code 1 via ConfigError or a new BuildError)
  - [x] 3.7 Add `build_test.go` tests for command assembly, tag formatting

- [x] Task 4: Wire `cmd/build.go` orchestration (AC: #1, #2, #3, #5)
  - [x] 4.1 Replace stub with full pipeline: `config.Parse()` -> read raw config bytes -> `template.Render()` -> read embedded scripts -> `hash.Compute()` -> `docker.ImageExists()` -> conditional `docker.BuildImage()`
  - [x] 4.2 Image tag pattern: `asbox-<project>:<hash>` and `asbox-<project>:latest`
  - [x] 4.3 Print skip message when image is up to date: `"image asbox-<project>:<hash> is up to date, skipping build"`
  - [x] 4.4 Print build success message with image tag
  - [x] 4.5 On `docker build` failure, print Docker error to stderr and exit code 1

- [x] Task 5: Tests (all ACs)
  - [x] 5.1 Unit tests for `hash.Compute()` — determinism, sensitivity to each input, 12-char output
  - [x] 5.2 Unit tests for `docker.ImageExists()` — command arg assembly
  - [x] 5.3 Unit tests for `docker.BuildImage()` — command arg assembly, tag flags, build-arg passthrough
  - [x] 5.4 Verify all existing tests still pass (`go test ./...`)

## Dev Notes

### Orchestration Flow

The `cmd/build.go` command must implement this exact pipeline (from architecture doc):

```
config.Parse(configFile)
  -> read raw config.yaml bytes (for hashing — use os.ReadFile on the resolved config path)
  -> template.Render(cfg)
  -> read embedded scripts from embed.Assets (entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh)
  -> hash.Compute(renderedDockerfile, scripts..., rawConfigContent)
  -> imageRef = fmt.Sprintf("asbox-%s:%s", cfg.ProjectName, contentHash)
  -> docker.ImageExists(imageRef)
  -> if exists: print skip message, return nil
  -> docker.BuildImage(opts) with tags: [imageRef, "asbox-<project>:latest"]
```

### Content Hash Design

- **Pure function**: `hash.Compute()` takes string inputs, returns 12-char hex string. No file I/O inside this function.
- **Hash inputs** (concatenated in deterministic order):
  1. Rendered Dockerfile (already contains the pinned base image digest `FROM ubuntu:24.04@sha256:...`, so base image changes are captured automatically)
  2. `entrypoint.sh` content (read from `embed.Assets`)
  3. `git-wrapper.sh` content (read from `embed.Assets`)
  4. `healthcheck-poller.sh` content (read from `embed.Assets`)
  5. Raw `config.yaml` content (read via `os.ReadFile`, NOT the parsed struct)
- **Why raw config.yaml?** Ensures any config change (even comments or formatting) triggers a rebuild. The rendered Dockerfile already captures the semantic config changes, but the raw file catches edge cases.
- Use `crypto/sha256` from Go stdlib. No external dependencies.
- Return `fmt.Sprintf("%x", sum)[:12]` — first 12 hex chars of the SHA256 digest.

### Docker Build Execution

- Write the rendered Dockerfile to a temp file (`os.CreateTemp`), clean up with `defer os.Remove(tmpFile.Name())`
- Build context is the `embed/` directory content — scripts need to be available for COPY directives. Write embedded assets to a temp directory as the build context.
- Command: `docker build -f <tmpDockerfile> -t asbox-<project>:<hash> -t asbox-<project>:latest --build-arg NODE_VERSION=22 ... <contextDir>`
- Use `cmd.Stdout = os.Stdout` and `cmd.Stderr = os.Stderr` to stream Docker output live
- The existing `docker.BuildArgs(cfg)` function already assembles `--build-arg` flags — reuse it

### Image Existence Check

- Run `docker image inspect asbox-<project>:<hash>` via `os/exec`
- Discard stdout (we only care about exit code)
- Exit 0 = image exists (skip build), non-zero = image not found (proceed with build)
- Do NOT use `docker images -q` — `inspect` is more reliable for exact tag matching

### Error Handling

- `docker build` failure: wrap the error with Docker's stderr content, return as error (maps to exit code 1 via existing error-to-exit-code logic in `cmd/root.go`)
- Config parse failure: already handled by `config.Parse()` returning `ConfigError`
- Template render failure: already handled by `template.Render()` returning `TemplateError`
- Consider adding a `BuildError` type in `internal/docker/errors.go` for docker build failures (maps to exit code 1)

### Project Structure Notes

Files to create:
- `internal/hash/hash.go` — `Compute()` function (delete `.gitkeep`)
- `internal/hash/hash_test.go` — table-driven tests

Files to modify:
- `internal/docker/build.go` — add `ImageExists()`, `BuildImage()`, `BuildOptions` struct
- `internal/docker/build_test.go` — add tests for new functions
- `cmd/build.go` — replace stub with full orchestration pipeline

Files NOT to modify:
- `internal/config/` — config parsing is complete
- `internal/template/` — template rendering is complete
- `embed/` — all embedded assets are complete from story 1-5

### Existing Patterns to Follow

- **Error types**: Each package defines its own error type in `errors.go` (e.g., `config.ConfigError`, `docker.DependencyError`, `template.TemplateError`). Follow this pattern for `docker.BuildError` if needed.
- **Test style**: Table-driven tests with `t.Run()` subtests. See `internal/config/parse_test.go` and `internal/docker/build_test.go` for examples.
- **Function naming**: Verb-first exported functions: `Parse()`, `Render()`, `BuildArgs()`. Follow with `Compute()`, `ImageExists()`, `BuildImage()`.
- **No cross-package imports**: Internal packages don't import each other. `cmd/` orchestrates by calling each package independently.
- **Config struct passed by value**: `config.Config` is passed to consuming packages. Don't add methods to it from other packages.
- **Embedded assets access**: Use `embed.Assets.ReadFile("filename")` — see `internal/template/render.go:14` for the pattern.

### Previous Story Intelligence

From story 1-5 completion:
- All embedded scripts are fully implemented and tested (entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh)
- `embed.Assets` already exports all 6 files needed (Dockerfile.tmpl, entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh, agent-instructions.md.tmpl, config.yaml)
- 34 Go tests pass across all packages — do not break existing tests
- `go vet` is clean — keep it clean
- Template rendering produces valid Dockerfile output — `template.Render()` is the tested entry point
- `docker.BuildArgs()` correctly assembles `--build-arg` flags for all SDK combinations

### Git Intelligence

Recent commit pattern: `feat: implement story 1-X <story description>`
All 5 previous stories landed cleanly. No merge conflicts or regressions detected.
Files touched in recent commits align with architecture — `cmd/`, `internal/`, `embed/` directories.

### References

- [Source: _bmad-output/planning-artifacts/architecture.md — Content-Hash Caching section]
- [Source: _bmad-output/planning-artifacts/architecture.md — Code Structure section]
- [Source: _bmad-output/planning-artifacts/epics.md — Story 1.6]
- [Source: _bmad-output/planning-artifacts/prd.md — FR10, FR12, FR13, FR43]
- [Source: _bmad-output/implementation-artifacts/1-5-container-scripts-and-tooling-in-dockerfile-template.md — Completion Notes]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None — clean implementation with no debugging required.

### Completion Notes List

- Implemented `hash.Compute()` as a pure function using `crypto/sha256`, returning first 12 hex chars
- Added `ImageExists()` using `docker image inspect` with exit code detection
- Added `BuildImage()` with temp Dockerfile + temp context directory for embedded assets, `BuildOptions` struct, and `BuildError` error type
- Added `buildCmdArgs()` helper (exported for testability) that assembles the `docker build` CLI arguments
- Wired full orchestration pipeline in `cmd/build.go`: Parse -> ReadFile -> Render -> ReadEmbedded -> Compute -> ImageExists -> BuildImage
- Added `BuildError` to `exitCode()` mapping in `cmd/root.go` (maps to exit code 1)
- 46 total tests pass across all packages, zero regressions
- `go vet` is clean

### File List

- `internal/hash/hash.go` — NEW: `Compute()` content hash function
- `internal/hash/hash_test.go` — NEW: table-driven tests for hash computation
- `internal/hash/.gitkeep` — DELETED: replaced by actual implementation
- `internal/docker/build.go` — MODIFIED: added `BuildOptions`, `ImageExists()`, `BuildImage()`, `buildCmdArgs()`
- `internal/docker/build_test.go` — MODIFIED: added tests for `ImageExists`, `BuildImage` command assembly, tag formatting
- `internal/docker/errors.go` — MODIFIED: added `BuildError` type
- `cmd/build.go` — MODIFIED: replaced stub with full build orchestration pipeline
- `cmd/root.go` — MODIFIED: added `BuildError` to `exitCode()` mapping

### Review Findings

- [x] [Review][Decision→Patch] `ImageExists` — distinguish Docker errors from "not found" via exit code check [internal/docker/build.go:28] — FIXED
- [x] [Review][Patch] Hash concatenation — added null-byte delimiter between inputs [internal/hash/hash.go:13] — FIXED
- [x] [Review][Patch] `TestImageExists` — replaced no-op test with real assertion [internal/docker/build_test.go:76] — FIXED
- [x] [Review][Patch] `BuildError` added to `TestExitCode_mapping` test table [cmd/root_test.go:38] — FIXED

### Change Log

- 2026-04-09: Implemented story 1-6 — image build with content-hash caching. Added hash computation, image existence check, Docker build execution, and full CLI orchestration.
