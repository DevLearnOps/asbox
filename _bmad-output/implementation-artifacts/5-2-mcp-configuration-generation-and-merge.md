# Story 5.2: MCP Configuration Generation and Merge

Status: review

## Story

As a developer,
I want the sandbox entrypoint to generate a `.mcp.json` in the workspace so the agent can discover and start MCP servers,
So that MCP integration works automatically without manual configuration.

## Acceptance Criteria

1. **Given** a sandbox starts with Playwright MCP installed, **When** the entrypoint runs, **Then** a `.mcp.json` is generated in the workspace root with the Playwright server configuration in standard MCP format.
2. **Given** the mounted project already has a `.mcp.json` with its own MCP server entries, **When** the entrypoint runs, **Then** sandbox MCP servers are merged into the existing file, and if a server name conflicts, the project's version takes precedence.
3. **Given** a running sandbox with `.mcp.json` in the workspace, **When** the agent starts an MCP server (e.g., Playwright) via the MCP protocol, **Then** the server starts successfully and is available for the agent to use (NFR9). *(Deferred to manual validation — requires a running agent sandbox. Automated integration testing tracked for a future iteration.)*
4. **Given** a sandbox with Playwright MCP and a running web application on an inner container, **When** the agent uses Playwright MCP to open a browser and navigate to the application, **Then** the browser renders the page and the agent can interact with it for E2E testing. *(Deferred to manual validation — requires a running agent sandbox. Automated integration testing tracked for a future iteration.)*

## Tasks / Subtasks

