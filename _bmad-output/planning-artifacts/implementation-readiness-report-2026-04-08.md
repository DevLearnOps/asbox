# Implementation Readiness Assessment Report

**Date:** 2026-04-08
**Project:** asbox

---
stepsCompleted:
  - step-01-document-discovery
  - step-02-prd-analysis
  - step-03-epic-coverage-validation
  - step-04-ux-alignment
  - step-05-epic-quality-review
  - step-06-final-assessment
status: complete
documents:
  - planning-artifacts/prd.md
  - planning-artifacts/architecture.md
  - planning-artifacts/epics.md
---

## Document Inventory

| Document | File | Status |
|---|---|---|
| PRD | prd.md | Found |
| Architecture | architecture.md | Found (complete) |
| Epics & Stories | epics.md | Found |
| UX Design | — | Not found (not required for CLI tool) |

## PRD Analysis

### Functional Requirements

**Sandbox Configuration:**
- FR1: Developer can define SDK versions (Node.js, Go, Python) in a YAML configuration file
- FR2: Developer can specify additional system packages to install in the sandbox image
- FR3: Developer can configure which MCP servers to pre-install (e.g., Playwright)
- FR4: Developer can declare host directories to mount into the sandbox with source and target paths
- FR5: Developer can declare secret names that will be resolved from host environment variables at runtime
- FR6: Developer can set non-secret environment variables for the agent runtime
- FR7: Developer can select which AI agent runtime to use (claude-code, gemini-cli)
- FR8: Developer can override the default config file path with a `-f` flag
- FR9: Developer can generate a starter configuration file with sensible defaults via `asbox init`
- FR9a: Developer can enable automatic dependency isolation (`auto_isolate_deps: true`) to create named Docker volume mounts over platform-specific dependency directories
- FR9b: When `auto_isolate_deps` is enabled, the system scans mounted project paths at launch for `package.json` files and creates named Docker volumes for each corresponding `node_modules/` directory
- FR9c: The system logs all auto-detected dependency isolation mounts at launch
- FR9d: Developer can configure a host agent configuration directory mount (`host_agent_config`) with source and target paths for OAuth token synchronization
- FR9e: Developer can optionally set `project_name` in configuration to override the default project identifier

**Sandbox Lifecycle:**
- FR10: Developer can build a sandbox container image from configuration via `asbox build`
- FR11: Developer can launch a sandbox session in TTY mode via `asbox run`
- FR12: System automatically builds the image if not present when `asbox run` is invoked
- FR13: System detects when configuration or template files have changed and triggers a rebuild
- FR14: Developer can stop a running sandbox session with Ctrl+C
- FR15: System validates that Docker is present before proceeding
- FR16: System validates that all declared secrets are set in the host environment before launching
- FR16a: When `auto_isolate_deps` is enabled, the system creates named Docker volume mounts for each detected dependency directory during sandbox launch and ensures correct ownership

**Agent Runtime:**
- FR17: AI agent can execute inside the sandbox with full terminal access
- FR18: Claude Code agent launches with permissions skipped (sandbox provides isolation)
- FR19: Gemini CLI agent launches with standard configuration
- FR20: Agent can interact with project files mounted from the host
- FR21: Agent can execute BMAD method workflows
- FR22: Agent can install additional packages and dependencies at runtime via package managers

**Development Toolchain:**
- FR23: Agent can use git for local operations (add, commit, log, diff, branch, checkout, merge, amend)
- FR24: Agent can access the internet for fetching documentation, packages, and dependencies
- FR25: Agent can use common CLI tools (curl, wget, dig, etc.)
- FR26: Agent can build Docker images from Dockerfiles inside the sandbox
- FR27: Agent can run `docker compose up` to start multi-service applications inside the sandbox
- FR28: Agent can reach application ports of services running in inner containers
- FR29: Agent can run Playwright tests against running applications via MCP integration
- FR29a: Agent can run Playwright tests using chromium for desktop and webkit for mobile device emulation
- FR30: Agent can start MCP servers as needed via MCP protocol

