---
stepsCompleted:
  - step-01-validate-prerequisites
  - step-02-design-epics
  - step-03-create-stories
  - step-04-final-validation
inputDocuments:
  - planning-artifacts/prd.md
  - planning-artifacts/architecture.md
---

# sandbox - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for sandbox, decomposing the requirements from the PRD and Architecture requirements into implementable stories.

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
FR9: Developer can generate a starter configuration file with sensible defaults via `sandbox init`
FR10: Developer can build a sandbox container image from configuration via `sandbox build`
FR11: Developer can launch a sandbox session in TTY mode via `sandbox run`
FR12: System automatically builds the image if not present when `sandbox run` is invoked
FR13: System detects when configuration or template files have changed and triggers a rebuild
FR14: Developer can stop a running sandbox session with Ctrl+C
FR15: System validates that required dependencies (Docker, yq) are present before proceeding
FR16: System validates that all declared secrets are set in the host environment before launching
FR17: AI agent can execute inside the sandbox with full terminal access
FR18: Claude Code agent launches with permissions skipped (sandbox provides isolation)
FR19: Gemini CLI agent launches with standard configuration
FR20: Agent can interact with project files mounted from the host
FR21: Agent can execute BMAD method workflows (read/write planning artifacts, project knowledge, output documents)
FR22: Agent can install additional packages and dependencies at runtime via package managers
FR23: Agent can use git for local operations (add, commit, log, diff, branch, checkout, merge, amend)
FR24: Agent can access the internet for fetching documentation, packages, and dependencies
FR25: Agent can use common CLI tools (curl, wget, dig, etc.)
FR26: Agent can build Docker images from Dockerfiles inside the sandbox
FR27: Agent can run `docker compose up` to start multi-service applications inside the sandbox
FR28: Agent can reach application ports of services running in inner containers
FR29: Agent can run Playwright tests against running applications via MCP integration
FR30: Agent can start MCP servers as needed via MCP protocol
FR31: System blocks git push operations via a git wrapper, returning standard "unauthorized" errors
FR32: System prevents agent access to host filesystem beyond explicitly mounted paths
FR33: System prevents agent access to host credentials, SSH keys, and cloud tokens not explicitly declared as secrets
FR34: System prevents inner containers from being reachable outside the sandbox network
FR35: System enforces non-privileged inner Docker (no `--privileged` mode, no host Docker socket mount)
FR36: System provides a private network bridge for communication between inner containers only
FR37: System returns standard CLI error codes when agent attempts operations outside the boundary
FR38: System generates a Dockerfile from a Dockerfile.template using configuration values
FR39: System pins base images to digest for reproducible builds
FR40: System passes SDK versions as build arguments to the Dockerfile
FR41: System installs configured MCP servers at image build time
FR42: System bakes git wrapper and isolation boundary scripts into the image
FR43: System tags images using content hash of config + template + sandbox files for cache management

### NonFunctional Requirements

NFR1: No host credential, SSH key, or cloud token is accessible inside the sandbox unless explicitly declared in the config file's secrets list
NFR2: Git push operations fail with standard error codes regardless of remote configuration present in mounted repositories
NFR3: Inner containers cannot bind to network interfaces reachable from outside the sandbox
NFR4: The sandbox container runs without `--privileged` flag and without host Docker socket mount
NFR5: Secrets are injected as runtime environment variables only -- never written to the filesystem, image layers, or build cache
NFR6: Mounted paths are limited to those explicitly declared in config -- no implicit access to home directory, `.ssh`, `.aws`, or other host paths
NFR7: The CLI works with Docker Engine 20.10+ and is compatible with Podman as an alternative runtime
NFR8: YAML configuration is parsed via `yq` v4+ -- the script fails clearly if `yq` is not installed or is an incompatible version
NFR9: MCP servers installed in the sandbox follow the standard MCP protocol and are startable by any MCP-compatible agent
NFR10: The git wrapper is transparent to the agent for all operations except push -- git log, diff, commit, branch, etc. behave identically to standard git
NFR11: The CLI runs on macOS (arm64/amd64) and Linux (amd64) with bash 4+
NFR12: Image builds are reproducible -- the same config + template + sandbox files produce an identical image regardless of when or where the build runs (base image pinned to digest)
NFR13: The CLI fails fast with clear, actionable error messages when dependencies are missing, secrets are unset, or configuration is invalid
NFR14: A crashed or Ctrl+C'd sandbox leaves no orphaned containers or dangling networks on the host

