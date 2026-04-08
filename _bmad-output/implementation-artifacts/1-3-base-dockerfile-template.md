# Story 1.3: Base Dockerfile Template

Status: done

## Story

As a developer,
I want a base Dockerfile template that sets up Ubuntu, Tini, non-root user, and common packages,
so that the sandbox has a solid foundation before SDK-specific customization.

## Acceptance Criteria

1. **Given** the embedded Dockerfile template
   **When** rendered with any valid config
   **Then** the output starts with `FROM ubuntu:24.04@sha256:<pinned-digest>` (FR39)

2. **Given** the template renders the base section
   **When** inspecting the output
   **Then** it installs Tini as PID 1, creates a non-root sandbox user, and installs common packages (curl, wget, dig, git, jq, etc.)

3. **Given** the template renders the entrypoint section
   **When** inspecting the output
   **Then** tini is configured as the ENTRYPOINT with entrypoint.sh as the argument

4. **Given** non-secret environment variables are defined in config
   **When** the template renders
   **Then** they are set via `ENV` directives in the Dockerfile

## Tasks / Subtasks

- [x] Task 1: Create `internal/template/render.go` with `Render()` function (AC: #1, #2, #3, #4)
  - [x] Signature: `func Render(cfg *config.Config) (string, error)`
  - [x] Read `Dockerfile.tmpl` from `embed.Assets` via `embed.Assets.ReadFile("Dockerfile.tmpl")`
  - [x] Parse template using `text/template.New("Dockerfile").Parse(string(tmplBytes))`
  - [x] Execute template into a `bytes.Buffer` with `cfg` as the data context
  - [x] On template parse/execute failure, return `&TemplateError{Msg: "failed to render Dockerfile: <detail>"}`
  - [x] Return rendered string

- [x] Task 2: Replace placeholder `embed/Dockerfile.tmpl` with full base template (AC: #1, #2, #3, #4)
  - [x] Line 1: `FROM ubuntu:24.04@sha256:<pinned-digest>` — look up the current Ubuntu 24.04 digest for `linux/amd64` and pin it
  - [x] `ARG DEBIAN_FRONTEND=noninteractive` to suppress apt prompts
  - [x] Install Tini via `apt-get install -y tini`
  - [x] Install common packages in a single `apt-get install -y` layer:
    - `curl`, `wget`, `dnsutils` (provides `dig`), `git`, `jq`, `unzip`, `zip`, `less`, `vim`, `ca-certificates`, `gnupg`, `lsb-release`, `sudo`, `build-essential`
  - [x] Clean up apt cache: `rm -rf /var/lib/apt/lists/*`
  - [x] Create non-root sandbox user:
    ```
    RUN groupadd -g 1000 sandbox && \
        useradd -m -u 1000 -g sandbox -s /bin/bash sandbox && \
        echo "sandbox ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers.d/sandbox
    ```
  - [x] COPY embedded scripts into image:
    ```
    COPY entrypoint.sh /usr/local/bin/entrypoint.sh
    COPY git-wrapper.sh /usr/local/bin/git
    COPY healthcheck-poller.sh /usr/local/bin/healthcheck-poller.sh
    RUN chmod +x /usr/local/bin/entrypoint.sh /usr/local/bin/git /usr/local/bin/healthcheck-poller.sh
    ```
  - [x] Set non-secret environment variables using `{{range $k, $v := .Env}}ENV {{$k}}={{$v}}{{end}}` with whitespace control
  - [x] Set ENTRYPOINT and default user:
    ```
    ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/entrypoint.sh"]
    USER sandbox
    WORKDIR /workspace
    ```
  - [x] Use `{{- ` and ` -}}` trim markers to prevent blank lines from conditional blocks

- [x] Task 3: Create `internal/template/render_test.go` (AC: #1, #2, #3, #4)
  - [x] `TestRender_baseImage` — verify output starts with `FROM ubuntu:24.04@sha256:`
  - [x] `TestRender_tiniInstalled` — verify output contains `apt-get install` with `tini`
  - [x] `TestRender_sandboxUserCreated` — verify output contains `useradd` for sandbox user
  - [x] `TestRender_commonPackages` — verify curl, wget, dnsutils, git, jq present in apt-get install
  - [x] `TestRender_entrypoint` — verify `ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/entrypoint.sh"]`
  - [x] `TestRender_envVars` — config with `Env: {"MY_VAR": "value"}`, verify `ENV MY_VAR=value` in output
  - [x] `TestRender_noEnvVars` — config with empty Env map, verify no `ENV` directives for user vars
  - [x] `TestRender_copyScripts` — verify COPY directives for entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh
  - [x] `TestRender_minimalConfig` — config with only `agent: "claude-code"` and no SDKs/packages, verify valid Dockerfile output with no SDK blocks
  - [x] Use `strings.Contains()` for output assertions
  - [x] Create test configs using `&config.Config{Agent: "claude-code", ...}` — do NOT parse YAML in tests, construct structs directly

- [x] Task 4: Verify build and tests (AC: all)
  - [x] Run `go vet ./...`
  - [x] Run `go test ./...`
  - [x] Run `CGO_ENABLED=0 go build -o asbox .`

## Dev Notes

### Architecture Compliance

- **`internal/template/render.go`**: Single `Render(cfg *config.Config) (string, error)` function. Assumes config is already validated by `config.Parse()`. Does NOT validate config fields — that is `internal/config/`'s responsibility. Returns rendered Dockerfile string or `*TemplateError`.
- **`embed/Dockerfile.tmpl`**: Go `text/template` syntax. Uses `{{if}}`, `{{range}}`, `{{end}}`, and whitespace control via `{{-`/`-}}`. Values come from `config.Config` struct fields.
- **`embed/embed.go`**: Already exports `Assets embed.FS` with `Dockerfile.tmpl` included. No changes needed to embed.go.
- **Error type**: Use existing `TemplateError{Msg}` from `internal/template/errors.go`. Do NOT create new error types.
- **Dependency direction**: `internal/template/` imports `internal/config/` (for `Config` type) and `embed` package (for `Assets`). It does NOT import `cmd/` or other `internal/` packages.

### Template Structure (Story 1.3 Scope)

Story 1.3 creates the **base layer** of the Dockerfile template. Stories 1.4 and 1.5 will extend it with SDK blocks and tooling. The template should be structured with clear section comments so subsequent stories can add blocks:

```dockerfile
{{/* Base image - pinned to digest for reproducibility */}}
FROM ubuntu:24.04@sha256:<digest>

{{/* Base packages and Tini */}}
ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y \
    tini curl wget dnsutils git jq unzip zip less vim \
    ca-certificates gnupg lsb-release sudo build-essential \
    && rm -rf /var/lib/apt/lists/*

{{/* Non-root sandbox user */}}
RUN groupadd -g 1000 sandbox && \
    useradd -m -u 1000 -g sandbox -s /bin/bash sandbox && \
    echo "sandbox ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers.d/sandbox

{{/* Embedded scripts */}}
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
COPY git-wrapper.sh /usr/local/bin/git
COPY healthcheck-poller.sh /usr/local/bin/healthcheck-poller.sh
RUN chmod +x /usr/local/bin/entrypoint.sh /usr/local/bin/git /usr/local/bin/healthcheck-poller.sh

{{/* SDK installation blocks will be added by Story 1.4 */}}

{{/* Container scripts and tooling will be added by Story 1.5 */}}

{{/* Non-secret environment variables */}}
{{- range $k, $v := .Env}}
ENV {{$k}}={{$v}}
{{- end}}

{{/* Entrypoint and default user */}}
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/entrypoint.sh"]
USER sandbox
WORKDIR /workspace
```

### COPY Directives and Docker Build Context

The `Render()` function produces a Dockerfile that uses `COPY` for embedded scripts. At build time (Story 1.6), the Go binary will extract these files from `embed.Assets` into a temporary build context directory before calling `docker build`. The template assumes scripts are at the build context root — `COPY entrypoint.sh ...` not `COPY embed/entrypoint.sh ...`.

This story does NOT implement the build context extraction — just the template COPY directives. Story 1.6 (`docker.BuildImage()`) will handle writing files to a temp dir and passing it as the Docker build context.

### Ubuntu 24.04 Base Image Digest

Pin the digest for `ubuntu:24.04` on `linux/amd64`. To get the current digest:
```bash
docker manifest inspect ubuntu:24.04 | jq -r '.manifests[] | select(.platform.architecture=="amd64" and .platform.os=="linux") | .digest'
```
Use the digest returned. The exact value will change with Ubuntu updates — pin whatever is current at implementation time.

### Go `text/template` Usage

- Parse template string, not file path: `template.New("Dockerfile").Parse(string(tmplData))`
- Execute into `bytes.Buffer`: `tmpl.Execute(&buf, cfg)`
- Template receives `*config.Config` as dot context — access fields via `.Agent`, `.SDKs.NodeJS`, `.Env`, `.Packages`, etc.
- Whitespace control: `{{-` trims preceding whitespace, `-}}` trims following whitespace. Use judiciously to avoid blank lines in rendered Dockerfile.
- Map range: `{{range $k, $v := .Env}}` iterates map entries. Note: Go maps have non-deterministic iteration order — ENV directive order may vary between renders. This is acceptable for Dockerfiles.

### Previous Story Intelligence (Story 1.2)

Key patterns from Story 1.2 to follow:
- **Error types**: `TemplateError{Msg}` already exists in `internal/template/errors.go`. Reuse it.
- **Test patterns**: Table-driven tests, `TestFunctionName_scenario` naming. Use `errors.As()` for error type assertions.
- **Config struct**: `config.Config` is the validated struct. Construct test configs directly (`&config.Config{...}`), don't parse YAML in template tests.
- **No `os.Exit()` in internal packages**: Return `*TemplateError`, let `cmd/` handle exit codes.
- **`embed.Assets`**: Already exported from `embed/embed.go`. Import as `github.com/mcastellin/asbox/embed` — note the `embed` package name may collide with Go's `embed` standard library. Use an import alias if needed: `asboxEmbed "github.com/mcastellin/asbox/embed"`.

### Import Alias for Embed Package

The project's `embed/embed.go` package is named `embed`, which collides with Go's standard library `embed` package. When importing in `render.go`, use an alias:
```go
import (
    asboxEmbed "github.com/mcastellin/asbox/embed"
)
```
Then access via `asboxEmbed.Assets.ReadFile("Dockerfile.tmpl")`.

### Git Intelligence

Recent commits show:
- `28905c7` — Story 1-2: configuration parsing and validation (latest)
- `54871d8` — Story 1-1: Go project scaffold and CLI skeleton

Files created by Story 1.2: `internal/config/config.go`, `internal/config/parse.go`, `internal/config/parse_test.go`, modified `cmd/build.go` and `cmd/run.go`.

Story 1.2 review findings (carry-forward context):
- Mount targets validated as absolute paths
- Tilde expansion added for `~/` prefix in paths
- `sanitizeProjectName` falls back to `"asbox"` on empty result

### Key Anti-Patterns to Avoid

- Do NOT validate config fields in `Render()` — config is already validated by `Parse()`. Trust the input.
- Do NOT create `internal/utils/` or helper packages
- Do NOT add color codes, spinners, or progress bars
- Do NOT use `os.Exit()` in `internal/template/`
- Do NOT scatter `//go:embed` directives — they're centralized in `embed/embed.go`
- Do NOT use raw string literals in the template — all values from config structs
- Do NOT hardcode the Dockerfile content as a Go string — use the embedded template file
- Do NOT import `cmd/` or other `internal/` packages from `internal/template/` (except `config` for the type)

### Go Code Conventions

- **Formatting**: `gofmt` is law, `go vet` must pass
- **File naming**: `snake_case.go` — `render.go`, `render_test.go`
- **Test naming**: `TestRender_scenario` — `TestRender_baseImage`, `TestRender_envVars`
- **Variable naming**: `camelCase` — `tmplData`, `renderedOutput`
- **No `interface{}` or `any` as function parameters** — use `*config.Config`

### Project Structure Notes

Files created/modified by this story:
```
internal/template/render.go       (new) — Render() function reading template from embed.Assets
internal/template/render_test.go  (new) — Tests for base Dockerfile rendering
embed/Dockerfile.tmpl             (modified) — Replace placeholder with full base template
```

Existing files NOT modified:
- `embed/embed.go` — Already embeds Dockerfile.tmpl, no changes needed
- `internal/template/errors.go` — TemplateError already exists, reuse as-is
- `internal/config/*` — No changes, Config struct consumed as-is
- `cmd/build.go` — Not wired to template.Render() yet (Story 1.6 wires the full build pipeline)
- `embed/entrypoint.sh`, `embed/git-wrapper.sh`, `embed/healthcheck-poller.sh` — Still placeholder content (Story 1.5 implements full scripts)

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Story 1.3: Base Dockerfile Template]
- [Source: _bmad-output/planning-artifacts/architecture.md — Dockerfile Generation decision]
- [Source: _bmad-output/planning-artifacts/architecture.md — Go Template Conventions]
- [Source: _bmad-output/planning-artifacts/architecture.md — Go Project Organization — internal/template/]
- [Source: _bmad-output/planning-artifacts/architecture.md — Implementation Patterns — Go Code Conventions]
- [Source: _bmad-output/planning-artifacts/architecture.md — Anti-Patterns section]
- [Source: _bmad-output/planning-artifacts/architecture.md — Project Structure — embed/ directory]
- [Source: _bmad-output/planning-artifacts/architecture.md — Container Lifecycle — Entrypoint Startup Sequence]
- [Source: _bmad-output/planning-artifacts/prd.md — FR38 (Dockerfile from template), FR39 (pinned digest), FR48 (Tini), FR49 (UID/GID)]
- [Source: _bmad-output/implementation-artifacts/1-2-configuration-parsing-and-validation.md — Dev Notes, Review Findings, embed alias pattern]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- Initial template had Go template comment syntax issue (`*/ }}` needs to be `*/}}`). Fixed by using standard `{{/* ... */}}` comment delimiters without extra spaces before closing `}}`.

### Completion Notes List

- Task 1: Created `internal/template/render.go` with `Render(cfg *config.Config) (string, error)` function. Reads embedded `Dockerfile.tmpl` via `asboxEmbed.Assets.ReadFile()`, parses with `text/template`, executes with config as context. Returns `*TemplateError` on failure.
- Task 2: Replaced placeholder `embed/Dockerfile.tmpl` with full base template. Pinned Ubuntu 24.04 digest (`sha256:e21f810fa78c09944446ec02048605eb3ab1e4e2e261c387ecc7456b38400d79`), installs tini + common packages, creates sandbox user (UID/GID 1000), COPYs embedded scripts, renders ENV vars from config map, sets tini as ENTRYPOINT with sandbox user.
- Task 3: Created `internal/template/render_test.go` with 10 tests covering all ACs: base image pinning, tini installation, sandbox user creation, common packages, entrypoint directive, env vars (present and absent), COPY script directives, minimal config rendering, and error type validation.
- Task 4: All verification passed — `go vet ./...` clean, `go test ./...` all pass (6 packages), `CGO_ENABLED=0 go build` succeeds.

### Review Findings

- [x] [Review][Patch] ENV values must be quoted to handle spaces correctly [embed/Dockerfile.tmpl:28]
- [x] [Review][Patch] `TestRender_commonPackages` only checks 5 of 15 required packages [internal/template/render_test.go:57]
- [x] [Review][Patch] `TestRender_sandboxUserCreated` doesn't verify UID/GID 1000 [internal/template/render_test.go:42]
- [x] [Review][Patch] `TestRender_templateError` is a no-op — renamed to `TestRender_errorType`, clarified comment [internal/template/render_test.go:141]
- [x] [Review][Patch] `TestRender_copyScripts` doesn't verify `chmod +x` directive [internal/template/render_test.go:109]
- [x] [Review][Defer] ENV key format validation missing in `config.Parse` — deferred, pre-existing
- [x] [Review][Defer] ENV value newline injection not blocked by `config.Parse` — deferred, pre-existing

### Change Log

- 2026-04-08: Implemented Story 1.3 — Base Dockerfile Template. Created render.go, updated Dockerfile.tmpl with full base template, added comprehensive test suite (10 tests). All ACs satisfied.

### File List

- `internal/template/render.go` (new) — Render() function for Dockerfile template
- `internal/template/render_test.go` (new) — 10 tests covering all acceptance criteria
- `embed/Dockerfile.tmpl` (modified) — Full base Dockerfile template replacing placeholder
