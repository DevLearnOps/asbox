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
completedAt: '2026-04-07'
inputDocuments:
  - planning-artifacts/prd.md
workflowType: 'architecture'
project_name: 'asbox'
user_name: 'Manuel'
date: '2026-04-07'
---

# Architecture Decision Document

_This document builds collaboratively through step-by-step discovery. Sections are appended as we work through each architectural decision together._

## Project Context Analysis

### Requirements Overview

**Functional Requirements:**
54 functional requirements across 6 categories:
- **Sandbox Configuration (FR1-FR9, FR9a-FR9e, FR56-FR59):** YAML-driven configuration for SDKs, packages, MCP servers, mounts, secrets, env vars, installed agents list, default agent selection (overridable via `--agent` CLI flag), config path override, auto_isolate_deps, boolean host_agent_config with automatic path resolution, and project_name override. Agent names use short form (`claude`, `gemini`, `codex`). `asbox init` generates starter config.
- **Sandbox Lifecycle (FR10-FR16, FR16a):** Build, run, and auto-rebuild lifecycle managed by Go CLI. TTY mode with Ctrl+C lifecycle via Tini. Docker presence and secret validation before launch. Named volume mounts assembled at launch when auto_isolate_deps enabled.
- **Agent Runtime (FR17-FR22):** AI agents (Claude Code, Gemini CLI, Codex CLI) run with full terminal access inside the sandbox. All agents run in permissionless mode — Claude Code with `--dangerously-skip-permissions`, Gemini CLI with `-y`, Codex CLI with `--dangerously-bypass-approvals-and-sandbox` (sandbox provides isolation). Agents interact with mounted project files and can execute BMAD workflows.
- **Development Toolchain (FR23-FR30):** Local git, internet access, CLI tools, Docker/Docker Compose via inner Podman, Playwright MCP (chromium + webkit), and MCP server management inside the sandbox.
- **Isolation Boundaries (FR31-FR37):** Git push blocked via wrapper, host filesystem restricted to declared mounts, host credentials inaccessible, inner containers network-isolated, non-privileged inner Docker, standard error codes at boundaries.
- **Image Build System (FR38-FR54):** Dockerfile generated from embedded Go `text/template`, base images pinned to digest, SDK versions as build args, MCP servers installed at build time, content-hash image tagging, agent instruction files baked in, Tini as PID 1, UID/GID alignment, MCP manifest merge, embedded assets via Go `embed`, bmad_repos multi-repo mounts with generated agent instructions, single binary distribution.

**Non-Functional Requirements:**
15 NFRs across 3 categories:
- **Security & Isolation (NFR1-NFR6):** No implicit credential access, git push fails with standard errors, inner containers cannot bind external interfaces, no privileged mode, secrets as runtime env vars only, mounts limited to declared paths.
- **Integration (NFR7-NFR10):** Docker Engine 20.10+ / Podman compatible on host, Go `gopkg.in/yaml.v3` for YAML parsing (no external dependency), MCP protocol standard, git wrapper transparent except for push.
- **Portability & Reliability (NFR11-NFR15):** macOS (arm64/amd64) and Linux (amd64) as statically-linked Go binary, reproducible builds via digest pinning, fast-fail with named errors and distinct exit codes (1-4), clean shutdown with Tini as PID 1, integration test suite covering all use cases with parallel Go test execution.

**Scale & Complexity:**

- Primary domain: Go CLI tooling / DevOps (container orchestration, image generation)
- Complexity level: Medium — complexity concentrates in inner Podman configuration, config-to-container data pipeline, and embedded asset management
- Architectural components: 6 (Go CLI with cobra, config parser, template engine, image builder, runtime launcher, embedded container scripts)

### Technical Constraints & Dependencies

- **Go as implementation language** — single statically-linked binary, all assets embedded via `embed` package. No external runtime dependencies on host beyond Docker.
- **Hard dependency on Docker (or Podman) on host** — the entire product wraps Docker operations via `os/exec`
- **No external parsing tools** — YAML via `gopkg.in/yaml.v3`, templates via Go `text/template`. No yq, no sed-based substitution.
- **Inner Podman 5.x** — decided, not deferred. Rootless, daemonless, `vfs` storage driver, `netavark`/`aardvark-dns` networking, `file` events logger. Docker CLI alias via `podman-docker`.
- **Base images pinned to digest** — Ubuntu 24.04 LTS, reproducibility constraint affects update strategy
- **No privileged mode, no host socket mount** — hard security constraint
- **macOS + Linux portability** — Go binary handles this natively; container-side scripts are bash running inside Ubuntu
- **Tini as PID 1** — signal forwarding and zombie reaping inside the container
- **UID/GID alignment at startup** — entrypoint uses `usermod`/`groupmod` with `HOST_UID`/`HOST_GID` env vars
- **Testcontainers compatibility** — Ryuk disabled, socket override, localhost host override set inside container

### Threat Model Boundaries

The security model protects against **accidental leakage from AI agents that hallucinate or attempt unintended operations**. It is NOT designed to resist deliberate adversarial exfiltration. Specific boundaries:

- **Git wrapper** is a convenience boundary, not a security boundary. Intercepts `git push` but does not prevent exfiltration via `curl`, `ssh`, or other network tools.
- **Secrets as env vars** are visible via `docker inspect` and `/proc/*/environ` inside the container. Acceptable for single-developer use.
- **Outbound internet access** means an agent could theoretically exfiltrate data. Mitigation is scoped secrets. Egress filtering is explicitly deferred.
- **Host agent config mount** (`host_agent_config`) is read-write and enabled by default — the agent can read and modify OAuth tokens. This is intentional for token refresh but is the widest trust grant in the system. Paths are resolved automatically from the agent config registry based on the selected agent.

### Cross-Cutting Concerns Identified

- **Two execution domains** — Go on host, bash inside container. The architecture must clearly define what runs where and how data flows across the boundary (config → Docker build args → image → Docker run flags → entrypoint env vars).
- **Embedded asset lifecycle** — all supporting files compiled into the binary. Changes require rebuild. Version-tagged releases make the relationship explicit. The architecture must define which files are embedded and how they're extracted/used.
- **Config as single source of truth** — `.asbox/config.yaml` is parsed once by the Go CLI and drives: template rendering, build arg assembly, run flag assembly, mount assembly, secret validation, auto_isolate_deps scanning, bmad_repos mounting. Single parse, multiple consumers. The `--agent` CLI flag can override `default_agent` after parsing but before any downstream consumers read it.
- **Content-hash scoping** — hash inputs must be carefully defined: rendered template output + config content. Changes to the Go CLI source do NOT trigger image rebuilds.
- **Isolation enforcement is distributed** — not a standalone component. Git wrapper (shell script), network isolation (Podman rootless), filesystem isolation (mount config), secret isolation (runtime env handling). Each component owns its own isolation enforcement.
- **Path resolution** — mount paths resolved relative to config file location, not working directory. Applies to mounts, bmad_repos, and host_agent_config.
- **MCP manifest flow** — build-time manifest at `/etc/sandbox/mcp-servers.json` merged with project `.mcp.json` at runtime. Project config wins on name conflicts.

