# Story 10.1: Remove Bash Sandbox Files and Update README

Status: done

## Story

As a developer,
I want the legacy bash sandbox files removed and the README updated to reflect the Go CLI,
So that the repository contains only the canonical Go implementation with accurate documentation.

## Acceptance Criteria

1. **Legacy File Deletion**
   ```
   GIVEN the Go rewrite is complete (all Epics 1-9 stories done)
   WHEN the cleanup story is executed
   THEN the following files/directories are deleted:
     - sandbox.sh
     - scripts/ (entire directory)
     - tests/ (entire directory)
     - Dockerfile.template
     - podman/ (empty directory)
     - .sandbox-dockerfile (generated artifact)
   ```

2. **No Stale References Outside _bmad-output/**
   ```
   GIVEN the bash files are removed
   WHEN inspecting the repository
   THEN no remaining file outside _bmad-output/ references the deleted files as current/active code
   ```

3. **README Rewritten for Go CLI**
   ```
   GIVEN the README.md currently documents the bash implementation
   WHEN the cleanup story is executed
   THEN README.md is rewritten to document the Go CLI:
     - Title: "asbox" (not "Sandbox")
     - Prerequisites: Docker 20.10+ only (remove Bash 4.0+ and yq)
     - Installation via single binary or `go install` (not ln -s sandbox.sh)
     - `asbox` command reference (not `sandbox` command)
     - Go project structure (cmd/, internal/, embed/, integration/)
     - `go test` for testing (not bash tests/test_sandbox.sh)
     - Build caching via content-hash referencing Go implementation
     - Remove all references to sandbox.sh, Dockerfile.template, scripts/, tests/
   ```

## Tasks / Subtasks

- [x] Task 1: Delete legacy bash files (AC: #1)
  - [x] 1.1 Delete `sandbox.sh` (7KB bash CLI entry point)
  - [x] 1.2 Delete `scripts/` directory (entrypoint.sh, git-wrapper.sh, healthcheck-poller.sh, agent-instructions.md)
  - [x] 1.3 Delete `tests/` directory (test_sandbox.sh, test_integration_agent_cli.sh, test_integration_compose.sh, test_integration_podman.sh, fixtures/)
  - [x] 1.4 Delete `Dockerfile.template` (5.8KB bash-era template)
  - [x] 1.5 Delete `podman/` directory (empty)
  - [x] 1.6 Delete `.sandbox-dockerfile` (4.7KB generated artifact)

- [x] Task 2: Verify no stale references (AC: #2)
  - [x] 2.1 Grep the repo (excluding `_bmad-output/`) for: `sandbox.sh`, `Dockerfile.template`, `scripts/entrypoint`, `scripts/git-wrapper`, `tests/test_sandbox`, `podman/`
  - [x] 2.2 The ONLY file with references should be `README.md` — those are fixed in Task 3
  - [x] 2.3 Check `.gitignore` for references to deleted files (currently none exist — confirm)

- [x] Task 3: Rewrite README.md (AC: #3)
  - [x] 3.1 Update title: `# asbox` (not `# Sandbox`)
  - [x] 3.2 Update intro paragraph: reference "asbox (Agent-SandBox)" and Go binary
  - [x] 3.3 Rewrite Prerequisites: Docker Engine or Podman 20.10+ only. Remove Bash 4.0+ and yq rows — those were bash-only dependencies
  - [x] 3.4 Rewrite Quick Start: install via `go install` or download binary, use `asbox init`, `asbox run` commands, reference `.asbox/config.yaml` (not `.sandbox/config.yaml`)
  - [x] 3.5 Rewrite CLI Reference: `asbox [command]` with `--config` / `-f` flag; document `asbox init`, `asbox build`, `asbox run` including `--no-cache` flag on build/run
  - [x] 3.6 Update Configuration section: change `.sandbox/config.yaml` references to `.asbox/config.yaml`; keep config schema content (it's implementation-agnostic)
  - [x] 3.7 Rewrite Build Caching section: describe Go content-hash approach (rendered Dockerfile + embedded scripts + config content), `asbox-<project>:<hash>` tag format, mention `--no-cache` flag
  - [x] 3.8 Rewrite Testing section: `go test ./...` (unit), `go test -v ./integration/...` (integration), or `make test` / `make test-integration`
  - [x] 3.9 Rewrite Project Structure to match Go layout from architecture doc
  - [x] 3.10 Keep sections that are implementation-agnostic: Usage Patterns, Isolation Model (update any `sandbox` command references to `asbox`), Exit Codes
  - [x] 3.11 Scan entire README for any remaining `sandbox` → `asbox` command references, `.sandbox/` → `.asbox/` path references

- [x] Task 4: Final verification
  - [x] 4.1 Run `go test ./...` to confirm no Go code was accidentally broken
  - [x] 4.2 Confirm deleted files are gone: `ls sandbox.sh scripts/ tests/ Dockerfile.template podman/ .sandbox-dockerfile` should all fail
  - [x] 4.3 Grep repo (excluding _bmad-output/) for `sandbox.sh` — should return zero results

## Dev Notes

### What This Story Is

Pure deletion + documentation rewrite. **No Go code changes.** The Go implementation is complete and does not reference any of these legacy files.

### Files to Delete (Exhaustive List)

| File/Dir | Description | Size | Safe to Delete? |
|----------|-------------|------|-----------------|
| `sandbox.sh` | Original bash CLI | 7KB | Yes — replaced by `cmd/*.go` |
| `scripts/entrypoint.sh` | Bash container startup | 6.6KB | Yes — replaced by `embed/entrypoint.sh` |
| `scripts/git-wrapper.sh` | Bash git push interceptor | 239B | Yes — replaced by `embed/git-wrapper.sh` |
| `scripts/healthcheck-poller.sh` | Bash healthcheck | 508B | Yes — replaced by `embed/healthcheck-poller.sh` |
| `scripts/agent-instructions.md` | Static agent instructions | 1.4KB | Yes — replaced by `embed/agent-instructions.md.tmpl` |
| `tests/test_sandbox.sh` | Bash test suite | 155KB | Yes — replaced by Go tests in `cmd/*_test.go` + `integration/` |
| `tests/test_integration_agent_cli.sh` | Bash agent CLI test | 4.3KB | Yes — replaced by Go integration tests |
| `tests/test_integration_compose.sh` | Bash compose test | 8.1KB | Yes — replaced by Go integration tests |
| `tests/test_integration_podman.sh` | Bash podman test | 4.8KB | Yes — replaced by Go integration tests |
| `tests/fixtures/` | Test fixture files | small | Yes — Go tests use inline fixtures |
| `Dockerfile.template` | Bash-era Dockerfile template | 5.8KB | Yes — replaced by `embed/Dockerfile.tmpl` |
| `podman/` | Empty directory | 0 | Yes — unused |
| `.sandbox-dockerfile` | Generated build artifact | 4.7KB | Yes — build artifact from bash era |

### Go Replacements Already In Place

The `embed/` directory contains all Go-managed versions of the scripts:
- `embed/Dockerfile.tmpl` — Go `text/template` Dockerfile (replaces `Dockerfile.template`)
- `embed/entrypoint.sh` — Container startup (replaces `scripts/entrypoint.sh`)
- `embed/git-wrapper.sh` — Git push interceptor (replaces `scripts/git-wrapper.sh`)
- `embed/healthcheck-poller.sh` — Healthcheck (replaces `scripts/healthcheck-poller.sh`)
- `embed/agent-instructions.md.tmpl` — Agent instructions template (replaces `scripts/agent-instructions.md`)
- `embed/config.yaml` — Starter config (replaces any old templates/)

### README Rewrite Reference

The current README has these sections. Here's what to do with each:

| Section | Action |
|---------|--------|
| Title + intro | Rename to "asbox", update description |
| Why Sandbox | Keep content, rename section to "Why asbox" |
| Prerequisites | **Rewrite** — remove Bash 4.0+ and yq, keep Docker 20.10+ |
| Quick Start | **Rewrite** — `go install` or binary download, `asbox` commands, `.asbox/` paths |
| CLI Reference | **Rewrite** — `asbox` commands, add `--no-cache` flag |
| Configuration | **Update** — `.sandbox/` → `.asbox/` paths; config schema content is fine |
| Usage Patterns | **Update** — `sandbox` → `asbox` command references |
| Isolation Model | Keep mostly as-is (implementation-agnostic) |
| Build Caching | **Rewrite** — Go content-hash approach, `--no-cache` flag |
| Testing | **Rewrite** — Go test commands, Makefile targets |
| Project Structure | **Rewrite** — Go layout from architecture doc |
| Exit Codes | Keep as-is (same exit codes in Go implementation) |

### Go Project Structure for README

Use this layout (from architecture doc):

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

### Makefile Targets (for Testing section)

```makefile
make build              # go build -o asbox .
make install            # install to ~/.local/bin/asbox
make test               # go test ./... (all tests)
make test-unit          # go test -short ./... (unit only)
make test-integration   # go test -v ./integration/... (integration only)
make test-ci            # go vet + unit + integration
```

### Key Guardrails

- **DO NOT modify any Go code** — this is pure deletion + docs
- **DO NOT delete anything in `_bmad-output/`** — historical artifacts document the migration journey
- **DO NOT delete `embed/`** — those are the Go replacements, not legacy files
- **DO NOT delete `integration/`** — those are Go integration tests, not the legacy bash `tests/`
- **Keep the README structure and tone** — rewrite content but maintain the concise, developer-focused style
- **The `.asbox/` config directory convention** is what the Go CLI uses (not `.sandbox/`)
- **The `--no-cache` flag** was added recently (commit 7f7160e) — include it in CLI Reference and Build Caching sections

### Project Structure Notes

- All legacy bash files live at the repo root alongside the Go project
- The Go project (`cmd/`, `internal/`, `embed/`, `integration/`) is the canonical implementation
- Makefile already targets Go exclusively — no changes needed
- `go.mod` declares module `github.com/mcastellin/asbox`
- No CI configuration files reference the legacy bash files

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 10] — epic definition and acceptance criteria
- [Source: _bmad-output/planning-artifacts/architecture.md#Project Structure] — canonical Go project layout
- [Source: _bmad-output/planning-artifacts/prd.md#Executive Summary] — product context and "asbox" naming
- [Source: Makefile] — Go build targets (confirmed no bash references)
- [Source: embed/embed.go] — Go embed declarations for all replacement assets

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

No issues encountered. Pure deletion + documentation rewrite — no Go code changes required.

### Completion Notes List

- Deleted all 6 legacy bash files/directories: sandbox.sh, scripts/, tests/, Dockerfile.template, podman/, .sandbox-dockerfile
- Verified no stale references remain outside _bmad-output/ (only runtime podman socket paths in Go code, which are unrelated)
- Rewrote README.md: title to "asbox", prerequisites simplified to Docker-only, Quick Start with go install, CLI Reference with --no-cache flag, Configuration with .asbox/ paths and new features (auto_isolate_deps, host_agent_config, bmad_repos), Build Caching with Go content-hash approach, Testing with Go/Makefile commands, Project Structure with Go layout
- All Go unit tests pass (no regressions)
- Change date: 2026-04-10

### File List

- DELETED: sandbox.sh
- DELETED: scripts/entrypoint.sh
- DELETED: scripts/git-wrapper.sh
- DELETED: scripts/healthcheck-poller.sh
- DELETED: scripts/agent-instructions.md
- DELETED: tests/test_sandbox.sh
- DELETED: tests/test_integration_agent_cli.sh
- DELETED: tests/test_integration_compose.sh
- DELETED: tests/test_integration_podman.sh
- DELETED: tests/fixtures/ (directory)
- DELETED: Dockerfile.template
- DELETED: podman/ (directory)
- DELETED: .sandbox-dockerfile
- MODIFIED: README.md

### Review Findings

- [x] [Review][Dismissed] `.asbox/` in Project Structure tree — kept as-is, already in .gitignore
- [x] [Review][Patch] "sandbox container" → "asbox container" in Prerequisites [README.md:32]
- [x] [Review][Patch] "Docker availability" → "Docker or Podman availability" [README.md:34]
- [x] [Review][Patch] Removed stale `.sandbox/` and `.sandbox-Dockerfile` from .gitignore
- [x] [Review][Defer] `go install` path assumes repo is published to github.com/mcastellin/asbox — deferred, pre-existing
