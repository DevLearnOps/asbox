# Sprint Change Proposal — Remove Legacy Bash Implementation

**Date:** 2026-04-10
**Triggered by:** Go rewrite completion (all Epics 1-9 stories done)
**Scope Classification:** Minor

## Section 1: Issue Summary

The Go rewrite of asbox is complete — all 9 epics and their stories are implemented on the `feat/go-rewrite` branch. However, the legacy bash sandbox implementation still exists in the repository:

- `sandbox.sh` (26KB) — original bash CLI entry point
- `scripts/` — bash entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh, agent-instructions.md
- `tests/` — bash integration test suite and fixtures (~172KB)
- `Dockerfile.template` — bash-era Dockerfile template
- `podman/` — empty leftover directory

These files are fully superseded by the Go implementation (`cmd/`, `internal/`, `embed/`, `integration/`). They add confusion about which implementation is canonical, pollute code searches, and could mislead AI agents working in the repo.

Additionally, `README.md` still documents the bash implementation (installation via `ln -s sandbox.sh`, bash test commands, bash project structure).

## Section 2: Impact Analysis

**Epic Impact:** No existing epic is affected. All stories are done. A new Epic 10 is added.

**Story Impact:** No current or future stories are affected. One new story (10.1) is created for the cleanup.

**Artifact Conflicts:**
- **PRD:** No changes needed — PRD already describes the Go CLI
- **Architecture:** No changes needed — architecture already describes Go project structure
- **Epics:** New Epic 10 and Story 10.1 added
- **Sprint Status:** New epic and story tracked as backlog

**Technical Impact:**
- Pure file deletion — no Go code changes
- README.md rewrite to reflect Go CLI
- Historical references in `_bmad-output/` left intact (document the migration journey)

## Section 3: Recommended Approach

**Path:** Direct Adjustment — add a single cleanup epic with one story.

**Rationale:** This is the simplest possible change. File deletion is trivial, and the README rewrite is the only substantive work. No rollback, no scope reduction, no architectural changes needed.

- **Effort:** Low — single story, ~30 minutes of work
- **Risk:** Low — deleting dead code with no dependents
- **Timeline Impact:** None — does not block or delay any other work

**Alternatives considered:**
- Appending to Epic 1 instead of new epic: Rejected because cleanup is a distinct concern from "build and launch a sandbox"
- Deferring to post-merge: Rejected because dead code should not be merged to main

## Section 4: Detailed Change Proposals

### 4.1 — New Epic 10 in epics.md

Add after Epic 9 in the Epic List section:

```
### Epic 10: Remove Legacy Bash Implementation
Remove all files from the original bash sandbox implementation that have been fully replaced by the Go rewrite. Update documentation to reflect the Go project structure.
**FRs covered:** N/A (cleanup)
```

### 4.2 — New Story 10.1 in epics.md

Add full story definition at end of file:

```
## Epic 10: Remove Legacy Bash Implementation

Remove all files from the original bash sandbox implementation that have been fully replaced by the Go rewrite. Update documentation to reflect the Go project structure.

### Story 10.1: Remove Bash Sandbox Files and Update README

As a developer,
I want the legacy bash sandbox files removed and the README updated to reflect the Go CLI,
So that the repository contains only the canonical Go implementation with accurate documentation.

**Acceptance Criteria:**

**Given** the Go rewrite is complete (all Epics 1-9 stories done)
**When** the cleanup story is executed
**Then** the following files/directories are deleted:
- `sandbox.sh` (original bash CLI entry point)
- `scripts/` (bash entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh, agent-instructions.md)
- `tests/` (bash integration test suite and fixtures)
- `Dockerfile.template` (bash-era Dockerfile template)
- `podman/` (empty leftover directory)

**Given** the bash files are removed
**When** inspecting the repository
**Then** no remaining file outside `_bmad-output/` references the deleted files as current/active code

**Given** the README.md currently documents the bash implementation
**When** the cleanup story is executed
**Then** README.md is rewritten to document the Go CLI:
- Installation via single binary (not `ln -s sandbox.sh`)
- `asbox` command reference (not `sandbox` command)
- Go project structure (`cmd/`, `internal/`, `embed/`, `integration/`)
- `go test` for testing (not `bash tests/test_sandbox.sh`)
- Build caching via content-hash (referencing Go implementation)
- Remove all references to `sandbox.sh`, `Dockerfile.template`, `scripts/`, `tests/`

**Implementation Notes:**
- Pure deletion + documentation rewrite — no Go code changes
- Historical references in `_bmad-output/` implementation artifacts are left intact (they document the migration journey)
- The `Makefile` has no bash references and needs no changes
```

### 4.3 — Sprint status update in sprint-status.yaml

Add after Epic 9 block:

```yaml
  # Epic 10: Remove Legacy Bash Implementation
  epic-10: backlog
  10-1-remove-bash-sandbox-files-and-update-readme: backlog
  epic-10-retrospective: optional
```

## Section 5: Implementation Handoff

**Change Scope:** Minor — direct implementation by dev team.

**Handoff:** Development team (Amelia / dev agent)

**Responsibilities:**
1. Scrum Master (Bob): Create story file `10-1-remove-bash-sandbox-files-and-update-readme.md` via `bmad-create-story`
2. Developer (Amelia): Execute the story — delete files, rewrite README
3. No PO/PM/Architect involvement needed

**Success Criteria:**
- All listed bash files/directories deleted from repo
- README.md accurately documents the Go CLI with no bash references
- `go build ./...` and `go test ./...` still pass (no accidental breakage)
- Sprint status updated to reflect story completion
