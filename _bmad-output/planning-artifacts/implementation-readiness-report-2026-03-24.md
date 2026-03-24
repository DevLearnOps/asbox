---
stepsCompleted:
  - step-01-document-discovery
  - step-02-prd-analysis
  - step-03-epic-coverage-validation
  - step-04-ux-alignment
  - step-05-epic-quality-review
  - step-06-final-assessment
documentsIncluded:
  - prd.md
  - architecture.md
  - epics.md
documentsMissing:
  - UX Design document
---

# Implementation Readiness Assessment Report

**Date:** 2026-03-24
**Project:** sandbox

## Document Inventory

### Documents Found
| Document Type | File | Format |
|---|---|---|
| PRD | prd.md | Whole |
| Architecture | architecture.md | Whole |
| Epics & Stories | epics.md | Whole |

### Documents Missing
| Document Type | Status |
|---|---|
| UX Design | Not found - will impact UX assessment completeness |

### Duplicate Conflicts
None identified.

## PRD Analysis

### Functional Requirements

**Sandbox Configuration (FR1-FR9)**
- FR1: Developer can define SDK versions (Node.js, Go, Python) in a YAML configuration file
- FR2: Developer can specify additional system packages to install in the sandbox image
- FR3: Developer can configure which MCP servers to pre-install (e.g., Playwright)
- FR4: Developer can declare host directories to mount into the sandbox with source and target paths
- FR5: Developer can declare secret names that will be resolved from host environment variables at runtime
- FR6: Developer can set non-secret environment variables for the agent runtime
- FR7: Developer can select which AI agent runtime to use (claude-code, gemini-cli)
- FR8: Developer can override the default config file path with a `-f` flag
- FR9: Developer can generate a starter configuration file with sensible defaults via `sandbox init`

**Sandbox Lifecycle (FR10-FR16)**
- FR10: Developer can build a sandbox container image from configuration via `sandbox build`
- FR11: Developer can launch a sandbox session in TTY mode via `sandbox run`
- FR12: System automatically builds the image if not present when `sandbox run` is invoked
- FR13: System detects when configuration or template files have changed and triggers a rebuild
- FR14: Developer can stop a running sandbox session with Ctrl+C
- FR15: System validates that required dependencies (Docker, yq) are present before proceeding
- FR16: System validates that all declared secrets are set in the host environment before launching

**Agent Runtime (FR17-FR22)**
- FR17: AI agent can execute inside the sandbox with full terminal access
- FR18: Claude Code agent launches with permissions skipped (sandbox provides isolation)
- FR19: Gemini CLI agent launches with standard configuration
- FR20: Agent can interact with project files mounted from the host
- FR21: Agent can execute BMAD method workflows (read/write planning artifacts, project knowledge, output documents)
- FR22: Agent can install additional packages and dependencies at runtime via package managers

**Development Toolchain (FR23-FR30)**
- FR23: Agent can use git for local operations (add, commit, log, diff, branch, checkout, merge, amend)
- FR24: Agent can access the internet for fetching documentation, packages, and dependencies
- FR25: Agent can use common CLI tools (curl, wget, dig, etc.)
- FR26: Agent can build Docker images from Dockerfiles inside the sandbox
- FR27: Agent can run `docker compose up` to start multi-service applications inside the sandbox
- FR28: Agent can reach application ports of services running in inner containers
- FR29: Agent can run Playwright tests against running applications via MCP integration
- FR30: Agent can start MCP servers as needed via MCP protocol

**Isolation Boundaries (FR31-FR37)**
- FR31: System blocks git push operations via a git wrapper, returning standard "unauthorized" errors
- FR32: System prevents agent access to host filesystem beyond explicitly mounted paths
- FR33: System prevents agent access to host credentials, SSH keys, and cloud tokens not explicitly declared as secrets
- FR34: System prevents inner containers from being reachable outside the sandbox network
- FR35: System enforces non-privileged inner Docker (no `--privileged` mode, no host Docker socket mount)
- FR36: System provides a private network bridge for communication between inner containers only
- FR37: System returns standard CLI error codes when agent attempts operations outside the boundary

**Image Build System (FR38-FR43)**
- FR38: System generates a Dockerfile from a Dockerfile.template using configuration values
- FR39: System pins base images to digest for reproducible builds
- FR40: System passes SDK versions as build arguments to the Dockerfile
- FR41: System installs configured MCP servers at image build time
- FR42: System bakes git wrapper and isolation boundary scripts into the image
- FR43: System tags images using content hash of config + template + sandbox files for cache management

**Total FRs: 43**

### Non-Functional Requirements

