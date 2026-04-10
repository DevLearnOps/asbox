---
stepsCompleted:
  - step-01-init
  - step-02-discovery
  - step-02b-vision
  - step-02c-executive-summary
  - step-03-success
  - step-04-journeys
  - step-05-domain
  - step-06-innovation
  - step-07-project-type
  - step-08-scoping
  - step-09-functional
  - step-10-nonfunctional
  - step-11-polish
  - step-12-complete
  - step-e-01-discovery
  - step-e-02-review
  - step-e-03-edit
inputDocuments: []
workflowType: 'prd'
lastEdited: '2026-04-10'
editHistory:
  - date: '2026-04-10'
    changes: 'Course correction: added --no-cache flag for build command. New FR55. Per sprint-change-proposal-2026-04-10-no-cache.md.'
  - date: '2026-04-10'
    changes: 'Course correction: auto_isolate_deps now scans bmad_repos paths in addition to primary mounts. Modified FR9b, Additional Requirements auto_isolate_deps bullet, Runtime behavior auto_isolate_deps paragraph. Per sprint-change-proposal-2026-04-10.md.'
  - date: '2026-04-06'
    changes: 'Major course correction: rebrand sandbox → asbox (Agent-SandBox), rewrite from bash to Go CLI, single binary distribution with embedded assets, add bmad_repos multi-repo workflow support, add integration test coverage requirements. New FRs: FR50-FR54. Modified FRs: FR9, FR15, FR38, FR43, FR47. Modified NFRs: NFR8, NFR11. New NFR: NFR15. Per sprint-change-proposal-2026-04-06.md.'
  - date: '2026-04-06'
    changes: 'Comprehensive alignment with implementation: named Docker volumes (not anonymous) for auto_isolate_deps, Podman chosen as inner container runtime (no longer deferred), added host_agent_config and project_name config options, documented Tini init process, UID/GID alignment, healthcheck poller, Testcontainers compatibility, MCP manifest merge behavior, specific exit codes, content hash file list, Claude CLI official install script. Fixed implementation leakage in FR31/FR42/FR16a and NFR13 subjectivity per validation report.'
  - date: '2026-03-31'
    changes: 'Added webkit browser support for mobile device emulation, agent environment instruction files (CLAUDE.md/GEMINI.md) baked into sandbox image'
  - date: '2026-03-26'
    changes: 'Added auto_isolate_deps feature for named Docker volume mounts to prevent macOS/Linux dependency clashes'
documentCounts:
  briefs: 0
  research: 0
  brainstorming: 0
  projectDocs: 0
classification:
  projectType: developer_tool
  domain: general
  complexity: medium
  projectContext: greenfield
---

# Product Requirements Document - asbox

**Author:** Manuel
**Date:** 2026-03-23

## Executive Summary

asbox (Agent-SandBox) is a containerized development environment that enables AI coding agents (Claude Code, Gemini CLI) to operate with full development capability and no implicit access to host resources. It solves a specific bottleneck in AI-assisted development: the human overhead of supervising agents that have broad system access. By constraining the environment rather than the agent, asbox lets developers fire off tasks and walk away -- no permission prompts, no fear of broken host systems, no leaked credentials.

The target users are development teams building AI-native applications who need to delegate complex, multi-step tasks to coding agents without constant oversight. asbox provides agents with everything they need -- project files, local git, internet access, Docker, CLI tools, configurable SDKs, and MCP integrations for browser automation -- while enforcing hard boundaries: no host credentials or SSH keys by default, no remote git pushes, and no host filesystem access beyond explicitly mounted project paths. Developers may opt in to providing scoped secrets (e.g., private registry tokens) when a task requires them.

Sandbox sessions are configurable -- ephemeral for throwaway experimentation or persistent for multi-session work where git history and build artifacts carry forward. Distributed as a single Go binary, developers configure their instances with the specific SDK versions, tools, and MCP servers their projects require.

### What Makes This Special

The insight behind asbox is that agent capability is not the bottleneck -- supervision cost is. Current approaches force a choice: either restrict the agent (losing productivity) or trust it with full access (risking damage). asbox eliminates this tradeoff. Agents get a complete, realistic development environment with internet, Docker, and real toolchains. Developers get a guarantee that nothing outside the container is affected unless they explicitly allow it.

The deliberate tension in the design is Docker access: it's the widest capability surface and the point where containment must be most carefully enforced. This is a known trade-off -- agents need Docker to build images and test services, and the sandbox must ensure this access cannot be used to escape the container boundary.

## Project Classification

- **Project Type:** Developer Tool -- single Go binary, containerized environment manager
- **Domain:** Software Development Tooling
- **Complexity:** Medium -- container orchestration, security boundaries, multi-SDK configuration, MCP integration
- **Project Context:** Greenfield

## Success Criteria

### User Success

- A developer can assign a complex, multi-step task to an AI agent (e.g., "implement this story, write E2E tests, build the Docker image, and verify the app starts") and disconnect entirely. The agent completes the work unsupervised.
- On return, the developer finds working code, passing tests, and a chat history that explains what was done. The final state of the project is self-documenting.
- Auditability of agent actions is a nice-to-have, not a requirement -- but the system should not make it impossible to add later.

### Business Success

