# Story 2.2: Development Toolchain Verification

Status: ready-for-dev

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want to verify that the sandbox provides a complete development environment with git, internet, CLI tools, and BMAD support,
So that the agent can do real development work without missing tools.

## Acceptance Criteria

1. **Given** a sandbox built from Epic 1 is running with a project directory mounted
   **When** the agent runs git operations (add, commit, log, diff, branch, checkout, merge, amend)
   **Then** all local git operations work identically to standard git (NFR10)

2. **Given** a sandbox is running with internet access
   **When** the agent runs `curl`, `wget`, or `dig`
   **Then** the commands succeed and can reach external hosts

3. **Given** the agent needs additional packages at runtime
   **When** it runs `apt-get install`, `npm install`, or `pip install`
   **Then** packages install successfully (container has internet and package managers)

4. **Given** a project with BMAD planning artifacts mounted
   **When** the agent reads/writes to `_bmad-output/` or `docs/` directories
   **Then** changes persist in the mounted host directory

## Tasks / Subtasks

- [ ] Task 1: Build a sandbox image with a test config (AC: all)
  - [ ] Create a temporary `.asbox/config.yaml` with `agent: claude-code`, at least one SDK enabled (NodeJS for `npm` + `pip` test requires Python SDK too), and a mount pointing to a temp project dir
  - [ ] Run `go run . build -f <config>` to produce the image
  - [ ] Verify the build succeeds without errors