### Additional Requirements

- Podman 5.8.x is the inner container runtime (daemonless, rootless, no --privileged needed). Docker aliased to Podman inside container.
- Ubuntu 24.04 LTS base image pinned to digest
- Podman installed from upstream Kubic repository (not Ubuntu default apt) to ensure version 5.x
- Tini as PID 1 for signal forwarding and zombie reaping
- Dockerfile generation via bash template substitution with conditional block markers (`{{IF_NAME}}` / `{{/IF_NAME}}`)
- Template processing must validate matching open/close tags and resolve all placeholders before producing a Dockerfile
- Single `parse_config()` function consumed by both build and run paths -- no ad-hoc yq calls
- Content-hash cache key composed of: config.yaml, Dockerfile.template, scripts/entrypoint.sh, scripts/git-wrapper.sh
- Entrypoint generates `.mcp.json` from build-time manifest at `/etc/sandbox/mcp-servers.json`; merges with existing project `.mcp.json` (project config wins on name conflicts)
- Git wrapper at `/usr/local/bin/git` intercepts `push`, passes all else to `/usr/bin/git`
- Secret validation uses `${VAR+x}` (declared?) not `${VAR:+x}` (non-empty?) -- empty values are valid
- Exit codes: 0 (success), 1 (general error), 2 (usage error), 3 (dependency error), 4 (secret validation error)
- `set -euo pipefail` enforced in all bash scripts
- No color codes in output -- plain text for piping and logging
- Errors to stderr, info to stdout
- Script organization: shebang -> constants -> utilities -> config parsing -> build -> run -> init -> dispatch -> main
- macOS ships bash 3.2; users must `brew install bash` -- affects shebang and install docs

### UX Design Requirements

N/A -- sandbox is a CLI tool with no GUI. No UX design document applicable.

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
| FR9 | Epic 1 | `sandbox init` starter config |
| FR10 | Epic 1 | `sandbox build` command |
| FR11 | Epic 1 | `sandbox run` in TTY mode |
| FR12 | Epic 1 | Auto-build on run |
| FR13 | Epic 1 | Change detection and rebuild |
| FR14 | Epic 1 | Ctrl+C stop |
| FR15 | Epic 1 | Dependency validation |
| FR16 | Epic 2 | Secret validation before launch |
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
| FR30 | Epic 5 | Start MCP servers via protocol |
| FR31 | Epic 3 | Git push blocked |
| FR32 | Epic 3 | Filesystem restricted to mounts |
| FR33 | Epic 3 | Host credentials inaccessible |
| FR34 | Epic 4 | Inner containers not reachable externally |
| FR35 | Epic 4 | Non-privileged inner Docker |
| FR36 | Epic 4 | Private network bridge |
| FR37 | Epic 3 | Standard error codes at boundaries |
| FR38 | Epic 1 | Dockerfile from template |
| FR39 | Epic 1 | Base images pinned to digest |
| FR40 | Epic 1 | SDK versions as build args |
| FR41 | Epic 5 | MCP servers installed at build time |
| FR42 | Epic 4 | Git wrapper and isolation scripts baked in |
| FR43 | Epic 1 | Content-hash image tagging |

## Epic List

### Epic 1: Sandbox Foundation -- Developer Can Build and Launch a Sandbox
A developer can create a configuration file, build a container image from it, and launch an interactive sandbox session with a working agent inside.
**FRs covered:** FR1, FR2, FR6, FR7, FR8, FR9, FR10, FR11, FR12, FR13, FR14, FR15, FR38, FR39, FR40, FR43

### Epic 2: Project Integration -- Agent Can Work With Project Files and Git
An agent inside the sandbox can access mounted project files, use local git for version control, and work with common CLI tools and internet access -- a complete development workflow minus Docker and MCP.
**FRs covered:** FR4, FR5, FR16, FR17, FR18, FR19, FR20, FR21, FR22, FR23, FR24, FR25

