# Story 1.9: Multi-Agent Runtime Support

Status: done

## Story

As a developer,
I want to install multiple agents in a single sandbox image and switch between them at runtime,
So that I can work with Claude and Gemini without rebuilding or editing config.

## Acceptance Criteria

1. **AC1: Multi-Agent Installation**
   **Given** a config with `installed_agents: [claude, gemini]`
   **When** the sandbox image is built
   **Then** both Claude Code and Gemini CLI are installed in the image

2. **AC2: Default Agent Launch**
   **Given** a config with `installed_agents: [claude, gemini]` and `default_agent: claude`
   **When** the developer runs `asbox run`
   **Then** the sandbox launches with Claude Code (`claude --dangerously-skip-permissions`)

3. **AC3: Runtime Agent Override via Flag**
   **Given** a config with `installed_agents: [claude, gemini]`
   **When** the developer runs `asbox run --agent gemini`
   **Then** the sandbox launches with Gemini CLI (`gemini -y`), overriding the default

4. **AC4: Agent Not Installed Error**
   **Given** a config with `installed_agents: [claude]`
   **When** the developer runs `asbox run --agent gemini`
   **Then** the CLI exits with code 1: `"error: agent 'gemini' is not installed in the image. Installed agents: claude. Add it to installed_agents in config or choose a different agent"`

5. **AC5: Default to First Agent**
   **Given** a config with `installed_agents: [claude, gemini]` and no `default_agent` set
   **When** the developer runs `asbox run`
   **Then** the first agent in the list (`claude`) is used as the default

6. **AC6: Short Agent Name Validation**
   **Given** agent names use short form (`claude`, `gemini`)
   **When** the config or CLI flag uses an old-style name (e.g., `claude-code`)
   **Then** the CLI exits with code 1: `"error: unsupported agent 'claude-code'. Use 'claude' or 'gemini'"`

7. **AC7: Host Agent Config with Claude**
   **Given** `host_agent_config` is enabled (default) and agent is `claude`
   **When** the sandbox launches and `~/.claude` exists on the host
   **Then** `~/.claude` is mounted read-write at `/opt/claude-config` and `CLAUDE_CONFIG_DIR=/opt/claude-config` is set

8. **AC8: Host Agent Config with Gemini**
   **Given** `host_agent_config` is enabled (default) and agent is `gemini`
   **When** the sandbox launches and `~/.gemini` exists on the host
   **Then** `~/.gemini` is mounted read-write at `/opt/gemini-home/.gemini` and `GEMINI_CLI_HOME=/opt/gemini-home` is set

9. **AC9: Missing Host Config Directory**
   **Given** `host_agent_config` is enabled and the host config directory does not exist (e.g., `~/.gemini` missing)
   **When** the sandbox launches
   **Then** the mount is silently skipped -- no error

10. **AC10: Disabled Host Agent Config**
    **Given** `host_agent_config: false` in config
    **When** the sandbox launches
    **Then** no host agent config directory is mounted regardless of agent

11. **AC11: Gemini CLI Launch Flag**
    **Given** Gemini CLI is selected as the agent
    **When** the sandbox launches
    **Then** Gemini CLI runs with `-y` flag (yolo mode, no permission prompts)

## Tasks / Subtasks

