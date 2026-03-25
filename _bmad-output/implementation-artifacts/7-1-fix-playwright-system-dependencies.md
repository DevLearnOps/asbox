# Story 7.1: Fix Playwright System Dependencies

Status: review

## Story

As a developer,
I want Playwright to have all required system libraries pre-installed in the sandbox image,
So that the agent can run browser-based E2E tests without missing library errors.

## Acceptance Criteria

1. **Given** a sandbox image is built with `mcp: [playwright]` configured, **When** the agent runs Playwright tests inside the sandbox, **Then** the browser launches without missing library errors.
2. **Given** the Dockerfile.template Playwright block, **When** `npx playwright install --with-deps chromium` runs at build time, **Then** all required system libraries are present (libnspr4, libnss3, libatk1.0-0t64, libatk-bridge2.0-0t64, libdbus-1-3, libcups2t64, libxcb1, libxkbcommon0, libatspi2.0-0t64, libx11-6, libxcomposite1, libxdamage1, libxext6, libxfixes3, libxrandr2, libgbm1, libcairo2, libpango-1.0-0, libasound2t64).

## Tasks / Subtasks

- [x] Task 1: Diagnose which system libraries are missing (AC: #1, #2)
  - [x] Build the sandbox image with `mcp: [playwright]` configured
  - [x] Run `ldd` against the Chromium binary inside the container to identify missing `.so` files
  - [x] Map missing `.so` files to Ubuntu 24.04 package names
  - [x] Determine why `--with-deps` is insufficient (likely: runs in a separate `apt-get` layer where the package cache was already cleaned)
- [x] Task 2: Add explicit system dependency installation to Dockerfile.template (AC: #1, #2)
  - [x] Add an `apt-get install` for the missing Playwright/Chromium system libraries inside the `IF_MCP_PLAYWRIGHT` block
  - [x] Place the `apt-get install` BEFORE the `npx playwright install --with-deps chromium` line (so `--with-deps` becomes a no-op safety net)
  - [x] Verify Ubuntu 24.04 `t64` suffix package names are correct
  - [x] Clean up apt lists after install (`rm -rf /var/lib/apt/lists/*`)
- [x] Task 3: Update content hash test expectations if Dockerfile.template line count changes (AC: #1)
  - [x] Check if any existing tests assert exact line numbers or specific content ordering in `.sandbox-dockerfile`
  - [x] Update any affected test assertions
- [x] Task 4: Add Story 7.1 tests to `tests/test_sandbox.sh` (AC: #1, #2)
  - [x] Test: Dockerfile generated with `mcp: [playwright]` contains the Playwright system dependency packages
  - [x] Test: System deps are installed before `npx playwright install` in the generated Dockerfile
  - [x] Test: No Playwright system deps when MCP not configured

## Dev Notes

### Root Cause Analysis

The `--with-deps` flag on `npx playwright install --with-deps chromium` is designed to install system dependencies automatically via `apt-get`. However, in the current Dockerfile.template, the apt package cache (`/var/lib/apt/lists/*`) is cleaned after each `RUN` layer. When `--with-deps` runs, it calls `apt-get install` internally but the package lists are stale/missing, causing silent failures or partial installs.

The fix is to explicitly install the required system libraries in the same `RUN` layer, making `--with-deps` a redundant safety net rather than the primary mechanism.

### What Already Exists

- **Dockerfile.template:75-78** -- the Playwright MCP block:
  ```dockerfile
  # {{IF_MCP_PLAYWRIGHT}}
  ENV PLAYWRIGHT_BROWSERS_PATH=/opt/playwright-browsers
  RUN npm install -g @playwright/mcp@{{MCP_PLAYWRIGHT_VERSION}} && npx playwright install --with-deps chromium
  # {{/IF_MCP_PLAYWRIGHT}}
  ```
- **sandbox.sh:389-393** -- Playwright MCP version pinned to `0.0.68`, manifest JSON generated
- **sandbox.sh:321-329** -- Template processing: strips or keeps the `IF_MCP_PLAYWRIGHT` block based on config
- **Tests (line ~2985)** -- Story 5.1 tests already verify the Playwright block content in generated Dockerfiles

### Implementation Approach

Modify the `IF_MCP_PLAYWRIGHT` block in `Dockerfile.template` to add explicit system library installation. The recommended change:

```dockerfile
# {{IF_MCP_PLAYWRIGHT}}
ENV PLAYWRIGHT_BROWSERS_PATH=/opt/playwright-browsers
RUN apt-get update && apt-get install -y --no-install-recommends \
    libnspr4 libnss3 libatk1.0-0t64 libatk-bridge2.0-0t64 \
    libdbus-1-3 libcups2t64 libxcb1 libxkbcommon0 libatspi2.0-0t64 \
    libx11-6 libxcomposite1 libxdamage1 libxext6 libxfixes3 libxrandr2 \
    libgbm1 libcairo2 libpango-1.0-0 libasound2t64 \
    && rm -rf /var/lib/apt/lists/*
RUN npm install -g @playwright/mcp@{{MCP_PLAYWRIGHT_VERSION}} && npx playwright install --with-deps chromium
# {{/IF_MCP_PLAYWRIGHT}}
```

**IMPORTANT:** Before committing to this package list, the developer MUST verify the actual missing libraries by:
1. Building the current image: `sandbox build` with Playwright config
2. Running inside it: `docker run --rm <image> ldd /opt/playwright-browsers/chromium-*/chrome-linux/chrome 2>&1 | grep "not found"`
3. Mapping the missing `.so` files to Ubuntu 24.04 packages using `apt-file search <lib>.so`

The package list in the acceptance criteria is from the sprint change proposal and may need adjustment based on actual `ldd` output. Some libraries may already be present, others may be missing from this list.

### Ubuntu 24.04 Package Name Notes

Ubuntu 24.04 (Noble) uses `t64` suffixes on some library packages as part of the 64-bit time_t migration. Key mappings:
- `libatk1.0-0t64` (not `libatk1.0-0`)
- `libatk-bridge2.0-0t64` (not `libatk-bridge2.0-0`)
- `libcups2t64` (not `libcups2`)
- `libatspi2.0-0t64` (not `libatspi2.0-0`)
- `libasound2t64` (not `libasound2`)

Other packages (libnspr4, libnss3, libxcb1, etc.) do NOT have the `t64` suffix. Get the exact names from `apt-cache search` inside an Ubuntu 24.04 container.

### Architecture Compliance

- **Dockerfile.template changes** only within the existing `IF_MCP_PLAYWRIGHT` conditional block -- no structural changes
- **Template processing** in `sandbox.sh` is unchanged -- the block markers remain, sandbox.sh strips/keeps the whole block as before
- **Content hash** will change because `Dockerfile.template` is a hash input -- this correctly triggers a rebuild
- **No changes to sandbox.sh** -- all changes are in `Dockerfile.template` and `tests/test_sandbox.sh`

### File Modifications Required

| File | Change |
|------|--------|
| `Dockerfile.template` | Add explicit `apt-get install` for Playwright system libraries inside `IF_MCP_PLAYWRIGHT` block |
| `tests/test_sandbox.sh` | Add Story 7.1 tests verifying system deps in generated Dockerfile |

### Test Strategy

Follow existing story 5.1 test patterns (line ~2985 in `tests/test_sandbox.sh`):

1. Build with `mcp: [playwright]` config (reuse the existing 5.1 test build or create a new one)
2. Read `.sandbox-dockerfile` content
3. Assert system dependency packages appear in the generated Dockerfile
4. Assert the system deps `apt-get` line appears before the `npx playwright install` line
5. Build without MCP config and assert no Playwright system deps appear

Test naming convention: `7.1: <description>`

### Anti-Patterns to Avoid

- Do NOT remove `--with-deps` from the `npx playwright install` command -- keep it as a safety net
- Do NOT add system deps outside the `IF_MCP_PLAYWRIGHT` block -- they should only be installed when Playwright is configured
- Do NOT merge the system deps `apt-get` with the common CLI tools `apt-get` (line 10-18) -- keep Playwright deps conditional
- Do NOT modify `sandbox.sh` -- this story is entirely about `Dockerfile.template` changes
- Do NOT modify `scripts/entrypoint.sh` -- not relevant to this story
- Do NOT hardcode the package list without verifying via `ldd` first -- the sprint proposal list is a starting point, not a guarantee

### Previous Story Intelligence

From story 5-1 (MCP server installation):
- Playwright MCP version pinned to `0.0.68` via `sandbox.sh:390`
- `PLAYWRIGHT_BROWSERS_PATH=/opt/playwright-browsers` is set as ENV in the Dockerfile
- `npx playwright install --with-deps chromium` installs Chromium browser binaries
- MCP block comes BEFORE `useradd sandbox` (root ownership enforced)
- Tests use `assert_contains`/`assert_not_contains` on `.sandbox-dockerfile` content

From story 5-2 (MCP configuration):
- `jq` was added to common CLI tools in Dockerfile.template
- Test count was 429 assertions at that point
- Review feedback pattern: `with review fixes` in commit message

### Git Conventions

- Commit format: `feat: add Playwright system dependencies to sandbox image (story 7-1)`
- Include `with review fixes` if code review feedback is incorporated

### Project Structure Notes

- `Dockerfile.template` is at project root -- human-readable template, NOT a valid Dockerfile
- `.sandbox-dockerfile` is the generated/resolved Dockerfile (used by tests, gitignored)
- Template conditional blocks use `# {{IF_NAME}}` / `# {{/IF_NAME}}` format as Dockerfile comments
- The `IF_MCP_PLAYWRIGHT` block is currently lines 75-78 of `Dockerfile.template`

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 7.1] Story requirements and acceptance criteria
- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-03-25.md] Root cause analysis and implementation notes
- [Source: _bmad-output/planning-artifacts/architecture.md] Dockerfile generation patterns, template marker format
- [Source: Dockerfile.template:75-78] Current Playwright MCP block
- [Source: sandbox.sh:321-329, 389-393] Template processing for Playwright block
- [Source: tests/test_sandbox.sh:2985-3073] Existing Story 5.1 MCP tests (pattern to follow)
- [Source: _bmad-output/implementation-artifacts/5-2-mcp-configuration-generation-and-merge.md] Previous story learnings

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- Task 1: `ldd` against Chromium binary showed 0 missing libraries — `--with-deps` currently works because apt cache is still warm in the same RUN layer. However, this is fragile and depends on Playwright's internal apt-get having access to package lists.
- Task 3: No existing tests assert exact line numbers — all use `grep -n` for relative ordering comparisons. No test changes needed.

### Completion Notes List

- Verified all 19 Playwright/Chromium system libraries from AC #2 are correct Ubuntu 24.04 package names (including t64 suffixes)
- Added explicit `apt-get install` with all 19 packages inside `IF_MCP_PLAYWRIGHT` block, placed before `npx playwright install --with-deps chromium`
- `--with-deps` retained as safety net per anti-pattern guidance
- Built and verified image: no missing libraries via `ldd` check
- Added 25 new test assertions (19 package presence + 1 ordering + 1 build success + 3 absence checks + 1 no-mcp build success)
- Full test suite: 482/482 passed, 0 failed — no regressions

### File List

- `Dockerfile.template` — Added explicit system library apt-get install inside IF_MCP_PLAYWRIGHT block (lines 77-82)
- `tests/test_sandbox.sh` — Added Story 7.1 test section (25 assertions: package presence, ordering, and absence)

### Change Log

- 2026-03-25: Implemented Story 7.1 — Added explicit Playwright system dependency installation to Dockerfile.template and corresponding tests