### Epic 3: Isolation Boundaries -- Sandbox Enforces Security Constraints
The sandbox enforces hard boundaries: git push is blocked, host filesystem is restricted to declared mounts, host credentials are inaccessible, and all boundary violations return standard CLI error codes.
**FRs covered:** FR31, FR32, FR33, FR37

### Epic 4: Inner Container Runtime -- Agent Can Build and Run Docker Services
An agent can build Docker images, run multi-service applications with Docker Compose, and reach application ports -- all inside the sandbox using Podman as a transparent Docker replacement.
**FRs covered:** FR26, FR27, FR28, FR34, FR35, FR36, FR42

### Epic 5: MCP Integration -- Agent Can Run Browser Tests via Playwright
An agent can use MCP servers (starting with Playwright) to run end-to-end browser tests against running applications inside the sandbox.
**FRs covered:** FR3, FR29, FR30, FR41

## Epic 1: Sandbox Foundation -- Developer Can Build and Launch a Sandbox

A developer can create a configuration file, build a container image from it, and launch an interactive sandbox session with a working agent inside.

### Story 1.1: CLI Skeleton and Dependency Validation

As a developer,
I want to run `sandbox` and see usage help, and have the script validate that Docker and yq are installed,
So that I know the tool is working and will fail clearly if prerequisites are missing.

**Acceptance Criteria:**

**Given** a developer has sandbox.sh on their PATH
**When** they run `sandbox --help`
**Then** usage information is displayed showing available commands (init, build, run) and flags (-f, --help)

**Given** Docker is not installed on the host
**When** the developer runs any sandbox command
**Then** the script exits with code 3 and prints "error: docker not found" to stderr

**Given** yq is not installed or is below v4
**When** the developer runs any sandbox command
**Then** the script exits with code 3 and prints a clear error about the yq requirement to stderr

**Given** bash version is below 4
**When** the developer runs sandbox.sh
**Then** the script exits with code 3 and prints a clear error about the bash version requirement

### Story 1.2: Configuration Initialization

As a developer,
I want to run `sandbox init` to generate a starter `.sandbox/config.yaml` with sensible defaults,
So that I have a working starting point for configuring my sandbox.

**Acceptance Criteria:**

**Given** no `.sandbox/config.yaml` exists in the current directory
**When** the developer runs `sandbox init`
**Then** a `.sandbox/config.yaml` is created with default agent (claude-code), Node.js SDK, common packages, and inline comments explaining each option

**Given** a `.sandbox/config.yaml` already exists
**When** the developer runs `sandbox init`
**Then** the script exits with code 1 and prints "error: config already exists" to stderr without overwriting

**Given** the developer specifies `-f custom/path/config.yaml`
**When** they run `sandbox init -f custom/path/config.yaml`
**Then** the config is generated at the specified path

### Story 1.3: Configuration Parsing

As a developer,
I want sandbox to parse my config.yaml and extract all settings (SDKs, packages, env vars, agent),
So that the build and run steps can use my configuration correctly.

**Acceptance Criteria:**

**Given** a valid config.yaml with SDK versions, packages, env vars, and agent selection
**When** sandbox parses the config via `parse_config()`
**Then** all values are extracted correctly and available for downstream use (build and run)

**Given** a config.yaml with missing required fields
**When** sandbox parses the config
**Then** the script exits with code 1 and prints a clear error identifying the missing field

**Given** the developer passes `-f path/to/config.yaml`
**When** sandbox parses the config
**Then** it reads from the specified path instead of `.sandbox/config.yaml`

### Story 1.4: Dockerfile Generation from Template

As a developer,
I want sandbox to generate a valid Dockerfile from Dockerfile.template using my config values,
So that my sandbox image is built with the SDKs and packages I specified.

**Acceptance Criteria:**

**Given** a config with Node.js 22 and Python 3.12 SDKs and packages [build-essential, curl]
**When** sandbox processes Dockerfile.template
**Then** a resolved Dockerfile is produced with Node.js 22 and Python 3.12 install blocks active, Go blocks stripped, and packages included