**Security & Isolation (NFR1-NFR6)**
- NFR1: No host credential, SSH key, or cloud token is accessible inside the sandbox unless explicitly declared in the config file's secrets list
- NFR2: Git push operations fail with standard error codes regardless of remote configuration present in mounted repositories
- NFR3: Inner containers cannot bind to network interfaces reachable from outside the sandbox
- NFR4: The sandbox container runs without `--privileged` flag and without host Docker socket mount
- NFR5: Secrets are injected as runtime environment variables only -- never written to the filesystem, image layers, or build cache
- NFR6: Mounted paths are limited to those explicitly declared in config -- no implicit access to home directory, `.ssh`, `.aws`, or other host paths

**Integration (NFR7-NFR10)**
- NFR7: The CLI works with Docker Engine 20.10+ and is compatible with Podman as an alternative runtime
- NFR8: YAML configuration is parsed via `yq` v4+ -- the script fails clearly if `yq` is not installed or is an incompatible version
- NFR9: MCP servers installed in the sandbox follow the standard MCP protocol and are startable by any MCP-compatible agent
- NFR10: The git wrapper is transparent to the agent for all operations except push -- git log, diff, commit, branch, etc. behave identically to standard git

**Portability & Reliability (NFR11-NFR14)**
- NFR11: The CLI runs on macOS (arm64/amd64) and Linux (amd64) with bash 4+
- NFR12: Image builds are reproducible -- the same config + template + sandbox files produce an identical image regardless of when or where the build runs (base image pinned to digest)
- NFR13: The CLI fails fast with clear, actionable error messages when dependencies are missing, secrets are unset, or configuration is invalid
- NFR14: A crashed or Ctrl+C'd sandbox leaves no orphaned containers or dangling networks on the host

**Total NFRs: 14**

### Additional Requirements & Constraints

- **Security model scope:** Protects against accidental leakage from AI agents, not deliberately adversarial agents
- **Inner Docker approach:** Must be rootless Docker, sysbox, or Podman -- host socket mount and `--privileged` DinD explicitly ruled out
- **Mount path resolution:** Relative paths resolve relative to config file location, not working directory
- **Image naming convention:** `sandbox-<project-name>:<content-hash>`
- **Hard dependency:** `yq` for YAML parsing (eliminates fragile awk/sed parsing)
- **Distribution model:** Source code via git repo, no build/compilation step
- **Agent command mapping:** claude-code maps to `claude --dangerously-skip-permissions`, gemini-cli maps to `gemini`
- **Dockerfile generation:** Template with placeholder substitution, human-readable and inspectable

### PRD Completeness Assessment

The PRD is thorough and well-structured. Requirements are clearly numbered and organized by domain. The document covers configuration, lifecycle, agent runtime, toolchain, isolation boundaries, and image build concerns comprehensively. The security model scope is explicitly stated, and deferred features are clearly called out. No significant gaps detected at this stage -- coverage validation against epics will follow.

## Epic Coverage Validation

### Coverage Matrix

| FR | PRD Requirement | Epic Coverage | Status |
|---|---|---|---|
| FR1 | SDK versions in YAML config | Epic 1 | Covered |
| FR2 | System packages in config | Epic 1 | Covered |
| FR3 | MCP server configuration | Epic 5 | Covered |
| FR4 | Host directory mounts | Epic 2 | Covered |
| FR5 | Secret declaration and injection | Epic 2 | Covered |
| FR6 | Non-secret environment variables | Epic 1 | Covered |
| FR7 | Agent runtime selection | Epic 1 | Covered |
| FR8 | Config file path override (`-f`) | Epic 1 | Covered |
| FR9 | `sandbox init` starter config | Epic 1 | Covered |
| FR10 | `sandbox build` command | Epic 1 | Covered |
| FR11 | `sandbox run` in TTY mode | Epic 1 | Covered |
| FR12 | Auto-build on run | Epic 1 | Covered |
| FR13 | Change detection and rebuild | Epic 1 | Covered |
| FR14 | Ctrl+C stop | Epic 1 | Covered |
| FR15 | Dependency validation | Epic 1 | Covered |
| FR16 | Secret validation before launch | Epic 2 | Covered |
| FR17 | Agent terminal access | Epic 2 | Covered |
| FR18 | Claude Code with --dangerously-skip-permissions | Epic 2 | Covered |
| FR19 | Gemini CLI standard launch | Epic 2 | Covered |
| FR20 | Agent interacts with mounted files | Epic 2 | Covered |
| FR21 | BMAD workflow support | Epic 2 | Covered |
| FR22 | Runtime package installation | Epic 2 | Covered |
| FR23 | Local git operations | Epic 2 | Covered |
| FR24 | Internet access | Epic 2 | Covered |
| FR25 | Common CLI tools | Epic 2 | Covered |
| FR26 | Build Docker images inside sandbox | Epic 4 | Covered |
| FR27 | Docker Compose inside sandbox | Epic 4 | Covered |
| FR28 | Reach inner container ports | Epic 4 | Covered |
| FR29 | Playwright tests via MCP | Epic 5 | Covered |
| FR30 | Start MCP servers via protocol | Epic 5 | Covered |
| FR31 | Git push blocked | Epic 3 | Covered |
| FR32 | Filesystem restricted to mounts | Epic 3 | Covered |
| FR33 | Host credentials inaccessible | Epic 3 | Covered |
| FR34 | Inner containers not reachable externally | Epic 4 | Covered |
| FR35 | Non-privileged inner Docker | Epic 4 | Covered |
| FR36 | Private network bridge | Epic 4 | Covered |
| FR37 | Standard error codes at boundaries | Epic 3 | Covered |
| FR38 | Dockerfile from template | Epic 1 | Covered |
| FR39 | Base images pinned to digest | Epic 1 | Covered |
| FR40 | SDK versions as build args | Epic 1 | Covered |
| FR41 | MCP servers installed at build time | Epic 5 | Covered |
| FR42 | Git wrapper and isolation scripts baked in | Epic 4 | Covered |
| FR43 | Content-hash image tagging | Epic 1 | Covered |

