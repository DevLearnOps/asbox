# Story 2.1: Host Directory Mounts

Status: review

## Story

As a developer,
I want to declare host directories in my config that get mounted into the sandbox,
so that the agent can read and write my project files.

## Acceptance Criteria

1. **Given** a config with mounts `[{source: ".", target: "/workspace"}]`
   **When** the sandbox launches
   **Then** the host directory is mounted at `/workspace` inside the container and the agent can read/write files there

2. **Given** mount source paths are relative (e.g., `.` or `../shared`)
   **When** sandbox resolves mount paths
   **Then** paths are resolved relative to the config file location, not the working directory

3. **Given** a config with multiple mount entries
   **When** the sandbox launches
   **Then** all declared mounts are applied with correct source and target paths

## Tasks / Subtasks

- [x] Task 1: Implement mount path resolution in `cmd_run()` (AC: 1, 2, 3)
  - [x] Determine `CONFIG_DIR` from the resolved config file path (dirname of config file)
  - [x] Loop over `CFG_MOUNT_SOURCES` and `CFG_MOUNT_TARGETS` arrays
  - [x] Resolve each relative source path against `CONFIG_DIR` using `cd "$dir" && pwd`
  - [x] Assemble `-v "resolved_source:target"` flags for each mount
  - [x] Add `-w` working directory flag set to the first mount's target path (conventional workspace)
  - [x] Append mount flags to the `docker run` command

- [x] Task 2: Add tests for mount flag assembly in `cmd_run()` (AC: 1, 2, 3)
  - [x] Test: single mount `{source: ".", target: "/workspace"}` produces `-v /abs/path:/workspace` flag
  - [x] Test: relative source path resolved against config file directory, not `$PWD`
  - [x] Test: multiple mounts produce multiple `-v` flags in docker run
  - [x] Test: `-w` flag is set to first mount's target path
  - [x] Test: `sandbox run` with no mounts in config produces no `-v` flags (zero mounts is valid)
  - [x] Test: absolute source paths are passed through unchanged

- [x] Task 3: Verify end-to-end mount behavior (AC: 1)
  - [x] Test: config with mounts uncommented passes parse + build + run without errors
  - [x] Verify mount flags appear in mock docker log with correct resolved paths

## Dev Notes

### Architecture Compliance