**Given** a config with no SDKs specified
**When** sandbox processes Dockerfile.template
**Then** all conditional SDK blocks are stripped and only base tooling remains

**Given** a template with mismatched `{{IF_NAME}}` / `{{/IF_NAME}}` tags
**When** sandbox processes the template
**Then** the script exits with code 1 and prints an error identifying the unmatched tag

**Given** a resolved Dockerfile would contain unresolved `{{PLACEHOLDER}}` values
**When** sandbox processes the template
**Then** the script exits with code 1 and prints an error identifying the unresolved placeholder

### Story 1.5: Image Build with Content-Hash Caching

As a developer,
I want `sandbox build` to build my container image and skip rebuilds when nothing has changed,
So that I get fast iteration without unnecessary rebuilds.

**Acceptance Criteria:**

**Given** a valid config.yaml and Dockerfile.template
**When** the developer runs `sandbox build`
**Then** a Docker image is built and tagged as `sandbox-<project>:<content-hash>` where content-hash is derived from config.yaml, Dockerfile.template, entrypoint.sh, and git-wrapper.sh

**Given** an image already exists with the current content hash
**When** the developer runs `sandbox build`
**Then** the build is skipped and a message indicates the image is up to date

**Given** the developer modifies config.yaml
**When** they run `sandbox build`
**Then** a new content hash is computed and the image is rebuilt

**Given** the base image is pinned to digest in Dockerfile.template
**When** the image is built
**Then** the same config always produces the same image regardless of when/where it's built (NFR12)

### Story 1.6: Sandbox Run with TTY and Lifecycle

As a developer,
I want `sandbox run` to launch my sandbox in interactive TTY mode with tini as PID 1,
So that I can interact with the agent and cleanly stop it with Ctrl+C.

**Acceptance Criteria:**

**Given** an image exists for the current config
**When** the developer runs `sandbox run`
**Then** the container launches in TTY mode (`-it`) with tini as the entrypoint and the configured agent as the foreground process

**Given** no image exists for the current config
**When** the developer runs `sandbox run`
**Then** the image is automatically built first, then the container launches (FR12)

**Given** a running sandbox session
**When** the developer presses Ctrl+C
**Then** tini forwards SIGTERM, the agent terminates, and the container exits cleanly with no orphaned containers or dangling networks (NFR14)

**Given** the config specifies `agent: claude-code`
**When** the sandbox starts
**Then** the entrypoint execs into `claude --dangerously-skip-permissions`

**Given** the config specifies `agent: gemini-cli`
**When** the sandbox starts
**Then** the entrypoint execs into `gemini`

## Epic 2: Project Integration -- Agent Can Work With Project Files and Git

An agent inside the sandbox can access mounted project files, use local git for version control, and work with common CLI tools and internet access -- a complete development workflow minus Docker and MCP.

### Story 2.1: Host Directory Mounts

As a developer,
I want to declare host directories in my config that get mounted into the sandbox,
So that the agent can read and write my project files.

**Acceptance Criteria:**

**Given** a config with mounts `[{source: ".", target: "/workspace"}]`
**When** the sandbox launches
**Then** the host directory is mounted at `/workspace` inside the container and the agent can read/write files there

**Given** mount source paths are relative (e.g., `.` or `../shared`)
**When** sandbox resolves mount paths
**Then** paths are resolved relative to the config file location, not the working directory

**Given** a config with multiple mount entries
**When** the sandbox launches
**Then** all declared mounts are applied with correct source and target paths

### Story 2.2: Secret Injection and Validation

As a developer,
I want to declare secret names in my config that are resolved from host environment variables at runtime,
So that the agent has the API keys it needs without them being baked into the image.

**Acceptance Criteria:**

**Given** a config declaring secrets `[ANTHROPIC_API_KEY]` and the variable is set in the host environment
**When** the sandbox launches
**Then** `ANTHROPIC_API_KEY` is available as an environment variable inside the container

**Given** a declared secret is not set in the host environment (not declared at all)
**When** the developer runs `sandbox run`
**Then** the script exits with code 4 and prints a clear error identifying the missing secret

**Given** a declared secret is set to an empty string in the host environment
**When** the sandbox launches
**Then** the empty value is passed through -- empty is a valid value (uses `${VAR+x}` check)