- Increase in the number of autonomous tasks a developer can delegate per day -- from near-zero (constant supervision) to multiple concurrent fire-and-forget sessions.
- Reduction in agent-related interruptions: no permission prompts, no approval gates, no "are you sure?" friction during agent execution.
- Faster iteration cycles: the team ships more because the human is unblocked while the agent works.

### Technical Success

- The sandbox provides a complete development environment: project files, local git, internet, Docker, Docker Compose, CLI tools, configurable SDKs, MCP servers, and browser automation.
- Hard isolation boundaries are enforced: no host credentials or SSH keys by default, no remote git pushes, no host filesystem access beyond mounted project paths, no host browser access, no containers with public network exposure.
- The balance between isolation and autonomy is the primary design metric. Docker access inside the sandbox is explicitly accepted as a trade-off, but the agent must not be able to escape the container boundary or affect resources outside it.
- Configuration-driven: different sandbox instances can be built with different SDK versions and tool configurations from build arguments.

### Measurable Outcomes

- An agent can execute a full BMAD workflow (planning through implementation) inside the sandbox without hitting a permissions wall or missing tool.
- An agent can build a Dockerfile, run `docker compose up`, and reach application ports from Playwright tests -- all within the sandbox.
- A developer can configure and build a new sandbox variant (different SDK versions) without modifying source code -- configuration only.
- Zero host-side side effects from any agent session: no leaked credentials, no modified host files, no unexpected network exposure.

## User Journeys

### Journey 1: Manuel Delegates a Feature Build (Developer - Happy Path)

Manuel is a developer building an AI-native application. He has a story spec ready from a BMAD planning session and wants the agent to handle the full implementation. Today, he'd run Claude Code on his host machine, babysitting it through permission prompts and worrying about it touching things it shouldn't.

With asbox, Manuel opens his terminal and launches a sandbox instance for his project, mounting the project directory and passing his Anthropic API key as a scoped secret. The sandbox spins up with Node.js 22, Playwright, and Docker pre-configured. He starts a Claude Code session inside, gives it the story spec, and tells it: "Implement this story, write E2E tests, build the Docker image, run docker compose up, and verify the app with Playwright."

Manuel closes his laptop and goes to lunch. The agent works through the BMAD workflow -- reading the story spec, implementing the code, writing Playwright E2E tests, building the Dockerfile, running `docker compose up`, hitting the app's ports from Playwright, and iterating until tests pass. It commits each logical change to local git.

Manuel comes back, opens the project directory, and reviews the git log. The code is there, tests pass, the Docker image builds. He reads the Claude Code chat history to understand the decisions the agent made. He's satisfied, copies the branch out, and pushes it from his host machine after a quick review.

**Capabilities revealed:** Sandbox launch with project mount, scoped secrets, configurable SDKs, Claude Code runtime, BMAD workflow support, local git, Docker/Docker Compose, Playwright MCP, network isolation (internal only), project file persistence.

### Journey 2: Manuel Hits a Problem (Developer - Edge Case / Troubleshooting)

Manuel launches a sandbox and kicks off an agent task, but when he comes back, the agent has stalled. It tried to install a Python package that requires a C compiler not included in the sandbox image. The chat history shows the agent attempted the install, got the error, tried a workaround, and eventually gave up.

Manuel reads the error, realizes he needs to add `build-essential` to his asbox configuration. He updates the build arguments, rebuilds the sandbox image, and relaunches. This time the agent completes the task successfully.

In another scenario, Manuel launches a sandbox for a Node.js project he's been developing on macOS. The agent tries to run the app but crashes on a native module compiled for Darwin. Manuel adds `auto_isolate_deps: true` to his config and relaunches. asbox detects three `package.json` files (root, `packages/api`, `packages/web`), creates named Docker volumes for each `node_modules/` directory (e.g., `asbox-myproject-packages-api-node_modules`), logs the isolation mounts, and the agent runs `npm install` inside the sandbox to get Linux-native binaries. Everything works. Named volumes persist across sandbox restarts, so subsequent launches reuse the Linux-native dependencies without reinstalling.

In another scenario, the agent tries to `git push` to the remote. It gets "unauthorized" and moves on -- it logs the commit locally and notes in the chat that it couldn't push. Manuel sees this when he reviews and pushes manually from his host. No damage done.

**Capabilities revealed:** Build argument configuration, sandbox image rebuild, agent error recovery behavior, git push boundary enforcement, clear failure messages at boundaries, automatic dependency isolation via named Docker volumes for cross-platform compatibility.

### Journey 3: Manuel Sets Up a New Project's Sandbox (Builder/Maintainer)

Manuel's team is starting a new project that uses Go 1.23 and PostgreSQL. Manuel needs a sandbox configured for this stack. He looks at the asbox configuration file, sets the Go version, adds PostgreSQL as a Docker Compose service, and includes the necessary CLI tools. He builds the image.

He tests it by launching a sandbox, starting a quick Claude Code session, and asking the agent to scaffold a Go project, write a database migration, and run it against the PostgreSQL container. Everything works. He shares the configuration file with his team so they can build the same sandbox image.

**Capabilities revealed:** Configuration-driven SDK versioning, Docker Compose service definitions, sandbox image build process, shareable configuration, team distribution via single binary.

### Journey 4: The Agent's Perspective (AI Agent - Inside the Sandbox)

