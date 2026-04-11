# Sprint Change Proposal - Multi-Agent Runtime Support

**Date:** 2026-04-11
**Triggered by:** Multi-agent usability friction during real usage
**Scope classification:** Moderate
**Recommended approach:** Direct Adjustment

---

## Section 1: Issue Summary

### Problem Statement

asbox assumes single-agent workflows. Switching between Claude and Gemini requires editing `.asbox/config.yaml` each time, and `host_agent_config` forces the user to manually specify paths that are predictable per agent. This creates unnecessary friction for developers who work with multiple AI agents.

### Context

Discovered through real multi-agent usage patterns. The current `agent` field name is misleading -- it's a default, not a fixed binding. The `host_agent_config` mount paths are entirely predictable given the agent name, yet the user must specify them manually.

### Evidence

- Config requires editing `agent: claude-code` -> `agent: gemini-cli` to switch agents
- `host_agent_config` requires manual source/target paths that are deterministic per agent
- The embedded config template already documents per-agent defaults in comments (`embed/config.yaml` lines 56-58)
- Agent name suffixes (`-code`, `-cli`) add unnecessary verbosity

---

## Section 2: Impact Analysis

### Epic Impact

| Epic | Impact | Details |
|------|--------|---------|
| Epic 1 (Build and Launch) | Modified | Config struct changes, new CLI flag, Dockerfile template iterates `installed_agents` |
| Epic 2 (Agent Work) | Minimal | `cfg.Agent` references become `cfg.DefaultAgent`, resolved value unchanged |
| Epic 7 (Host Agent Auth) | Simplified | `host_agent_config` becomes boolean with automatic path resolution |
| Epic 8 (bmad_repos) | Minimal | Agent name references updated to short names |
| Epic 9 (Integration Tests) | Modified | Test fixtures need updated config fields and agent names |
| Epics 3-6 | None | No impact |

### Story Impact

- **Story 1.2 (Config Parsing):** Acceptance criteria amended for `installed_agents`, `default_agent`, short agent names, boolean `host_agent_config`
- **Story 1.5 (Container Scripts):** Dockerfile template iterates `installed_agents` for agent installation
- **Story 1.7 (Sandbox Run):** New `--agent` flag, agent override logic, runtime host config resolution
- **Epic 7 stories:** Rewritten -- simpler boolean logic replaces MountConfig validation

### Artifact Conflicts

- **PRD:** FR7, FR9d, FR45 need amendments
- **Architecture:** Config struct definition, agent resolution flow, host_agent_config section
- **Embedded config template:** Restructured for new fields
- **Integration tests:** Config fixtures updated

### Technical Impact

- **Breaking config change:** `agent` -> `installed_agents` + `default_agent`, `host_agent_config` source/target -> boolean
- **Acceptable:** Pre-1.0 tool with small user base, single migration moment
- **Image rebuild required:** Adding agents to `installed_agents` changes the Dockerfile, triggers content-hash rebuild
- **Gemini yolo mode:** Gemini CLI now runs with `-y` flag inside sandbox (no permission prompts)

---

## Section 3: Recommended Approach

**Selected: Direct Adjustment**

All changes are additive refactoring within the existing architecture. No rollbacks, no new architectural patterns, no MVP scope change.

**Effort:** Low-Medium
**Risk:** Low
**Timeline impact:** None -- this is a quality-of-life improvement that simplifies ongoing development

### Alternatives Considered

- **Rollback:** Not applicable (nothing to roll back)
- **MVP Review:** Not applicable (MVP unaffected)

---

## Section 4: Detailed Change Proposals

### Change 1: Rename `agent` to `default_agent` in config struct and parsing

- `Config.Agent` -> `Config.DefaultAgent` with YAML tag `default_agent`
- Validation error messages updated
- Field is optional -- defaults to first entry in `installed_agents` if omitted

### Change 2: Add `--agent` CLI flag to `asbox run`

- New `--agent` string flag on `run` command
- Validates against supported agents and `installed_agents` list
- Overrides `cfg.DefaultAgent` before any downstream code reads it
- New exported `ValidateAgent()` and `ValidateAgentInstalled()` functions in config package

### Change 3: Add `installed_agents` field + short agent names

- New `Config.InstalledAgents []string` with YAML tag `installed_agents`
- Required field -- at least one agent must be listed
- `default_agent` must be in `installed_agents`
- Short names: `claude` (was `claude-code`), `gemini` (was `gemini-cli`)
- `agentCommand()` updated: `claude` -> `claude --dangerously-skip-permissions`, `gemini` -> `gemini -y`
- Dockerfile template iterates `installed_agents` to install all listed agents at build time
- All references throughout codebase updated to short names

### Change 4: Simplify `host_agent_config` to boolean with automatic path resolution

- `Config.HostAgentConfig` changes from `*MountConfig` to `*bool`
- New `AgentConfigRegistry` maps agent names to `{Source, Target, EnvVar}`
  - `claude`: `~/.claude` -> `/opt/claude-config`, `CLAUDE_CONFIG_DIR`
  - `gemini`: `~/.gemini` -> `/opt/gemini-config`, `GEMINI_CONFIG_DIR`
- `AssembleHostAgentConfig(agent)` resolves paths at runtime from registry
- Enabled by default (`nil` = true). Set `host_agent_config: false` to disable
- Silently skips if host config directory doesn't exist (agent not set up on host)
- Config validation and path resolution blocks removed from `parse.go`
- `AssembleMounts()` simplified to only handle regular mounts

---

## Section 5: Implementation Handoff

### Scope: Moderate

Changes span config parsing, CLI flags, mount assembly, Dockerfile template, and embedded config -- multiple packages affected but all within existing architecture.

### Handoff: Developer Agent

**Files to modify:**

| File | Changes |
|------|---------|
| `internal/config/config.go` | Rename field, change type, add `AgentConfigMapping` + registry |
| `internal/config/parse.go` | New validation logic, remove old validation, add exported validators |
| `cmd/run.go` | Add `--agent` flag, agent override logic, host config resolution |
| `internal/mount/mount.go` | Simplify `AssembleMounts`, add `AssembleHostAgentConfig` |
| `embed/config.yaml` | Restructure for new fields |
| `embed/Dockerfile.tmpl` | Iterate `installed_agents` for agent installation |
| `internal/mount/bmad_repos.go` | Update agent name references |
| `integration/testdata/config.yaml` | Update fixture |
| `*_test.go` | Update all tests for renamed fields and short names |

### Success Criteria

- `asbox run` launches with `default_agent` (or first `installed_agents` entry)
- `asbox run --agent gemini` overrides to gemini without config edit
- `asbox run --agent gemini` fails clearly if gemini not in `installed_agents`
- `host_agent_config` auto-mounts correct directory for selected agent
- `host_agent_config: false` disables the mount
- Missing host config directory (e.g., `~/.gemini` doesn't exist) silently skips
- Both agents run in yolo mode (`claude --dangerously-skip-permissions`, `gemini -y`)
- Image builds install all agents listed in `installed_agents`
- Old config with `agent:` field fails with clear migration error message
