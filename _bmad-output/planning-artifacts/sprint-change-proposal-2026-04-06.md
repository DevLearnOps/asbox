# Sprint Change Proposal — asbox Go Rewrite & BMAD Multi-Repo Support

**Date:** 2026-04-06
**Author:** Manuel (via Correct Course workflow)
**Scope Classification:** Major
**Status:** Approved

---

## 1. Issue Summary

### Problem Statement

The current sandbox implementation (bash shell script + external supporting files) has reached its practical limits. Five interconnected changes are needed:

1. **Rebranding to `asbox`** — `sandbox.sh` is tedious to type and the name conflicts with potential macOS programs. The tool should be a single binary called `asbox` (Agent-SandBox).
2. **Go CLI rewrite** — Bash limitations are blocking progress: serial test execution, fragile sed-based template substitution, external yq dependency for YAML parsing, poor error handling patterns.
3. **Embedded assets** — As a single Go binary, all supporting files (Dockerfile template, entrypoint scripts, agent instructions, config template) should be compiled into the binary via Go's `embed` package.
4. **Integration test coverage** — Go's parallel test runner enables comprehensive integration testing across all supported use cases.
5. **BMAD multi-repo workflow** — New `bmad_repos` config attribute to support development workflows spanning multiple repositories with auto-generated agent instructions.

### Trigger Context

This is a strategic pivot, not triggered by a specific failing story. The bash implementation (Epics 1-8) is complete and functional. The course correction is driven by:
- `sandbox.sh` growing to 26KB+ of bash — increasingly hard to maintain
- yq as a hard dependency creating friction for new users
- sed-based template substitution being fragile compared to Go's `text/template`
- Bash tests running serially; Go tests parallelize natively
- `bmad_repos` feature requiring dynamic agent instruction generation — easier with Go templates
- Single binary distribution eliminating "did you copy all the scripts?" problems

---

## 2. Impact Analysis

### Epic Impact

| Epic | Impact Level | Action |
|------|-------------|--------|
| Epic 1: Sandbox Foundation | **Rewrite** | CLI skeleton, config parsing, Dockerfile generation, build/run all move from bash to Go |
| Epic 2: Project Integration | **Moderate** | Mount assembly, secret injection, agent runtime logic rewritten in Go |
| Epic 3: Isolation Boundaries | **Minimal** | Git wrapper/entrypoint remain bash (container-side). Host-side validation moves to Go |
| Epic 4: Inner Container Runtime | **Minimal** | Podman setup is Dockerfile-side, unaffected |
| Epic 5: MCP Integration | **Minimal** | MCP manifest/merge logic moves to Go, behavior identical |
| Epic 6: Host Agent Config | **Minimal** | Mount logic moves to Go |
| Epic 7: Runtime Hardening | **N/A** | Already complete, no changes |
| Epic 8: Auto Dependency Isolation | **Moderate** | `detect_isolate_deps()` rewritten in Go |

### New Epics Required

| Epic | Description |
|------|-------------|
| Epic 9: Go CLI Foundation | Go project scaffolding, CLI framework, embedded assets, config parsing, Go template-based Dockerfile rendering |
| Epic 10: BMAD Multi-Repo Workflow | `bmad_repos` config, auto-mount to `/workspace/repos/<name>`, generated agent instructions |
| Epic 11: Integration Test Suite | Comprehensive Go integration tests with parallel execution |

### Artifact Impact

| Artifact | Impact | Action |
|----------|--------|--------|
| PRD | **Edit** | 12 approved edit proposals (naming, technology, new FRs, NFRs, risks) |
| Architecture | **Rewrite** | Preserve decisions (Podman, isolation, MCP merge), replace implementation patterns for Go |
| Epics | **Rewrite** | New epic structure with Go-based stories, add Epics 9-11 |
| Container scripts | **Minimal** | entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh remain bash; minor naming updates |
| agent-instructions.md | **None** | Content unchanged, embedded in binary |
| Dockerfile.template | **Moderate** | `{{IF_NAME}}` markers → Go `{{if .Field}}` syntax; embedded in binary |
| README.md | **Rewrite** | Deferred until after implementation |

