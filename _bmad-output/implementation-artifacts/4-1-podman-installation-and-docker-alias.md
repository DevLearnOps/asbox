# Story 4.1: Podman Installation and Docker Alias

Status: review

## Story

As a developer,
I want Podman installed inside the sandbox with Docker aliased to it,
so that the agent can use familiar `docker` commands without knowing it's using Podman.

## Acceptance Criteria

1. **Given** a sandbox image is built
   **When** the agent runs `docker --version` inside the container
   **Then** Podman responds (via the `podman-docker` package or symlink at `/usr/local/bin/docker` -> `/usr/bin/podman`)

2. **Given** the sandbox image is built from Dockerfile.template
   **When** inspecting the Podman installation
   **Then** Podman 5.x is installed from the upstream Kubic/OBS repository, not the Ubuntu default apt repo (which ships 4.9.x)

3. **Given** the sandbox container is running
   **When** inspecting the container's runtime flags
   **Then** the outer container was NOT launched with `--privileged` and does NOT mount the host Docker socket (NFR4)

## Tasks / Subtasks

- [x] Task 1: Add Podman installation to Dockerfile.template (AC: #1, #2)
  - [x] 1.1 Add the upstream Kubic/OBS repository for Ubuntu 24.04 (deb repository + GPG key)
  - [x] 1.2 Install podman from the upstream repository (targeting 5.x)
  - [x] 1.3 Install `podman-docker` package (provides `/usr/bin/docker` -> `/usr/bin/podman` alias and suppresses docker CLI warnings)
  - [x] 1.4 Install rootless dependencies: `uidmap`, `fuse-overlayfs` (overlay storage driver for rootless mode)
  - [x] 1.5 Create `/etc/containers/nodocker` to silence "Emulate Docker CLI" warnings
- [x] Task 2: Configure rootless Podman for the sandbox user (AC: #1)
  - [x] 2.1 Configure `/etc/subuid` and `/etc/subgid` entries for the `sandbox` user (e.g., `sandbox:100000:65536`)
  - [x] 2.2 Verify the existing non-root `sandbox` user setup (lines 44-46 of current Dockerfile.template) is compatible with Podman rootless
  - [x] 2.3 Configure Podman storage for rootless mode if needed (containers/storage.conf with `driver = "overlay"`)
- [x] Task 3: Add Docker Compose compatibility (AC: #1)
  - [x] 3.1 Install `podman-compose` or ensure `podman compose` subcommand is available
  - [x] 3.2 If needed, create symlink for `docker-compose` -> `podman compose` wrapper
- [x] Task 4: Write tests verifying Podman installation in generated Dockerfile (AC: #1, #2)
  - [x] 4.1 Test: generated Dockerfile contains Podman repository setup commands
  - [x] 4.2 Test: generated Dockerfile contains `podman` package installation
  - [x] 4.3 Test: generated Dockerfile contains docker alias setup (podman-docker or symlink)
  - [x] 4.4 Test: generated Dockerfile contains rootless configuration (subuid/subgid)
  - [x] 4.5 Test: generated Dockerfile contains `uidmap` dependency
- [x] Task 5: Verify AC #3 is already covered (AC: #3)
  - [x] 5.1 Confirm Story 3-2's existing tests already verify no `--privileged` and no Docker socket mount in `cmd_run()` output
  - [x] 5.2 If not adequately covered, add additional assertions

## Dev Notes

### What Changes

**Dockerfile.template** is the ONLY source file that needs modification. Podman is always installed (not conditional on config), so NO template markers (`{{IF_...}}`) are needed. Add the installation block directly as regular Dockerfile instructions.

**tests/test_sandbox.sh** gets new test assertions verifying the generated Dockerfile content.

**No changes needed to:** `sandbox.sh`, `scripts/entrypoint.sh`, `scripts/git-wrapper.sh`, `templates/config.yaml`. The content-hash cache in `sandbox.sh` already includes `Dockerfile.template`, so changes automatically trigger rebuilds.

### Current Dockerfile.template Structure (50 lines)

```
Line 1:     FROM ubuntu:24.04@sha256:... (base image pinned to digest)
Lines 5-7:  tini installation (PID 1 signal handler)
Lines 10-17: Common tools (curl, wget, git, dnsutils, ca-certificates, gnupg)
Line 20:    {{PACKAGES}} placeholder (user-specified system packages)
Lines 22-26: {{IF_NODE}} conditional SDK block
Lines 28-32: {{IF_PYTHON}} conditional SDK block
Lines 34-37: {{IF_GO}} conditional SDK block
Lines 39-42: Git wrapper deployment (/usr/local/bin/git)
Lines 44-46: Non-root sandbox user creation
Lines 48-49: ENTRYPOINT [tini] + CMD [entrypoint.sh]
```

**Insert Podman installation AFTER common tools (after line 17) and BEFORE the {{PACKAGES}} placeholder.** This ensures Podman's repository dependencies (gnupg, ca-certificates, curl) are already available.

### Podman Installation Approach

**Primary approach (upstream Kubic/OBS repository for Podman 5.x):**

```dockerfile
# Podman 5.x from upstream repository (not Ubuntu default 4.9.x)
RUN curl -fsSL "https://download.opensuse.org/repositories/devel:kubic:libcontainers:unstable/xUbuntu_24.04/Release.key" \
      | gpg --dearmor -o /etc/apt/keyrings/podman-kubic.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/podman-kubic.gpg] \
      https://download.opensuse.org/repositories/devel:kubic:libcontainers:unstable/xUbuntu_24.04/ /" \
      > /etc/apt/sources.list.d/podman-kubic.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends podman podman-docker uidmap fuse-overlayfs && \
    rm -rf /var/lib/apt/lists/*
```

**Important:** The Kubic "unstable" repo contains builds from the main branch. If this repo does not have packages for Ubuntu 24.04 or has broken packages at build time, fall back to:
1. The Kubic "stable" repo: `devel:kubic:libcontainers:stable`
2. Ubuntu's default apt Podman (4.9.x) -- still satisfies Docker CLI compatibility, just older
3. Document which approach was used and any version constraints

**Dev agent MUST verify the chosen repo actually provides packages** by checking the URL is reachable and the package version is 5.x.

### Rootless Podman Configuration

The `sandbox` user (created at Dockerfile.template lines 44-46) needs rootless container support:

```dockerfile
# Configure rootless Podman for sandbox user
RUN echo "sandbox:100000:65536" >> /etc/subuid && \
    echo "sandbox:100000:65536" >> /etc/subgid
```

**Storage configuration** -- Podman rootless defaults to overlay with fuse-overlayfs, which should work with the `fuse-overlayfs` package installed. If issues arise, explicitly configure:

```
# /etc/containers/storage.conf
[storage]
driver = "overlay"
[storage.options.overlay]
mount_program = "/usr/bin/fuse-overlayfs"
```

### Docker Alias Mechanism

The `podman-docker` package (from the upstream repo) provides:
- `/usr/bin/docker` symlink pointing to `/usr/bin/podman`
- Suppresses "Emulate Docker CLI using podman" warnings

If `podman-docker` is not available in the chosen repo, create the symlink manually:
```dockerfile
RUN ln -s /usr/bin/podman /usr/local/bin/docker
```

Additionally create the nodocker marker to suppress warnings:
```dockerfile
RUN touch /etc/containers/nodocker
```

### Docker Compose Support

Podman 5.x includes `podman compose` as a built-in subcommand that delegates to `docker-compose` or `podman-compose`. Options:
1. Install `podman-compose` via pip: `pip3 install podman-compose` (Python dependency)
2. Install `docker-compose-v2` if available in repos
3. Rely on Podman's built-in `podman compose` (requires docker-compose or podman-compose backend)

**Note:** Full Docker Compose testing is Story 4-2's scope. Story 4-1 just needs the foundation (Podman installed with docker alias). The compose backend can be finalized in 4-2.

### AC #3 Already Verified

Story 3-2 already added tests verifying:
- `cmd_run()` docker run command does NOT contain `--privileged`
- `cmd_run()` docker run command does NOT mount Docker socket (`/var/run/docker.sock`)

These tests are at the end of `tests/test_sandbox.sh` in the "Filesystem and Credential Isolation Verification" section. Confirm they exist; do not duplicate them.

### Test Pattern

Follow the established test pattern. The template processing tests capture the output of `process_template()` and verify the generated Dockerfile content. Example pattern:

```bash
# --- Podman Installation Verification ---

# Test: generated Dockerfile contains Podman repository setup
output="$(process_template_capture ...)"
assert_contains "${output}" "podman" "generated Dockerfile installs podman"

# Test: generated Dockerfile contains docker alias
assert_contains "${output}" "podman-docker" "generated Dockerfile provides docker alias"

# Test: generated Dockerfile contains rootless user mapping
assert_contains "${output}" "subuid" "generated Dockerfile configures rootless user namespace"

# Test: generated Dockerfile contains uidmap dependency
assert_contains "${output}" "uidmap" "generated Dockerfile installs uidmap for rootless"
```

Check existing template processing tests in `test_sandbox.sh` to find the exact capture mechanism used for generated Dockerfile output.

### Previous Story Intelligence

**From Story 3-2 (most recent):**
- 320 test assertions currently passing
- Negative assertion pattern well-established (testing what should NOT be present)
- No debugging needed in 3-2 -- clean implementation

**From Story 1-4 (Dockerfile generation):**
- Template processing in `process_template()` function (sandbox.sh lines 261-351)
- Conditional blocks use `# {{IF_NAME}}` / `# {{/IF_NAME}}` format
- Podman installation is NOT conditional -- add it as static Dockerfile instructions (no markers)
- Template tests verify generated Dockerfile content via function output capture

**Key pattern:** Dockerfile.template changes automatically trigger image rebuilds via the content hash (config.yaml + Dockerfile.template + entrypoint.sh + git-wrapper.sh).

### Git Recent Commits

```
0d7fbb7 feat: add filesystem and credential isolation verification tests with review fixes (story 3-2)
5f87361 feat: implement git push blocking with review fixes (story 3-1)
b936ee9 feat: implement git wrapper passthrough and CLI toolchain with review fixes (story 2-4)
322198e feat: implement env var injection with validation and review fixes (story 2-3)
80b8cd1 feat: implement secret injection and validation with review fixes (story 2-2)
```

This is the first story in Epic 4. All Epics 1-3 are complete (12 stories done).

### Architecture Compliance

- **FR26:** Agent can build Docker images from Dockerfiles inside the sandbox -- Podman provides this via rootless build
- **FR27:** Agent can run `docker compose up` -- Podman compose compatibility (foundation in this story, full testing in 4-2)
- **FR35:** System enforces non-privileged inner Docker -- Podman is rootless by default, no `--privileged` needed
- **FR42:** System bakes isolation scripts into the image -- Podman + docker alias baked at build time
- **NFR4:** Sandbox container runs without `--privileged` and without host Docker socket mount -- inherent to Podman's daemonless architecture
- **NFR7:** CLI compatible with Docker Engine 20.10+ and Podman -- Podman IS the inner runtime, docker alias provides CLI compatibility

### Threat Model Context

Per architecture: Podman's rootless, daemonless architecture means no daemon startup, no socket to mount, no privilege escalation path. The agent sees a standard `docker` CLI that transparently routes to Podman. Inner containers use rootless networking (slirp4netns/pasta) which is inherently isolated from the host network.

### Project Structure Notes

- **Dockerfile.template** -- MODIFY: add Podman installation block, docker alias, rootless config
- **tests/test_sandbox.sh** -- MODIFY: add Podman verification tests
- No new files needed

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 4, Story 4.1]
- [Source: _bmad-output/planning-artifacts/architecture.md - Inner Container Runtime Decision (Podman 5.8.x)]
- [Source: _bmad-output/planning-artifacts/architecture.md - Gap 1: Podman-Docker alias mechanism]
- [Source: _bmad-output/planning-artifacts/architecture.md - Gap 8: Podman version availability in Ubuntu 24.04]
- [Source: _bmad-output/planning-artifacts/prd.md - FR26, FR27, FR35, NFR4, NFR7]
- [Source: _bmad-output/implementation-artifacts/3-2-filesystem-and-credential-isolation.md - AC #3 test coverage]
- [Source: Dockerfile.template - Current structure, insertion point after line 17]
- [Source: sandbox.sh - process_template() lines 261-351, compute_content_hash() lines 213-241]

## Dev Agent Record

### Agent Model Used
Claude Opus 4.6 (1M context)

### Debug Log References
- Updated existing "no SDKs: no Python install" test to accommodate python3 as a system dependency for podman-compose (not an SDK). Changed assertion from checking for "python" substring to checking for Python SDK version pattern and IF_PYTHON conditional block.

### Completion Notes List
- Task 1: Added Podman 5.x installation from upstream Kubic/OBS repository with GPG key, podman-docker alias, uidmap, fuse-overlayfs, and nodocker marker. Inserted after common tools and before PACKAGES placeholder.
- Task 2: Configured rootless Podman with subuid/subgid entries (sandbox:100000:65536). Existing sandbox user (useradd -m -s /bin/bash) is compatible with Podman rootless. Default overlay storage with fuse-overlayfs is sufficient (no explicit storage.conf needed).
- Task 3: Installed podman-compose via pip3 and created docker-compose wrapper script that delegates to `podman compose`.
- Task 4: Added 15 new test assertions verifying Podman repo setup, package installation, docker alias, rootless config, uidmap, fuse-overlayfs, podman-compose, and docker-compose wrapper. All 335 tests pass (up from 320).
- Task 5: Confirmed Story 3-2 tests at lines 2383-2410 already verify no --privileged and no Docker socket mount. No additional assertions needed.

### Change Log
- 2026-03-24: Story 4-1 implementation complete. Added Podman 5.x installation, Docker alias, rootless config, and Docker Compose compatibility to Dockerfile.template. Added 15 test assertions.

### File List
- Dockerfile.template (modified) — Added Podman installation block, nodocker marker, Docker Compose compatibility, rootless user config
- tests/test_sandbox.sh (modified) — Added 15 Podman verification test assertions; updated "no SDKs: no Python install" test for compatibility
