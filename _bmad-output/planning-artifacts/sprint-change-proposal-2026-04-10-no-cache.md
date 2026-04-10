# Sprint Change Proposal — `--no-cache` Build Flag

**Date:** 2026-04-10
**Triggered by:** Developer experience — no way to force clean image rebuild
**Scope:** Minor
**Recommended approach:** Direct Adjustment

---

## 1. Issue Summary

When debugging image build issues or when Docker's layer cache is stale, there is no way to force a complete rebuild of the sandbox image. The asbox content-hash mechanism skips `docker build` entirely if the image tag exists, and even when a build runs, Docker's own layer cache may serve stale layers. Adding a `--no-cache` flag addresses both layers of caching.

This is a standard capability in container tooling (`docker build --no-cache`, `docker compose build --no-cache`) and its absence is a gap in the CLI's build workflow.

## 2. Impact Analysis

### Epic Impact

- **Epic 1** (Developer Can Build and Launch a Sandbox): Minor scope addition. All stories already `done`. The new behavior fits within Story 1.6 (Image Build with Content-Hash Caching) and Story 1.7 (Sandbox Run with TTY and Lifecycle).
- **Epics 2-9:** No impact.

### Artifact Conflicts

| Artifact | Impact | Details |
|----------|--------|---------|
| PRD | Add FR55 | New functional requirement for `--no-cache` flag |
| Architecture | Update Content-Hash Caching section | Document two-layer bypass behavior |
| Epics | Update Story 1.6 AC, Story 1.7 notes | Add acceptance criteria for `--no-cache` on both `build` and `run` |
| UI/UX | N/A | CLI tool |

### Technical Impact

Files affected:
- `cmd/build.go` — add `--no-cache` persistent flag
- `cmd/build_helper.go` — `ensureBuild()` accepts noCache parameter, skips hash check, forwards to Docker
- `cmd/run.go` — propagate `--no-cache` flag to `ensureBuild()`
- `internal/docker/build.go` — `BuildOptions` gains `NoCache` field, forwarded as `--no-cache` to `docker build`

No new packages. No new dependencies. No breaking changes.

## 3. Recommended Approach

**Direct Adjustment** — modify existing code within the current epic structure.

- **Effort:** Low — single flag threaded through 4 files
- **Risk:** Low — additive change, no existing behavior modified when flag is absent
- **Timeline impact:** None — this is a small enhancement

### Alternatives considered

- **New story:** Overkill for a single flag addition. Updating Story 1.6 scope is sufficient.
- **Defer to post-MVP:** Not recommended — this is a basic developer ergonomic that should ship with the tool.

## 4. Detailed Change Proposals

### 4.1 PRD — New FR55

After FR54, add:

> FR55: Developer can pass `--no-cache` to `asbox build` (and implicitly to `asbox run`) to bypass the content-hash image existence check and pass `--no-cache` to the underlying Docker build, forcing a complete image rebuild with no cached layers

Add FR Coverage Map entry:

| FR55 | Epic 1 | --no-cache flag for build command |

### 4.2 Architecture — Content-Hash Caching Section

Add after "Image tagging" bullet:

> **Cache bypass (`--no-cache`):** When `--no-cache` is passed to `asbox build` or `asbox run`, two things happen: (1) the content-hash image existence check is skipped — `docker build` runs unconditionally, and (2) `--no-cache` is forwarded to the `docker build` command, forcing Docker to rebuild all layers from scratch. The resulting image is still tagged with the content hash and `latest` — cache bypass affects how the image is built, not how it's tagged.

Update "Affects" to include `cmd/build.go` and `cmd/run.go`.

### 4.3 Epics — Story 1.6 New Acceptance Criteria

Add to Story 1.6:

> **Given** an image already exists with the current content hash
> **When** the developer runs `asbox build --no-cache`
> **Then** the hash existence check is skipped, `docker build` runs with `--no-cache`, and the image is rebuilt from scratch (FR55)
>
> **Given** the developer runs `asbox run --no-cache`
> **When** the image needs to be built or already exists
> **Then** `--no-cache` is forwarded to the build step, bypassing both the hash check and Docker layer cache

Add FR Coverage Map entry:

| FR55 | Epic 1 | --no-cache flag for build command |

### 4.4 Epics — Story 1.7 Implementation Notes

Update `cmd/run.go` note to mention `--no-cache` flag propagation.

## 5. Implementation Handoff

- **Scope:** Minor — direct implementation by development team
- **Handoff to:** Developer (Amelia / bmad-dev-story)
- **Responsibilities:**
  1. Apply PRD, Architecture, and Epics edits from Section 4
  2. Implement `--no-cache` flag in `cmd/build.go`, `cmd/build_helper.go`, `cmd/run.go`, `internal/docker/build.go`
  3. Verify flag works on both `asbox build --no-cache` and `asbox run --no-cache`
- **Success criteria:** Running `asbox build --no-cache` always triggers a full Docker build with no cached layers, even when the content-hash image already exists