Claude Code starts in a sandbox. It sees a project directory with source code, a BMAD planning artifacts folder, and a story spec. It has access to git (local only), Node.js, Docker, curl, wget, and a Playwright MCP server.

The agent begins working. It reads the story spec, plans the implementation, and starts writing code. It needs to check API documentation online -- it uses curl to fetch it. It writes Playwright E2E tests and needs a running app to test against -- it writes a Dockerfile, creates a docker-compose.yml, and runs `docker compose up`. It uses the Playwright MCP to open a browser, navigate to `localhost:3000`, and verify the UI renders correctly.

At no point does the agent encounter the developer's SSH keys, cloud credentials, or host filesystem. It operates in a complete but bounded world. When it tries operations outside the boundary (git push, accessing host paths), it gets standard CLI errors and adapts its approach. The agent doesn't know or care that it's in a sandbox -- it just has a well-provisioned development environment.

**Capabilities revealed:** Agent-transparent isolation (sandbox is invisible to the agent), full toolchain availability, MCP integration for browser automation, internet access for documentation, Docker Compose for service orchestration, standard error codes at boundaries.

### Journey Requirements Summary

| Capability Area | Journeys | Priority |
|---|---|---|
| Sandbox launch with project mount | 1, 2, 3 | MVP |
| Scoped secrets injection | 1 | MVP |
| Configurable SDK versions (build args) | 1, 2, 3 | MVP |
| Claude Code / Gemini CLI runtime | 1, 4 | MVP |
| Local-only git (push blocked) | 1, 2, 4 | MVP |
| Docker and Docker Compose access | 1, 3, 4 | MVP |
| Playwright MCP integration | 1, 4 | MVP |
| Internet access (outbound) | 4 | MVP |
| Common CLI tools | 4 | MVP |
| BMAD method support | 1, 4 | MVP |
| Build argument configuration | 2, 3 | MVP |
| Sandbox image rebuild | 2, 3 | MVP |
| Shareable configuration | 3 | MVP |
| Standard error codes at boundaries | 2, 4 | MVP |
| Auto dependency isolation (named Docker volumes) | 2 | MVP |

## Innovation & Novel Patterns

### Detected Innovation Areas

**IDE-Agnostic Agent Isolation:** Existing sandboxed development environments (dev containers, Codespaces) operate at the IDE extension layer -- they define what's inside the environment and assume a VS Code-compatible client. asbox operates at a different layer entirely: it defines what can't get out. The agent works in a terminal, decoupled from any editor or IDE. Any workflow, any editor, any automation pipeline can interact with it. This is not competing with dev containers -- it's a containment boundary, not an IDE protocol.

**Stage-Gated Autonomy Model:** Rather than binary "supervised vs unsupervised," sandbox enables a stage-gated autonomy pattern. Developers delegate an entire stage of work (e.g., a full BMAD story with dozens of subtasks), the agent self-assesses and iterates within that stage, and the human reviews only at stage boundaries. This is a novel trust model -- the blast radius is constrained by the environment, so the agent can be trusted with longer autonomous runs.

**Environment-as-Trust-Boundary:** The core paradigm shift is treating the container environment itself as the primary trust mechanism, rather than relying on agent-level permission systems. This inverts the conventional approach (restrict what the agent can do) into a containment approach (restrict where the agent can reach).

### Market Context & Competitive Landscape

- **VS Code Dev Containers / Codespaces:** Operates at the IDE extension layer -- defines environment contents, assumes VS Code client. Not designed for autonomous AI agents or credential isolation. Different layer, different concern.
- **Daytona / Gitpod:** Cloud-based remote development environments. Solve "development from anywhere," not "agent isolation." Adjacent space, different problem.
- **Claude Code `--dangerously-skip-permissions`:** Removes permission prompts but provides no isolation -- the agent has full host access. Solves the interruption problem but not the trust problem.
- **Docker-based development:** General-purpose containerization exists, but no purpose-built solution combines container isolation with AI agent runtime configuration, MCP integration, and credential boundary enforcement.
- No direct competitor currently solves the specific combination of **agent isolation + IDE independence + stage-gated autonomy**.

### Validation Approach

Adversarial testing: explicitly task AI agents with breaking out of the sandbox boundary. Validation scenarios include:
- Agent attempts to access host filesystem outside mounted paths
- Agent attempts to read host SSH keys or cloud credentials
- Agent attempts `git push` to remote repositories (intercepted by git wrapper returning standard error)
- Agent attempts to spawn containers with public network exposure
- Agent attempts to access host browser or GUI
- Agent attempts Docker socket escape or container breakout
- Agent attempts to escalate privileges via inner Docker

The sandbox passes validation when all adversarial scenarios fail with standard CLI error codes and the host remains unaffected.

### Risk Mitigation

