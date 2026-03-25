# Sprint Change Proposal: Epic 7 -- Sandbox Runtime Hardening

**Date:** 2026-03-25
**Author:** John (PM Agent)
**Triggered by:** Real agent usage after Epics 1-6 completion
**Change scope:** Minor -- Direct implementation by dev team
**Mode:** Batch review, approved by Manuel

---

## 1. Issue Summary

Four operational gaps discovered during actual agent usage inside the sandbox. Three are blocking (Playwright, Docker Compose, .claude config) and one is planned maintenance (Claude CLI install method). All relate to the sandbox runtime environment not fully delivering capabilities that the PRD requires and the architecture supports.

### Issues

| # | Issue | Severity | Category |
|---|-------|----------|----------|
| 1 | Playwright missing system dependencies -- agent cannot run browser tests | Blocking | Implementation gap (Epic 5) |
| 2 | Docker Compose `docker compose` subcommand fails with "docker daemon not running" | Blocking | Implementation gap (Epic 4) |
| 3 | `.claude` config mount not picked up by agent -- re-auth required every session | Blocking | Bug in Epic 6 implementation |
| 4 | Claude CLI installed via deprecated npm package -- needs migration to official install script | Planned | Upstream deprecation |

---

## 2. Impact Analysis

### Epic Impact

- **Epics 1-5:** No changes needed (remain complete)
- **Epic 6 (host-agent-config):** Bug fix needed (mounted config not picked up by agent)
- **New Epic 7** added to the backlog covering all 4 issues

### Artifact Changes

| Artifact | Change Needed |
|----------|---------------|
| `Dockerfile.template` | Modified for stories 7.1 (Playwright deps), 7.2 (Compose plugin), 7.3 (Claude CLI install) |
| `scripts/entrypoint.sh` | Potentially modified for story 7.4 (pending investigation) |
| `tests/test_sandbox.sh` | New integration tests for Docker Compose |
| PRD | No changes -- capabilities already required |
| Architecture | No changes -- decisions remain sound |
| `templates/config.yaml` | No changes |

### PRD Alignment

All 4 issues map to existing PRD requirements:

- FR26-FR28: Docker/Docker Compose inside sandbox (Story 7.2)
- FR29: Playwright tests via MCP (Story 7.1)
- FR18: Claude Code agent launches correctly (Story 7.3)
- NFR7: Podman compatible (Story 7.2 root cause)

MVP scope is unchanged. These are fulfillment bugs, not scope changes.

---

## 3. Recommended Approach

**Direct Adjustment** -- add Epic 7 with 4 stories to the existing plan.

**Rationale:** These are implementation fixes to already-required capabilities, not new features. The existing architecture decisions remain sound. Each story is independently implementable and testable. No rollback needed -- existing code is a foundation, not broken.

**Effort:** Low-Medium
**Risk:** Low
**Timeline impact:** Minimal -- focused, well-scoped fixes

---

## 4. Detailed Change Proposals

### Epic 7: Sandbox Runtime Hardening

An agent operating inside the sandbox can use all runtime capabilities (Playwright, Docker Compose, Claude CLI auth) without encountering missing dependencies, broken commands, or authentication failures.

**FRs covered:** FR18, FR27, FR29 + Epic 6 bug fix

---

### Story 7.1: Fix Playwright System Dependencies

As a developer,
I want Playwright to have all required system libraries pre-installed in the sandbox image,
So that the agent can run browser-based E2E tests without missing library errors.

**Acceptance Criteria:**

**Given** a sandbox image is built with `mcp: [playwright]` configured
**When** the agent runs Playwright tests inside the sandbox
**Then** the browser launches without missing library errors

**Given** the Dockerfile.template Playwright block
**When** `npx playwright install --with-deps chromium` runs at build time
**Then** all required system libraries are present (verify the list: libnspr4, libnss3, libatk1.0-0t64, libatk-bridge2.0-0t64, libdbus-1-3, libcups2t64, libxcb1, libxkbcommon0, libatspi2.0-0t64, libx11-6, libxcomposite1, libxdamage1, libxext6, libxfixes3, libxrandr2, libgbm1, libcairo2, libpango-1.0-0, libasound2t64)

**Implementation Notes:**
- First, verify which libs are actually missing by building and running `ldd` against the Chromium binary
- The `--with-deps` flag should install these automatically -- investigate why it isn't
- If `--with-deps` is insufficient, add explicit `apt-get install` for the missing packages in the Dockerfile.template before or after the Playwright install
- The package names in the user's list have Ubuntu 24.04 `t64` suffixes -- verify exact names

**FRs:** FR29

---

### Story 7.2: Fix Docker Compose Plugin Registration for Podman

As a developer,
I want `docker compose` (as a subcommand) to work inside the sandbox,
So that the agent can run multi-service applications using standard Docker Compose commands.

**Acceptance Criteria:**

**Given** a sandbox is running
**When** the agent runs `docker compose version`
**Then** the Docker Compose version is displayed (no "daemon not running" error)

**Given** a sandbox with a valid docker-compose.yml
**When** the agent runs `docker compose up -d`
**Then** the services start successfully using Podman as the backend

**Given** `docker compose` works
**When** the agent runs the standalone `docker-compose` command
**Then** that also works (backwards compatibility)

**Integration Tests Required:**
- Test that `docker compose version` returns successfully inside the sandbox
- Test that `docker compose up` with a simple service (e.g., nginx) starts and is reachable via curl
- Test that `docker compose down` cleans up properly

