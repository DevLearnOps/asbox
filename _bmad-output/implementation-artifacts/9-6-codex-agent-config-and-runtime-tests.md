# Story 9.6: Codex Agent Config and Runtime Tests

Status: done

## Story

As a developer,
I want integration tests for codex agent configuration, installation, and runtime behavior,
So that codex support is validated alongside claude and gemini.

## Acceptance Criteria

1. **Codex CLI Available in Image**
   ```
   GIVEN a test config with `installed_agents: [claude, codex]`
   WHEN the sandbox image is built
   THEN both `claude` and `codex` CLI commands are available inside the container
   ```

2. **Codex Default Agent Command Resolution**
   ```
   GIVEN a test config with `installed_agents: [codex]` and `default_agent: codex`
   WHEN the sandbox launches
   THEN the agent command is `codex --dangerously-bypass-approvals-and-sandbox`
   ```

3. **Codex Agent Flag Override**
   ```
   GIVEN a test config with `installed_agents: [claude, gemini, codex]`
   WHEN the sandbox launches with `--agent codex`
   THEN the agent command is `codex --dangerously-bypass-approvals-and-sandbox`
   ```

4. **Codex Not Installed Rejection**
   ```
   GIVEN a test config with `installed_agents: [claude]`
   WHEN `--agent codex` is passed
   THEN the CLI exits with code 1 with a clear error about codex not being installed
   ```

5. **Host Agent Config with Codex**
   ```
   GIVEN a test config with `host_agent_config: true` and agent is `codex`
   WHEN the sandbox launches with a valid `~/.codex` directory
   THEN the directory is mounted at `/opt/codex-config` and the entrypoint symlinks config/auth files into `CODEX_HOME=/home/sandbox/.codex`
   ```

6. **Codex Instruction File in Image**
   ```
   GIVEN a test config with `installed_agents: [codex]`
   WHEN inspecting the container
   THEN `AGENTS.md` is present in `/home/sandbox/.codex/` (the `CODEX_HOME` directory)
   ```

## Tasks / Subtasks

