# Full Project Retrospective — asbox

**Date:** 2026-04-15
**Scope:** All 10 Epics (Full Project Review)
**Facilitator:** Amelia (Developer)
**Participants:** Alice (Product Owner), Charlie (Senior Dev), Dana (QA Engineer), Elena (Junior Dev), Winston (Architect), Manuel (Project Lead)

---

## Project Summary

| Metric | Value |
|--------|-------|
| Epics Completed | 10/10 (100%) |
| Stories Completed | 27/27 (100%) |
| Timeline | 2026-03-24 → 2026-04-14 (22 days) |
| Commits | 46 |
| Go Source Lines | 6,793 (19 files) |
| Go Test Lines | 5,421 (27 files) |
| Embedded Assets | 9 files |
| Unit Tests | 169 |
| Integration Tests | 24 |
| Total Tests | 193 (all passing) |
| Sprint Changes | 4 mid-sprint course corrections |
| Production Incidents | 0 |

### Key Milestone — The Go Pivot

The project started as a bash tool (2026-03-24) and was completely rewritten in Go (2026-04-06/04-11). The bash era (13 days) built the foundation; the Go era (8 days) rebuilt and extended everything.

### Velocity by Phase

| Phase | Dates | Commits | Stories Equivalent |
|-------|-------|---------|--------------------|
| Bash implementation | Mar 24-31 | 29 | Original epics 1-8 (bash) |
| Go pivot & rewrite | Apr 6-11 | 8 | Epics 1-9 (Go rewrite + multi-agent) |
| Codex & final tests | Apr 13-14 | 4 | Stories 1-10, 9-5, 9-6 |
| Cleanup | Apr 14 | 1 | Epic 10 (bash removal) |

---

## Successes

### 1. The Go Pivot Was the Defining Decision

Rewrote from bash to Go, eliminating dependency on yq/bash4, gaining type safety, testability, single-binary distribution with embedded assets. The rewrite landed cleanly and unlocked the architecture that made multi-agent and codex support trivial.

### 2. Sprint Change Process Worked Beautifully

Four mid-sprint course corrections — Go pivot, `--no-cache` flag, `auto_isolate_deps` bmad_repos extension, multi-agent support, and codex support — each proposed, documented, and integrated without breaking stride.

### 3. Testing Discipline From Day One

Conventions established in Story 1-1 held across all 27 stories: table-driven tests, `t.TempDir()`, `errors.As` for type assertions. Three distinct test layers (unit, integration via testcontainers, binary invocation) each covering what the others couldn't. Test-to-source line ratio near 1:1.

### 4. Registry Patterns Unlocked Velocity

`AgentConfigRegistry` (Story 1-9) made codex support (Story 1-10) nearly trivial — one map entry per registry location. MCP server registry (Story 5-1) did the same for MCP extensibility.

### 5. Code Review Caught Critical Bugs

UID/GID privilege drop (1-5), null-byte hash delimiter (1-6), `defer cancel()` vs `t.Cleanup(cancel)` race (9-5), dotglob skip (9-6) — all found by review, not tests.

### 6. Verification Stories Proved Their Worth

Stories 2-2 and 4-1 ("build and run for real") found 6 and 7 real bugs respectively. Story 9-3 confirmed all isolation tests were already complete. Scheduling verification passes after implementation is proven effective.

---

## Challenges

### 1. Integration Tests Don't Catch Real-World Issues

Tests verify individual features in single-container isolation. No tests simulate concurrent sandboxes, long-lived agent sessions, or the CLI authentication dance. Issues like container name collision only surface in real usage.

**Resolution:** Manual smoke test checklist for story completion. Automated E2E smoke testing deferred due to authentication complexity.

### 2. Sprint Artifact Staleness

The bash-to-Go pivot left 15+ orphaned story files, incorrect epic statuses (1-9 still showing `in-progress`), and mixed bash/Go items in `deferred-work.md`. Artifact cleanup was not part of the course correction process.

**Resolution:** Immediate cleanup as urgent action item.

### 3. Tests Required by Story Specs Routinely Omitted

Stories 1-1, 1-5, 1-7, and 1-10 all had tests explicitly listed in task breakdowns that weren't written until code review caught the gap. Implementation momentum overran test checklists.

### 4. Same Error Handling Bug Appeared Four Times

Stories 1-1, 1-6, 1-8, and 2-1 all had bare `==` or `os.IsNotExist` instead of `errors.As`/`errors.Is`. Fixed each time but no convention was established to prevent recurrence.

### 5. Injection Risks Deferred Three Times

SDK version strings, package names, and ENV values flow unsanitized into Dockerfile `ARG`/`RUN` directives. Flagged in reviews for Stories 1-3, 1-4, 1-5. Deferred every time. Highest-severity unresolved debt.