## Starter Template Evaluation

### Primary Technology Domain

Go CLI binary wrapping Docker operations. No web framework, no frontend, no database — distributed as a single statically-linked binary with embedded assets.

### Foundation Decisions

**Language & Runtime:** Go (latest stable). Statically-linked binary via `CGO_ENABLED=0`. Cross-compiled for macOS (arm64/amd64) and Linux (amd64).

**CLI Framework:** Cobra — the standard Go CLI framework. Provides command dispatch (`init`, `build`, `run`), flag parsing (`-f`), and `--help` generation. Minimal surface — three commands, one flag.

**Configuration Parsing:** `gopkg.in/yaml.v3` for `.asbox/config.yaml`. Parsed into typed Go structs with validation.

**Template Engine:** Go's `text/template` with conditional blocks (`{{if}}`, `{{range}}`) for Dockerfile generation. Replaces the previous bash sed-based approach.

**Embedded Assets:** Go's `embed` package for all supporting files: Dockerfile template, entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh, agent-instructions.md, starter config.yaml.

**Testing Framework:** Go's built-in `testing` package with `t.Parallel()` for integration tests. Testcontainers-go for Docker-based integration testing.

**Build Tooling:** Standard `go build` with `goreleaser` for cross-platform release binaries.

**Project Structure:**

```
asbox/
├── main.go                     # Entry point
├── go.mod / go.sum             # Module definition
├── cmd/                        # Cobra command definitions
│   ├── root.go                 # Root command, global flags
│   ├── init.go                 # asbox init
│   ├── build.go                # asbox build
│   └── run.go                  # asbox run
├── internal/                   # Private application logic
│   ├── config/                 # YAML parsing, validation, typed structs
│   ├── template/               # Dockerfile template rendering + validation
│   ├── docker/                 # Docker/Podman CLI interaction via os/exec
│   ├── hash/                   # Content-hash computation for image caching
│   └── mount/                  # Mount assembly, auto_isolate_deps, bmad_repos
├── embed/                      # Embedded asset source files
│   ├── Dockerfile.tmpl         # Go text/template Dockerfile
│   ├── entrypoint.sh           # Container entrypoint
│   ├── git-wrapper.sh          # Git push interceptor
│   ├── healthcheck-poller.sh   # Healthcheck daemon
│   ├── agent-instructions.md   # Agent CLAUDE.md/GEMINI.md/CODEX.md template
│   └── config.yaml             # Starter config for asbox init
├── integration/                # Integration tests (testcontainers-go)
└── README.md
```

### Architectural Decisions Provided by Foundation

- **Language & Runtime:** Go with static linking, no CGO
- **CLI Framework:** Cobra for command dispatch and flag parsing
- **Configuration:** `gopkg.in/yaml.v3` into typed structs
- **Template Engine:** Go `text/template` for Dockerfile generation
- **Asset Management:** Go `embed` package — all files compiled into binary
- **Testing:** Go `testing` + testcontainers-go for integration tests
- **Distribution:** Single binary via goreleaser, GitHub releases
- **Project Layout:** Standard Go project layout with `cmd/`, `internal/`, `embed/`

## Core Architectural Decisions

### Decision Priority Analysis

**Critical Decisions (Block Implementation):**
1. Inner container runtime: Podman 5.x (daemonless, rootless, no --privileged)
2. Dockerfile generation: Go `text/template` with config validation before rendering
3. Git wrapper: Shell script at /usr/local/bin/git
4. Entrypoint: tini → entrypoint.sh → UID/GID align → env setup → exec agent

**Important Decisions (Shape Architecture):**
5. Network isolation: Podman rootless networking (netavark/aardvark-dns)
6. Secret injection: Fail-closed on undeclared, empty values allowed
7. MCP integration: Build-time manifest merged with project .mcp.json at startup
8. auto_isolate_deps: Host-side `filepath.WalkDir` scan, named volume assembly
9. bmad_repos: Convention-based mounts to `/workspace/repos/<name>`, collision detection, generated agent instructions

**Deferred Decisions (Post-MVP):**
- Egress filtering for outbound network
- Audit trail / activity logging
- Multi-agent parallel orchestration (multiple sandboxes simultaneously)
- Session persistence options
- Agent-specific MCP configuration for Gemini CLI
- `host_agent_config` integrity checking (snapshot config state on startup for post-session diff)

### Inner Container Runtime

- **Decision:** Podman 5.x installed from upstream Kubic/OBS repository
- **Rationale:** Only option satisfying all constraints — daemonless (no startup overhead), rootless by default (no --privileged on outer container), Docker-compatible CLI. Sysbox ruled out (Linux-only). Rootless Docker DinD requires --privileged (violates NFR4).
- **Configuration:** `vfs` storage driver (compatible with nested containers in Docker Desktop), `netavark` networking with `aardvark-dns` for service name DNS resolution, `file` events logger (avoids journald dependency)
- **Known trade-off:** `vfs` storage driver performs a full filesystem copy per layer — inner image pulls and builds will be slower and use more disk than overlay2. Acceptable for a development tool where agents pull a handful of images. Not a bug, documented as inherent to the nested container model.
- **Agent transparency:** `podman-docker` package provides docker CLI alias. `docker compose` routes through `podman compose`. Agent does not know it's using Podman.
- **Podman API socket:** Started at `$XDG_RUNTIME_DIR/podman/podman.sock` with `DOCKER_HOST` pointing to it for Docker SDK/Testcontainers compatibility. Socket owned by sandbox user only — must not be accessible from inner containers via mount leak.
- **Testcontainers compatibility:** Ryuk disabled, socket override configured, localhost host override set
- **Affects:** Image builder (Podman + podman-docker installed at build time), runtime launcher (no daemon startup needed), isolation (rootless networking is inherently isolated)

### Dockerfile Generation

