# Story 7.1: Host Agent Config Mount for OAuth Token Sync

Status: done

## Story

As a developer,
I want to mount my host agent config directory into the sandbox,
so that the agent picks up my existing authentication without re-login each session.

## Acceptance Criteria

1. **Given** a config with `host_agent_config: {source: "~/.claude", target: "/opt/claude-config"}`
   **When** the sandbox launches
   **Then** `~/.claude` is mounted read-write at `/opt/claude-config` and `CLAUDE_CONFIG_DIR=/opt/claude-config` is set

2. **Given** the host `~/.claude` contains valid OAuth tokens
   **When** the agent refreshes tokens during a session
   **Then** updated tokens are written back to the host directory via the read-write mount

3. **Given** `host_agent_config` is not configured
   **When** the sandbox launches
   **Then** no additional mount or env var is added

## Tasks / Subtasks

- [x] Task 1: Extend `mount.AssembleMounts()` to include `host_agent_config` mount (AC: #1, #3)
  - [x] 1.1 **CRITICAL**: Remove or adjust the early return on line 13-15 (`if len(cfg.Mounts) == 0 { return nil, nil }`). Currently this returns early when there are no regular mounts, which would silently skip `host_agent_config` processing. The guard must account for `cfg.HostAgentConfig != nil`.
  - [x] 1.2 When `cfg.HostAgentConfig != nil`, validate source path exists (os.Stat), return ConfigError if not found
  - [x] 1.3 Append `source:target` string to returned mount flags. Pre-allocate capacity for `len(cfg.Mounts)+1` when host_agent_config is set.
  - [x] 1.4 When `cfg.HostAgentConfig == nil`, no change to existing behavior
- [x] Task 2: Set `CLAUDE_CONFIG_DIR` env var in `cmd/run.go` (AC: #1, #3)
  - [x] 2.1 After `mount.AssembleMounts()` returns, check `cfg.HostAgentConfig != nil`
  - [x] 2.2 If set, add `envVars["CLAUDE_CONFIG_DIR"] = cfg.HostAgentConfig.Target`
  - [x] 2.3 If not set, do nothing (zero overhead)
- [x] Task 3: Unit tests for mount assembly (AC: #1, #3)
  - [x] 3.1 Test: `HostAgentConfig` set with valid source -> mount flag included in output
  - [x] 3.2 Test: `HostAgentConfig` nil -> no additional mount flags, no error
  - [x] 3.3 Test: `HostAgentConfig` source does not exist -> ConfigError returned
  - [x] 3.4 Test: `HostAgentConfig` combined with regular mounts -> both present in output
  - [x] 3.5 Test: `HostAgentConfig` set but NO regular mounts (empty `cfg.Mounts`) -> host_agent_config mount still returned (regression guard for early return fix)
- [x] Task 4: Unit test for env var injection in `cmd/run.go` (AC: #1, #3)
  - [x] 4.1 Verify `CLAUDE_CONFIG_DIR` is set when `HostAgentConfig` is configured
  - [x] 4.2 Verify `CLAUDE_CONFIG_DIR` is absent when `HostAgentConfig` is nil

## Dev Notes

### What Already Exists (DO NOT recreate)

- **Config struct field**: `HostAgentConfig *MountConfig` already exists in `internal/config/config.go:34`
- **YAML parsing**: Already handled by `yaml:"host_agent_config"` tag on the struct
- **Validation**: `parse.go:108-128` already validates source/target non-empty and target is absolute path
- **Path resolution**: `parse.go:154-156` already resolves `host_agent_config.Source` relative to config dir
- **Tilde expansion**: `resolvePath()` in `parse.go:168-178` already handles `~/` via `os.UserHomeDir()`
- **MountConfig type**: `config.go:65-69` already defines `Source`/`Target` fields

The config layer is COMPLETE. This story only touches the mount assembly and run command layers.

### What Must Be Implemented

1. **`internal/mount/mount.go` — extend `AssembleMounts()`**
   - Architecture specifies: `mount.go` handles regular mounts AND `host_agent_config` ([Source: architecture.md — Package Responsibilities])
   - **CRITICAL**: Current `AssembleMounts()` has an early return at line 13-15 (`if len(cfg.Mounts) == 0 { return nil, nil }`) that exits before any host_agent_config processing. This guard must be adjusted to also check `cfg.HostAgentConfig != nil` before returning early.
   - Add host_agent_config source path validation (os.Stat) and mount flag assembly after existing regular mount loop
   - Same error pattern as regular mounts: `ConfigError` with descriptive message
   - Return value: append the host_agent_config mount string to the existing slice

2. **`cmd/run.go` — env var injection**
   - After `mount.AssembleMounts()` call (line 25), add conditional: if `cfg.HostAgentConfig != nil`, set `envVars["CLAUDE_CONFIG_DIR"] = cfg.HostAgentConfig.Target`
   - Place this BEFORE the `agentCommand()` call to maintain the logical flow: mounts -> env vars -> agent command -> build -> run
   - Note: env var priority from `buildEnvVars()` (cfg.Env < secrets < HOST_UID/GID). `CLAUDE_CONFIG_DIR` should be set AFTER `buildEnvVars()` returns, at same level as `AGENT_CMD` and `AUTO_ISOLATE_VOLUME_PATHS`

3. **Tests in `internal/mount/mount_test.go`**
   - Follow existing table-driven pattern (see current tests in mount_test.go)
   - Use `t.TempDir()` for valid source paths (same pattern as existing tests)
   - Test the combination of regular mounts + host_agent_config together

### Anti-Patterns (DO NOT do these)

- Do NOT create a separate `AssembleHostAgentConfig()` function — architecture says `AssembleMounts()` handles both regular mounts and host_agent_config
- Do NOT modify `internal/config/config.go` or `internal/config/parse.go` — config layer is complete
- Do NOT modify `embed/Dockerfile.tmpl` or `embed/entrypoint.sh` — this is a host-side (cmd/run.go) feature only
- Do NOT add `host_agent_config` to the content hash — it's a runtime mount, not a build-time input
- Do NOT add any validation beyond source path existence — parse.go already validates required fields and path format
- Do NOT hardcode `~/.claude` or `/opt/claude-config` — values come from config
- Do NOT add the env var inside `buildEnvVars()` — it depends on `cfg.HostAgentConfig` which is separate from the env/secrets flow

### Security Context

- This is the **widest trust grant** in the system — read-write mount of agent config directory ([Source: architecture.md — Host Agent Config])
- It is **opt-in and explicit** — only active when `host_agent_config` is configured
- Known limitation (Phase 2): no integrity checking on config directory changes
- Threat model is accidental, not adversarial — agent could modify config that persists after sandbox exits
- AC #2 (token writeback) is satisfied by the read-write mount itself — no code needed, just verify Docker mount is not read-only

### Project Structure Notes

- All changes are in the host-side Go binary (cmd/ and internal/mount/)
- No container-side changes (embed/ directory untouched)
- Follows the existing mount assembly pattern established in story 2-1
- Deferred work item in `deferred-work.md` line 43 explicitly tracks this: "`HostAgentConfig` mount not included in `AssembleMounts` — future Story 7-1 scope"
- After implementation, that deferred item is resolved

### References

- [Source: architecture.md — Host Agent Config decision: read-write mount + CLAUDE_CONFIG_DIR env var]
- [Source: architecture.md — Package Responsibilities: mount.go handles regular mounts AND host_agent_config]
- [Source: architecture.md — Host vs Container Boundary: cmd/, internal/ run on host]
- [Source: epics.md — Epic 7, Story 7.1: acceptance criteria and implementation notes]
- [Source: prd.md — FR9d, FR45: host_agent_config mount and CLAUDE_CONFIG_DIR]
- [Source: prd.md — NFR6: Mounts limited to declared paths]
- [Source: deferred-work.md — Story 2-1 deferred: HostAgentConfig not in AssembleMounts]
- [Source: internal/mount/mount.go — current AssembleMounts() implementation]
- [Source: internal/config/parse.go:108-128 — existing host_agent_config validation]
- [Source: internal/config/parse.go:153-156 — existing host_agent_config path resolution]
- [Source: cmd/run.go — current run command flow]

### Previous Story Intelligence (Story 6.1)

- **Pattern**: New functionality added directly to existing package files, not new files
- **Testing**: Table-driven tests with `t.TempDir()` for filesystem operations
- **Integration point**: `cmd/run.go` is the main assembly point — append flags/env vars in logical sequence
- **Config pattern**: Boolean flags use early return; pointer fields use nil check
- **Deferred work**: Track any review findings in `deferred-work.md` with category and file reference

### Git Intelligence

- Commit format: `feat: implement story 7-1 host agent config mount for OAuth token sync`
- Recent commits show pattern: implementation + tests + sprint-status update + deferred-work update
- Go 1.25.0, no external dependencies needed for this story
- All tests must pass: `go test ./...`

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None — clean implementation with no debugging issues.

### Completion Notes List

- Extended `AssembleMounts()` early return guard to check `cfg.HostAgentConfig != nil` in addition to `len(cfg.Mounts) == 0`, preventing silent skip of host_agent_config processing
- Added host_agent_config source path validation via `os.Stat` with `ConfigError` on failure, following the same error pattern as regular mounts
- Pre-allocated mount slice capacity accounting for host_agent_config when present
- Added `CLAUDE_CONFIG_DIR` env var injection in `cmd/run.go` RunE, set after `buildEnvVars()` at the same level as `AGENT_CMD`
- 5 new unit tests in `mount_test.go` covering: valid source, nil config, nonexistent source, combined with regular mounts, and regression guard for early return fix
- 2 new unit tests in `run_test.go` verifying CLAUDE_CONFIG_DIR presence/absence based on HostAgentConfig
- All 7 new tests pass; full regression suite passes (0 failures across all packages)
- AC #2 (token writeback) is satisfied by the read-write mount default — no read-only flag is applied

### Change Log

- 2026-04-09: Implemented story 7-1 — host agent config mount and CLAUDE_CONFIG_DIR env var injection

### File List

- `internal/mount/mount.go` — modified: extended AssembleMounts() with host_agent_config support
- `internal/mount/mount_test.go` — modified: added 5 host_agent_config unit tests
- `cmd/run.go` — modified: added CLAUDE_CONFIG_DIR env var injection
- `cmd/run_test.go` — modified: added 2 env var injection tests
- `_bmad-output/implementation-artifacts/sprint-status.yaml` — modified: story status updated
- `_bmad-output/implementation-artifacts/7-1-host-agent-config-mount-for-oauth-token-sync.md` — modified: task checkboxes, dev record, status

### Review Findings

- [x] [Review][Patch] CLAUDE_CONFIG_DIR gated on `cfg.Agent == "claude-code"` — matches bash reference behavior. [cmd/run.go:36-38]
- [x] [Review][Patch] HostAgentConfig source validated as directory via `IsDir()` check — matches bash `-d` behavior. [internal/mount/mount.go:38-43]
- [x] [Review][Defer] Tests replicate RunE logic inline instead of exercising actual command handler — pragmatic trade-off since RunE calls ensureBuild()/RunContainer() which require mock infrastructure that doesn't exist. [cmd/run_test.go:52-120]
- [x] [Review][Defer] Error message hardcodes `.asbox/config.yaml` path — pre-existing pattern across all mount error messages, config path may be overridden. [internal/mount/mount.go:42]