### Technical Impact

- **Host-side:** Complete rewrite from bash to Go
- **Container-side:** Unchanged — entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh remain bash
- **Distribution:** Source-distributed script → single statically-linked Go binary
- **Dependencies:** Docker + yq → Docker only
- **Build system:** None → `go build` (or pre-built binaries via GitHub releases)

---

## 3. Recommended Approach

### Selected Path: Hybrid — Fresh Implementation Sprint with Artifact Rewrite

The existing bash implementation is complete and working. The recommended approach:

1. **Edit the PRD** per the 12 approved proposals
2. **Rewrite the architecture document** — preserve core decisions, replace implementation patterns
3. **Rewrite the epics document** — new structure with Go-based stories, Epics 9-11
4. **Implement as a clean Go project** — new directory structure alongside existing bash
5. **Delete bash implementation** once Go version passes all integration tests

### Rationale

- Functional requirements are stable and validated through the working bash implementation
- Container-side behavior is unchanged — only host-side CLI changes technology
- Bash version remains as fallback during migration
- Go rewrite is bounded in scope: config parsing, template rendering, docker command assembly, mount/secret/env flag assembly

### Effort Estimate: High

Full rewrite of host-side CLI + 3 artifact rewrites (PRD edit, architecture rewrite, epics rewrite).

### Risk Assessment: Low

- Functional scope preserved — no features removed
- Container internals unchanged — proven behavior carries forward
- Bash implementation remains until Go reaches parity
- Go is a well-understood technology with strong tooling

### Timeline Impact

New sprint — this is a complete implementation cycle, not a mid-sprint patch.

---

## 4. Detailed Change Proposals

### 4.1 PRD Changes (12 Approved Proposals)

**Proposal 1: Product Name & Executive Summary**
- "Sandbox" → "asbox" as product name
- "source code distribution" → "single binary distribution"

**Proposal 2: Project Classification**
- "source-distributed, locally built" → "single Go binary, containerized environment manager"

**Proposal 3: Configuration Surface**
- `.sandbox/config.yaml` → `.asbox/config.yaml`
- Add `bmad_repos` config attribute with description and example
- Document auto-mount to `/workspace/repos/<repo_name>` and agent instruction generation

**Proposal 4: CLI Interface**
- `sandbox.sh` (bash) → `asbox` (Go binary)
- All commands renamed `sandbox` → `asbox`
- Installation via binary download or `go install`
- yq dependency removed; "zero external dependencies" principle

**Proposal 5: Technical Architecture Considerations**
- Bash shell structure → Go CLI structure
- yq → `gopkg.in/yaml.v3`
- sed substitution → `text/template`
- External files → `embed` package
- Docker only as host dependency

**Proposal 6: Installation & Distribution**
- Source code via git → single statically-linked binary
- Pre-built binaries for macOS/Linux via GitHub releases
- `go install` as alternative

**Proposal 7: Implementation Considerations**
- All bash-specific patterns → Go equivalents
- `find` → `filepath.WalkDir`
- `bmad_repos` mount assembly implementation note
- Image prefix `sandbox-` → `asbox-`

**Proposal 8: MVP Feature Set**
- "Shell CLI" → "Go CLI binary"
- Remove yq as must-have
- Add: embedded assets, BMAD multi-repo workflow

**Proposal 9: Functional Requirements**
- FR9, FR15, FR38, FR43, FR47 modified for Go/asbox
- FR50: Embedded supporting files via `embed` package
- FR51: `bmad_repos` configuration
- FR52: Auto-mount repos to `/workspace/repos/<name>`
- FR53: Generated agent instructions for multi-repo awareness
- FR54: Single statically-linked Go binary distribution

