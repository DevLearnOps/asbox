# Story 1.10: Codex Agent Support

Status: done

## Story

As a developer,
I want to install and run OpenAI Codex CLI as a sandboxed agent,
So that I can use Codex for autonomous coding tasks with the same isolation guarantees as Claude and Gemini.

## Acceptance Criteria

1. **AC1: Codex Installation**
   **Given** a config with `installed_agents: [codex]`
   **When** the sandbox image is built
   **Then** Codex CLI is installed via `npm install -g @openai/codex`

2. **AC2: Codex Default Launch**
   **Given** a config with `installed_agents: [claude, codex]` and `default_agent: codex`
   **When** the developer runs `asbox run`
   **Then** the sandbox launches with Codex CLI (`codex --dangerously-bypass-approvals-and-sandbox`)

3. **AC3: Codex Runtime Override**
   **Given** a config with `installed_agents: [claude, gemini, codex]`
   **When** the developer runs `asbox run --agent codex`
   **Then** the sandbox launches with Codex CLI, overriding the default

4. **AC4: Host Agent Config with Codex**
   **Given** `host_agent_config` is enabled (default) and agent is `codex`
   **When** the sandbox launches and `~/.codex` exists on the host
   **Then** `~/.codex` is mounted read-write at `/opt/codex-config` and `CODEX_HOME=/opt/codex-config` is set

5. **AC5: Missing Host Config Silent Skip**
   **Given** `host_agent_config` is enabled and `~/.codex` does not exist on the host
   **When** the sandbox launches with codex
   **Then** the mount is silently skipped -- no error

6. **AC6: Codex Instruction File**
   **Given** a config with `installed_agents: [codex]`
   **When** the sandbox image is built
   **Then** `CODEX.md` agent instruction file is present in `/home/sandbox/`

7. **AC7: Codex Requires Node.js**
   **Given** codex is in `installed_agents`
   **When** validating the config
   **Then** `sdks.nodejs` must be set (codex requires Node.js, same validation as gemini)

## Tasks / Subtasks

