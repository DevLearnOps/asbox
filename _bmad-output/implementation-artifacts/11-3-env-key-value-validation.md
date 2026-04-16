# Story 11.3: ENV Key/Value Validation

Status: done

## Story

As a developer,
I want asbox to validate environment variable keys and values in my config,
so that malformed ENV entries cannot inject arbitrary Dockerfile directives.

## Acceptance Criteria

1. **Given** a config.yaml with an ENV key containing spaces (e.g., `env: {"MY VAR": "value"}`)
   **When** asbox parses the config
   **Then** the CLI exits with code 1 and prints an error identifying the invalid ENV key and the allowed format

2. **Given** a config.yaml with an ENV key starting with a digit (e.g., `env: {"1VAR": "value"}`)
   **When** asbox parses the config
   **Then** the CLI exits with code 1 and prints an error -- ENV keys must start with a letter or underscore

3. **Given** a config.yaml with valid ENV keys (e.g., `env: {"MY_VAR": "value", "DEBUG": "true", "_INTERNAL": "1"}`)
   **When** asbox parses the config
   **Then** all keys are accepted -- allowed format: `^[a-zA-Z_][a-zA-Z0-9_]*$`

4. **Given** a config.yaml with an ENV value containing a newline (via YAML multiline string)
   **When** asbox parses the config
   **Then** the CLI exits with code 1 and prints an error -- newlines in ENV values could inject Dockerfile directives

5. **Given** a config.yaml with normal ENV values including quotes, equals signs, and spaces (e.g., `env: {"DSN": "host=localhost dbname=test"}`)
   **When** asbox parses the config
   **Then** the values are accepted -- only newlines and null bytes are rejected in values

## Tasks / Subtasks