- **Docker-in-Docker escape risk:** The widest attack surface. Hard constraint: inner Docker must not require privileged mode on the outer container. **Resolved:** Rootless Podman was chosen as the inner container runtime, with docker CLI alias for agent compatibility. Podman runs daemonless and rootless by default, using `vfs` storage driver (compatible with nested containers in Docker Desktop), `netavark` networking with `aardvark-dns` for service name resolution, and `file` events logger (avoids journald dependency). Host Docker socket mount and `--privileged` DinD are explicitly ruled out.
- **Network exposure risk:** Inner containers share a private Docker network visible only within the sandbox. No inner container port is reachable from outside the sandbox boundary. This is standard Docker bridge networking, explicitly configured and enforced.
- **Credential leakage via internet:** Agent has outbound internet access and could theoretically exfiltrate scoped secrets. Mitigation: scoped secrets are opt-in and minimal; developers only inject what's strictly needed. Future: egress filtering.
- **Git push isolation:** Implemented via a git wrapper that allows all commands except `push`, returning standard "unauthorized" errors. Simpler and more testable than network-level blocking.
- **Build reproducibility:** Base images pinned to digest (not tag) to prevent silent drift. SDK versions set via explicit build arguments.
- **MCP server lifecycle:** MCP servers (Playwright, etc.) are pre-installed in the sandbox image. The agent starts them as needed via MCP protocol. No pre-running processes required at sandbox launch.
- **Dockerfile.template complexity (mitigated):** Go's `text/template` with conditional blocks (`{{if}}`, `{{range}}`) and typed config structs provides robust, readable template logic. No longer a significant risk compared to the previous bash sed substitution approach.
- **Go rewrite migration risk:** The entire CLI is being rewritten from bash to Go. Mitigation: the container-side scripts (entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh) remain bash and are unchanged -- the rewrite scope is limited to the host-side CLI. All existing functional behavior is preserved; only the implementation technology changes.
- **Embedded asset drift risk:** Supporting files compiled into the binary cannot be patched without rebuilding. Mitigation: this is intentional -- it ensures consistency between the CLI version and its embedded assets. Version-tagged releases make the relationship explicit.
- **BMAD multi-repo mount complexity:** Mounting multiple external repositories with generated agent instructions adds configuration surface area. Mitigation: `bmad_repos` is a flat list of paths with convention-based target mapping (`/workspace/repos/<name>`). The generated agent instructions file uses a Go template with the repo list -- no user configuration beyond the path list.

## Developer Tool Specific Requirements

### Project-Type Overview

asbox is a Go CLI binary that wraps Docker operations to provide AI coding agents with isolated development environments. The user interface is entirely terminal-based: a configuration file defines the environment, and the CLI handles build and run operations. There is no GUI, no web interface, no IDE plugin.

### Configuration Surface

The asbox configuration file (`.asbox/config.yaml` by default) is the primary interface for defining a sandbox environment. It specifies:

- **SDK versions:** Node.js, Go, Python, and other runtimes with explicit version pinning
- **System packages:** Additional OS-level packages needed (e.g., build-essential, libpq-dev)
- **MCP servers:** Which MCP integrations to pre-install (e.g., Playwright)
- **Mounts:** Host directories to mount into the sandbox (project files, shared assets). Relative paths are resolved relative to the config file location, not the working directory.
- **Secrets:** Names of host environment variables to inject into the sandbox. The config declares which secrets are needed; the CLI resolves their values from the host environment at runtime and passes them via `docker run --env`. If a declared secret is not set in the host environment, the CLI errors with a clear message. Secrets never appear in the config file.
- **Environment variables:** Non-secret configuration for the agent runtime
- **Agent runtime:** Which AI agent to run, mapped to specific commands inside the container:
  - `claude-code` -> `claude --dangerously-skip-permissions` (permissions handled by sandbox isolation). Installed via official Anthropic install script.
  - `gemini-cli` -> `gemini`. Installed via npm global install.
- **Host agent config:** Optional read-write mount of the host agent's configuration directory (e.g., `~/.claude`) into the container for OAuth token synchronization. Sets the corresponding config directory environment variable (e.g., `CLAUDE_CONFIG_DIR`) so the agent can read and refresh tokens.
- **Project name:** Optional override for the project identifier used in image and volume naming. Defaults to the parent directory name of the `.asbox/` folder, sanitized to lowercase alphanumeric with hyphens.
- **BMAD multi-repo workflow (`bmad_repos`):** A list of local paths to checked-out repositories. When configured, the system automatically creates mounts for each repository into `/workspace/repos/<repo_name>` inside the sandbox container. An agent configuration file (e.g., `CLAUDE.md`) is generated and mounted that instructs the agent to perform git operations and code changes within the relevant repositories under the `repos/` directory. This enables development workflows that span multiple repositories as a unified workspace.

Example structure:
```yaml
agent: claude-code
project_name: my-app
sdks:
  nodejs: "22"
  python: "3.12"
packages:
  - build-essential
  - curl
  - wget
mcp:
  - playwright
mounts:
  - source: .
    target: /workspace
auto_isolate_deps: true
host_agent_config:
  source: "~/.claude"
  target: "/opt/claude-config"
bmad_repos:
  - /Users/manuel/repos/frontend-app
  - /Users/manuel/repos/api-service
  - /Users/manuel/repos/shared-lib
secrets:
  - ANTHROPIC_API_KEY
env:
  GIT_AUTHOR_NAME: "Sandbox Agent"
  GIT_AUTHOR_EMAIL: "sandbox@localhost"
  NODE_ENV: development
```

When `auto_isolate_deps` is enabled, asbox scans all mounted project paths at launch for `package.json` files and creates named Docker volume mounts over each sibling `node_modules/` directory. Volumes are named deterministically using the pattern `asbox-<project_name>-<relative-path>-node_modules`, so they persist across sandbox restarts and avoid reinstallation. This prevents macOS-native compiled dependencies from clashing with the Linux sandbox environment. The scan is logged so the developer can see which directories were isolated.