- **Decision:** Go `text/template` with conditional blocks and value injection, preceded by config validation
- **Rationale:** Type-safe, readable, robust conditional logic via `{{if}}`, `{{range}}`, `{{with}}`. Replaces fragile bash sed substitution. Template is an embedded asset compiled into the binary.
- **Config validation before rendering:** Go's `text/template` does not leave unresolved placeholders — it either renders or errors on missing fields. The real risk is **zero-value fields**: if `config.SDKs.NodeJS` is empty string, the template silently renders an empty version. Validation MUST check for required-but-empty fields **before** template execution in `internal/config/`, not after rendering in `internal/template/`. Template rendering assumes a fully validated config struct.
- **Pattern:**
  ```
  {{if .SDKs.NodeJS}}
  RUN curl -fsSL ... | bash - && apt-get install -y nodejs={{.SDKs.NodeJS}}
  {{end}}
  ```
- **Affects:** `internal/config/` (validation of required fields), `internal/template/` (rendering only — assumes valid input), `embed/Dockerfile.tmpl` (template source)
- **Multi-architecture support:** All binary download steps in the Dockerfile template MUST detect host architecture at build time. Use `$(dpkg --print-architecture)` (returns `amd64`/`arm64`) for URLs using Debian-style arch names (e.g., Go SDK tarball). Use `$(uname -m)` (returns `x86_64`/`aarch64`) for URLs using kernel-style arch names (e.g., Docker Compose). Package manager installs (apt, npm) are architecture-transparent. Never hardcode architecture strings like `amd64` or `x86_64` in download URLs.

### Isolation Mechanisms

**Git Wrapper:**
- **Decision:** Shell script at `/usr/local/bin/git` that intercepts `push` and returns standard "Authentication failed" error, passes all other commands to `/usr/bin/git`
- **Rationale:** Simple, testable, transparent for all operations except push. Aligns with accidental-not-adversarial threat model. Note: a curious agent could call `/usr/bin/git push` directly — this is a known boundary of the accidental threat model, not a bug.
- **Affects:** Image builder (wrapper baked in), agent runtime (transparent)

**Network Isolation:**
- **Decision:** Podman rootless networking via netavark, private bridge for inner containers
- **Rationale:** Rootless Podman uses netavark/aardvark-dns, inherently isolated from host network. Inner containers can reach each other and the internet, but nothing outside can reach in.
- **Affects:** Runtime launcher (no network setup steps), isolation (inherent to Podman rootless)

**Secret Injection:**
- **Decision:** Fail-closed on undeclared secrets, empty values are valid
- **Rationale:** If a secret name declared in config is not set in the host environment (not declared at all), the CLI errors with exit code 4 before launching. If set to empty string, it's passed through. Go implementation checks via `os.LookupEnv()` (returns bool for existence).
- **Affects:** `cmd/run.go` (validation logic), `internal/docker/` (--env flag assembly)

### Error Handling Strategy

- **Error types per package:** Each `internal/` package defines its own typed errors (`ConfigError`, `SecretError`, `DependencyError`, `TemplateError`). No centralized errors package — keeps packages decoupled.
- **Exit code mapping in `cmd/` only:** The `cmd/` layer maps error types to exit codes (0-4). Internal packages return typed errors without knowledge of exit codes.
- **Error message format:** Every user-facing error includes **what failed**, **why**, and **what to do**: `"error: mount source './frontend' not found (resolved to /Users/manuel/projects/myapp/frontend). Check mounts in .asbox/config.yaml"`
- **Affects:** All packages (typed errors), `cmd/` layer (exit code mapping, message formatting)

### Container Lifecycle

**Entrypoint Startup Sequence:**
- **Decision:** tini → entrypoint.sh → (UID/GID align, chown volumes, write .mcp.json, start healthcheck poller, start Podman socket) → exec agent
- **Rationale:** Tini handles signal forwarding and zombie reaping. Entrypoint handles all runtime setup that can't be done at build time.
- **Startup steps:**
  1. Tini as PID 1 (signal forwarding, zombie reaping)
  2. entrypoint.sh: align UID/GID via `usermod`/`groupmod` using `HOST_UID`/`HOST_GID` env vars. **Conflict handling:** if target UID is already taken by another user in the image (common: UID 1000 = default Ubuntu user), delete the conflicting user first (`userdel`). If sandbox user already has the correct UID, skip modification entirely.
  3. entrypoint.sh: `chown` named volume mounts (auto_isolate_deps) for unprivileged user
  4. entrypoint.sh: generate `.mcp.json` from build-time manifest, merge with project .mcp.json if present
  5. entrypoint.sh: start healthcheck-poller.sh as background daemon (polls every 10s). **Fault tolerance:** poller runs in a trap-and-restart loop; PID tracked in a variable for cleanup on entrypoint exit.
  6. entrypoint.sh: start Podman API socket (`podman system service`), socket owned by sandbox user only
  7. entrypoint.sh: `exec` into configured agent command
- **Affects:** `embed/entrypoint.sh`, `embed/healthcheck-poller.sh`, image builder (tini installed at build time)

**MCP Integration:**
- **Decision:** Build-time manifest at `/etc/sandbox/mcp-servers.json`, entrypoint merges with project `.mcp.json`
- **Rationale:** Decouples build-time from run-time. Entrypoint doesn't need to re-parse config. Project config wins on name conflicts — user's explicit configuration takes precedence over sandbox defaults.
- **Merge behavior:** Sandbox MCP servers added to project's existing `.mcp.json`. If a server name already exists in project config, project version kept. Entrypoint logs which servers were added vs. skipped.
- **Affects:** Image builder (MCP packages installed, manifest written), `embed/entrypoint.sh` (merge logic)

### Content-Hash Caching

- **Decision:** SHA256 hash over the complete image input set, first 12 characters used in tag
- **Hash inputs:** rendered Dockerfile + all embedded scripts that get COPY'd into the image (entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh) + base image digest + config.yaml content. Changes to the Go CLI source, README, or LICENSE do NOT trigger rebuilds.
- **Rationale:** If any file that affects the container image changes, the hash changes and triggers a rebuild. Including the base image digest ensures that updating the pinned digest triggers a rebuild even if nothing else changed. Including embedded scripts ensures that changing git wrapper logic (for example) triggers a rebuild even though the Dockerfile template didn't change.
- **Image tagging:** Primary tag `asbox-<project>:<hash>`. Additionally tag `asbox-<project>:latest` pointing to the most recent build for convenience (`docker image ls` friendliness, quick re-runs without knowing the hash).
- **Cache bypass (`--no-cache`):** When `--no-cache` is passed to `asbox build` or `asbox run`, two things happen: (1) the content-hash image existence check is skipped — `docker build` runs unconditionally, and (2) `--no-cache` is forwarded to the `docker build` command, forcing Docker to rebuild all layers from scratch. The resulting image is still tagged with the content hash and `latest` — cache bypass affects how the image is built, not how it's tagged.
- **Affects:** `internal/hash/` (computation), `internal/docker/` (build + tag commands, `--no-cache` flag forwarding), `cmd/build.go` and `cmd/run.go` (flag definition and propagation)

