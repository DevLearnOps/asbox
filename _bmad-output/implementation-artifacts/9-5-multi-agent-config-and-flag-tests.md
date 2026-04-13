# Story 9.5: Multi-Agent Config and Flag Tests

Status: done

## Story

As a developer,
I want integration tests for the multi-agent configuration, --agent flag override, and boolean host_agent_config,
So that agent switching and config resolution are validated automatically.

## Acceptance Criteria

1. **Installed Agents Available in Image**
   ```
   GIVEN a test config with `installed_agents: [claude, gemini]`
   WHEN the sandbox image is built
   THEN both `claude` and `gemini` CLI commands are available inside the container
   ```

2. **Default Agent Command Resolution**
   ```
   GIVEN a test config with `installed_agents: [claude, gemini]` and `default_agent: claude`
   WHEN the sandbox launches without `--agent` flag
   THEN the agent command is `claude --dangerously-skip-permissions`
   ```

3. **Agent Flag Override**
   ```
   GIVEN a test config with `installed_agents: [claude, gemini]`
   WHEN the sandbox launches with `--agent gemini`
   THEN the agent command is `gemini -y`
   ```

4. **Agent Not Installed Rejection**
   ```
   GIVEN a test config with `installed_agents: [claude]`
   WHEN `--agent gemini` is passed
   THEN the CLI exits with code 1 with a clear error about gemini not being installed
   ```

5. **Host Agent Config Enabled**
   ```
   GIVEN a test config with `host_agent_config: true` (or default)
   WHEN the sandbox launches with a valid host agent config directory
   THEN the directory is mounted and the appropriate env var is set
   ```

6. **Host Agent Config Disabled**
   ```
   GIVEN a test config with `host_agent_config: false`
   WHEN the sandbox launches
   THEN no host agent config mount is present
   ```

## Tasks / Subtasks