**Given** secrets are injected via `--env` flags
**When** the container is running
**Then** secrets are never written to the filesystem, image layers, or build cache (NFR5)

### Story 2.3: Agent Runtime with Project Files and BMAD Support

As a developer,
I want the agent to launch with full terminal access and be able to interact with mounted project files, install packages, and execute BMAD workflows,
So that the agent can do real development work inside the sandbox.

**Acceptance Criteria:**

**Given** a sandbox is running with a project directory mounted
**When** the agent executes
**Then** it has full terminal access and can read/write files in the mounted project directory

**Given** the agent needs additional packages at runtime
**When** it runs `apt-get install` or `npm install` or `pip install`
**Then** packages install successfully (the container has internet access and package managers)

**Given** a project with BMAD planning artifacts mounted
**When** the agent reads/writes to `_bmad-output/` or `docs/` directories
**Then** changes persist in the mounted host directory

### Story 2.4: Git and CLI Toolchain

As a developer,
I want the agent to have local git, internet access, and common CLI tools available inside the sandbox,
So that the agent can use standard development workflows.

**Acceptance Criteria:**

**Given** a sandbox is running
**When** the agent runs git operations (add, commit, log, diff, branch, checkout, merge, amend)
**Then** all local git operations work identically to standard git (NFR10)

**Given** a sandbox is running with internet access
**When** the agent runs `curl`, `wget`, or `dig`
**Then** the commands succeed and can reach external hosts

**Given** the agent needs to fetch documentation or packages from the internet
**When** it makes outbound HTTP/HTTPS requests
**Then** requests succeed (outbound internet is unrestricted)

## Epic 3: Isolation Boundaries -- Sandbox Enforces Security Constraints

The sandbox enforces hard boundaries: git push is blocked, host filesystem is restricted to declared mounts, host credentials are inaccessible, and all boundary violations return standard CLI error codes.

### Story 3.1: Git Push Blocking

As a developer,
I want git push to be blocked inside the sandbox with a standard error,
So that the agent cannot accidentally push code to remote repositories.

**Acceptance Criteria:**

**Given** a sandbox is running with a mounted git repository that has a remote configured
**When** the agent runs `git push` (or `git push origin main`, etc.)
**Then** the command fails with exit code 1 and prints "fatal: Authentication failed" to stderr

**Given** a sandbox is running
**When** the agent runs any other git command (add, commit, log, diff, branch, checkout, merge, amend, pull, fetch)
**Then** the command passes through to real git and behaves identically to standard git (NFR10)

**Given** the git wrapper is at `/usr/local/bin/git`
**When** the agent inspects the environment
**Then** the wrapper is transparent -- the agent does not know git is intercepted

### Story 3.2: Filesystem and Credential Isolation

As a developer,
I want the sandbox to restrict filesystem access to declared mounts only and prevent access to host credentials,
So that the agent cannot accidentally read or modify anything outside the project scope.

**Acceptance Criteria:**

**Given** a config declares only `[{source: ".", target: "/workspace"}]` as a mount
**When** the agent attempts to access paths outside `/workspace` (e.g., `/home/user/.ssh`, `/home/user/.aws`)
**Then** those paths do not exist inside the container -- no host home directory, SSH keys, or cloud credentials are accessible (NFR1, NFR6)

**Given** a sandbox is running
**When** the agent attempts any operation outside the declared boundaries
**Then** the system returns standard CLI error codes (file not found, permission denied) -- not sandbox-specific errors (FR37)

**Given** the container is launched
**When** inspecting the environment
**Then** only explicitly declared secrets are present as environment variables -- no host credentials leak through

## Epic 4: Inner Container Runtime -- Agent Can Build and Run Docker Services

An agent can build Docker images, run multi-service applications with Docker Compose, and reach application ports -- all inside the sandbox using Podman as a transparent Docker replacement.

### Story 4.1: Podman Installation and Docker Alias

As a developer,
I want Podman installed inside the sandbox with Docker aliased to it,
So that the agent can use familiar `docker` commands without knowing it's using Podman.

**Acceptance Criteria:**

