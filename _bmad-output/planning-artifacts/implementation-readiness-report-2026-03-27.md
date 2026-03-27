---
stepsCompleted: [step-01-document-discovery, step-02-prd-analysis, step-03-epic-coverage-validation, step-04-ux-alignment, step-05-epic-quality-review, step-06-final-assessment]
files:
  prd: prd.md
  architecture: architecture.md
  epics: epics.md
  ux: null
---

# Implementation Readiness Assessment Report

**Date:** 2026-03-27
**Project:** sandbox

## Document Inventory

| Document Type | File | Size | Modified |
|---|---|---|---|
| PRD | prd.md | 35,766 bytes | 2026-03-26 |
| Architecture | architecture.md | 36,914 bytes | 2026-03-27 |
| Epics & Stories | epics.md | 36,225 bytes | 2026-03-26 |
| UX Design | Not found | — | — |

**Notes:**
- No duplicate documents detected
- UX Design document not found — UX assessment will be skipped

## PRD Analysis

### Functional Requirements

**Sandbox Configuration (FR1–FR9c):**
- FR1: Developer can define SDK versions (Node.js, Go, Python) in a YAML configuration file
- FR2: Developer can specify additional system packages to install in the sandbox image
- FR3: Developer can configure which MCP servers to pre-install (e.g., Playwright)
- FR4: Developer can declare host directories to mount into the sandbox with source and target paths
- FR5: Developer can declare secret names that will be resolved from host environment variables at runtime
- FR6: Developer can set non-secret environment variables for the agent runtime
- FR7: Developer can select which AI agent runtime to use (claude-code, gemini-cli)
- FR8: Developer can override the default config file path with a `-f` flag
- FR9: Developer can generate a starter configuration file with sensible defaults via `sandbox init`
- FR9a: Developer can enable automatic dependency isolation (`auto_isolate_deps: true`) to create anonymous volume mounts over platform-specific dependency directories (e.g., `node_modules/`) within mounted project paths
- FR9b: When `auto_isolate_deps` is enabled, the system scans mounted project paths at launch for `package.json` files and creates anonymous Docker volume mounts for each corresponding `node_modules/` directory
- FR9c: The system logs all auto-detected dependency isolation mounts at launch so the developer has visibility into what was isolated

**Sandbox Lifecycle (FR10–FR16a):**
- FR10: Developer can build a sandbox container image from configuration via `sandbox build`
- FR11: Developer can launch a sandbox session in TTY mode via `sandbox run`
- FR12: System automatically builds the image if not present when `sandbox run` is invoked
- FR13: System detects when configuration or template files have changed and triggers a rebuild
- FR14: Developer can stop a running sandbox session with Ctrl+C
- FR15: System validates that required dependencies (Docker, yq) are present before proceeding
- FR16: System validates that all declared secrets are set in the host environment before launching
- FR16a: When `auto_isolate_deps` is enabled, the system creates anonymous volume mounts for each detected dependency directory during sandbox launch

**Agent Runtime (FR17–FR22):**
- FR17: AI agent can execute inside the sandbox with full terminal access
- FR18: Claude Code agent launches with permissions skipped (sandbox provides isolation)
- FR19: Gemini CLI agent launches with standard configuration
- FR20: Agent can interact with project files mounted from the host
- FR21: Agent can execute BMAD method workflows (read/write planning artifacts, project knowledge, output documents)
- FR22: Agent can install additional packages and dependencies at runtime via package managers

**Development Toolchain (FR23–FR30):**
- FR23: Agent can use git for local operations (add, commit, log, diff, branch, checkout, merge, amend)
- FR24: Agent can access the internet for fetching documentation, packages, and dependencies
- FR25: Agent can use common CLI tools (curl, wget, dig, etc.)
- FR26: Agent can build Docker images from Dockerfiles inside the sandbox
- FR27: Agent can run `docker compose up` to start multi-service applications inside the sandbox
- FR28: Agent can reach application ports of services running in inner containers
- FR29: Agent can run Playwright tests against running applications via MCP integration
- FR30: Agent can start MCP servers as needed via MCP protocol

**Isolation Boundaries (FR31–FR37):**
- FR31: System blocks git push operations, returning standard "unauthorized" errors
- FR32: System prevents agent access to host filesystem beyond explicitly mounted paths
- FR33: System prevents agent access to host credentials, SSH keys, and cloud tokens not explicitly declared as secrets
- FR34: System prevents inner containers from being reachable outside the sandbox network
- FR35: System enforces non-privileged inner Docker (no `--privileged` mode, no host Docker socket mount)
- FR36: System provides a private network bridge for communication between inner containers only
- FR37: System returns standard CLI error codes when agent attempts operations outside the boundary

