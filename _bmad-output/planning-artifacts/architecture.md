---
stepsCompleted:
  - step-01-init
  - step-02-context
  - step-03-starter
  - step-04-decisions
  - step-05-patterns
  - step-06-structure
  - step-07-validation
  - step-08-complete
lastStep: 8
status: 'complete'
completedAt: '2026-03-24'
inputDocuments:
  - planning-artifacts/prd.md
workflowType: 'architecture'
project_name: 'sandbox'
user_name: 'Manuel'
date: '2026-03-23'
---

# Architecture Decision Document

_This document builds collaboratively through step-by-step discovery. Sections are appended as we work through each architectural decision together._

## Project Context Analysis

### Requirements Overview

**Functional Requirements:**
46 functional requirements across 6 categories:
- **Sandbox Configuration (FR1-FR9, FR9a-FR9c):** YAML-driven configuration for SDKs, packages, MCP servers, mounts, secrets, env vars, agent selection, and config path override. `sandbox init` generates starter config. Optional `auto_isolate_deps` enables automatic detection and isolation of platform-specific dependency directories (e.g., `node_modules/`) via named volumes.
- **Sandbox Lifecycle (FR10-FR16, FR16a):** Build, run, and auto-rebuild lifecycle managed by shell CLI. TTY mode with Ctrl+C lifecycle. Dependency and secret validation before launch. When `auto_isolate_deps` is enabled, named volume mounts are assembled at launch for detected dependency directories.
- **Agent Runtime (FR17-FR22):** AI agents (Claude Code, Gemini CLI) run with full terminal access inside the sandbox. Agents interact with mounted project files and can execute BMAD workflows.
- **Development Toolchain (FR23-FR30):** Local git, internet access, CLI tools, Docker/Docker Compose, Playwright MCP, and MCP server management inside the sandbox.
- **Isolation Boundaries (FR31-FR37):** Git push blocked via wrapper, host filesystem restricted to declared mounts, host credentials inaccessible, inner containers network-isolated, non-privileged inner Docker, standard error codes at boundaries.
- **Image Build System (FR38-FR43):** Dockerfile generated from template, base images pinned to digest, SDK versions as build args, MCP servers installed at build time, content-hash image tagging.

**Non-Functional Requirements:**
14 NFRs across 3 categories:
- **Security & Isolation (NFR1-NFR6):** No implicit credential access, git push fails with standard errors, inner containers cannot bind external interfaces, no privileged mode, secrets as runtime env vars only, mounts limited to declared paths.
- **Integration (NFR7-NFR10):** Docker Engine 20.10+ / Podman compatible, yq v4+ required, MCP protocol standard, git wrapper transparent except for push.
- **Portability & Reliability (NFR11-NFR14):** macOS (arm64/amd64) and Linux (amd64), bash 4+, reproducible builds via digest pinning, fast-fail with clear errors, clean shutdown with no orphans.

**Scale & Complexity:**

- Primary domain: DevOps / CLI tooling (bash, Docker, container orchestration)
- Complexity level: Medium — the real complexity concentrates in the inner Docker isolation model (daemon requirements, base image size, startup time), not in the CLI tooling itself
- Architectural components: 5 (CLI entry point, config parser, image builder, runtime launcher, MCP integration). Isolation is not a sixth component — it is a design constraint enforced within each of the other five.

### Technical Constraints & Dependencies

- **Hard dependency on Docker (or Podman)** — the entire product is a Docker wrapper
- **Hard dependency on yq v4+** — YAML parsing for nested config structures
- **Bash 4+ required** — arrays, string manipulation features used throughout. Note: macOS ships bash 3.2 (GPLv2, 2007). Every macOS user must `brew install bash` or equivalent. This affects shebang line, install instructions, and CI testing matrix.
- **macOS + Linux portability** — must work on both, with macOS as primary dev platform
- **Inner Docker model undecided** — rootless Docker, sysbox, or Podman; highest-risk decision. Key implication: without host socket mount, inner Docker must run its own daemon or use a daemonless alternative. This determines base image size (rootless Docker adds ~200MB), startup time (daemon initialization), and CI compatibility. Single highest-impact constraint on the entire build system.
- **Base images pinned to digest** — reproducibility constraint affects update strategy
- **No privileged mode, no host socket mount** — hard security constraint that limits inner Docker options

### Threat Model Boundaries

