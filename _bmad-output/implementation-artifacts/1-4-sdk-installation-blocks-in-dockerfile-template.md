# Story 1.4: SDK Installation Blocks in Dockerfile Template

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want conditional SDK installation blocks in the Dockerfile template,
so that my sandbox image includes only the SDKs I specified in my config.

## Acceptance Criteria

1. **Given** a config with `sdks: {nodejs: "22"}`
   **When** the template renders
   **Then** the Node.js 22 installation block is included and the Go/Python blocks are excluded

2. **Given** a config with `sdks: {nodejs: "22", python: "3.12"}`
   **When** the template renders
   **Then** both Node.js 22 and Python 3.12 blocks are included

3. **Given** a config with no SDKs specified
   **When** the template renders
   **Then** all SDK conditional blocks are excluded with no blank lines remaining

4. **Given** a config with `packages: [build-essential, libpq-dev]`
   **When** the template renders
   **Then** the additional packages are installed via `apt-get install` using `{{range .Packages}}`

5. **Given** SDK versions are passed as build arguments
   **When** the Dockerfile is built
   **Then** each SDK version is available as a `--build-arg` (FR40)

## Tasks / Subtasks

- [x] Task 1: Add SDK conditional blocks to `embed/Dockerfile.tmpl` (AC: #1, #2, #3)
  - [x] Replace the `{{- /* SDK installation blocks will be added by Story 1.4 */ -}}` placeholder comment
  - [x] Add Node.js conditional block using NodeSource setup script:
    ```
    {{- if .SDKs.NodeJS}}

    {{/* Node.js SDK */}}
    ARG NODE_VERSION={{.SDKs.NodeJS}}
    RUN curl -fsSL https://deb.nodesource.com/setup_${NODE_VERSION}.x | bash - && \
        apt-get install -y nodejs && \
        rm -rf /var/lib/apt/lists/*
    {{- end}}
    ```
  - [x] Add Go conditional block using official Go tarball:
    ```
    {{- if .SDKs.Go}}

    {{/* Go SDK */}}
    ARG GO_VERSION={{.SDKs.Go}}
    RUN curl -fsSL https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz | tar -C /usr/local -xz
    ENV PATH="/usr/local/go/bin:${PATH}"
    {{- end}}
    ```
  - [x] Add Python conditional block using deadsnakes PPA:
    ```
    {{- if .SDKs.Python}}

    {{/* Python SDK */}}
    ARG PYTHON_VERSION={{.SDKs.Python}}
    RUN apt-get update && \
        apt-get install -y software-properties-common && \
        add-apt-repository -y ppa:deadsnakes/ppa && \
        apt-get update && \
        apt-get install -y python${PYTHON_VERSION} python${PYTHON_VERSION}-venv python${PYTHON_VERSION}-dev && \
        update-alternatives --install /usr/bin/python3 python3 /usr/bin/python${PYTHON_VERSION} 1 && \
        update-alternatives --install /usr/bin/python python /usr/bin/python${PYTHON_VERSION} 1 && \
        rm -rf /var/lib/apt/lists/*
    {{- end}}
    ```
  - [x] Use `{{-` and `-}}` trim markers on all conditional blocks to prevent blank lines when SDKs are absent

- [x] Task 2: Add additional packages block to `embed/Dockerfile.tmpl` (AC: #4)
  - [x] Add packages block after SDK blocks (before ENV section):
    ```
    {{- if .Packages}}

    {{/* Additional packages */}}
    RUN apt-get update && apt-get install -y \
    {{- range $i, $pkg := .Packages}}
        {{$pkg}} \
    {{- end}}
        && rm -rf /var/lib/apt/lists/*
    {{- end}}
    ```

- [x] Task 3: Add `BuildArgs()` function to `internal/docker/build.go` (AC: #5)
  - [x] Create file `internal/docker/build.go`
  - [x] Signature: `func BuildArgs(cfg *config.Config) []string`
  - [x] Return slice of `--build-arg` flags for each non-empty SDK version:
    - If `cfg.SDKs.NodeJS != ""` → append `"--build-arg"`, `"NODE_VERSION=<value>"`
    - If `cfg.SDKs.Go != ""` → append `"--build-arg"`, `"GO_VERSION=<value>"`
    - If `cfg.SDKs.Python != ""` → append `"--build-arg"`, `"PYTHON_VERSION=<value>"`
  - [x] Return empty slice if no SDKs configured

- [x] Task 4: Add tests to `internal/template/render_test.go` (AC: #1, #2, #3, #4)
  - [x] `TestRender_nodejsOnly` — config with `SDKs: config.SDKConfig{NodeJS: "22"}`, verify output contains NodeSource setup and `nodejs` install, does NOT contain Go or Python blocks
  - [x] `TestRender_pythonOnly` — config with `SDKs: config.SDKConfig{Python: "3.12"}`, verify deadsnakes PPA and python3.12 install, no Node.js or Go blocks
  - [x] `TestRender_goOnly` — config with `SDKs: config.SDKConfig{Go: "1.23"}`, verify go tarball download and PATH set, no Node.js or Python blocks
  - [x] `TestRender_multipleSDKs` — config with `SDKs: config.SDKConfig{NodeJS: "22", Python: "3.12"}`, verify both blocks present
  - [x] `TestRender_allSDKs` — config with all three SDKs set, verify all blocks present
  - [x] `TestRender_noSDKs` — config with empty SDKs (already covered by `TestRender_minimalConfig`), verify no SDK-specific blocks appear (no `nodesource`, no `go.dev`, no `deadsnakes`)
  - [x] `TestRender_additionalPackages` — config with `Packages: []string{"libpq-dev", "redis-tools"}`, verify `apt-get install` with both packages present
  - [x] `TestRender_noPackages` — config with empty Packages, verify no additional apt-get install beyond the base packages layer
  - [x] `TestRender_noBlankLinesWithoutSDKs` — config with no SDKs, verify no consecutive blank lines in output between the scripts COPY block and ENV/ENTRYPOINT section

- [x] Task 5: Create `internal/docker/build_test.go` (AC: #5)
  - [x] `TestBuildArgs_nodejsOnly` — verify `["--build-arg", "NODE_VERSION=22"]`
  - [x] `TestBuildArgs_allSDKs` — verify all three `--build-arg` flags present
  - [x] `TestBuildArgs_noSDKs` — verify empty slice returned
  - [x] `TestBuildArgs_partialSDKs` — verify only non-empty SDKs produce flags

- [x] Task 6: Verify build and tests (AC: all)
  - [x] Run `go vet ./...`
  - [x] Run `go test ./...`
  - [x] Run `CGO_ENABLED=0 go build -o asbox .`

### Review Findings

- [x] [Review][Defer] SDK version strings not validated — potential shell injection in Dockerfile ARG/RUN [internal/config/parse.go] — deferred, pre-existing. `Parse()` performs no format validation on SDK version strings; malicious values could inject shell commands via template rendering. Fix belongs in config parser, not template.
- [x] [Review][Defer] Package names not validated — injection risk in apt-get install [internal/config/parse.go] — deferred, pre-existing. `Parse()` does not validate package name format; values with shell metacharacters could inject commands via the Packages template block.
- [x] [Review][Defer] Empty string in Packages slice produces invalid Dockerfile syntax [internal/config/parse.go] — deferred, pre-existing. `Packages: ["", "vim"]` would render a bare backslash continuation line. Config parser should reject empty package names.

## Dev Notes

### Architecture Compliance

- **`embed/Dockerfile.tmpl`**: Extend the existing template from Story 1.3. SDK blocks go where the `{{- /* SDK installation blocks will be added by Story 1.4 */ -}}` placeholder is. Keep the `{{- /* Container scripts and tooling will be added by Story 1.5 */ -}}` placeholder intact.
- **`internal/docker/build.go`**: New file. `BuildArgs()` is a pure function that assembles `--build-arg` flags from config. It does NOT execute `docker build` — that's a later story (1.6). This story only creates the flag assembly logic.
- **`internal/template/render.go`**: NO changes needed. The `Render()` function already reads from `embed.Assets` and executes the template with config as context. Adding blocks to `Dockerfile.tmpl` is sufficient.
- **`internal/template/render_test.go`**: Add new test functions. Do NOT modify existing tests.
- **Error type**: Use existing `DependencyError` from `internal/docker/errors.go` if error handling is needed in `build.go`. However, `BuildArgs()` is a pure function that should not error — it just reads config fields.
- **Dependency direction**: `internal/docker/` imports `internal/config/` for the `Config` type. Does NOT import `internal/template/` or `cmd/`.

### SDK Installation Approaches

**Node.js**: Use NodeSource setup script (`https://deb.nodesource.com/setup_<major>.x`). This is the standard Dockerfile approach for Node.js on Ubuntu. The `setup_` script adds the NodeSource APT repository and then `apt-get install nodejs` installs the specified major version. The ARG `NODE_VERSION` should be the major version number (e.g., `22`, not `22.1.0`).

**Go**: Use the official Go binary tarball from `https://go.dev/dl/go<version>.linux-amd64.tar.gz`. Extract to `/usr/local` and add `/usr/local/go/bin` to PATH via ENV. The ARG `GO_VERSION` is the full semver (e.g., `1.23.0`).

**Python**: Use the deadsnakes PPA (`ppa:deadsnakes/ppa`) for Ubuntu. Install `python<version>`, `python<version>-venv`, and `python<version>-dev`. Set up `update-alternatives` so `python3` and `python` point to the installed version. The ARG `PYTHON_VERSION` is major.minor (e.g., `3.12`).

### Template Ordering in Dockerfile.tmpl

The final template structure after this story:
```
FROM ubuntu:24.04@sha256:<digest>          (Story 1.3 - unchanged)
ARG DEBIAN_FRONTEND=noninteractive         (Story 1.3 - unchanged)
RUN apt-get install base packages          (Story 1.3 - unchanged)
RUN groupadd/useradd sandbox               (Story 1.3 - unchanged)
COPY scripts                               (Story 1.3 - unchanged)
RUN chmod +x scripts                       (Story 1.3 - unchanged)
{{if .SDKs.NodeJS}} Node.js block {{end}}  (Story 1.4 - NEW)
{{if .SDKs.Go}} Go block {{end}}           (Story 1.4 - NEW)
{{if .SDKs.Python}} Python block {{end}}   (Story 1.4 - NEW)
{{if .Packages}} additional packages {{end}} (Story 1.4 - NEW)
{{/* Container scripts placeholder */}}    (Story 1.5 - future)
ENV directives                             (Story 1.3 - unchanged)
ENTRYPOINT / USER / WORKDIR                (Story 1.3 - unchanged)
```

### Whitespace Control

This is critical. When no SDKs are configured, the conditional blocks must produce NO output — no blank lines, no whitespace. Use `{{-` at the start and `-}}` at the end of each conditional block outer boundary. Test this explicitly (Task 4: `TestRender_noBlankLinesWithoutSDKs`).

### Build Args vs Template Values

The template uses `ARG` directives with default values from config (`ARG NODE_VERSION={{.SDKs.NodeJS}}`). This means the rendered Dockerfile has the version baked in as the ARG default. The `BuildArgs()` function in `internal/docker/build.go` produces `--build-arg` flags that override these defaults at build time. Both mechanisms exist for correctness — the ARG default makes the Dockerfile self-documenting, the `--build-arg` ensures the builder explicitly passes versions. In practice, the values are identical (both from config).

### Previous Story Intelligence (Story 1.3)

Key patterns established in Story 1.3 to follow:
- **Template comments**: Use `{{/* comment */}}` style, NOT `{{- /* comment */ -}}` with extra spaces before closing (that was a bug in 1.3 initial implementation)
- **Whitespace trim markers**: `{{-` trims preceding whitespace, `-}}` trims following. Use at block boundaries.
- **ENV quoting**: ENV values use double quotes — `ENV {{$k}}="{{$v}}"` — this was a review fix in 1.3
- **Test patterns**: Use `strings.Contains()` for output assertions. Construct test configs directly with `&config.Config{...}`, do NOT parse YAML.
- **Import alias**: The embed package uses `asboxEmbed "github.com/mcastellin/asbox/embed"` to avoid collision with Go's standard `embed` package. Not needed for `internal/docker/build.go` since it doesn't import the embed package.
- **Error type contract**: `Render()` returns `*TemplateError` on failure. `BuildArgs()` should not fail — it's a pure mapping function.

### Git Intelligence

Recent commits:
- `b8709e7` — Story 1-3: Base Dockerfile template with review fixes (latest)
- `28905c7` — Story 1-2: configuration parsing and validation
- `54871d8` — Story 1-1: Go project scaffold and CLI skeleton

Story 1.3 review findings relevant to this story:
- ENV values must be quoted (already fixed in template)
- Tests should be thorough — check all expected content, not just sampling
- Template error handling is adequate via `*TemplateError`

### Key Anti-Patterns to Avoid

- Do NOT validate SDK version formats in `Render()` or `BuildArgs()` — config is already validated by `Parse()`. Trust the input.
- Do NOT create `internal/utils/` or helper packages
- Do NOT add color codes, spinners, or progress bars
- Do NOT use `os.Exit()` in `internal/docker/`
- Do NOT scatter `//go:embed` directives — they're centralized in `embed/embed.go`
- Do NOT use raw string literals in the template — all values from config structs
- Do NOT install SDKs from source compilation — use binary distributions or package managers
- Do NOT add `pip` installation in the Python block — just the interpreter. pip comes with python-venv.
- Do NOT modify existing tests in `render_test.go` — add new test functions only

### Go Code Conventions

- **Formatting**: `gofmt` is law, `go vet` must pass
- **File naming**: `snake_case.go` — `build.go`, `build_test.go`
- **Test naming**: `TestBuildArgs_scenario` for build.go tests, `TestRender_scenario` for template tests
- **Variable naming**: `camelCase` — `buildArgs`, `nodeVersion`
- **No `interface{}` or `any` as function parameters** — use `*config.Config`

### Project Structure Notes

Files created/modified by this story:
```
embed/Dockerfile.tmpl              (modified) — Add SDK conditional blocks and packages block
internal/docker/build.go           (new) — BuildArgs() function for --build-arg flag assembly
internal/docker/build_test.go      (new) — Tests for BuildArgs()
internal/template/render_test.go   (modified) — Add SDK and packages rendering tests
```

Existing files NOT modified:
- `internal/template/render.go` — No changes needed, template rendering works as-is
- `internal/template/errors.go` — TemplateError already exists, reuse as-is
- `internal/config/*` — No changes, Config struct and SDKConfig already have NodeJS/Go/Python fields
- `embed/embed.go` — Already embeds Dockerfile.tmpl, no changes needed
- `internal/docker/errors.go` — DependencyError exists, reuse if needed

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Story 1.4: SDK Installation Blocks in Dockerfile Template]
- [Source: _bmad-output/planning-artifacts/architecture.md — Dockerfile Generation decision, `{{if .SDKs.NodeJS}}` pattern]
- [Source: _bmad-output/planning-artifacts/architecture.md — Go Template Conventions]
- [Source: _bmad-output/planning-artifacts/architecture.md — Go Project Organization — internal/docker/]
- [Source: _bmad-output/planning-artifacts/architecture.md — FR40 (SDK build args) mapped to internal/docker/build.go]
- [Source: _bmad-output/planning-artifacts/architecture.md — Implementation Patterns — Go Code Conventions]
- [Source: _bmad-output/planning-artifacts/architecture.md — Anti-Patterns section]
- [Source: _bmad-output/planning-artifacts/prd.md — FR1 (SDK versions), FR40 (build args)]
- [Source: _bmad-output/implementation-artifacts/1-3-base-dockerfile-template.md — Template structure, review findings, import patterns]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- Fixed `TestRender_pythonOnly`: template uses `${PYTHON_VERSION}` shell var, not literal version — updated assertion to match template output
- Fixed `TestRender_noBlankLinesWithoutSDKs`: existing Story 1.3 template has blank lines from `{{/* comment */}}` blocks; narrowed test scope to SDK region between scripts and ENTRYPOINT

### Completion Notes List

- Replaced Story 1.4 placeholder in `embed/Dockerfile.tmpl` with conditional SDK blocks for Node.js (NodeSource), Go (official tarball), and Python (deadsnakes PPA)
- Added conditional additional packages block using `{{range .Packages}}`
- All SDK blocks use `{{-` / `-}}` trim markers for clean whitespace when absent
- Created `internal/docker/build.go` with `BuildArgs()` pure function that maps SDK config to `--build-arg` flags
- Added 9 new render tests covering all SDK combinations, packages, and whitespace behavior
- Added 4 new BuildArgs tests covering all SDK combinations
- All existing tests pass with no regressions
- `go vet`, `go test ./...`, and `go build` all pass clean

### Change Log

- 2026-04-08: Implemented Story 1.4 — SDK conditional blocks and BuildArgs function

### File List

- `embed/Dockerfile.tmpl` (modified) — Added Node.js, Go, Python conditional SDK blocks and additional packages block
- `internal/docker/build.go` (new) — `BuildArgs()` function for `--build-arg` flag assembly from config
- `internal/docker/build_test.go` (new) — 4 tests for `BuildArgs()`
- `internal/template/render_test.go` (modified) — 9 new tests for SDK and packages rendering