**Image Build System (FR38–FR43):**
- FR38: System generates a Dockerfile from a Dockerfile.template using configuration values
- FR39: System pins base images to digest for reproducible builds
- FR40: System passes SDK versions as build arguments to the Dockerfile
- FR41: System installs configured MCP servers at image build time
- FR42: System includes git push blocking and isolation boundary enforcement in the built image
- FR43: System tags images using content hash of config + template + sandbox files for cache management

**Total FRs: 47** (FR1–FR43 plus FR9a, FR9b, FR9c, FR16a)

### Non-Functional Requirements

**Security & Isolation (NFR1–NFR6):**
- NFR1: No host credential, SSH key, or cloud token is accessible inside the sandbox unless explicitly declared in the config file's secrets list
- NFR2: Git push operations fail with standard error codes regardless of remote configuration present in mounted repositories
- NFR3: Inner containers cannot bind to network interfaces reachable from outside the sandbox
- NFR4: The sandbox container runs without `--privileged` flag and without host Docker socket mount
- NFR5: Secrets are injected as runtime environment variables only — never written to the filesystem, image layers, or build cache
- NFR6: Mounted paths are limited to those explicitly declared in config — no implicit access to home directory, `.ssh`, `.aws`, or other host paths

**Integration (NFR7–NFR10):**
- NFR7: The CLI works with Docker Engine 20.10+ and is compatible with Podman as an alternative runtime
- NFR8: YAML configuration is parsed via `yq` v4+ — the script fails clearly if `yq` is not installed or is an incompatible version
- NFR9: MCP servers installed in the sandbox follow the standard MCP protocol and are startable by any MCP-compatible agent
- NFR10: The git wrapper is transparent to the agent for all operations except push — git log, diff, commit, branch, etc. behave identically to standard git

**Portability & Reliability (NFR11–NFR14):**
- NFR11: The CLI runs on macOS (arm64/amd64) and Linux (amd64) with bash 4+
- NFR12: Image builds are reproducible — the same config + template + sandbox files produce an identical image regardless of when or where the build runs (base image pinned to digest)
- NFR13: The CLI fails fast with error messages that name the missing dependency, unset secret, or invalid field and state the required fix action
- NFR14: A crashed or Ctrl+C'd sandbox leaves no orphaned containers or dangling networks on the host

**Total NFRs: 14**

### Additional Requirements

**Constraints & Assumptions:**
- Bash 4+ required (not POSIX shell)
- `yq` v4+ is a hard dependency for YAML parsing
- Docker or Podman must be installed on the host
- Solo developer (Manuel) is the primary user in Phase 1
- Inner Docker must not require privileged mode — viable options are rootless Docker, sysbox, or Podman
- Host Docker socket mount is explicitly ruled out
- Base images pinned to digest, not tag
- Mount paths resolved relative to config file location, not working directory
- MCP servers pre-installed at build time, started by agent at runtime
- Security model protects against accidental leakage, not deliberate adversarial container escape

### PRD Completeness Assessment

The PRD is comprehensive and well-structured. It covers:
- Clear problem statement and success criteria
- 4 detailed user journeys covering happy path, edge cases, setup, and agent perspective
- 47 functional requirements with logical grouping
- 14 non-functional requirements covering security, integration, and portability
- Phased development strategy (MVP, Team Adoption, Scale)
- Risk mitigation for technical, market, and resource risks
- Innovation context and competitive landscape

No obvious gaps in the PRD. Requirements are specific and testable.

## Epic Coverage Validation

### Coverage Matrix