**Isolation Boundaries:**
- FR31: System blocks git push operations, returning standard authentication failure errors
- FR32: System prevents agent access to host filesystem beyond explicitly mounted paths
- FR33: System prevents agent access to host credentials, SSH keys, and cloud tokens not explicitly declared as secrets
- FR34: System prevents inner containers from being reachable outside the sandbox network
- FR35: System enforces non-privileged inner Docker (no `--privileged` mode, no host Docker socket mount)
- FR36: System provides a private network bridge for communication between inner containers only
- FR37: System returns standard CLI error codes when agent attempts operations outside the boundary

**Image Build System:**
- FR38: System generates a Dockerfile from an embedded Go template using configuration values
- FR39: System pins base images to digest for reproducible builds
- FR40: System passes SDK versions as build arguments to the Dockerfile
- FR41: System installs configured MCP servers at image build time
- FR42: System enforces git push blocking and isolation boundaries from within the sandbox image
- FR43: System tags images using content hash for cache management
- FR44: System installs agent environment instruction files into the sandbox user's home directory at image build time
- FR45: When `host_agent_config` is configured, system mounts the host agent configuration directory read-write and sets the config directory environment variable
- FR46: System merges build-time MCP server manifest with project-level `.mcp.json` at runtime; project config takes precedence on name conflicts
- FR47: System exits with specific exit codes to distinguish error categories (0-4)
- FR48: System uses Tini as init process (PID 1) for proper signal forwarding and zombie process reaping
- FR49: System aligns sandbox user UID/GID with host user at container startup
- FR50: All supporting files are embedded in the Go binary via the `embed` package
- FR51: Developer can configure `bmad_repos` as a list of local paths to checked-out repositories
- FR52: When `bmad_repos` is configured, the system automatically creates mount mappings for each repository into `/workspace/repos/<repo_name>`
- FR53: When `bmad_repos` is configured, the system generates an agent configuration file instructing the agent about git operations within repos
- FR54: The system is distributed as a single statically-linked Go binary with no external runtime dependencies beyond Docker

**Total FRs: 54**

### Non-Functional Requirements

**Security & Isolation:**
- NFR1: No host credential, SSH key, or cloud token is accessible inside the sandbox unless explicitly declared in config
- NFR2: Git push operations fail with standard error codes regardless of remote configuration
- NFR3: Inner containers cannot bind to network interfaces reachable from outside the sandbox
- NFR4: The sandbox container runs without `--privileged` flag and without host Docker socket mount
- NFR5: Secrets are injected as runtime environment variables only — never written to filesystem, image layers, or build cache
- NFR6: Mounted paths are limited to those explicitly declared in config

**Integration:**
- NFR7: The CLI works with Docker Engine 20.10+ on the host; inside, rootless Podman 5.x with docker CLI alias
- NFR8: YAML configuration is parsed via Go's `gopkg.in/yaml.v3` — no external parsing dependency
- NFR9: MCP servers follow standard MCP protocol and are startable by any MCP-compatible agent
- NFR10: Git wrapper is transparent for all operations except push

**Portability & Reliability:**
- NFR11: CLI runs on macOS (arm64/amd64) and Linux (amd64) as statically-linked Go binary
- NFR12: Image builds are reproducible — same config + template + sandbox files produce identical image
- NFR13: CLI fails fast with error messages naming the missing dependency, unset secret, or invalid field with fix action
- NFR14: Crashed or Ctrl+C'd sandbox leaves no orphaned containers or dangling networks
- NFR15: Integration test suite covers all supported use cases with parallel Go test execution

**Total NFRs: 15**

### Additional Requirements

