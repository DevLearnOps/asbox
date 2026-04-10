# Story 5.1: MCP Server Installation at Build Time

Status: done

## Story

As a developer,
I want to configure which MCP servers are pre-installed in my sandbox image,
So that the agent has browser automation and other MCP capabilities available immediately.

## Acceptance Criteria

1. **Given** a config with `mcp: [playwright]`, **When** the sandbox image is built, **Then** the Playwright MCP package (`@playwright/mcp`) is installed in the image along with its browser dependencies.
2. **Given** MCP servers are installed at build time, **When** inspecting the image, **Then** a manifest file at `/etc/sandbox/mcp-servers.json` lists all installed MCP servers and their startup commands.
3. **Given** a config with no MCP servers specified, **When** the sandbox image is built, **Then** no MCP packages are installed and the manifest is empty (`{"mcpServers": {}}`).

## Tasks / Subtasks

- [x] Task 1: Add MCP conditional block to Dockerfile.template (AC: #1)
  - [x] Add `# {{IF_MCP_PLAYWRIGHT}}` / `# {{/IF_MCP_PLAYWRIGHT}}` block after SDK installations
  - [x] Install Node.js if not already present (Playwright MCP requires npx)
  - [x] Install `@playwright/mcp` globally via npm
  - [x] Install Playwright browser dependencies via `npx playwright install --with-deps chromium`
  - [x] Write manifest entry to `/etc/sandbox/mcp-servers.json`
- [x] Task 2: Add MCP template processing to `process_template()` in sandbox.sh (AC: #1, #3)
  - [x] Add `IF_MCP_PLAYWRIGHT` conditional block processing (same pattern as IF_NODE/IF_PYTHON/IF_GO)
  - [x] Enable block when `CFG_MCP` array contains "playwright"
  - [x] Strip block when playwright not in CFG_MCP
- [x] Task 3: Generate `/etc/sandbox/mcp-servers.json` manifest (AC: #2, #3)
  - [x] When MCP servers are configured: write JSON manifest with server entries
  - [x] When no MCP servers: write empty manifest `{"mcpServers": {}}`
  - [x] Manifest must always exist (entrypoint in story 5-2 will read it unconditionally)
- [x] Task 4: Add tests to test_sandbox.sh (AC: #1, #2, #3)
  - [x] Test: Playwright block present in generated Dockerfile when `mcp: [playwright]`
  - [x] Test: Playwright block absent when no MCP configured
  - [x] Test: Manifest file creation directive present in Dockerfile
  - [x] Test: npm install of @playwright/mcp in Dockerfile
  - [x] Test: Playwright browser install command in Dockerfile
  - [x] Test: Empty manifest when no MCP configured
  - [x] Test: Node.js dependency check for Playwright

### Review Findings

- [x] [Review][Decision] MCPManifestJSON hardcoded to playwright only — fixed: builds manifest dynamically from cfg.MCP via MCPServerRegistry + json.Marshal [internal/config/config.go]
- [x] [Review][Patch] `hasMCP` template func duplicates `HasMCP` method — fixed: removed template func, use `.HasMCP` directly in template [internal/template/render.go, embed/Dockerfile.tmpl]
- [x] [Review][Patch] Duplicate MCP entries silently accepted — fixed: added dedup check in parse.go validation [internal/config/parse.go]
- [x] [Review][Patch] Error message "Supported: playwright" hardcoded separately from validMCPServers map — fixed: generated from MCPServerRegistry keys [internal/config/parse.go]
- [x] [Review][Defer] Misleading error message prefix in render.go — both parse and FS-read failures use "failed to render Dockerfile" prefix. Pre-existing. [internal/template/render.go:15-17]

## Dev Notes

### What Already Exists

- **Config parsing done**: `sandbox.sh:170-178` already reads `CFG_MCP` array from `mcp:` config key via yq. No changes needed to `parse_config()`.
- **Config template ready**: `templates/config.yaml:35-37` already has the `mcp: - playwright` entry (commented out).
- **Conditional block pattern established**: `Dockerfile.template:49-64` shows the `{{IF_NODE}}`/`{{IF_PYTHON}}`/`{{IF_GO}}` pattern. MCP blocks follow the same pattern.
- **Template processing pattern**: `sandbox.sh:289-311` shows conditional block processing for SDKs. MCP processing follows identical pattern.
- **Test infrastructure**: `tests/test_sandbox.sh` has 385 assertions with `assert_contains`/`assert_not_contains` helpers and Dockerfile content verification via `.sandbox-dockerfile`.

### Architecture Compliance

**Dockerfile.template placement**: Add MCP block AFTER SDK installations (line 64) but BEFORE isolation script COPY (line 66). Playwright MCP requires Node.js + npx, so it must come after `{{IF_NODE}}` block. However, since Node.js might not be configured as an SDK, the MCP block must handle its own Node.js dependency.

**Manifest format** (`/etc/sandbox/mcp-servers.json`):
```json
{
  "mcpServers": {
    "playwright": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@playwright/mcp"]
    }
  }
}
```

This is the standard MCP protocol format (NFR9). The entrypoint (story 5-2) will read this manifest and generate `.mcp.json` in the workspace.

**Empty manifest** (when no MCP configured):
```json
{
  "mcpServers": {}
}
```

The manifest MUST always be created. Use a RUN directive outside any conditional to write the empty base, then conditionals append/overwrite.

### Implementation Strategy

**Node.js dependency for Playwright MCP**: The `@playwright/mcp` package requires Node.js/npx. Two cases:
1. User has `sdks.nodejs` configured -- Node.js already installed by `{{IF_NODE}}` block
2. User has `mcp: [playwright]` but no `sdks.nodejs` -- must install Node.js within the MCP block

Recommended approach: In the `{{IF_MCP_PLAYWRIGHT}}` block, check if Node.js is already available. If not, install a minimal Node.js. Use `command -v node || ...` pattern in the RUN directive. Alternatively, since Dockerfile template is generated at build time and `parse_config()` already knows both `CFG_SDK_NODEJS` and `CFG_MCP`, you can add a validation in `sandbox.sh` that warns or auto-includes Node.js when playwright is configured.

**Simpler approach**: Just require Node.js for Playwright. Add a validation check in `sandbox.sh` (after `parse_config()`) that if "playwright" is in `CFG_MCP` and `CFG_SDK_NODEJS` is empty, emit an error: `die "mcp server 'playwright' requires sdks.nodejs to be configured" 1`. This keeps the Dockerfile simple and avoids duplicate Node.js installations.

### Template Processing Pattern (follow exactly)

In `process_template()`, after the IF_GO block (line 311), add:

```bash
# IF_MCP_PLAYWRIGHT
local has_mcp_playwright=""
for _mcp in "${CFG_MCP[@]:-}"; do
  if [[ "${_mcp}" == "playwright" ]]; then has_mcp_playwright="yes"; break; fi
done
if [[ -n "${has_mcp_playwright}" ]]; then
  template="$(echo "${template}" | sed '/^# {{IF_MCP_PLAYWRIGHT}}$/d; /^# {{\/IF_MCP_PLAYWRIGHT}}$/d')"
else
  template="$(echo "${template}" | sed '/^# {{IF_MCP_PLAYWRIGHT}}$/,/^# {{\/IF_MCP_PLAYWRIGHT}}$/d')"
fi
```

### Manifest Generation Approach

Two options for manifest creation in the Dockerfile:

**Option A (recommended)**: Generate manifest content in `sandbox.sh` during template processing and inject it as a value placeholder `{{MCP_MANIFEST_JSON}}`:
```dockerfile
RUN echo '{{MCP_MANIFEST_JSON}}' > /etc/sandbox/mcp-servers.json
```

**Option B**: Build manifest inside the MCP conditional block and use a base empty manifest outside it. This is more complex and harder to test.

Go with Option A: compute the JSON in `sandbox.sh`, escape it for sed substitution, and inject it. This keeps the Dockerfile simple and the manifest content testable.

### Content Hash

The content hash computation (`sandbox.sh:213-241`) hashes `config.yaml`, `Dockerfile.template`, `scripts/entrypoint.sh`, and `scripts/git-wrapper.sh`. No new files are being added to the content hash inputs -- changes to `Dockerfile.template` and `config.yaml` already trigger rebuilds.

### Test Pattern (follow existing conventions)

Tests verify generated Dockerfile content from `.sandbox-dockerfile`. Example pattern from story 4-3:
```bash
assert_contains "${dockerfile_content}" "COPY scripts/entrypoint.sh" "Dockerfile copies entrypoint script"
```

For story 5-1, test against the generated Dockerfile content:
```bash
# With playwright MCP
assert_contains "${dockerfile_content}" "@playwright/mcp" "Dockerfile installs Playwright MCP package"
assert_contains "${dockerfile_content}" "playwright install" "Dockerfile installs Playwright browsers"
assert_contains "${dockerfile_content}" "mcp-servers.json" "Dockerfile creates MCP manifest"

# Without MCP
assert_not_contains "${dockerfile_no_mcp}" "@playwright/mcp" "No MCP without config"
```

You will need test configs with and without `mcp:` entries.

### File Modifications Required

| File | Change |
|------|--------|
| `Dockerfile.template` | Add `{{IF_MCP_PLAYWRIGHT}}` block, add manifest RUN directive |
| `sandbox.sh` | Add MCP conditional processing in `process_template()`, add manifest JSON generation, add Node.js validation for playwright |
| `tests/test_sandbox.sh` | Add MCP installation and manifest tests |

### Anti-Patterns to Avoid

- Do NOT install Playwright MCP at runtime (entrypoint) -- it must be at build time per FR41
- Do NOT use `eval` for JSON generation -- use printf or heredoc
- Do NOT add MCP manifest to content hash inputs -- it's derived from config.yaml which is already hashed
- Do NOT modify `parse_config()` -- MCP parsing already works
- Do NOT modify `entrypoint.sh` -- that's story 5-2 scope
- Do NOT add `.mcp.json` generation -- that's story 5-2 scope
- Do NOT create new helper scripts -- keep changes in existing files

### Previous Story Intelligence

From story 4-3 (most recent):
- Isolation scripts are COPY'd without `--chown` so root owns them (security pattern)
- COPY must come before `useradd` for root ownership -- MCP installation blocks should also be before `useradd` (line 72)
- Test assertions follow TAP-like format with `assert_contains`/`assert_not_contains`
- Total test count was 385 after 4-3; story 5-1 will add to this

### Git Conventions

- Commit format: `feat: <description> (story 5-1)`
- Include review feedback mention if applicable

### References

- [Source: _bmad-output/planning-artifacts/epics.md] Epic 5, Story 5.1 acceptance criteria
- [Source: _bmad-output/planning-artifacts/architecture.md] MCP Integration Decision, MCP Metadata Flow, Dockerfile Template Patterns
- [Source: _bmad-output/planning-artifacts/prd.md] FR3, FR29, FR30, FR41, NFR9
- [Source: sandbox.sh:170-178] Existing MCP config parsing
- [Source: sandbox.sh:261-362] Template processing function
- [Source: Dockerfile.template:49-64] Existing conditional block pattern
- [Source: tests/test_sandbox.sh] Test assertion patterns

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- Existing MCP parse_config test updated to include `sdks.nodejs` since playwright now validates Node.js dependency

### Completion Notes List

- Task 1: Added `IF_MCP_PLAYWRIGHT` conditional block to Dockerfile.template after SDK installations and before isolation script COPY. Block installs `@playwright/mcp` globally via npm and runs `npx playwright install --with-deps chromium` for browser dependencies.
- Task 2: Added IF_MCP_PLAYWRIGHT conditional block processing in `process_template()` following the exact same pattern as IF_NODE/IF_PYTHON/IF_GO. Added validation in `cmd_build()` that dies with clear error if playwright MCP is configured without `sdks.nodejs`.
- Task 3: Manifest JSON generated in `process_template()` and injected via `{{MCP_MANIFEST_JSON}}` placeholder. Full manifest with playwright entry when configured, empty `{"mcpServers": {}}` when not. Manifest RUN directive is unconditional (always created).
- Task 4: Added 16 new tests (402 total, up from 386). Tests cover: playwright block present/absent, npm install command, browser install command, manifest creation, manifest content, empty manifest, MCP block ordering before useradd, and Node.js dependency validation error.

### File List

- `Dockerfile.template` — Added IF_MCP_PLAYWRIGHT conditional block and unconditional MCP manifest RUN directive
- `sandbox.sh` — Added MCP conditional processing in process_template(), MCP manifest JSON generation, and playwright→nodejs validation in cmd_build()
- `tests/test_sandbox.sh` — Added 16 Story 5.1 tests; updated existing MCP parse_config test to include sdks.nodejs

### Change Log

- 2026-03-25: Implemented Story 5.1 — MCP server installation at build time. Added Playwright MCP conditional Dockerfile block, manifest generation, Node.js dependency validation, and 16 tests.
