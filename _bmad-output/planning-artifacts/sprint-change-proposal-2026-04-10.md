# Sprint Change Proposal — 2026-04-10

**Triggered by:** Gap discovered at intersection of Epic 6 (Auto Dependency Isolation) and Epic 8 (BMAD Multi-Repo Mounts)
**Scope Classification:** Minor
**Recommended Approach:** Direct Adjustment

---

## Section 1: Issue Summary

When `auto_isolate_deps: true` is combined with `bmad_repos`, the dependency isolation scan (`ScanDeps`) only scans primary project mounts (`cfg.Mounts`). BMAD repos — which are full code repositories containing their own `package.json` files and `node_modules/` — are excluded from the scan. This defeats the purpose of dependency isolation for the multi-repo workflow, causing the same macOS/Linux native module clash that `auto_isolate_deps` is designed to prevent.

**Discovery context:** Identified during review of Epic 6 and Epic 8 interaction. The PRD and architecture both describe these features independently but never specify behavior when used together. FR9b was written before FR51-53 (`bmad_repos`) were added in the 2026-04-06 course correction, and was never updated to account for the new mount source.

**Evidence:**
- `ScanDeps()` at `internal/mount/isolate_deps.go:25-62` iterates `cfg.Mounts` only
- PRD FR9b says "the system scans mounted project paths" — bmad_repos are mounted project paths but are not included
- Architecture `auto_isolate_deps` section references "each mount" without cross-referencing bmad_repos
- No FR, architecture section, or story addresses the intersection

---

## Section 2: Impact Analysis

### Epic Impact

| Epic | Impact | Details |
|------|--------|---------|
| Epic 6 | **Modified** | Story 6.1 scope extended to include bmad_repos paths in scan |
| Epic 8 | None | No changes — bmad_repos mounting logic is correct as-is |
| Epic 9 | Minor addition | Needs integration test case for auto_isolate_deps + bmad_repos combination |
| Epics 1-5, 7 | None | No interaction with scanning logic |

### Artifact Conflicts

| Artifact | Impact |
|----------|--------|
| PRD (FR9b) | Ambiguous — must explicitly include bmad_repos in scan scope |
| PRD (Additional Requirements) | Missing reference to bmad_repos in auto_isolate_deps bullet |
| PRD (Runtime behavior) | Missing bmad_repos as scan source |
| Architecture (Detection Logic) | Only references primary mounts — must add bmad_repos input |
| Architecture (Implementation) | Must document dual input source and container path derivation |
| Epics (Story 6.1) | Missing acceptance criteria for bmad_repos combination |
| Code (isolate_deps.go) | ScanDeps must accept bmad_repos as additional input |
| Code (isolate_deps_test.go) | Needs test cases for bmad_repos paths |
| entrypoint.sh | No change — already handles all paths in AUTO_ISOLATE_VOLUME_PATHS |

### Technical Impact

Low. The scan logic (`filepath.WalkDir`) is already generic. The volume naming and assembly functions work for any path. Only the input set to `ScanDeps` needs extension. Container path derivation for bmad_repos uses `/workspace/repos/<basename>` instead of the primary mount target — this is the only logic branch to add.

---

## Section 3: Recommended Approach

**Direct Adjustment** — modify Story 6.1 and update PRD/Architecture to close the gap.

**Rationale:**
- The scanning logic is already generic — `filepath.WalkDir` works on any directory
- Volume naming pattern works for any path
- `AssembleIsolateDeps` already produces generic volume flags
- Entrypoint chown requires zero changes
- The change is additive: extend the scan input set, not redesign the feature
- No timeline impact, no scope reduction, no rollback needed

**Alternatives considered:**
- **Rollback (Option 2):** Not viable — both features work correctly in isolation, rework is unjustified
- **MVP Review (Option 3):** Not applicable — MVP scope is not at risk

**Effort estimate:** Low
**Risk level:** Low
**Timeline impact:** None

---

## Section 4: Detailed Change Proposals

### 4.1 PRD — FR9b Clarification

**File:** `_bmad-output/planning-artifacts/prd.md`

OLD:
> FR9b: When `auto_isolate_deps` is enabled, the system scans mounted project paths at launch for `package.json` files and creates named Docker volumes for each corresponding `node_modules/` directory

NEW:
> FR9b: When `auto_isolate_deps` is enabled, the system scans all mounted project paths at launch — including primary mounts and `bmad_repos` mounts — for `package.json` files and creates named Docker volumes for each corresponding `node_modules/` directory

### 4.2 Architecture — Detection Logic

**File:** `_bmad-output/planning-artifacts/architecture.md`
**Section:** Automatic Dependency Isolation — Detection Logic