**Constraints & Assumptions:**
- Docker (or Podman) required on host — only external dependency
- Ubuntu 24.04 LTS as base image, pinned to digest
- Podman 5.x from upstream Kubic/OBS repository for inner container runtime
- `vfs` storage driver for nested container compatibility
- `netavark`/`aardvark-dns` for inner container networking
- Testcontainers compatibility: Ryuk disabled, socket override, localhost host override
- Mount paths resolved relative to config file location, not working directory
- Content hash over rendered template output + config content for image caching

### PRD Completeness Assessment

The PRD is comprehensive and well-structured. All 54 FRs and 15 NFRs are explicitly numbered and unambiguous. The PRD includes user journeys, market context, risk mitigation, phased development plan, and detailed technical architecture considerations. The recent edit (2026-04-06) brought the PRD in line with the Go rewrite, bmad_repos, and other new features. No gaps or ambiguities detected in the requirements themselves.

## Epic Coverage Validation

### Critical Finding: Epics Are Stale

The epics document was written against the **old PRD** (bash-based `sandbox`). The PRD underwent a major course correction on 2026-04-06 (rebrand to `asbox`, rewrite from bash to Go, new features). **The epics have NOT been updated to reflect these changes.** Evidence:

- Epics reference `sandbox.sh`, `parse_config()` bash function, `yq`, bash 4+ requirement
- Epics reference `sandbox init`, `sandbox build`, `sandbox run` (old naming, should be `asbox`)
- Epics reference `Dockerfile.template` with `{{IF_NAME}}`/`{{/IF_NAME}}` bash markers (now Go `text/template`)
- FR coverage map only goes to FR43 — PRD now has FR44-FR54
- NFR list has 14 items (old) — PRD now has 15 (NFR15 added)
- NFR7 references Podman as "alternative runtime" — PRD now specifies Podman as THE inner runtime
- NFR8 references yq v4+ — PRD now specifies Go `gopkg.in/yaml.v3`
- NFR11 references bash 4+ — PRD now specifies Go static binary
- Epic 6 is a vague "Tech Spec" without clear stories or FR coverage

### FR Coverage Matrix

| FR | In Epics? | Epic | Status |
|---|---|---|---|
| FR1-FR7 | Yes | Epic 1 | Covered but stories reference bash implementation |
| FR8 | Yes | Epic 1 | Covered but references `sandbox` not `asbox` |
| FR9 | Yes | Epic 1 | Covered but references `sandbox init` |
| FR9a-FR9c | Yes | Epic 8 | Covered |
| FR9d | **NO** | — | **MISSING** — `host_agent_config` config option |
| FR9e | **NO** | — | **MISSING** — `project_name` config override |
| FR10-FR16 | Yes | Epic 1-2 | Covered but FR15 references yq (removed) |
| FR16a | Yes | Epic 8 | Covered |
| FR17-FR25 | Yes | Epic 2 | Covered |
| FR26-FR30 | Yes | Epic 4-5 | Covered |
| FR29a | **NO** | — | **MISSING** — WebKit browser support for mobile emulation |
| FR31-FR37 | Yes | Epic 3-4 | Covered |
| FR38 | Yes | Epic 1 | Covered but references bash template substitution |
| FR39-FR43 | Yes | Epic 1-5 | Covered |
| FR44 | **NO** | — | **MISSING** — Agent instruction files (CLAUDE.md/GEMINI.md) baked into image |
| FR45 | **NO** | — | **MISSING** — `host_agent_config` mount + `CLAUDE_CONFIG_DIR` env var (Epic 7.4 partially covers investigation, not the full FR) |
| FR46 | Partial | Epic 5.2 | MCP merge covered in Story 5.2 |
| FR47 | **NO** | — | **MISSING** — Go structured error types with distinct exit codes |
| FR48 | Partial | Epic 1.6 | Tini referenced in Story 1.6 but not as explicit FR |
| FR49 | **NO** | — | **MISSING** — UID/GID alignment with host user at startup |
| FR50 | **NO** | — | **MISSING** — Embedded assets via Go `embed` package |
| FR51 | **NO** | — | **MISSING** — `bmad_repos` config option |
| FR52 | **NO** | — | **MISSING** — `bmad_repos` auto-mount to `/workspace/repos/<name>` |
| FR53 | **NO** | — | **MISSING** — `bmad_repos` generated agent instruction file |
| FR54 | **NO** | — | **MISSING** — Single statically-linked Go binary distribution |