- **Mount path resolution**: Relative paths resolved relative to config file location, not working directory. This is a cross-cutting concern documented in architecture. [Source: architecture.md#Cross-Cutting Concerns Identified]
- **Config YAML conventions**: Mount entries use `source`/`target` keys matching Docker convention. [Source: architecture.md#Config YAML Conventions]
- **Single parse_config()**: Mounts are already parsed by `parse_config()` into `CFG_MOUNT_SOURCES[]` and `CFG_MOUNT_TARGETS[]`. Do NOT re-parse config or add ad-hoc yq calls. [Source: architecture.md#Gap 4]
- **Exit codes**: 0 = success, 1 = general error (e.g., mount source dir doesn't exist). [Source: architecture.md#Exit Codes]
- **No color codes, no spinners** -- plain text only.
- **`set -euo pipefail`** already enforced.
- **Quoting**: Always `"${var}"` -- critical for paths with spaces.

### Implementation Specifics

**What already exists (DO NOT recreate):**
- `parse_config()` (sandbox.sh lines 91-175) already parses mounts from config YAML into:
  - `CFG_MOUNT_SOURCES=()` -- array of source paths (strings from config)
  - `CFG_MOUNT_TARGETS=()` -- array of target paths (strings from config)
- Tests for mount parsing already exist (test_sandbox.sh lines 481-496): verifies extraction of source/target pairs from config
- `cmd_run()` (sandbox.sh lines 355-362) currently runs: `docker run -it --rm -e "SANDBOX_AGENT=${CFG_AGENT}" "${IMAGE_TAG}"`

**What needs to change:**

`cmd_run()` must be extended to assemble `-v` mount flags. The implementation pattern:

```bash
cmd_run() {
  parse_config
  cmd_build

  # Resolve mount paths and assemble flags
  local run_flags=()
  run_flags+=("-it" "--rm")
  run_flags+=("-e" "SANDBOX_AGENT=${CFG_AGENT}")

  local config_dir
  config_dir="$(cd "$(dirname "${CONFIG_FILE}")" && pwd)"

  local i
  for i in "${!CFG_MOUNT_SOURCES[@]}"; do
    local src="${CFG_MOUNT_SOURCES[$i]}"
    local tgt="${CFG_MOUNT_TARGETS[$i]}"

    # Resolve relative source paths against config file directory
    if [[ "${src}" != /* ]]; then
      src="$(cd "${config_dir}/${src}" && pwd)"
    fi

    run_flags+=("-v" "${src}:${tgt}")
  done

  # Set working directory to first mount target (if any mounts exist)
  if [[ ${#CFG_MOUNT_TARGETS[@]} -gt 0 ]]; then
    run_flags+=("-w" "${CFG_MOUNT_TARGETS[0]}")
  fi

  info "starting sandbox: ${IMAGE_TAG}"
  docker run "${run_flags[@]}" "${IMAGE_TAG}"
}
```

**Key design decisions:**
- `CONFIG_FILE` is the global set during argument parsing (or default `.sandbox/config.yaml`). Use `dirname` of this to get the config directory for relative path resolution.
- `cd "${config_dir}/${src}" && pwd` resolves `.` and `../foo` to absolute paths. If the directory doesn't exist, `cd` fails and `set -e` aborts with an error -- this is the desired behavior (fail fast with clear error).
- `-w` (working directory) is set to the first mount's target. This gives the agent a sensible starting directory. Only set if mounts exist.
- Use an array (`run_flags`) for docker run arguments. This is safer than string concatenation -- handles paths with spaces correctly and avoids eval.
- Zero mounts is a valid config -- the agent just won't have project files. No error needed.

**Path resolution edge cases:**
- `.` resolves to the directory containing the config file
- `../shared` resolves relative to config file directory
- Absolute paths (e.g., `/opt/data`) are passed through unchanged
- Non-existent source directory: `cd` fails, script exits with code 1 (set -e). The error message from bash is clear enough ("No such file or directory").

### Testing Strategy

**Extend existing mock docker approach from stories 1.5 and 1.6:**
- Mock docker binary logs all invocations to `MOCK_DOCKER_LOG`
- Tests create temp config files with mount declarations
- After running `sandbox run`, inspect `docker.log` for `-v` flags with correct resolved paths
- Use config files in temp directories to verify relative path resolution

**Test setup for path resolution verification:**
```bash
# Create config in a subdirectory to test relative path resolution
config_dir="${tmpdir}/project/.sandbox"
mkdir -p "${config_dir}"
cat > "${config_dir}/config.yaml" << 'EOF'
agent: claude-code
mounts:
  - source: ".."
    target: "/workspace"
EOF
# source ".." relative to config_dir should resolve to "${tmpdir}/project"
```

**What to verify in docker.log:**
- `-v /absolute/path:/workspace` -- resolved absolute path, not relative
- Multiple `-v` flags for multiple mounts
- `-w /workspace` -- working directory matches first mount target
- No `-v` flags when config has no mounts

### Previous Story (1-6) Intelligence

**Established patterns to reuse:**
- `setup_build_mock()` creates mock docker with configurable inspect exit code
- Mock docker logs all invocations for assertion
- `run_flags` array pattern was not used in 1-6 (it used inline flags) -- this story introduces array-based flag assembly, which is cleaner and extensible for future stories (secrets in 2.2, env vars in 2.3)
- 165 tests currently pass -- extend without regressions

**Key learning from 1-6:**
- `cmd_run()` calls `cmd_build()` which also calls `parse_config()`. Double parse is accepted (idempotent).
- Tests use `PATH="${mockdir}:${PATH}" bash "${SANDBOX}" run -f "${config}"` pattern

**Files modified in 1-6:**
- `sandbox.sh` -- `cmd_run()` implemented with basic docker run flags
- `scripts/entrypoint.sh` -- agent exec routing
- `tests/test_sandbox.sh` -- 27 new/updated tests (165 total)

### Git History Context

Recent commits follow `feat:` prefix convention:
- `158065b feat: implement sandbox run with TTY and lifecycle with review fixes (story 1-6)`
- `4e76771 feat: implement image build with content-hash caching and review fixes (story 1-5)`

### Scope Boundaries

**IN scope for story 2.1:**
- Extend `cmd_run()` to assemble `-v` volume mount flags from parsed config
- Resolve relative mount source paths against config file directory
- Set `-w` (working directory) to first mount target
- Tests for mount flag assembly and path resolution

**OUT of scope (later stories):**
- Secret injection `--env` for secrets (Story 2.2)
- Non-secret env vars `--env` (Story 2.3)
- Agent runtime verification with mounted files (Story 2.3)
- MCP .mcp.json generation in entrypoint (Story 5.2)
- Network configuration (Story 4.x)
- Mount permissions or read-only mounts (not in requirements)
- Validation that mount target paths are absolute (Docker handles this)

### Anti-Patterns to Avoid

- Do NOT add secret injection (`--env` for secrets) -- that's Story 2.2
- Do NOT add non-secret env var passing -- that's Story 2.3
- Do NOT use string concatenation for docker run flags -- use an array
- Do NOT use `eval` to construct the docker run command
- Do NOT re-parse config.yaml with ad-hoc yq calls -- use the existing `CFG_MOUNT_*` arrays
- Do NOT validate mount target paths -- Docker already validates these
- Do NOT add mount permission flags (`:ro`, `:rw`) -- not in requirements
- Do NOT add `--network` or other flags beyond mounts and `-w`
- Do NOT modify `parse_config()` -- mount parsing already works correctly
- Do NOT modify `entrypoint.sh` -- mounts are handled at docker run time, not inside the container

### Project Structure Notes

- `sandbox.sh` -- modify `cmd_run()` to add mount flag assembly (replace current inline flags with array-based approach)
- `tests/test_sandbox.sh` -- add new test section for mount flag verification
- No new files needed
- No changes to `parse_config()`, `entrypoint.sh`, `Dockerfile.template`, or `templates/config.yaml`

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Cross-Cutting Concerns Identified] -- path resolution relative to config file
- [Source: _bmad-output/planning-artifacts/architecture.md#Config YAML Conventions] -- source/target mount keys
- [Source: _bmad-output/planning-artifacts/architecture.md#Implementation Patterns & Consistency Rules] -- quoting, error handling
- [Source: _bmad-output/planning-artifacts/architecture.md#Anti-Patterns] -- no eval, no unquoted vars
- [Source: _bmad-output/planning-artifacts/architecture.md#Gap 4] -- single parse_config() mandate
- [Source: _bmad-output/planning-artifacts/architecture.md#File Responsibilities] -- sandbox.sh owns mount flag assembly
- [Source: _bmad-output/planning-artifacts/epics.md#Story 2.1] -- acceptance criteria
- [Source: _bmad-output/planning-artifacts/epics.md#FR4, FR20] -- mount requirements
- [Source: _bmad-output/planning-artifacts/prd.md#NFR6] -- mounted paths limited to declared config
- [Source: _bmad-output/implementation-artifacts/1-6-sandbox-run-with-tty-and-lifecycle.md] -- previous story patterns

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

No debug issues encountered.

### Completion Notes List

- Extended `cmd_run()` in sandbox.sh to assemble docker run flags using an array (`run_flags`), replacing inline flag concatenation
- Mount source paths are resolved relative to config file directory using `cd "$(dirname CONFIG_PATH)" && pwd`
- Relative paths resolved via `cd "${config_dir}/${src}" && pwd`; absolute paths passed through unchanged
- `-w` (working directory) set to first mount target when mounts exist; omitted when no mounts
- Zero mounts is valid -- no `-v` or `-w` flags emitted
- Added 12 unit tests for mount flag assembly (single mount, relative resolution, multiple mounts, -w flag, zero mounts, absolute paths)
- Added 6 end-to-end tests verifying full parse+build+run pipeline with mounts and correct docker log output
- All 187 tests pass (18 new, 169 existing -- 0 regressions)

### Change Log

- 2026-03-24: Implemented host directory mount support in cmd_run() -- Story 2.1 complete

### File List

- sandbox.sh (modified -- cmd_run() rewritten with array-based flag assembly and mount path resolution)
- tests/test_sandbox.sh (modified -- 18 new mount tests added)