### Missing Requirements

No missing FR coverage detected. All 43 functional requirements from the PRD are mapped to epics.

### Coverage Statistics

- Total PRD FRs: 43
- FRs covered in epics: 43
- Coverage percentage: 100%

## UX Alignment Assessment

### UX Document Status

Not Found -- and not required.

### Alignment Issues

None. The PRD explicitly classifies sandbox as a shell script CLI with no GUI, no web interface, and no IDE plugin. The epics document confirms: "N/A -- sandbox is a CLI tool with no GUI. No UX design document applicable."

### Warnings

None. The absence of a UX design document is expected and correct for a terminal-only CLI tool. All user interactions are via shell commands (`sandbox init`, `sandbox build`, `sandbox run`) and standard terminal I/O.

## Epic Quality Review

### Best Practices Compliance

#### Epic User Value Assessment

| Epic | Title | User Value | Verdict |
|---|---|---|---|
| Epic 1 | Developer Can Build and Launch a Sandbox | Developer gets a working sandbox | PASS |
| Epic 2 | Agent Can Work With Project Files and Git | Agent has complete dev workflow | PASS |
| Epic 3 | Sandbox Enforces Security Constraints | Developer gets safety guarantee | PASS |
| Epic 4 | Agent Can Build and Run Docker Services | Agent can test full-stack apps | PASS |
| Epic 5 | Agent Can Run Browser Tests via Playwright | Agent can do E2E testing | PASS |

No technical-milestone epics detected. All epics frame outcomes from user/agent perspective.

#### Epic Independence Assessment

| Epic | Dependencies | Verdict |
|---|---|---|
| Epic 1 | None (standalone) | PASS |
| Epic 2 | Epic 1 only | PASS |
| Epic 3 | Epic 1, 2 (but see issue below) | ISSUE |
| Epic 4 | Epic 1, 2, 3 | PASS |
| Epic 5 | Epic 1-4 | PASS |

#### Story Sizing & Structure

All 17 stories are appropriately sized, independently completable within their epic context, and follow proper Given/When/Then BDD acceptance criteria format.

#### Within-Epic Story Dependencies

- Epic 1: 1.1 -> 1.3 -> 1.4 -> 1.5 -> 1.6 (clean chain, 1.2 independent) -- PASS
- Epic 2: All depend on Epic 1; 2.3 builds on 2.1 -- PASS
- Epic 3: 3.1 and 3.2 independent of each other -- PASS
- Epic 4: 4.1 -> 4.2 -> 4.3 (clean chain) -- PASS
- Epic 5: 5.1 -> 5.2 (clean chain) -- PASS

### Quality Findings

#### Major Issues

**ISSUE-1: FR42 Placement Creates Forward Dependency (Epic 3 -> Epic 4)**

FR42 (git wrapper and isolation scripts baked into image) is assigned to Epic 4, Story 4.3. However:

1. The **Dockerfile.template** (Epic 1, Story 1.4) already includes git wrapper deployment per the architecture document: "Git installation + wrapper deployment to /usr/local/bin/git"
2. **Epic 1, Story 1.6** needs `entrypoint.sh` to launch the agent -- but entrypoint baking is covered by Story 4.3
3. **Epic 3, Story 3.1** tests git push blocking, which requires the git wrapper to already be in the image