### Missing FR Coverage

**Critical Missing FRs (new features not in any epic):**

- **FR9d** (`host_agent_config`): Config option for host agent config directory mount
- **FR9e** (`project_name`): Config override for project identifier
- **FR29a** (WebKit): Playwright webkit browser support for mobile device emulation
- **FR44** (Agent instructions): CLAUDE.md/GEMINI.md baked into image
- **FR47** (Go error types): Structured errors with distinct exit codes
- **FR49** (UID/GID alignment): `usermod`/`groupmod` at container startup
- **FR50** (Embedded assets): Go `embed` package for all supporting files
- **FR51-FR53** (`bmad_repos`): Multi-repo workflow with generated agent instructions
- **FR54** (Single binary): Statically-linked Go binary distribution

**Critical Missing NFR:**

- **NFR15** (Integration tests): Test suite covering all use cases with parallel Go test execution

**Systemic Issue — All stories reference bash implementation:**

Every story in the epics references bash-specific implementation details:
- `sandbox.sh` script → should be Go CLI binary `asbox`
- `parse_config()` bash function → should be Go `config.Parse()`
- `yq` dependency → eliminated (Go `gopkg.in/yaml.v3`)
- `{{IF_NAME}}`/`{{/IF_NAME}}` template markers → Go `text/template` syntax
- `set -euo pipefail` → Go error handling
- bash 4+ requirement → Go static binary
- `Dockerfile.template` → `Dockerfile.tmpl` (Go template)
- Content hash of specific file list → rendered Dockerfile + scripts + base digest + config

### Coverage Statistics

- Total PRD FRs: 54
- FRs with epic coverage (any mention): 43
- FRs missing from epics entirely: 11
- Coverage percentage: **80%** (but effectively lower — covered FRs reference wrong implementation)
- NFRs covered: 14/15 (NFR15 missing)
- **Stories requiring rewrite for Go implementation: ALL (every story references bash)**

## UX Alignment Assessment

### UX Document Status

Not found — not applicable. asbox is a CLI tool with no GUI, no web interface, no IDE plugin. The PRD explicitly states: "The user interface is entirely terminal-based." No UX design document is needed.

### Alignment Issues

None. The CLI interface is fully specified in the PRD (three commands: init, build, run; one flag: -f; exit codes 0-4). No visual design decisions required.

### Warnings

None.

## Epic Quality Review

### Overarching Finding: Epics Require Complete Rewrite

**Before evaluating individual epic quality, the fundamental problem must be stated: the entire epics document was written for a bash-based CLI tool called "sandbox" and has not been updated for the Go-based "asbox" rewrite.** Every story references bash implementation details that no longer apply. Quality review findings below are noted but are secondary to this systemic issue.

### Epic Structure Validation

#### User Value Focus

| Epic | User-Centric? | Assessment |
|---|---|---|
| Epic 1: Sandbox Foundation | Yes | "Developer Can Build and Launch a Sandbox" — clear user value |
| Epic 2: Project Integration | Yes | "Agent Can Work With Project Files and Git" — clear user value |
| Epic 3: Isolation Boundaries | Yes | "Sandbox Enforces Security Constraints" — user value (safety) |
| Epic 4: Inner Container Runtime | Yes | "Agent Can Build and Run Docker Services" — clear user value |
| Epic 5: MCP Integration | Yes | "Agent Can Run Browser Tests via Playwright" — clear user value |
| Epic 6: Host Agent Config | Borderline | "Tech Spec" label — lacks user-centric framing, no clear stories |
| Epic 7: Runtime Hardening | Yes | Fixes real user-facing issues (broken Playwright, Compose, auth) |
| Epic 8: Auto Dependency Isolation | Yes | "Prevent Cross-Platform Module Clashes" — clear user value |

