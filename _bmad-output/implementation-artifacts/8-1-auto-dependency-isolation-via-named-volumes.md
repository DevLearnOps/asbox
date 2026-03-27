# Story 8.1: Auto Dependency Isolation via Named Volumes

Status: done

## Story

As a developer,
I want the sandbox to automatically detect `package.json` files in my mounted project paths and isolate their `node_modules/` directories with named Docker volumes,
So that macOS-compiled native modules don't crash inside the Linux sandbox and I don't have to manually manage volume mounts.

## Acceptance Criteria

1. **Given** a config with `auto_isolate_deps: true` and a mount with a project containing `package.json` at the root, **When** the developer runs `sandbox run`, **Then** the system creates a named volume mount over `<container_target>/node_modules` (e.g., `-v sandbox-myapp-node_modules:/workspace/node_modules`) and logs `isolating: /workspace/node_modules (volume: sandbox-myapp-node_modules)` to stdout.

2. **Given** a monorepo with `package.json` files at root, `packages/api/`, and `packages/web/`, **When** the sandbox launches with `auto_isolate_deps: true`, **Then** three named volume mounts are created, one for each `node_modules/` sibling, with volume names following the convention `sandbox-<project>-<relative-path-dashed>-node_modules`.

3. **Given** `auto_isolate_deps` is absent or `false` in config, **When** the sandbox launches, **Then** no scanning occurs, no volumes are added, zero overhead.

4. **Given** a fresh project with no `package.json` files yet, **When** the sandbox launches with `auto_isolate_deps: true`, **Then** no volumes are created, no output is logged -- the agent creates Linux-native dependencies from scratch on first `npm install`.

5. **Given** named volumes were created in a previous session, **When** the developer launches a new sandbox session, **Then** the same named volumes are reused, preserving previously installed Linux-native dependencies across sessions.

6. **Given** `sandbox init` generates a starter config, **When** the developer inspects the generated config, **Then** `auto_isolate_deps` appears as a commented-out option with an inline explanation of when to enable it.

## Tasks / Subtasks

