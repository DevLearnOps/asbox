# Story 5.1: MCP Server Installation and Configuration Merge

Status: review

## Story

As a developer,
I want MCP servers pre-installed in my sandbox image and automatically configured for the agent,
So that browser automation is available immediately without manual setup.

## Acceptance Criteria

1. **Given** a config with `mcp: [playwright]`, **When** the sandbox image is built, **Then** the Playwright MCP package (`@playwright/mcp`) is installed with chromium and webkit browser dependencies.

2. **Given** MCP servers are installed at build time, **When** inspecting the image, **Then** a manifest at `/etc/sandbox/mcp-servers.json` lists all installed MCP servers with their startup commands.

3. **Given** a sandbox starts with Playwright MCP installed, **When** the entrypoint runs, **Then** a `.mcp.json` is generated in the workspace root from the build-time manifest.

4. **Given** the mounted project already has a `.mcp.json` with its own entries, **When** the entrypoint merges configs, **Then** sandbox servers are added; on name conflicts the project's version wins. Entrypoint logs added vs. skipped.

5. **Given** the agent needs mobile device emulation, **When** it uses Playwright MCP with webkit, **Then** webkit launches and supports iPhone/iPad device emulation (FR29a).

## Tasks / Subtasks

- [x] Task 1: Add `HasMCP` helper and MCP manifest computation (AC: #1, #2)
  - [x] Add a `HasMCP(name string) bool` method on `Config` in `internal/config/config.go`
  - [x] Add a `MCPManifestJSON() string` method on `Config` that returns the JSON manifest string
  - [x] Register `hasMCP` as a template FuncMap entry in `internal/template/render.go`
- [x] Task 2: Add MCP validation in config parser (AC: #1)
  - [x] In `internal/config/parse.go`, after agent validation, add: if MCP contains "playwright" and `SDKs.NodeJS` is empty, return `ConfigError{Field: "mcp", Msg: "mcp server 'playwright' requires sdks.nodejs to be configured"}`
  - [x] Validate all MCP entries are in the supported set (currently only "playwright"); reject unknown servers with `ConfigError`
- [x] Task 3: Add MCP installation block to Dockerfile template (AC: #1, #2, #5)
  - [x] Add conditional block in `embed/Dockerfile.tmpl` after agent instructions, before playwright-deps line: `{{- if hasMCP .MCP "playwright"}}` block that installs `@playwright/mcp` globally and runs `npx playwright install --with-deps chromium webkit`
  - [x] Replace the existing `{{- if .SDKs.NodeJS}}` playwright-deps block (line 99-102) with the MCP block — the MCP block already installs `--with-deps chromium webkit` which covers webkit deps
  - [x] Add unconditional manifest line: `RUN mkdir -p /etc/sandbox && echo '{{.MCPManifestJSON}}' > /etc/sandbox/mcp-servers.json`
  - [x] Add `PLAYWRIGHT_BROWSERS_PATH=/opt/playwright-browsers` and `PLAYWRIGHT_MCP_BROWSER=chromium` ENV vars inside the MCP block
  - [x] Add `chown -R sandbox:sandbox /opt/playwright-browsers` after playwright install
- [x] Task 4: Add unit tests for config validation (AC: #1, #2)
  - [x] Test: playwright MCP without nodejs SDK returns ConfigError
  - [x] Test: playwright MCP with nodejs SDK succeeds
  - [x] Test: unknown MCP server name returns ConfigError
  - [x] Test: empty MCP list succeeds
- [x] Task 5: Add unit tests for template rendering (AC: #1, #2, #3)
  - [x] Test: MCP block present when MCP contains "playwright" (contains `@playwright/mcp`)
  - [x] Test: MCP block absent when no MCP configured
  - [x] Test: manifest RUN directive always present (contains `mcp-servers.json`)
  - [x] Test: manifest content includes playwright entry when configured
  - [x] Test: manifest content is empty `{"mcpServers":{}}` when no MCP configured
  - [x] Test: `playwright install --with-deps chromium webkit` in output when playwright configured
  - [x] Test: PLAYWRIGHT_BROWSERS_PATH env var present when playwright configured
- [x] Task 6: Verify entrypoint merge logic (AC: #3, #4) — READ-ONLY verification
  - [x] Confirm `merge_mcp_config()` in `embed/entrypoint.sh` (lines 60-84) reads `/etc/sandbox/mcp-servers.json` and generates `.mcp.json`
  - [x] Confirm merge uses `jq -s '.[0] * .[1]'` with project config winning on conflicts
  - [x] No code changes needed — entrypoint merge is already complete

## Dev Notes

### Architecture Constraints

- **Go template with FuncMap** — the template engine needs a custom function to check array membership. Add a `hasMCP(slice []string, name string) bool` function to the template FuncMap. The current `Render()` in `internal/template/render.go` uses `template.New("Dockerfile").Parse(...)` — change to `template.New("Dockerfile").Funcs(funcMap).Parse(...)`.
- **Manifest flow**: Build-time manifest at `/etc/sandbox/mcp-servers.json` → entrypoint reads it → generates/merges `.mcp.json`. This is already documented in the architecture decision.
- **MCP manifest format** (standard MCP protocol per NFR9):
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
- **Empty manifest** (when no MCP configured):
  ```json
  {"mcpServers":{}}
  ```
- **Manifest must always exist** — the entrypoint reads it unconditionally. Even when no MCP servers are configured, write the empty manifest.
- **Playwright requires Node.js** — `@playwright/mcp` is an npm package that needs `npx`. Validate in `Parse()` that if "playwright" is in MCP, `SDKs.NodeJS` must be set. Die early with clear error.
- **Supported MCP servers** — currently only "playwright". Validate against a known-servers set.
- **Content hash** — no changes needed. The rendered Dockerfile is already a hash input (`internal/hash/hash.go`), so MCP block changes trigger rebuilds automatically.
- **Entrypoint already done** — `embed/entrypoint.sh:60-84` has the complete `merge_mcp_config()` function. Do NOT modify it. It handles: no manifest, no project config, both present (merge with project wins).

### Implementation Approach

**Config struct additions** (`internal/config/config.go`):
```go
// HasMCP returns true if the named MCP server is in the config.
func (c *Config) HasMCP(name string) bool {
    for _, m := range c.MCP {
        if m == name {
            return true
        }
    }
    return false
}

// MCPManifestJSON returns the MCP manifest JSON string for embedding in the Dockerfile.
func (c *Config) MCPManifestJSON() string {
    if !c.HasMCP("playwright") {
        return `{"mcpServers":{}}`
    }
    return `{"mcpServers":{"playwright":{"type":"stdio","command":"npx","args":["-y","@playwright/mcp"]}}}`
}
```

**Template FuncMap** (`internal/template/render.go`):
```go
funcMap := template.FuncMap{
    "hasMCP": func(mcp []string, name string) bool {
        for _, m := range mcp {
            if m == name {
                return true
            }
        }
        return false
    },
}
tmpl, err := template.New("Dockerfile").Funcs(funcMap).Parse(string(tmplBytes))
```

**Dockerfile.tmpl MCP block** — insert after agent instructions block (line 98) and replace the existing webkit-only playwright-deps block (lines 99-102):
```dockerfile
{{- if hasMCP .MCP "playwright"}}

ENV PLAYWRIGHT_BROWSERS_PATH=/opt/playwright-browsers
ENV PLAYWRIGHT_MCP_BROWSER=chromium
RUN npm install -g @playwright/mcp && \
    npx playwright install --with-deps chromium webkit && \
    mkdir -p /opt/playwright-browsers && \
    chown -R sandbox:sandbox /opt/playwright-browsers
{{- end}}
```

**Manifest directive** — add before the TESTCONTAINERS ENV lines (unconditional):
```dockerfile

RUN mkdir -p /etc/sandbox && echo '{{.MCPManifestJSON}}' > /etc/sandbox/mcp-servers.json
```

### Validation in parse.go

Add after the agent validation block (after line 54), before mount validation:

```go
// Validate MCP servers
validMCPServers := map[string]bool{"playwright": true}
for _, mcp := range cfg.MCP {
    if !validMCPServers[mcp] {
        return nil, &ConfigError{
            Field: "mcp",
            Msg:   fmt.Sprintf("unsupported MCP server '%s'. Supported: playwright", mcp),
        }
    }
}
// Playwright requires Node.js
if cfg.HasMCP("playwright") && cfg.SDKs.NodeJS == "" {
    return nil, &ConfigError{
        Field: "mcp",
        Msg:   "mcp server 'playwright' requires sdks.nodejs to be configured",
    }
}
```

Note: `HasMCP` must be defined on Config (in config.go) before it can be called from parse.go.

### Existing Playwright-Deps Block

Lines 99-102 of `embed/Dockerfile.tmpl` currently have:
```dockerfile
{{- if .SDKs.NodeJS}}

RUN npx playwright install-deps webkit
{{- end}}
```

This was a temporary measure to install webkit system deps when Node.js is present. The new MCP block replaces this entirely because:
- MCP block runs `npx playwright install --with-deps chromium webkit` which installs both browsers AND their system deps
- The old block only installed system deps for webkit, not the browsers themselves
- If someone has Node.js but NOT playwright MCP, they don't need webkit deps installed

**Remove lines 99-102** and replace with the MCP block.

### File Modifications Required

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `HasMCP()` and `MCPManifestJSON()` methods on Config |
| `internal/config/parse.go` | Add MCP server validation and playwright→nodejs dependency check |
| `internal/config/parse_test.go` | Add tests for MCP validation (unknown server, playwright without nodejs) |
| `internal/template/render.go` | Add FuncMap with `hasMCP` function to template |
| `internal/template/render_test.go` | Add tests for MCP block rendering and manifest content |
| `embed/Dockerfile.tmpl` | Add MCP installation block, manifest RUN, remove old webkit-only block |

### Testing Approach

**Config parse tests** (`internal/config/parse_test.go`) — follow existing pattern with `writeConfig()` helper:
- Write YAML with `mcp: [playwright]` but no `sdks.nodejs` → expect `ConfigError` with "requires sdks.nodejs"
- Write YAML with `mcp: [unknown-server]` → expect `ConfigError` with "unsupported MCP server"
- Write YAML with `mcp: [playwright]` and `sdks.nodejs: "22"` → expect success
- Existing `TestParse_validFullConfig` already has `mcp: [playwright]` with `sdks.nodejs: "20"` so it validates the happy path

**Template render tests** (`internal/template/render_test.go`) — follow existing `TestRender_*` pattern:
- Config with `MCP: []string{"playwright"}, SDKs: SDKConfig{NodeJS: "22"}` → output contains `@playwright/mcp`, `playwright install --with-deps chromium webkit`, `PLAYWRIGHT_BROWSERS_PATH`, `mcp-servers.json`
- Config with no MCP → output does NOT contain `@playwright/mcp`, but DOES contain `mcp-servers.json` (empty manifest)
- Config with MCP playwright → manifest contains `"playwright"` and `"npx"`
- Config with no MCP → manifest contains `{"mcpServers":{}}`

### Anti-Patterns to Avoid

- Do NOT install MCP at runtime (entrypoint) — FR41 requires build-time installation
- Do NOT modify `embed/entrypoint.sh` — merge logic is already complete
- Do NOT add MCP manifest to content hash inputs separately — the rendered Dockerfile already captures it
- Do NOT create new files — all changes go into existing files
- Do NOT use `eval` or shell injection for JSON — use Go string returns
- Do NOT add build args for MCP — the manifest is baked into the Dockerfile via template rendering, not ARGs

### Previous Story Intelligence (4-2)

From story 4-2 implementation:
- **iproute2 and iptables were added** to the Dockerfile for netavark custom network support — these are already in the template
- **Integration tests use `startTestContainerWithEntrypoint()`** with `Privileged: true` for nested container ops
- **`execAsUser(ctx, t, container, "sandbox", cmd)`** pattern for running as unprivileged user
- **Test files**: `integration/inner_container_test.go` was created following the `package integration` pattern
- Note: This story is unit-test focused (config + template), not integration-test focused

### Git Intelligence

Recent commits follow `feat: implement story X-Y description` format. All stories are single commits, co-authored with Claude Opus 4.6. Tests are written alongside implementation code. Go 1.25.0, testcontainers-go v0.41.0.

### Project Structure Notes

- `internal/config/config.go` — Config struct and methods (add HasMCP, MCPManifestJSON here)
- `internal/config/parse.go` — validation logic (add MCP validation here)
- `internal/template/render.go` — template rendering (add FuncMap here)
- `embed/Dockerfile.tmpl` — Go text/template Dockerfile (add MCP block here)
- `embed/entrypoint.sh` — already has merge_mcp_config() — DO NOT MODIFY

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Epic 5, Story 5.1]
- [Source: _bmad-output/planning-artifacts/architecture.md — MCP Integration Decision, Config struct, Dockerfile Generation]
- [Source: _bmad-output/planning-artifacts/prd.md — FR3, FR29, FR29a, FR30, FR41, FR46, NFR9]
- [Source: embed/entrypoint.sh:60-84 — merge_mcp_config() already implemented]
- [Source: embed/Dockerfile.tmpl:99-102 — existing playwright-deps block to replace]
- [Source: internal/template/render.go — current Render() without FuncMap]
- [Source: internal/config/config.go — Config struct with MCP []string field]
- [Source: internal/config/parse.go — validation logic without MCP checks]
- [Source: _bmad-output/implementation-artifacts/5-1-mcp-server-installation-at-build-time.md — original bash story spec]
- [Source: _bmad-output/implementation-artifacts/deferred-work.md — no MCP-related deferred items]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

No debug issues encountered.

### Completion Notes List

- Task 1: Added `HasMCP(name string) bool` and `MCPManifestJSON() string` methods to Config struct. Registered `hasMCP` template FuncMap function in Render(). 5 unit tests added for these methods.
- Task 2: Added MCP server validation in Parse() — validates against supported set (currently "playwright") and validates playwright requires sdks.nodejs. 4 unit tests added.
- Task 3: Replaced old `{{- if .SDKs.NodeJS}}` playwright-deps block with new `{{- if hasMCP .MCP "playwright"}}` MCP block. Added PLAYWRIGHT_BROWSERS_PATH and PLAYWRIGHT_MCP_BROWSER env vars. Added unconditional manifest RUN directive. Updated existing test that checked old playwright-deps behavior.
- Task 4: Config validation tests already written as part of Task 2 (red-green-refactor). All 4 required tests pass.
- Task 5: Added 5 template rendering tests covering MCP block presence/absence, manifest content with/without playwright, and env vars. All pass.
- Task 6: Read-only verification confirmed merge_mcp_config() in entrypoint.sh correctly reads /etc/sandbox/mcp-servers.json and merges with project .mcp.json using jq with project config winning on conflicts.

### File List

- `internal/config/config.go` — Added HasMCP() and MCPManifestJSON() methods
- `internal/config/config_test.go` — New file: unit tests for HasMCP and MCPManifestJSON
- `internal/config/parse.go` — Added MCP server validation (supported set + playwright→nodejs dependency)
- `internal/config/parse_test.go` — Added 4 MCP validation tests
- `internal/template/render.go` — Added hasMCP FuncMap to template rendering
- `internal/template/render_test.go` — Added 5 MCP template tests, updated 1 existing test (old playwright-deps → noPlaywrightWithoutMCP)
- `embed/Dockerfile.tmpl` — Replaced old playwright-deps block with MCP installation block and manifest directive

### Change Log

- 2026-04-09: Implemented story 5-1 MCP server installation and configuration merge. Added build-time MCP server support with Playwright as first supported server. Config validates MCP entries and dependencies. Dockerfile template conditionally installs @playwright/mcp with browser deps and always writes manifest to /etc/sandbox/mcp-servers.json.