**Proposal 10: Non-Functional Requirements**
- NFR8: yq → Go native YAML
- NFR11: bash 4+ no longer required on host
- NFR15 (new): Integration test coverage with parallel execution

**Proposal 11: Risk Mitigation**
- Dockerfile.template complexity risk mitigated by Go templates
- New: Go rewrite migration risk (low — container-side unchanged)
- New: Embedded asset drift risk (intentional — ensures version consistency)
- New: BMAD multi-repo mount complexity (mitigated by convention-based mapping)

**Proposal 12: Global Rename**
- `sandbox` → `asbox` for tool name, CLI command, config dir, image prefix, volume prefix
- Generic "sandbox" concept references preserved

### 4.2 Architecture Document

**Action:** Full rewrite

**Preserved decisions:**
- Podman 5.x as inner container runtime
- Git wrapper at `/usr/local/bin/git`
- Default Podman rootless networking
- Fail-closed secret injection
- Tini as PID 1
- MCP manifest merge (project config wins)
- Auto dependency isolation algorithm
- Content-hash caching strategy

**New content:**
- Go project structure (`cmd/asbox/`, `internal/config/`, `internal/build/`, `internal/run/`, `internal/embed/`)
- Go coding conventions (standard Go naming, error handling, struct-based config)
- Go template syntax for Dockerfile rendering
- `embed` package strategy for supporting files
- `bmad_repos` architectural decision
- Integration test architecture
- Updated data flow with Go types
- Updated requirements-to-structure mapping

### 4.3 Epics Document

**Action:** Full rewrite

**Preserved functional scope from Epics 1-8:**
All existing FRs (FR1-FR49) carry forward with implementation changes.

**New epics to add:**
- Epic 9: Go CLI Foundation
- Epic 10: BMAD Multi-Repo Workflow
- Epic 11: Integration Test Suite

**New FRs to cover:**
- FR50-FR54 mapped to appropriate epics

---

## 5. Implementation Handoff

### Scope Classification: Major

This requires a fundamental replan with PM/Architect involvement.

### Handoff Recipients and Responsibilities

| Role | Responsibility |
|------|---------------|
| **Product Manager** (or Manuel as PM) | Apply 12 PRD edit proposals |
| **Architect** | Rewrite architecture document preserving core decisions, defining Go structure |
| **Scrum Master** | Rewrite epics with Go-based stories, define new sprint plan |
| **Developer** | Implement Go CLI, integration tests, bmad_repos feature |

### Implementation Sequence

1. **Phase 1: Artifact Updates**
   - Apply PRD edits (12 proposals)
   - Rewrite architecture document
   - Rewrite epics with story breakdowns

2. **Phase 2: Go CLI Foundation (Epic 9)**
   - Go project scaffolding with `embed` package
   - Config parsing via native Go YAML
   - Dockerfile rendering via Go templates
   - Build and run commands via `os/exec`
   - Content-hash caching

3. **Phase 3: Feature Parity (Epics 1-8 reimplemented)**
   - Mount assembly, secret injection, env vars
   - Auto dependency isolation
   - Host agent config mount
   - All existing behavior ported to Go

4. **Phase 4: New Features (Epics 10-11)**
   - BMAD multi-repo workflow
   - Integration test suite

5. **Phase 5: Cutover**
   - Validate Go version passes all integration tests
   - Delete bash implementation
   - Update README

### Success Criteria

- `asbox init`, `asbox build`, `asbox run` work identically to former `sandbox init/build/run`
- All existing functional requirements (FR1-FR49) pass with Go implementation
- New FRs (FR50-FR54) implemented and tested
- Integration test suite passes with parallel execution
- `bmad_repos` mounts repos and generates agent instructions correctly
- Single binary distribution — `asbox` has no external dependencies beyond Docker
- Container-side behavior unchanged — entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh unmodified