**Given** a sandbox image is built
**When** the agent runs `docker --version` inside the container
**Then** Podman responds (via symlink at `/usr/local/bin/docker` -> `/usr/bin/podman`)

**Given** the sandbox image is built from Dockerfile.template
**When** inspecting the Podman installation
**Then** Podman 5.x is installed from the upstream Kubic repository, not the Ubuntu default apt repo

**Given** the sandbox container is running
**When** inspecting the container's runtime flags
**Then** the outer container was NOT launched with `--privileged` and does NOT mount the host Docker socket (NFR4)

### Story 4.2: Building and Running Inner Containers

As a developer,
I want the agent to build Docker images and run multi-service applications with Docker Compose inside the sandbox,
So that the agent can test full-stack applications as part of its development workflow.

**Acceptance Criteria:**

**Given** a sandbox is running with Podman available
**When** the agent runs `docker build -t myapp .` with a valid Dockerfile
**Then** the image builds successfully using Podman's rootless build

**Given** the agent has a docker-compose.yml defining multiple services
**When** the agent runs `docker compose up`
**Then** all services start and can communicate with each other over the private network

**Given** an inner container exposes port 3000
**When** the agent runs `curl localhost:3000` from the sandbox
**Then** the request reaches the inner container's application

**Given** inner containers are running
**When** attempting to reach their ports from outside the sandbox
**Then** the ports are not reachable -- inner containers are isolated to the private network bridge (NFR3)

### Story 4.3: Isolation Scripts Baked into Image

As a developer,
I want the git wrapper and entrypoint scripts baked into the sandbox image at build time,
So that isolation boundaries are always present regardless of what the agent does at runtime.

**Acceptance Criteria:**

**Given** a sandbox image is built
**When** inspecting the image contents
**Then** `scripts/git-wrapper.sh` is installed at `/usr/local/bin/git` and `scripts/entrypoint.sh` is the container's entrypoint

**Given** the agent attempts to modify or remove `/usr/local/bin/git`
**When** the file is owned by root and the agent runs as a non-root user
**Then** the modification fails with a permission denied error

**Given** the image is built
**When** inspecting the non-root user setup
**Then** a non-root user is configured for Podman rootless operation

## Epic 5: MCP Integration -- Agent Can Run Browser Tests via Playwright

An agent can use MCP servers (starting with Playwright) to run end-to-end browser tests against running applications inside the sandbox.

### Story 5.1: MCP Server Installation at Build Time

As a developer,
I want to configure which MCP servers are pre-installed in my sandbox image,
So that the agent has browser automation and other MCP capabilities available immediately.

**Acceptance Criteria:**

**Given** a config with `mcp: [playwright]`
**When** the sandbox image is built
**Then** the Playwright MCP package (`@playwright/mcp`) is installed in the image along with its browser dependencies

**Given** MCP servers are installed at build time
**When** inspecting the image
**Then** a manifest file at `/etc/sandbox/mcp-servers.json` lists all installed MCP servers and their startup commands

**Given** a config with no MCP servers specified
**When** the sandbox image is built
**Then** no MCP packages are installed and the manifest is empty

### Story 5.2: MCP Configuration Generation and Merge

As a developer,
I want the sandbox entrypoint to generate a `.mcp.json` in the workspace so the agent can discover and start MCP servers,
So that MCP integration works automatically without manual configuration.

**Acceptance Criteria:**

**Given** a sandbox starts with Playwright MCP installed
**When** the entrypoint runs
**Then** a `.mcp.json` is generated in the workspace root with the Playwright server configuration in standard MCP format

**Given** the mounted project already has a `.mcp.json` with its own MCP server entries
**When** the entrypoint runs
**Then** sandbox MCP servers are merged into the existing file, and if a server name conflicts, the project's version takes precedence

**Given** a running sandbox with `.mcp.json` in the workspace
**When** the agent starts an MCP server (e.g., Playwright) via the MCP protocol
**Then** the server starts successfully and is available for the agent to use (NFR9)

**Given** a sandbox with Playwright MCP and a running web application on an inner container
**When** the agent uses Playwright MCP to open a browser and navigate to the application
**Then** the browser renders the page and the agent can interact with it for E2E testing
