---
date: '2026-04-23'
author: Manuel
status: proposed
scope: moderate
---

# Sprint Change Proposal — Configurable Agent Instructions Extension

## Section 1: Issue Summary

Today, asbox generates the agent instruction file (`CLAUDE.md` / `GEMINI.md` / `AGENTS.md`) from a single embedded template — users cannot inject project-specific instructions without forking or editing the binary. Individual projects have recurring, project-specific constraints an agent needs to know (coding conventions, deployment pipelines, "don't run migrations against prod", etc.) that don't belong in the generic sandbox template.

The obvious workaround — dropping a `CLAUDE.md` at the project root — only works for one agent and breaks when the user switches runtime via `--agent`. The value this feature delivers is **DRY**: one extension file, applied to every supported agent via the existing shared template.

**Triggering context:** Flagged by the user on 2026-04-23 as a focused, limited feature to insert into the PRD, architecture, and sprint.

## Section 2: Impact Analysis

### Epic Impact

- **New Epic 16** (`Project-Specific Agent Instructions Extension`) — one story, self-contained. Epic 8 (`Developer Can Run Multi-Repo BMAD Workflows`) is closed and shipped; adding this to a closed epic would muddy its scope. New epic is cleaner.
- No other epics are impacted (all others closed or in-progress on unrelated work).

### Story Impact

- **New Story 16.1:** Configurable Agent Instructions Extension.
- **No existing stories require changes.** Story 8.1 is done but its code needs a light refactor (the instruction generator broadens from "only when bmad_repos set" to "whenever bmad_repos OR agent_instructions is set"). This is delivery work inside Story 16.1, not a change to Story 8.1's scope.

### Artifact Conflicts

- **PRD** (`_bmad-output/planning-artifacts/prd.md`): new FR68, Configuration Surface update, YAML example update, Runtime behavior update, new editHistory entry.
- **Architecture** (`_bmad-output/planning-artifacts/architecture.md`): new decision section, File Responsibilities update, Requirements to Structure Mapping row.
- **Epics** (`_bmad-output/planning-artifacts/epics.md`): new FR68 in requirements inventory, new FR Coverage Map row, new Epic 16 entry in Epic List, new Epic 16 + Story 16.1 block.
- **Sprint status** (`_bmad-output/implementation-artifacts/sprint-status.yaml`): new `epic-16` + `16-1-configurable-agent-instructions-extension` entries.

### Technical Impact