**Accepted edge case:** On a completely fresh project with no `package.json` files yet, the first sandbox launch will have no automatic mounts. This is acceptable because the agent will run `npm install` inside the Linux sandbox, producing Linux-native dependencies from the start -- no clash occurs.

### CLI Interface

**Binary:** `asbox` (Go CLI)

**Commands:**
- `asbox build` -- Builds the sandbox container image from configuration. Reads `.asbox/config.yaml` by default, override with `-f path/to/config.yaml`.
- `asbox run` -- Builds if image not present, then launches the sandbox in TTY mode with the configured agent. Override config with `-f`. The container runs interactively; Ctrl+C stops it.
- `asbox init` -- Generates a starter `.asbox/config.yaml` with sensible defaults and inline comments.

**Flags:**
- `-f, --file` -- Path to config file (default: `.asbox/config.yaml`)
- `--help` for usage information

**Exit Codes:**
- `0` -- Success
- `1` -- Configuration or general error
- `2` -- Usage error (invalid command or flags)
- `3` -- Missing dependency (Docker not found)
- `4` -- Secret validation error (declared secret not set in host environment)

**Installation:** Distributed as a single statically-linked Go binary. Download or `go install` and place on PATH. All supporting files (Dockerfile template, entrypoint script, git wrapper, agent instructions, config template) are embedded in the binary.

**Design principles:**
- Minimal command surface -- three commands cover the full workflow (init, build, run)
- Build-if-needed on `run` eliminates manual build steps for common use
- TTY mode means the developer sees agent output in real-time if they choose to watch, or can background/detach
- No daemon, no server, no background processes -- the sandbox lifecycle is the process lifecycle
- Zero external dependencies -- single binary with embedded assets, only Docker required on host

### Technical Architecture Considerations

**Go CLI structure:**
- Single `asbox` binary built from Go, with cobra (or similar) CLI framework for command dispatch (init, build, run)
- YAML configuration parsed via Go's `gopkg.in/yaml.v3` -- no external yq dependency
- Dockerfile generated from embedded Go `text/template` templates with conditional logic and value substitution -- replaces fragile sed-based placeholder stripping
- All supporting files embedded via Go's `embed` package: Dockerfile template, entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh, agent-instructions.md, starter config.yaml
- Calls `docker build` and `docker run` via `os/exec`
- Content-hash caching via Go's `crypto/sha256` over rendered template output + config content
- Structured error handling with typed errors and distinct exit codes
- Assembles Docker run flags for mounts, secrets, env vars, and TTY

**Dependencies:** Docker (or Podman) on the host. No other external dependencies -- single statically-linked binary. The CLI validates Docker is present at startup with error messages that name the missing dependency and state the required fix action.

**Image build strategy:**
- Base image (Ubuntu 24.04 LTS) pinned to digest for reproducibility
- Tini installed as init process for proper signal forwarding and zombie process reaping
- SDK installation via version managers or official binaries based on build arguments. Binary downloads MUST detect host architecture at build time (`$(dpkg --print-architecture)` or `$(uname -m)`) to support both amd64 and arm64 — no hardcoded architecture strings.
- Rootless Podman 5.x installed from upstream Kubic/OBS repository with docker CLI alias (`podman-docker`) for agent compatibility
- Docker Compose v2 installed as standalone binary
- MCP servers installed at build time; MCP manifest embedded at `/etc/sandbox/mcp-servers.json`
- Agent CLI installed at build time: Claude Code via official Anthropic install script, Gemini CLI via npm
- Agent environment instruction files (`CLAUDE.md` or `GEMINI.md`) installed to agent config directory
- Git wrapper and isolation boundaries baked into the image
- Layer caching for fast rebuilds when only configuration changes

**Runtime behavior:**
- Container runs with Tini as PID 1 for signal forwarding, launching the entrypoint script
- Entrypoint aligns sandbox user UID/GID with host via `HOST_UID`/`HOST_GID` environment variables (using `usermod`/`groupmod`) to ensure correct mount permissions
- Project directory mounted at configured target path (paths resolved relative to config file location)
- Secrets resolved from host environment variables and injected via `--env` flags (not written to filesystem)
- Host agent config directory (if configured) mounted read-write for OAuth token synchronization, with `CLAUDE_CONFIG_DIR` set for Claude Code
- Inner containers available via rootless Podman with docker CLI alias. Podman runs daemonless with `vfs` storage driver, `netavark` networking with `aardvark-dns` for service name DNS resolution, and `file` events logger
- Podman API socket started at `$XDG_RUNTIME_DIR/podman/podman.sock` with `DOCKER_HOST` pointing to it for Docker CLI compatibility
- Healthcheck poller runs as a background daemon, polling containers every 10 seconds for health status (required because the container has no systemd to run healthcheck timers)
- Testcontainers compatibility: Ryuk disabled, socket override configured, localhost host override set
- MCP manifest merge at runtime: build-time manifest merged with project `.mcp.json` if present; project config takes precedence on name conflicts
- Private network bridge for inner container communication
- When `auto_isolate_deps` is enabled: at launch, scan all mounted paths — primary mounts and `bmad_repos` — for `package.json` files, derive `node_modules/` sibling paths, and create named Docker volume mounts (`-v asbox-<project>-<path>-node_modules:<container-path>/node_modules`) on the `docker run` invocation. For bmad_repos, container paths are derived from `/workspace/repos/<basename>`. Entrypoint fixes ownership of named volume mounts for the unprivileged sandbox user. Log each discovered mount. On fresh projects with no `package.json`, no mounts are added -- the agent creates Linux-native dependencies from scratch.
- When `bmad_repos` is configured: each listed repository path is mounted into `/workspace/repos/<repo_name>` inside the container. An agent configuration file (e.g., `CLAUDE.md`) is generated from a Go template containing the list of mounted repositories and instructions for the agent to perform git operations and code changes within the `repos/` directory. This file is mounted into the container at the agent's expected configuration location.