| FR | PRD Requirement | Epic Coverage | Status |
|---|---|---|---|
| FR1 | SDK versions in YAML config | Epic 1, Story 1.3/1.4 | Covered |
| FR2 | System packages in config | Epic 1, Story 1.3/1.4 | Covered |
| FR3 | MCP server configuration | Epic 5, Story 5.1 | Covered |
| FR4 | Host directory mounts | Epic 2, Story 2.1 | Covered |
| FR5 | Secret declaration and injection | Epic 2, Story 2.2 | Covered |
| FR6 | Non-secret environment variables | Epic 1, Story 1.3 | Covered |
| FR7 | Agent runtime selection | Epic 1, Story 1.6 | Covered |
| FR8 | Config file path override (`-f`) | Epic 1, Story 1.1/1.2/1.3 | Covered |
| FR9 | `sandbox init` starter config | Epic 1, Story 1.2 | Covered |
| FR9a | Enable `auto_isolate_deps` in config | **NOT FOUND** | MISSING |
| FR9b | Scan for package.json, create anonymous volume mounts | **NOT FOUND** | MISSING |
| FR9c | Log auto-detected dependency isolation mounts | **NOT FOUND** | MISSING |
| FR10 | `sandbox build` command | Epic 1, Story 1.5 | Covered |
| FR11 | `sandbox run` in TTY mode | Epic 1, Story 1.6 | Covered |
| FR12 | Auto-build on run | Epic 1, Story 1.6 | Covered |
| FR13 | Change detection and rebuild | Epic 1, Story 1.5 | Covered |
| FR14 | Ctrl+C stop | Epic 1, Story 1.6 | Covered |
| FR15 | Dependency validation | Epic 1, Story 1.1 | Covered |
| FR16 | Secret validation before launch | Epic 2, Story 2.2 | Covered |
| FR16a | Anonymous volume mounts for detected dependency dirs | **NOT FOUND** | MISSING |
| FR17 | Agent terminal access | Epic 2, Story 2.3 | Covered |
| FR18 | Claude Code with --dangerously-skip-permissions | Epic 2, Story 2.3 / Epic 7, Story 7.3 | Covered |
| FR19 | Gemini CLI standard launch | Epic 2, Story 2.3 | Covered |
| FR20 | Agent interacts with mounted files | Epic 2, Story 2.3 | Covered |
| FR21 | BMAD workflow support | Epic 2, Story 2.3 | Covered |
| FR22 | Runtime package installation | Epic 2, Story 2.3 | Covered |
| FR23 | Local git operations | Epic 2, Story 2.4 | Covered |
| FR24 | Internet access | Epic 2, Story 2.4 | Covered |
| FR25 | Common CLI tools | Epic 2, Story 2.4 | Covered |
| FR26 | Build Docker images inside sandbox | Epic 4, Story 4.2 | Covered |
| FR27 | Docker Compose inside sandbox | Epic 4, Story 4.2 / Epic 7, Story 7.2 | Covered |
| FR28 | Reach inner container ports | Epic 4, Story 4.2 | Covered |
| FR29 | Playwright tests via MCP | Epic 5, Story 5.2 / Epic 7, Story 7.1 | Covered |
| FR30 | Start MCP servers via protocol | Epic 5, Story 5.2 | Covered |
| FR31 | Git push blocked | Epic 3, Story 3.1 | Covered |
| FR32 | Filesystem restricted to mounts | Epic 3, Story 3.2 | Covered |
| FR33 | Host credentials inaccessible | Epic 3, Story 3.2 | Covered |
| FR34 | Inner containers not reachable externally | Epic 4, Story 4.2 | Covered |
| FR35 | Non-privileged inner Docker | Epic 4, Story 4.1 | Covered |
| FR36 | Private network bridge | Epic 4, Story 4.2 | Covered |
| FR37 | Standard error codes at boundaries | Epic 3, Story 3.2 | Covered |
| FR38 | Dockerfile from template | Epic 1, Story 1.4 | Covered |
| FR39 | Base images pinned to digest | Epic 1, Story 1.5 | Covered |
| FR40 | SDK versions as build args | Epic 1, Story 1.4 | Covered |
| FR41 | MCP servers installed at build time | Epic 5, Story 5.1 | Covered |
| FR42 | Git wrapper and isolation scripts baked in | Epic 4, Story 4.3 | Covered |
| FR43 | Content-hash image tagging | Epic 1, Story 1.5 | Covered |

### Missing Requirements

#### Critical Missing FRs

**FR9a:** Developer can enable automatic dependency isolation (`auto_isolate_deps: true`) to create anonymous volume mounts over platform-specific dependency directories (e.g., `node_modules/`) within mounted project paths
- Impact: Cross-platform development (macOS host -> Linux container) will fail for projects with native dependencies
- Recommendation: Add to Epic 2 (Project Integration) as a new story

**FR9b:** When `auto_isolate_deps` is enabled, the system scans mounted project paths at launch for `package.json` files and creates anonymous Docker volume mounts for each corresponding `node_modules/` directory
- Impact: Core mechanism for dependency isolation is missing from implementation plan
- Recommendation: Add to Epic 2 alongside FR9a

**FR9c:** The system logs all auto-detected dependency isolation mounts at launch so the developer has visibility into what was isolated
- Impact: Developer visibility into automatic isolation behavior
- Recommendation: Add to the same Epic 2 story as FR9a/FR9b

