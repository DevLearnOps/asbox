# Story 11.5: Pinned Build Dependencies

Status: done

## Story

As a developer,
I want Dockerfile dependencies to be version-pinned for reproducible builds,
so that sandbox images built today produce the same result as images built next month.

## Acceptance Criteria

1. **Given** the Dockerfile template installs Docker Compose
   **When** the image is built
   **Then** Docker Compose is installed at a specific pinned version (e.g., `v5.1.3`) fetched from a versioned GitHub release URL -- not from the GitHub API `latest` endpoint

2. **Given** the Dockerfile template installs `gemini-cli` via npm
   **When** the image is built
   **Then** `gemini-cli` is installed at a specific pinned version (e.g., `npm install -g @google/gemini-cli@0.38.1`)

3. **Given** the Dockerfile template installs `@openai/codex` via npm
   **When** the image is built
   **Then** `@openai/codex` is installed at a specific pinned version (e.g., `npm install -g @openai/codex@0.121.0`)

4. **Given** the Dockerfile template specifies the base image
   **When** the image is built
   **Then** the base image uses a multi-arch manifest digest (e.g., `ubuntu:24.04@sha256:<multi-arch-digest>`) that works on both amd64 and arm64

5. **Given** a developer or maintainer needs to bump pinned versions
   **When** they look for guidance
   **Then** a comment block in `embed/Dockerfile.tmpl` documents the version update process: which URLs to check, how to obtain multi-arch digests, and how to verify the update

## Tasks / Subtasks