**Implementation Notes:**
- Root cause: Docker Compose v2 is installed at `/usr/local/bin/docker-compose` (standalone). The `docker compose` subcommand (note: space, not hyphen) requires the binary to be registered as a Docker CLI plugin.
- Fix: In Dockerfile.template, after downloading the compose binary, symlink to Docker CLI plugin path: `mkdir -p /usr/local/lib/docker/cli-plugins && ln -s /usr/local/bin/docker-compose /usr/local/lib/docker/cli-plugins/docker-compose`
- Alternatively, podman-docker may look for plugins at a different path -- verify with `podman info`
- The "docker daemon not running" error likely comes from docker-compose trying to contact a Docker daemon socket. Need to ensure it routes through podman's socket or uses podman-compose.

**FRs:** FR27

---

### Story 7.3: Migrate Claude CLI Installation to Official Install Script

As a developer,
I want Claude Code installed via the official install script instead of the deprecated npm package,
So that the sandbox uses the supported installation method and avoids future breakage.

**Acceptance Criteria:**

**Given** a sandbox image is built with `agent: claude-code`
**When** the agent runs `claude --version` inside the sandbox
**Then** Claude Code responds with its version (installed via official script)

**Given** the Dockerfile.template
**When** the Claude Code install block is processed
**Then** it uses the official install script (e.g., `curl -fsSL https://claude.ai/install.sh | sh`) instead of `npm install -g @anthropic-ai/claude-code`

**Given** Claude Code is installed via the official script
**When** the agent launches with `claude --dangerously-skip-permissions`
**Then** the agent starts and operates normally

**Implementation Notes:**
- Current: `Dockerfile.template:68` -- `RUN npm install -g @anthropic-ai/claude-code`
- New: Replace with the official curl|sh install script
- Verify the exact install URL -- the agent reported it but we need to confirm
- This may change where `claude` is installed (PATH may differ from npm global install path)
- Ensure the binary is available to both root (build time) and sandbox user (runtime)
- Node.js may still be a dependency even with the official installer -- verify
- This story is not urgent but should be done to avoid future breakage

**FRs:** FR18

---

### Story 7.4: Investigate and Fix Host Agent Config Mount (.claude)

As a developer,
I want the `host_agent_config` mount to work correctly so the agent picks up my Claude authentication,
So that I don't have to re-authenticate every time I start a sandbox session.

**Acceptance Criteria:**

**Given** a config with `host_agent_config: {source: "~/.claude", target: "/home/sandbox/.claude"}`
**When** the sandbox starts and the agent launches
**Then** Claude Code recognizes the existing authentication and does not prompt for login

**Given** the host `~/.claude` directory contains valid OAuth tokens
**When** the agent runs inside the sandbox
**Then** the tokens are accessible and the agent can make API calls

**Given** the UID alignment works correctly
**When** the sandbox user reads files from the mounted `.claude` directory
**Then** the files are readable and writable by the sandbox user

**Implementation Notes -- Investigation Required:**
This story requires root cause analysis before implementation. Possible causes:

1. **HOME mismatch**: Claude CLI may look for config at `$HOME/.claude`, but `HOME` might not be `/home/sandbox` when running via `runuser`. Check what `HOME` resolves to inside the container.
2. **Path structure mismatch**: Claude CLI may expect a specific directory structure inside `.claude/` that differs between versions. Compare host `.claude/` structure with what the CLI expects.
3. **UID alignment not triggered**: If `HOST_UID` matches the sandbox user's UID (1000), the alignment is skipped -- but the GID might still differ, causing group permission issues.
4. **Symlink resolution**: If `.claude` contains symlinks, the bind mount may not resolve them correctly across the host/container boundary.
5. **File locking**: Claude CLI may use file locks that don't work correctly over bind mounts (especially on macOS with Docker Desktop's VirtioFS).
6. **Claude CLI version mismatch**: If Story 7.3 (install method change) is implemented first, the config format expected by the new version may differ.

**Recommended investigation steps:**
- Start a sandbox with the mount, exec into it, and check: `ls -la /home/sandbox/.claude/`, `echo $HOME`, `id`, `whoami`
- Run `claude --version` and check for any config-related errors in verbose mode
- Compare the `.claude` directory contents on host vs inside container

**FRs:** Related to Epic 6 tech spec (host-agent-config-inheritance)

---

## 5. Implementation Handoff

### Scope Classification

**Minor** -- direct implementation by development team. No backlog reorganization or strategic replan needed.

### Recommended Sequence

1. **Story 7.4 first** (investigation) -- understanding the .claude issue may inform Story 7.3
2. **Story 7.1** (Playwright deps) -- quickest win, unblocks browser testing
3. **Story 7.2** (Docker Compose) -- unblocks multi-service testing
4. **Story 7.3** (Claude CLI install) -- not urgent, can follow

### Handoff

- **Architect:** Creates story files from this proposal
- **Dev agent:** Implements each story following existing patterns (Dockerfile.template changes, test patterns from `tests/test_sandbox.sh`)

### Success Criteria

An agent can complete a full development workflow inside the sandbox:
- Write code and commit locally
- Build Docker images
- Run `docker compose up` for multi-service apps
- Run Playwright E2E tests against running services
- All without re-authenticating or hitting missing dependency errors

### Dependencies

- Story 7.4 investigation results may affect Story 7.3 implementation
- Stories 7.1, 7.2, 7.3 are independent of each other
- All stories modify `Dockerfile.template` -- coordinate to avoid merge conflicts
