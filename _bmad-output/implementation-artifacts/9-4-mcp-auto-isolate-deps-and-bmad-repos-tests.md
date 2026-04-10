# Story 9.4: MCP, Auto-Isolate-Deps, and BMAD Repos Tests

Status: done

## Story

As a developer,
I want integration tests for MCP configuration, auto dependency isolation, and multi-repo mounts,
So that advanced features are validated automatically.

## Acceptance Criteria

1. **MCP Configuration Validation**
   ```
   GIVEN a test sandbox built with `mcp: [playwright]`
   WHEN inspecting the container
   THEN `/etc/sandbox/mcp-servers.json` exists with Playwright entry
     AND `.mcp.json` is generated in the workspace
   ```

2. **Auto-Isolate-Deps Volume Creation**
   ```
   GIVEN a test config with `auto_isolate_deps: true` and a project with `package.json`
   WHEN the sandbox launches
   THEN a named volume mount is created for `node_modules/`
   ```

3. **BMAD Repos Multi-Repo Mount**
   ```
   GIVEN a test config with `bmad_repos` pointing to two test directories
   WHEN the sandbox launches
   THEN repos are mounted at `/workspace/repos/<name>` and agent instruction file is present in the container
   ```

## Tasks / Subtasks

- [x] Task 1: Create `integration/mcp_test.go` (AC: #1)
  - [x] 1.1 Build image with `MCP: []string{"playwright"}` and `SDKs.NodeJS: "20"` via `buildTestImageWithConfig`
  - [x] 1.2 Start container with `startTestContainerWithEntrypoint` (needs real entrypoint for MCP merge)
  - [x] 1.3 Test: `/etc/sandbox/mcp-servers.json` exists and contains `"playwright"` entry with `"npx"` command
  - [x] 1.4 Test: `/home/sandbox/.mcp.json` exists after entrypoint runs (merge output)
  - [x] 1.5 Test: `.mcp.json` contains `"playwright"` key in `mcpServers` object

- [x] Task 2: Create `integration/isolate_deps_test.go` (AC: #2)
  - [x] 2.1 Build the `asbox` binary into temp dir for CLI invocation
  - [x] 2.2 Create temp project directory with `package.json` and a minimal `.asbox/config.yaml` with `auto_isolate_deps: true`
  - [x] 2.3 Test: Run `asbox run` (with timeout/early kill), capture stdout, verify it logs `auto_isolate_deps: scanned N mount paths, found M package.json files`
  - [x] 2.4 Test: Verify stdout contains `isolating:` line with volume name matching `asbox-*-node_modules` pattern
  - [x] 2.5 Test: Verify monorepo scenario — create `projectDir/packages/api/package.json` and `projectDir/packages/web/package.json`, verify output contains multiple `isolating:` lines (one per package.json)

- [x] Task 3: Create `integration/bmad_repos_test.go` (AC: #3)
  - [x] 3.1 Create two temp directories as mock repos (each with at least one file)
  - [x] 3.2 Build the `asbox` binary into temp dir
  - [x] 3.3 Write config with `bmad_repos` pointing to the two temp directories
  - [x] 3.4 Test: Run `asbox run`, capture stdout, verify `bmad_repos: mounting 2 repositories` log line
  - [x] 3.5 Test: Verify the run command assembles mount flags with `/workspace/repos/<basename>` targets
  - [x] 3.6 Note: Instruction file *content* is already covered by unit tests in `internal/mount/bmad_repos_test.go`. The integration test verifies the CLI-level plumbing (log output, mount assembly) — not template rendering.

- [x] Task 4: Run full integration test suite and verify no regressions
  - [x] 4.1 `go test -v -count=1 ./integration/...` passes all existing + new tests
  - [x] 4.2 `go vet ./integration/...` passes

## Dev Notes

### Critical Implementation Context

**Container Pattern Selection:**
- **AC #1 (MCP):** MUST use `startTestContainerWithEntrypoint` because MCP manifest merge runs in `entrypoint.sh`. The `tail -f /dev/null` pattern skips the entrypoint entirely, so `/home/sandbox/.mcp.json` would never be created.
- **AC #2 (Auto-Isolate-Deps):** This is a CLI-level feature that runs in `cmd/run.go` BEFORE the container starts. The volume flags and log output are produced by the Go binary, not inside the container. Test by invoking the compiled binary and checking stdout/stderr, similar to `TestSecrets_missingSecretExitsCode4` in `mount_test.go`.
- **AC #3 (BMAD Repos):** Same as AC #2 — mount assembly and instruction generation happen in `cmd/run.go` before Docker run. Test via compiled binary invocation.

**Key Code Paths:**
| Feature | Config Field | Code Entry Point | What Produces the Tested Output |
|---------|-------------|-----------------|-------------------------------|
| MCP manifest | `MCP: []string{"playwright"}` | `template.Render(cfg)` → Dockerfile `COPY` | Build-time: `/etc/sandbox/mcp-servers.json` baked into image |
| MCP merge | — | `entrypoint.sh:merge_mcp_config()` | Runtime: `/home/sandbox/.mcp.json` written by entrypoint |
| Auto-isolate | `AutoIsolateDeps: true` | `cmd/run.go` → `mount.ScanDeps(cfg)` | CLI stdout: `auto_isolate_deps: scanned...` |
| BMAD repos | `BmadRepos: []string{...}` | `cmd/run.go` → `mount.AssembleBmadRepos(cfg)` | CLI stdout: `bmad_repos: mounting N repositories` |

**MCP Build Prerequisite:** Building with `MCP: ["playwright"]` requires `SDKs.NodeJS` to be set (Playwright needs npm/npx). The config validator in `parse.go` enforces this. Use `SDKs: config.SDKConfig{NodeJS: "20"}` in the test config.

### Architecture Compliance

**Test File Locations (per epics spec):**
- `integration/mcp_test.go` — MCP manifest presence and merge verification
- `integration/isolate_deps_test.go` — auto_isolate_deps CLI output verification
- `integration/bmad_repos_test.go` — bmad_repos CLI output and mount assembly verification

**Note:** The architecture doc lists auto_isolate_deps and bmad_repos tests under `mount_test.go`, but the epics override this with dedicated files per feature. Follow the epics — separate files provide cleaner test organization.

**Naming Convention:** `TestFeature_scenario` (e.g., `TestMCP_manifestExistsWithPlaywrightEntry`, `TestAutoIsolateDeps_logsVolumeCreation`)

**Parallel Execution:** All test functions must include `if testing.Short() { t.Skip("...") }` guard. Independent subtests must use `t.Parallel()`.

### Library/Framework Requirements

- Use ONLY Go stdlib `testing` package — no `testify/assert`
- Use `testcontainers-go` v0.41.0 for container lifecycle (already in go.mod)
- Use `tcexec.Multiplexed()` only with `fileContentInContainer` — `execInContainer` returns raw stream, assert with `strings.Contains`
- Use `wait.ForExec([]string{"true"})` for container readiness (not `time.Sleep`)
- For binary invocation tests: `exec.Command`, `cmd.Dir`, `cmd.Env`, `cmd.CombinedOutput()` — same pattern as `TestSecrets_missingSecretExitsCode4`

### File Structure Requirements

```
integration/
├── integration_test.go          # Existing shared helpers — DO NOT MODIFY
├── lifecycle_test.go            # Existing — DO NOT MODIFY
├── mount_test.go                # Existing — DO NOT MODIFY
├── isolation_test.go            # Existing — DO NOT MODIFY
├── inner_container_test.go      # Existing — DO NOT MODIFY
├── podman_test.go               # Existing — DO NOT MODIFY
├── mcp_test.go                  # NEW: MCP manifest and merge tests
├── isolate_deps_test.go         # NEW: auto_isolate_deps CLI output tests
├── bmad_repos_test.go           # NEW: bmad_repos mounting and instruction tests
└── testdata/
    ├── config.yaml              # Existing minimal fixture
    ├── docker-compose.yml       # Existing
    └── project/package.json     # Existing — reusable for auto_isolate_deps
```

### Testing Requirements

**AC #1 — MCP Test Strategy:**
```go
func TestMCP_manifestExistsWithPlaywrightEntry(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }
    // Build image with MCP + NodeJS SDK
    cfg := &config.Config{
        Agent:       "claude-code",
        ProjectName: "integration-test",
        MCP:         []string{"playwright"},
        SDKs:        config.SDKConfig{NodeJS: "20"},
    }
    image := buildTestImageWithConfig(t, cfg)
    ctx := context.Background()
    
    // MUST use real entrypoint for MCP merge
    container := startTestContainerWithEntrypoint(ctx, t, image)
    
    t.Run("build_time_manifest_exists", func(t *testing.T) {
        t.Parallel()
        content := fileContentInContainer(ctx, t, container, "/etc/sandbox/mcp-servers.json")
        // Verify Playwright entry
        if !strings.Contains(content, `"playwright"`) {
            t.Errorf("manifest missing playwright entry: %s", content)
        }
        if !strings.Contains(content, `"npx"`) {
            t.Errorf("manifest missing npx command: %s", content)
        }
    })
    
    t.Run("runtime_mcp_json_generated", func(t *testing.T) {
        t.Parallel()
        if !fileExistsInContainer(ctx, t, container, "/home/sandbox/.mcp.json") {
            t.Error("expected /home/sandbox/.mcp.json after entrypoint merge")
        }
    })
    
    t.Run("merged_config_contains_playwright", func(t *testing.T) {
        t.Parallel()
        content := fileContentInContainer(ctx, t, container, "/home/sandbox/.mcp.json")
        if !strings.Contains(content, `"playwright"`) {
            t.Errorf("merged .mcp.json missing playwright: %s", content)
        }
    })
}
```

**AC #2 — Auto-Isolate-Deps Test Strategy:**

The auto_isolate_deps feature runs in the Go CLI (`cmd/run.go`) BEFORE Docker. It scans host-side directories for `package.json` and logs volume creation. Testing approach: build the `asbox` binary and invoke it with a controlled config, then assert on its stdout. The `asbox run` command will fail (since we're not in a real Docker environment or we interrupt early), but the log output appears before the Docker run call.

**Important:** The `ScanDeps` function walks real host-side directories. Create temp dirs with `package.json` files, write a config YAML pointing mounts at those temp dirs, then run `asbox run -f <config>`. The scan + log happens before Docker run starts. Note: `auto_isolate_deps: true` alone does nothing if there are no `mounts` or `bmad_repos` entries — the scan needs directories to walk.

Pattern (similar to `TestSecrets_missingSecretExitsCode4` in `mount_test.go`):
```go
func TestAutoIsolateDeps_logsVolumeCreation(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }
    // Build binary
    binDir := t.TempDir()
    binPath := filepath.Join(binDir, "asbox")
    buildCmd := exec.Command("go", "build", "-o", binPath, ".")
    buildCmd.Dir = ".."  // project root (integration/ is CWD)
    if out, err := buildCmd.CombinedOutput(); err != nil {
        t.Fatalf("build failed: %v\n%s", err, out)
    }

    // Create project with package.json
    projectDir := t.TempDir()
    os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(`{"name":"test"}`), 0644)

    // Write config with auto_isolate_deps + mount pointing to projectDir
    configDir := t.TempDir()
    configPath := filepath.Join(configDir, "config.yaml")
    configContent := fmt.Sprintf(`agent: claude-code
project_name: test-isolate
auto_isolate_deps: true
mounts:
  - source: %s
    target: /workspace
`, projectDir)
    os.WriteFile(configPath, []byte(configContent), 0644)

    // Run — will fail on Docker run, but scan logs appear first
    cmd := exec.Command(binPath, "run", "-f", configPath)
    output, _ := cmd.CombinedOutput()  // ignore error — Docker run will fail
    
    outStr := string(output)
    if !strings.Contains(outStr, "auto_isolate_deps: scanned") {
        t.Errorf("missing scan summary in output: %s", outStr)
    }
    if !strings.Contains(outStr, "isolating:") {
        t.Errorf("missing isolation line in output: %s", outStr)
    }
}
```

**AC #3 — BMAD Repos Test Strategy:**

Same binary-invocation pattern as AC #2. Create two temp directories, write config with `bmad_repos` pointing to them, invoke `asbox run`, check stdout for `bmad_repos: mounting 2 repositories`.

For instruction file content verification, use the unit tests already in `internal/mount/bmad_repos_test.go`. The integration test focuses on the CLI-level log output confirming mount assembly.

### Previous Story Intelligence

**From Story 9-3 (Isolation and Inner Container Tests):**
- `startTestContainerWithEntrypoint` is defined in `podman_test.go` (line 22-51), NOT in `integration_test.go`. It uses `Privileged: true` and waits for Podman socket.
- Subtests within `TestIsolation` are parallel; subtests within `TestDockerBuild` are sequential (dependencies).
- Image built once per top-level test function.

**From Story 9-2 (Lifecycle and Mount Tests):**
- Binary invocation pattern for CLI tests: `exec.Command("go", "build", "-o", binPath, ".")` with `cmd.Dir = ".."` (relative to integration/).
- Env filtering for controlled environment: `os.Environ()` loop with `strings.HasPrefix` exclusion.
- Exit code checking: `errors.As(err, &exitErr)` then `exitErr.ExitCode()`.

**From Story 9-1 (Infrastructure):**
- `buildTestImageWithConfig` accepts `*config.Config` — use for MCP tests with custom SDKs/MCP fields.
- `fileContentInContainer` uses `tcexec.Multiplexed()` — safe for exact JSON comparisons.
- `fileExistsInContainer` returns bool without failing — use for existence checks.
- Nanosecond image tags prevent collision in parallel tests.

**Deferred Issues (DO NOT FIX):**
- `execInContainer` missing `tcexec.Multiplexed()` — use `strings.Contains` for output assertions (deferred from 9-1)
- Cleanup closures capture caller's `ctx` — all callers use `context.Background()` (deferred)
- No context timeout on integration tests (deferred)

### Anti-Patterns to Avoid

- Do NOT import `testify/assert` — project uses stdlib `testing` only
- Do NOT modify existing test files from stories 9-1/9-2/9-3
- Do NOT add new helpers to `integration_test.go` — use existing ones
- Do NOT use `time.Sleep` — use `wait.ForExec` or retry loops
- Do NOT use `os.Setenv` in test process — use `cmd.Env` for binary invocations
- Do NOT add `//go:embed` — use `os.ReadFile` or inline test data
- Do NOT create a separate `testhelpers` package
- Do NOT test auto_isolate_deps or bmad_repos inside the container — these features execute in the CLI before Docker

### Git Intelligence

Recent commits show single-commit-per-story pattern:
```
cd8642a feat: implement story 9-3 isolation and inner container test validation
2cf4b37 feat: implement story 9-2 lifecycle and mount integration tests
5311a19 feat: implement story 9-1 integration test infrastructure
```

Relevant feature implementations:
```
9d8b688 feat: implement story 5-1 MCP server installation and configuration merge
45c4a36 feat: Story 6.1 Auto Dependency Isolation Via Named Volumes
3872206 feat: implement story 8-1 bmad multi-repo mounts and agent instructions
903c533 chore: course correction — extend auto_isolate_deps to scan bmad_repos paths
```

### Project Structure Notes

- All integration tests in `integration/` package (separate from unit tests in `internal/`)
- Test execution: `go test -v -count=1 ./integration/...`
- CI: Integration tests run on PRs only (`.github/workflows/ci.yml`)
- Makefile: `make test-integration` runs integration suite

### Key Source References

- [Source: internal/config/config.go] — `Config` struct, `MCPServerRegistry`, `HasMCP()`, `MCPManifestJSON()`
- [Source: internal/mount/isolate_deps.go] — `ScanDeps()`, `ScanResult`, `AssembleIsolateDeps()`, `buildVolumeName()`
- [Source: internal/mount/bmad_repos.go] — `AssembleBmadRepos()`, `BmadRepoInfo`, `InstructionData`, collision detection
- [Source: cmd/run.go] — CLI-level assembly: mount flags, env vars, scan logging, instruction file generation
- [Source: embed/entrypoint.sh:60-84] — `merge_mcp_config()` function: merge logic for `/etc/sandbox/mcp-servers.json` + `/workspace/.mcp.json` → `/home/sandbox/.mcp.json`
- [Source: internal/template/render.go] — Dockerfile template rendering including MCP blocks
- [Source: integration/integration_test.go] — All shared helpers (10 functions)
- [Source: integration/podman_test.go:22-51] — `startTestContainerWithEntrypoint` helper
- [Source: integration/mount_test.go:120-184] — Binary invocation pattern for CLI-level testing

### FR Coverage Matrix

| FR | Description | Test Approach |
|----|-------------|---------------|
| FR3 | MCP configuration | `mcp_test.go`: build with `MCP: ["playwright"]`, verify manifest |
| FR41 | MCP servers installed at build time | `mcp_test.go`: `/etc/sandbox/mcp-servers.json` exists in container |
| FR46 | MCP manifest merge at runtime | `mcp_test.go`: `/home/sandbox/.mcp.json` after entrypoint |
| FR9a | auto_isolate_deps config option | `isolate_deps_test.go`: CLI output with `auto_isolate_deps: true` |
| FR9b | Scan and create volumes for node_modules | `isolate_deps_test.go`: log output shows volume creation |
| FR16a | Volume mounts assembled at launch | `isolate_deps_test.go`: `isolating:` log lines in CLI output |
| FR51 | bmad_repos config option | `bmad_repos_test.go`: CLI accepts `bmad_repos` list |
| FR52 | Auto-mount repos to /workspace/repos/<name> | `bmad_repos_test.go`: `mounting N repositories` log line |
| FR53 | Generated agent instruction file | `bmad_repos_test.go`: instruction content verification |

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6 (1M context)

### Debug Log References
- All tests passed on first run — no debug issues encountered
- MCP test: image build with NodeJS 20 + Playwright MCP took ~120s (includes npm install)
- Auto-isolate-deps and BMAD repos tests: ~0.5s each (binary invocation, no container needed)
- Full suite regression: 74s, 16 test functions, 0 failures

### Completion Notes List
- Task 1: Created `integration/mcp_test.go` with `TestMCP_manifestExistsWithPlaywrightEntry` — 3 subtests verify build-time manifest, runtime merge, and merged config content. Uses `startTestContainerWithEntrypoint` for real entrypoint execution.
- Task 2: Created `integration/isolate_deps_test.go` with `TestAutoIsolateDeps_logsVolumeCreation` — 2 subtests verify single package.json detection and monorepo scenario with multiple package.json files. Uses binary invocation pattern.
- Task 3: Created `integration/bmad_repos_test.go` with `TestBmadRepos_mountsAndInstructions` — 2 subtests verify CLI log output for mounting repositories and mount flag assembly with named basenames. Uses binary invocation pattern.
- Task 4: Full regression suite passed — 16 test functions, 0 failures, `go vet` clean.

### File List
- `integration/mcp_test.go` (NEW) — MCP manifest and merge integration tests
- `integration/isolate_deps_test.go` (NEW) — auto_isolate_deps CLI output integration tests
- `integration/bmad_repos_test.go` (NEW) — BMAD repos mounting integration tests
- `_bmad-output/implementation-artifacts/9-4-mcp-auto-isolate-deps-and-bmad-repos-tests.md` (MODIFIED) — Story status and task tracking
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (MODIFIED) — Sprint status update

### Review Findings

- [x] [Review][Decision] `mount_flags_contain_workspace_repos_targets` doesn't verify actual mount paths — resolved: added per-mount logging to `cmd/run.go` and updated test to assert on `/workspace/repos/frontend` and `/workspace/repos/backend`.
- [x] [Review][Patch] `merged_config_contains_playwright` should also verify `"mcpServers"` key presence [integration/mcp_test.go:46] — resolved: added `"mcpServers"` assertion.

## Change Log
- 2026-04-10: Implemented story 9-4 — added integration tests for MCP configuration validation, auto-isolate-deps CLI output verification, and BMAD repos mount assembly verification. 3 new test files, 7 subtests total, all passing with zero regressions.
