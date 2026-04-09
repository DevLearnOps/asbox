# Story 1.7: Sandbox Run with TTY and Lifecycle

Status: done

## Story

As a developer,
I want `asbox run` to launch my sandbox in interactive TTY mode with proper signal handling,
so that I can interact with the agent and cleanly stop it with Ctrl+C.

## Acceptance Criteria

1. **Given** an image exists for the current config **When** the developer runs `asbox run` **Then** the container launches in TTY mode (`-it`) with tini as PID 1 and the configured agent as the foreground process

2. **Given** no image exists for the current config **When** the developer runs `asbox run` **Then** the image is automatically built first, then the container launches (FR12)

3. **Given** config changes have been made since the last build **When** the developer runs `asbox run` **Then** the image is automatically rebuilt before launching (FR13)

4. **Given** a running sandbox session **When** the developer presses Ctrl+C **Then** tini forwards SIGTERM, the agent terminates, and the container exits with no orphaned containers or dangling networks (NFR14)

5. **Given** the host user has UID 1001 **When** the container starts **Then** the entrypoint aligns the sandbox user's UID/GID via HOST_UID/HOST_GID env vars

6. **Given** the host user has UID 1000 matching the image default user **When** the container starts **Then** the entrypoint skips UID modification

7. **Given** a config declaring secrets `[ANTHROPIC_API_KEY]` and the variable is set in the host environment **When** the sandbox launches **Then** `ANTHROPIC_API_KEY` is available inside the container via `--env` flag

8. **Given** a declared secret is not set in the host environment **When** the developer runs `asbox run` **Then** the CLI exits with code 4 and prints an error message to stderr (FR16)

## Tasks / Subtasks