- [x] Task 1: Add `auto_isolate_deps` config parsing (AC: #3)
  - [x] 1.1 Add `CFG_AUTO_ISOLATE_DEPS=""` to global config variables in `sandbox.sh` (~line 72-87)
  - [x] 1.2 Add yq extraction in `parse_config()` after the MCP extraction block (~line 180): `CFG_AUTO_ISOLATE_DEPS="$(yq eval '.auto_isolate_deps // ""' "${CONFIG_PATH}")"` with null-to-empty normalization
- [x] Task 2: Implement `detect_isolate_deps()` function (AC: #1, #2, #4)
  - [x] 2.1 Create function `detect_isolate_deps()` in `sandbox.sh` after `validate_secrets()` (~line 207) and before `sed_escape_replacement()` (~line 214)
  - [x] 2.2 Early return if `CFG_AUTO_ISOLATE_DEPS` is not `true` (AC #3 -- zero overhead)
  - [x] 2.3 For each mount in `CFG_MOUNT_SOURCES` / `CFG_MOUNT_TARGETS`, resolve the host-side source path (same tilde/relative resolution as `cmd_run()`)
  - [x] 2.4 Run `find <resolved_source> -name package.json -not -path '*/node_modules/*'` on each mount source
  - [x] 2.5 For each discovered `package.json`: compute relative path from mount source, derive `node_modules` sibling path
  - [x] 2.6 Build volume name: `sandbox-<CFG_PROJECT_NAME>-<relative-path-with-slashes-as-dashes>-node_modules` (root-level package.json produces `sandbox-<project>-node_modules`, no trailing dash)
  - [x] 2.7 Build container target: `<mount_target>/<relative_dir>/node_modules`
  - [x] 2.8 Append `-v <volume_name>:<container_target>` to an output array
  - [x] 2.9 Log each isolation: `info "isolating: <container_target> (volume: <volume_name>)"`
  - [x] 2.10 If no `package.json` found across all mounts: silent return, no output (AC #4)
- [x] Task 3: Integrate `detect_isolate_deps()` into `cmd_run()` (AC: #1, #2)
  - [x] 3.1 Call `detect_isolate_deps` in `cmd_run()` after `parse_config` / `validate_secrets` but before `cmd_build` and docker run flag assembly
  - [x] 3.2 Append the returned `-v` flags to `run_flags` array before the `docker run` invocation
- [x] Task 4: Extract project name for volume naming (AC: #1, #2)
  - [x] 4.1 Add `CFG_PROJECT_NAME` extraction in `parse_config()`: derive from config -- check if a `project_name` key exists in config, else fall back to the directory name of the config file's parent
  - [x] 4.2 Sanitize project name for volume naming (lowercase, replace non-alphanumeric with dashes)
- [x] Task 5: Update `templates/config.yaml` (AC: #6)
  - [x] 5.1 Add commented-out `auto_isolate_deps` option with inline explanation, placed after `mounts` section and before `secrets` section
  - [x] 5.2 Comment should explain: "Enable to auto-detect package.json files in mounts and isolate node_modules via named volumes (prevents macOS/Linux native module clashes)"
- [x] Task 6: Add unit tests in `tests/test_sandbox.sh` (AC: #1-#6)
  - [x] 6.1 Test: `auto_isolate_deps: true` with single `package.json` at mount root -- verify `-v sandbox-<project>-node_modules:<target>/node_modules` flag generated
  - [x] 6.2 Test: monorepo with multiple `package.json` files -- verify correct volume names and paths for each
  - [x] 6.3 Test: `auto_isolate_deps` absent from config -- verify no `-v` flags, no output
  - [x] 6.4 Test: `auto_isolate_deps: false` -- verify no scanning
  - [x] 6.5 Test: no `package.json` files in mount -- verify silent return
  - [x] 6.6 Test: volume naming with nested paths (slashes become dashes)
  - [x] 6.7 Test: config parsing extracts `auto_isolate_deps` correctly (true, false, absent)

## Dev Notes

### Architecture Compliance

**Source:** [architecture.md#Automatic Dependency Isolation]

The architecture specifies:
- New function `detect_isolate_deps()` in `sandbox.sh`
- Called from run path after `parse_config()` but before `docker run` command assembly
- Host-side scan using `find` -- once inside the container it's too late to add mounts
- Named volumes (not anonymous, not host-mapped) for persistence across sessions
- Volume naming convention: `sandbox-<project_name>-<relative_path_with_dashes>-node_modules`
- Returns additional `-v` flags appended to the run command
- Logging follows existing output conventions using `info` function

**IMPORTANT -- Named volumes, NOT anonymous volumes:** The PRD mentions "anonymous volume mounts" in some places but the architecture decision document (which is authoritative for implementation) specifies **named volumes**. Use named volumes. Named volumes persist across sessions and have predictable names for management. This is the correct implementation.

### Existing Code Patterns to Follow

**Function placement:** `sandbox.sh` is organized top-to-bottom: utilities -> config parsing -> validation -> build functions -> run functions -> init -> dispatch. Place `detect_isolate_deps()` after `validate_secrets()` (~line 207) since it's a pre-run validation/setup function.

**Config variable pattern:** All config values use `CFG_` prefix globals. Follow the existing yq extraction pattern:
```bash
CFG_AUTO_ISOLATE_DEPS="$(yq eval '.auto_isolate_deps // ""' "${CONFIG_PATH}")"
if [[ "${CFG_AUTO_ISOLATE_DEPS}" == "null" ]]; then CFG_AUTO_ISOLATE_DEPS=""; fi
```

**Path resolution pattern:** `cmd_run()` already resolves mount paths with tilde expansion and config-dir-relative resolution. The `detect_isolate_deps()` function MUST use the same resolution logic. Extract path resolution into a reusable helper or duplicate the pattern -- but it MUST match exactly.

**Output convention:** Use `info "isolating: ..."` (the existing `info()` helper prints to stdout without prefix). Errors go to stderr via `die()`. No color codes, no emojis.

**Array pattern for volume flags:** Follow the existing `run_flags+=()` array pattern. The function should populate a global array (e.g., `ISOLATE_VOLUME_FLAGS=()`) that `cmd_run()` appends to `run_flags`.

### Implementation Details

**Volume name construction:**
```
# Root package.json in mount with target /workspace:
# relative_dir = "" (empty)
# volume = sandbox-myapp-node_modules

# packages/api/package.json in mount with target /workspace:
# relative_dir = "packages/api"
# volume = sandbox-myapp-packages-api-node_modules
```

**Deriving relative path from find output:**
```bash
# For each package.json found:
# 1. Get dirname of package.json path
# 2. Compute path relative to mount source
# 3. Replace "/" with "-" for volume name
# 4. If relative path is empty (root), omit the middle segment
```

**Config-dir resolution (reuse from cmd_run):**
```bash
config_dir="$(cd "$(dirname "${CONFIG_PATH}")" && pwd)"
# For each mount source:
if [[ "${src}" == "~/"* ]]; then
  src="${HOME}/${src#\~/}"
elif [[ "${src}" != /* ]]; then
  src="$(cd "${config_dir}/${src}" && pwd)"
fi
```

**Project name derivation:** The config file doesn't currently have a `project_name` field. Derive it from the parent directory name of the config file (the project root). Sanitize: lowercase, replace spaces/special chars with dashes.

### Testing Standards

Tests live in `tests/test_sandbox.sh` using the project's custom TAP-like runner (no external dependency). Follow patterns:
- Use `setup_build_mock` for test isolation
- Create temp directories with mock `package.json` files
- Assert on generated `-v` flags (use `assert_contains` / `assert_not_contains`)
- Test the function in isolation by sourcing `sandbox.sh` functions

### File List (expected changes)

| File | Change |
|------|--------|
| `sandbox.sh` | Add `CFG_AUTO_ISOLATE_DEPS` global, parse in `parse_config()`, new `detect_isolate_deps()` function, integrate into `cmd_run()` |
| `templates/config.yaml` | Add commented-out `auto_isolate_deps` option |
| `tests/test_sandbox.sh` | Add unit tests for config parsing + volume detection |

### Edge Cases

- **Mount source doesn't exist yet:** `find` on non-existent path returns error. Guard with `-d` check before scanning.
- **Symlinked node_modules:** `find` follows symlinks by default. Use `-not -path '*/node_modules/*'` to exclude nested ones.
- **Special characters in path:** Volume names must be alphanumeric + dashes + underscores. Sanitize any special characters from paths.
- **Multiple mounts pointing to overlapping paths:** Each mount is scanned independently. If two mounts overlap, the same `package.json` could be found twice. Deduplicate by container target path.
- **Windows-style paths:** Not applicable -- sandbox only runs on macOS/Linux (NFR11).

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Automatic Dependency Isolation]
- [Source: _bmad-output/planning-artifacts/prd.md#FR9a, FR9b, FR9c, FR16a]
- [Source: _bmad-output/planning-artifacts/epics.md#Epic 8, Story 8.1]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None -- all tests passed on first run (501/501).

### Completion Notes List

- Added `CFG_AUTO_ISOLATE_DEPS` and `CFG_PROJECT_NAME` config globals with yq extraction and null normalization in `parse_config()`
- Project name derived from config `project_name` key or fallback to parent directory name, sanitized (lowercase, dashes)
- Implemented `detect_isolate_deps()` function: scans mount sources for `package.json` files via `find`, builds named volume flags using `ISOLATE_VOLUME_FLAGS` global array, deduplicates by container target path, handles edge cases (non-existent dirs, nested paths)
- Integrated into `cmd_run()`: called after `validate_secrets` but before `cmd_build`, volume flags appended to `run_flags` before `docker run`
- Updated `templates/config.yaml` with commented-out `auto_isolate_deps: true` option between mounts and secrets sections
- Added 25 unit tests covering: single package.json, monorepo (3 package.json), absent config, false config, no package.json (silent), nested path volume naming, build compatibility, template content

### Change Log

- 2026-03-27: Implemented story 8-1 — auto dependency isolation via named volumes. All 6 tasks complete, 501/501 tests passing.

### File List

| File | Change |
|------|--------|
| `sandbox.sh` | Added `CFG_AUTO_ISOLATE_DEPS`, `CFG_PROJECT_NAME`, `ISOLATE_VOLUME_FLAGS` globals; added parsing in `parse_config()`; new `detect_isolate_deps()` function; integrated into `cmd_run()` |
| `templates/config.yaml` | Added commented-out `auto_isolate_deps` option with explanation |
| `tests/test_sandbox.sh` | Added 25 unit tests for story 8.1 (tests 477-501) |