### Automatic Dependency Isolation (`auto_isolate_deps`)

- **Decision:** Host-side scan via Go's `filepath.WalkDir` for `package.json` files, named Docker volume mounts assembled programmatically
- **Rationale:** macOS-compiled native modules in `node_modules/` crash inside the Linux sandbox. Named volumes provide isolated Linux-native `node_modules/` that persist across sessions.

**Detection Logic:**
- Triggered only when `auto_isolate_deps: true` in config
- For each primary mount: resolve host-side source path (relative to config file)
- For each `bmad_repos` entry: resolve host-side path and use its corresponding container target (`/workspace/repos/<basename>`)
- Walk all resolved directory trees for `package.json` files, excluding `node_modules/` subtrees
- For each discovered `package.json`: derive `node_modules` sibling path

**Volume Assembly:**
- Named volume pattern: `asbox-<project_name>-<relative-path-with-dashes>-node_modules`
- Volumes managed by Docker/Podman, persist across sessions
- Entrypoint `chown`s volume mounts for unprivileged sandbox user

**Logging:**
- Always log scan summary when enabled: `"auto_isolate_deps: scanned N mount paths, found M package.json files"`
- Each discovered mount logged: `"isolating: /workspace/node_modules (volume: asbox-myapp-node_modules)"`
- If scan finds zero `package.json` files, the summary line still prints so the user can distinguish "scan ran, found nothing" from "scan didn't run"

**Implementation:** `internal/mount/` package, called from `cmd/run.go` after config parse, before Docker run command assembly. `ScanDeps` accepts both primary mounts and `bmad_repos` entries as scan inputs. For bmad_repos, container paths are derived from the `/workspace/repos/<basename>` convention rather than from mount target config.
- **Affects:** `internal/mount/` (scan + volume flag generation), `embed/entrypoint.sh` (chown)

### BMAD Multi-Repo Workflow (`bmad_repos`)

- **Decision:** Each repo path mounted to `/workspace/repos/<basename>`, agent instruction file generated from Go template
- **Rationale:** Enables multi-repository development workflows. Convention-based target mapping keeps config simple (flat list of paths). Generated agent instructions ensure the agent knows where repos are and how to work with them.
- **Basename collision detection:** If two repo paths resolve to the same basename (e.g., `/Users/manuel/repos/client` and `/Users/manuel/work/client`), the CLI errors with exit code 1: `"error: bmad_repos basename collision — 'client' resolves from both /Users/manuel/repos/client and /Users/manuel/work/client. Rename one directory or use symlinks to disambiguate."` No automatic disambiguation — explicit failure forces the user to resolve the ambiguity.
- **Implementation:** `internal/mount/` package assembles mount flags with collision check. Go `text/template` generates agent instruction file (CLAUDE.md/GEMINI.md/CODEX.md) with repo list. Instruction file mounted into container at agent's expected config location.
- **Affects:** `internal/mount/` (mount assembly + collision detection), `embed/agent-instructions.md` (Go template), `cmd/run.go` (mount + instruction generation)

### Host Agent Config (`host_agent_config`)

- **Decision:** Boolean flag (default: true) that automatically mounts the host agent config directory read-write into the container, with paths resolved from the agent config registry based on the selected agent at runtime
- **Agent config registry:** Maps each agent to its default host config directory, container target, and environment variable:
  - `claude`: `~/.claude` -> `/opt/claude-config`, `CLAUDE_CONFIG_DIR`
  - `gemini`: `~/.gemini` -> `/opt/gemini-config`, `GEMINI_CONFIG_DIR`
  - `codex`: `~/.codex` -> `/opt/codex-config`, `CODEX_HOME`
- **Rationale:** Enables OAuth token synchronization — agent can read and refresh tokens without re-authentication. Automatic path resolution eliminates manual config when switching agents via `--agent`. This is the widest trust grant in the system but is clearly visible (enabled by default, explicitly disableable).
- **Silent skip:** If the host config directory does not exist (agent not set up on host), the mount is silently skipped — no error. This supports multi-agent images where not all agents are configured on the host.
- **Known limitation (Phase 2):** No integrity checking — an agent with write access could modify configuration that persists after the sandbox exits. Future enhancement: snapshot config directory state on startup for post-session diff.
- **Affects:** `internal/config/config.go` (AgentConfigRegistry), `internal/mount/mount.go` (AssembleHostAgentConfig), `cmd/run.go` (mount flag + env var assembly)

### Decision Impact Analysis

**Implementation Sequence:**
1. Go project scaffold with Cobra commands (init, build, run)
2. Config parser (`internal/config/`) — typed structs, validation including required-field checks, path resolution
3. Dockerfile template rendering (`internal/template/`) — assumes validated config
4. Docker build command assembly (`internal/docker/`) — build args, content-hash tagging (rendered Dockerfile + scripts + base digest + config), latest tag
5. Docker run command assembly — mounts, env, secrets, TTY, tini entrypoint
6. Container-side scripts — entrypoint.sh (UID/GID with conflict handling, volume chown, MCP merge, healthcheck poller with restart loop, Podman socket), git-wrapper.sh, healthcheck-poller.sh
7. MCP server installation + manifest generation + merge logic
8. auto_isolate_deps scanning + named volume assembly + scan summary logging
9. bmad_repos mount assembly with basename collision detection + agent instruction generation
10. Content-hash caching for smart rebuild detection
11. Integration tests via testcontainers-go

**Cross-Component Dependencies:**
- Config parser output drives both template rendering (build time) and flag assembly (run time) — single parse, consumed by multiple packages
- Config validation is a prerequisite for template rendering — zero-value required fields caught before rendering, not after
- Podman choice eliminates daemon startup from entrypoint but adds Podman API socket start
- MCP config generation depends on knowing which MCP servers were installed at build time — manifest at `/etc/sandbox/mcp-servers.json` bridges this
- UID/GID alignment must happen before any file operations in entrypoint (volume chown, .mcp.json write) and must handle UID 1000 conflict
- Error types flow from internal packages → cmd layer maps to exit codes → user sees formatted messages with fix actions

## Implementation Patterns & Consistency Rules

### Purpose

These patterns ensure that AI agents implementing different parts of asbox produce consistent, compatible code. Without these rules, agents could make different naming, formatting, and structural choices that create a fragmented codebase.

### Go Code Conventions

**Package naming:** lowercase, single word where possible — `config`, `template`, `docker`, `hash`, `mount`