This creates a confusing dependency where Epic 3 appears to depend on Epic 4. In practice, the Dockerfile.template processed in Epic 1 already installs these scripts, so Story 4.3 is verifying something that already exists rather than building new functionality.

**Recommendation:** Split Story 4.3:
- Move git wrapper installation and entrypoint baking to Epic 1 (foundation concern -- these are part of Dockerfile.template processing)
- Keep non-root user setup for Podman rootless in Epic 4 (specific to inner container runtime)

This eliminates the forward dependency and makes the epic boundaries cleaner.

#### Minor Concerns

**ISSUE-2: Missing AC for Non-Existent Mount Source Path (Story 2.1)**

Story 2.1 (Host Directory Mounts) has no acceptance criterion for what happens when a declared mount source path doesn't exist on the host. Should sandbox error before launch? Docker would fail with an unclear error.

**Recommendation:** Add AC: "Given a config declares a mount source path that does not exist on the host, When the developer runs `sandbox run`, Then the script exits with code 1 and prints a clear error identifying the missing path."

**ISSUE-3: Missing AC for Mount Write Permission Failure (Story 2.3)**

Story 2.3 assumes the agent can write to mounted directories, but has no AC for permission failures.

**Recommendation:** Low priority -- Docker mounts preserve host permissions by default. This is standard Docker behavior and likely not worth an explicit AC.

### Best Practices Checklist

| Check | Epic 1 | Epic 2 | Epic 3 | Epic 4 | Epic 5 |
|---|---|---|---|---|---|
| Delivers user value | PASS | PASS | PASS | PASS | PASS |
| Functions independently | PASS | PASS | ISSUE | PASS | PASS |
| Stories sized appropriately | PASS | PASS | PASS | PASS | PASS |
| No forward dependencies | PASS | PASS | ISSUE | PASS | PASS |
| Clear acceptance criteria | PASS | MINOR | PASS | PASS | PASS |
| FR traceability maintained | PASS | PASS | PASS | ISSUE | PASS |

## Summary and Recommendations

### Overall Readiness Status

**READY** -- with one structural issue to address in the epics document before implementation begins.

The planning artifacts are comprehensive, well-aligned, and implementation-ready. The PRD has 43 clearly numbered functional requirements and 14 non-functional requirements. The architecture document makes explicit decisions for every technical concern and maps all requirements to specific files and functions. The epics achieve 100% FR coverage across 5 user-value-oriented epics with 17 well-structured stories. One structural dependency issue needs resolution.

### Critical Issues Requiring Immediate Action

**1. Resolve FR42 forward dependency (ISSUE-1) -- HIGH PRIORITY**

Story 4.3 (Isolation Scripts Baked into Image) in Epic 4 contains foundation work that Epic 1 and Epic 3 depend on. The git wrapper and entrypoint.sh are part of the Dockerfile.template and are needed from the very first image build. This should be addressed before handing stories to an implementing agent, as the current structure would confuse an agent trying to implement epics sequentially.

**Options:**
- (A) Move git wrapper + entrypoint baking into Epic 1 (recommended -- cleanest fix)
- (B) Add a note to Epic 1 Story 1.4 that the Dockerfile.template includes all scripts, and reframe Story 4.3 as verification-only
- (C) Proceed as-is with awareness that the implementing agent should include scripts in Epic 1's image build regardless of where FR42 is formally assigned

### Recommended Next Steps

1. **Address ISSUE-1:** Restructure Story 4.3 to resolve the forward dependency. Move git wrapper and entrypoint installation into Epic 1, keep Podman non-root user setup in Epic 4.
2. **Consider ISSUE-2:** Add an acceptance criterion to Story 2.1 for non-existent mount source paths. This is a quick win that prevents unclear Docker errors.
3. **Begin implementation:** Start with Epic 1 stories sequentially. The architecture document provides a clear implementation handoff with explicit AI agent guidelines.

### Final Note

This assessment identified 3 issues across 2 categories (1 major structural issue, 2 minor AC gaps). The project planning is thorough -- the PRD, Architecture, and Epics documents are well-aligned with 100% FR coverage. The single major issue (FR42 placement) is a structural concern in the epics document, not a missing requirement. It can be resolved with a minor reorganization before implementation begins. These findings can be used to improve the artifacts, or you may choose to proceed as-is with awareness of the dependency.

**Assessed by:** Implementation Readiness Workflow
**Date:** 2026-03-24