- [x] Task 1: Add version pinning comment block to Dockerfile template (AC: #5)
  - [x] 1.1 Add a Go template comment block (`{{/* ... */}}`) at the top of `embed/Dockerfile.tmpl` documenting: pinned versions list, how to update each, how to get multi-arch digests, verification steps
- [x] Task 2: Pin base image to multi-arch manifest digest (AC: #4)
  - [x] 2.1 Replace `FROM ubuntu:24.04` (line 2) with `FROM ubuntu:24.04@sha256:<multi-arch-digest>` -- use current digest from `docker manifest inspect ubuntu:24.04`
  - [x] 2.2 Remove the existing line 1 comment `{{- /* Base image - tag-pinned; digest omitted for multi-arch ... */ -}}` -- the digest is no longer omitted
- [x] Task 3: Pin Docker Compose to explicit version (AC: #1)
  - [x] 3.1 Replace the dynamic fetch on lines 72-73 (`COMPOSE_VERSION=$(curl -fsSL https://api.github.com/repos/docker/compose/releases/latest | jq -r .tag_name)`) with a hardcoded version variable (e.g., `COMPOSE_VERSION=v5.1.3`)
  - [x] 3.2 Keep the rest of the Docker Compose install block unchanged -- the download URL pattern and symlink are already correct
- [x] Task 4: Pin npm agent packages (AC: #2, #3)
  - [x] 4.1 Pin `@google/gemini-cli` (line 89): change `npm install -g @google/gemini-cli` to `npm install -g @google/gemini-cli@0.38.1`
  - [x] 4.2 Pin `@openai/codex` (line 93): change `npm install -g @openai/codex` to `npm install -g @openai/codex@0.121.0`
- [x] Task 5: Pin Playwright MCP package (follows AC #2/#3 pattern)
  - [x] 5.1 Pin `@playwright/mcp` (line 118): change `npm install -g @playwright/mcp` to `npm install -g @playwright/mcp@0.0.70`
- [x] Task 6: Add regression tests for version pinning (AC: #1, #4)
  - [x] 6.1 In `internal/template/render_test.go`, add assertion to `TestRender_baseImage` that output contains `@sha256:` (base image pinned to digest)
  - [x] 6.2 In `internal/template/render_test.go`, add assertion to `TestRender_dockerCompose` that output does NOT contain `api.github.com` (no dynamic latest fetch)
  - [x] 6.3 In `internal/template/render_test.go`, add assertion to `TestRender_geminiAgent` that npm install includes an `@` version suffix after package name
  - [x] 6.4 Optionally add a new `TestRender_npmVersionsPinned` that renders with all agents and asserts `@google/gemini-cli@`, `@openai/codex@`, and `@playwright/mcp@` patterns exist
- [x] Task 7: Run full test suite and format (AC: all)
  - [x] 7.1 Run `gofmt -w` on all modified `.go` files
  - [x] 7.2 Run `go vet ./...`
  - [x] 7.3 Run `go test ./...` and confirm all tests pass

## Dev Notes

### Single File Change (Primary)

All Dockerfile template changes are in **one file**: `embed/Dockerfile.tmpl`. No Go logic changes are needed because:
- Versions are hardcoded in the template, not in Go code
- The content hash automatically changes when the template content changes (the hash covers the rendered Dockerfile)
- No new config fields, flags, or error types are introduced
- No changes to `internal/template/render.go` -- it renders whatever is in the template

### Current State of embed/Dockerfile.tmpl

**Line 1-2 (base image):**
```dockerfile
{{- /* Base image - tag-pinned; digest omitted for multi-arch (amd64 + arm64) support */ -}}
FROM ubuntu:24.04
```
Change to:
```dockerfile
FROM ubuntu:24.04@sha256:<multi-arch-digest>
```
The digest must be a **multi-arch manifest digest** (not platform-specific). Verify it works on both amd64 and arm64 by running `docker manifest inspect ubuntu:24.04` and confirming the digest resolves to both platforms.

**Lines 72-76 (Docker Compose):**
```dockerfile
RUN COMPOSE_VERSION=$(curl -fsSL https://api.github.com/repos/docker/compose/releases/latest | jq -r .tag_name) && \
    curl -fsSL "https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-linux-$(uname -m)" -o /usr/local/bin/docker-compose && \
```
Change to:
```dockerfile
RUN COMPOSE_VERSION=v5.1.3 && \
    curl -fsSL "https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-linux-$(uname -m)" -o /usr/local/bin/docker-compose && \
```
This removes the `api.github.com` call entirely. The download URL pattern stays the same -- only the version source changes from dynamic to static. The `$(uname -m)` for multi-arch stays.

**Line 89 (Gemini CLI):**
```dockerfile
RUN npm install -g @google/gemini-cli && \
```
Change to:
```dockerfile
RUN npm install -g @google/gemini-cli@0.38.1 && \
```

**Line 93 (Codex):**
```dockerfile
RUN npm install -g @openai/codex && \
```
Change to:
```dockerfile
RUN npm install -g @openai/codex@0.121.0 && \
```

**Line 118 (Playwright MCP):**
```dockerfile
RUN npm install -g @playwright/mcp && \
```
Change to:
```dockerfile
RUN npm install -g @playwright/mcp@0.0.70 && \
```

### Version Update Comment Block

Add a Go template comment block at the very top of `embed/Dockerfile.tmpl` (before the FROM line) documenting all pinned versions and update procedures. Use `{{/* ... */}}` syntax so it's stripped from the rendered output. Include:

1. Table of all pinned dependencies with current versions
2. For Docker Compose: check https://github.com/docker/compose/releases
3. For npm packages: run `npm view <package> version`
4. For base image digest: run `docker manifest inspect ubuntu:24.04 | jq -r .digest`
5. After bumping any version, run `go test ./...` to verify

### Claude Code Install -- NOT Pinned (Intentional)

The Claude Code install (`curl -fsSL https://claude.ai/install.sh | bash`) uses the official Anthropic install script, NOT npm. This is intentionally left unpinned:
- Anthropic controls the script and updates it for security fixes
- The script installs the Claude Code CLI binary, not an npm package
- Pinning would require hosting a versioned copy of the install script

### Existing Tests -- Mostly Unaffected

Most existing test assertions use substring matching that remains valid after pinning:

| Test | Assertion | Still valid? |
|------|-----------|:---:|
| `TestRender_baseImage` (line 17) | `HasPrefix("FROM ubuntu:24.04")` | YES -- `FROM ubuntu:24.04@sha256:...` starts with `FROM ubuntu:24.04` |
| `TestRender_minimalConfig` (line 169) | `Contains("FROM ubuntu:24.04")` | YES -- substring match |
| `TestRender_dockerCompose` (line 440) | `Contains("docker/compose/releases")` | YES -- still in the URL |
| `TestRender_dockerCompose` (line 443) | `Contains("docker-compose-linux-$(uname -m)")` | YES -- URL pattern unchanged |
| `TestRender_geminiAgent` (line 474) | `Contains("npm install -g @google/gemini-cli")` | YES -- substring of versioned string |
| `TestRender_claudeCodeAgent` (line 463) | `Contains("npm install -g @google/gemini-cli")` negation | YES -- still substring |
| Playwright tests (line 543) | `Contains("@playwright/mcp")` | YES -- substring match |
| Hash tests (line 39) | Test fixture data, not template content | NO CHANGE NEEDED |
| Build tests (line 97) | Test fixture data for build command | NO CHANGE NEEDED |

**New assertions to add** (prevent regression to unpinned state):
- `TestRender_baseImage`: add `Contains(output, "@sha256:")` check
- `TestRender_dockerCompose`: add `!Contains(output, "api.github.com")` check
- `TestRender_geminiAgent`: add `Contains(output, "@google/gemini-cli@")` check (note trailing `@`)

### Content Hash Impact

When versions are bumped, the rendered Dockerfile changes, which changes the content hash (`internal/hash/hash.go`). This is **correct behavior** -- version bumps should trigger image rebuilds. No changes to the hash computation logic are needed.

### What NOT To Change

- `cmd/` -- no new commands, flags, or exit codes
- `internal/config/` -- no config changes; versions are in the template, not configurable
- `internal/template/render.go` -- no rendering logic changes
- `internal/docker/` -- no build/run changes
- `internal/hash/` -- no hash computation changes
- `embed/entrypoint.sh` -- no runtime changes
- `embed/git-wrapper.sh` -- unrelated
- `embed/embed.go` -- no new embedded files
- Integration tests -- template content tested at unit level, not integration level

### Obtaining Multi-Arch Digest

To get the current multi-arch manifest digest for ubuntu:24.04:

```bash
docker manifest inspect ubuntu:24.04 | jq -r .digest
```

Or check Docker Hub: https://hub.docker.com/_/ubuntu/tags?name=24.04

The digest must resolve to BOTH amd64 and arm64 platforms. Verify:
```bash
docker manifest inspect ubuntu:24.04 | jq '.manifests[].platform'
```

### Project Structure Notes

- All changes within existing file boundaries -- `embed/Dockerfile.tmpl` and `internal/template/render_test.go`
- No new packages, no new files, no new error types
- No Go dependency changes (`go.mod`/`go.sum` untouched)
- Follows the existing pattern from Epic 11 stories: tightly scoped, single concern

### Previous Story Intelligence (11-4)

**Key learnings from story 11-4:**
- Scoped tightly to specific files with clean separation
- Table-driven tests with explicit assertions on both positive and negative paths
- Reused existing patterns rather than creating new abstractions
- Full project test suite must stay green: `go test ./...`
- Review patches fixed fragile test assertions and stale doc comments

**Git patterns from recent commits:**
- `1c84cec`: `feat: non-TTY runtime support with stdin-based TTY detection (story 11-4)`
- `0dae1e1`: `feat: ENV key/value validation and Dockerfile injection hardening (story 11-3)`
- Convention: `feat:` prefix for new capabilities, `(story N-M)` suffix

### Error Pattern

No new error types needed. This story only changes static content in the Dockerfile template. The only failure mode is a wrong version string that causes `docker build` to fail, which is already handled by the existing `RunError` type.

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 11, Story 11-5]
- [Source: _bmad-output/planning-artifacts/architecture.md - Dockerfile Generation, Content-Hash Caching, Multi-architecture support]
- [Source: _bmad-output/planning-artifacts/prd.md - FR39 (pinned base images), NFR12 (reproducible builds)]
- [Source: embed/Dockerfile.tmpl - base image (line 2), Docker Compose (lines 72-76), gemini install (line 89), codex install (line 93), playwright (line 118)]
- [Source: internal/template/render_test.go - TestRender_baseImage (line 11), TestRender_dockerCompose (line 434), TestRender_geminiAgent (line 468)]
- [Source: internal/hash/hash.go - Compute() hashes rendered Dockerfile, so version changes auto-invalidate cache]

## Dev Agent Record

### Agent Model Used

GPT-5 Codex

### Debug Log References

- Initial implementation using an inferred Docker Hub digest and the story's example `Docker Compose v5.1.3` failed the escalated integration suite. Resolved by querying live registry metadata with `docker buildx imagetools inspect ubuntu:24.04` and pinning the published `Docker Compose v5.0.1` release URL.

### Completion Notes List

- Added a top-level Go template maintenance comment block to `embed/Dockerfile.tmpl` documenting the pinned dependency set, version bump workflow, digest lookup commands, and verification steps.
- Pinned the Ubuntu base image to the multi-arch OCI index digest `sha256:c4a8d5503dfb2a3eb8ab5f807da5bc69a85730fb49b5cfca2330194ebcc41c7b`, pinned Docker Compose to `v5.0.1`, and pinned `@google/gemini-cli`, `@openai/codex`, and `@playwright/mcp` to explicit npm versions.
- Added regression assertions in `internal/template/render_test.go` to enforce digest pinning, block GitHub `latest` API usage, and verify version-pinned npm installs across Gemini, Codex, and Playwright rendering paths.
- Validation passed with `gofmt -w internal/template/render_test.go`, `go vet ./...`, `go test ./internal/template`, `go test -v ./integration -count=1 -timeout 20m`, and a final `go test ./...`.

### Review Findings

- [x] [Review][Dismissed] MCP runtime `npx` invocation not version-pinned in config.go — npx resolves globally installed packages first; the build-time global install of `@playwright/mcp@0.0.70` covers runtime.
- [x] [Review][Defer] No checksum verification on Docker Compose binary download [embed/Dockerfile.tmpl:101] — deferred, pre-existing. The curl-based Docker Compose install has no SHA256 integrity check. This existed before this story and is not caused by the version pinning change.
- [x] [Review][Defer] Build-time `npx playwright install` browser versions determined by transitive deps [embed/Dockerfile.tmpl:147] — deferred, pre-existing. `npx playwright install --with-deps chromium webkit` fetches browser builds based on the transitive playwright version from `@playwright/mcp@0.0.70`, not an explicitly pinned browser version.

### File List

- `embed/Dockerfile.tmpl` (modified)
- `internal/template/render_test.go` (modified)
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (modified)
- `_bmad-output/implementation-artifacts/11-5-pinned-build-dependencies.md` (modified)

### Change Log

- 2026-04-17: Implemented Story 11.5 by pinning Dockerfile build dependencies, documenting the update process, adding regression coverage, and validating the full Go test suite including Docker-backed integration tests.