**Function/method naming:** `PascalCase` for exported, `camelCase` for unexported. Verb-first for actions: `ParseConfig`, `RenderTemplate`, `BuildImage`, `RunContainer`

**Struct naming:** `PascalCase`, noun-based. Config structs mirror YAML structure: `Config`, `SDKConfig`, `MountConfig`, `AgentConfigMapping`

**Variable naming:** `camelCase` — `configPath`, `imageName`, `mountFlags`. No abbreviations except universally understood ones (`ctx`, `err`, `cmd`).

**Interface naming:** Single-method interfaces use `-er` suffix: `Runner`, `Builder`. Avoid premature interface extraction — define interfaces where consumed, not where implemented.

**Error naming:** Typed errors per package: `type ConfigError struct{ ... }`. Error messages lowercase, no trailing punctuation: `return fmt.Errorf("mount source %q not found", path)`

**File naming:** `snake_case.go` — `config.go`, `config_test.go`, `parse_config.go`. One primary type per file where practical.

**Test naming:** `TestFunctionName_scenario` — `TestParseConfig_missingSDK`, `TestBuildImage_cacheHit`. Table-driven tests preferred for multiple scenarios.

**Formatting:** `gofmt` is law. `go vet` must pass. No linter exceptions without comment.

### Bash Script Conventions (Container-Side)

**Function naming:** `snake_case` — `align_uid_gid`, `merge_mcp_config`, `start_healthcheck_poller`

**Variable naming:**
- Local variables: `lower_snake_case` — `config_file`, `host_uid`
- Constants/environment: `UPPER_SNAKE_CASE` — `HOST_UID`, `DOCKER_HOST`, `MCP_MANIFEST_PATH`

**Quoting:** Always double-quote variable expansions: `"${var}"` not `$var`. Exception: inside `[[ ]]` tests.

**Error handling:**
- `set -euo pipefail` at script top
- Functions return exit codes, not echo to stdout for status
- `die()` helper for fatal errors: prints to stderr, exits non-zero

**Script headers:** Every script starts with shebang, `set -euo pipefail`, and a one-line comment describing purpose:
```bash
#!/usr/bin/env bash
set -euo pipefail
# Entrypoint: aligns UID/GID, sets up runtime env, execs agent
```

### Output & Error Formatting

**Go CLI output:**
- **Info:** plain text to stdout — `fmt.Println("Building sandbox image...")`
- **Errors:** `fmt.Fprintf(os.Stderr, "error: %s\n", msg)` — always includes what failed, why, and fix action
- **Warnings:** `fmt.Fprintf(os.Stderr, "warning: %s\n", msg)`
- **Success:** no prefix — `fmt.Printf("Sandbox image built: %s\n", imageTag)`
- No color codes — clean output for piping and logging

**Bash script output:** Same format as Go CLI — `echo "error: <message>" >&2` for errors, plain stdout for info. Consistent user experience regardless of which layer produces the message.

### Exit Codes

| Code | Meaning | Go Error Type |
|------|---------|---------------|
| 0 | Success | nil |
| 1 | Configuration or general error | `ConfigError`, `TemplateError` |
| 2 | Usage error (invalid command or flags) | Cobra handles automatically |
| 3 | Missing dependency (Docker not found) | `DependencyError` |
| 4 | Secret validation error | `SecretError` |

### Go Template Conventions (Dockerfile.tmpl, agent-instructions.md)

- **Conditional blocks:** `{{if .Field}}` ... `{{end}}` — standard Go template syntax
- **Range blocks:** `{{range .Packages}}` ... `{{end}}` for lists
- **Value injection:** `{{.SDKs.NodeJS}}` — dot-notation into typed config structs
- **Whitespace control:** Use `{{-` and `-}}` trim markers to prevent blank lines in rendered Dockerfile
- **Comments:** `{{/* comment */}}` for template-level documentation
- **No raw string literals in templates** — all values come from config structs

### Config YAML Conventions

- Keys are `lower_snake_case` — `auto_isolate_deps`, `host_agent_config`, `bmad_repos`
- List items are simple strings where possible: `- playwright`, `- build-essential`
- Mount entries use `source`/`target` keys (matching Docker convention)
- No nesting beyond two levels deep
- Config structs in Go use `yaml:"field_name"` tags matching YAML keys exactly

### Go Project Organization

**Package responsibilities:**
- `cmd/` — Cobra commands, flag parsing, exit code mapping. No business logic.
- `internal/config/` — YAML parsing, typed structs, validation (required fields, path resolution), agent config registry. Single `Parse()` function returns validated `Config` struct. Exported `ValidateAgent()` and `ValidateAgentInstalled()` for CLI flag validation.
- `internal/template/` — Dockerfile rendering from validated config. Single `Render()` function. Assumes valid input.
- `internal/docker/` — Docker/Podman CLI interaction via `os/exec`. Build and run command assembly.
- `internal/hash/` — Content-hash computation. Pure function: inputs in, hash out.
- `internal/mount/` — Mount flag assembly, host agent config resolution from registry, auto_isolate_deps scanning, bmad_repos mapping with collision detection.
- `embed/` — Single `embed.go` file exporting the embedded filesystem. All `//go:embed` directives in one place.

**Dependency direction:** `cmd/` → `internal/*` → standard library. Internal packages do NOT import each other except through interfaces. `config.Config` struct is passed by value to consuming packages.

**No `internal/utils/` or `internal/helpers/`** — utility functions live in the package that uses them. If shared, they belong in the most relevant package.

### Enforcement Guidelines

**All AI Agents MUST:**
- Run `gofmt` and `go vet` — code that doesn't pass is rejected
- Use `"${var}"` quoting in all bash scripts — no unquoted expansions
- Write errors to stderr, not stdout
- Use the exit code table above — no arbitrary exit codes
- Follow the template syntax exactly — no inventing new placeholder conventions
- Define error types in their owning package — no centralized error package
- Keep `cmd/` thin — business logic belongs in `internal/`

### Anti-Patterns

- `fmt.Println("Error: ...")` — errors go to stderr via `fmt.Fprintf(os.Stderr, ...)`
- `os.Exit(1)` inside `internal/` packages — return typed errors, let `cmd/` handle exit codes
- `echo $var` in bash — must be `echo "${var}"`
- `interface{}` or `any` as function parameters — use typed config structs
- Scattering `//go:embed` across multiple files — centralize in `embed/embed.go`
- Creating `internal/utils/` or `internal/common/` packages — put functions where they're used
- Adding color codes, spinners, or progress bars to output
- Creating helper scripts outside the defined project structure

## Project Structure & Boundaries

### Complete Project Directory Structure