- [x] Task 1: Implement `RunContainer()` in `internal/docker/run.go` (AC: #1, #4, #5)
  - [x] 1.1 Create `RunOptions` struct with fields for image ref, env vars, container name, stdout/stderr writers
  - [x] 1.2 Implement `RunContainer(opts RunOptions) error` — assembles and executes `docker run` via `os/exec`
  - [x] 1.3 Implement `runCmdArgs(opts RunOptions) []string` helper for testable command assembly
  - [x] 1.4 Flags: `-it` (interactive TTY), `--rm` (auto-remove on exit), `--init` (tini as PID 1)
  - [x] 1.5 Pass HOST_UID/HOST_GID as `--env HOST_UID=<uid> --env HOST_GID=<gid>`
  - [x] 1.6 Pass each env var and secret as `--env KEY=VALUE`
  - [x] 1.7 Connect cmd.Stdin to os.Stdin for interactive TTY
  - [x] 1.8 On failure, return `RunError` with Docker's stderr content

- [x] Task 2: Add `RunError` type to `internal/docker/errors.go` (AC: #1)
  - [x] 2.1 Add `RunError` struct matching existing `BuildError`/`DependencyError` pattern
  - [x] 2.2 Map `RunError` to exit code 1 in `cmd/root.go` `exitCode()` function

- [x] Task 3: Implement secret validation (AC: #7, #8)
  - [x] 3.1 Add `SecretError` type to `internal/config/errors.go` if not already present
  - [x] 3.2 Implement secret validation in `cmd/run.go`: iterate `cfg.Secrets`, call `os.LookupEnv()` for each
  - [x] 3.3 If a secret is not set (LookupEnv returns false), return `config.SecretError` with message: `"secret '<name>' not set in host environment. Export it or remove from .asbox/config.yaml secrets list"`
  - [x] 3.4 If secret is set to empty string, pass it through — empty is valid

- [x] Task 4: Wire `cmd/run.go` orchestration (AC: #1, #2, #3, #5, #6, #7, #8)
  - [x] 4.1 Parse config: `config.Parse(configFile)`
  - [x] 4.2 Build-if-needed: reuse build pipeline logic (read raw config, render template, read scripts, compute hash, check image exists, build if missing)
  - [x] 4.3 Validate secrets via `os.LookupEnv()` — fail with exit code 4 before launching
  - [x] 4.4 Assemble env vars: HOST_UID/HOST_GID from `os.Getuid()`/`os.Getgid()`, secrets from `os.LookupEnv()`, custom env from `cfg.Env`
  - [x] 4.5 Construct container name: `asbox-<project>` for easy identification
  - [x] 4.6 Call `docker.RunContainer(opts)` with assembled options
  - [x] 4.7 Print info message before launch: `"launching sandbox asbox-<project>..."`

- [x] Task 5: Tests (all ACs)
  - [x] 5.1 Unit tests for `RunContainer()` command assembly — verify `-it`, `--rm`, `--init`, `--env` flags, container name
  - [x] 5.2 Unit tests for `runCmdArgs()` — table-driven tests for various option combinations
  - [x] 5.3 Unit tests for secret validation — secret set, secret missing, secret empty
  - [x] 5.4 Unit test for `RunError` exit code mapping
  - [x] 5.5 Verify all existing tests still pass (`go test ./...`)

## Dev Notes

### Orchestration Flow

The `cmd/run.go` command must implement this pipeline (from architecture doc):

```
config.Parse(configFile)
  -> read raw config.yaml bytes (for hashing)
  -> template.Render(cfg)
  -> read embedded scripts (entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh)
  -> hash.Compute(renderedDockerfile, scripts..., rawConfigContent)
  -> imageRef = fmt.Sprintf("asbox-%s:%s", cfg.ProjectName, contentHash)
  -> docker.ImageExists(imageRef)
  -> if NOT exists: run full build pipeline (same as cmd/build.go)
  -> validate secrets: for each cfg.Secrets, os.LookupEnv() — fail with SecretError if not set
  -> assemble env vars (HOST_UID, HOST_GID, secrets, cfg.Env)
  -> docker.RunContainer(opts)
```

### Build-If-Needed Pattern

The auto-build logic must replicate the same hash computation as `cmd/build.go`. The image tag format is `asbox-<project>:<hash>`. If `docker.ImageExists(imageRef)` returns false, trigger a full build using the same pipeline as `cmd/build.go`:

```go
// Same steps as cmd/build.go lines 25-86
rawConfig, _ := os.ReadFile(configFile)
rendered, _ := template.Render(cfg)
scripts := readEmbeddedScripts()  // entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh
contentHash := hash.Compute(rendered, scripts[0], scripts[1], scripts[2], string(rawConfig))
imageRef := fmt.Sprintf("asbox-%s:%s", cfg.ProjectName, contentHash)
if exists, _ := docker.ImageExists(imageRef); !exists {
    // Build image — same logic as cmd/build.go
}
```

**Important:** Consider extracting a shared `ensureBuild()` helper function in `cmd/` to avoid duplicating build logic between `cmd/build.go` and `cmd/run.go`. Both need the same hash computation + build pipeline. Keep the helper in `cmd/` (not `internal/`) since it orchestrates across packages. This is an acceptable refactor for story 1-7 since both commands exist in `cmd/`.

### RunContainer Design

`internal/docker/run.go` must implement:

```go
type RunOptions struct {
    ImageRef      string            // e.g., "asbox-myapp:a1b2c3d4e5f6"
    ContainerName string            // e.g., "asbox-myapp"
    EnvVars       map[string]string // HOST_UID, HOST_GID, secrets, custom env
    Stdin         io.Reader         // os.Stdin for interactive TTY
    Stdout        io.Writer         // os.Stdout
    Stderr        io.Writer         // os.Stderr
}

func RunContainer(opts RunOptions) error
```

Docker command assembly:
```
docker run -it --rm --init \
  --name asbox-<project> \
  --env HOST_UID=<uid> --env HOST_GID=<gid> \
  --env ANTHROPIC_API_KEY=<val> \
  --env CUSTOM_VAR=<val> \
  asbox-<project>:<hash>
```

Key flags:
- `-it` — interactive TTY mode, required for agent interaction
- `--rm` — auto-remove container on exit, prevents container accumulation (NFR14)
- `--init` — uses tini as PID 1 for signal forwarding (FR48). Note: the Dockerfile also has `ENTRYPOINT ["/usr/bin/tini", "--"]` but `--init` is a defense-in-depth measure
- `--name asbox-<project>` — deterministic container name for easy `docker stop`/`docker logs`

**Stdin must be connected:** Unlike `BuildImage()`, `RunContainer()` must set `cmd.Stdin = opts.Stdin` for interactive TTY to work. Without this, the container starts but the terminal is not interactive.

### Secret Validation (FR16)

Validate secrets AFTER build-if-needed but BEFORE `docker run`:

```go
for _, secret := range cfg.Secrets {
    val, ok := os.LookupEnv(secret)
    if !ok {
        return &config.SecretError{
            Msg: fmt.Sprintf("secret '%s' not set in host environment. Export it or remove from .asbox/config.yaml secrets list", secret),
        }
    }
    envVars[secret] = val  // empty string is valid
}
```

`SecretError` already exists in `internal/config/errors.go` and maps to exit code 4 in `cmd/root.go`. Verify this before implementing — if `SecretError` doesn't exist yet, add it following the existing error type pattern.

### HOST_UID/HOST_GID Injection

Pass the current host user's UID and GID so `entrypoint.sh` can align the sandbox user:

```go
envVars["HOST_UID"] = strconv.Itoa(os.Getuid())
envVars["HOST_GID"] = strconv.Itoa(os.Getgid())
```

The entrypoint.sh already handles UID/GID alignment — it reads these env vars and runs `usermod`/`groupmod`. No container-side changes needed.

### Error Handling

| Error Source | Error Type | Exit Code |
|---|---|---|
| Config parsing fails | `config.ConfigError` | 1 |
| Template render fails | `template.TemplateError` | 1 |
| Docker not found | `docker.DependencyError` | 3 (handled by PersistentPreRunE in root.go) |
| Secret not set in host env | `config.SecretError` | 4 |
| Docker build fails | `docker.BuildError` | 1 |
| Docker run fails | `docker.RunError` | 1 |

### Scope Boundaries — What This Story Does NOT Include

This story implements the basic `docker run` with TTY, auto-build, secrets, HOST_UID/HOST_GID, and custom env vars. The following are implemented in later stories:

- **Mount flags** (`-v` source:target) — Story 2-1 implements mount assembly via `internal/mount/`
- **auto_isolate_deps volumes** — Story 6-1
- **host_agent_config mount** — Story 7-1
- **bmad_repos mounts** — Story 8-1

For story 1-7, `RunContainer()` should accept mount flags but the `cmd/run.go` orchestration does NOT assemble any mounts yet. The `RunOptions` struct should include a `Mounts []string` field (each entry is a `-v source:target` flag pair) even though it will be empty for now, so later stories can plug in without modifying the interface.

### Project Structure Notes

Files to create:
- `internal/docker/run.go` — `RunContainer()`, `RunOptions`, `runCmdArgs()`
- `internal/docker/run_test.go` — table-driven tests for command assembly

Files to modify:
- `cmd/run.go` — replace stub with full orchestration pipeline
- `internal/docker/errors.go` — add `RunError` type
- `cmd/root.go` — add `RunError` to `exitCode()` mapping

Files NOT to modify:
- `internal/config/` — config parsing is complete (verify SecretError exists)
- `internal/template/` — template rendering is complete
- `internal/hash/` — hash computation is complete
- `embed/` — all embedded assets are complete
- `internal/docker/build.go` — build logic is complete

### Existing Patterns to Follow

- **Error types**: Each package defines its own error type in `errors.go` (e.g., `docker.DependencyError`, `docker.BuildError`). Add `docker.RunError` following same pattern.
- **Test style**: Table-driven tests with `t.Run()` subtests. See `internal/docker/build_test.go` for the pattern in the same package.
- **Function naming**: Verb-first exported functions: `BuildImage()`, `ImageExists()`. Follow with `RunContainer()`.
- **Command assembly**: `buildCmdArgs()` helper is exported for testability in `build.go`. Follow same pattern with `runCmdArgs()`.
- **No cross-package imports**: Internal packages don't import each other. `cmd/` orchestrates.
- **Piping I/O**: `BuildImage()` takes `Stdout`/`Stderr` writers. `RunContainer()` must additionally take `Stdin` reader for interactive TTY.
- **Struct-based options**: `BuildOptions` struct pattern — follow with `RunOptions`.

### Previous Story Intelligence

From story 1-6 completion:
- 46 Go tests pass across all packages — do not break existing tests
- `go vet` is clean — keep it clean
- `docker.ImageExists()` distinguishes "not found" (false, nil) from Docker errors (false, err) via exit code check — reuse this exact function
- `docker.BuildImage()` uses temp directories for build context — no temp dirs needed for `RunContainer()`
- `docker.BuildArgs()` assembles `--build-arg` flags — not needed for `docker run`, but shows the flag assembly pattern
- Hash uses null-byte delimiter between inputs — do not change hash computation
- `cmd/build.go` reads embedded scripts via `asboxEmbed.Assets.ReadFile("filename")` — reuse same pattern in `cmd/run.go` for build-if-needed

### Git Intelligence

Recent commit pattern: `feat: implement story 1-X <story description>`
All 6 previous stories landed cleanly. No merge conflicts or regressions detected.
Key files from recent commits:
- `cmd/build.go` — full build orchestration (pattern to follow)
- `internal/docker/build.go` — `BuildImage()`, `ImageExists()`, `BuildArgs()`
- `internal/docker/errors.go` — `DependencyError`, `BuildError`
- `internal/hash/hash.go` — `Compute()` function
- `cmd/root.go` — `exitCode()` mapping, `PersistentPreRunE` Docker check

### References

- [Source: _bmad-output/planning-artifacts/architecture.md — Container Lifecycle section]
- [Source: _bmad-output/planning-artifacts/architecture.md — Secret Injection decision]
- [Source: _bmad-output/planning-artifacts/architecture.md — Error Handling Strategy]
- [Source: _bmad-output/planning-artifacts/architecture.md — Code Structure: cmd/run.go]
- [Source: _bmad-output/planning-artifacts/architecture.md — Implementation Patterns & Consistency Rules]
- [Source: _bmad-output/planning-artifacts/epics.md — Story 1.7]
- [Source: _bmad-output/planning-artifacts/prd.md — FR10-FR16, FR48, NFR14]
- [Source: _bmad-output/implementation-artifacts/1-6-image-build-with-content-hash-caching.md — Completion Notes]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None — clean implementation, no debug issues encountered.

### Completion Notes List

- Implemented `RunContainer()` with `RunOptions` struct and `runCmdArgs()` helper in `internal/docker/run.go`
- Added `RunError` type following existing `BuildError`/`DependencyError` pattern, mapped to exit code 1
- `SecretError` already existed in `internal/config/errors.go` and mapped to exit code 4 — verified, no changes needed
- Wired full `cmd/run.go` orchestration: config parse -> build-if-needed -> secret validation -> env assembly -> docker run
- Extracted `ensureBuild()` shared helper in `cmd/build_helper.go` to DRY the build pipeline between `cmd/build.go` and `cmd/run.go`
- Refactored `cmd/build.go` to use `ensureBuild()` — no behavior change, same output messages preserved
- `RunOptions.Mounts` field included (empty for now) for future story compatibility (2-1, 6-1, 7-1, 8-1)
- 77 tests passing (up from 46), `go vet` clean, `go build` clean
- 7 new test functions in `internal/docker/run_test.go` covering command assembly, env vars, mounts, empty values, full options
- 1 new test case added to `cmd/root_test.go` for `RunError` exit code mapping

### Review Findings

- [x] [Review][Patch] Signal exits (Ctrl+C) reported as error — suppress exit codes 130/143, return nil [internal/docker/run.go:46-49]
- [x] [Review][Patch] `cfg.Env` can overwrite HOST_UID/HOST_GID and secrets — reordered priority, extracted buildEnvVars() [cmd/run.go]
- [x] [Review][Patch] Missing secret validation unit tests per Task 5.3 — added 5 tests in cmd/root_test.go
- [x] [Review][Defer] `-it` flag fails in non-TTY contexts (CI/piped input) — deferred, out of scope for story 1-7
- [x] [Review][Defer] Unsanitized explicit `project_name` breaks Docker tags — deferred, pre-existing config validation gap
- [x] [Review][Defer] Container name collision on concurrent runs — deferred, single-instance by design

### Change Log

- 2026-04-09: Implemented story 1-7 — sandbox run with TTY and lifecycle

### File List

- `internal/docker/run.go` (new) — `RunOptions`, `runCmdArgs()`, `RunContainer()`
- `internal/docker/run_test.go` (new) — 7 unit tests for run command assembly
- `internal/docker/errors.go` (modified) — added `RunError` type
- `cmd/run.go` (modified) — full orchestration pipeline replacing stub
- `cmd/build_helper.go` (new) — shared `ensureBuild()` helper
- `cmd/build.go` (modified) — refactored to use `ensureBuild()`
- `cmd/root.go` (modified) — added `RunError` to `exitCode()` mapping
- `cmd/root_test.go` (modified) — added `RunError` test case