The security model protects against **accidental leakage from AI agents that hallucinate or attempt unintended operations**. It is NOT designed to resist deliberate adversarial exfiltration. Specific boundaries to document:

- **Git wrapper** is a convenience boundary, not a security boundary. It intercepts the `git push` command but does not prevent code exfiltration via `curl` to a git HTTP endpoint, `ssh`, or other network tools. For the stated threat model (accidental, not adversarial), this is appropriate.
- **Secrets as env vars** are visible via `docker inspect` and `/proc/*/environ` inside the container. Acceptable for single-developer use. Must be revisited if multi-tenant or shared sandbox scenarios are added in Phase 3.
- **Outbound internet access** means an agent could theoretically exfiltrate any data it can read. Mitigation is scoped secrets (inject only what's needed). Egress filtering is explicitly deferred.

### Cross-Cutting Concerns Identified

- **Isolation as design constraint** — not a standalone component, but a behavior enforced differently in each component: git wrapper (shell script), network isolation (Docker flags), filesystem isolation (mount config), secret isolation (runtime env handling). Each component owns its own isolation enforcement.
- **Configuration parsing and data flow direction** — config.yaml is parsed once and consumed downstream. The architecture must enforce a unidirectional pipeline: `config.yaml -> parsed config -> Dockerfile -> image -> container`. The runtime launcher must NOT re-parse config independently; it should read build-time metadata from the image or receive it from the same parsed config. Two independent parsers will diverge.
- **Config-to-builder interface** — the architecture must define the contract between config parsing and image building: what data structure does the parser output that the builder consumes? This prevents drift when an AI agent implements them as separate modules.
- **Error handling at boundaries** — every isolation boundary must return standard CLI error codes so agents adapt gracefully. Consistency across git wrapper, network rules, filesystem restrictions.
- **Content-hash caching scope** — hash inputs must be carefully scoped to only image-affecting files. Hashing the entire sandbox repo (including comments in entrypoint.sh) would trigger unnecessary rebuilds. Define the exact set of files that constitute the cache key.
- **Path resolution** — mount paths resolved relative to config file location, not working directory. Used in mount assembly and config file discovery.

## Starter Template Evaluation

### Primary Technology Domain

Bash shell script CLI tool wrapping Docker/Podman operations. No web framework, no compilation, no package manager — distributed as source code.

### Foundation Decisions

**Script Structure:** Single `sandbox.sh` file with bash functions for each command (init, build, run). Monolithic by design — keeps distribution simple (one file to copy/symlink). Functions can be extracted to sourced modules later if complexity warrants it.

**Base Image:** Ubuntu 24.04 LTS, pinned to digest. Chosen for widest package availability, long-term support (through 2029), and familiarity for AI agents that need to install additional tools at runtime.

**Testing Strategy:** Manual validation for MVP. No automated bash testing framework. Bats-core is the natural choice for Phase 2 when automated testing is added.

**Inner Docker Approach (Preliminary):** Podman is the leading candidate based on constraint analysis:
- Sysbox ruled out (Linux-only, Manuel develops on macOS)
- Rootless Docker DinD requires `--privileged` on outer container (violates NFR4)
- Podman: daemonless, rootless by default, no `--privileged` needed, Docker-compatible CLI
- Full trade-off analysis deferred to Architecture Decisions step

### Project Structure

```
sandbox/
├── sandbox.sh              # Single CLI script (init, build, run)
├── Dockerfile.template     # Template for generating sandbox Dockerfiles
├── templates/
│   └── config.yaml         # Default config generated by `sandbox init`
├── scripts/
│   ├── entrypoint.sh       # Container entrypoint script
│   └── git-wrapper.sh      # Git wrapper that blocks push
└── README.md
```

### Architectural Decisions Provided by Foundation

- **Language & Runtime:** Bash 4+, no compilation, yq v4+ for YAML parsing
- **Container Base:** Ubuntu 24.04 LTS pinned to digest
- **Inner Container Runtime:** Podman (preliminary, pending full evaluation)
- **Distribution:** Source code via git, single script + supporting files
- **Configuration:** YAML at `.sandbox/config.yaml` per project

## Core Architectural Decisions

### Decision Priority Analysis

**Critical Decisions (Block Implementation):**
1. Inner container runtime: Podman (daemonless, rootless, no --privileged)
2. Dockerfile generation: Bash template substitution with conditional block markers
3. Git wrapper: Shell script at /usr/local/bin/git
4. Entrypoint: Simple entrypoint.sh with tini for signal handling

**Important Decisions (Shape Architecture):**
5. Network isolation: Default Podman rootless networking
6. Secret injection: Fail-closed on undeclared, empty values allowed
7. MCP integration: Pre-configured via .mcp.json generated at container startup

**Deferred Decisions (Post-MVP):**
- Egress filtering for outbound network
- Audit trail / activity logging
- Multi-agent orchestration
- Session persistence options
- Agent-specific MCP configuration for Gemini CLI

### Inner Container Runtime

- **Decision:** Podman 5.8.x
- **Rationale:** Only option satisfying all constraints simultaneously — daemonless (no startup overhead), rootless by default (no --privileged on outer container), Docker-compatible CLI. Sysbox ruled out (Linux-only). Rootless Docker DinD requires --privileged on outer container (violates NFR4).
- **Agent transparency:** `docker` aliased to `podman` inside the container. `docker compose` calls route through `podman compose`. Agent does not know it's using Podman.
- **Affects:** Image builder (Podman installed at build time), runtime launcher (no daemon startup needed), isolation layer (rootless networking is inherently isolated)

### Dockerfile Generation

- **Decision:** Bash template substitution (Option A)
- **Rationale:** Single mechanism to understand, full control over output, template is human-inspectable. Conditional blocks marked with `{{IF_SDK}}` / `{{/IF_SDK}}` tags, stripped or kept by sandbox.sh based on config. Version values substituted in the same pass.
- **Pattern:**
  ```
  # {{IF_NODE}}
  RUN curl -fsSL ... | bash - && apt-get install -y nodejs=${NODE_VERSION}
  # {{/IF_NODE}}
  ```
- **Affects:** Image builder (template processing logic), config parser (must output values consumable by template engine)

### Isolation Mechanisms

**Git Wrapper:**
- **Decision:** Shell script at `/usr/local/bin/git` that intercepts `push` and returns standard "unauthorized" error, passes all other commands to `/usr/bin/git`
- **Rationale:** Simple, testable, transparent for all operations except push. Aligns with accidental-not-adversarial threat model.
- **Affects:** Image builder (wrapper baked into image), agent runtime (transparent)

**Network Isolation:**
- **Decision:** Default Podman rootless networking, no explicit network configuration
- **Rationale:** Rootless Podman uses slirp4netns/pasta, which is already isolated from the host network. Inner containers can reach each other and the internet, but nothing outside can reach in. No extra configuration needed.
- **Affects:** Runtime launcher (no network setup steps), isolation (inherent to Podman rootless)

**Secret Injection:**
- **Decision:** Fail-closed on undeclared secrets, empty values are valid
- **Rationale:** If a secret name declared in config is not set in the host environment (not declared at all), sandbox.sh errors before launching. If set to empty string, it's passed through — this allows developers to `unset` a secret rather than editing config. Check uses `${VAR+x}` (declared?) not `${VAR:+x}` (non-empty?).
- **Affects:** CLI entry point (validation logic), runtime launcher (--env flag assembly)

### Container Lifecycle & MCP

**Entrypoint:**
- **Decision:** Simple `entrypoint.sh` with tini as PID 1
- **Rationale:** Podman is daemonless (no background service to manage), agent is the single foreground process. Tini handles signal forwarding (SIGTERM on Ctrl+C) and zombie reaping. Entrypoint sets up environment (PATH for git wrapper, MCP config), then `exec`s into agent command.
- **Startup sequence:** tini -> entrypoint.sh -> (setup env, write .mcp.json) -> exec agent
- **Affects:** Image builder (tini installed at build time), runtime launcher (tini as entrypoint)

**MCP Integration:**
- **Decision:** Entrypoint generates `.mcp.json` in workspace root based on configured MCP servers
- **Rationale:** Explicit and inspectable — the agent finds a standard `.mcp.json` and MCP servers are immediately available. No agent-specific knowledge needed at build time. If user's project already has a `.mcp.json`, entrypoint merges sandbox MCP servers into it.
- **Format:** Standard MCP project configuration:
  ```json
  {
    "mcpServers": {
      "playwright": {
        "type": "stdio",
        "command": "npx",
        "args": ["-y", "@playwright/mcp"]
      }
    }
  }
  ```
- **Affects:** Image builder (MCP packages installed at build time), entrypoint (generates config), agent runtime (discovers MCP via standard config)

### Automatic Dependency Isolation (`auto_isolate_deps`)

- **Decision:** Host-side scan of mounted project paths for `package.json` files at launch, with named Docker/Podman volumes overlaying each corresponding `node_modules/` directory inside the container
- **Rationale:** macOS-compiled native modules in `node_modules/` crash inside the Linux sandbox. Named volumes (not anonymous, not host-mapped) provide an isolated Linux-native `node_modules/` that persists across sessions, avoiding re-install on every launch. The scan runs on the host before `docker run` — once inside the container it's too late to add mounts.

**Detection Logic:**
- Triggered only when `auto_isolate_deps: true` is set in config
- For each mount declared in config: resolve the host-side source path
- Run `find <mount_source> -name package.json -not -path '*/node_modules/*'` to discover `package.json` files while excluding any nested inside existing `node_modules/` directories
- For each discovered `package.json`: derive the `node_modules` sibling path relative to the mount source

**Volume Assembly:**
- Each detected `node_modules/` becomes a named volume mount: `-v <volume_name>:<container_target_path>/node_modules`
- **Naming convention:** `sandbox-<project_name>-<relative_path_with_dashes>-node_modules`
  - Example: project `myapp`, mount target `/workspace`, `package.json` at root → volume name `sandbox-myapp-node_modules`
  - Example: project `myapp`, `packages/api/package.json` → volume name `sandbox-myapp-packages-api-node_modules`
- Named volumes are managed by Docker/Podman and persist across sessions — no host filesystem mapping
- Slashes in relative paths are replaced with dashes in the volume name

**Implementation Location:**
- New function `detect_isolate_deps()` in `sandbox.sh`
- Called from `run_sandbox()` after `parse_config()` but before `docker run` command assembly
- Returns additional `-v` flags appended to the run command
- If `auto_isolate_deps` is not set or `false`: function returns immediately with no output

**Logging:**
- Each discovered mount logged to stdout following existing output conventions:
  ```
  isolating: /workspace/node_modules (volume: sandbox-myapp-node_modules)
  isolating: /workspace/packages/api/node_modules (volume: sandbox-myapp-packages-api-node_modules)
  ```
- If no `package.json` files found (fresh project): no output, silent pass

**Edge Cases:**
- Fresh project with no `package.json`: no volumes created, no output — agent creates Linux-native deps from scratch on first `npm install`
- Monorepo with workspaces: multiple `package.json` files found, each gets its own named volume
- `auto_isolate_deps` absent or `false`: no scanning, no volumes, zero overhead

- **Affects:** `sandbox.sh` (new `detect_isolate_deps()` function in run path), `templates/config.yaml` (new `auto_isolate_deps` option with inline comment)

### Decision Impact Analysis

**Implementation Sequence:**
1. Dockerfile.template with Podman + tini + base tooling (foundation)
2. sandbox.sh config parser and template substitution engine
3. Git wrapper and entrypoint.sh scripts
4. MCP server installation and .mcp.json generation
5. Secret injection and validation logic
6. CLI commands (init, build, run)

**Cross-Component Dependencies:**
- Config parser outputs drive both template substitution (build time) and --env flag assembly (run time) — single parse, consumed twice
- Podman choice eliminates daemon startup from entrypoint, simplifying lifecycle
- MCP config generation depends on knowing which MCP servers were installed at build time — this metadata must flow from config through build into the container

## Implementation Patterns & Consistency Rules

### Purpose

These patterns ensure that AI agents implementing different parts of sandbox produce consistent, compatible code. Without these rules, agents could make different naming, formatting, and structural choices that create a fragmented codebase.

### Bash Coding Conventions

**Function naming:** `snake_case` — `parse_config`, `build_image`, `validate_secrets`

**Variable naming:**
- Local variables: `lower_snake_case` — `config_file`, `image_name`
- Constants/globals: `UPPER_SNAKE_CASE` — `DEFAULT_CONFIG_PATH`, `SANDBOX_VERSION`
- yq output variables: match config keys — `sdks.nodejs` becomes `sdk_nodejs`

**Quoting:** Always double-quote variable expansions: `"${var}"` not `$var`. Exception: inside `[[ ]]` tests.

**Error handling:**
- `set -euo pipefail` at script top
- Functions return exit codes, not echo to stdout for status
- `die()` helper for fatal errors: prints to stderr, exits non-zero

### Output & Error Formatting

- **Info:** plain text to stdout — `echo "Building sandbox image..."`
- **Errors:** `echo "error: <message>" >&2`
- **Warnings:** `echo "warning: <message>" >&2`
- **Success:** no prefix — `echo "Sandbox image built: ${image_tag}"`
- No color codes — clean output for piping and logging

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error (invalid config, missing file) |
| 2 | Usage error (bad arguments, unknown command) |
| 3 | Dependency error (Docker/Podman not found, yq missing/wrong version) |
| 4 | Secret validation error (undeclared secret in host env) |

### Template Placeholder Format

- **Conditional blocks:** `# {{IF_NAME}}` ... `# {{/IF_NAME}}` on their own lines as Dockerfile comments
- **Value substitution:** `{{NAME}}` inline — e.g., `nodejs={{NODE_VERSION}}`
- Block names derive from config keys in UPPER_SNAKE: `sdks.nodejs` -> `IF_NODE`, value -> `NODE_VERSION`

### Config YAML Conventions

- Keys are `lower_snake_case`
- List items are simple strings where possible: `- playwright`, `- build-essential`
- Mount entries use `source`/`target` keys (matching Docker convention)
- No nesting beyond two levels deep

### Script Organization (sandbox.sh)

Top-to-bottom reading order:
1. Shebang and `set -euo pipefail`
2. Constants and defaults
3. Utility functions (`die`, `info`, `warn`)
4. Config parsing functions
5. Build functions (template processing, docker build)
6. Run functions (flag assembly, docker run)
7. Init function (generate starter config)
8. Command dispatch (argument parsing, route to function)
9. Main entry point

**Comment style:**
- Section headers: `# --- Section Name ---`
- Function docs: one-line comment above function describing purpose
- No inline comments unless logic is non-obvious

### Enforcement Guidelines

**All AI Agents MUST:**
- Follow `set -euo pipefail` — no unset variable access, no silent failures
- Use `"${var}"` quoting — no unquoted expansions
- Write errors to stderr, not stdout
- Use the exit code table above — no arbitrary exit codes
- Follow the template marker format exactly — agents must not invent new placeholder syntaxes
- Keep sandbox.sh as a single file — no sourcing external modules without explicit approval

### Anti-Patterns

- `echo $var` — must be `echo "${var}"`
- `exit 1` with no error message — always `die "message"`
- Putting status messages on stderr or error messages on stdout
- Using `eval` for template substitution — use `sed` or parameter expansion
- Adding color codes, spinners, or progress bars
- Creating helper scripts outside the defined project structure

## Project Structure & Boundaries

### Complete Project Directory Structure

```
sandbox/
├── sandbox.sh                    # CLI entry point — all commands (init, build, run)
├── Dockerfile.template           # Template with conditional blocks and placeholders
├── templates/
│   └── config.yaml               # Starter config generated by `sandbox init`
├── scripts/
│   ├── entrypoint.sh             # Container entrypoint — env setup, .mcp.json, exec agent
│   └── git-wrapper.sh            # Git wrapper — blocks push, passes all else to /usr/bin/git
├── README.md                     # Usage, installation, configuration reference
└── LICENSE
```

### File Responsibilities

**sandbox.sh** — Single entry point, all project logic:
- Argument parsing and command dispatch (`init`, `build`, `run`, `--help`)
- Config file discovery and parsing via yq
- Dependency validation (Docker/Podman, yq, bash version)
- Secret validation (declared in config, checked against host env)
- Template processing (conditional block stripping, value substitution)
- Docker/Podman build command assembly and execution
- Docker/Podman run command assembly (mounts, env, TTY, tini)
- Content-hash computation for image tagging
- Starter config generation (`init` command)

**Dockerfile.template** — Human-readable template, NOT a valid Dockerfile until processed:
- Base image declaration (Ubuntu 24.04 LTS, pinned to digest)
- Tini installation
- Podman installation and rootless setup
- Conditional SDK installation blocks (`{{IF_NODE}}`, `{{IF_GO}}`, `{{IF_PYTHON}}`)
- Common CLI tools installation (curl, wget, dig, etc.)
- Git installation + wrapper deployment to /usr/local/bin/git
- MCP server package installation (conditional per config)
- User setup (non-root user for Podman rootless)
- Entrypoint configuration

**templates/config.yaml** — Starter config with inline comments explaining each option:
- Agent selection, SDK versions, packages, MCP servers
- Mount and secret declarations, environment variables
- `auto_isolate_deps` option (commented out by default, with explanation of when to enable)
- Serves as both documentation and scaffolding

**scripts/entrypoint.sh** — Runs inside container at startup:
- Sets up PATH (git wrapper takes precedence)
- Generates `.mcp.json` in workspace from build-time MCP metadata
- Merges with existing `.mcp.json` if present in mounted project
- Execs into configured agent command (claude/gemini)

**scripts/git-wrapper.sh** — Isolation boundary script:
- Checks first argument for `push` — returns exit code 1 with "fatal: Authentication failed" message
- All other arguments — passes through to `/usr/bin/git` unchanged

### Architectural Boundaries

**Build-Time vs Run-Time Boundary:**
- `sandbox.sh` operates at build time (template processing, image building) AND run time (flag assembly, container launch)
- `Dockerfile.template` -> resolved Dockerfile is a build-time artifact only
- `entrypoint.sh` and `git-wrapper.sh` are build-time installed, run-time executed
- Config is parsed at build time (for template) and run time (for docker run flags) — same parse logic, invoked twice

**Host vs Container Boundary:**
- `sandbox.sh` runs on the host — has access to host env, filesystem, Docker/Podman CLI
- Everything in `scripts/` runs inside the container — no host access
- Mounts are the only bridge: declared paths flow host -> container
- Secrets bridge host -> container via `--env` flags at launch

**Agent vs Sandbox Boundary:**
- The agent sees: a normal Linux environment with tools, a workspace, and a `.mcp.json`
- The agent does NOT see: that git push is intercepted, that Docker is actually Podman, that it's in a container
- Standard error codes at every boundary — no sandbox-specific errors

### Data Flow

```
config.yaml --> sandbox.sh (parse) --> Dockerfile.template (process) --> Dockerfile (resolved)
                     |                                                         |
                     |                                                    docker build
                     |                                                         |
                     |                                                    sandbox image
                     |                                                         |
                     +---> sandbox.sh (run) --> docker run (mounts, env, tty) -+
                                                    |
                                              +-----+-----+
                                              | container  |
                                              |  tini      |
                                              |  entrypoint|
                                              |  agent     |
                                              +------------+
```

### Requirements to Structure Mapping

| Requirement | File | Function/Section |
|---|---|---|
| FR1-FR7 (config options) | `templates/config.yaml`, `sandbox.sh` | Config parsing functions |
| FR9a (auto_isolate_deps config) | `templates/config.yaml`, `sandbox.sh` | Config parsing, `parse_config()` |
| FR9b (scan and create volumes) | `sandbox.sh` | `detect_isolate_deps()` |
| FR9c (log isolated mounts) | `sandbox.sh` | `detect_isolate_deps()` stdout logging |
| FR16a (volume mounts at launch) | `sandbox.sh` | `run_sandbox()` → `detect_isolate_deps()` → `-v` flags |
| FR8 (`-f` flag) | `sandbox.sh` | Command dispatch |
| FR9 (`sandbox init`) | `sandbox.sh`, `templates/config.yaml` | `init_config()` |
| FR10 (`sandbox build`) | `sandbox.sh` | `build_image()` |
| FR11 (`sandbox run`) | `sandbox.sh` | `run_sandbox()` |
| FR12 (auto-build) | `sandbox.sh` | `run_sandbox()` checks image exists |
| FR13 (change detection) | `sandbox.sh` | Content-hash comparison |
| FR14 (Ctrl+C stop) | `scripts/entrypoint.sh` | Tini signal forwarding |
| FR15 (dependency check) | `sandbox.sh` | `check_dependencies()` |
| FR16 (secret validation) | `sandbox.sh` | `validate_secrets()` |
| FR17-FR19 (agent runtime) | `scripts/entrypoint.sh` | Agent command exec |
| FR20-FR22 (project files, BMAD) | `sandbox.sh` | Mount flag assembly |
| FR23 (local git) | `Dockerfile.template` | Git installation |
| FR24-FR25 (internet, CLI tools) | `Dockerfile.template` | Tool installation |
| FR26-FR28 (Docker/Compose) | `Dockerfile.template` | Podman installation + alias |
| FR29-FR30 (MCP/Playwright) | `Dockerfile.template`, `scripts/entrypoint.sh` | MCP install + .mcp.json |
| FR31 (git push block) | `scripts/git-wrapper.sh` | Push interception |
| FR32-FR33 (filesystem/creds) | `sandbox.sh` | Mount restriction (only declared paths) |
| FR34-FR36 (network isolation) | Default Podman rootless networking | No explicit config |
| FR37 (standard errors) | `scripts/git-wrapper.sh` | Standard git error codes |
| FR38-FR42 (image build) | `sandbox.sh`, `Dockerfile.template` | Template processing + build |
| FR43 (content-hash tags) | `sandbox.sh` | `compute_image_tag()` |

## Architecture Validation Results

### Coherence Validation

**Decision Compatibility:** All technology choices (Bash 4+, yq v4+, Podman 5.8.x, Ubuntu 24.04 LTS, tini) are independent tools that compose via CLI with no version conflicts or incompatibilities.

**Pattern Consistency:** snake_case naming used consistently across bash functions, variables, and YAML keys. Error handling patterns (set -euo pipefail, die(), stderr) are consistent across all scripts. Template markers are syntactically distinct from YAML and bash variable expansion.

**Structure Alignment:** Single-file sandbox.sh with defined function order. scripts/ directory cleanly separates host-side from container-side code. Data flow is unidirectional: config -> parse -> template -> build -> run.

### Requirements Coverage

**Functional Requirements:** All 46 FRs (FR1-FR43, FR9a-FR9c, FR16a) have explicit architectural support mapped to specific files and functions in the Project Structure section.

**Non-Functional Requirements:** All 14 NFRs (NFR1-NFR14) are architecturally addressed through Podman rootless (security), version pinning (integration), tini signal handling (reliability), and bash 4+ with macOS caveat documented (portability).

### Gap Analysis & Resolutions

**Gap 1: Podman-Docker alias mechanism**
- Resolution: Symlink `/usr/local/bin/docker` -> `/usr/bin/podman` installed at build time in Dockerfile.template. No wrapper script needed — Podman's CLI is Docker-compatible.

**Gap 2: MCP metadata flow from build to runtime**
- Resolution: Dockerfile.template writes `/etc/sandbox/mcp-servers.json` during build, listing installed MCP servers. Entrypoint reads this manifest to generate `.mcp.json` at startup. Keeps build-time and run-time decoupled — entrypoint does not need to re-parse config.yaml.

**Gap 3: Inner container cleanup on Ctrl+C**
- Resolution: Podman's fork-exec model means inner containers are child processes that terminate when the parent process dies. Tini forwards SIGTERM to entrypoint, which propagates to the agent and its children. Documented as expected behavior and noted as an adversarial validation test case.

**Gap 4: Config parse duplication risk**
- Resolution: Config parsing MUST happen through a single `parse_config()` function. Both `build_image()` and `run_sandbox()` consume its output. Never parse config.yaml directly with ad-hoc yq calls outside this function. This prevents the two parse paths (build-time and run-time) from diverging.

**Gap 5: Template processing error handling**
- Resolution: Template processing MUST validate before substituting: (1) all conditional blocks have matching open/close tags — every `{{IF_NAME}}` has a corresponding `{{/IF_NAME}}`, (2) all value placeholders `{{NAME}}` have corresponding config values or are inside a stripped conditional block. Fail with exit code 1 and a clear error message identifying the unmatched tag or missing value. Never produce a Dockerfile with unresolved placeholders.

**Gap 6: .mcp.json merge strategy**
- Resolution: Sandbox MCP servers are added to the project's existing `.mcp.json`. If a server name conflicts (same key already exists in project config), the project's version takes precedence — the user's explicit configuration wins over sandbox defaults. The entrypoint writes the merged result to `.mcp.json` and logs which servers were added vs. skipped to stdout.

**Gap 7: Content-hash cache key composition**
- Resolution: Content hash inputs are exactly: the project's `config.yaml`, `Dockerfile.template`, `scripts/entrypoint.sh`, `scripts/git-wrapper.sh`. Changes to `sandbox.sh`, `README.md`, `templates/config.yaml`, or `LICENSE` do NOT trigger rebuilds — they don't affect the image content.

**Gap 8: Podman version availability in Ubuntu 24.04**
- Resolution: Ubuntu 24.04's default apt repository may ship an older Podman version. Dockerfile.template should use the official Kubic/upstream Podman repository to ensure version 5.x is available. Pin the major version (5.x) but allow minor/patch updates for security fixes.

### Architecture Completeness Checklist

**Requirements Analysis**
- [x] Project context thoroughly analyzed (43 FRs, 14 NFRs categorized)
- [x] Scale and complexity assessed (medium — complexity in inner Docker, not CLI)
- [x] Technical constraints identified (bash 3.2 on macOS, no --privileged, no host socket)
- [x] Cross-cutting concerns mapped (isolation as constraint, unidirectional data flow, config-to-builder interface)
- [x] Threat model boundaries documented (accidental not adversarial)

**Architectural Decisions**
- [x] Inner container runtime decided (Podman 5.8.x via upstream repo)
- [x] Dockerfile generation approach decided (bash template substitution with validation)
- [x] Isolation mechanisms decided (git wrapper, Podman rootless networking, fail-closed secrets)
- [x] Container lifecycle decided (tini + entrypoint.sh + exec agent)
- [x] MCP integration decided (.mcp.json generated from build-time manifest, project config wins on conflicts)

**Implementation Patterns**
- [x] Bash coding conventions established (snake_case, quoting, error handling)
- [x] Output formatting standardized (info/error/warning prefixes, stderr)
- [x] Exit codes defined (0-4 range with specific meanings)
- [x] Template placeholder format specified (IF blocks, value substitution, validation required)
- [x] Script organization ordered (top-to-bottom reading order)
- [x] Single parse_config() mandate documented
- [x] Anti-patterns documented

**Project Structure**
- [x] Complete directory structure defined (6 files + LICENSE)
- [x] File responsibilities documented
- [x] Architectural boundaries established (build/run, host/container, agent/sandbox)
- [x] Data flow mapped
- [x] All 46 FRs mapped to specific files and functions (including FR9a-FR9c, FR16a for auto_isolate_deps)
- [x] Content-hash cache key files explicitly listed

### Architecture Readiness Assessment

**Overall Status:** READY FOR IMPLEMENTATION

**Confidence Level:** High — all requirements covered (including auto_isolate_deps added 2026-03-26), no critical gaps, 8 important gaps identified and resolved during validation, decisions are coherent and well-constrained.

**Key Strengths:**
- Minimal surface area — 6 files total, single-script CLI
- Every FR mapped to a specific file and function
- Clear boundaries between host, container, and agent concerns
- Podman's daemonless architecture eliminates an entire class of lifecycle complexity
- Threat model is explicitly scoped — no false security promises
- Gaps found during validation were all resolvable without changing core decisions

**Areas for Future Enhancement:**
- Egress filtering for outbound network (Phase 2+)
- Agent-specific MCP configuration for Gemini CLI
- Audit trail / activity logging
- Session persistence options
- Automated testing with bats-core

### Implementation Handoff

**AI Agent Guidelines:**
- Follow all architectural decisions exactly as documented
- Use implementation patterns consistently — especially quoting, error handling, and exit codes
- Respect the file structure — sandbox.sh is a single file, scripts/ is container-side only
- Template markers must use the exact `{{IF_NAME}}` / `{{/IF_NAME}}` format
- Template processing must validate before substituting — never produce unresolved placeholders
- Config parsing must go through a single `parse_config()` function — no ad-hoc yq calls
- MCP merge: project config wins on name conflicts
- Refer to this document for all architectural questions

**First Implementation Priority:**
1. Create project structure (sandbox.sh skeleton, Dockerfile.template, scripts/)
2. Implement `parse_config()` via yq — single function, consumed by build and run
3. Implement template processing with validation (matching tags, resolved values)
4. Build a minimal working container (Ubuntu 24.04 + tini + entrypoint)
5. Add Podman installation from upstream repo and Docker symlink alias
6. Add git wrapper and isolation boundaries
7. Add MCP server installation, manifest generation, and .mcp.json merge logic
8. Implement content-hash caching (config.yaml + Dockerfile.template + scripts/*)
9. Implement `detect_isolate_deps()` — host-side `package.json` scan, named volume assembly, logging
10. Polish CLI (init command, --help, error messages)