### Installation & Distribution

- Distributed as a single statically-linked Go binary
- Pre-built binaries for macOS (arm64/amd64) and Linux (amd64) via GitHub releases
- Alternatively: `go install` from source
- Dependencies: Docker (or Podman) on the host -- no other external dependencies
- Documentation in project README and `asbox --help`

### Implementation Considerations

- Go for type safety, parallel test execution, embedded assets, and native YAML/template support
- `gopkg.in/yaml.v3` for YAML parsing -- no external dependencies
- Go `text/template` for Dockerfile rendering with conditional blocks and value injection
- Go `embed` package for all supporting files: Dockerfile template, entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh, agent-instructions.md, starter config template
- Content-hash-based image caching over rendered template output + config content for smart rebuild detection
- Mount path resolution relative to config file location, not working directory
- Structured error types with clear messages for missing dependencies, unset secrets, and invalid configuration
- `auto_isolate_deps` scan: `filepath.WalkDir` for `package.json` files under primary mounts and `bmad_repos` paths, excluding `node_modules`; named volume flags assembled programmatically (`-v asbox-<project>-<path>-node_modules:<target>`). Entrypoint `chown`s named volume mounts for the unprivileged sandbox user
- `bmad_repos` mount assembly: each repo path mounted to `/workspace/repos/<basename>`, agent instruction file generated via Go template and mounted into container
- Image naming convention: `asbox-<project-name>:<content-hash>` for cache management (first 12 characters of SHA256)

## Project Scoping & Phased Development

### MVP Strategy & Philosophy

**MVP Approach:** Problem-solving MVP -- a working tool that proves unsupervised agent development is safe and productive. Built for personal daily use first, extended to the team once validated.

**Resource Requirements:** Solo developer (Manuel). The tool's user and builder are the same person in Phase 1, which means feedback loops are instant and the feature set is driven by direct pain points.

### MVP Feature Set (Phase 1)

