# Story 6.1: Auto Dependency Isolation via Named Volumes

Status: in-progress

## Story

As a developer,
I want the sandbox to automatically detect `package.json` files and isolate `node_modules/` with named Docker volumes,
So that macOS-compiled native modules don't crash inside the Linux sandbox.

## Acceptance Criteria

1. **Given** a config with `auto_isolate_deps: true` and a mount with `package.json` at the root, **When** the developer runs `asbox run`, **Then** a named volume mount is created (`-v asbox-myapp-node_modules:/workspace/node_modules`).

2. **Given** a monorepo with `package.json` at root, `packages/api/`, and `packages/web/`, **When** the sandbox launches with `auto_isolate_deps: true`, **Then** three named volume mounts are created with pattern `asbox-<project>-<path-dashed>-node_modules`.

3. **Given** `auto_isolate_deps: true` and `bmad_repos` configured with repos containing `package.json` files, **When** the sandbox launches, **Then** named volume mounts are also created for each `node_modules/` in the bmad_repos, using container paths under `/workspace/repos/<basename>/`.

4. **Given** `auto_isolate_deps: true` and `bmad_repos` with a monorepo containing nested `package.json` files, **When** the sandbox launches, **Then** all nested `node_modules/` directories within the bmad repo are isolated with named volumes.

5. **Given** `auto_isolate_deps` is enabled, **When** the scan completes, **Then** summary is logged: `"auto_isolate_deps: scanned N mount paths, found M package.json files"` — where N includes both primary mounts and bmad_repos, even if M is zero.

6. **Given** `auto_isolate_deps` is absent or `false`, **When** the sandbox launches, **Then** no scanning occurs, zero overhead.

7. **Given** the container starts with named volume mounts, **When** the entrypoint runs, **Then** it `chown`s the volume mount directories for the unprivileged sandbox user.

## Tasks / Subtasks