```
asbox/
├── main.go                          # Entry point — Cobra root command execute
├── go.mod
├── go.sum
├── .goreleaser.yaml                 # Cross-platform release configuration
├── .gitignore
├── README.md
├── LICENSE
├── cmd/                             # Cobra command definitions — thin, no business logic
│   ├── root.go                      # Root command, -f flag, version, error-to-exit-code mapping
│   ├── init.go                      # asbox init — writes starter config
│   ├── build.go                     # asbox build — config parse → template render → docker build
│   └── run.go                       # asbox run — config parse → agent override → build-if-needed → flag assembly → docker run
├── internal/                        # Private application logic
│   ├── config/                      # Configuration parsing and validation
│   │   ├── config.go                # Config struct definitions, YAML tags
│   │   ├── parse.go                 # Parse() — reads YAML, validates required fields, resolves paths
│   │   └── parse_test.go            # Table-driven tests for parsing and validation
│   ├── template/                    # Dockerfile template rendering
│   │   ├── render.go                # Render() — executes Go template with validated config
│   │   └── render_test.go           # Tests for conditional blocks, value injection, whitespace
│   ├── docker/                      # Docker/Podman CLI interaction
│   │   ├── build.go                 # BuildImage() — assembles and executes docker build
│   │   ├── run.go                   # RunContainer() — assembles and executes docker run
│   │   ├── build_test.go
│   │   └── run_test.go
│   ├── hash/                        # Content-hash computation
│   │   ├── hash.go                  # Compute() — SHA256 over rendered Dockerfile + scripts + digest + config
│   │   └── hash_test.go
│   └── mount/                       # Mount assembly and dependency isolation
│       ├── mount.go                 # AssembleMounts() — regular mounts; AssembleHostAgentConfig() — agent-aware config mount
│       ├── isolate_deps.go          # ScanDeps() — filepath.WalkDir for package.json, volume flags
│       ├── bmad_repos.go            # AssembleBmadRepos() — repo mounts, collision detection, instruction gen
│       ├── mount_test.go
│       ├── isolate_deps_test.go
│       └── bmad_repos_test.go
├── embed/                           # Embedded asset source files
│   ├── embed.go                     # //go:embed directives, exports embedded FS
│   ├── Dockerfile.tmpl              # Go text/template Dockerfile
│   ├── entrypoint.sh               # Container entrypoint — UID/GID, MCP merge, poller, Podman socket, exec
│   ├── git-wrapper.sh              # Git push interceptor — blocks push, passes all else to /usr/bin/git
│   ├── healthcheck-poller.sh       # Healthcheck daemon — polls every 10s, trap-and-restart loop
│   ├── agent-instructions.md.tmpl  # Go template for CLAUDE.md/GEMINI.md/CODEX.md with bmad_repos list
│   └── config.yaml                 # Starter config for asbox init — sensible defaults, inline comments
├── integration/                     # Integration tests (testcontainers-go)
│   ├── integration_test.go          # Test setup, shared helpers, testcontainers config
│   ├── lifecycle_test.go            # Build, run, Ctrl+C, auto-rebuild tests
│   ├── mount_test.go                # Mount verification, auto_isolate_deps, bmad_repos
│   ├── isolation_test.go            # Git push blocked, no host creds, no privileged containers
│   ├── secret_test.go               # Secret injection, fail-closed validation
│   ├── mcp_test.go                  # MCP manifest merge, server availability
│   └── inner_container_test.go      # Podman inside sandbox, docker compose, port reachability
└── .github/
    └── workflows/
        └── ci.yml                   # Build, vet, test, integration test, goreleaser
```

### File Responsibilities

**main.go** — Single line: `cmd.Execute()`. Nothing else.

**cmd/root.go** — Root Cobra command, `-f` flag definition, `--version` flag, error-to-exit-code mapping function that switches on error types (`ConfigError` → 1, `DependencyError` → 3, `SecretError` → 4).

**cmd/init.go** — Reads starter config from embedded FS, writes to `.asbox/config.yaml`. Creates `.asbox/` directory if needed.

**cmd/build.go** — Orchestrates: `config.Parse()` → `template.Render()` → `hash.Compute()` → check if image exists → `docker.BuildImage()`. Applies both `asbox-<project>:<hash>` and `asbox-<project>:latest` tags.

**cmd/run.go** — Orchestrates: `config.Parse()` → build-if-needed (calls build logic) → `mount.AssembleMounts()` → `mount.ScanDeps()` (if enabled) → `mount.AssembleBmadRepos()` (if configured) → secret validation via `os.LookupEnv()` → `docker.RunContainer()`.

**internal/config/config.go** — Typed structs:
```go
type Config struct {
    Agent           string            `yaml:"agent"`
    ProjectName     string            `yaml:"project_name"`
    SDKs            SDKConfig         `yaml:"sdks"`
    Packages        []string          `yaml:"packages"`
    MCP             []string          `yaml:"mcp"`
    Mounts          []MountConfig     `yaml:"mounts"`
    Secrets         []string          `yaml:"secrets"`
    Env             map[string]string `yaml:"env"`
    AutoIsolateDeps bool              `yaml:"auto_isolate_deps"`
    HostAgentConfig *MountConfig      `yaml:"host_agent_config"`
    BmadRepos       []string          `yaml:"bmad_repos"`
}
```

**embed/embed.go** — Single file with all embed directives:
```go
//go:embed Dockerfile.tmpl entrypoint.sh git-wrapper.sh healthcheck-poller.sh agent-instructions.md.tmpl config.yaml
var Assets embed.FS
```

**embed/entrypoint.sh** — Runs inside container at startup. Ordered steps: UID/GID alignment (with UID 1000 conflict handling), volume chown, MCP manifest merge, healthcheck poller start, Podman API socket start, exec agent.

**embed/git-wrapper.sh** — Checks first argument for `push` — returns exit code 1 with "fatal: Authentication failed" message. All other arguments pass through to `/usr/bin/git` unchanged.

**embed/healthcheck-poller.sh** — Polls `podman healthcheck run` every 10s for all containers with healthchecks. Runs in trap-and-restart loop for fault tolerance.

### Architectural Boundaries

**Host vs Container Boundary:**
- `main.go`, `cmd/`, `internal/`, `embed/embed.go` — run on the host as the Go binary
- Everything in `embed/` (except embed.go) — gets COPY'd into the container image, runs inside the container
- Mounts are the only bridge: declared paths flow host → container
- Secrets bridge host → container via `--env` flags at launch
- Host agent config (if configured) bridges host ↔ container via read-write mount