- [x] Task 1: Add codex to `AgentConfigRegistry` in `internal/config/config.go` (AC: #4, #5)
  - [x] Add registry entry: `"codex": {Source: "~/.codex", Target: "/opt/codex-config", EnvVar: "CODEX_HOME", EnvVal: "/opt/codex-config"}`
- [x] Task 2: Add codex to validation in `internal/config/parse.go` (AC: #7)
  - [x] Add `"codex": true` to `validAgents` map
  - [x] Add codex to the nodejs requirement validation alongside gemini (line 105-110): `slices.Contains(cfg.InstalledAgents, "codex")`
  - [x] Update error message in `ValidateAgent()` to include codex: `"Use 'claude', 'gemini', or 'codex'"`
- [x] Task 3: Add codex case to `agentCommand()` in `cmd/run.go` (AC: #2, #3)
  - [x] Add `case "codex": return "codex --dangerously-bypass-approvals-and-sandbox", nil` to switch
  - [x] Update default error message: `"supported agents are claude, gemini, codex"`
  - [x] Add `"codex"` case to bmad_repos instruction target switch (line 82-88): target is `/home/sandbox/CODEX.md`
  - [x] Update `--agent` flag help text to include codex: `"(e.g., claude, gemini, codex)"`
- [x] Task 4: Add codex installation block in `embed/Dockerfile.tmpl` (AC: #1, #6)
  - [x] Add codex case in the `{{range .InstalledAgents}}` block (after gemini, line 90): `npm install -g @openai/codex` with npm-global chown (same pattern as gemini)
  - [x] Add codex case in the instruction file copy section (line 101): `cp` to `/home/sandbox/CODEX.md`
- [x] Task 5: Update `embed/config.yaml` starter template (AC: #1)
  - [x] Add `# - codex` as commented option in `installed_agents` list (after gemini comment)
- [x] Task 6: Update unit tests
  - [x] `internal/config/parse_test.go`: add test for codex in `validAgents`, codex nodejs requirement validation, codex in multi-agent configs
  - [x] `internal/mount/mount_test.go`: add codex entry to `AssembleHostAgentConfig` tests (enabled with dir, missing dir silent skip)
  - [x] `cmd/run_test.go`: add `"codex"` case to `agentCommand` tests verifying `"codex --dangerously-bypass-approvals-and-sandbox"` output

### Review Findings

- [x] [Review][Patch] Add `--instructions /home/sandbox/CODEX.md` to codex command — Codex CLI does not auto-discover `CODEX.md` from home directory; requires explicit `--instructions` flag. Fixed: updated `agentCommand()` and test. [cmd/run.go:176, cmd/run_test.go:203]
- [x] [Review][Dismiss] Codex config path `~/.codex` and `CODEX_HOME` env var verified correct — confirmed from Codex CLI source (`codex-rs/utils/home-dir/src/lib.rs`). [internal/config/config.go:36]
- [x] [Review][Patch] No test for instruction-target switch (`case "codex"`) in bmad_repos code path — Fixed: extracted `agentInstructionTarget()` helper and added tests. [cmd/run.go:182-194, cmd/run_test.go:208-225]
- [x] [Review][Defer] bmad_repos instruction mount only targets default agent, non-default agents retain generic build-time instructions [cmd/run.go:81-92] — deferred, pre-existing
- [x] [Review][Defer] Mutable global `AgentConfigRegistry` in tests risks data races if `t.Parallel()` added [internal/mount/mount_test.go:245-277] — deferred, pre-existing
- [x] [Review][Defer] Node.js SDK validation checks not consolidated (separate if-blocks per agent) [internal/config/parse.go:109-117] — deferred, pre-existing
- [x] [Review][Defer] npm install with no version pinning for @openai/codex [embed/Dockerfile.tmpl:92] — deferred, pre-existing
- [x] [Review][Defer] Hardcoded supported-agent lists in error messages need manual maintenance [multiple files] — deferred, pre-existing
- [x] [Review][Defer] `AGENT_CMD` via `bash -c` string expansion is fragile pattern [embed/entrypoint.sh] — deferred, pre-existing

## Dev Notes

### All Changes are Additive

Every change follows the established multi-agent registry pattern from Story 1.9. No existing behavior is modified. Each touchpoint already handles N agents via maps, lists, or range loops.

### Config Registry Addition (config.go)

Current `AgentConfigRegistry` at `internal/config/config.go:33-36`:
```go
var AgentConfigRegistry = map[string]AgentConfigMapping{
    "claude": {Source: "~/.claude", Target: "/opt/claude-config", EnvVar: "CLAUDE_CONFIG_DIR", EnvVal: "/opt/claude-config"},
    "gemini": {Source: "~/.gemini", Target: "/opt/gemini-home/.gemini", EnvVar: "GEMINI_CLI_HOME", EnvVal: "/opt/gemini-home"},
}
```

Add one entry:
```go
    "codex": {Source: "~/.codex", Target: "/opt/codex-config", EnvVar: "CODEX_HOME", EnvVal: "/opt/codex-config"},
```

### Validation Changes (parse.go)

**`validAgents` map** at `internal/config/parse.go:15-18` -- add `"codex": true`:
```go
var validAgents = map[string]bool{
    "claude": true,
    "gemini": true,
    "codex":  true,
}
```

**Node.js requirement** at `internal/config/parse.go:105-110` -- the existing gemini check is:
```go
if slices.Contains(cfg.InstalledAgents, "gemini") && cfg.SDKs.NodeJS == "" {
    return nil, &ConfigError{
        Field: "installed_agents",
        Msg:   "agent 'gemini' requires sdks.nodejs to be configured",
    }
}
```

Add an identical block for codex immediately after (or combine with OR logic):
```go
if slices.Contains(cfg.InstalledAgents, "codex") && cfg.SDKs.NodeJS == "" {
    return nil, &ConfigError{
        Field: "installed_agents",
        Msg:   "agent 'codex' requires sdks.nodejs to be configured",
    }
}
```

**`ValidateAgent()` error message** at `internal/config/parse.go:182` -- update to include codex:
```go
return &ConfigError{Field: "agent", Msg: fmt.Sprintf("unsupported agent '%s'. Use 'claude', 'gemini', or 'codex'", agent)}
```

### Run Command Changes (cmd/run.go)

**`agentCommand()` switch** at `cmd/run.go:174-183` -- add codex case:
```go
func agentCommand(agent string) (string, error) {
    switch agent {
    case "claude":
        return "claude --dangerously-skip-permissions", nil
    case "gemini":
        return "gemini -y", nil
    case "codex":
        return "codex --dangerously-bypass-approvals-and-sandbox", nil
    default:
        return "", fmt.Errorf("unknown agent %q: supported agents are claude, gemini, codex", agent)
    }
}
```

**bmad_repos instruction target switch** at `cmd/run.go:82-89` -- add codex case:
```go
switch cfg.DefaultAgent {
case "claude":
    instructionTarget = "/home/sandbox/CLAUDE.md"
case "gemini":
    instructionTarget = "/home/sandbox/GEMINI.md"
case "codex":
    instructionTarget = "/home/sandbox/CODEX.md"
default:
    return fmt.Errorf("bmad_repos: unsupported agent %q for instruction file mount", cfg.DefaultAgent)
}
```

**`--agent` flag help text** at `cmd/run.go:187` -- update to include codex:
```go
runCmd.Flags().String("agent", "", "Override default agent for this session (e.g., claude, gemini, codex)")
```

### Dockerfile Template Changes (Dockerfile.tmpl)

**Agent installation** at `embed/Dockerfile.tmpl:77-91` -- add codex block inside the `{{range .InstalledAgents}}` loop, after gemini (same npm install pattern):
```
{{- if eq . "codex"}}

RUN npm install -g @openai/codex && \
    chown -R sandbox:sandbox /home/sandbox/.npm-global
{{- end}}
```

**Instruction file copy** at `embed/Dockerfile.tmpl:95-102` -- add codex case:
```
{{- if eq . "codex"}}
RUN cp /tmp/agent-instructions.md /home/sandbox/CODEX.md && chown sandbox:sandbox /home/sandbox/CODEX.md
{{- end}}
```

### Starter Config Template (embed/config.yaml)

At `embed/config.yaml:6-8`, add codex comment:
```yaml
installed_agents:
  - claude
  # - gemini
  # - codex
```

Also update the `host_agent_config` comment at line 54-57 to mention codex:
```yaml
# Mount host agent config directory into sandbox for OAuth/SSO token sync
# Automatically resolves paths per agent (claude: ~/.claude, gemini: ~/.gemini, codex: ~/.codex)
# Set to false to disable
# host_agent_config: false
```

### Testing Patterns

All tests follow individual test functions (not table-driven), using `t.TempDir()` for filesystem isolation.

**Config test pattern** (`internal/config/parse_test.go`): write YAML to temp file, call `config.Parse()`, assert results. See existing multi-agent tests for patterns.

**Mount test pattern** (`internal/mount/mount_test.go`): build `config.Config` struct directly, call `AssembleHostAgentConfig()`, assert mount string and env vars. Tests for codex should mirror existing claude/gemini tests, creating a temp `~/.codex`-style dir.

**agentCommand test pattern** (`cmd/run_test.go`): call `agentCommand("codex")`, assert `"codex --dangerously-bypass-approvals-and-sandbox"` and nil error.

### Project Structure Notes

No new files created. All changes modify existing files:
- `internal/config/config.go` -- one map entry addition
- `internal/config/parse.go` -- one map entry, one validation block, one error message
- `cmd/run.go` -- two switch cases, one flag help text, one error message
- `embed/Dockerfile.tmpl` -- two template blocks (installation + instruction file)
- `embed/config.yaml` -- two comment additions
- `internal/config/parse_test.go` -- new test functions
- `internal/mount/mount_test.go` -- new test functions
- `cmd/run_test.go` -- new test function

### Previous Story Intelligence

**From Story 1.9 (Multi-Agent Runtime Support):**
- The multi-agent infrastructure is registry-based: `validAgents` map, `AgentConfigRegistry` map, `agentCommand()` switch, Dockerfile `{{range .InstalledAgents}}`. Adding codex means adding one entry to each.
- `HostAgentConfig` is a `*bool` -- `nil` or `true` means enabled (mount if host dir exists), `false` means disabled. `AssembleHostAgentConfig()` already handles unknown agents by returning empty (silent skip).
- The Dockerfile template uses `cp` (not `mv`) for instruction files since multiple agents may be installed. The temp file is cleaned up after the range loop.
- Gemini's npm install pattern includes `chown -R sandbox:sandbox /home/sandbox/.npm-global` to fix npm global permissions. Codex uses the same npm install pattern and needs the same chown.
- Error messages follow the AC-specified format. `ValidateAgent()` returns `ConfigError` with the supported agent list.
- Review finding from 1.9: gemini (and now codex) requires Node.js SDK validation at parse time. This was added as a post-review patch.

**From Story 1.9 review findings:**
- Permission errors on host config dir should return an error (not silently skip). Only `os.IsNotExist` triggers silent skip. Already implemented in `mount.go:57-63`.
- Duplicate agents in `installed_agents` are rejected. Already implemented in `parse.go:51-63`.

### Git Intelligence

Most recent commit: `d917eea docs: course correct to include codex into the sandbox` -- updated planning artifacts (PRD, architecture, epics) with codex references. The codebase is ready for the implementation changes.

Previous implementation commit: `de3c08b feat: multi-agent runtime support with installed_agents and --agent flag` -- established the patterns this story extends.

All tests currently pass. The integration test suite (`integration/multi_agent_test.go`) was added in `6abdb16` for story 9-5.

### References

- [Source: _bmad-output/planning-artifacts/epics.md -- Epic 1, Story 1.10]
- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-04-14.md -- Full proposal with technical impact table]
- [Source: _bmad-output/planning-artifacts/architecture.md -- Host Agent Config registry (lines 281-291), BMAD repos CODEX.md (line 278)]
- [Source: _bmad-output/planning-artifacts/prd.md -- FR7, FR9d, FR19a, FR44, FR45, FR58]
- [Source: _bmad-output/implementation-artifacts/1-9-multi-agent-runtime-support.md -- Dev Notes, File List, Review Findings]
- [Source: internal/config/config.go -- AgentConfigRegistry (lines 33-36)]
- [Source: internal/config/parse.go -- validAgents (lines 15-18), nodejs validation (lines 105-110), ValidateAgent (lines 180-185)]
- [Source: cmd/run.go -- agentCommand() (lines 174-183), bmad_repos instruction target (lines 82-89), --agent flag (line 187)]
- [Source: embed/Dockerfile.tmpl -- InstalledAgents range (lines 77-103)]
- [Source: embed/config.yaml -- installed_agents (lines 6-8), host_agent_config comment (lines 54-57)]
- [Source: internal/mount/mount.go -- AssembleHostAgentConfig (lines 41-68)]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

No issues encountered. All changes were additive, following established patterns from Story 1.9.

### Completion Notes List

- Added codex to AgentConfigRegistry with ~/.codex source, /opt/codex-config target, CODEX_HOME env var
- Added codex to validAgents map and Node.js requirement validation in parse.go
- Updated ValidateAgent() error message to include codex as supported agent
- Added codex case to agentCommand() returning "codex --dangerously-bypass-approvals-and-sandbox"
- Added codex case to bmad_repos instruction target switch (CODEX.md)
- Updated --agent flag help text to include codex
- Added codex installation block in Dockerfile.tmpl (npm install -g @openai/codex with chown)
- Added codex instruction file copy block in Dockerfile.tmpl (CODEX.md)
- Added # - codex commented option in embed/config.yaml starter template
- Updated host_agent_config comment to mention codex: ~/.codex
- Updated existing test error message expectations for new 3-agent list
- Added 6 new test functions: TestParse_codexInValidAgents, TestParse_codexRequiresNodejs, TestParse_codexInMultiAgentConfig, TestAssembleHostAgentConfig_codexWithExistingDir, TestAssembleHostAgentConfig_codexMissingDirSilentSkip, TestAgentCommand_codex
- All tests pass (0 failures), go vet clean, no regressions

### Change Log

- 2026-04-14: Implemented codex agent support across config, validation, runtime, Dockerfile template, and tests

### File List

- internal/config/config.go (modified: added codex to AgentConfigRegistry)
- internal/config/parse.go (modified: added codex to validAgents, nodejs validation, error message)
- cmd/run.go (modified: added codex to agentCommand, bmad_repos instruction target, flag help text)
- embed/Dockerfile.tmpl (modified: added codex installation and instruction file copy blocks)
- embed/config.yaml (modified: added codex comment in installed_agents and host_agent_config)
- internal/config/parse_test.go (modified: updated error expectations, added 3 codex test functions)
- internal/mount/mount_test.go (modified: added 2 codex host agent config test functions)
- cmd/run_test.go (modified: added 1 codex agentCommand test function)
