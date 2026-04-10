# Story 1.2: Configuration Parsing and Validation

Status: done

## Story

As a developer,
I want asbox to parse my `.asbox/config.yaml` and validate all settings,
so that invalid configuration is caught early with clear error messages.

## Acceptance Criteria

1. **Given** a valid config.yaml with SDKs, packages, env vars, agent, and project_name
   **When** asbox parses the config via `config.Parse()`
   **Then** all values are extracted into typed Go structs and available for downstream use

2. **Given** no config file exists at the default path (`.asbox/config.yaml`) or the path specified by `-f`
   **When** asbox attempts to parse the config
   **Then** the CLI exits with code 1 and prints `"error: config file not found at <path>. Run 'asbox init' to create one"`

3. **Given** a config.yaml with a required field set to empty string (e.g., `agent: ""`)
   **When** asbox validates the config
   **Then** the CLI exits with code 1 and prints an error naming the empty required field and stating the fix action

4. **Given** the developer passes `-f path/to/config.yaml`
   **When** asbox parses the config
   **Then** it reads from the specified path instead of `.asbox/config.yaml`

5. **Given** mount paths in config use relative paths (e.g., `source: "."`)
   **When** asbox resolves mount paths
   **Then** paths are resolved relative to the config file location, not the working directory

6. **Given** `project_name` is not set in config
   **When** asbox derives the project name
   **Then** it defaults to the parent directory name of the `.asbox/` folder, sanitized to lowercase alphanumeric with hyphens

## Tasks / Subtasks