**Build-Time vs Run-Time Boundary:**
- Build time: `config.Parse()` → `template.Render()` → `docker build` (produces image with scripts, Podman, SDKs, tools)
- Run time: `config.Parse()` → `mount assembly` → `secret validation` → `docker run` (mounts, env, TTY)
- Config is parsed at build time (for template) and run time (for docker run flags) — same `Parse()` function, invoked twice
- Build-time metadata (MCP manifest at `/etc/sandbox/mcp-servers.json`) bridges build → run inside the container

**Agent vs Sandbox Boundary:**
- The agent sees: a normal Linux environment with tools, a workspace, and a `.mcp.json`
- The agent does NOT see: that git push is intercepted, that Docker is actually Podman, that it's in a container
- Standard error codes at every boundary — no asbox-specific errors visible to the agent

### Data Flow

```
.asbox/config.yaml
       │
       ▼
  config.Parse()  ──────────────────────────────────┐
       │                                             │
       ▼                                             ▼
  template.Render()                           mount.AssembleMounts()
       │                                      mount.ScanDeps()
       ▼                                      mount.AssembleBmadRepos()
  hash.Compute()                                     │
       │                                             │
       ▼                                             ▼
  docker.BuildImage()                         docker.RunContainer()
       │                                             │
       ▼                                             ▼
  asbox-<project>:<hash>                      container starts
  asbox-<project>:latest                             │
                                                     ▼
                                              tini → entrypoint.sh
                                                     │
                                              ┌──────┼──────┐
                                              │      │      │
                                           UID/GID  MCP   poller
                                           align   merge  start
                                              │      │      │
                                              └──────┼──────┘
                                                     │
                                                     ▼
                                              exec agent (claude/gemini/codex)
```

### Requirements to Structure Mapping

| Requirement | File | Function/Section |
|---|---|---|
| FR1-FR7 (config options) | `embed/config.yaml`, `internal/config/` | `Config` struct, `Parse()` |
| FR8 (`-f` flag) | `cmd/root.go` | Cobra persistent flag |
| FR9 (`asbox init`) | `cmd/init.go`, `embed/config.yaml` | Reads embedded starter config |
| FR9a-FR9c (auto_isolate_deps) | `internal/mount/isolate_deps.go` | `ScanDeps()` |
| FR9d (host_agent_config) | `internal/mount/mount.go`, `cmd/run.go` | Mount + env var assembly |
| FR9e (project_name) | `internal/config/config.go` | `Config.ProjectName` with default derivation |
| FR10 (`asbox build`) | `cmd/build.go` | Orchestrates parse → render → hash → build |
| FR11 (`asbox run`) | `cmd/run.go` | Orchestrates parse → build-if-needed → run |
| FR12 (auto-build) | `cmd/run.go` | Checks image exists before run |
| FR13 (change detection) | `internal/hash/` | Content-hash comparison |
| FR14 (Ctrl+C stop) | `embed/entrypoint.sh` | Tini signal forwarding |
| FR15 (dependency check) | `cmd/root.go` | Docker presence validation |
| FR16 (secret validation) | `cmd/run.go` | `os.LookupEnv()` check |
| FR16a (auto_isolate_deps volumes) | `internal/mount/isolate_deps.go`, `embed/entrypoint.sh` | Volume flags + chown |
| FR17-FR19 (agent runtime) | `embed/entrypoint.sh` | `exec` agent command |
| FR20-FR22 (project files, BMAD) | `cmd/run.go` | Mount flag assembly |
| FR23-FR25 (git, internet, CLI tools) | `embed/Dockerfile.tmpl` | Tool installation |
| FR26-FR28 (Docker/Compose) | `embed/Dockerfile.tmpl` | Podman installation + alias |
| FR29-FR30 (MCP/Playwright) | `embed/Dockerfile.tmpl`, `embed/entrypoint.sh` | MCP install + .mcp.json merge |
| FR31 (git push block) | `embed/git-wrapper.sh` | Push interception |
| FR32-FR33 (filesystem/creds) | `cmd/run.go` | Mount restriction (only declared paths) |
| FR34-FR36 (network isolation) | Podman rootless networking | No explicit config |
| FR37 (standard errors) | `embed/git-wrapper.sh` | Standard git error codes |
| FR38 (Dockerfile generation) | `internal/template/`, `embed/Dockerfile.tmpl` | `Render()` |
| FR39 (pinned base images) | `embed/Dockerfile.tmpl` | Digest in FROM |
| FR40 (SDK build args) | `internal/docker/build.go` | `--build-arg` flags |
| FR41 (MCP install) | `embed/Dockerfile.tmpl` | MCP package installation |
| FR42 (isolation in image) | `embed/Dockerfile.tmpl` | Git wrapper + boundaries baked in |
| FR43 (content-hash tags) | `internal/hash/`, `internal/docker/build.go` | `Compute()` + tag flags |
| FR44 (agent instruction files) | `embed/Dockerfile.tmpl`, `embed/agent-instructions.md.tmpl` | COPY into image |
| FR45 (host_agent_config mount) | `cmd/run.go` | Mount flag + `CLAUDE_CONFIG_DIR` env |
| FR46 (MCP manifest merge) | `embed/entrypoint.sh` | Merge logic |
| FR47 (exit codes) | `cmd/root.go` | Error type → exit code mapping |
| FR48 (Tini as PID 1) | `embed/Dockerfile.tmpl`, `embed/entrypoint.sh` | Tini installed + entrypoint |
| FR49 (UID/GID alignment) | `embed/entrypoint.sh` | `usermod`/`groupmod` with conflict handling |
| FR50 (embedded assets) | `embed/embed.go` | `//go:embed` directives |
| FR51-FR53 (bmad_repos) | `internal/mount/bmad_repos.go`, `embed/agent-instructions.md.tmpl` | Mount assembly + instruction gen |
| FR54 (single binary) | `go.mod`, `.goreleaser.yaml` | `CGO_ENABLED=0` static linking |
| NFR15 (integration tests) | `integration/` | testcontainers-go test suite |

## Architecture Validation Results

### Coherence Validation

**Decision Compatibility:** All technology choices (Go, Cobra, yaml.v3, text/template, embed, Podman 5.x, Ubuntu 24.04 LTS, tini, testcontainers-go) are independently maintained, well-established tools with no version conflicts or incompatibilities.

**Pattern Consistency:** Go conventions (PascalCase/camelCase, snake_case files) and bash conventions (snake_case functions, UPPER_SNAKE_CASE env) are standard for their respective domains. Output formatting (plain text, errors to stderr, no color) is consistent across both host and container layers. Exit codes defined once, used everywhere.

