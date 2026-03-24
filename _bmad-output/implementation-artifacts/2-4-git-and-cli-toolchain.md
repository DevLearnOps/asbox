# Story 2.4: Git and CLI Toolchain

Status: review

## Story

As a developer,
I want the agent to have local git, internet access, and common CLI tools available inside the sandbox,
So that the agent can use standard development workflows.

## Acceptance Criteria

1. **Given** a sandbox is running
   **When** the agent runs git operations (add, commit, log, diff, branch, checkout, merge, amend)
   **Then** all local git operations work identically to standard git (NFR10)

2. **Given** a sandbox is running with internet access
   **When** the agent runs `curl`, `wget`, or `dig`
   **Then** the commands succeed and can reach external hosts

3. **Given** the agent needs to fetch documentation or packages from the internet
   **When** it makes outbound HTTP/HTTPS requests
   **Then** requests succeed (outbound internet is unrestricted)

## Tasks / Subtasks

- [x] Task 1: Implement git-wrapper.sh as transparent passthrough (AC: #1)
  - [x] 1.1 Replace placeholder in `scripts/git-wrapper.sh` with passthrough to `/usr/bin/git`
  - [x] 1.2 Wrapper must forward ALL arguments and stdin transparently
  - [x] 1.3 Wrapper must preserve exit codes from real git
  - [x] 1.4 Wrapper must NOT alter any git operation (push blocking is Story 3-1)
- [x] Task 2: Ensure git is installed as a base image package (AC: #1)
  - [x] 2.1 Verify `git` is in the always-installed apt-get list in Dockerfile.template (not dependent on user config packages)
  - [x] 2.2 If git is missing from base packages, add it alongside curl/wget/dnsutils
- [x] Task 3: Verify CLI tools installed in base image (AC: #2)
  - [x] 3.1 Confirm curl, wget, dnsutils (provides dig) already in Dockerfile.template base packages
  - [x] 3.2 Verify no additional changes needed for AC #2
- [x] Task 4: Write tests for git wrapper transparency (AC: #1)
  - [x] 4.1 Test: git-wrapper.sh forwards all arguments to /usr/bin/git
  - [x] 4.2 Test: git-wrapper.sh preserves exit codes
  - [x] 4.3 Test: git-wrapper.sh passes stdin through (for commit messages)
  - [x] 4.4 Test: common git operations (add, commit, log, diff, branch, checkout, merge, commit --amend) all pass through
- [x] Task 5: Write tests for CLI tool and internet availability (AC: #2, #3)
  - [x] 5.1 Test: Dockerfile.template includes git, curl, wget, dnsutils in base apt-get install
  - [x] 5.2 Test: resolved Dockerfile contains expected package installation lines
  - [x] 5.3 Test: no network isolation flags in docker run command (internet unrestricted by default)

## Dev Notes

### Critical: git-wrapper.sh Scope for This Story

The git wrapper at `/usr/local/bin/git` must be a **transparent passthrough** for this story. It simply forwards ALL commands to `/usr/bin/git`. The push-blocking logic is **Story 3-1** (Epic 3) -- do NOT implement push blocking here.

The wrapper exists now as infrastructure that Story 3-1 will extend. For 2-4, it must be invisible to the agent per NFR10.

### Implementation Pattern for git-wrapper.sh

```bash
#!/usr/bin/env bash
set -euo pipefail
# Transparent passthrough to real git - push blocking added in Story 3-1
exec /usr/bin/git "$@"
```

This is intentionally minimal. Key points:
- `exec` replaces the wrapper process with real git (preserves exit codes, signals, stdin/stdout)
- `"$@"` forwards all arguments with proper quoting
- No conditional logic needed yet -- Story 3-1 adds the push interception

### Dockerfile.template -- What Already Exists

The base image already installs these via apt-get (always, not config-dependent):
- `curl`, `wget`, `dnsutils` (provides `dig`), `ca-certificates`, `gnupg`

**Action needed:** Verify `git` is in this base package list. If not, add it. Git MUST be a base package (not user-config dependent) because the git-wrapper depends on `/usr/bin/git` existing.

The starter config (`templates/config.yaml`) lists git under packages, but the base image must install it regardless since the wrapper references it.

### Internet Access -- No Code Changes Needed

Outbound internet is unrestricted by default in Docker/Podman containers. No firewall or network isolation flags are set in `cmd_run()`. AC #3 is satisfied by design -- verify with a test that no `--network=none` or similar flags appear in docker run.

### Previous Story Patterns to Follow

From Story 2-3 implementation:
- **Test pattern:** Mock docker binary logs invocations to `MOCK_DOCKER_LOG`, tests grep the log
- **Flag assembly:** `run_flags+=()` array pattern in `cmd_run()`
- **Config parsing:** Reuse existing `parse_config()` output -- do NOT add ad-hoc yq calls
- **Test count baseline:** 245 tests currently passing (222 assertions + review tests)

From Story 2-2:
- Exit code conventions: 0=success, 1=general, 2=usage, 3=dependency, 4=secret validation
- `set -euo pipefail` mandatory in all scripts
- Double-quote all variable expansions: `"${var}"`

### What NOT to Change

- `sandbox.sh` -- no changes needed (config parsing, build, and run already handle packages)
- `entrypoint.sh` -- no changes needed (PATH already has /usr/local/bin before /usr/bin)
- `templates/config.yaml` -- no changes needed (already lists git in packages)
- `parse_config()` -- no new config sections needed
- `process_template()` -- no new conditional blocks needed

### Project Structure Notes

- Alignment: git-wrapper.sh is already in `scripts/` directory, already copied to `/usr/local/bin/git` in Dockerfile.template, already included in content-hash computation
- No new files needed -- only modify existing `scripts/git-wrapper.sh` and `tests/test_sandbox.sh`
- Potentially add `git` to base apt-get install in `Dockerfile.template` if not already there

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 2, Story 2.4]
- [Source: _bmad-output/planning-artifacts/architecture.md - Git wrapper section, Dockerfile.template section]
- [Source: _bmad-output/planning-artifacts/prd.md - FR23, FR24, FR25, NFR10]
- [Source: scripts/git-wrapper.sh - Current placeholder]
- [Source: Dockerfile.template - Base package installation, git-wrapper COPY]
- [Source: _bmad-output/implementation-artifacts/2-3-agent-runtime-with-project-files-and-bmad-support.md - Test patterns, flag assembly patterns]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None — no issues encountered during implementation.

### Completion Notes List

- Replaced placeholder git-wrapper.sh with transparent `exec /usr/bin/git "$@"` passthrough using `set -euo pipefail`
- Added `git` to base apt-get install in Dockerfile.template (was missing from always-installed packages, only in user config)
- Verified curl, wget, dnsutils already present in base packages — no changes needed
- Verified no network isolation flags in docker run — internet unrestricted by default
- Added 28 new tests (257→285 total): git wrapper argument forwarding, exit code preservation, stdin passthrough, all common git operations, Dockerfile.template package verification, network isolation absence checks
- All 285 tests pass with 0 failures

### Change Log

- 2026-03-24: Story created by create-story workflow
- 2026-03-24: Implemented git-wrapper.sh passthrough, added git to base packages, added 28 tests

### File List

- scripts/git-wrapper.sh (modified — replaced placeholder with exec passthrough)
- Dockerfile.template (modified — added git to base apt-get install)
- tests/test_sandbox.sh (modified — added git wrapper and CLI tool tests)