- [x] Task 1: Create `integration/multi_agent_test.go` — container-based tests (AC: #1)
  - [x] 1.1 Build multi-agent image with `InstalledAgents: []string{"claude", "gemini"}, SDKs: config.SDKConfig{NodeJS: "22"}` via `buildTestImageWithConfig`
  - [x] 1.2 Start container with `startTestContainer` (tail -f /dev/null)
  - [x] 1.3 Test: `which claude` returns exit code 0 (claude CLI available)
  - [x] 1.4 Test: `which gemini` returns exit code 0 (gemini CLI available)
  - [x] 1.5 Test: `/home/sandbox/CLAUDE.md` exists (instruction file for claude)
  - [x] 1.6 Test: `/home/sandbox/GEMINI.md` exists (instruction file for gemini)

- [x] Task 2: Create CLI binary tests for --agent flag validation (AC: #4)
  - [x] 2.1 Build the `asbox` binary into temp dir
  - [x] 2.2 Write config: `installed_agents: [claude]`, `project_name: test-multi-agent`
  - [x] 2.3 Test: `asbox run --agent gemini -f <config>` exits code 1, stderr contains `"not installed"` and `"gemini"`
  - [x] 2.4 Test: `asbox run --agent invalidname -f <config>` exits code 1, stderr contains `"unsupported agent"`
  - [x] 2.5 Test: `asbox run --agent claude-code -f <config>` exits code 1, stderr contains `"unsupported agent"` (old-style name rejected)

- [x] Task 3: Create CLI binary test for host_agent_config disabled (AC: #6)
  - [x] 3.1 Write config: `installed_agents: [claude]`, `host_agent_config: false`
  - [x] 3.2 Test: `asbox run -f <config>` output does NOT contain `"host_agent_config"` error — command proceeds past config/mount validation (will fail at docker build/run, which is expected)

- [x] Task 4: Run full integration test suite and verify no regressions
  - [x] 4.1 `go test -v -count=1 ./integration/...` passes all existing + new tests
  - [x] 4.2 `go vet ./integration/...` passes

## Dev Notes

### Critical Implementation Context

**Test Approach by AC:**

| AC | Test Type | Why |
|----|-----------|-----|
| #1 (both CLIs in image) | Container-based via `buildTestImageWithConfig` + `startTestContainer` | Need a running container to verify installed binaries |
| #2 (default agent command) | Covered by unit tests | `agentCommand()` in `cmd/run_test.go` + `default_agent` resolution in `parse_test.go` already validate this |
| #3 (--agent override) | Covered by unit tests | `ValidateAgent()` + `ValidateAgentInstalled()` in `parse_test.go` validate the validators; the wiring in `run.go` is exercised by AC #4's integration test (error path proves the flag is read) |
| #4 (uninstalled agent rejected) | Binary invocation | Tests the full CLI flow: config parse → flag read → validation → error with exit code 1 |
| #5 (host_agent_config enabled) | Covered by unit tests | `AssembleHostAgentConfig()` in `internal/mount/mount_test.go` thoroughly tests enabled/disabled/missing-dir/silent-skip behavior. Integration verification depends on `~/.claude` or `~/.gemini` existing on host, which is not reliable |
| #6 (host_agent_config disabled) | Binary invocation | Verifies full CLI flow doesn't error when host_agent_config is explicitly disabled |

**Why Not Integration Test AC #2, #3, and #5 Directly:**
- `AGENT_CMD` env var is passed to `docker run` at runtime by `cmd/run.go` — it is NOT baked into the image and NOT logged to stdout. There is no observable output to assert against before docker run starts.
- The `agentCommand()` function and `--agent` flag override logic are straightforward and fully unit-tested.
- `host_agent_config` mount assembly depends on host filesystem state (`~/.claude` or `~/.gemini` existing). Unit tests in `mount_test.go` use temp dirs to test all paths.
- The deferred work item from story 1-9 review ("No cmd-level tests for `--agent` flag path") is addressed by Task 2's error-path tests, which prove the flag is wired up and validation runs.

**Container Build for Multi-Agent Image:**
Building with `InstalledAgents: [claude, gemini]` requires `SDKs.NodeJS` to be set because gemini is installed via `npm install -g @google/gemini-cli`. The config validator in `parse.go:105` enforces this: `slices.Contains(cfg.InstalledAgents, "gemini") && cfg.SDKs.NodeJS == ""` → error.

**Exit Code Mapping for Binary Tests:**
From `cmd/root.go:50-71`:
- `*config.ConfigError` → exit code 1
- `*config.SecretError` → exit code 4
- `*docker.DependencyError` → exit code 3
- `*docker.RunError`, `*docker.BuildError` → exit code 1
- Errors are printed to stderr: `fmt.Fprintf(os.Stderr, "error: %s\n", err)`
- Cobra errors silenced: `SilenceErrors: true`, `SilenceUsage: true`

Since both config errors and docker errors map to exit code 1, binary tests must assert on **error message content** (not just exit code) to distinguish config validation failures from docker failures.

**Binary Invocation Pattern:**
For Task 2/3, the `asbox run` flow when `--agent` validation fails:
1. `PersistentPreRunE`: checks `docker` binary exists → passes (docker available in test env)
2. `RunE`: `config.Parse(configPath)` → success (config is valid)
3. `RunE`: reads `--agent` flag → e.g., "gemini"
4. `RunE`: `ValidateAgent("gemini")` → pass (valid short name)
5. `RunE`: `ValidateAgentInstalled("gemini", ["claude"])` → returns `*ConfigError`
6. `Execute()`: `fmt.Fprintf(os.Stderr, "error: %s\n", err)` → prints to stderr
7. `os.Exit(1)`

For valid overrides, the flow continues past validation into docker build/run, which will eventually fail. But the absence of config-error keywords in output proves validation passed.

### Architecture Compliance

**Test File Location (per epics spec):**
- `integration/multi_agent_test.go` — all multi-agent integration tests in one file

**Naming Convention:** `TestFeature_scenario` (e.g., `TestMultiAgent_bothAgentsInstalledInImage`, `TestMultiAgent_uninstalledAgentExitsCode1`)

**Parallel Execution:** All test functions must include `if testing.Short() { t.Skip("...") }` guard. Independent subtests must use `t.Parallel()`.

### Library/Framework Requirements

- Use ONLY Go stdlib `testing` package — no `testify/assert`
- Use `testcontainers-go` v0.41.0 for container lifecycle (already in go.mod)
- Use `tcexec.Multiplexed()` only with `fileContentInContainer` — `execInContainer` returns raw stream, assert with `strings.Contains`
- Use `wait.ForExec([]string{"true"})` for container readiness (not `time.Sleep`)
- For binary invocation tests: `exec.Command`, `cmd.Dir`, `cmd.Env`, `cmd.CombinedOutput()` — same pattern as `TestSecrets_missingSecretExitsCode4` in `mount_test.go`

### File Structure Requirements

```
integration/
├── integration_test.go          # Existing shared helpers — DO NOT MODIFY
├── lifecycle_test.go            # Existing — DO NOT MODIFY
├── mount_test.go                # Existing — DO NOT MODIFY
├── isolation_test.go            # Existing — DO NOT MODIFY
├── inner_container_test.go      # Existing — DO NOT MODIFY
├── podman_test.go               # Existing — DO NOT MODIFY
├── mcp_test.go                  # Existing — DO NOT MODIFY
├── isolate_deps_test.go         # Existing — DO NOT MODIFY
├── bmad_repos_test.go           # Existing — DO NOT MODIFY
├── multi_agent_test.go          # NEW: multi-agent config, flag, and host config tests
└── testdata/
    └── config.yaml              # Existing fixture — DO NOT MODIFY
```

### Testing Requirements

**Task 1 — Container-Based Multi-Agent Image Test:**
```go
func TestMultiAgent_bothAgentsInstalledInImage(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }
    // Build multi-agent image — gemini requires NodeJS
    cfg := &config.Config{
        InstalledAgents: []string{"claude", "gemini"},
        ProjectName:     "integration-test",
        SDKs:            config.SDKConfig{NodeJS: "22"},
    }
    image := buildTestImageWithConfig(t, cfg)
    ctx := context.Background()
    container := startTestContainer(ctx, t, image)

    t.Run("claude_cli_available", func(t *testing.T) {
        t.Parallel()
        _, exitCode := execInContainer(ctx, t, container, []string{"which", "claude"})
        if exitCode != 0 {
            t.Error("claude CLI not found in container")
        }
    })

    t.Run("gemini_cli_available", func(t *testing.T) {
        t.Parallel()
        _, exitCode := execInContainer(ctx, t, container, []string{"which", "gemini"})
        if exitCode != 0 {
            t.Error("gemini CLI not found in container")
        }
    })

    t.Run("claude_instruction_file_exists", func(t *testing.T) {
        t.Parallel()
        if !fileExistsInContainer(ctx, t, container, "/home/sandbox/CLAUDE.md") {
            t.Error("expected /home/sandbox/CLAUDE.md to exist")
        }
    })

    t.Run("gemini_instruction_file_exists", func(t *testing.T) {
        t.Parallel()
        if !fileExistsInContainer(ctx, t, container, "/home/sandbox/GEMINI.md") {
            t.Error("expected /home/sandbox/GEMINI.md to exist")
        }
    })
}
```

**Task 2 — Binary Invocation Agent Flag Tests:**
```go
func TestMultiAgent_agentFlagValidation(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }

    // Build binary once for all subtests
    tmpDir := t.TempDir()
    binaryPath := filepath.Join(tmpDir, "asbox")
    buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
    buildCmd.Dir = ".." // project root
    if out, err := buildCmd.CombinedOutput(); err != nil {
        t.Fatalf("building asbox binary: %v\noutput: %s", err, out)
    }

    // Config with only claude installed
    configDir := t.TempDir()
    configPath := filepath.Join(configDir, "config.yaml")
    configContent := "installed_agents:\n  - claude\nproject_name: test-multi-agent\n"
    if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
        t.Fatalf("writing config: %v", err)
    }

    t.Run("uninstalled_agent_exits_code_1", func(t *testing.T) {
        t.Parallel()
        cmd := exec.Command(binaryPath, "run", "--agent", "gemini", "-f", configPath)
        output, err := cmd.CombinedOutput()
        outStr := string(output)

        if err == nil {
            t.Fatal("expected non-zero exit, got nil error")
        }
        var exitErr *exec.ExitError
        if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
            t.Errorf("expected exit code 1, got %v", err)
        }
        if !strings.Contains(outStr, "not installed") {
            t.Errorf("expected 'not installed' in output:\n%s", outStr)
        }
    })

    t.Run("invalid_agent_name_exits_code_1", func(t *testing.T) {
        t.Parallel()
        cmd := exec.Command(binaryPath, "run", "--agent", "invalidname", "-f", configPath)
        output, err := cmd.CombinedOutput()
        outStr := string(output)

        if err == nil {
            t.Fatal("expected non-zero exit, got nil error")
        }
        if !strings.Contains(outStr, "unsupported agent") {
            t.Errorf("expected 'unsupported agent' in output:\n%s", outStr)
        }
    })

    t.Run("old_style_name_rejected", func(t *testing.T) {
        t.Parallel()
        cmd := exec.Command(binaryPath, "run", "--agent", "claude-code", "-f", configPath)
        output, err := cmd.CombinedOutput()
        outStr := string(output)

        if err == nil {
            t.Fatal("expected non-zero exit, got nil error")
        }
        if !strings.Contains(outStr, "unsupported agent") {
            t.Errorf("expected 'unsupported agent' in output:\n%s", outStr)
        }
    })
}
```

**Task 3 — Host Agent Config Disabled Test:**
```go
func TestMultiAgent_hostAgentConfigDisabledNoError(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }

    tmpDir := t.TempDir()
    binaryPath := filepath.Join(tmpDir, "asbox")
    buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
    buildCmd.Dir = ".."
    if out, err := buildCmd.CombinedOutput(); err != nil {
        t.Fatalf("building asbox binary: %v\noutput: %s", err, out)
    }

    configDir := t.TempDir()
    configPath := filepath.Join(configDir, "config.yaml")
    // host_agent_config: false should not cause any config error
    configContent := "installed_agents:\n  - claude\nproject_name: test-host-config\nhost_agent_config: false\n"
    if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
        t.Fatalf("writing config: %v", err)
    }

    cmd := exec.Command(binaryPath, "run", "-f", configPath)
    output, _ := cmd.CombinedOutput() // ignore error — docker run/build will fail
    outStr := string(output)

    // The command should proceed past config validation. It will fail at
    // docker build/run, but should NOT fail at host_agent_config processing.
    // If host_agent_config: false caused an error, we'd see it here.
    if strings.Contains(outStr, "host_agent_config") || strings.Contains(outStr, "host agent config") {
        t.Errorf("unexpected host_agent_config error in output:\n%s", outStr)
    }
}
```

### Previous Story Intelligence

**From Story 9-4 (MCP, Auto-Isolate-Deps, BMAD Repos Tests):**
- `startTestContainerWithEntrypoint` is defined in `podman_test.go` (line 22-51), NOT in `integration_test.go`. Uses `Privileged: true` and waits for Podman socket.
- Subtests within a top-level function can be parallel (independent) or sequential (dependencies).
- Image built once per top-level test function, shared across subtests.
- Binary invocation pattern: `exec.Command("go", "build", "-o", binPath, ".")` with `cmd.Dir = ".."`.
- All tests passed on first run. Full suite: 74s, 16 test functions, 0 failures.

**From Story 9-2 (Lifecycle and Mount Tests):**
- Exit code checking: `errors.As(err, &exitErr)` then `exitErr.ExitCode()`.
- Env filtering for controlled environment: `os.Environ()` loop with `strings.HasPrefix` exclusion.

**From Story 9-1 (Infrastructure):**
- `buildTestImageWithConfig` accepts `*config.Config` — use for multi-agent config.
- `fileContentInContainer` uses `tcexec.Multiplexed()` — safe for exact comparisons.
- `fileExistsInContainer` returns bool without failing — use for existence checks.
- Nanosecond image tags prevent collision in parallel tests.

**From Story 1-9 (Multi-Agent Runtime Support — implementation):**
- `installed_agents` is validated: non-empty, all entries valid, no duplicates.
- `default_agent` defaults to first of `installed_agents` if not set.
- `gemini` requires `sdks.nodejs` — validator enforces this.
- `host_agent_config` is `*bool`: nil means default (true), `false` means disabled, `true` means enabled.
- `AssembleHostAgentConfig` silently skips when: disabled, unknown agent, missing dir, not a directory.

**Deferred Issues (DO NOT FIX — from story 1-9 review):**
- No cmd-level tests for `--agent` flag path — this story addresses it via binary invocation error-path tests.

### Anti-Patterns to Avoid

- Do NOT import `testify/assert` — project uses stdlib `testing` only
- Do NOT modify existing test files from stories 9-1 through 9-4
- Do NOT add new helpers to `integration_test.go` — use existing ones
- Do NOT use `time.Sleep` — use `wait.ForExec` or retry loops
- Do NOT use `os.Setenv` in test process — use `cmd.Env` for binary invocations
- Do NOT test `agentCommand()` mapping in integration — already unit-tested in `cmd/run_test.go`
- Do NOT test `ValidateAgent`/`ValidateAgentInstalled` in isolation — already unit-tested in `internal/config/parse_test.go`
- Do NOT test `AssembleHostAgentConfig` with temp dirs — already unit-tested in `internal/mount/mount_test.go`
- Do NOT add `//go:embed` — use `os.ReadFile` or inline test data

### Git Intelligence

Recent commits show single-commit-per-story pattern:
```
de3c08b feat: multi-agent runtime support with installed_agents and --agent flag
```

This is the implementation being tested. Key files changed:
- `cmd/run.go` — --agent flag, agent override logic, host config assembly
- `internal/config/config.go` — Config struct, AgentConfigRegistry
- `internal/config/parse.go` — installed_agents validation, default_agent resolution
- `internal/mount/mount.go` — AssembleHostAgentConfig()
- `embed/Dockerfile.tmpl` — range over InstalledAgents for CLI installation and instruction files

### Project Structure Notes

- All integration tests in `integration/` package (separate from unit tests in `internal/`)
- Test execution: `go test -v -count=1 ./integration/...`
- CI: Integration tests run on PRs only (`.github/workflows/ci.yml`)
- Makefile: `make test-integration` runs integration suite

### Key Source References

- [Source: internal/config/config.go:39-52] — `Config` struct with `InstalledAgents`, `DefaultAgent`, `HostAgentConfig` fields
- [Source: internal/config/config.go:24-36] — `AgentConfigMapping` and `AgentConfigRegistry` with claude/gemini mappings
- [Source: internal/config/parse.go:44-75] — installed_agents validation, default_agent resolution
- [Source: internal/config/parse.go:105-110] — gemini requires nodejs validation
- [Source: internal/config/parse.go:179-196] — `ValidateAgent()` and `ValidateAgentInstalled()` exported functions
- [Source: cmd/run.go:24-34] — --agent flag override logic with validation
- [Source: cmd/run.go:47-55] — AssembleHostAgentConfig() call with mount and env var assembly
- [Source: cmd/run.go:172-183] — `agentCommand()` mapping: claude → `--dangerously-skip-permissions`, gemini → `-y`
- [Source: cmd/run.go:185-188] — --agent flag registration in init()
- [Source: cmd/root.go:49-71] — exitCode() mapping: ConfigError → 1, SecretError → 4, DependencyError → 3
- [Source: cmd/root.go:73-79] — Execute() prints errors to stderr with "error: " prefix
- [Source: internal/mount/mount.go:41-68] — `AssembleHostAgentConfig()`: disabled check, registry lookup, tilde expansion, silent skip
- [Source: embed/Dockerfile.tmpl:77-103] — `{{range .InstalledAgents}}` blocks for agent CLI installation and instruction files
- [Source: integration/integration_test.go] — Shared helpers: `buildTestImageWithConfig`, `startTestContainer`, `execInContainer`, `fileExistsInContainer`, `fileContentInContainer`
- [Source: integration/mount_test.go] — Binary invocation pattern reference

### FR Coverage Matrix

| FR | Description | Test Approach |
|----|-------------|---------------|
| FR56 | installed_agents list → agents installed at build time | `multi_agent_test.go`: container test, `which claude` + `which gemini` |
| FR57 | --agent flag override, must be in installed_agents | `multi_agent_test.go`: binary test, `--agent gemini` with only claude → error |
| FR58 | Short agent names (claude, gemini) | `multi_agent_test.go`: binary test, old-style name `claude-code` → error |
| FR59 | Agent config registry for host_agent_config | `multi_agent_test.go`: binary test, `host_agent_config: false` no error |
| FR44 | Agent instruction files baked into image | `multi_agent_test.go`: container test, CLAUDE.md + GEMINI.md exist |
| FR9d | host_agent_config boolean with auto path resolution | Unit tests in `mount_test.go` (primary); binary smoke test (secondary) |
| FR7 | default_agent from installed_agents | Unit tests in `parse_test.go` (primary coverage) |

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- Initial container test failed due to `defer cancel()` firing before parallel subtests resumed — fixed by using `t.Cleanup(cancel)` instead

### Completion Notes List

- Task 1: Created `TestMultiAgent_bothAgentsInstalledInImage` — builds multi-agent image with claude+gemini, verifies both CLIs available via `which` and both instruction files (CLAUDE.md, GEMINI.md) exist in container. Used `t.Cleanup(cancel)` for context lifecycle with parallel subtests.
- Task 2: Created `TestMultiAgent_agentFlagValidation` — binary invocation tests proving `--agent` flag is wired up: uninstalled agent (gemini not in installed_agents) exits 1 with "not installed", invalid name exits 1 with "unsupported agent", old-style name (claude-code) exits 1 with "unsupported agent".
- Task 3: Created `TestMultiAgent_hostAgentConfigDisabledNoError` — binary invocation test verifying `host_agent_config: false` does not produce config errors. Command proceeds past config/mount validation (fails at docker build, which is expected).
- Task 4: Full regression suite passes — 19 test functions, 0 failures, 139.87s. `go vet` clean.

### Change Log

- Created `integration/multi_agent_test.go` with 3 test functions covering AC #1, #4, #6 (Date: 2026-04-13)

### Review Findings

- [x] [Review][Decision] AC #6 test (`hostAgentConfigDisabledNoError`) is effectively a no-op — Dismissed: accepted as-is. Smoke test proves config parse doesn't error on `host_agent_config: false`; unit tests in mount_test.go are primary coverage for AC #6.
- [x] [Review][Patch] Missing exit code 1 assertion in `invalid_agent_name_exits_code_1` and `old_style_name_rejected` subtests — Fixed: added `errors.As(err, &exitErr) || exitErr.ExitCode() != 1` check to both subtests. [integration/multi_agent_test.go:108-138]

### File List

- `integration/multi_agent_test.go` (NEW)