**Core User Journeys Supported:**
- Journey 1 (Delegate a feature build) -- full support
- Journey 2 (Hit a problem, reconfigure, rebuild) -- full support
- Journey 3 (Set up a new project's sandbox) -- full support
- Journey 4 (Agent operates transparently inside sandbox) -- full support

**Must-Have Capabilities:**

All of the following are non-negotiable for MVP -- removing any one breaks the core use case:

| Capability | Rationale |
|---|---|
| Go CLI binary (init, build, run) | Primary user interface -- no CLI, no product |
| `.asbox/config.yaml` configuration | Drives everything -- SDKs, mounts, secrets, agent selection |
| Embedded Dockerfile template generation | Config-driven builds are the core value over manual Docker |
| Embedded assets via Go `embed` | Single binary distribution -- no external files to manage |
| Configurable SDK versions (Node.js, Go, Python) | Different projects need different stacks |
| Docker and Docker Compose inside sandbox | Agents must build images and run services to test |
| Non-privileged inner Docker (rootless/sysbox/Podman) | Isolation is the product -- privileged mode defeats the purpose |
| Playwright MCP integration (chromium + webkit) | E2E testing is part of the inner development loop -- webkit required for mobile device emulation |
| Local-only git (push blocked via wrapper) | Core isolation boundary |
| Scoped secrets injection (host env -> container env) | Agent needs API keys, nothing else |
| Internet access (outbound) | Agent must fetch docs, packages, dependencies |
| Common CLI tools (curl, wget, dig, etc.) | Standard development workflow |
| BMAD method support | Full planning-through-implementation workflow inside sandbox |
| BMAD multi-repo workflow (`bmad_repos`) | Multi-repository development workflows with auto-generated agent instructions |
| Base image pinned to digest | Build reproducibility |
| Content-hash image caching | Avoid unnecessary rebuilds |
| TTY mode with Ctrl+C lifecycle | Simple, no daemon complexity |
| Auto dependency isolation (`auto_isolate_deps`) | Prevents macOS/Linux native module clashes via named Docker volumes |
| Host agent config mount (`host_agent_config`) | OAuth token synchronization for agent CLI authentication |

**Explicitly deferred from MVP:**
- Multi-agent parallel orchestration
- Session persistence options
- Audit trail / activity logging
- Pre-built image variants
- Additional MCP integrations beyond Playwright
- Team onboarding documentation (README + --help is sufficient for solo use)

### Phase 2: Team Adoption

- Polished README and onboarding documentation for team members
- Pre-built sandbox image variants for common project types (Node.js + Playwright, Go + PostgreSQL, Python + ML stack)
- Session persistence options (ephemeral vs persistent lifecycle)
- Additional MCP integrations (browser automation beyond Playwright, file search, etc.)
- Audit trail and agent activity logging for team visibility

### Phase 3: Scale & Orchestration

- Multi-agent parallel sandbox orchestration -- multiple agents on different tasks simultaneously
- Lightweight orchestration layer for chaining agent tasks across sandboxes
- Multi-agent collaboration within a single sandbox (frontend + backend agents)
- Sandbox telemetry and productivity analytics (tasks completed, time saved)
- Community-contributed sandbox configurations and MCP integration packs

### Risk Mitigation Strategy

**Technical Risks:**
- **Docker isolation model (resolved):** Rootless Podman was selected as the inner container runtime after evaluating rootless Docker, sysbox, and Podman. Podman's daemonless, rootless-by-default architecture provides the strongest isolation with the least friction on both macOS (via Docker Desktop) and Linux. The `vfs` storage driver ensures compatibility with nested container scenarios. Docker CLI compatibility is provided via the `podman-docker` alias package.
- **MCP server integration inside container:** Playwright MCP needs browser runtimes (chromium and webkit) inside Docker. Mitigation: validate Playwright in Docker early -- this is a known-solvable problem but requires correct base image and dependencies. WebKit requires significantly more system libraries (~60 packages including GTK4, GStreamer, Wayland) but is necessary for mobile device emulation tests.

**Market Risks:**
- Minimal -- this is a personal tool first. The market validation is: does Manuel use it daily? If yes, extend to team. If the team uses it daily, consider broader distribution.

**Resource Risks:**
- Solo developer means serial execution. The MVP feature set is large but each piece is well-understood (Go CLI tooling, Docker, container orchestration). The risk is calendar time, not complexity. Mitigation: build in the order of the inner development loop -- get a basic container running first, then add isolation boundaries, then config-driven builds, then MCP.

## Functional Requirements

### Sandbox Configuration

- FR1: Developer can define SDK versions (Node.js, Go, Python) in a YAML configuration file
- FR2: Developer can specify additional system packages to install in the sandbox image
- FR3: Developer can configure which MCP servers to pre-install (e.g., Playwright)
- FR4: Developer can declare host directories to mount into the sandbox with source and target paths
- FR5: Developer can declare secret names that will be resolved from host environment variables at runtime
- FR6: Developer can set non-secret environment variables for the agent runtime
- FR7: Developer can select which AI agent runtime to use (claude-code, gemini-cli)
- FR8: Developer can override the default config file path with a `-f` flag
- FR9: Developer can generate a starter configuration file with sensible defaults via `asbox init`
- FR9a: Developer can enable automatic dependency isolation (`auto_isolate_deps: true`) to create named Docker volume mounts over platform-specific dependency directories (e.g., `node_modules/`) within mounted project paths
- FR9b: When `auto_isolate_deps` is enabled, the system scans all mounted project paths at launch — including primary mounts and `bmad_repos` mounts — for `package.json` files and creates named Docker volumes (pattern: `asbox-<project>-<path>-node_modules`) for each corresponding `node_modules/` directory. Named volumes persist across sandbox restarts.
- FR9c: The system logs all auto-detected dependency isolation mounts at launch so the developer has visibility into what was isolated
- FR9d: Developer can configure a host agent configuration directory mount (`host_agent_config`) with source and target paths for OAuth token synchronization between host and sandbox
- FR9e: Developer can optionally set `project_name` in configuration to override the default project identifier used for image and volume naming

### Sandbox Lifecycle

- FR10: Developer can build a sandbox container image from configuration via `asbox build`
- FR11: Developer can launch a sandbox session in TTY mode via `asbox run`
- FR12: System automatically builds the image if not present when `asbox run` is invoked
- FR13: System detects when configuration or template files have changed and triggers a rebuild
- FR14: Developer can stop a running sandbox session with Ctrl+C
- FR15: System validates that Docker is present before proceeding
- FR16: System validates that all declared secrets are set in the host environment before launching
- FR16a: When `auto_isolate_deps` is enabled, the system creates named Docker volume mounts for each detected dependency directory during sandbox launch and ensures correct ownership for the unprivileged sandbox user

### Agent Runtime

- FR17: AI agent can execute inside the sandbox with full terminal access
- FR18: Claude Code agent launches with permissions skipped (sandbox provides isolation)
- FR19: Gemini CLI agent launches with standard configuration
- FR20: Agent can interact with project files mounted from the host
- FR21: Agent can execute BMAD method workflows (read/write planning artifacts, project knowledge, output documents)
- FR22: Agent can install additional packages and dependencies at runtime via package managers

### Development Toolchain

- FR23: Agent can use git for local operations (add, commit, log, diff, branch, checkout, merge, amend)
- FR24: Agent can access the internet for fetching documentation, packages, and dependencies
- FR25: Agent can use common CLI tools (curl, wget, dig, etc.)
- FR26: Agent can build Docker images from Dockerfiles inside the sandbox
- FR27: Agent can run `docker compose up` to start multi-service applications inside the sandbox
- FR28: Agent can reach application ports of services running in inner containers
- FR29: Agent can run Playwright tests against running applications via MCP integration
- FR29a: Agent can run Playwright tests using chromium for desktop browser testing and webkit for mobile device emulation (iPhone, iPad, etc.)
- FR30: Agent can start MCP servers as needed via MCP protocol

### Isolation Boundaries

- FR31: System blocks git push operations, returning standard authentication failure errors
- FR32: System prevents agent access to host filesystem beyond explicitly mounted paths
- FR33: System prevents agent access to host credentials, SSH keys, and cloud tokens not explicitly declared as secrets
- FR34: System prevents inner containers from being reachable outside the sandbox network
- FR35: System enforces non-privileged inner Docker (no `--privileged` mode, no host Docker socket mount)
- FR36: System provides a private network bridge for communication between inner containers only
- FR37: System returns standard CLI error codes when agent attempts operations outside the boundary

### Image Build System

- FR38: System generates a Dockerfile from an embedded Go template using configuration values
- FR39: System pins base images to digest for reproducible builds
- FR40: System passes SDK versions as build arguments to the Dockerfile
- FR41: System installs configured MCP servers at image build time
- FR42: System enforces git push blocking and isolation boundaries from within the sandbox image
- FR43: System tags images using content hash (pattern: `asbox-<project>:<hash>`) for cache management
- FR44: System installs agent environment instruction files (`CLAUDE.md`, `GEMINI.md`) into the sandbox user's home directory at image build time, providing the agent with sandbox-specific constraints (no sudo, container runtime details, Playwright usage, e2e test workflow) so it can operate without trial-and-error troubleshooting
- FR45: When `host_agent_config` is configured, system mounts the host agent configuration directory read-write into the sandbox and sets the appropriate config directory environment variable (e.g., `CLAUDE_CONFIG_DIR`) for the agent runtime
- FR46: System merges build-time MCP server manifest with project-level `.mcp.json` at runtime; project configuration takes precedence on name conflicts
- FR47: System exits with specific exit codes to distinguish error categories: 0 (success), 1 (configuration error), 2 (usage error), 3 (missing dependency), 4 (secret validation failure). Implemented via Go structured error types.
- FR48: System uses Tini as init process (PID 1) for proper signal forwarding and zombie process reaping
- FR49: System aligns sandbox user UID/GID with host user at container startup to ensure correct file permissions on mounted directories
- FR50: All supporting files (Dockerfile template, entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh, agent-instructions.md, starter config template) are embedded in the Go binary via the `embed` package and require no external file dependencies
- FR51: Developer can configure `bmad_repos` as a list of local paths to checked-out repositories in `.asbox/config.yaml`
- FR52: When `bmad_repos` is configured, the system automatically creates mount mappings for each repository into `/workspace/repos/<repo_name>` inside the sandbox container
- FR53: When `bmad_repos` is configured, the system generates an agent configuration file (e.g., `CLAUDE.md`) instructing the agent that git operations and code changes should be executed within the relevant repositories under the `repos/` directory, and mounts it into the container
- FR54: The system is distributed as a single statically-linked Go binary with no external runtime dependencies beyond Docker
- FR55: Developer can pass `--no-cache` to `asbox build` (and implicitly to `asbox run`) to bypass the content-hash image existence check and pass `--no-cache` to the underlying Docker build, forcing a complete image rebuild with no cached layers

## Non-Functional Requirements

### Security & Isolation

The security model protects against accidental leakage from AI agents that hallucinate or attempt unintended operations. It is not designed to resist a deliberately adversarial agent actively attempting container escape.

- NFR1: No host credential, SSH key, or cloud token is accessible inside the sandbox unless explicitly declared in the config file's secrets list
- NFR2: Git push operations fail with standard error codes regardless of remote configuration present in mounted repositories
- NFR3: Inner containers cannot bind to network interfaces reachable from outside the sandbox
- NFR4: The sandbox container runs without `--privileged` flag and without host Docker socket mount
- NFR5: Secrets are injected as runtime environment variables only -- never written to the filesystem, image layers, or build cache
- NFR6: Mounted paths are limited to those explicitly declared in config -- no implicit access to home directory, `.ssh`, `.aws`, or other host paths

### Integration

- NFR7: The CLI works with Docker Engine 20.10+ on the host. Inside the sandbox, rootless Podman 5.x is the container runtime with docker CLI alias for agent compatibility
- NFR8: YAML configuration is parsed via Go's `gopkg.in/yaml.v3` -- no external parsing dependency required
- NFR9: MCP servers installed in the sandbox follow the standard MCP protocol and are startable by any MCP-compatible agent
- NFR10: The git wrapper is transparent to the agent for all operations except push -- git log, diff, commit, branch, etc. behave identically to standard git

### Portability & Reliability

- NFR11: The CLI runs on macOS (arm64/amd64) and Linux (amd64) as a statically-linked Go binary. Bash 4+ is no longer required on the host.
- NFR12: Image builds are reproducible -- the same config + template + sandbox files produce an identical image regardless of when or where the build runs (base image pinned to digest)
- NFR13: The CLI fails fast with error messages that name the missing dependency, unset secret, or invalid field and state the required fix action. Each error category returns a distinct exit code (1-4) for programmatic detection
- NFR14: A crashed or Ctrl+C'd sandbox leaves no orphaned containers or dangling networks on the host. Tini as PID 1 ensures proper signal forwarding and cleanup of child processes
- NFR15: Integration test suite covers all supported use cases (sandbox lifecycle, mounts, secrets, isolation boundaries, inner containers, MCP, auto_isolate_deps, bmad_repos) with parallel test execution via Go's testing framework