- [x] Task 1: Install `jq` in Dockerfile.template for JSON processing (AC: #1, #2)
  - [x] Add `jq` to the common CLI tools `apt-get install` line (line 10-12)
- [x] Task 2: Add MCP configuration generation to `entrypoint.sh` (AC: #1)
  - [x] Read manifest from `/etc/sandbox/mcp-servers.json`
  - [x] If manifest has servers (mcpServers is not empty `{}`), generate `.mcp.json` in workspace root (`$PWD`)
  - [x] If manifest is empty, skip .mcp.json generation (no servers to add)
  - [x] Log which servers were written to .mcp.json
- [x] Task 3: Add merge logic when project already has `.mcp.json` (AC: #2)
  - [x] If `.mcp.json` already exists in workspace, read it
  - [x] Merge sandbox servers into existing file: for each server in manifest, add only if key does NOT exist in project's mcpServers
  - [x] Project's version takes precedence on name conflicts
  - [x] Log which servers were added vs. skipped (conflict)
  - [x] Write merged result back to `.mcp.json`
- [x] Task 4: Add tests to `tests/test_sandbox.sh` (AC: #1, #2, #3)
  - [x] Test: entrypoint.sh contains mcp-servers.json read logic
  - [x] Test: entrypoint.sh contains .mcp.json write logic
  - [x] Test: entrypoint.sh contains merge/conflict logic
  - [x] Test: jq is in Dockerfile common tools
  - [x] Test: Dockerfile template generates valid entrypoint with MCP logic

## Dev Notes

### What Already Exists

- **MCP manifest at build time**: `Dockerfile.template:80` writes `/etc/sandbox/mcp-servers.json` with server entries (story 5-1). Format: `{"mcpServers": {"playwright": {"type": "stdio", "command": "npx", "args": ["-y", "@playwright/mcp"]}}}` or empty `{"mcpServers": {}}`.
- **Entrypoint script**: `scripts/entrypoint.sh` (44 lines) handles rootful init, Podman setup, and agent exec. **No MCP logic exists yet** -- this is where the new code goes.
- **Workspace location**: The container working directory is set to the first mount target (default `/workspace`) via `-w` flag in `sandbox.sh:499`. Inside entrypoint.sh, `$PWD` is the workspace root.
- **Test infrastructure**: `tests/test_sandbox.sh` has 366 assertions. Uses `assert_contains`/`assert_not_contains` helpers and verifies `.sandbox-dockerfile` content. Story 5-1 tests start at line 2985.
- **Config parsing**: `sandbox.sh:170-178` reads `CFG_MCP` array. No changes needed to parsing.

### Architecture Compliance

**MCP `.mcp.json` format** (standard MCP project configuration per architecture.md and Claude Code docs):
```json
{
  "mcpServers": {
    "playwright": {
      "command": "npx",
      "args": ["-y", "@playwright/mcp"]
    }
  }
}
```

Note: The build-time manifest at `/etc/sandbox/mcp-servers.json` includes `"type": "stdio"` in entries. The `type` field is accepted but optional in `.mcp.json` -- it can be preserved or stripped. Either way is fine.

**Merge strategy** (architecture.md Gap 6): Sandbox MCP servers are added to the project's existing `.mcp.json`. If a server name conflicts (same key already exists in project config), the project's version takes precedence. Log which servers were added vs. skipped to stdout.

**Entrypoint placement**: The MCP configuration generation must happen AFTER the rootful init / Podman setup but BEFORE the `exec` into the agent. The agent needs `.mcp.json` to exist when it starts. Insert the new code between the Podman init block (line 24) and the agent validation (line 26).

### Implementation Strategy

**JSON processing with `jq`**: The entrypoint needs to read JSON (manifest), potentially read/merge JSON (existing .mcp.json), and write JSON. `jq` is the standard bash tool for this. It must be added to the Dockerfile.template common CLI tools line.

**Recommended entrypoint code structure** (insert between lines 24 and 26 of `scripts/entrypoint.sh`):

```bash
# Generate .mcp.json from build-time manifest
MCP_MANIFEST="/etc/sandbox/mcp-servers.json"
if [[ -f "${MCP_MANIFEST}" ]]; then
  # Check if manifest has any servers
  server_count="$(jq '.mcpServers | length' "${MCP_MANIFEST}")"
  if [[ "${server_count}" -gt 0 ]]; then
    if [[ -f ".mcp.json" ]]; then
      # Merge: project config wins on name conflicts
      # Read existing project config, add sandbox servers only if key not present
      merged="$(jq -s '
        .[0] as $project | .[1] as $sandbox |
        $project * {mcpServers: ($sandbox.mcpServers * $project.mcpServers)}
      ' ".mcp.json" "${MCP_MANIFEST}")"
      echo "${merged}" > ".mcp.json"
      # Log what happened
      added="$(jq -r --argjson proj "$(cat .mcp.json)" '
        .mcpServers | keys[] | select(. as $k | $proj.mcpServers[$k] == null)
      ' "${MCP_MANIFEST}" 2>/dev/null || true)"
      skipped="$(jq -r --argjson proj "$(cat .mcp.json)" '
        .mcpServers | keys[] | select(. as $k | $proj.mcpServers[$k] != null)
      ' "${MCP_MANIFEST}" 2>/dev/null || true)"
      # ... log added/skipped
    else
      # No existing .mcp.json -- just copy manifest
      cp "${MCP_MANIFEST}" ".mcp.json"
      echo "sandbox: generated .mcp.json with $(jq -r '.mcpServers | keys | join(", ")' "${MCP_MANIFEST}") servers"
    fi
  fi
fi
```

**IMPORTANT jq merge detail**: The merge uses `$sandbox.mcpServers * $project.mcpServers` -- since jq's `*` (recursive merge) applies right-side-wins, putting project last ensures project config wins on conflicts. Then wrapping with `$project *` preserves any non-mcpServers keys from the project file.

**Simpler alternative**: Since the merge logic is tricky to get right, consider this cleaner approach:

```bash
# For each sandbox server, add to project only if not already present
tmp_file="$(mktemp)"
cp ".mcp.json" "${tmp_file}"
for server_name in $(jq -r '.mcpServers | keys[]' "${MCP_MANIFEST}"); do
  if jq -e ".mcpServers[\"${server_name}\"]" "${tmp_file}" >/dev/null 2>&1; then
    echo "sandbox: skipping ${server_name} (project override exists)"
  else
    jq --arg name "${server_name}" --argjson config "$(jq ".mcpServers[\"${server_name}\"]" "${MCP_MANIFEST}")" \
      '.mcpServers[$name] = $config' "${tmp_file}" > "${tmp_file}.new" && mv "${tmp_file}.new" "${tmp_file}"
    echo "sandbox: added ${server_name} to .mcp.json"
  fi
done
mv "${tmp_file}" ".mcp.json"
```

Choose whichever approach is cleaner. The key requirement is: **project config wins on name conflicts**.

### Output Format

Follow the project's output conventions:
- Info to stdout: `echo "sandbox: generated .mcp.json with playwright server"`
- Use `sandbox:` prefix for entrypoint messages (consistent with existing entrypoint logging)
- No color codes

### Dockerfile.template Change

Add `jq` to the common CLI tools line (`Dockerfile.template:10-12`):

```dockerfile
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    wget \
    git \
    jq \
    dnsutils \
    ca-certificates \
    gnupg \
    && rm -rf /var/lib/apt/lists/*
```

This is a ~1MB package. It is the standard tool for JSON processing in bash scripts and is needed for the merge operation.

### Content Hash Impact

Adding `jq` to Dockerfile.template modifies that file, which IS in the content hash inputs (`sandbox.sh:213-241`). This will trigger a rebuild, which is correct -- the image contents change. Similarly, `scripts/entrypoint.sh` is a content hash input, so changes there also trigger rebuilds. No changes needed to hash computation logic.

### Test Strategy

Tests for story 5-2 should verify the entrypoint.sh file contents and the Dockerfile.template changes. Since tests use `.sandbox-dockerfile` artifact inspection (not actual container runs), focus on:

1. **Dockerfile tests**: Verify `jq` appears in the common CLI tools installation line
2. **Entrypoint tests**: Read `scripts/entrypoint.sh` directly and assert it contains the expected MCP logic:
   - References `mcp-servers.json` manifest path
   - Contains `.mcp.json` write logic
   - Contains merge/conflict handling (jq merge or loop)
   - Contains logging output for server additions

Test pattern (matches existing conventions):
```bash
entrypoint_content="$(cat "${PROJECT_ROOT}/scripts/entrypoint.sh")"
assert_contains "${entrypoint_content}" "mcp-servers.json" "5.2: entrypoint reads MCP manifest"
assert_contains "${entrypoint_content}" ".mcp.json" "5.2: entrypoint writes .mcp.json"
```

For Dockerfile tests, reuse the existing build-with-mcp config to verify jq is in the generated Dockerfile.

### File Modifications Required

| File | Change |
|------|--------|
| `Dockerfile.template` | Add `jq` to common CLI tools apt-get line (line 10-12) |
| `scripts/entrypoint.sh` | Add MCP manifest reading, .mcp.json generation, and merge logic (between lines 24 and 26) |
| `tests/test_sandbox.sh` | Add Story 5.2 tests for entrypoint MCP logic and jq in Dockerfile |

### Anti-Patterns to Avoid

- Do NOT generate `.mcp.json` at build time -- it must be at runtime in the entrypoint because the workspace is mounted at run time and may already contain a `.mcp.json`
- Do NOT modify `sandbox.sh` -- this story is entirely about `entrypoint.sh` (plus jq in Dockerfile)
- Do NOT modify the MCP manifest format or generation -- that was story 5-1 scope
- Do NOT use `eval` or `sed` for JSON manipulation -- use `jq`
- Do NOT fail if manifest file is missing or empty -- handle gracefully (the manifest always exists per story 5-1, but defensive coding is fine)
- Do NOT overwrite an existing `.mcp.json` without merging -- project config must be preserved
- Do NOT add complex error handling for jq failures -- if jq is unavailable somehow, the script will fail via `set -euo pipefail` which is the correct behavior
- Do NOT create new helper scripts -- keep changes in existing files

### Previous Story Intelligence

From story 5-1:
- MCP manifest is always created at `/etc/sandbox/mcp-servers.json` (even empty `{"mcpServers": {}}`)
- Playwright MCP version pinned to `0.0.68` at build time
- `PLAYWRIGHT_BROWSERS_PATH=/opt/playwright-browsers` env var set in Dockerfile
- Manifest format includes `"type": "stdio"` field
- The `parse_config()` function reads `CFG_MCP` array -- no changes needed
- Playwright requires `sdks.nodejs` -- enforced by validation in `sandbox.sh`

From story 4-3:
- Files COPY'd as root before `useradd` for security (root ownership)
- Entrypoint is at `/usr/local/bin/entrypoint.sh`, owned by root, not modifiable by sandbox user

### Git Conventions

- Commit format: `feat: <description> (story 5-2)`
- Include review feedback mention if applicable

### Project Structure Notes

- `scripts/entrypoint.sh` runs INSIDE the container as the `sandbox` user (after rootful init drops privileges)
- `$PWD` inside the entrypoint is the workspace root (set by `-w` flag in sandbox.sh)
- The workspace is a mounted host directory -- `.mcp.json` changes persist on host
- `jq` must be available inside the container (hence the Dockerfile change)

### References

- [Source: _bmad-output/planning-artifacts/epics.md] Epic 5, Story 5.2 acceptance criteria (lines 545-567)
- [Source: _bmad-output/planning-artifacts/architecture.md] MCP Integration Decision (lines 186-201), Gap 6 merge strategy (lines 461-463)
- [Source: _bmad-output/planning-artifacts/prd.md] FR29, FR30, FR41, NFR9
- [Source: scripts/entrypoint.sh] Current entrypoint (44 lines, no MCP logic)
- [Source: Dockerfile.template:80] MCP manifest creation
- [Source: Dockerfile.template:10-12] Common CLI tools installation
- [Source: sandbox.sh:379-388] MCP manifest JSON generation
- [Source: sandbox.sh:497-499] Workspace mount and working directory setup
- [Source: _bmad-output/implementation-artifacts/5-1-mcp-server-installation-at-build-time.md] Previous story learnings

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

### Completion Notes List

- Added `jq` to Dockerfile.template common CLI tools for JSON processing
- Implemented MCP configuration generation in `scripts/entrypoint.sh` between Podman init and agent exec
- Fresh `.mcp.json` generation: copies manifest directly when no existing file
- Merge logic: iterates sandbox servers, adds only if key not present in project config (project wins on conflicts)
- Logging: outputs `sandbox: added/skipping/generated` messages following project output conventions
- Used the "simpler alternative" loop approach from Dev Notes for clearer merge logic
- Added 8 tests (tests 407-414) verifying jq installation, manifest reading, .mcp.json writing, merge/conflict logic
- All 429 tests pass with zero regressions

### Change Log

- 2026-03-25: Implemented story 5-2 — MCP configuration generation and merge logic

### File List

- `Dockerfile.template` — Added `jq` to common CLI tools apt-get line
- `scripts/entrypoint.sh` — Added MCP manifest reading, .mcp.json generation, and merge logic (lines 26-51)
- `tests/test_sandbox.sh` — Added 8 story 5.2 tests for entrypoint MCP logic and jq in Dockerfile