- **`internal/config/config.go`** — add `AgentInstructions string `yaml:"agent_instructions"`` field to `Config` struct.
- **`internal/mount/bmad_repos.go`** — rename and broaden. Current function `AssembleBmadRepos(cfg)` returns `(mounts, instructionContent, error)` only when `bmad_repos` is non-empty. Refactor to `AssembleAgentInstructions(cfg, configPath)` that returns `(bmadMounts, instructionContent, error)` when **either** `bmad_repos` is set **or** `agent_instructions` is set. File is renamed to `agent_instructions.go` (mount logic for bmad_repos stays in the same function — they're co-located because they share the same output file).
- **`embed/agent-instructions.md.tmpl`** — add a new trailing conditional block `{{if .ProjectExtension}} ... {{.ProjectExtension}} ... {{end}}` that renders the extension content after the Best Practices section.
- **`internal/mount/bmad_repos_test.go`** — rename to `agent_instructions_test.go`, extend test coverage to cover the new field paths.
- **`cmd/run.go`** — rename call site (`mount.AssembleBmadRepos` → `mount.AssembleAgentInstructions`), pass config file path for relative-path resolution, and broaden the condition that generates/mounts the instruction temp file (no longer gated on `len(bmadMounts) > 0`).
- **No changes** to error handling surface, exit code mapping, or boundary enforcement — this fits cleanly into the existing `ConfigError` path.

## Section 3: Recommended Approach

**Path forward:** Direct Adjustment — modify the PRD, architecture, epics, and sprint status to add one new FR, one new epic, one new story. Implementation is a small Go refactor + template extension + tests.

**Rationale:** The feature is scoped, reuses existing infrastructure (Go template, mount assembly, agent-target lookup), and follows the `bmad_repos` fail-closed precedent. No architectural redesign, no new packages, no new trust boundaries.

**Effort estimate:** ~1 story, ~0.5–1 day. Light code surface (one struct field, one function rename+broaden, one template conditional, ~3 new test cases).

**Risk assessment:** Low. The refactor keeps the existing bmad_repos behavior byte-identical when `agent_instructions` is unset. The fail-closed missing-file policy is consistent with existing patterns — no new user-surprising failure modes.

**Timeline impact:** None. Epic 11 (hardening) is in-progress with no dependency on this; Epic 13 is in-progress with no overlap (different file, different concern).

## Section 4: Detailed Change Proposals

### 4.1 PRD — new FR68

**File:** `_bmad-output/planning-artifacts/prd.md`
**Location:** Under `## Functional Requirements` → `### Sandbox Configuration` subsection, inserted after FR61.

**Add:**

```markdown
- FR68: Developer can optionally set `agent_instructions` in `.asbox/config.yaml` to the path of a markdown file (resolved relative to the config file location) whose contents are appended to the generated agent instruction file (`CLAUDE.md` / `GEMINI.md` / `AGENTS.md`). The extension is applied uniformly across all installed agents via the shared template — one file, one set of project-specific instructions, DRY across agent runtimes. If `agent_instructions` is set but the file does not exist or is not readable at launch, the CLI aborts with exit code 1 and a descriptive error naming the offending path and the config field — fail-closed, consistent with `bmad_repos` (FR52). If `agent_instructions` is unset or empty, behavior is unchanged from today. When `agent_instructions` is set, the instruction file is generated and mounted into the container even if `bmad_repos` is empty — the instruction mount is no longer gated solely on `bmad_repos`.
```

**Rationale:** Captures the core user capability, the DRY multi-agent goal, the fail-closed policy (matching FR52), and the explicit decoupling from bmad_repos.

### 4.2 PRD — Configuration Surface narrative update

**File:** `_bmad-output/planning-artifacts/prd.md`
**Location:** `### Configuration Surface` bullet list.

**Add new bullet** (right after the `BMAD multi-repo workflow (bmad_repos)` bullet):

```markdown
- **Project agent instructions extension (`agent_instructions`):** Optional path to a markdown file (resolved relative to the config file) whose contents are appended to the generated agent instruction file at launch. Lets a project layer on its own conventions, constraints, or coding standards without forking asbox or maintaining per-agent `CLAUDE.md` / `GEMINI.md` / `AGENTS.md` copies — one extension file is applied to whichever agent runtime is launched. Fail-closed on missing/unreadable file (same policy as `bmad_repos`).
```

### 4.3 PRD — Example YAML update

**File:** `_bmad-output/planning-artifacts/prd.md`
**Location:** The `yaml` example block inside `### Configuration Surface`.

**OLD (end of block):**

```yaml
secrets:
  - ANTHROPIC_API_KEY
env:
  GIT_AUTHOR_NAME: "Sandbox Agent"
  GIT_AUTHOR_EMAIL: "sandbox@localhost"
  NODE_ENV: development
```

**NEW:**

```yaml
agent_instructions: ./AGENT_INSTRUCTIONS.md
secrets:
  - ANTHROPIC_API_KEY
env:
  GIT_AUTHOR_NAME: "Sandbox Agent"
  GIT_AUTHOR_EMAIL: "sandbox@localhost"
  NODE_ENV: development
```

**Rationale:** Shows the field in context of a realistic config.

### 4.4 PRD — Runtime behavior update

**File:** `_bmad-output/planning-artifacts/prd.md`
**Location:** `### Technical Architecture Considerations` → `**Runtime behavior:**` bullet list. Insert immediately after the `When bmad_repos is configured` bullet.

**Add:**

```markdown
- When `agent_instructions` is configured: at launch, the referenced file is read from disk (path resolved relative to the config file) and its contents are injected into the `ProjectExtension` template variable. The rendered agent instruction file (`CLAUDE.md` / `GEMINI.md` / `AGENTS.md`) appends a trailing "Project-Specific Instructions" section containing the extension content verbatim, preserving markdown structure. If the path does not exist or is not readable, the launch aborts with exit code 1 and a descriptive error. If both `bmad_repos` and `agent_instructions` are set, both contribute to the same generated file — multi-repo section from `bmad_repos`, extension section from `agent_instructions`. If `agent_instructions` is set but `bmad_repos` is empty, the file is still generated and mounted.
```

### 4.5 PRD — editHistory entry

**File:** `_bmad-output/planning-artifacts/prd.md`
**Location:** Front matter `editHistory` list (prepend new entry) and `lastEdited` field.

**Update:**

```yaml
lastEdited: '2026-04-23'
editHistory:
  - date: '2026-04-23'
    changes: 'Course correction: add FR68 (agent_instructions extension). Optional config field pointing to a project-local markdown file that is appended to the generated agent instruction file (CLAUDE.md / GEMINI.md / AGENTS.md) for any installed agent — DRY project-specific instructions across agent runtimes. Fail-closed on missing/unreadable file (matches bmad_repos FR52). Decouples instruction-file generation from bmad_repos presence. Updated Configuration Surface, example YAML, and Runtime behavior sections. Spawns Epic 16. Per sprint-change-proposal-2026-04-23.md.'
  - date: '2026-04-17'
    changes: 'Epic 13 UX review ...'
  # ... remaining entries unchanged
```

### 4.6 Architecture — new decision section

**File:** `_bmad-output/planning-artifacts/architecture.md`
**Location:** Inserted as a new section after `### BMAD Multi-Repo Workflow (`bmad_repos`)` and before `### Host Agent Config (`host_agent_config`)`.

**Add:**

```markdown
### Project-Specific Agent Instructions Extension (`agent_instructions`)

- **Decision:** Optional config field `agent_instructions` (string path, relative to config file location) pointing to a markdown file whose contents are appended to the generated agent instruction file via a new `{{if .ProjectExtension}}` block in `embed/agent-instructions.md.tmpl`. Applies uniformly to whichever agent launches (claude/gemini/codex) because the template output is agent-agnostic — only the mount target changes (`agentInstructionTarget()` is unchanged).
- **Rationale:** Projects have project-specific constraints (conventions, deployment gates, "don't touch X") that don't belong in the generic template and shouldn't require forking asbox. A project-root `CLAUDE.md` works for one agent but breaks when the user switches runtime via `--agent` — this violates DRY. One extension file applied through the shared template solves both.
- **Fail-closed on missing file:** If `agent_instructions` is set but the file does not exist or is not readable, `AssembleAgentInstructions()` returns `ConfigError{Msg: "agent_instructions path '<path>' not found. Check agent_instructions in .asbox/config.yaml"}` (or "is not readable") and the CLI exits with code 1. Matches the `bmad_repos` policy (fail-closed for declared workspace inputs) — opposite of `host_agent_config` (silent-skip for optional OAuth convenience). Rationale: if the user explicitly names a file, a missing file is a user error, not a convenience. Silent-skip would let a typo become "the agent silently ignored my project conventions for three weeks."
- **Decoupling from bmad_repos:** Current `AssembleBmadRepos()` generates and mounts the instruction file only when `bmad_repos` is non-empty. This function is renamed/broadened to `AssembleAgentInstructions(cfg, configPath) (bmadMounts []string, instructionContent string, error)` and runs whenever **either** `bmad_repos` is non-empty **or** `agent_instructions` is set. File renames from `internal/mount/bmad_repos.go` to `internal/mount/agent_instructions.go`; the bmad_repos mount-assembly logic stays co-located because it shares the output file. Existing behavior (bmad_repos set, agent_instructions unset) is byte-identical.
- **Template integration:** The template gains a new data field `ProjectExtension string` on `InstructionData`. The trailing block is:
  ```gotemplate
  {{- if .ProjectExtension}}

  ## Project-Specific Instructions

  {{.ProjectExtension}}
  {{- end}}
  ```
  Verbatim-passthrough — no further transformation of extension content. The extension owns its internal markdown structure.
- **Path resolution:** `agent_instructions` resolves relative to the config file's directory (same rule as `mounts`). Matches the existing path-resolution convention (see Cross-Cutting Concerns).
- **Validation location:** Runs at runtime in `internal/mount/agent_instructions.go`, not at config-parse time. Matches the pattern used by `bmad_repos`, `mounts`, and `isolate_deps` — keeps `internal/config/parse.go` pure (no filesystem dependency).
- **Content-hash impact:** None. The extension content is read at runtime, not baked into the image — bumping the extension file does NOT trigger an image rebuild. The extension lives at the instruction-file-mount layer, parallel to bmad_repos (which also does not trigger rebuilds).
- **Affects:** `internal/config/config.go` (new `AgentInstructions` field), `internal/mount/agent_instructions.go` (renamed file, broadened function, new validation), `embed/agent-instructions.md.tmpl` (new trailing block), `cmd/run.go` (rename call site, decouple mount-gating condition from `len(bmadMounts) > 0`)
```

### 4.7 Architecture — File Responsibilities update

**File:** `_bmad-output/planning-artifacts/architecture.md`
**Location:** `### Complete Project Directory Structure` tree and `### File Responsibilities` section. In both, rename `bmad_repos.go` → `agent_instructions.go` and `bmad_repos_test.go` → `agent_instructions_test.go`.

Also update the structure tree comment for the renamed file:

**OLD:**

```
│   │   ├── bmad_repos.go            # AssembleBmadRepos() — repo mounts, collision detection, instruction gen
```

**NEW:**

```
│   │   ├── agent_instructions.go    # AssembleAgentInstructions() — bmad_repos mounts + collision detection + instruction file gen with project extension
```

### 4.8 Architecture — Requirements to Structure Mapping row

**File:** `_bmad-output/planning-artifacts/architecture.md`
**Location:** `### Requirements to Structure Mapping` table. Append after the FR67 row.

**Add:**

```markdown
| FR68 (agent_instructions extension) | `internal/mount/agent_instructions.go`, `embed/agent-instructions.md.tmpl` | `AssembleAgentInstructions()` + `{{if .ProjectExtension}}` template block |
```

### 4.9 Epics — requirements inventory, FR coverage map, Epic List, Epic 16

**File:** `_bmad-output/planning-artifacts/epics.md`

**Change 9a:** In `### Functional Requirements` list (alphabetical by FR number), append after FR67:

```markdown
FR68: Developer can set `agent_instructions` in config to a markdown file path; its contents are appended to the generated agent instruction file (CLAUDE.md / GEMINI.md / AGENTS.md), uniformly across all installed agents via the shared template. Fail-closed on missing/unreadable file. Decouples instruction-file generation from bmad_repos presence
```

**Change 9b:** In `### FR Coverage Map` table, append row:

```markdown
| FR68 | Epic 16 | agent_instructions extension — project-specific markdown appended to agent instruction file |
```

**Change 9c:** In `## Epic List`, append after Epic 15:

```markdown
### Epic 16: Project-Specific Agent Instructions Extension
A developer can drop a project-local markdown file (`agent_instructions` config field) that gets appended to the generated agent instruction file, applied uniformly to whichever agent launches (claude/gemini/codex) via the shared template. DRY: one extension file covers every agent runtime. Fail-closed on missing/unreadable file (matches bmad_repos policy). The instruction-file generation/mount is decoupled from bmad_repos presence — it now runs whenever agent_instructions OR bmad_repos is set.
**FRs covered:** FR68
```

**Change 9d:** Append new section at end of file (after Story 15.1):

```markdown
## Epic 16: Project-Specific Agent Instructions Extension

A developer can inject project-specific instructions into the generated agent instruction file via a single config field, applied uniformly across all installed agents. Solves the DRY gap where a project-root `CLAUDE.md` worked for only one agent and broke under `--agent` overrides.

### Story 16.1: Configurable Agent Instructions Extension

As a developer,
I want to configure a path to a project-local markdown file that gets appended to the agent's instruction file,
So that my project-specific constraints and conventions are visible to whichever agent I launch — without forking asbox or duplicating a file per agent.

**Acceptance Criteria:**

**Given** a developer sets `agent_instructions: ./AGENT_INSTRUCTIONS.md` in `.asbox/config.yaml` and the file exists
**When** they run `asbox run`
**Then** the rendered agent instruction file mounted into the container contains the extension file's contents under a trailing `## Project-Specific Instructions` section, verbatim

**Given** a developer sets `agent_instructions` and installs both `claude` and `gemini`
**When** they launch with `-a claude` and then separately with `-a gemini`
**Then** both runs produce an instruction file with the same project-specific section (the mount target differs per agent via `agentInstructionTarget()` but the template output is agent-agnostic)

**Given** `agent_instructions` is set but the file does not exist at launch
**When** the CLI runs
**Then** it exits with code 1 and prints `"error: agent_instructions path '<path>' not found. Check agent_instructions in .asbox/config.yaml"` to stderr — no partial launch

**Given** `agent_instructions` is set but the file exists and is not readable (permissions)
**When** the CLI runs
**Then** it exits with code 1 with a descriptive error naming the path and the config field

**Given** `agent_instructions` is unset or empty
**When** the CLI runs
**Then** behavior is unchanged — the instruction file is generated and mounted only when `bmad_repos` is non-empty (existing behavior preserved byte-identically)

**Given** `agent_instructions` is set and `bmad_repos` is empty
**When** the CLI runs
**Then** the instruction file is still generated and mounted — the mount is no longer gated solely on `bmad_repos`

**Given** `agent_instructions` is set and `bmad_repos` is also non-empty
**When** the CLI runs
**Then** the rendered file contains both the Multi-Repo Workspace section (from `bmad_repos`) and the Project-Specific Instructions section (from `agent_instructions`)

**Given** the `agent_instructions` value is a relative path
**When** the CLI resolves it
**Then** it is resolved relative to the directory containing the config file, not the working directory (same rule as `mounts`)

**Implementation Notes:**

- `internal/config/config.go`: add `AgentInstructions string `yaml:"agent_instructions"`` to `Config` struct. No validation in `parse.go` — deferred to mount-assembly.
- `internal/mount/bmad_repos.go` → rename to `internal/mount/agent_instructions.go`; rename `AssembleBmadRepos(cfg)` → `AssembleAgentInstructions(cfg, configPath)`. Broaden the early-return guard: run whenever `len(cfg.BmadRepos) > 0` OR `cfg.AgentInstructions != ""`. When only `agent_instructions` is set, `bmadMounts` stays empty and only the instruction file is produced.
- File read: `os.ReadFile(resolved-path)`; `os.IsNotExist(err)` → fail-closed ConfigError with `not found` message; other read errors → ConfigError with `is not readable: <err>` message. The `configPath` argument gives the base directory for relative-path resolution via `filepath.Join(filepath.Dir(configPath), cfg.AgentInstructions)` — then `filepath.Abs`.
- Template data: extend `InstructionData` with `ProjectExtension string`. Assign the read file content (not the path) to this field.
- Template: append the trailing block shown in the architecture decision section. Trim markers (`{{-`) matter — avoid leading blank lines when `bmad_repos` is also set.
- `internal/mount/bmad_repos_test.go` → rename to `agent_instructions_test.go`. New test cases:
  - `agent_instructions` set, file exists → rendered content contains `## Project-Specific Instructions` and the file body verbatim.
  - `agent_instructions` set, file missing → ConfigError with `not found`.
  - `agent_instructions` set, `bmad_repos` empty → mounts list is empty but `instructionContent` is non-empty.
  - `agent_instructions` set, `bmad_repos` non-empty → rendered content contains both sections.
  - `agent_instructions` unset, `bmad_repos` non-empty → rendered content unchanged from today (regression guard).
  - Relative-path resolution against the config-file directory.
- `cmd/run.go`: rename the call site; pass `configFile` (already a local in RunE). Change the condition `if instructionContent != ""` is already correct (no change needed — it already triggers when either bmad_repos OR the extension produces content). The `if len(bmadMounts) > 0` block for the "bmad_repos: mounting N" log stays gated on `bmadMounts` (log is about repo mounts, not about the instruction file).
- Exit code mapping: no changes to `cmd/root.go`. `ConfigError` already maps to exit code 1.
- `embed/config.yaml` (starter): add a commented-out example line:
  ```yaml
  # Optional: append project-specific instructions to the generated agent file
  # (CLAUDE.md / GEMINI.md / AGENTS.md). Path is relative to this config file.
  # agent_instructions: ../AGENT_INSTRUCTIONS.md
  ```
- No integration test required — the behavior is observable via a binary invocation test plus the unit tests above. Add an integration test only if the existing bmad_repos integration coverage doesn't exercise the instruction-file mount path (check `integration/` first; if it does, the refactor is already covered).
```

### 4.10 Sprint status — new epic and story entries

**File:** `_bmad-output/implementation-artifacts/sprint-status.yaml`
**Location:** `development_status:` map. Append after Epic 15 block.

**Add:**

```yaml
  # Epic 16: Project-Specific Agent Instructions Extension (sprint change 2026-04-23)
  epic-16: backlog
  16-1-configurable-agent-instructions-extension: backlog
  epic-16-retrospective: optional
```

Also bump the `last_updated` field at the top of the file to `"2026-04-23"`.

## Section 5: Implementation Handoff

**Scope classification:** **Moderate.** A new FR, a new epic, a new story, artifact updates across PRD/architecture/epics/sprint-status, plus implementation work (Go struct field, file rename, function rename+broaden, template block, 5–6 test cases). No new packages, no new trust boundaries, no new runtime dependencies.

**Handoff recipients:**
- **Manuel (PO/Dev):** approve this proposal, then apply the PRD/architecture/epics/sprint-status edits (Section 4.1–4.10).
- **Dev (Amelia / bmad-agent-dev):** once artifacts are updated, pick up Story 16.1 from `_bmad-output/implementation-artifacts/16-1-configurable-agent-instructions-extension.md` (to be created via `/bmad-create-story` or equivalent). The story file is already drafted content-wise in Section 4.9 above.

**Success criteria for implementation:**
- All acceptance criteria in Story 16.1 pass.
- `go test ./...` passes (including the renamed/extended test file).
- `go vet` and `gofmt` clean.
- Binary invocation test: with `agent_instructions` set to a small fixture file and `bmad_repos` empty, `asbox run` renders an instruction file containing the extension section and mounts it to the correct agent-specific target.
- Regression guard: with `agent_instructions` unset and `bmad_repos` set, the rendered output is byte-identical to today's.
- No changes to exit-code semantics, error-type surface, or boundary enforcement.