**FR16a:** When `auto_isolate_deps` is enabled, the system creates anonymous volume mounts for each detected dependency directory during sandbox launch
- Impact: Runtime implementation of dependency isolation is missing
- Recommendation: Add to Epic 2 alongside FR9a-FR9c (this is the runtime counterpart)

### Coverage Statistics

- Total PRD FRs: 47
- FRs covered in epics: 43
- FRs missing from epics: 4 (FR9a, FR9b, FR9c, FR16a)
- Coverage percentage: 91.5%

All 4 missing FRs relate to the `auto_isolate_deps` feature for anonymous volume mounts to prevent macOS/Linux dependency clashes. This feature is described in the PRD (added via edit on 2026-03-26) but was not carried through to the epics document.

## UX Alignment Assessment

### UX Document Status

Not Found — and not required.

### Assessment

Sandbox is a CLI-only developer tool with no GUI, web interface, or IDE plugin. The PRD states: "The user interface is entirely terminal-based: a configuration file defines the environment, and a shell script handles build and run operations. There is no GUI, no web interface, no IDE plugin." The epics document confirms: "N/A -- sandbox is a CLI tool with no GUI. No UX design document applicable."

### Alignment Issues

None — UX documentation is not applicable for this project type.

### Warnings

None — no UX document is needed. CLI usability concerns (error messages, help output, flag naming) are adequately covered in the PRD's functional requirements (FR9, FR15, FR16) and non-functional requirements (NFR13).

## Epic Quality Review

### User Value Focus

| Epic | Title | User Value? | Assessment |
|---|---|---|---|
| Epic 1 | Sandbox Foundation — Developer Can Build and Launch a Sandbox | Yes | Clear user outcome: developer gets a working sandbox |
| Epic 2 | Project Integration — Agent Can Work With Project Files and Git | Yes | Clear user outcome: agent can do real development work |
| Epic 3 | Isolation Boundaries — Sandbox Enforces Security Constraints | Borderline | Framed as system behavior, but user value is trust/safety — acceptable |
| Epic 4 | Inner Container Runtime — Agent Can Build and Run Docker Services | Yes | Clear user outcome: agent can build and run Docker services |
| Epic 5 | MCP Integration — Agent Can Run Browser Tests via Playwright | Yes | Clear user outcome: agent can run E2E tests |
| Epic 6 | Host Agent Config Inheritance | Borderline | Technical, but user value is "no re-auth per session" — acceptable |
| Epic 7 | Sandbox Runtime Hardening | No — technical milestone | Sprint change/bug fix epic — acceptable as corrective work |

### Epic Independence

- Epic 1: Fully independent — stands alone
- Epic 2: Depends on Epic 1 (needs running sandbox) — acceptable forward dependency
- Epic 3: Depends on Epic 1 (needs sandbox to enforce boundaries) — acceptable
- Epic 4: Depends on Epic 1 (needs base image build) — acceptable, no dependency on Epic 2 or 3
- Epic 5: Depends on Epic 1 (needs image build). Story 5.2 AC references "running web application on an inner container" which implies Epic 4 — but Epic 4 comes before Epic 5, so this is a backward dependency (acceptable)
- Epic 6: Depends on Epic 1 and 2 — acceptable
- Epic 7: Bug fix epic, depends on prior epics — acceptable for corrective work

No forward dependencies or circular dependencies found.

### Story Quality Assessment

#### Acceptance Criteria Quality

All 22 stories across 7 epics use proper Given/When/Then BDD format. ACs are:
- Testable and specific
- Cover both happy path and error scenarios
- Reference specific NFRs where applicable (e.g., NFR5, NFR10, NFR14)
- Include exit codes and error message expectations

#### Story Sizing

All stories are appropriately sized — each delivers a discrete, independently testable capability. No epic-sized stories found. No stories that are trivially small.

#### Within-Epic Dependencies

- Epic 1: Stories 1.1→1.2→1.3→1.4→1.5→1.6 form a natural build sequence (CLI skeleton → config init → config parsing → Dockerfile generation → image build → run). Each story builds on the previous one's output. This is acceptable — within-epic sequential dependencies are normal.
- Epic 2: Stories 2.1→2.2→2.3→2.4 are mostly independent (mounts, secrets, agent runtime, git/CLI). Story 2.3 implicitly depends on 2.1 (needs mounted files) — acceptable.
- Epic 3: Stories 3.1 and 3.2 are independent of each other.
- Epic 4: Story 4.1 (Podman install) must come before 4.2 (build/run containers). Story 4.3 (scripts baked in) is independent. Acceptable.
- Epic 5: Story 5.1 (MCP install) must come before 5.2 (MCP config/merge). Acceptable.
- Epic 7: Stories 7.1-7.4 are independent bug fixes that can be done in any order.

