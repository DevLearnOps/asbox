# Sprint Change Proposal: Codex Agent Support

**Date:** 2026-04-14
**Author:** Manuel
**Scope:** Minor
**Status:** Approved

---

## Section 1: Issue Summary

### Problem Statement

asbox currently supports two sandboxed AI agents: Claude Code (`claude`) and Gemini CLI (`gemini`). OpenAI's Codex CLI is now a viable third agent for autonomous coding tasks. The multi-agent infrastructure established in Story 1.9 (installed_agents, default_agent, --agent flag, AgentConfigRegistry, host_agent_config boolean) was designed for extensibility but currently only knows two agents.

### Context

- Story 1.9 (Multi-Agent Runtime Support) was completed on 2026-04-13, establishing the extensible multi-agent pattern
- Codex CLI is available as `@openai/codex` npm package
- Codex supports `--dangerously-bypass-approvals-and-sandbox` for use inside isolated environments (mirrors Claude's `--dangerously-skip-permissions` and Gemini's `-y`)
- Codex uses `CODEX_HOME` env var for config directory (defaults to `~/.codex`)
- Codex requires Node.js (npm package, same as Gemini)

### Evidence

- [OpenAI Codex CLI docs](https://developers.openai.com/codex/cli)
- [@openai/codex npm package](https://www.npmjs.com/package/@openai/codex)
- [Config basics](https://developers.openai.com/codex/config-basic) -- confirms `CODEX_HOME` env var and `~/.codex` directory
- [Agent approvals & security](https://developers.openai.com/codex/agent-approvals-security) -- confirms `--dangerously-bypass-approvals-and-sandbox` flag

---

## Section 2: Impact Analysis

### Epic Impact

| Epic | Impact | Details |
|------|--------|---------|
| Epic 1 | **New story** | Story 1.10: Codex agent installation, config registry, command mapping, host_agent_config, instruction file |
| Epic 2 | **FR addition** | FR19a: Codex launch command. Added to FRs covered list |
| Epic 7 | **Scope extension** | host_agent_config registry gains codex entry (`~/.codex` -> `/opt/codex-config`, `CODEX_HOME`) |
| Epic 8 | **Scope extension** | BMAD repos generates `CODEX.md` instruction file alongside CLAUDE.md/GEMINI.md |
| Epic 9 | **New story** | Story 9.6: Codex agent integration tests |
| Epics 3-6, 10 | **No impact** | Isolation, inner containers, MCP, auto_isolate_deps, legacy removal are agent-agnostic |

### Story Impact

**New stories:**
- **Story 1.10: Codex Agent Support** -- config registry, validation, Dockerfile template, command mapping, instruction file
- **Story 9.6: Codex Agent Config and Runtime Tests** -- integration test coverage for codex

**Modified stories (documentation updates only, already done):**
- Story 1.9 -- description, error message AC, implementation notes updated to reference codex
- Story 8.1 -- instruction file generation AC updated to include CODEX.md
- Story 9.5 -- no changes needed (existing claude/gemini tests remain valid)

### Artifact Conflicts

**PRD (15 edits):**
- Executive Summary: add Codex CLI to agent list
- FR7: add `codex` to valid agent names
- FR9d: add codex config directory mapping (`~/.codex` -> `/opt/codex-config`)
- FR19a (new): Codex CLI launch with `--dangerously-bypass-approvals-and-sandbox`
- FR44: add `CODEX.md` to instruction files
- FR45: add `CODEX_HOME` env var
- FR58: add `codex` short name mapping
- Configuration Surface: installed agents, default agent command mapping, host agent config paths, example YAML
- Journey Requirements Summary: add Codex CLI to runtime capability row
- Technical Architecture: agent CLI installation, runtime behavior host agent config, build-time instruction files

**Architecture (8 edits):**
- Requirements Overview: sandbox configuration agent names, agent runtime description
- Host Agent Config decision: add codex to registry table
- BMAD repos: add CODEX.md to instruction file references
- Project structure comments (2 locations): add CODEX.md
- Data flow diagram: add codex to exec agent line
- Gap analysis: add CODEX.md to FR44 reference

**Epics (12 edits):**
- Requirements inventory: FR7, FR19a (new), FR44
- FR Coverage Map: add FR19a, update FR58
- Epic 2 FRs covered: add FR19a
- Story 1.9: description, error message AC, implementation notes
- Story 8.1: instruction file AC
- New Story 1.10: Codex Agent Support
- New Story 9.6: Codex Agent Integration Tests
- Sprint status: register new stories

### Technical Impact

**Code changes required (all additive, no behavior modifications):**

| File | Change |
|------|--------|
| `internal/config/config.go` | Add codex entry to `AgentConfigRegistry`: `{Source: "~/.codex", Target: "/opt/codex-config", EnvVar: "CODEX_HOME", EnvVal: "/opt/codex-config"}` |
| `internal/config/parse.go` | Add `"codex": true` to `validAgents` map; add codex to nodejs requirement validation alongside gemini |
| `cmd/run.go` | Add `"codex"` case to `agentCommand()` returning `"codex --dangerously-bypass-approvals-and-sandbox"` |
| `embed/Dockerfile.tmpl` | Add codex installation block: `npm install -g @openai/codex` (same pattern as gemini); add codex case for `CODEX.md` instruction file |
| `embed/config.yaml` | Add codex as commented option in `installed_agents` |
| `integration/multi_agent_test.go` | Add codex-specific test cases |
| `internal/config/parse_test.go` | Add codex validation tests |
| `internal/mount/mount_test.go` | Add codex AssembleHostAgentConfig tests |

---

## Section 3: Recommended Approach

### Selected Path: Direct Adjustment

The multi-agent infrastructure from Story 1.9 was explicitly designed as a registry pattern to make adding agents trivial. Every touchpoint uses maps or lists that handle N agents:

- `validAgents` map -- add one entry
- `AgentConfigRegistry` map -- add one entry
- `agentCommand()` switch -- add one case
- Dockerfile template `{{range .InstalledAgents}}` -- add one `{{if}}` block
- Agent instruction file copy -- add one case

No architectural changes, no structural refactoring, no behavior modifications to existing agents.

### Rationale

- **Effort: Low** -- follows established pattern, every change mirrors existing gemini implementation
- **Risk: Low** -- all changes are additive; existing claude/gemini behavior is untouched
- **Timeline: Minimal** -- Story 1.10 is a single-story implementation; Story 9.6 extends existing test infrastructure
- **No rollback needed** -- nothing existing is modified
- **No MVP scope change** -- codex is additive within existing multi-agent capability

### Alternatives Considered

- **Rollback:** Not applicable -- nothing to roll back
- **PRD MVP Review:** Not needed -- codex is additive within existing scope, not a scope expansion

---

## Section 4: Detailed Change Proposals

### PRD Changes

#### 1. Executive Summary (line 57)
Add "Codex CLI" to the agent list: `(Claude Code, Gemini CLI, Codex CLI)`

#### 2. FR7 (line 453)
Add `codex` to valid agent names: `(claude, gemini, codex)`

#### 3. FR9d (line 459)
Add codex config mapping: `codex: ~/.codex -> /opt/codex-config`

#### 4. New FR19a (after line 477)
```
- FR19a: Codex CLI agent launches with approvals and sandbox bypassed
  (`codex --dangerously-bypass-approvals-and-sandbox`) since asbox provides isolation
```

#### 5. FR44 (line 512)
Add `CODEX.md` to instruction files list

#### 6. FR45 (line 513)
Add `CODEX_HOME for codex` to env var list

#### 7. FR58 (line 526)
Add `codex (maps to Codex CLI)` to short name mapping

#### 8. Configuration Surface -- Installed Agents (line 228)
Add `codex` to supported agents: `claude`, `gemini`, and `codex`

#### 9. Configuration Surface -- Default Agent (lines 229-231)
Add codex command mapping:
```
- `codex` -> `codex --dangerously-bypass-approvals-and-sandbox` (approvals and sandbox
  bypassed, asbox provides isolation). Installed via npm global install (`@openai/codex`).
```

#### 10. Configuration Surface -- Host Agent Config (line 232)
Add codex path: `codex: ~/.codex -> /opt/codex-config`

#### 11. Configuration Surface -- Example YAML (line 238-240)
Add `- codex` to `installed_agents` list

#### 12. Journey Requirements Summary (line 155)
Update row: `Claude Code / Gemini CLI / Codex CLI runtime`

#### 13. Technical Architecture -- Agent CLI Installation (line 324)
Add: `Codex CLI via npm (@openai/codex)`

#### 14. Runtime Behavior -- Host Agent Config (line 334)
Add: `CODEX_HOME for codex`

#### 15. Build-Time Instruction Files (line 325)
Add `CODEX.md` to the list

### Architecture Changes

#### 16. Requirements Overview -- Sandbox Configuration (line 32)
Add `codex` to short agent name list

#### 17. Requirements Overview -- Agent Runtime (line 34)
Add Codex CLI with `--dangerously-bypass-approvals-and-sandbox`; change "Both" to "All"

#### 18. Host Agent Config Registry (lines 284-286)
Add entry: `codex: ~/.codex -> /opt/codex-config, CODEX_HOME`

#### 19. BMAD Repos -- Agent Instructions (line 278)
Add `CODEX.md` to generated file list

#### 20-21. Project Structure Comments (lines 127, 485)
Add `CODEX.md` to directory structure comments

#### 22. Data Flow Diagram (line 592)
Update: `exec agent (claude/gemini/codex)`

#### 23. Gap Analysis (line 658)
Add `CODEX.md` to FR44 reference

### Epics Changes

#### 24. FR7 Requirements Inventory (line 29)
Update to short names with codex: `(claude, gemini, codex)`

#### 25. New FR19a in Requirements Inventory (after line 47)
Add: `FR19a: Codex CLI agent launches with approvals and sandbox bypassed`

#### 26. FR44 Requirements Inventory (line 73)
Add `CODEX.md`

#### 27. FR Coverage Map (after line 170, line 210)
Add row: `FR19a | Epic 2 | Codex CLI with --dangerously-bypass-approvals-and-sandbox`
Update FR58 row: `Short agent names (claude, gemini, codex)`

#### 28. Epic 2 FRs Covered (line 221)
Add `FR19a`

#### 29-31. Story 1.9 Updates (lines 555, 579-581, 605-610)
Update description, error message AC, and implementation notes to reference codex

#### 32. Story 8.1 (line 931)
Add `CODEX.md` to instruction file generation AC

#### 33. New Story 1.10: Codex Agent Support
Full story with ACs for installation, launch command, host_agent_config, instruction file, nodejs validation

#### 34. New Story 9.6: Codex Agent Config and Runtime Tests
Integration tests for codex installation, command resolution, host config mount, instruction file

#### 35. Sprint Status
Register Story 1.10 and Story 9.6 as `backlog`

---

## Section 5: Implementation Handoff

### Change Scope: Minor

All changes are additive, follow established patterns, and can be implemented directly by the Developer agent.

### Handoff Plan

| Role | Responsibility |
|------|---------------|
| Developer agent | Implement Story 1.10 (code changes across 5 files) |
| Developer agent | Implement Story 9.6 (extend integration tests) |
| Manuel | Apply document edits to PRD, Architecture, Epics (35 proposals) |
| Manuel | Update sprint-status.yaml with new stories |

### Implementation Sequence

1. **Document updates first** -- Apply all 35 artifact edits to PRD, Architecture, Epics
2. **Story 1.10** -- Code changes: config registry, validation, Dockerfile template, command mapping, starter config
3. **Story 9.6** -- Integration tests: extend multi_agent_test.go, parse_test.go, mount_test.go
4. **Verify** -- Run full test suite to confirm no regressions

### Success Criteria

- `asbox build` with `installed_agents: [codex]` produces an image with Codex CLI installed
- `asbox run --agent codex` launches with `codex --dangerously-bypass-approvals-and-sandbox`
- `host_agent_config` correctly mounts `~/.codex` with `CODEX_HOME` env var
- `CODEX.md` instruction file is present in built images
- All existing claude/gemini tests continue to pass
- New codex integration tests pass
- Config validation requires nodejs SDK when codex is in installed_agents
