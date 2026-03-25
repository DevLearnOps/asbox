# Sandbox

Containerized development environment for AI coding agents. Sandbox gives Claude Code and Gemini CLI full development capability — git, Docker, SDKs, MCP servers — while isolating them from your host system. Assign a task, disconnect, and come back to working code.

## Table of Contents

- [Why Sandbox](#why-sandbox)
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

## Why Sandbox

Without Sandbox, delegating multi-step tasks to AI agents requires constant supervision — permission prompts, approval gates, and risk of host contamination. Sandbox constrains the environment itself so the agent operates freely within safe boundaries:

- **No permission prompts** — the agent runs with `--dangerously-skip-permissions` inside the container
- **No host side effects** — only explicitly declared mounts, secrets, and env vars are accessible
- **No accidental pushes** — git push is blocked at the wrapper level
- **Full dev environment** — Node.js, Python, Go, Docker Compose, MCP servers all available inside

## Prerequisites

| Dependency | Minimum Version | Notes |
|---|---|---|
| Bash | 4.0+ | macOS ships 3.2 — run `brew install bash` |
| Docker Engine or Podman | 20.10+ | Used to build and run the sandbox container |
| yq | 4.0+ | YAML parser — install via `brew install yq` or [GitHub releases](https://github.com/mikefarah/yq) |

Sandbox validates all dependencies at startup and exits with code `3` if anything is missing or too old.

## Quick Start

```bash
# 1. Clone and make available on PATH
git clone <repo-url> && ln -s "$(pwd)/sandbox.sh" /usr/local/bin/sandbox

# 2. Navigate to your project and initialize
cd ~/my-project
sandbox init

# 3. Edit .sandbox/config.yaml — set your agent, SDKs, secrets

# 4. Build and run
sandbox run
```

`sandbox run` auto-builds the image if one doesn't exist yet. You only need an explicit `sandbox build` when you want to pre-build or verify the image without launching a session.

## CLI Reference

```
sandbox [command] [-f <config-path>]
```

### Commands

**`sandbox init`** — Generate a starter config file.

```bash
sandbox init                           # Creates .sandbox/config.yaml
sandbox init -f configs/gpu.yaml       # Creates config at custom path
```

Fails with exit code `1` if the config file already exists.

**`sandbox build`** — Build the container image from config.

```bash
sandbox build                          # Uses .sandbox/config.yaml
sandbox build -f configs/gpu.yaml      # Uses custom config
```

Generates a Dockerfile from `Dockerfile.template`, computes a content hash, and tags the image as `sandbox-<project>:<hash>`. Skips the build entirely if an image with the same hash already exists.

**`sandbox run`** — Launch an interactive sandbox session.

```bash
sandbox run                            # Uses .sandbox/config.yaml
sandbox run -f configs/gpu.yaml        # Uses custom config
```

Validates that all declared secrets exist in your host environment, auto-builds if needed, then starts the container in TTY mode with the configured agent as the foreground process. Press `Ctrl+C` to stop cleanly — Tini handles signal forwarding with no orphaned processes.

### Global Flags

| Flag | Description |
|---|---|
| `-f <path>` | Override config file path (default: `.sandbox/config.yaml`) |
| `--help` | Display usage information |

The `-f` flag works before or after the command: both `sandbox -f path run` and `sandbox run -f path` are valid.

## Configuration

`sandbox init` generates `.sandbox/config.yaml` with sensible defaults and inline comments. Here is the full schema:

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
```

### Key Behaviors

- **Mount paths resolve relative to the config file**, not your working directory. This makes configs portable across machines.
- **Secrets are names only** — values are never written to config, image, or build cache. Sandbox reads them from your host environment at `run` time and injects via `--env` flags.
- **Env values must be single-line** — multiline YAML values break the key/value parser.
- **Empty lists are valid** — omitting `sdks`, `packages`, or `secrets` entirely is fine.

## Usage Patterns

### Pattern 1: Fire-and-Forget Feature Work

The primary workflow. Mount your project, hand the agent a task, walk away.

```bash
# Config: mount project root, inject API key
sandbox run
# Inside sandbox: agent has full git, Node.js, tests, etc.
# Agent commits to local branch — you review the diff later
```

The agent works in a local git clone inside the container. All commits stay local since `git push` is blocked. Review the branch after the session.

### Pattern 2: Multiple Config Profiles

Maintain different configs for different workloads in the same project.

```bash
sandbox init -f .sandbox/frontend.yaml    # Node.js + Playwright MCP
sandbox init -f .sandbox/backend.yaml     # Python + Go, no MCP
sandbox init -f .sandbox/full-stack.yaml  # Everything

sandbox run -f .sandbox/backend.yaml
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

Commit `.sandbox/config.yaml` to your repo. New team members get a working sandbox environment with one command.

```bash
git clone <repo> && cd <repo>
sandbox run    # Auto-builds from committed config
```

The content-hash cache means everyone with the same config gets the same image, regardless of when they build.

## Isolation Model

Sandbox protects against **accidental** agent mistakes (hallucinations, runaway commands), not deliberate adversarial behavior. The threat model assumes agents are well-intentioned but fallible.

### Boundaries

| Boundary | Mechanism | What Happens |
|---|---|---|
| **Git push** | Wrapper at `/usr/local/bin/git` intercepts `push` | Returns `fatal: Authentication failed` (exit 1) — agent adapts gracefully |
| **Filesystem** | Only declared mounts are visible | No access to `~/.ssh`, `~/.aws`, or host home directory |
| **Credentials** | Only declared secrets injected via `--env` | No implicit access to host env vars, tokens, or keys |
| **Network (inner containers)** | Podman rootless networking | Inner container ports unreachable from host; internet access preserved |
| **Docker daemon** | Podman replaces Docker | No host Docker socket mount, no `--privileged`, fully daemonless |

### Tamper Resistance

Isolation scripts (`git-wrapper.sh`, `entrypoint.sh`) are baked into the image owned by root. The non-root sandbox user cannot modify them at runtime.

## Build Caching

Sandbox computes a content hash from exactly four files:

1. `.sandbox/config.yaml` (or your `-f` path)
2. `Dockerfile.template`
3. `scripts/entrypoint.sh`
4. `scripts/git-wrapper.sh`

Images are tagged `sandbox-<project>:<hash>`. If an image with that hash already exists, the build is skipped entirely. Changes to `sandbox.sh`, `README.md`, or `templates/` do **not** trigger rebuilds.

The base image (Ubuntu 24.04 LTS) is pinned to a specific digest, ensuring identical builds regardless of when or where you build.

## Testing

Run the test suite:

```bash
bash tests/test_sandbox.sh
```

Tests use a custom TAP-like runner with no external dependencies. The suite covers:

- CLI help output and flag parsing
- Dependency validation (Docker, yq, Bash version)
- Error handling and exit codes
- `sandbox init` config generation
- Config parsing for all YAML fields
- Build mock execution and caching behavior
- Output stream correctness (stdout vs stderr)
- Script quality (shebang, `set -euo pipefail`, no `eval`)
- Filesystem and credential isolation verification
- Git push blocking
- Tamper resistance of isolation scripts

Exit code `0` means all tests pass.

## Project Structure

```
sandbox.sh                  # CLI entry point (all commands)
Dockerfile.template         # Template with conditional blocks and placeholders
scripts/
  entrypoint.sh             # Container startup (user switch, Podman init, agent exec)
  git-wrapper.sh            # Git push interceptor
templates/
  config.yaml               # Starter config generated by `sandbox init`
tests/
  test_sandbox.sh           # Full test suite
.sandbox/
  config.yaml               # Your project-specific config (git-committed)
```

## Exit Codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | General error (missing config, invalid values) |
| `2` | Usage error (unknown command, missing flag argument) |
| `3` | Dependency error (missing Docker, yq, or Bash 4+) |
| `4` | Secret validation error (declared secret not found in host env) |