- [x] Task 1: Add `gopkg.in/yaml.v3` dependency (AC: #1)
  - [x] Run `go get gopkg.in/yaml.v3@v3.0.1`
  - [x] Verify `go mod tidy` produces clean `go.sum`

- [x] Task 2: Create Config struct definitions in `internal/config/config.go` (AC: #1)
  - [x] Define `Config` struct with YAML tags matching config keys exactly:
    ```go
    type Config struct {
        Agent           string            `yaml:"agent"`
        ProjectName     string            `yaml:"project_name"`
        SDKs            SDKConfig         `yaml:"sdks"`
        Packages        []string          `yaml:"packages"`
        MCP             []string          `yaml:"mcp"`
        Mounts          []MountConfig     `yaml:"mounts"`
        Secrets         []string          `yaml:"secrets"`
        Env             map[string]string `yaml:"env"`
        AutoIsolateDeps bool              `yaml:"auto_isolate_deps"`
        HostAgentConfig *MountConfig      `yaml:"host_agent_config"`
        BmadRepos       []string          `yaml:"bmad_repos"`
    }
    ```
  - [x] Define `SDKConfig` struct:
    ```go
    type SDKConfig struct {
        NodeJS string `yaml:"nodejs"`
        Go     string `yaml:"go"`
        Python string `yaml:"python"`
    }
    ```
  - [x] Define `MountConfig` struct:
    ```go
    type MountConfig struct {
        Source string `yaml:"source"`
        Target string `yaml:"target"`
    }
    ```

- [x] Task 3: Implement `Parse()` function in `internal/config/parse.go` (AC: #1, #2, #3, #4, #5, #6)
  - [x] Signature: `func Parse(configPath string) (*Config, error)`
  - [x] Read file at `configPath` — if not found, return `&ConfigError{Field: "", Msg: fmt.Sprintf("config file not found at %s. Run 'asbox init' to create one", configPath)}`
  - [x] Unmarshal YAML using `yaml.Unmarshal()` into `Config` struct
  - [x] If YAML unmarshal fails, return `&ConfigError{Msg: "invalid YAML: <parse error detail>"}`
  - [x] Validate required field `agent` is non-empty — if empty, return `&ConfigError{Field: "agent", Msg: "required field is empty. Set agent to 'claude-code' or 'gemini-cli'"}`
  - [x] Validate `agent` value is one of: `claude-code`, `gemini-cli` — if invalid, return `&ConfigError{Field: "agent", Msg: "unsupported agent '<value>'. Use 'claude-code' or 'gemini-cli'"}`
  - [x] Validate `mounts` entries have non-empty `source` and `target` — return `&ConfigError{Field: "mounts[N].source", Msg: "..."}` for empty values
  - [x] If `host_agent_config` is set, validate `source` and `target` are non-empty
  - [x] Derive `project_name` if empty: use parent directory of config file's directory, sanitize to `[a-z0-9-]+` (lowercase, replace non-alnum with hyphens, trim leading/trailing hyphens, collapse consecutive hyphens)
  - [x] Resolve mount paths relative to config file directory (use `filepath.Dir(configPath)` + `filepath.Join()` for relative paths; absolute paths passed through)
  - [x] Resolve `host_agent_config.Source` and `bmad_repos` paths the same way
  - [x] Return validated `*Config`

- [x] Task 4: Wire `Parse()` into `cmd/build.go` stub (AC: #4)
  - [x] In `cmd/build.go` RunE, call `config.Parse(configFile)` where `configFile` is the `-f` flag value from `cmd/root.go`
  - [x] On error, return the error (root command's exit code mapping handles it)
  - [x] On success, print `"config loaded: <configPath>"` to stdout (temporary — downstream stories will use the config)
  - [x] Remove the previous `ConfigError{Msg: "not implemented"}` return

- [x] Task 5: Wire `Parse()` into `cmd/run.go` stub (AC: #4)
  - [x] Same pattern as build.go — call `config.Parse(configFile)`, return error or print confirmation
  - [x] Remove the previous `ConfigError{Msg: "not implemented"}` return

- [x] Task 6: Write tests in `internal/config/parse_test.go` (AC: #1, #2, #3, #4, #5, #6)
  - [x] Use `t.TempDir()` for test config files — no real filesystem dependencies
  - [x] Table-driven tests for validation:
    - `TestParse_validFullConfig` — all fields populated, verify struct values match YAML
    - `TestParse_validMinimalConfig` — only `agent` and one mount, verify defaults
    - `TestParse_fileNotFound` — verify ConfigError with "config file not found" message
    - `TestParse_invalidYAML` — malformed YAML, verify ConfigError
    - `TestParse_emptyAgent` — `agent: ""`, verify ConfigError naming "agent" field
    - `TestParse_invalidAgent` — `agent: "chatgpt"`, verify ConfigError with supported values
    - `TestParse_emptyMountSource` — mount with empty source, verify ConfigError
    - `TestParse_emptyMountTarget` — mount with empty target, verify ConfigError
    - `TestParse_projectNameDerivation` — no `project_name`, verify derived from parent dir
    - `TestParse_projectNameSanitization` — name with spaces/special chars, verify sanitized
    - `TestParse_relativeMountPaths` — relative `source: "."`, verify resolved relative to config dir
    - `TestParse_absoluteMountPaths` — absolute source path, verify passed through unchanged
    - `TestParse_hostAgentConfigValidation` — set but empty source, verify ConfigError
  - [x] Use `errors.As()` to check error types (matches pattern from story 1.1)

- [x] Task 7: Verify build and tests (AC: all)
  - [x] Run `go vet ./...`
  - [x] Run `go test ./...`
  - [x] Run `CGO_ENABLED=0 go build -o asbox .`

## Dev Notes

### Architecture Compliance

- **`internal/config/config.go`**: Struct definitions only. YAML tags match keys exactly (`yaml:"field_name"`). No methods beyond what's needed. Config structs mirror YAML structure: `Config`, `SDKConfig`, `MountConfig`.
- **`internal/config/parse.go`**: Single `Parse()` function — reads YAML, validates required fields, resolves paths. Returns validated `*Config` or `*ConfigError`. This is the **single source of truth** — config is parsed ONCE and consumed by template, docker, hash, and mount packages.
- **Config validation before template rendering**: Zero-value required fields MUST be caught in `Parse()`, not after rendering. Template rendering (Story 1.3+) assumes a fully validated config struct.
- **Path resolution**: Mount paths resolved relative to config file location, NOT working directory. Use `filepath.Dir(configPath)` as base. This applies to `mounts`, `host_agent_config`, and `bmad_repos`.
- **`cmd/` layer stays thin**: `cmd/build.go` and `cmd/run.go` call `config.Parse(configFile)` and return errors. No validation logic in `cmd/`.
- **Error types**: Reuse existing `ConfigError{Field, Msg}` from `internal/config/errors.go`. Do NOT create new error types. When `Field` is set, error formats as `"config field <field>: <msg>"`.
- **Dependency direction**: `cmd/` -> `internal/config/` -> standard library + `gopkg.in/yaml.v3`. The config package does NOT import other internal packages.

### YAML Library: `gopkg.in/yaml.v3`

- Latest stable: **v3.0.1**
- Usage: `yaml.Unmarshal(data, &cfg)` — unmarshals into typed Go structs
- YAML tags: `yaml:"field_name"` on struct fields
- Zero-value handling: unset YAML keys produce Go zero values (empty string, nil slice, false). Validation must catch required-but-empty fields explicitly.
- Optional fields (slices, maps, pointers) naturally handle absence: nil slice for missing `packages`, nil map for missing `env`, nil pointer for missing `host_agent_config`

### Config YAML Conventions (from Architecture)

- Keys are `lower_snake_case`: `auto_isolate_deps`, `host_agent_config`, `bmad_repos`
- Mount entries use `source`/`target` keys (matching Docker convention)
- No nesting beyond two levels deep
- Config structs use `yaml:"field_name"` tags matching YAML keys exactly

### Required vs Optional Fields

| Field | Required | Default/Behavior if absent |
|-------|----------|---------------------------|
| `agent` | YES | Error — must be `claude-code` or `gemini-cli` |
| `project_name` | NO | Derived from parent dir of `.asbox/`, sanitized |
| `sdks` | NO | Zero value — no SDKs installed |
| `packages` | NO | nil slice — no extra packages |
| `mcp` | NO | nil slice — no MCP servers |
| `mounts` | NO | nil slice — no mounts (valid but unusual) |
| `secrets` | NO | nil slice — no secrets |
| `env` | NO | nil map — no env vars |
| `auto_isolate_deps` | NO | false |
| `host_agent_config` | NO | nil pointer — not configured |
| `bmad_repos` | NO | nil slice — no repos |

### Project Name Derivation Logic

When `project_name` is not set in config:
1. Get the directory containing the config file: `filepath.Dir(configPath)`
2. If config is at `.asbox/config.yaml`, the parent is the project root
3. Get the base name of the project root directory
4. Sanitize: lowercase, replace `[^a-z0-9-]` with hyphens, collapse consecutive hyphens, trim leading/trailing hyphens
5. Example: `/Users/Manuel/My Project/` -> `my-project`

### Error Message Format (from Architecture)

Every user-facing error includes **what failed**, **why**, and **what to do**:
- `"config file not found at .asbox/config.yaml. Run 'asbox init' to create one"`
- `"config field agent: required field is empty. Set agent to 'claude-code' or 'gemini-cli'"`
- `"config field mounts[0].source: required field is empty. Set source path for each mount entry"`
- `"config field agent: unsupported agent 'chatgpt'. Use 'claude-code' or 'gemini-cli'"`

### Previous Story Intelligence (Story 1.1)

Key patterns established in Story 1.1 to follow:
- **Error types in `internal/config/errors.go`**: `ConfigError{Field, Msg}` and `SecretError{Msg}` already exist. Reuse `ConfigError` — do NOT create a new error type.
- **Exit code mapping in `cmd/root.go`**: Uses `errors.As()` for type checking. `ConfigError` maps to exit code 1. Already handles wrapped errors.
- **`configFile` variable**: Package-level `var configFile string` in `cmd/root.go` with default `.asbox/config.yaml`. Accessible from `cmd/build.go` and `cmd/run.go`.
- **Test patterns**: Table-driven tests, `TestFunctionName_scenario` naming, `errors.As()` for type assertions.
- **`cmd/init.go`**: Leave as-is (still returns "not implemented"). Story 1.8 implements the real init.
- **`SilenceErrors: true`**: Root command silences Cobra's error output; `Execute()` in `root.go` handles all error printing to stderr.
- **Stub removal**: When wiring `Parse()` into `build.go` and `run.go`, remove the `ConfigError{Msg: "not implemented"}` return. The Docker check in `PersistentPreRunE` still runs before these commands.

### Go Code Conventions (from Architecture)

- **Formatting**: `gofmt` is law, `go vet` must pass
- **File naming**: `snake_case.go` — `config.go`, `parse.go`, `parse_test.go`
- **Test naming**: `TestFunctionName_scenario` — `TestParse_missingAgent`, `TestParse_validFullConfig`
- **Table-driven tests** preferred for multiple scenarios
- **Variable naming**: `camelCase` — `configPath`, `configDir`, `projectName`
- **No `os.Exit()` in `internal/` packages** — return typed errors

### Key Anti-Patterns to Avoid

- Do NOT create `internal/utils/` or `internal/common/` packages
- Do NOT add validation logic in `cmd/` layer — all validation in `internal/config/parse.go`
- Do NOT use `interface{}` or `any` as function parameters — use typed config structs
- Do NOT add color codes, spinners, or progress bars to output
- Do NOT validate fields that are optional and empty — only validate required fields and structural correctness
- Do NOT import other `internal/` packages from `internal/config/`
- Do NOT use `os.Getwd()` for path resolution — always resolve relative to config file directory

### Project Structure Notes

Files created/modified by this story:
```
internal/config/config.go      (new) — Config, SDKConfig, MountConfig struct definitions
internal/config/parse.go       (new) — Parse() function with validation and path resolution
internal/config/parse_test.go  (new) — Table-driven tests for all parse/validate scenarios
cmd/build.go                   (modified) — Wire config.Parse(), remove "not implemented" stub
cmd/run.go                     (modified) — Wire config.Parse(), remove "not implemented" stub
go.mod                         (modified) — Add gopkg.in/yaml.v3 dependency
go.sum                         (modified) — Updated checksums
```

Existing files NOT modified:
- `internal/config/errors.go` — ConfigError already exists, reuse as-is
- `cmd/root.go` — No changes needed, configFile flag already defined
- `cmd/init.go` — Leave as stub (Story 1.8)
- `embed/` — No changes

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Story 1.2: Configuration Parsing and Validation]
- [Source: _bmad-output/planning-artifacts/architecture.md — Go Project Organization section]
- [Source: _bmad-output/planning-artifacts/architecture.md — Config YAML Conventions section]
- [Source: _bmad-output/planning-artifacts/architecture.md — Dockerfile Generation decision — config validation before rendering]
- [Source: _bmad-output/planning-artifacts/architecture.md — Error Handling Strategy section]
- [Source: _bmad-output/planning-artifacts/architecture.md — internal/config/config.go struct definitions]
- [Source: _bmad-output/planning-artifacts/architecture.md — Cross-Cutting Concerns — Path resolution]
- [Source: _bmad-output/planning-artifacts/architecture.md — Anti-Patterns section]
- [Source: _bmad-output/planning-artifacts/prd.md — Configuration File section with example YAML]
- [Source: _bmad-output/implementation-artifacts/1-1-go-project-scaffold-and-cli-skeleton.md — Dev Notes, Review Findings]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None — implementation proceeded without blockers.

### Completion Notes List

- Implemented Config, SDKConfig, MountConfig structs with exact YAML tags matching config keys
- Parse() function handles: file reading, YAML unmarshaling, required field validation (agent), agent value validation, mount validation, host_agent_config validation, project name derivation with sanitization, and relative path resolution
- All error messages follow the "what failed + why + what to do" format using existing ConfigError type
- Wired config.Parse() into build and run commands, removing "not implemented" stubs
- Updated cmd/root_test.go to reflect that build/run now call config.Parse() instead of returning stub errors
- 15 tests in parse_test.go covering all acceptance criteria scenarios
- All tests pass, go vet clean, binary builds successfully

### Change Log

- 2026-04-08: Implemented story 1-2 configuration parsing and validation — all tasks complete

### File List

- internal/config/config.go (new) — Config, SDKConfig, MountConfig struct definitions
- internal/config/parse.go (new) — Parse() function with validation and path resolution
- internal/config/parse_test.go (new) — 15 table-driven tests for all parse/validate scenarios
- cmd/build.go (modified) — Wired config.Parse(), removed "not implemented" stub
- cmd/run.go (modified) — Wired config.Parse(), removed "not implemented" stub
- cmd/root_test.go (modified) — Updated stub test for init-only, added build/run config error test
- go.mod (modified) — Added gopkg.in/yaml.v3 v3.0.1 dependency
- go.sum (modified) — Updated checksums

### Review Findings

- [x] [Review][Decision] Mount `target` not validated as absolute path — fixed: added `filepath.IsAbs` check for mount and host_agent_config targets [internal/config/parse.go:72]
- [x] [Review][Patch] Tilde (`~`) in mount/path fields not expanded — fixed: `resolvePath` now expands `~/` prefix to `$HOME` [internal/config/parse.go:128-133]
- [x] [Review][Patch] `sanitizeProjectName` can return empty string — fixed: falls back to `"asbox"` when sanitization yields empty string [internal/config/parse.go:105-107]
- [x] [Review][Defer] `os.IsNotExist` misses `ENOTDIR` errors — path with file component instead of directory gives opaque error instead of "config file not found" hint [internal/config/parse.go:25] — deferred, pre-existing pattern
- [x] [Review][Defer] `init` stub misuses `ConfigError` for "not implemented" — semantically incorrect error type [cmd/init.go] — deferred, story 1-8 replaces this
- [x] [Review][Defer] `resolvePath` allows `..` traversal outside project directory — by design for user-owned host mounts, but undocumented [internal/config/parse.go:104-106] — deferred, not actionable now