- [x] Task 1: Update `internal/config/config.go` — struct and registry (AC: #1, #6, #7, #8, #10)
  - [x] Rename `Agent string` field to `DefaultAgent string` with yaml tag `default_agent`
  - [x] Add `InstalledAgents []string` field with yaml tag `installed_agents`
  - [x] Change `HostAgentConfig *MountConfig` to `HostAgentConfig *bool` with yaml tag `host_agent_config`
  - [x] Add `AgentConfigMapping` struct with fields `Source`, `Target`, `EnvVar` (all string)
  - [x] Add package-level `AgentConfigRegistry map[string]AgentConfigMapping` with entries for `claude` and `gemini`
- [x] Task 2: Update `internal/config/parse.go` — validation logic (AC: #1, #2, #4, #5, #6)
  - [x] Update `validAgents` map: `"claude-code"` -> `"claude"`, `"gemini-cli"` -> `"gemini"`
  - [x] Replace single `agent` required-field validation with `installed_agents` validation (required, non-empty, all entries valid)
  - [x] Validate `default_agent`: if set, must be in `installed_agents`; if empty, set to first entry in `installed_agents`
  - [x] Remove `host_agent_config` source/target/abs-path validation block (lines 108-128) — no longer a MountConfig
  - [x] Remove `host_agent_config` path resolution block (lines 154-156) — paths resolved at runtime from registry
  - [x] Add exported `ValidateAgent(agent string) error` — validates against `validAgents`
  - [x] Add exported `ValidateAgentInstalled(agent string, installed []string) error` — validates agent is in list, returns `ConfigError` with AC4 error message format
- [x] Task 3: Update `cmd/run.go` — `--agent` flag, override logic, host config assembly (AC: #2, #3, #4, #7, #8, #9, #10, #11)
  - [x] Add `--agent` string flag to `runCmd` in `init()`
  - [x] After `config.Parse()`, read `--agent` flag; if set, call `config.ValidateAgent()` + `config.ValidateAgentInstalled()` and override `cfg.DefaultAgent`
  - [x] Replace hardcoded `cfg.Agent == "claude-code"` host config logic (line 36-38) with `AssembleHostAgentConfig()` call
  - [x] Update `agentCommand()`: `"claude"` -> `"claude --dangerously-skip-permissions"`, `"gemini"` -> `"gemini -y"`
  - [x] Update bmad_repos instruction target switch (lines 65-71): `"claude-code"` -> `"claude"`, `"gemini-cli"` -> `"gemini"`
  - [x] Pass `cfg.DefaultAgent` (instead of `cfg.Agent`) to `agentCommand()` and all agent-dependent logic
- [x] Task 4: Update `internal/mount/mount.go` — simplify and add `AssembleHostAgentConfig` (AC: #7, #8, #9, #10)
  - [x] Remove all `HostAgentConfig` logic from `AssembleMounts()` (lines 13, 17-20, 37-55) — it now only handles regular mounts
  - [x] Add `AssembleHostAgentConfig(agent string, enabled *bool) (mountFlag string, envKey string, envVal string, err error)`:
    - Look up agent in `config.AgentConfigRegistry`
    - If `enabled` is non-nil and `*enabled == false`, return empty (no mount)
    - Expand `~` in source path using `os.UserHomeDir()`
    - `os.Stat()` source — if not exists, return empty (silent skip per AC9)
    - If exists and is directory, return `"source:target"`, envKey, envVal
- [x] Task 5: Update `embed/Dockerfile.tmpl` — iterate `InstalledAgents` (AC: #1)
  - [x] Replace `{{- if eq .Agent "claude-code"}}` block (lines 77-84) with `{{range .InstalledAgents}}{{if eq . "claude"}}` block
  - [x] Replace `{{- if eq .Agent "gemini-cli"}}` block (lines 85-89) with `{{if eq . "gemini"}}` in the same range
  - [x] Replace agent instruction file block (lines 90-99): iterate `InstalledAgents`, copy template to CLAUDE.md for `claude` and GEMINI.md for `gemini` (install instruction files for ALL installed agents, not just the default)
- [x] Task 6: Update `embed/config.yaml` — restructure for new fields (AC: #1, #2, #5, #10)
  - [x] Replace `agent: claude-code` with `installed_agents:` list and `default_agent:` field
  - [x] Replace `host_agent_config:` MountConfig section with boolean example
  - [x] Update all agent name references in comments to short names
- [x] Task 7: Update unit tests (all ACs)
  - [x] `internal/config/parse_test.go`: update all test configs from `agent: claude-code` to `installed_agents: [claude]`; add tests for multi-agent validation, `default_agent` defaulting, short name validation, boolean `host_agent_config`
  - [x] `internal/mount/mount_test.go`: remove `HostAgentConfig` tests from `AssembleMounts`; add tests for `AssembleHostAgentConfig()` — enabled/disabled/missing-dir/silent-skip
  - [x] `cmd/run_test.go`: update `agentCommand` tests for short names and gemini `-y` flag
  - [x] `internal/config/config_test.go`: update any tests referencing old agent names
- [x] Task 8: Update integration test fixture and dependent files (AC: #1, #6)
  - [x] `integration/testdata/config.yaml`: change `agent: claude-code` to `installed_agents: [claude]`
  - [x] `internal/mount/bmad_repos.go`: update any agent name references if present
  - [x] Global search-and-replace: ensure no remaining references to `claude-code` or `gemini-cli` as agent names (except in error messages that mention them as invalid)

## Dev Notes

### Config Struct Changes (config.go)

Current struct at `internal/config/config.go:25-37`:
```go
type Config struct {
    Agent           string            `yaml:"agent"`
    // ... other fields ...
    HostAgentConfig *MountConfig      `yaml:"host_agent_config"`
}
```

Must become:
```go
type Config struct {
    InstalledAgents []string          `yaml:"installed_agents"`
    DefaultAgent    string            `yaml:"default_agent"`
    // ... other fields ...
    HostAgentConfig *bool             `yaml:"host_agent_config"`
}
```

Add the agent config registry as a package-level var (NOT in Config struct):
```go
type AgentConfigMapping struct {
    Source string // host path, e.g. "~/.claude"
    Target string // container path, e.g. "/opt/claude-config"
    EnvVar string // e.g. "CLAUDE_CONFIG_DIR"
}

var AgentConfigRegistry = map[string]AgentConfigMapping{
    "claude": {Source: "~/.claude", Target: "/opt/claude-config", EnvVar: "CLAUDE_CONFIG_DIR"},
    "gemini": {Source: "~/.gemini", Target: "/opt/gemini-config", EnvVar: "GEMINI_CONFIG_DIR"},
}
```

### Validation Logic Changes (parse.go)

Current `validAgents` at `internal/config/parse.go:14-17`:
```go
var validAgents = map[string]bool{
    "claude-code": true,
    "gemini-cli":  true,
}
```

Must become:
```go
var validAgents = map[string]bool{
    "claude": true,
    "gemini": true,
}
```

**Remove entirely** the `host_agent_config` validation block (lines 108-128) and resolution block (lines 154-156). The `host_agent_config` field is now a `*bool` — no paths to validate or resolve.

**New validation sequence** for agents (replaces lines 43-55):
1. `installed_agents` must be non-empty
2. Each entry in `installed_agents` must be in `validAgents`
3. If `default_agent` is set, it must be in `installed_agents`
4. If `default_agent` is empty, set it to `installed_agents[0]`

**New exported validators** for runtime use by `cmd/run.go`:
```go
func ValidateAgent(agent string) error {
    if !validAgents[agent] {
        return &ConfigError{Field: "agent", Msg: fmt.Sprintf("unsupported agent '%s'. Use 'claude' or 'gemini'", agent)}
    }
    return nil
}

func ValidateAgentInstalled(agent string, installed []string) error {
    if !slices.Contains(installed, agent) {
        return &ConfigError{
            Field: "agent",
            Msg: fmt.Sprintf("agent '%s' is not installed in the image. Installed agents: %s. Add it to installed_agents in config or choose a different agent", agent, strings.Join(installed, ", ")),
        }
    }
    return nil
}
```

### Run Command Changes (cmd/run.go)

**New `--agent` flag** — add in `init()` alongside existing `--no-cache`:
```go
runCmd.Flags().String("agent", "", "Override default agent for this session (e.g., claude, gemini)")
```

**Agent override logic** — after `config.Parse()`, before any agent-dependent code:
```go
agentOverride, _ := cmd.Flags().GetString("agent")
if agentOverride != "" {
    if err := config.ValidateAgent(agentOverride); err != nil {
        return err
    }
    if err := config.ValidateAgentInstalled(agentOverride, cfg.InstalledAgents); err != nil {
        return err
    }
    cfg.DefaultAgent = agentOverride
}
```

**Host agent config** — replace the hardcoded claude-only block (line 36-38) with:
```go
hostMountFlag, envKey, envVal, err := mount.AssembleHostAgentConfig(cfg.DefaultAgent, cfg.HostAgentConfig)
if err != nil {
    return err
}
if hostMountFlag != "" {
    mountFlags = append(mountFlags, hostMountFlag)
    envVars[envKey] = envVal
}
```

**agentCommand** — update switch cases (line 157-165):
- `"claude"` -> `"claude --dangerously-skip-permissions"`
- `"gemini"` -> `"gemini -y"` (note: currently missing the `-y` flag)

**bmad_repos instruction target** — update switch (lines 64-72):
- `"claude-code"` -> `"claude"`, target stays `/home/sandbox/CLAUDE.md`
- `"gemini-cli"` -> `"gemini"`, target stays `/home/sandbox/GEMINI.md`

### Mount Package Changes (mount.go)

`AssembleMounts()` currently handles both regular mounts and `HostAgentConfig` (lines 12-58). **Remove all HostAgentConfig logic** — the function should only process `cfg.Mounts`:

```go
func AssembleMounts(cfg *config.Config) ([]string, error) {
    if len(cfg.Mounts) == 0 {
        return nil, nil
    }
    mounts := make([]string, 0, len(cfg.Mounts))
    for _, m := range cfg.Mounts {
        // ... existing mount validation (unchanged) ...
    }
    return mounts, nil
}
```

**New function** `AssembleHostAgentConfig`:
```go
func AssembleHostAgentConfig(agent string, enabled *bool) (string, string, string, error) {
    if enabled != nil && !*enabled {
        return "", "", "", nil
    }
    mapping, ok := config.AgentConfigRegistry[agent]
    if !ok {
        return "", "", "", nil // unknown agent, skip silently
    }
    source := mapping.Source
    if strings.HasPrefix(source, "~/") {
        home, err := os.UserHomeDir()
        if err != nil {
            return "", "", "", nil // can't resolve home, skip
        }
        source = filepath.Join(home, source[2:])
    }
    info, err := os.Stat(source)
    if err != nil || !info.IsDir() {
        return "", "", "", nil // AC9: silently skip if missing
    }
    return source + ":" + mapping.Target, mapping.EnvVar, mapping.Target, nil
}
```

### Dockerfile Template Changes (Dockerfile.tmpl)

Replace the single-agent conditional blocks (lines 77-99) with a range over `InstalledAgents`:

```
{{- range .InstalledAgents}}
{{- if eq . "claude"}}

USER sandbox
ENV HOME=/home/sandbox
RUN curl -fsSL https://claude.ai/install.sh | bash
ENV PATH="/home/sandbox/.local/bin:${PATH}"
USER root
{{- end}}
{{- if eq . "gemini"}}

RUN npm install -g @google/gemini-cli && \
    chown -R sandbox:sandbox /home/sandbox/.npm-global
{{- end}}
{{- end}}
```

For instruction files, install for ALL agents in `InstalledAgents`:
```
COPY agent-instructions.md.tmpl /tmp/agent-instructions.md
{{- range .InstalledAgents}}
{{- if eq . "claude"}}
RUN cp /tmp/agent-instructions.md /home/sandbox/CLAUDE.md && chown sandbox:sandbox /home/sandbox/CLAUDE.md
{{- end}}
{{- if eq . "gemini"}}
RUN cp /tmp/agent-instructions.md /home/sandbox/GEMINI.md && chown sandbox:sandbox /home/sandbox/GEMINI.md
{{- end}}
{{- end}}
RUN rm -f /tmp/agent-instructions.md
```

Note: Use `cp` instead of `mv` since multiple agents may need the file. Clean up after the loop.

### Embedded Config Template (embed/config.yaml)

Replace the single `agent:` line with:
```yaml
# Agents to install in the sandbox image
# At least one agent is required
installed_agents:
  - claude
  # - gemini

# Default agent to launch (defaults to first in installed_agents if omitted)
# default_agent: claude
```

Replace the `host_agent_config:` MountConfig section with:
```yaml
# Mount host agent config directory into sandbox for OAuth/SSO token sync
# Automatically resolves paths per agent (claude: ~/.claude, gemini: ~/.gemini)
# Set to false to disable
# host_agent_config: false
```

### Global Agent Name Updates

All references to `claude-code` and `gemini-cli` as agent identifiers must be updated to `claude` and `gemini` throughout the codebase. Key locations:
- `internal/config/parse.go` — `validAgents` map, error messages
- `cmd/run.go` — `agentCommand()` switch, bmad instruction switch, host config conditional
- `embed/Dockerfile.tmpl` — all `eq .Agent` conditionals
- `embed/config.yaml` — agent field and comments
- `integration/testdata/config.yaml` — test fixture
- All `*_test.go` files — test configs and assertions

### Gemini `-y` Flag

The current `agentCommand()` maps `gemini-cli` to just `"gemini"`. Per AC11 and the sprint change proposal, gemini must run with `-y` flag (yolo mode, no permission prompts). The updated mapping is `"gemini"` -> `"gemini -y"`.

### Breaking Config Change

This is a breaking change to the config format. Since asbox is pre-1.0 with a small user base, this is acceptable. Existing configs with `agent: claude-code` will fail with the error from AC6: `"unsupported agent 'claude-code'. Use 'claude' or 'gemini'"`. The validation for `installed_agents` being empty will also catch old configs that don't have the new field.

### Project Structure Notes

No new files are created. All changes modify existing files:
- `internal/config/config.go` — struct modifications, new types and registry
- `internal/config/parse.go` — validation logic rewrite for multi-agent
- `cmd/run.go` — new flag, override logic, updated agent functions
- `internal/mount/mount.go` — simplification + new function
- `embed/Dockerfile.tmpl` — range iteration over agents
- `embed/config.yaml` — restructured config template
- `integration/testdata/config.yaml` — fixture update
- Various `*_test.go` — test updates

### Previous Story Intelligence

**From Story 1-8 (Configuration Initialization):**
- `embed/config.yaml` is the embedded starter config template — this file is what `asbox init` generates. It's the single source of truth (the old `templates/` directory was deleted).
- `configFile` var in `cmd/root.go` defaults to `.asbox/config.yaml` — reused by all commands.
- Error types: `config.ConfigError{Field: "...", Msg: "..."}` for exit code 1.
- Output pattern: `fmt.Fprintf(cmd.OutOrStdout(), ...)` for testable output.
- Testing convention: individual test functions per scenario (not table-driven), `t.TempDir()` for filesystem isolation.

**From Story 1-7 (Sandbox Run):**
- `cmd/build_helper.go` has shared `ensureBuild()` function used by both build and run commands.
- `docker.RunContainer()` accepts `docker.RunOptions` struct.
- Signal handling (exit code 130/143 suppression) is in `cmd/root.go`.

**From Story 7-1 (Host Agent Config):**
- The current `HostAgentConfig` implementation (as MountConfig) is being simplified — this story replaces it entirely. The old mount validation and path resolution in `parse.go` must be removed, not modified.

### Git Intelligence

Recent commits show the project is on `main` branch. The Go rewrite (from bash) landed as PR #1 (`40c6f41`). The most recent commit (`77db906`) added this sprint change proposal and updated planning artifacts. All prior stories (1-1 through 1-8, all epic 2-10 stories) are complete.

### Testing Patterns

- Unit tests use individual test functions (not table-driven).
- `t.TempDir()` for temp directories, `os.WriteFile()` to create test config files.
- Error checking: `errors.As()` for typed error assertions.
- Config test pattern: write YAML string to temp file, call `config.Parse()`, assert results.
- Mount test pattern: build `config.Config` struct directly, call mount functions, assert output.

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Epic 1, Story 1.9]
- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-04-11.md — Full proposal]
- [Source: _bmad-output/planning-artifacts/architecture.md — Config struct, Agent config registry, Mount assembly, Dockerfile template, CLI flags]
- [Source: _bmad-output/planning-artifacts/prd.md — FR7, FR9d, FR18, FR19, FR38, FR40, FR44, FR45, FR56-FR59]
- [Source: _bmad-output/implementation-artifacts/1-8-configuration-initialization.md — Dev Notes, File List, Testing Patterns]
- [Source: internal/config/config.go — Current Config struct, MountConfig, MCPServerRegistry]
- [Source: internal/config/parse.go — Current validAgents, Parse(), validation logic]
- [Source: cmd/run.go — Current runCmd, agentCommand(), buildEnvVars(), host config logic]
- [Source: internal/mount/mount.go — Current AssembleMounts() with HostAgentConfig handling]
- [Source: embed/Dockerfile.tmpl — Current single-agent conditional blocks]
- [Source: embed/config.yaml — Current single-agent config template]
- [Source: integration/testdata/config.yaml — Current test fixture]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

No debug issues encountered. All tests passed on first run after implementation.

### Completion Notes List

- Replaced single `Agent` field with `InstalledAgents` list and `DefaultAgent` for multi-agent support
- Added `AgentConfigRegistry` mapping agents to their host config paths and env vars
- Changed `HostAgentConfig` from `*MountConfig` (source/target paths) to `*bool` (enabled/disabled)
- New `AssembleHostAgentConfig()` function resolves paths from registry at runtime, silently skips missing dirs (AC9)
- Added `--agent` CLI flag to override default agent at runtime
- Updated `agentCommand()` to use short names: `claude` -> `claude --dangerously-skip-permissions`, `gemini` -> `gemini -y`
- Dockerfile template now iterates `InstalledAgents` with `range`, installs all specified agents and copies instruction files for each using `cp` (not `mv`) to support multiple agents
- Old agent names (`claude-code`, `gemini-cli`) are rejected with AC6-format error messages
- All unit tests updated and passing (6 packages, 0 failures)
- Added new tests: multi-agent validation, default_agent defaulting, old name rejection, AssembleHostAgentConfig (7 new tests), agentCommand short names (4 new tests)

### Review Findings

- [x] [Review][Decision] Gemini registry values diverge from AC8 spec — Resolved: spec updated to match implementation (`GEMINI_CLI_HOME` / `/opt/gemini-home/.gemini`). [internal/config/config.go:35]
- [x] [Review][Decision] `host_agent_config: true` behaves identically to `nil` when host dir missing — Dismissed: silent skip in all cases is acceptable; agent surfaces auth errors at runtime. [internal/mount/mount.go:41-44]
- [x] [Review][Patch] Gemini agent requires NodeJS SDK but no parse-time validation guard — Fixed: added validation in parse.go. [internal/config/parse.go]
- [x] [Review][Patch] Permission error on host config dir silently treated as missing — Fixed: now uses os.IsNotExist check, returns error on permission failures. [internal/mount/mount.go:57-63]
- [x] [Review][Patch] Duplicate agent names in `installed_agents` not rejected — Fixed: added duplicate check in parse.go. [internal/config/parse.go:51-63]
- [x] [Review][Defer] No cmd-level tests for `--agent` flag path [cmd/run_test.go] — deferred, story 9-5 explicitly planned for multi-agent config and flag tests

### Change Log

- 2026-04-11: Implemented multi-agent runtime support — all 8 tasks complete, all ACs satisfied

### File List

- internal/config/config.go (modified — struct changes, new types and registry)
- internal/config/parse.go (modified — new validation logic, exported validators)
- cmd/run.go (modified — --agent flag, override logic, updated agent functions)
- internal/mount/mount.go (modified — simplified AssembleMounts, new AssembleHostAgentConfig)
- embed/Dockerfile.tmpl (modified — range iteration over InstalledAgents)
- embed/config.yaml (modified — restructured config template)
- internal/config/parse_test.go (modified — updated configs, added multi-agent tests)
- internal/mount/mount_test.go (modified — removed old HostAgentConfig tests, added AssembleHostAgentConfig tests)
- cmd/run_test.go (modified — updated configs, added agentCommand tests)
- internal/template/render_test.go (modified — updated config struct literals)
- integration/testdata/config.yaml (modified — updated to installed_agents format)
- integration/integration_test.go (modified — updated config struct literals)
- integration/lifecycle_test.go (modified — updated config struct literals)
- integration/mcp_test.go (modified — updated config struct literals)
- integration/mount_test.go (modified — updated config string)
- integration/isolate_deps_test.go (modified — updated config strings)
- integration/bmad_repos_test.go (modified — updated config strings)