- [x] Task 1: Create `ScanDeps()` function in `internal/mount/isolate_deps.go` (AC: #1, #2, #3, #4)
  - [x] Implement `ScanDeps(cfg *config.Config) ([]ScanResult, error)` using `filepath.WalkDir`
  - [x] Walk each mount's host-side `Source` path for `package.json` files, excluding `node_modules/` subtrees
  - [x] For each `package.json` found: derive sibling `node_modules` path, compute container-side target, compute named volume name
  - [x] Return `[]ScanResult` with volume name, host path, and container target for each discovered dependency dir
  - [x] Skip scanning entirely when `cfg.AutoIsolateDeps` is false (return nil, nil immediately)
- [x] Task 2: Create `AssembleIsolateDeps()` function in `internal/mount/isolate_deps.go` (AC: #1, #2, #5)
  - [x] Convert `ScanResult` slice into Docker `-v` volume flags: `asbox-<project>-<path-dashed>-node_modules:<container-target>/node_modules`
  - [x] Compute `AUTO_ISOLATE_VOLUME_PATHS` env var value: comma-separated container-side paths for entrypoint chown
  - [x] Return `(volumeFlags []string, autoIsolateVolumePaths string)`
- [x] Task 3: Integrate into `cmd/run.go` (AC: #1, #2, #3, #5)
  - [x] After `mount.AssembleMounts(cfg)` call, call `mount.ScanDeps(cfg)` when `cfg.AutoIsolateDeps` is true
  - [x] Log scan summary: `"auto_isolate_deps: scanned N mount paths, found M package.json files"`
  - [x] Log each discovered mount: `"isolating: <container-target>/node_modules (volume: <volume-name>)"`
  - [x] Call `mount.AssembleIsolateDeps()` and append volume flags to `mountFlags`
  - [x] Set `AUTO_ISOLATE_VOLUME_PATHS` in `envVars` map
- [x] Task 4: Unit tests for `ScanDeps()` in `internal/mount/isolate_deps_test.go` (AC: #1, #2, #3, #4)
  - [x] Test: single mount with `package.json` at root returns one result with correct volume name and container target
  - [x] Test: monorepo with `package.json` at root and in `packages/api/` and `packages/web/` returns three results
  - [x] Test: `auto_isolate_deps: false` returns nil immediately
  - [x] Test: mount path with no `package.json` returns empty results
  - [x] Test: `node_modules/` subtrees are excluded from walk (nested `package.json` inside `node_modules/` ignored)
  - [x] Test: volume naming uses project name and dashed relative path
- [x] Task 5: Unit tests for `AssembleIsolateDeps()` in `internal/mount/isolate_deps_test.go` (AC: #1, #2, #5)
  - [x] Test: volume flags match expected `-v` format
  - [x] Test: `AUTO_ISOLATE_VOLUME_PATHS` is comma-separated container paths
  - [x] Test: empty scan results return empty flags and empty paths string

### Rework: Extend scan to bmad_repos (sprint-change-proposal-2026-04-10)

- [ ] Task 6: Extend `ScanDeps()` to scan `cfg.BmadRepos` paths (AC: #3, #4, #5)
  - [ ] After iterating `cfg.Mounts`, iterate `cfg.BmadRepos` entries
  - [ ] For each bmad repo: use host path as walk root, derive container target from `/workspace/repos/<basename>` convention (not from mount target config)
  - [ ] Reuse existing `buildVolumeName()` and `buildContainerPath()` — container path base is `/workspace/repos/<basename>` instead of `m.Target`
  - [ ] Include bmad_repos count in the scan summary log (N = primary mounts + bmad repos)
- [ ] Task 7: Unit tests for bmad_repos scanning in `internal/mount/isolate_deps_test.go` (AC: #3, #4, #5)
  - [ ] Test: single bmad repo with root `package.json` → volume mount with container path under `/workspace/repos/<basename>/node_modules`
  - [ ] Test: bmad repo with nested `package.json` files (monorepo) → multiple volume mounts with correct paths
  - [ ] Test: combined config with primary mounts AND bmad_repos → all `package.json` files discovered across both
  - [ ] Test: bmad_repos with no `package.json` → included in scan count N but contributes 0 to M
  - [ ] Test: volume naming for bmad repos uses project name and path relative to repo root
- [ ] Task 8: Update scan summary log in `cmd/run.go` (AC: #5)
  - [ ] Mount count N must include both `len(cfg.Mounts)` and `len(cfg.BmadRepos)`

### Review Findings

- [x] [Review][Patch] `buildVolumeName` produces Docker-invalid volume names from `@scope/` directories, spaces, and special characters in path components — Docker volume names only allow `[a-zA-Z0-9][a-zA-Z0-9_.-]` [internal/mount/isolate_deps.go:60-68] — fixed: added volumeNameRe sanitization + test
- [x] [Review][Defer] Comma in directory names breaks entrypoint `chown_volumes` IFS parsing — requires entrypoint change, out of scope per spec [internal/mount/isolate_deps.go:93, embed/entrypoint.sh:52] — deferred, requires entrypoint modification
- [x] [Review][Defer] Symlinked subdirectories in monorepos silently skipped by `filepath.WalkDir` — Go stdlib limitation, adding follow would risk cycles [internal/mount/isolate_deps.go:27] — deferred, known WalkDir behavior
- [x] [Review][Defer] Multiple mounts to same container target can produce duplicate volume entries — unusual config edge case [internal/mount/isolate_deps.go:26-56] — deferred, edge case
- [x] [Review][Defer] `project_name` special chars amplified to volume names — pre-existing, already tracked in deferred-work.md [internal/mount/isolate_deps.go:61] — deferred, pre-existing

## Dev Notes

### Architecture Constraints

- **Host-side scan only** — `ScanDeps()` runs on the host before Docker launch. It uses `filepath.WalkDir` to scan the host filesystem where mount sources reside.
- **Named Docker volumes, not bind mounts** — the `-v asbox-<name>:<target>` format creates a Docker-managed named volume. Docker/Podman manages the volume lifecycle. Volumes persist across sandbox restarts (FR9b).
- **Entrypoint chown already implemented** — `embed/entrypoint.sh:45-58` has `chown_volumes()` that reads `AUTO_ISOLATE_VOLUME_PATHS` env var (comma-separated container paths) and `chown -R sandbox:sandbox` each one. Do NOT modify entrypoint.sh.
- **No config validation needed** — `AutoIsolateDeps` is a boolean field already in the Config struct. No validation required beyond what YAML parsing provides.
- **Content hash unaffected** — auto_isolate_deps operates at runtime (Docker run flags), not at build time. The image is the same regardless of this setting. No hash computation changes needed.

### Volume Naming Convention

Pattern: `asbox-<project_name>-<relative-path-with-dashes>-node_modules`

Examples:
- Root `package.json` in mount targeting `/workspace`: `asbox-myapp-node_modules` (no path component for root)
- `packages/api/package.json` in mount targeting `/workspace`: `asbox-myapp-packages-api-node_modules`
- `packages/web/package.json` in mount targeting `/workspace`: `asbox-myapp-packages-web-node_modules`

The relative path is computed from the mount source root to the directory containing `package.json`, with `/` replaced by `-`.

### Container-Side Target Derivation

The container-side mount target for each volume is derived from:

**For primary mounts:**
1. The mount's `Target` (e.g., `/workspace`)
2. The relative path from mount `Source` to the `package.json` directory
3. Appending `/node_modules`

Example: mount `Source=/Users/me/project`, `Target=/workspace`, `package.json` at `Source/packages/api/package.json` → container target `/workspace/packages/api/node_modules`.

**For bmad_repos:**
1. The convention-based target `/workspace/repos/<basename>` (where `<basename>` is the directory name of the repo path)
2. The relative path from the repo root to the `package.json` directory
3. Appending `/node_modules`

Example: bmad_repo `/Users/me/repos/frontend`, `package.json` at root → container target `/workspace/repos/frontend/node_modules`.
Example: bmad_repo `/Users/me/repos/api`, `package.json` at `packages/core/package.json` → container target `/workspace/repos/api/packages/core/node_modules`.

### ScanDeps Implementation Approach

```go
// ScanResult represents a discovered dependency directory to isolate.
type ScanResult struct {
    VolumeName    string // e.g., "asbox-myapp-packages-api-node_modules"
    ContainerPath string // e.g., "/workspace/packages/api/node_modules"
}

// ScanDeps walks each mount's source path for package.json files and returns
// volume isolation targets. Returns nil, nil if AutoIsolateDeps is false.
func ScanDeps(cfg *config.Config) ([]ScanResult, error) {
    if !cfg.AutoIsolateDeps {
        return nil, nil
    }

    var results []ScanResult

    // Scan primary mounts
    for _, m := range cfg.Mounts {
        found, err := scanDir(m.Source, m.Target, cfg.ProjectName)
        if err != nil {
            return nil, fmt.Errorf("auto_isolate_deps: failed to scan %s: %w", m.Source, err)
        }
        results = append(results, found...)
    }

    // Scan bmad_repos — container target is /workspace/repos/<basename>
    for _, repoPath := range cfg.BmadRepos {
        basename := filepath.Base(repoPath)
        containerTarget := "/workspace/repos/" + basename
        found, err := scanDir(repoPath, containerTarget, cfg.ProjectName)
        if err != nil {
            return nil, fmt.Errorf("auto_isolate_deps: failed to scan bmad_repo %s: %w", repoPath, err)
        }
        results = append(results, found...)
    }

    return results, nil
}

// scanDir walks a directory for package.json files and returns ScanResults.
func scanDir(sourcePath, containerTarget, projectName string) ([]ScanResult, error) {
    var results []ScanResult
    err := filepath.WalkDir(sourcePath, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return nil // skip unreadable directories
        }
        if d.IsDir() && d.Name() == "node_modules" {
            return filepath.SkipDir
        }
        if d.IsDir() || d.Name() != "package.json" {
            return nil
        }

        dir := filepath.Dir(path)
        rel, _ := filepath.Rel(sourcePath, dir)

        volumeName := buildVolumeName(projectName, rel)
        containerPath := buildContainerPath(containerTarget, rel)

        results = append(results, ScanResult{
            VolumeName:    volumeName,
            ContainerPath: containerPath,
        })
        return nil
    })
    return results, err
}
```

### Volume Name Builder

```go
func buildVolumeName(projectName, relPath string) string {
    name := "asbox-" + projectName
    if relPath != "" && relPath != "." {
        // Replace path separators with dashes
        dashed := strings.ReplaceAll(relPath, string(filepath.Separator), "-")
        name += "-" + dashed
    }
    name += "-node_modules"
    return name
}
```

### AssembleIsolateDeps Implementation

```go
// AssembleIsolateDeps converts scan results into Docker volume flags and
// the AUTO_ISOLATE_VOLUME_PATHS env var value for entrypoint chown.
func AssembleIsolateDeps(results []ScanResult) (volumeFlags []string, autoIsolatePaths string) {
    if len(results) == 0 {
        return nil, ""
    }

    flags := make([]string, len(results))
    paths := make([]string, len(results))
    for i, r := range results {
        flags[i] = r.VolumeName + ":" + r.ContainerPath
        paths[i] = r.ContainerPath
    }
    return flags, strings.Join(paths, ",")
}
```

### Integration into cmd/run.go

After the existing `mount.AssembleMounts(cfg)` call (line 24), add:

```go
// Auto-isolate platform dependencies via named volumes
if cfg.AutoIsolateDeps {
    scanResults, err := mount.ScanDeps(cfg)
    if err != nil {
        return err
    }

    // Log scan summary (FR9c: always log when enabled, N = primary mounts + bmad repos)
    mountCount := len(cfg.Mounts) + len(cfg.BmadRepos)
    fmt.Fprintf(cmd.OutOrStdout(), "auto_isolate_deps: scanned %d mount paths, found %d package.json files\n", mountCount, len(scanResults))

    for _, r := range scanResults {
        fmt.Fprintf(cmd.OutOrStdout(), "isolating: %s (volume: %s)\n", r.ContainerPath, r.VolumeName)
    }

    if len(scanResults) > 0 {
        volumeFlags, autoIsolatePaths := mount.AssembleIsolateDeps(scanResults)
        mountFlags = append(mountFlags, volumeFlags...)
        envVars["AUTO_ISOLATE_VOLUME_PATHS"] = autoIsolatePaths
    }
}
```

### Logging (FR9c)

- **Always** log scan summary when `auto_isolate_deps: true`: `"auto_isolate_deps: scanned N mount paths, found M package.json files"` — even when M is 0
- **Per-mount** log for each discovered dependency: `"isolating: /workspace/node_modules (volume: asbox-myapp-node_modules)"`
- Log output goes to `cmd.OutOrStdout()` (same as "launching sandbox..." message)

### Entrypoint Chown (Already Done)

`embed/entrypoint.sh:45-58` has `chown_volumes()`:
```bash
chown_volumes() {
    local volume_paths="${AUTO_ISOLATE_VOLUME_PATHS:-}"
    if [[ -z "${volume_paths}" ]]; then
        return 0
    fi
    IFS=',' read -ra paths <<< "${volume_paths}"
    for path in "${paths[@]}"; do
        if [[ -d "${path}" ]]; then
            chown -R sandbox:sandbox "${path}"
        fi
    done
}
```

This expects `AUTO_ISOLATE_VOLUME_PATHS` as a comma-separated list of container-side paths. The env var is set in `cmd/run.go` and passed through `docker.RunOptions.EnvVars`.

### File Modifications Required

| File | Change |
|------|--------|
| `internal/mount/isolate_deps.go` | **New file**: `ScanResult` type, `ScanDeps()`, `AssembleIsolateDeps()`, `buildVolumeName()`, `buildContainerPath()` |
| `internal/mount/isolate_deps_test.go` | **New file**: table-driven tests for scan and assembly |
| `cmd/run.go` | Add auto_isolate_deps integration after line 24: scan, log, assemble, append to mounts + envVars |

### Testing Approach

**Unit tests** (`internal/mount/isolate_deps_test.go`) — use `t.TempDir()` with created `package.json` files to simulate project structures. Follow existing patterns from `mount_test.go`:

- Create temp dirs with `package.json` files at strategic locations
- Build `config.Config` with `AutoIsolateDeps: true`, `ProjectName`, and `Mounts` pointing to temp dirs
- Assert `ScanDeps()` returns expected `ScanResult` entries
- Assert `AssembleIsolateDeps()` returns correct volume flags and paths string

Key test cases:
- Single root `package.json` → 1 result
- Monorepo with 3 `package.json` files → 3 results
- `package.json` inside `node_modules/` → excluded (0 results from that subtree)
- `AutoIsolateDeps: false` → nil, nil immediately
- No `package.json` anywhere → empty results, no error
- Volume names use project name and dashed path
- `AUTO_ISOLATE_VOLUME_PATHS` matches comma-separated container paths

### Anti-Patterns to Avoid

- Do NOT create Docker volumes explicitly — the `-v name:/path` syntax causes Docker to create named volumes automatically on `docker run`
- Do NOT modify `embed/entrypoint.sh` — chown logic is already complete and tested
- Do NOT add this to the content hash — it's a runtime feature, not a build-time feature
- Do NOT validate `AutoIsolateDeps` in `internal/config/parse.go` — it's a bool, YAML handles it
- Do NOT scan inside `node_modules/` directories — skip them with `filepath.SkipDir` to avoid scanning potentially massive dependency trees
- Do NOT modify `internal/docker/run.go` — it already handles arbitrary mount flags via `opts.Mounts`
- Do NOT use `os/exec` or shell commands for scanning — pure Go `filepath.WalkDir`

### Previous Story Intelligence (5-1)

From story 5-1:
- **Config struct methods** follow the pattern of adding methods directly on `Config` (e.g., `HasMCP()`, `MCPManifestJSON()`). For this story, the logic belongs in `internal/mount/` not on Config, since it's filesystem scanning not config introspection.
- **Test patterns**: table-driven tests with `t.TempDir()`, `errors.As()` for error type checking
- **Integration point in cmd/run.go**: follows the pattern of computing flags/env vars before `docker.RunContainer(opts)` call
- **Commit convention**: `feat: implement story X-Y description`

### Git Intelligence

Recent commits (10 stories) follow `feat: implement story X-Y description` format. Single commits per story, co-authored with Claude Opus 4.6. Tests alongside implementation. Go 1.25.0, testcontainers-go v0.41.0.

### Project Structure Notes

- `internal/mount/isolate_deps.go` — new file in existing package, follows architecture specification
- `internal/mount/isolate_deps_test.go` — new test file in existing package
- `cmd/run.go` — minimal additions after line 24 (mount assembly)
- All changes align with architecture's file responsibility map

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Epic 6, Story 6.1 (updated 2026-04-10 with bmad_repos criteria)]
- [Source: _bmad-output/planning-artifacts/architecture.md — Automatic Dependency Isolation section (updated 2026-04-10)]
- [Source: _bmad-output/planning-artifacts/architecture.md — File structure: internal/mount/ (lines 466-472)]
- [Source: _bmad-output/planning-artifacts/prd.md — FR9a, FR9b (updated 2026-04-10), FR9c, FR16a]
- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-04-10.md — course correction for bmad_repos + auto_isolate_deps]
- [Source: internal/mount/mount.go — AssembleMounts() pattern]
- [Source: internal/mount/bmad_repos.go — AssembleBmadRepos() for container path convention]
- [Source: internal/mount/isolate_deps.go — existing ScanDeps implementation to extend]
- [Source: internal/mount/isolate_deps_test.go — existing tests to extend]
- [Source: cmd/run.go — integration point after mount assembly]
- [Source: internal/docker/run.go — mount flags passed as -v]
- [Source: embed/entrypoint.sh:45-58 — chown_volumes() already complete, no changes needed]
- [Source: internal/config/config.go — AutoIsolateDeps bool, BmadRepos []string fields]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

### Completion Notes List

- Implemented `ScanDeps()` with `filepath.WalkDir` — scans each mount's host-side source for `package.json` files, skips `node_modules/` subtrees, computes volume names and container paths
- Implemented `AssembleIsolateDeps()` — converts scan results into Docker `-v` volume flags and `AUTO_ISOLATE_VOLUME_PATHS` comma-separated env var
- Helper functions `buildVolumeName()` and `buildContainerPath()` handle naming convention and path normalization
- Integrated into `cmd/run.go` — scan runs after `AssembleMounts()`, logs summary and per-mount details, appends volume flags and sets env var
- 8 unit tests covering: disabled config, single root package.json, monorepo with 3 package.json files, no package.json, node_modules exclusion, volume naming with dashed paths, volume flag format, and empty results
- All tests pass (8/8), full regression suite green, go vet clean

### File List

- `internal/mount/isolate_deps.go` (new) — ScanResult type, ScanDeps(), AssembleIsolateDeps(), buildVolumeName(), buildContainerPath()
- `internal/mount/isolate_deps_test.go` (new) — 8 unit tests for ScanDeps and AssembleIsolateDeps
- `cmd/run.go` (modified) — auto_isolate_deps integration after mount assembly

### Change Log

- 2026-04-09: Implemented story 6-1 — auto dependency isolation via named volumes. Added ScanDeps/AssembleIsolateDeps functions, integrated into run command, 8 unit tests all passing.