**Structure Alignment:** `cmd/` → `internal/*` dependency direction is clean. Each internal package has single responsibility. Embedded assets centralized. Config struct is shared data contract. No circular dependencies possible.

### Requirements Coverage

**Functional Requirements:** All 54 FRs (FR1-FR54) have explicit architectural support mapped to specific files and functions in the requirements-to-structure mapping table.

**Non-Functional Requirements:** All 15 NFRs (NFR1-NFR15) are architecturally addressed through Podman rootless (security), Go static binary (portability), typed errors with fix messages (reliability), tini (cleanup), digest pinning (reproducibility), and testcontainers-go integration test suite (test coverage).

### Gap Analysis & Resolutions

**Gap 1: Agent instruction file dual use (FR44 vs FR53)**
- FR44: Static agent instruction file (`CLAUDE.md`/`GEMINI.md`/`CODEX.md`) baked into image at build time with sandbox-specific constraints
- FR53: Dynamically generated agent instruction file mounted at runtime when `bmad_repos` is configured, containing repo list and multi-repo workflow instructions
- Resolution: When bmad_repos is active, the runtime-generated file takes precedence (mounted over the build-time file at the same path). When bmad_repos is not configured, the build-time file is used as-is. Agents should understand these are two separate mechanisms.

**Gaps 2-12: Resolved during Expert Panel Review (step 4)**
All findings from the expert panel have been incorporated into the architectural decisions:
- Config validation before template rendering (zero-value field check)
- Content-hash scope expanded (rendered Dockerfile + scripts + base digest + config)
- UID/GID conflict handling (UID 1000 collision, userdel before usermod)
- bmad_repos basename collision detection (error with exit code 1)
- Error types per-package, exit code mapping in cmd/ only
- Healthcheck poller fault tolerance (trap-and-restart loop)
- auto_isolate_deps scan summary logging (always logs when enabled)
- Error message format standardized (what + why + fix)
- vfs storage driver performance documented as known trade-off
- host_agent_config integrity checking noted for Phase 2
- Podman API socket restricted to sandbox user
- asbox-\<project\>:latest tag alongside content-hash tag

### Architecture Completeness Checklist

**Requirements Analysis**
- [x] Project context thoroughly analyzed (54 FRs, 15 NFRs categorized)
- [x] Scale and complexity assessed (medium — complexity in inner Podman, not CLI)
- [x] Technical constraints identified (Go binary, no external deps, no --privileged, no host socket)
- [x] Cross-cutting concerns mapped (two execution domains, embedded assets, config as single source, distributed isolation)
- [x] Threat model boundaries documented (accidental not adversarial)

**Architectural Decisions**
- [x] Inner container runtime decided (Podman 5.x with vfs, netavark, aardvark-dns)
- [x] Dockerfile generation decided (Go text/template with config validation before rendering)
- [x] Isolation mechanisms decided (git wrapper, Podman rootless networking, fail-closed secrets)
- [x] Container lifecycle decided (tini + entrypoint with UID/GID conflict handling + Podman socket + healthcheck poller)
- [x] MCP integration decided (.mcp.json from build-time manifest, project config wins on conflicts)
- [x] Content-hash caching decided (rendered Dockerfile + scripts + base digest + config, 12-char SHA256, plus latest tag)
- [x] auto_isolate_deps decided (filepath.WalkDir + named volumes + scan summary logging)
- [x] bmad_repos decided (convention-based mounts + collision detection + generated agent instructions)
- [x] host_agent_config decided (read-write mount + CLAUDE_CONFIG_DIR, integrity checking deferred)
- [x] Error handling decided (typed errors per package, exit code mapping in cmd/, formatted messages with fix actions)

**Implementation Patterns**
- [x] Go code conventions established (naming, file structure, testing, formatting)
- [x] Bash script conventions established (naming, quoting, error handling, headers)
- [x] Output formatting standardized (info/error/warning, stderr, no color)
- [x] Exit codes defined (0-4 with Go error type mapping)
- [x] Go template conventions specified (conditionals, ranges, whitespace control)
- [x] Config YAML conventions specified (snake_case keys, mount format, nesting limit)
- [x] Project organization rules defined (package responsibilities, dependency direction)
- [x] Anti-patterns documented

**Project Structure**
- [x] Complete directory structure defined (all files, all directories)
- [x] File responsibilities documented with function signatures
- [x] Architectural boundaries established (host/container, build/run, agent/sandbox)
- [x] Data flow mapped
- [x] All 54 FRs + NFR15 mapped to specific files and functions
- [x] Config struct defined with YAML tags

### Architecture Readiness Assessment

**Overall Status:** READY FOR IMPLEMENTATION

**Confidence Level:** High — all requirements covered, 12 expert panel findings addressed, no critical gaps, decisions are coherent and well-constrained.

**Key Strengths:**
- Clean separation between host (Go) and container (bash) execution domains
- Single binary distribution with zero external dependencies on host
- Every FR mapped to a specific file and function
- Expert panel review hardened the design (UID conflict, collision detection, cache scope, error formatting)
- Typed Go structs as shared contract between packages prevents drift
- Config validation before template rendering catches issues early

**Areas for Future Enhancement:**
- Egress filtering for outbound network (Phase 2+)
- Agent-specific MCP configuration for Gemini CLI
- Audit trail / activity logging
- Session persistence options
- host_agent_config integrity checking (post-session diff)
- Automated testing with bats-core for container-side bash scripts

### Implementation Handoff

**AI Agent Guidelines:**
- Follow all architectural decisions exactly as documented
- Use implementation patterns consistently — especially Go naming, bash quoting, and error handling
- Respect the project structure — cmd/ is thin, business logic in internal/, embeds centralized
- Config validation happens in internal/config/, not in template rendering
- Error types defined per-package, exit code mapping only in cmd/root.go
- Refer to this document for all architectural questions

**First Implementation Priority:**
1. `go mod init` + Cobra scaffold (`main.go`, `cmd/root.go`, `cmd/init.go`, `cmd/build.go`, `cmd/run.go`)
2. `internal/config/` — Config struct with YAML tags, `Parse()` with validation and path resolution
3. `embed/` — All embedded assets with `embed.go` exporting the FS
4. `internal/template/` — `Render()` consuming validated Config struct
5. `internal/hash/` — `Compute()` over rendered Dockerfile + scripts + digest + config
6. `internal/docker/` — `BuildImage()` and `RunContainer()` via os/exec
7. `internal/mount/` — `AssembleMounts()`, `ScanDeps()`, `AssembleBmadRepos()`
8. Container-side scripts — entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh
9. Integration tests via testcontainers-go
10. `.goreleaser.yaml` for cross-platform release builds