- [x] Task 1: Create container-based codex image tests in `integration/multi_agent_test.go` (AC: #1, #6)
  - [x] 1.1 Build codex image with `InstalledAgents: []string{"claude", "codex"}, SDKs: config.SDKConfig{NodeJS: "22"}` via `buildTestImageWithConfig`
  - [x] 1.2 Start container with `startTestContainer` (tail -f /dev/null)
  - [x] 1.3 Test: `which claude` returns exit code 0 (claude CLI available)
  - [x] 1.4 Test: `which codex` returns exit code 0 (codex CLI available)
  - [x] 1.5 Test: `/home/sandbox/.codex/AGENTS.md` exists via `fileExistsInContainer`
  - [x] 1.6 Test: `/home/sandbox/CLAUDE.md` exists via `fileExistsInContainer`

- [x] Task 2: Create binary invocation test for `--agent codex` not installed (AC: #4)
  - [x] 2.1 Build the `asbox` binary into temp dir (same pattern as story 9-5)
  - [x] 2.2 Write config: `installed_agents: [claude]`, `project_name: test-codex-agent`
  - [x] 2.3 Test: `asbox run --agent codex -f <config>` exits code 1, stderr contains `"not installed"` and `"codex"`

- [x] Task 3: Run full integration test suite and verify no regressions
  - [x] 3.1 `go test -v -count=1 ./integration/...` passes all existing + new tests
  - [x] 3.2 `go vet ./integration/...` passes

## Dev Notes

### Critical Implementation Context

**Test Approach by AC:**

| AC | Test Type | Why |
|----|-----------|-----|
| #1 (both CLIs in image) | Container-based via `buildTestImageWithConfig` + `startTestContainer` | Need a running container to verify installed binaries |
| #2 (default agent codex command) | Covered by unit tests | `agentCommand("codex")` in `cmd/run_test.go:198-206` already validates output; `default_agent` resolution in `parse_test.go:787-814` already validates codex as default |
| #3 (--agent codex override) | Covered by unit tests | `ValidateAgent()` + `ValidateAgentInstalled()` in `parse_test.go` validate the validators; the wiring in `run.go` is exercised by AC #4's binary test (error path proves flag is read) |
| #4 (codex not installed rejected) | Binary invocation | Tests full CLI flow: config parse -> flag read -> validation -> error with exit code 1 |
| #5 (host_agent_config with codex) | Covered by unit tests | `TestAssembleHostAgentConfig_codexWithExistingDir` (mount_test.go:238-267) and `TestAssembleHostAgentConfig_codexMissingDirSilentSkip` (mount_test.go:269-286) already cover all paths with temp dirs. Integration depends on `~/.codex` existing on host, which is not reliable. |
| #6 (AGENTS.md instruction file) | Container-based via `fileExistsInContainer` | Instruction file is baked at build time via Dockerfile template into CODEX_HOME; need running container to verify |

**Why Not Integration Test AC #2, #3, and #5 Directly:**
- `AGENT_CMD` env var is passed to `docker run` at runtime by `cmd/run.go` -- it is NOT baked into the image and NOT logged to stdout. There is no observable output to assert against before docker run starts.
- The `agentCommand()` function and `--agent` flag override logic are straightforward and fully unit-tested.
- `host_agent_config` mount assembly depends on host filesystem state (`~/.codex` existing). Unit tests in `mount_test.go` use temp dirs to test all paths including the codex-specific entries. Note: codex registry entry has empty EnvVar/EnvVal — CODEX_HOME is baked into the image, not set at runtime.

**Container Build for Codex Image:**
Building with `InstalledAgents: [claude, codex]` requires `SDKs.NodeJS` to be set because codex is installed via `npm install -g @openai/codex`. The config validator in `parse.go:112-117` enforces this: `slices.Contains(cfg.InstalledAgents, "codex") && cfg.SDKs.NodeJS == ""` -> error.

**Codex Agent Command Detail:**
The `agentCommand("codex")` returns `"codex --dangerously-bypass-approvals-and-sandbox"`. Codex CLI auto-discovers `AGENTS.md` from `CODEX_HOME` (set to `/home/sandbox/.codex` via Dockerfile ENV). The integration tests here verify the binary (`which codex`) and file (`AGENTS.md` exists in CODEX_HOME), not the command string.

**Exit Code Mapping for Binary Tests:**
From `cmd/root.go:50-71`:
- `*config.ConfigError` -> exit code 1
- `*config.SecretError` -> exit code 4
- `*docker.DependencyError` -> exit code 3
- `*docker.RunError`, `*docker.BuildError` -> exit code 1
- Errors are printed to stderr: `fmt.Fprintf(os.Stderr, "error: %s\n", err)`

Since both config errors and docker errors map to exit code 1, binary tests must assert on **error message content** (not just exit code) to distinguish config validation failures from docker failures.

**Binary Invocation Pattern for AC #4:**
For `--agent codex` with only claude installed, the flow:
1. `PersistentPreRunE`: checks `docker` binary exists -> passes
2. `RunE`: `config.Parse(configPath)` -> success (config is valid)
3. `RunE`: reads `--agent` flag -> "codex"
4. `RunE`: `ValidateAgent("codex")` -> pass (valid short name)
5. `RunE`: `ValidateAgentInstalled("codex", ["claude"])` -> returns `*ConfigError`
6. `Execute()`: `fmt.Fprintf(os.Stderr, "error: %s\n", err)` -> prints to stderr
7. `os.Exit(1)`

### Architecture Compliance

**Test File Location (per epics spec):**
- Extend `integration/multi_agent_test.go` -- codex tests added to existing multi-agent test file

**Naming Convention:** `TestFeature_scenario` (e.g., `TestCodex_cliAndInstructionFileInImage`, `TestCodex_notInstalledExitsCode1`)

**Parallel Execution:** All test functions must include `if testing.Short() { t.Skip("...") }` guard. Independent subtests within a function must use `t.Parallel()`.

### Library/Framework Requirements

- Use ONLY Go stdlib `testing` package -- no `testify/assert`
- Use `testcontainers-go` v0.41.0 for container lifecycle (already in go.mod)
- Use `wait.ForExec([]string{"true"})` for container readiness (not `time.Sleep`)
- For binary invocation tests: `exec.Command`, `cmd.Dir`, `cmd.Env`, `cmd.CombinedOutput()` -- same pattern as `TestMultiAgent_agentFlagValidation` in `multi_agent_test.go`

### File Structure Requirements

```
integration/
├── integration_test.go          # Shared helpers -- DO NOT MODIFY
├── lifecycle_test.go            # Existing -- DO NOT MODIFY
├── mount_test.go                # Existing -- DO NOT MODIFY
├── isolation_test.go            # Existing -- DO NOT MODIFY
├── inner_container_test.go      # Existing -- DO NOT MODIFY
├── podman_test.go               # Existing -- DO NOT MODIFY
├── mcp_test.go                  # Existing -- DO NOT MODIFY
├── isolate_deps_test.go         # Existing -- DO NOT MODIFY
├── bmad_repos_test.go           # Existing -- DO NOT MODIFY
├── multi_agent_test.go          # MODIFY: add codex test functions
└── testdata/
    └── config.yaml              # Existing fixture -- DO NOT MODIFY
```

### Testing Requirements

**Task 1 -- Container-Based Codex Image Test:**
```go
func TestCodex_cliAndInstructionFileInImage(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }
    // Build codex image -- codex requires NodeJS (npm install -g @openai/codex)
    cfg := &config.Config{
        InstalledAgents: []string{"claude", "codex"},
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

    t.Run("codex_cli_available", func(t *testing.T) {
        t.Parallel()
        _, exitCode := execInContainer(ctx, t, container, []string{"which", "codex"})
        if exitCode != 0 {
            t.Error("codex CLI not found in container")
        }
    })

    t.Run("claude_instruction_file_exists", func(t *testing.T) {
        t.Parallel()
        if !fileExistsInContainer(ctx, t, container, "/home/sandbox/CLAUDE.md") {
            t.Error("expected /home/sandbox/CLAUDE.md to exist")
        }
    })

    t.Run("codex_instruction_file_exists", func(t *testing.T) {
        t.Parallel()
        if !fileExistsInContainer(ctx, t, container, "/home/sandbox/.codex/AGENTS.md") {
            t.Error("expected /home/sandbox/.codex/AGENTS.md to exist")
        }
    })
}
```

**Task 2 -- Binary Invocation Codex Not Installed Test:**
```go
func TestCodex_notInstalledExitsCode1(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }

    // Build binary once
    tmpDir := t.TempDir()
    binaryPath := filepath.Join(tmpDir, "asbox")
    buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
    buildCmd.Dir = ".." // project root
    if out, err := buildCmd.CombinedOutput(); err != nil {
        t.Fatalf("building asbox binary: %v\noutput: %s", err, out)
    }

    // Config with only claude installed (codex NOT installed)
    configDir := t.TempDir()
    configPath := filepath.Join(configDir, "config.yaml")
    configContent := "installed_agents:\n  - claude\nproject_name: test-codex-agent\n"
    if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
        t.Fatalf("writing config: %v", err)
    }

    cmd := exec.Command(binaryPath, "run", "--agent", "codex", "-f", configPath)
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
    if !strings.Contains(outStr, "codex") {
        t.Errorf("expected 'codex' in output:\n%s", outStr)
    }
}
```

### Previous Story Intelligence

**From Story 9-5 (Multi-Agent Config and Flag Tests):**
- Container test pattern: build image with `buildTestImageWithConfig`, start with `startTestContainer`, use `execInContainer` for `which` checks and `fileExistsInContainer` for file checks.
- Binary invocation pattern: build `asbox` binary into temp dir, write config file, run `exec.Command(binaryPath, "run", "--agent", ...)`, assert exit code and error message content.
- Use `t.Cleanup(cancel)` instead of `defer cancel()` for context lifecycle when using parallel subtests (bug fix from 9-5).
- Story 9-5 already tests: gemini+claude in image, `--agent gemini` not installed, invalid agent names, old-style names rejected, host_agent_config disabled.
- All tests currently pass: 19 test functions, 0 failures, 139.87s. `go vet` clean.

**From Story 1-10 (Codex Agent Support -- implementation):**
- Codex is installed via `npm install -g @openai/codex` (same npm pattern as gemini).
- Codex requires `SDKs.NodeJS` to be set -- config validator enforces this at parse time.
- `agentCommand("codex")` returns `"codex --dangerously-bypass-approvals-and-sandbox"` -- Codex CLI auto-discovers `AGENTS.md` from `CODEX_HOME`.
- Agent instruction files use a shared Go template (`embed/agent-instructions.md.tmpl`), rendered at build time and copied to `AGENTS.md` in `CODEX_HOME=/home/sandbox/.codex/`.
- `AgentConfigRegistry["codex"]` = `{Source: "~/.codex", Target: "/opt/codex-config", EnvVar: "", EnvVal: ""}` -- CODEX_HOME baked in image, not set at runtime.
- Entrypoint `setup_codex_home()` symlinks host config/auth from `/opt/codex-config` into CODEX_HOME (skips instruction files).
- `ValidateAgent()` error message at `parse.go:182`: `"unsupported agent '%s'. Use 'claude', 'gemini', or 'codex'"`.
- `ValidateAgentInstalled()` error: includes agent name and "not installed".
- 6 unit tests added: parse (3), mount (2), run (1). All pass.

**From Story 9-1 (Infrastructure):**
- `buildTestImageWithConfig` accepts `*config.Config` -- use for codex config.
- `fileExistsInContainer` returns bool without failing -- use for existence checks.
- Nanosecond image tags prevent collision in parallel tests.

### Anti-Patterns to Avoid

- Do NOT import `testify/assert` -- project uses stdlib `testing` only
- Do NOT modify existing test files from stories 9-1 through 9-5
- Do NOT add new helpers to `integration_test.go` -- use existing ones
- Do NOT use `time.Sleep` -- use `wait.ForExec` or retry loops
- Do NOT use `os.Setenv` in test process -- use `cmd.Env` for binary invocations
- Do NOT test `agentCommand("codex")` mapping in integration -- already unit-tested in `cmd/run_test.go:198-206`
- Do NOT test `ValidateAgent`/`ValidateAgentInstalled` in isolation -- already unit-tested in `internal/config/parse_test.go`
- Do NOT test `AssembleHostAgentConfig` with codex in integration -- already unit-tested in `internal/mount/mount_test.go:238-286`
- Do NOT add `//go:embed` -- use `os.ReadFile` or inline test data
- Do NOT create a separate test file -- extend `multi_agent_test.go`
- Do NOT duplicate the existing 9-5 multi-agent tests (claude+gemini image build, --agent flag validation for gemini/invalid/old-style) -- those already exist

### Git Intelligence

Recent commits show the codex implementation and test patterns:
```
5ef5104 feat: codex agent support with config, validation, runtime, and Dockerfile (story 1-10)
6abdb16 test: multi-agent config and flag integration tests (story 9-5)
de3c08b feat: multi-agent runtime support with installed_agents and --agent flag
```

Key files from story 1-10 (being tested):
- `internal/config/config.go` -- codex entry in `AgentConfigRegistry` (line 36)
- `internal/config/parse.go` -- codex in `validAgents` (line 17), nodejs validation (lines 112-117)
- `cmd/run.go` -- codex in `agentCommand()` (lines 175-176), bmad_repos instruction target (lines 87-88)
- `embed/Dockerfile.tmpl` -- codex installation block (lines 91-95), instruction file copy (lines 107-109)

### Project Structure Notes

- All integration tests in `integration/` package (separate from unit tests in `internal/`)
- Test execution: `go test -v -count=1 ./integration/...`
- Makefile: `make test-integration` runs integration suite

### Key Source References

- [Source: internal/config/config.go:33-37] -- `AgentConfigRegistry` with claude, gemini, codex mappings
- [Source: internal/config/parse.go:15-19] -- `validAgents` map including codex
- [Source: internal/config/parse.go:112-117] -- codex requires nodejs validation
- [Source: internal/config/parse.go:179-196] -- `ValidateAgent()` and `ValidateAgentInstalled()` exported functions
- [Source: cmd/run.go:175-176] -- `agentCommand("codex")` returns `"codex --dangerously-bypass-approvals-and-sandbox"`
- [Source: cmd/run.go:182-194] -- `agentInstructionTarget()` helper with codex case
- [Source: cmd/root.go:49-71] -- exitCode() mapping: ConfigError -> 1
- [Source: embed/Dockerfile.tmpl:91-95] -- codex installation block: `npm install -g @openai/codex`
- [Source: embed/Dockerfile.tmpl:107-110] -- codex instruction file: mkdir `.codex`, copy as `AGENTS.md`, set `ENV CODEX_HOME`
- [Source: integration/multi_agent_test.go] -- existing 9-5 test functions to extend alongside
- [Source: integration/integration_test.go] -- shared helpers: `buildTestImageWithConfig`, `startTestContainer`, `execInContainer`, `fileExistsInContainer`

### FR Coverage Matrix

| FR | Description | Test Approach |
|----|-------------|---------------|
| FR56 | installed_agents list -> agents installed at build time | `multi_agent_test.go`: container test, `which claude` + `which codex` |
| FR57 | --agent flag override, must be in installed_agents | `multi_agent_test.go`: binary test, `--agent codex` with only claude -> error |
| FR58 | Short agent names (claude, gemini, codex) | Covered by 9-5 tests (invalid names rejected); codex is a valid short name |
| FR19a | Codex with --dangerously-bypass-approvals-and-sandbox | Unit tests in `cmd/run_test.go:198-206` (primary coverage) |
| FR44 | Agent instruction files baked into image | `multi_agent_test.go`: container test, AGENTS.md in CODEX_HOME + CLAUDE.md exist |
| FR9d | host_agent_config with codex auto path resolution | Unit tests in `mount_test.go:238-286` (primary coverage) |

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

### Completion Notes List

- Task 1: Added `TestCodex_cliAndInstructionFileInImage` — container-based test building image with `installed_agents: [claude, codex]`, verifying both CLIs available via `which`, CLAUDE.md at `/home/sandbox/`, and AGENTS.md at `/home/sandbox/.codex/`. All 4 subtests pass in parallel.
- Task 2: Added `TestCodex_notInstalledExitsCode1` — binary invocation test with config containing only claude, running `--agent codex`. Verifies exit code 1, stderr contains "not installed" and "codex".
- Task 3: Full regression suite passes — 21 test functions, 0 failures, ~111s. `go vet` clean.
- All 6 acceptance criteria satisfied (AC #2, #3, #5 covered by existing unit tests as documented in Dev Notes).

### Change Log

- 2026-04-14: Added codex integration tests — 2 new test functions in `integration/multi_agent_test.go`

### Review Findings

- [x] [Review][Dismiss] New codex runtime files don't write-through to host — accepted: codex auth is pre-provisioned on host; new runtime file persistence not expected. [embed/entrypoint.sh:60-86, internal/config/config.go:36]
- [x] [Review][Patch] `setup_codex_home()` glob skips hidden (dot) files from host config — Fixed: added `shopt -s dotglob` before glob and `shopt -u dotglob` after loop to include dotfiles in symlink pass. [embed/entrypoint.sh:72]
- [x] [Review][Defer] Mutable global `AgentConfigRegistry` in tests risks data races if `t.Parallel()` added [internal/mount/mount_test.go:245-277] — deferred, pre-existing (tracked since story 1-10 review)
- [x] [Review][Defer] Parallel switch statements (`agentCommand`, `agentInstructionTarget`, Dockerfile.tmpl) can diverge when adding agents [cmd/run.go:171-195, embed/Dockerfile.tmpl:77-96] — deferred, pre-existing pattern (tracked since story 1-10 review as hardcoded agent lists)
- [x] [Review][Defer] No test for claude/gemini cases in extracted `agentInstructionTarget()` function [cmd/run_test.go:215-230] — deferred, pre-existing paths moved without behavior change
- [x] [Review][Defer] `setup_codex_home()` entrypoint function untested at integration level — symlink logic, AGENTS.md skip filter, and chown verified only by inspection; CI constraints (no guaranteed `~/.codex` on host) justify deferral [embed/entrypoint.sh:60-86]

### File List

- integration/multi_agent_test.go (modified — added TestCodex_cliAndInstructionFileInImage, TestCodex_notInstalledExitsCode1)