### Best Practices Compliance Checklist

| Check | Epic 1 | Epic 2 | Epic 3 | Epic 4 | Epic 5 | Epic 6 | Epic 7 |
|---|---|---|---|---|---|---|---|
| Delivers user value | Pass | Pass | Pass | Pass | Pass | Pass | Minor (bug fix) |
| Functions independently | Pass | Pass | Pass | Pass | Pass | Pass | Pass |
| Stories appropriately sized | Pass | Pass | Pass | Pass | Pass | N/A* | Pass |
| No forward dependencies | Pass | Pass | Pass | Pass | Pass | N/A* | Pass |
| Clear acceptance criteria | Pass | Pass | Pass | Pass | Pass | N/A* | Pass |
| FR traceability | Pass | Pass | Pass | Pass | Pass | Partial | Partial |

*Epic 6 has no detailed stories in the document — it is listed as a "Tech Spec" epic with only a description and FR reference.

### Quality Violations

#### Major Issues

1. **Epic 6 has no stories** — It is listed in the Epic List section with a description ("The sandbox can mount the host's agent config directory...") but has no detailed story breakdown with acceptance criteria. However, Story 7.4 in Epic 7 appears to address the same concern (host agent config mount). This creates confusion about where the feature is actually being implemented.
   - **Recommendation:** Either add stories to Epic 6 or merge its scope explicitly into Epic 7, Story 7.4. Clarify whether Epic 6 is superseded by Epic 7.

2. **Missing `auto_isolate_deps` stories** — As identified in coverage validation, the 4 FRs (FR9a, FR9b, FR9c, FR16a) related to automatic dependency isolation have no corresponding epic or story.
   - **Recommendation:** Add a new story to Epic 2 (Project Integration) covering the full `auto_isolate_deps` feature.

#### Minor Concerns

1. **Epic 7 naming** — "Runtime Hardening" is a technical milestone name rather than user-value framing. Better: "Agent Can Use All Runtime Capabilities Without Failures."
   - **Impact:** Low — the epic content and stories are well-structured despite the name.

2. **Epic 3 has only 2 stories** covering 4 FRs — Story 3.2 bundles filesystem isolation (FR32) and credential isolation (FR33) into a single story. This is borderline but acceptable since both are aspects of the same container boundary enforcement.

3. **No explicit database/entity creation concerns** — Not applicable for this project (CLI tool with no database).

## Summary and Recommendations

### Overall Readiness Status

**READY — with minor corrections needed**

The planning artifacts are comprehensive, well-structured, and largely aligned. The PRD is thorough with 47 testable FRs and 14 NFRs. The architecture document exists and is current. The epics document covers 91.5% of PRD requirements with detailed stories using proper BDD acceptance criteria. The project can proceed to implementation after addressing the items below.

### Critical Issues Requiring Immediate Action

1. **Missing `auto_isolate_deps` stories (FR9a, FR9b, FR9c, FR16a)** — 4 functional requirements for automatic dependency isolation were added to the PRD on 2026-03-26 but never carried into the epics document. This feature prevents macOS/Linux native module clashes and is listed as MVP in the PRD.

2. **Epic 6 has no stories** — Listed as "Host Agent Config Inheritance (Tech Spec)" with no detailed story breakdown. Its scope appears partially addressed by Epic 7, Story 7.4, creating ambiguity about where the feature is actually implemented.

### Recommended Next Steps

1. **Add an `auto_isolate_deps` story to Epic 2** — Create a story covering FR9a, FR9b, FR9c, and FR16a (config option, package.json scanning, anonymous volume mount creation, and launch-time logging). This closes the 4-FR gap.

2. **Clarify Epic 6 status** — Either add detailed stories to Epic 6 with acceptance criteria, or formally note that Epic 6 is superseded by Epic 7 Story 7.4 and remove it from the active epic list.

3. **Proceed to implementation** — With the above corrections, all 47 FRs will have traceable implementation paths. The epics are well-ordered, stories are properly sized with testable ACs, and no blocking structural issues exist.

### Final Note

This assessment identified **3 issues** across **2 categories** (coverage gaps and epic structure). The issues are straightforward to resolve — one new story and one clarification. The overall planning quality is high: the PRD is detailed and specific, the epics follow best practices for user-value framing and independence, and acceptance criteria are consistently in proper BDD format with error scenarios covered. The project is in strong shape for implementation.

**Assessed by:** Implementation Readiness Workflow
**Date:** 2026-03-27