- [x] Task 1: Add `validateEnvKey()` function (AC: #1, #2, #3)
  - [x] 1.1 Create regex `^[a-zA-Z_][a-zA-Z0-9_]*$` as package-level `var` alongside existing `sdkVersionRe` and `packageNameRe`
  - [x] 1.2 Return `*ConfigError` with field `env.<key>` and message including the bad key and allowed format
  - [x] 1.3 Call from `Parse()` in a loop over `cfg.Env` keys
- [x] Task 2: Add `validateEnvValue()` function (AC: #4, #5)
  - [x] 2.1 Check for `\n`, `\r`, and `\0` bytes in value string
  - [x] 2.2 Return `*ConfigError` with field `env.<key>` and message explaining newline/null byte rejection
  - [x] 2.3 Call from `Parse()` in same loop as key validation
- [x] Task 3: Add unit tests in `internal/config/parse_test.go` (AC: all)
  - [x] 3.1 Table-driven test for `validateEnvKey` (valid keys, spaces, leading digits, special chars)
  - [x] 3.2 Table-driven test for `validateEnvValue` (normal values, newlines, carriage returns, null bytes)
  - [x] 3.3 Verify existing `TestParse_validFullConfig` still passes (it has `env: {FOO: bar, BAZ: qux}`)

## Dev Notes

### Implementation Location

All changes in `internal/config/parse.go` and `internal/config/parse_test.go`. No new files, no new error types, no new exit codes.

### Validation Functions

**`validateEnvKey(key string) error`**

Add after the existing `validatePackageName` function (line 264 of `internal/config/parse.go`):

- Regex: `^[a-zA-Z_][a-zA-Z0-9_]*$` -- compile as package-level `var` like existing `sdkVersionRe` (line 23) and `packageNameRe` (line 24)
- Return `&ConfigError{Field: "env." + key, Msg: ...}` on failure
- Error message format: `"invalid environment variable key %q. Keys must match shell variable format: start with letter or underscore, followed by letters, digits, or underscores"` including the bad key
- Empty key check: `if key == ""` return error with message `"empty environment variable key is not allowed"`

**`validateEnvValue(key, value string) error`**

- Check for `\n` (newline), `\r` (carriage return), and `\0` (null byte) using `strings.ContainsAny(value, "\n\r\x00")`
- Return `&ConfigError{Field: "env." + key, Msg: ...}` on failure
- Error message: `"value contains newline or null byte characters which could inject Dockerfile directives. Remove newlines from the value"` -- DO NOT include the raw value in the message (it may contain control characters)
- All other characters (quotes, spaces, equals signs, backslashes, unicode) are allowed in values

**Call site in `Parse()`:**

Insert after package name validation (line 141) and before mount validation (line 143). Iterate `cfg.Env` with sorted keys for deterministic error reporting:

```go
// Validate env keys and values
envKeys := make([]string, 0, len(cfg.Env))
for k := range cfg.Env {
    envKeys = append(envKeys, k)
}
sort.Strings(envKeys)
for _, k := range envKeys {
    if err := validateEnvKey(k); err != nil {
        return nil, err
    }
    if err := validateEnvValue(k, cfg.Env[k]); err != nil {
        return nil, err
    }
}
```

Note: `sort` is already imported in `parse.go` (line 9). No new imports needed.

### Regex Placement

Add as package-level compiled regexp alongside existing ones at lines 21-24:

```go
var envKeyRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
```

No regex needed for value validation -- use `strings.ContainsAny` for `\n\r\0` check instead.

### Injection Points Protected

ENV values flow to two surfaces:

1. **Dockerfile `ENV` directive** (build-time, the PRIMARY risk):
   - `embed/Dockerfile.tmpl` line 131: `ENV {{$k}}="{{$v}}"`
   - Keys injected unquoted before `=` -- a key with spaces or special chars produces invalid Dockerfile syntax or injects directives
   - Values are double-quoted but a newline escapes the quoting and injects a new Dockerfile line (e.g., `ENV SAFE="ok"\nRUN malicious` becomes two directives)
   - This is the injection vector that story 11-3 closes

2. **Docker `--env` flag** (run-time, lower risk):
   - `internal/docker/run.go` line 37: `args = append(args, "--env", key+"="+val)`
   - Passed via `os/exec` args array (not shell string) -- safe from shell injection by construction
   - But invalid keys would still produce incorrect container environment

### Validation Timing

All validation MUST happen in `Parse()` BEFORE any downstream consumer sees values. The architecture principle: template rendering (`internal/template/render.go`) assumes fully validated input.

### Error Pattern

Follow existing `ConfigError` pattern in `internal/config/errors.go`:
- No new error types needed -- reuse `ConfigError`
- No new exit codes needed -- config errors already map to exit 1 in `cmd/root.go`
- No changes needed to `cmd/root_test.go:TestExitCode_mapping` table
- Error messages MUST include: what failed, why, fix action

### Deterministic Iteration Order

ENV map (`map[string]string`) has non-deterministic iteration order in Go. Sort keys before validating to ensure consistent, reproducible error messages. This follows the pattern established in story 11-2 where SDK version validation uses a slice of structs (not map iteration) for stable error ordering.

### Project Structure Notes

- All changes in `internal/config/` -- no new packages, no cross-package changes
- No changes to `cmd/` layer, `embed/`, `internal/docker/`, `internal/template/`, or `internal/mount/`
- The Dockerfile template at `embed/Dockerfile.tmpl:131` already quotes values (`ENV {{$k}}="{{$v}}"`), but this alone is insufficient because newlines escape double quotes in Dockerfiles. Validation is the proper fix.
- No template changes needed -- validation in the parser prevents dangerous values from reaching the template

### Testing Strategy

Use table-driven tests following existing patterns in `internal/config/parse_test.go`.

**ENV Key Tests** -- new function `TestParse_envKeyValidation`:

| Key | Expected |
|-----|----------|
| `"MY_VAR"` | accept |
| `"DEBUG"` | accept |
| `"_INTERNAL"` | accept |
| `"a"` | accept (single letter) |
| `"_"` | accept (single underscore) |
| `"A1_B2"` | accept (mixed) |
| `"MY VAR"` | reject, ConfigError field=`env.MY VAR` |
| `"1VAR"` | reject, ConfigError field=`env.1VAR` |
| `""` | reject (empty key -- note: YAML `{"": "val"}` is valid syntax) |
| `"MY-VAR"` | reject (hyphens not allowed in shell variable names) |
| `"MY.VAR"` | reject (dots not allowed) |
| `"FOO=BAR"` | reject (equals not allowed in key) |
| `"FOO\nBAR"` | reject (newline in key) |

**ENV Value Tests** -- new function `TestParse_envValueValidation`:

| Key | Value | Expected |
|-----|-------|----------|
| `"DSN"` | `"host=localhost dbname=test"` | accept (spaces, equals) |
| `"PATH"` | `"/usr/local/bin:/usr/bin"` | accept (colons, slashes) |
| `"QUOTED"` | `"it's a \"test\""` | accept (quotes) |
| `"EMPTY"` | `""` | accept (empty value is valid) |
| `"UNICODE"` | `"cafe\u0301"` | accept (unicode) |
| `"NEWLINE"` | `"line1\nline2"` | reject, ConfigError field=`env.NEWLINE` |
| `"CR"` | `"line1\rline2"` | reject, ConfigError field=`env.CR` |
| `"NULL"` | `"val\x00ue"` | reject, ConfigError field=`env.NULL` |

Test structure: use existing `writeConfig()` helper and `errors.As(*ConfigError)` assertion pattern.

Note on YAML multiline strings: in test configs, newlines in values should be created using YAML literal or folded block scalars, or by using Go string construction. The `writeConfig()` helper writes raw bytes, so `\n` in Go strings will create actual newlines in the YAML file. YAML multiline values (e.g., using `|`) naturally produce strings with newlines.

### Previous Story Intelligence (11-2)

**Key learnings from story 11-2:**
- Package-level regex vars at `internal/config/parse.go:21-24` establish the pattern for new regex vars
- Validation functions follow pattern: `validate<Thing>(params) error` returning `*ConfigError`
- Error messages include the bad value and allowed charset/format
- Tests use `writeConfig()` helper and `errors.As(*ConfigError)` assertion
- Sort-based deterministic ordering used for SDK fields (slice of structs pattern at lines 121-128)
- All validation happens in `Parse()` before any downstream consumers
- No new files, error types, or exit codes were created -- same pattern applies here
- Story 11-2 notes: "Partially addresses: Template injection via unsanitized config inputs -- SDK/package/project inputs sanitized; ENV key/value deferred to story 11-3"

**Git patterns from recent commits:**
- `4ba3efe`: `feat: SDK version, package name, and project name sanitization (story 11-2)`
- `5775826`: `feat: concurrent sandbox sessions with random-suffixed container names (story 11-1)`
- Convention: `feat:` prefix for new validation capabilities

### Deferred Work Items Resolved

This story resolves the following item from `_bmad-output/implementation-artifacts/deferred-work.md`:
- "ENV key/value injection" (line 13): "ENV keys not validated for shell variable format (spaces, leading digits). YAML multiline strings in env values can inject arbitrary Dockerfile directives via unescaped `ENV {{$k}}={{$v}}`."

Completes resolution of:
- "Template injection via unsanitized config inputs" (line 14) -- was partially addressed by story 11-2 (SDK/package/project); ENV key/value is the final remaining input vector in `cfg.Env`

### What NOT To Change

- `embed/Dockerfile.tmpl` -- template already quotes ENV values at line 131 (`ENV {{$k}}="{{$v}}"`); validation prevents dangerous values from reaching the template. No escaping logic needed in the template.
- `internal/docker/run.go` -- runtime `--env` flags are passed via `os/exec` args array (safe by construction). Validation in the parser ensures correctness, not injection prevention, for this path.
- `cmd/root.go` / `cmd/root_test.go` -- no new error types or exit codes needed
- `internal/config/errors.go` -- reuse existing `ConfigError`; no new types
- `internal/config/config.go` -- `Env map[string]string` field unchanged

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 11, Story 11-3]
- [Source: _bmad-output/planning-artifacts/architecture.md - Config Validation, Dockerfile Generation, Error Handling Strategy]
- [Source: _bmad-output/implementation-artifacts/deferred-work.md - Security/Input Validation section, line 13-14]
- [Source: _bmad-output/implementation-artifacts/11-2-sdk-package-and-project-name-sanitization.md - Previous story patterns and learnings]
- [Source: internal/config/parse.go - Parse(), lines 21-24 (regex vars), 121-141 (SDK/package validation), 239-264 (validation functions)]
- [Source: internal/config/config.go - Config struct, Env field at line 49]
- [Source: internal/config/errors.go - ConfigError type, lines 6-16]
- [Source: embed/Dockerfile.tmpl - ENV directive at line 131: `ENV {{$k}}="{{$v}}"`]
- [Source: internal/docker/run.go - Runtime --env flag assembly at line 36-38]
- [Source: cmd/run.go - buildEnvVars() at lines 144-151, cfg.Env consumption]

## Dev Agent Record

### Agent Model Used

Codex GPT-5

### Implementation Plan

- Add parser-first table-driven tests for invalid ENV keys and values plus valid coverage, keeping `TestParse_validFullConfig` green.
- Validate `cfg.Env` in `Parse()` with deterministic sorted-key iteration before any downstream consumers see the values.
- Reuse `ConfigError` for field-specific failures and keep all work scoped to `internal/config/parse.go` and `internal/config/parse_test.go`.

### Debug Log References

- Initialization: loaded story 11-3, verified parser/test patterns from story 11-2, and confirmed this is a fresh implementation with no review follow-ups.
- Red: added env validator coverage in `internal/config/parse_test.go`; confirmed parser-level failures for invalid ENV keys/values before implementing the new validation hooks.
- Green: added `envKeyRe`, `validateEnvKey()`, and `validateEnvValue()` in `internal/config/parse.go`, then validated `cfg.Env` with sorted-key iteration inside `Parse()`.
- Validation: `go test ./internal/config -run 'Test(Parse_(validFullConfig|envValidation)|ValidateEnv(Key|Value))$'`, `go vet ./...`, `go test ./...` (full suite passed with Docker access).

### Completion Notes List

- Added parser-time ENV key validation with shell-variable-format enforcement, explicit empty-key rejection, and deterministic `ConfigError` field reporting under `env.<key>`.
- Added parser-time ENV value validation that rejects newline, carriage return, and null-byte characters before values reach Dockerfile or runtime env consumers.
- Added table-driven validator tests for `validateEnvKey()` and `validateEnvValue()` plus parse-level integration coverage for invalid YAML-representable ENV inputs.
- Verified `TestParse_validFullConfig` still passes and the full project regression suite remains green.

### File List

- internal/config/parse.go (modified - added ENV key regex, validator functions, and sorted ENV validation in `Parse()`)
- internal/config/parse_test.go (modified - added ENV validator unit tests and parse-level env validation coverage)

### Change Log

- 2026-04-16: Implemented ENV key/value validation in `internal/config/parse.go` and added validator/parse coverage in `internal/config/parse_test.go`.

### Review Findings

- [x] [Review][Decision] Dockerfile ENV injection via unescaped double-quotes and backslashes in values — RESOLVED: added `dqescape` template function in `render.go` and applied in `Dockerfile.tmpl`. Values with `"` and `\` are now escaped before interpolation. Test added in `render_test.go`.
- [x] [Review][Patch] Misleading error message when null byte triggers value validation — FIXED: updated to "Remove newlines and null bytes from the value". [internal/config/parse.go:303]
