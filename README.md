# asbox

Containerized development environment for AI coding agents. asbox (Agent-SandBox) gives Claude Code and Gemini CLI full development capability — git, Docker, SDKs, MCP servers — while isolating them from your host system. Assign a task, disconnect, and come back to working code.

## Table of Contents

- [Why asbox](#why-asbox)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [CLI Reference](#cli-reference)
- [Configuration](#configuration)
- [Usage Patterns](#usage-patterns)
- [Isolation Model](#isolation-model)
- [Build Caching](#build-caching)
- [Testing](#testing)
- [Project Structure](#project-structure)
- [Exit Codes](#exit-codes)

## Why asbox

Without asbox, delegating multi-step tasks to AI agents requires constant supervision — permission prompts, approval gates, and risk of host contamination. asbox constrains the environment itself so the agent operates freely within safe boundaries:

- **No permission prompts** — the agent runs with `--dangerously-skip-permissions` inside the container
- **No host side effects** — only explicitly declared mounts, secrets, and env vars are accessible
- **No accidental pushes** — git push is blocked at the wrapper level
- **Full dev environment** — Node.js, Python, Go, Docker Compose, MCP servers all available inside

## Prerequisites

| Dependency | Minimum Version | Notes |
|---|---|---|
| Docker Engine or Podman | 20.10+ | Used to build and run the asbox container |

asbox is a single Go binary with no other runtime dependencies. It validates Docker or Podman availability at startup and exits with code `3` if missing or too old.

## Quick Start

```bash
# 1. Install via go install
go install github.com/mcastellin/asbox@latest

# Or build from source
git clone <repo-url> && cd asbox
make install    # Installs to ~/.local/bin/asbox

# 2. Navigate to your project and initialize
cd ~/my-project
asbox init

# 3. Edit .asbox/config.yaml — set your agent, SDKs, secrets

# 4. Build and run
asbox run
```

`asbox run` auto-builds the image if one doesn't exist yet. You only need an explicit `asbox build` when you want to pre-build or verify the image without launching a session.

## CLI Reference

```
asbox [command] [-f <config-path>]
```

### Commands

**`asbox init`** — Generate a starter config file.

```bash
asbox init                           # Creates .asbox/config.yaml
asbox init -f configs/gpu.yaml       # Creates config at custom path
```

Fails with exit code `1` if the config file already exists.

**`asbox build`** — Build the container image from config.

```bash
asbox build                          # Uses .asbox/config.yaml
asbox build -f configs/gpu.yaml      # Uses custom config
asbox build --no-cache               # Force full rebuild, bypass cache
```

Renders the Dockerfile template, computes a content hash, and tags the image as `asbox-<project>:<hash>`. Skips the build entirely if an image with the same hash already exists. Use `--no-cache` to force a complete rebuild, bypassing both the content-hash check and Docker layer cache.

**`asbox run`** — Launch an interactive sandbox session.

```bash
asbox run                            # Uses .asbox/config.yaml
asbox run -f configs/gpu.yaml        # Uses custom config
asbox run --no-cache                 # Force rebuild before running
asbox run --fetch                    # Fetch origin refs for mounted repos before launch
```

Validates that all declared secrets exist in your host environment, auto-builds if needed, then starts the container in TTY mode with the configured agent as the foreground process. Press `Ctrl+C` to stop cleanly — Tini handles signal forwarding with no orphaned processes.

### Global Flags

| Flag | Description |
|---|---|
| `-f, --file <path>` | Override config file path (default: `.asbox/config.yaml`) |
| `-h, --help` | Display usage information |
| `-v, --version` | Display version |

## Configuration

`asbox init` generates `.asbox/config.yaml` with sensible defaults and inline comments. Here is the full schema:

```yaml
# Required — which agent runs inside the sandbox
agent: claude-code                # "claude-code" or "gemini-cli"

# Optional — SDK versions to install
sdks:
  nodejs: "22"
  python: "3.12"
  go: "1.22"

# Optional — system packages (apt-get)
packages:
  - build-essential
  - curl
  - wget
  - git
  - jq

# Optional — host directories to mount
mounts:
  - source: "."                   # Relative to config file directory
    target: "/workspace"          # Absolute path inside container

# Optional — auto-detect package.json and isolate node_modules via named volumes
# auto_isolate_deps: true

# Optional — host env vars to inject (names only, values read at runtime)
secrets:
  - ANTHROPIC_API_KEY
  - GITHUB_TOKEN

# Optional — non-secret env vars
env:
  NODE_ENV: development

# Optional — MCP servers to pre-install
mcp:
  - playwright                    # Installs @playwright/mcp with browser deps

# Optional — share host agent config (OAuth/SSO tokens)
# host_agent_config:
#   source: "~/.claude"
#   target: "/opt/claude-config"

# Optional — BMAD multi-repo mounts for cross-repo workflows
# bmad_repos:
#   - ../other-repo
#   - ../shared-libs
```

### Key Behaviors

- **Mount paths resolve relative to the config file**, not your working directory. This makes configs portable across machines.
- **Secrets are names only** — values are never written to config, image, or build cache. asbox reads them from your host environment at `run` time and injects via `--env` flags.
- **Env values must be single-line** — multiline YAML values break the key/value parser.
- **Empty lists are valid** — omitting `sdks`, `packages`, or `secrets` entirely is fine.

## Usage Patterns

### Pattern 1: Fire-and-Forget Feature Work

The primary workflow. Mount your project, hand the agent a task, walk away.

```bash
# Config: mount project root, inject API key
asbox run
# Inside sandbox: agent has full git, Node.js, tests, etc.
# Agent commits to local branch — you review the diff later
```

The agent works in a local git clone inside the container. All commits stay local since `git push` is blocked. Review the branch after the session.

### Pattern 2: Multiple Config Profiles

Maintain different configs for different workloads in the same project.

```bash
asbox init -f .asbox/frontend.yaml    # Node.js + Playwright MCP
asbox init -f .asbox/backend.yaml     # Python + Go, no MCP
asbox init -f .asbox/full-stack.yaml  # Everything

asbox run -f .asbox/backend.yaml
```

Each config produces a separately cached image, so switching profiles doesn't trigger unnecessary rebuilds.

### Pattern 3: BMAD Planning + Implementation

Mount the BMAD output folder so the agent can read planning artifacts and write implementation artifacts during story execution.

```yaml
mounts:
  - source: "."
    target: "/workspace"
  - source: "./_bmad-output"
    target: "/workspace/_bmad-output"
```

The agent reads PRDs, architecture docs, and story specs, then implements with full context.

### Pattern 4: Inner Container Development

The sandbox includes Podman (rootless) and Docker Compose v2. Agents can build and run containers inside the sandbox without access to your host Docker daemon.

```yaml
# No special config needed — Podman is always available
agent: claude-code
sdks:
  nodejs: "22"
```

Inside the sandbox, `docker` is aliased to `podman`. The agent runs `docker compose up`, integration tests against local services, and tears everything down — all isolated within the sandbox's own network namespace.

### Pattern 5: MCP-Enabled Browser Automation

Pre-install the Playwright MCP server so the agent can interact with web pages.

```yaml
mcp:
  - playwright
```

At container startup, the entrypoint generates `.mcp.json` in the workspace root from a build-time manifest. If your project already has an `.mcp.json`, the two are merged — your project config wins on key conflicts.

### Pattern 6: Team Onboarding

Commit `.asbox/config.yaml` to your repo. New team members get a working sandbox environment with one command.

```bash
git clone <repo> && cd <repo>
asbox run    # Auto-builds from committed config
```

The content-hash cache means everyone with the same config gets the same image, regardless of when they build.

## Isolation Model

asbox protects against **accidental** agent mistakes (hallucinations, runaway commands), not deliberate adversarial behavior. The threat model assumes agents are well-intentioned but fallible.

### Boundaries

| Boundary | Mechanism | What Happens |
|---|---|---|
| **Git push** | Wrapper at `/usr/local/bin/git` intercepts `push` | Returns `fatal: Authentication failed` (exit 1) — agent adapts gracefully |
| **Filesystem** | Only declared mounts are visible | No access to `~/.ssh`, `~/.aws`, or host home directory |
| **Credentials** | Only declared secrets injected via `--env` | No implicit access to host env vars, tokens, or keys |
| **Network (inner containers)** | Podman rootless networking | Inner container ports unreachable from host; internet access preserved |
| **Docker daemon** | Podman replaces Docker | No host Docker socket mount, no `--privileged`, fully daemonless |

### Tamper Resistance

Isolation scripts (`git-wrapper.sh`, `entrypoint.sh`) are embedded in the Go binary at build time and baked into the image owned by root. The non-root sandbox user cannot modify them at runtime.

## Build Caching

asbox computes a content hash from:

- `.asbox/config.yaml` (or your `-f` path)
- The rendered Dockerfile (generated from the embedded template)
- Embedded container scripts (entrypoint, git-wrapper, healthcheck-poller)

Images are tagged `asbox-<project>:<hash>`. If an image with that hash already exists, the build is skipped entirely. Changes to the Go binary itself, README, or planning artifacts do **not** trigger rebuilds — only changes that affect the container image content matter.

Use `asbox build --no-cache` or `asbox run --no-cache` to force a complete rebuild, bypassing both the content-hash check and Docker layer cache.

The base image (Ubuntu 24.04 LTS) is pinned to a specific digest, ensuring identical builds regardless of when or where you build.

## Testing

```bash
# Run all tests (unit + integration)
make test               # or: go test ./...

# Unit tests only
make test-unit          # or: go test -short ./...

# Integration tests only (requires Docker)
make test-integration   # or: go test -v ./integration/...

# CI pipeline (vet + unit + integration)
make test-ci
```

Unit tests cover configuration parsing, template rendering, hash computation, and CLI flag handling. Integration tests use real containers to validate the full lifecycle: image build, container startup, mount isolation, git push blocking, inner container orchestration, MCP configuration, and dependency isolation.

## Project Structure

```
asbox/
├── main.go                     # Entry point
├── go.mod / go.sum             # Module definition
├── Makefile                    # Build targets (build, install, test, test-integration)
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
│   ├── agent-instructions.md.tmpl  # Agent instruction template
│   └── config.yaml             # Starter config for asbox init
├── integration/                # Integration tests (Go testing + testcontainers-go)
└── .asbox/
    config.yaml                 # Your project-specific config
```

## Exit Codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | General error (missing config, invalid values) |
| `2` | Usage error (unknown command, missing flag argument) |
| `3` | Dependency error (missing Docker) |
| `4` | Secret validation error (declared secret not found in host env) |