- [ ] Task 2: Verify git operations inside the container (AC: #1)
  - [ ] Exec into the container (or use `docker exec`) and run: `git init /tmp/test-repo && cd /tmp/test-repo && git add . && git commit --allow-empty -m "test" && git log && git diff && git branch test-branch && git checkout test-branch && git merge main`
  - [ ] Confirm git wrapper at `/usr/local/bin/git` passes all non-push operations to `/usr/bin/git`
  - [ ] Confirm `git push` is blocked with `"fatal: git push is disabled inside the sandbox"` (this validates NFR10 boundary — push blocking is Epic 3, but the wrapper is already installed)
- [ ] Task 3: Verify internet access and CLI tools (AC: #2)
  - [ ] Run `curl -sI https://example.com` — confirm HTTP 200
  - [ ] Run `wget -q --spider https://example.com` — confirm success
  - [ ] Run `dig example.com` — confirm DNS resolution returns records
  - [ ] Verify `jq`, `unzip`, `zip`, `less`, `vim` are available
- [ ] Task 4: Verify runtime package installation (AC: #3)
  - [ ] Run `sudo apt-get update && sudo apt-get install -y tree` — confirm apt works
  - [ ] Run `npm install -g cowsay` (requires NodeJS SDK enabled) — confirm npm works
  - [ ] Run `pip install requests` (requires Python SDK enabled) — confirm pip works
- [ ] Task 5: Verify BMAD artifact persistence via mounts (AC: #4)
  - [ ] Requires Story 2.1 to be implemented first (mounts must work)
  - [ ] Mount a host directory containing `_bmad-output/` into the container
  - [ ] Create/modify a file inside `_bmad-output/` from within the container
  - [ ] Confirm the change is visible on the host filesystem
- [ ] Task 6: Fix any missing tools in `embed/Dockerfile.tmpl` if discovered
  - [ ] If any tool is missing or broken, update `embed/Dockerfile.tmpl` to add/fix it
  - [ ] Rebuild and re-verify after any fix
- [ ] Task 7: Document verification results in this story's Dev Agent Record
  - [ ] Record which tools were verified and their versions
  - [ ] Note any issues found and fixes applied

## Dev Notes

### This Is a Verification Story — Not an Implementation Story

**No new Go source files are created.** This story validates that the Dockerfile template from Epic 1 (Stories 1.3–1.5) produces a working development environment. If any tool is missing, the fix goes in `embed/Dockerfile.tmpl`, not in new Go packages.

### Dependency: Story 2.1 Must Be Complete First

AC #4 (BMAD artifact persistence) requires host directory mounts to work. Story 2.1 implements `internal/mount/mount.go` and wires `AssembleMounts()` into `cmd/run.go`. Without it, the `-v` mount flags are never populated in `docker run`.

If Story 2.1 is not yet implemented when this story begins, Tasks 1-4 and 6 can still proceed using `docker run -v` manually (outside of `asbox`), but Task 5 requires the full `asbox run` flow with mount support.

### Tools Installed by `embed/Dockerfile.tmpl`

**Always installed (base layer):**
`tini curl wget dnsutils git jq unzip zip less vim gosu ca-certificates gnupg lsb-release sudo build-essential`

**Conditional on SDKs:**
- NodeJS: `nodejs` via nodesource (version from config)
- Go: tarball from `go.dev` (version from config)
- Python: from deadsnakes PPA (version from config)

**Always installed (after SDKs):**
- `uidmap slirp4netns` (rootless Podman deps)
- `podman podman-docker aardvark-dns netavark` (Podman 5.x from Kubic/OBS)
- Docker Compose v2 standalone binary (latest from GitHub releases)
- Agent CLI: Claude Code (install script) or Gemini CLI (`npm install -g`)
- Playwright webkit deps (if NodeJS SDK enabled)

### Git Wrapper Behavior (NFR10)

`embed/git-wrapper.sh` is installed at `/usr/local/bin/git` (shadows `/usr/bin/git`). It scans args for `push` subcommand — blocks it with exit 1. All other operations pass through to `/usr/bin/git` via `exec`. The wrapper is transparent for local operations (add, commit, log, diff, branch, checkout, merge, amend).

**Note:** Full git push blocking validation belongs to Epic 3 (Story 3.1). This story only confirms the wrapper doesn't interfere with local operations.

### Container Entrypoint Sequence

`embed/entrypoint.sh` runs as root via Tini:
1. `align_uid_gid()` — matches sandbox user UID/GID to host
2. `chown_volumes()` — fixes permissions on named volumes
3. `merge_mcp_config()` — merges build-time and project MCP configs
4. `set_testcontainers_socket()` — configures Testcontainers socket path
5. `start_healthcheck_poller()` — background daemon, polls every 10s
6. `start_podman_socket()` — starts Podman API socket for inner Docker
7. `exec gosu sandbox <agent_cmd>` — drops to sandbox user

### How to Verify Without `asbox run`

If testing before Story 2.1 is complete, build and run manually:

```bash
# Build the image (asbox build works — it doesn't need mounts)
go run . build -f /path/to/.asbox/config.yaml

# Run manually with docker, mounting a test dir
docker run -it --rm \
  -v /tmp/test-project:/workspace \
  -e HOST_UID=$(id -u) -e HOST_GID=$(id -g) \
  asbox-<project>:latest \
  bash
```

This gives you a shell inside the container to run all verification commands.

### Package Manager Availability Depends on SDK Config

- `npm` is only available if `sdks.nodejs` is set in config
- `pip` is only available if `sdks.python` is set in config
- `go` CLI is only available if `sdks.go` is set in config
- `apt-get` is always available (Ubuntu base)

For comprehensive verification, use a config with all three SDKs enabled.

### What Success Looks Like

All of the following work inside the container:
- `git init/add/commit/log/diff/branch/checkout/merge/amend` — identical to standard git
- `curl`, `wget`, `dig` — reach external hosts
- `apt-get install` — installs packages
- `npm install` (with NodeJS SDK) — installs packages
- `pip install` (with Python SDK) — installs packages
- Files written to mounted directories persist on the host

If anything fails, the fix is in `embed/Dockerfile.tmpl` (missing package) or `embed/entrypoint.sh` (startup issue).

### Project Structure Notes

- No new files or packages created by this story
- Only file potentially modified: `embed/Dockerfile.tmpl` (if a tool is missing)
- The `internal/mount/` package is created by Story 2.1, not this story

### Previous Story Intelligence

**From Story 2.1 (ready-for-dev):**
- `internal/mount/mount.go` will implement `AssembleMounts()` returning `-v source:target` strings
- `internal/docker/run.go` already has `Mounts []string` field wired to `-v` flags — just needs population
- `cmd/run.go` will call `mount.AssembleMounts(cfg)` after config parse
- Secret validation (`buildEnvVars()`) already works — uses `os.LookupEnv()`, returns `SecretError` for missing secrets

**From Story 1-7 (sandbox run):**
- `cmd/build_helper.go` has shared `ensureBuild()` logic
- `internal/docker/run.go` handles TTY, signal handling (130/143 suppressed), `--rm --init` flags
- `RunOptions` struct has `Mounts`, `EnvVars`, `ImageRef`, `ContainerName`

**From Stories 1-3/1-4/1-5 (Dockerfile template):**
- Base packages (curl, wget, git, etc.) installed in Story 1-3
- SDK conditional blocks in Story 1-4
- Podman, Docker Compose, agent CLI, git wrapper, entrypoint in Story 1-5
- All architecture detection uses `$(dpkg --print-architecture)` or `$(uname -m)` — never hardcoded

### Git Intelligence

Recent commits follow `feat: implement story X-Y <description>` on `feat/go-rewrite` branch. Last commit (`f65b28e`) implemented Story 1-8. All 8 stories implemented cleanly with no CI failures.

Key files relevant to verification:
- `embed/Dockerfile.tmpl` — all tool installation layers
- `embed/entrypoint.sh` — runtime setup (UID/GID, Podman socket, healthcheck poller)
- `embed/git-wrapper.sh` — git push blocking, transparent passthrough for local ops
- `cmd/run.go` — run flow: parse → build → env → docker run

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Epic 2, Story 2.2]
- [Source: _bmad-output/planning-artifacts/architecture.md — Container Lifecycle, Embedded Scripts, Podman Configuration]
- [Source: _bmad-output/planning-artifacts/architecture.md — Requirements Mapping: FR23, FR24, FR25, FR22]
- [Source: _bmad-output/planning-artifacts/prd.md — FR22, FR23, FR24, FR25, NFR10]
- [Source: _bmad-output/implementation-artifacts/2-1-host-directory-mounts-and-secret-injection.md — Dev Notes]
- [Source: embed/Dockerfile.tmpl — tool installation layers]
- [Source: embed/git-wrapper.sh — push blocking logic]
- [Source: embed/entrypoint.sh — container startup sequence]

## Dev Agent Record

### Agent Model Used

{{agent_model_name_version}}

### Debug Log References

### Completion Notes List

### File List