#### Epic Independence

- **Epic 1 → Epic 2:** Valid dependency. Epic 2 builds on a running sandbox from Epic 1.
- **Epic 2 → Epic 3:** Valid. Isolation boundaries extend the running sandbox.
- **Epic 3 → Epic 4:** Valid. Inner containers build on isolation framework.
- **Epic 4 → Epic 5:** Valid. MCP/Playwright requires inner containers.
- **Epic 6:** Poorly defined — listed as "Tech Spec" with no stories, just references to FR17/FR18. Should be folded into another epic or given proper story structure.
- **Epic 7:** Sprint change proposal with bug fixes — valid as a hardening epic but numbered out of order (7 before 8).
- **Epic 8:** Independent feature (auto_isolate_deps). Could run in parallel with Epics 3-5.

### Story Quality Assessment

#### Critical Violations (Red)

1. **All stories reference bash implementation** — Every story's acceptance criteria and implementation notes reference `sandbox.sh`, `parse_config()`, `yq`, bash functions, `{{IF_NAME}}` template markers. These are all wrong for the Go implementation.

2. **Epic 6 has no stories** — Listed as "Tech Spec" with vague FR coverage ("Related to FR17, FR18"). Story 7.4 partially addresses this but is framed as investigation, not implementation. FR9d and FR45 (the actual PRD requirements for `host_agent_config`) are not covered.

3. **Missing epics for Go rewrite fundamentals:**
   - No epic/story for Go project scaffolding (Cobra, go.mod, project structure)
   - No epic/story for Go `embed` package setup
   - No epic/story for Go `text/template` Dockerfile generation
   - No epic/story for Go config parsing via `gopkg.in/yaml.v3`
   - No epic/story for `bmad_repos` (FR51-FR53)
   - No epic/story for integration test suite (NFR15)

#### Major Issues (Orange)

4. **Story 1.1 references yq and bash 4+** — These dependencies no longer exist. The story needs to validate Docker only.

5. **Story 1.4 references bash template substitution** — `{{IF_NAME}}`/`{{/IF_NAME}}` markers are wrong. Should reference Go `text/template` with `{{if .Field}}`/`{{end}}`.

6. **Story 1.5 references wrong content-hash inputs** — Lists `config.yaml, Dockerfile.template, entrypoint.sh, git-wrapper.sh`. Architecture specifies: rendered Dockerfile + all embedded scripts + base image digest + config content.

7. **Story 8.1 references bash implementation** — `detect_isolate_deps()` function in `sandbox.sh`, `find` command. Should reference Go `filepath.WalkDir` in `internal/mount/isolate_deps.go`.

8. **Epic 7 stories reference old Dockerfile.template** — Specific line numbers and bash-era patterns.

#### Minor Concerns (Yellow)

9. **Naming inconsistency** — Document title says "sandbox" throughout, should be "asbox".

10. **Epic numbering gap** — Epic 6 → Epic 8 → Epic 7 (out of order). Epic 7 was a sprint change proposal inserted later.

### Acceptance Criteria Quality

Where present, acceptance criteria follow proper Given/When/Then format and are testable. However:
- All ACs reference the wrong technology (bash instead of Go)
- Error handling ACs reference bash-specific patterns
- No ACs exist for the 11 missing FRs

### Best Practices Compliance

| Check | Epic 1 | Epic 2 | Epic 3 | Epic 4 | Epic 5 | Epic 6 | Epic 7 | Epic 8 |
|---|---|---|---|---|---|---|---|---|
| User value | Pass | Pass | Pass | Pass | Pass | **FAIL** | Pass | Pass |
| Independence | Pass | Pass | Pass | Pass | Pass | N/A | Pass | Pass |
| Story sizing | Pass | Pass | Pass | Pass | Pass | **FAIL** (no stories) | Pass | Pass |
| No forward deps | Pass | Pass | Pass | Pass | Pass | N/A | Pass | Pass |
| Clear ACs | Pass* | Pass* | Pass* | Pass* | Pass* | **FAIL** | Pass* | Pass* |
| FR traceability | Pass | Pass | Pass | Pass | Pass | **FAIL** | Pass | Pass |