OLD:
```
**Detection Logic:**
- Triggered only when `auto_isolate_deps: true` in config
- For each mount: resolve host-side source path (relative to config file)
- Walk directory tree for `package.json` files, excluding `node_modules/` subtrees
- For each discovered `package.json`: derive `node_modules` sibling path
```

NEW:
```
**Detection Logic:**
- Triggered only when `auto_isolate_deps: true` in config
- For each primary mount: resolve host-side source path (relative to config file)
- For each `bmad_repos` entry: resolve host-side path and use its corresponding container target (`/workspace/repos/<basename>`)
- Walk all resolved directory trees for `package.json` files, excluding `node_modules/` subtrees
- For each discovered `package.json`: derive `node_modules` sibling path
```

### 4.3 Architecture — Implementation Reference

**File:** `_bmad-output/planning-artifacts/architecture.md`
**Section:** Automatic Dependency Isolation — Implementation

OLD:
```
**Implementation:** `internal/mount/` package, called from `cmd/run.go` after config parse, before Docker run command assembly
- **Affects:** `internal/mount/` (scan + volume flag generation), `embed/entrypoint.sh` (chown)
```

NEW:
```
**Implementation:** `internal/mount/` package, called from `cmd/run.go` after config parse, before Docker run command assembly. `ScanDeps` accepts both primary mounts and `bmad_repos` entries as scan inputs. For bmad_repos, container paths are derived from the `/workspace/repos/<basename>` convention rather than from mount target config.
- **Affects:** `internal/mount/` (scan + volume flag generation), `embed/entrypoint.sh` (chown)
```

### 4.4 Epics — Story 6.1 Acceptance Criteria

**File:** `_bmad-output/planning-artifacts/epics.md`
**Section:** Story 6.1 — new acceptance criteria added after monorepo criterion

Two new criteria added:

> **Given** `auto_isolate_deps: true` and `bmad_repos` configured with repos containing `package.json` files
> **When** the sandbox launches
> **Then** named volume mounts are also created for each `node_modules/` in the bmad_repos, using container paths under `/workspace/repos/<basename>/`
>
> **Given** `auto_isolate_deps: true` and `bmad_repos` with a monorepo containing nested `package.json` files
> **When** the sandbox launches
> **Then** all nested `node_modules/` directories within the bmad repo are isolated with named volumes

Logging criterion updated to clarify N includes both mount sources.

### 4.5 Epics — Story 6.1 Implementation Notes

**File:** `_bmad-output/planning-artifacts/epics.md`
**Section:** Story 6.1 — Implementation Notes

Updated to reflect:
- Correct function signature `ScanDeps(cfg *config.Config) ([]ScanResult, error)`
- Scans both `cfg.Mounts` and `cfg.BmadRepos`
- Test cases include bmad_repos paths
- Entrypoint needs no changes

### 4.6 PRD — Additional Requirements Bullet

**File:** `_bmad-output/planning-artifacts/prd.md`
**Section:** Additional Requirements

OLD:
> auto_isolate_deps: Go `filepath.WalkDir` scan, named volume pattern `asbox-<project>-<path>-node_modules`, always log scan summary

NEW:
> auto_isolate_deps: Go `filepath.WalkDir` scan over primary mounts and `bmad_repos` paths, named volume pattern `asbox-<project>-<path>-node_modules`, always log scan summary

### 4.7 PRD — Runtime Behavior Paragraph

**File:** `_bmad-output/planning-artifacts/prd.md`
**Section:** Technical Architecture Considerations — Runtime behavior

Updated to specify scan covers "all mounted paths — primary mounts and `bmad_repos`" and that bmad_repos container paths derive from `/workspace/repos/<basename>`.

---

## Section 5: Implementation Handoff

**Change scope:** Minor — direct implementation by development team.

**Handoff: Developer (Amelia / dev agent)**

Responsibilities:
1. Update `internal/mount/isolate_deps.go` — extend `ScanDeps` to iterate `cfg.BmadRepos` in addition to `cfg.Mounts`, deriving container paths from `/workspace/repos/<basename>` convention
2. Add test cases to `internal/mount/isolate_deps_test.go` for bmad_repos scan inputs
3. Update `cmd/run.go` if needed to pass bmad_repos context to `ScanDeps` (may already be available via `cfg`)
4. Add integration test case (Epic 9) for combined `auto_isolate_deps + bmad_repos`

**Handoff: Scrum Master (Bob)**
1. Apply the 7 approved edit proposals to PRD, Architecture, and Epics documents
2. Update sprint-status.yaml if applicable

**Success criteria:**
- `ScanDeps` discovers `package.json` files in bmad_repos paths
- Named volumes are created for bmad_repos `node_modules/` directories
- Volume container paths correctly use `/workspace/repos/<basename>/` prefix
- Scan summary log counts include bmad_repos paths
- All existing tests continue to pass
- New test cases cover the intersection behavior