---

## Key Patterns

### Pattern 1: Code Review Is the Strongest Quality Gate

`filepath.Rel` silent discard (6-1), temp file cleanup (8-1), default-less switch (8-1), privilege drop (1-5), dotglob (9-6) — a class of "plausible but wrong error handling" that happy-path testing doesn't reach. Review catches what tests miss.

### Pattern 2: Abstractions Pay for Themselves When Timed Right

`AgentConfigRegistry`, MCP server registry, `ensureBuild()` helper, `scanDir()` refactor — each introduced at the right moment, each making subsequent features dramatically cheaper.

### Pattern 3: Verification Stories Surface Real Bugs

Stories designated as "build it, run it, fix what breaks" consistently found multiple real issues (13+ bugs across Stories 2-2 and 4-1).

### Pattern 4: Mid-Sprint Scope Changes Are Manageable With Process

Four sprint changes integrated cleanly because each followed the proposal → document → implement → test pipeline. The only gap was artifact cleanup not being part of the change process.

### Pattern 5: Two-Domain Architecture Held Under Pressure

The Go-on-host / bash-in-container boundary with a clear data pipeline (config → build args → image → run flags → entrypoint env vars) meant every new feature had an obvious home. The architecture never needed rethinking.

---

## Technical Debt Summary

### High Priority

| Item | Source | Impact |
|------|--------|--------|
| Container name collision on concurrent runs | Story 1-7, future-work.md | Blocks parallel sandbox usage |
| SDK version / package name / ENV injection into Dockerfile | Stories 1-3, 1-4, 1-5 | Security: shell injection via config |
| `-it` flag hardcoded (breaks non-TTY/CI) | Story 1-7 | Blocks CI integration |

### Medium Priority

| Item | Source | Impact |
|------|--------|--------|
| No context timeout on integration tests | All integration files | Tests can hang indefinitely |
| Comma in dir names breaks `chown_volumes` IFS | Story 6-1 | Entrypoint failure for edge-case paths |
| Content hash includes runtime-only fields | Story 8-1 | Unnecessary image rebuilds |
| `execInContainer` missing `Multiplexed()` | Story 9-1 | Stream-framed output in test assertions |

### Low Priority

| Item | Source | Impact |
|------|--------|--------|
| Floating Docker Compose / npm agent versions | Stories 2-2, 4-2 | Non-reproducible builds |
| Hardcoded agent name lists in error messages | Story 1-10 | Manual sync when adding agents |
| Node.js validation copy-pasted per agent | Story 1-10 | Maintainability |
| `go install` won't work until version tag published | Story 10-1 | Installation path broken |

---

## Action Items

### Urgent: Sprint Status Cleanup

1. **Update all epic statuses to `done`** — Owner: Amelia
2. **Remove orphaned bash-era story files** — Owner: Amelia
3. **Prune `deferred-work.md` of bash-specific items** — Owner: Amelia

### Process Improvements

4. **Create manual smoke test checklist** — Owner: Dana
   - Single sandbox launch + agent start + graceful shutdown
   - Two concurrent sandboxes (different projects)
   - Two concurrent sandboxes (same project — name collision case)
   - Mount persistence across restart
   - Secret injection verification
   - `auto_isolate_deps` with real `node_modules`

5. **Establish error handling convention** — Owner: Charlie
   - Always `errors.Is`/`errors.As`, never bare comparison
   - Document in CLAUDE.md or project-context.md

### Technical Debt

6. **Fix container name collision** — Owner: Charlie — Priority: High
7. **Input sanitization for Dockerfile injection** — Owner: Charlie — Priority: High
8. **Fix `-it` flag for non-TTY contexts** — Owner: Elena — Priority: Medium
9. **Pin floating dependencies in Dockerfile** — Owner: Amelia — Priority: Low

---

## Key Takeaways

1. **The Go pivot was the defining decision** — eliminated a class of problems and unlocked clean architecture.
2. **Code review is the strongest quality gate** — invest in reviews, don't skip them.
3. **Registry patterns pay for themselves** — recognize load-bearing abstractions and invest in them.
4. **Verification stories are proven** — schedule "build and run for real" passes after implementation.
5. **Integration tests and real usage are different** — manual smoke checklist bridges the gap pragmatically.
6. **Sprint artifacts need hygiene** — course corrections should include cleanup, not defer it.

---

## Team Performance

22 days. 10 epics. 27 stories. 193 tests. Zero production incidents. A complete Go rewrite mid-project. Three AI agent runtimes supported. The team delivered a production-ready CLI tool with comprehensive test coverage and clean architecture.