*Pass with caveat: ACs reference wrong implementation technology

### Recommendations

**The epics document must be rewritten from scratch** to align with the current PRD and architecture. Specific actions:

1. **Rewrite all stories** to reference Go implementation (Cobra, yaml.v3, text/template, embed, os/exec)
2. **Add missing FRs** to coverage map: FR9d, FR9e, FR29a, FR44, FR45, FR47, FR49, FR50, FR51-FR53, FR54, NFR15
3. **Resolve Epic 6** — either fold into another epic with proper stories or create proper user-centric stories for `host_agent_config`
4. **Add new epic or stories** for Go project foundation (scaffolding, embed setup, template engine)
5. **Add integration test epic/stories** to cover NFR15
6. **Renumber epics** for logical ordering
7. **Rename from "sandbox" to "asbox"** throughout

## Summary and Recommendations

### Overall Readiness Status

**NOT READY** — The epics document is fundamentally misaligned with the current PRD and architecture. The PRD and architecture are solid and aligned with each other, but the epics were written for a different implementation (bash) and have not been updated for the Go rewrite.

### Critical Issues Requiring Immediate Action

1. **Epics document is stale** — Written for bash-based "sandbox", not Go-based "asbox". Every story references wrong technology. The entire document must be rewritten.
2. **11 FRs have no epic coverage** — FR9d, FR9e, FR29a, FR44, FR47, FR49, FR50, FR51-FR53, FR54 are not covered by any epic or story.
3. **NFR15 (integration tests) has no epic coverage** — Required integration test suite is not addressed in any story.
4. **Epic 6 is structurally invalid** — Labeled "Tech Spec" with no stories, vague FR coverage, no acceptance criteria.

### What's Working Well

- **PRD is comprehensive and current** — 54 FRs, 15 NFRs, well-structured, recently updated (2026-04-06)
- **Architecture is complete and aligned with PRD** — Just completed (2026-04-07), covers all FRs, expert-panel-reviewed
- **PRD ↔ Architecture alignment is strong** — Every FR maps to a specific file and function in the architecture
- **No UX gaps** — CLI tool, no UX document needed

### Recommended Next Steps

1. **Rewrite the epics document from scratch** — Use the current PRD and architecture as inputs. Run `bmad-create-epics-and-stories` in a fresh context to produce epics aligned with the Go implementation.
2. **Do NOT attempt to patch the existing epics** — The drift is too fundamental (different language, different project structure, different dependencies). A fresh rewrite will be faster and cleaner than trying to update every story.
3. **After new epics are created**, run this implementation readiness check again to validate coverage.

### Assessment Statistics

| Category | Finding |
|---|---|
| PRD completeness | Complete (54 FRs, 15 NFRs) |
| Architecture alignment | Aligned with PRD |
| Epic FR coverage | 80% (43/54) — but all stories reference wrong implementation |
| Missing FRs in epics | 11 (FR9d, FR9e, FR29a, FR44, FR47, FR49, FR50, FR51-FR54) |
| Missing NFRs in epics | 1 (NFR15) |
| Stories requiring rewrite | All |
| Epic quality violations | 4 critical, 4 major, 2 minor |
| UX issues | None (not applicable) |

### Final Note

This assessment identified **4 critical issues** and **4 major issues** across epic coverage and quality. The PRD and architecture are ready for implementation, but the **epics must be rewritten** before proceeding to sprint planning and story development. The good news: the PRD and architecture provide an excellent foundation for the rewrite — the requirements and technical decisions are clear, well-documented, and aligned.
