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
FR7: Developer can select which AI agent runtime to use (claude-code, gemini-cli)
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
FR44: System installs agent environment instruction files (CLAUDE.md, GEMINI.md) into the sandbox user's home directory at image build time
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
| FR7 | Epic 1 | Agent runtime selection |
| FR8 | Epic 1 | Config file path override (`-f`) |
| FR9 | Epic 1 | `asbox init` starter config |
| FR9a | Epic 6 | auto_isolate_deps config option |
| FR9b | Epic 6 | Scan and create volumes for node_modules |
| FR9c | Epic 6 | Log isolated mounts |
| FR9d | Epic 7 | host_agent_config config option |
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
| FR45 | Epic 7 | host_agent_config mount + CLAUDE_CONFIG_DIR |
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

## Epic List

### Epic 1: Developer Can Build and Launch a Sandbox
A developer can install the `asbox` binary, create a configuration file, build a container image, and launch an interactive sandbox session with a working agent inside.
**FRs covered:** FR1, FR2, FR6, FR7, FR8, FR9, FR9e, FR10, FR11, FR12, FR13, FR14, FR15, FR38, FR39, FR40, FR43, FR44, FR47, FR48, FR49, FR50, FR54

### Epic 2: Agent Can Work With Project Files and Git
An agent inside the sandbox can access mounted project files, use local git, install packages, and work with CLI tools and internet — a complete development workflow minus Docker and MCP.
**FRs covered:** FR4, FR5, FR16, FR17, FR18, FR19, FR20, FR21, FR22, FR23, FR24, FR25

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
A developer can mount their host agent config directory so the sandbox agent picks up existing OAuth tokens without re-authentication each session.
**FRs covered:** FR9d, FR45

### Epic 8: Developer Can Run Multi-Repo BMAD Workflows
A developer can configure multiple repositories that get auto-mounted into the sandbox with generated agent instructions, enabling multi-repo development workflows.
**FRs covered:** FR51, FR52, FR53

### Epic 9: Integration Test Suite Validates All Use Cases
Comprehensive integration tests verify sandbox lifecycle, mounts, secrets, isolation, inner containers, MCP, auto_isolate_deps, and bmad_repos with parallel Go test execution.
**FRs covered:** NFR15

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
- `embed/config.yaml` — starter template with inline comments, default agent claude-code, example SDK, example mount

## Epic 2: Agent Can Work With Project Files and Git

An agent inside the sandbox can access mounted project files, use local git, install packages, and work with CLI tools and internet — a complete development workflow minus Docker and MCP.

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
I want to mount my host agent config directory into the sandbox,
So that the agent picks up my existing authentication without re-login each session.

**Acceptance Criteria:**

**Given** a config with `host_agent_config: {source: "~/.claude", target: "/opt/claude-config"}`
**When** the sandbox launches
**Then** `~/.claude` is mounted read-write at `/opt/claude-config` and `CLAUDE_CONFIG_DIR=/opt/claude-config` is set

**Given** the host `~/.claude` contains valid OAuth tokens
**When** the agent refreshes tokens during a session
**Then** updated tokens are written back to the host directory via the read-write mount

**Given** `host_agent_config` is not configured
**When** the sandbox launches
**Then** no additional mount or env var is added

**Implementation Notes:**
- `cmd/run.go` — mount flag + env var assembly for `host_agent_config`
- `~` expansion via `os.UserHomeDir()`
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
**Then** a CLAUDE.md (or GEMINI.md) is generated from Go template with the repo list and instructions for git operations within `repos/`, mounted into the container

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
- `integration/bmad_repos_test.go` — mount verification, instruction file presence
