# Story 4.3: Isolation Scripts Baked into Image

Status: review

## Story

As a developer,
I want the git wrapper and entrypoint scripts baked into the sandbox image at build time,
so that isolation boundaries are always present regardless of what the agent does at runtime.

## Acceptance Criteria

1. **Given** a sandbox image is built
   **When** inspecting the image contents
   **Then** `scripts/git-wrapper.sh` is installed at `/usr/local/bin/git` and `scripts/entrypoint.sh` is the container's entrypoint

2. **Given** the agent attempts to modify or remove `/usr/local/bin/git`
   **When** the file is owned by root and the agent runs as a non-root user
   **Then** the modification fails with a permission denied error

3. **Given** the image is built
   **When** inspecting the non-root user setup
   **Then** a non-root user is configured for Podman rootless operation

## Tasks / Subtasks

- [x] Task 1: Add tests verifying isolation script deployment in generated Dockerfile (AC: #1)
  - [x] 1.1 Add test asserting generated Dockerfile contains `COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh`
  - [x] 1.2 Add test asserting generated Dockerfile contains `COPY scripts/git-wrapper.sh /usr/local/bin/git`
  - [x] 1.3 Add test asserting generated Dockerfile contains `chmod +x /usr/local/bin/entrypoint.sh /usr/local/bin/git`
  - [x] 1.4 Add test asserting generated Dockerfile contains `ENTRYPOINT ["tini", "--"]`
  - [x] 1.5 Add test asserting generated Dockerfile contains `CMD ["/usr/local/bin/entrypoint.sh"]`

- [x] Task 2: Add tests verifying script tamper resistance via file ownership model (AC: #2)
  - [x] 2.1 Add test asserting COPY of scripts happens BEFORE `useradd ... sandbox` in generated Dockerfile (root ownership implicit -- files COPY'd without `--chown` are owned by root)
  - [x] 2.2 Add test asserting generated Dockerfile does NOT contain a `USER` directive before ENTRYPOINT (no USER directive exists at all -- the container starts as root, and entrypoint.sh drops to sandbox user via `runuser`). This confirms COPY'd files retain root ownership.
  - [x] 2.3 Add test asserting generated Dockerfile does NOT contain `chown` targeting `/usr/local/bin/git` or `/usr/local/bin/entrypoint.sh` (note: a `chown sandbox:sandbox` DOES exist for `/run/user/` at line 82 -- use path-specific grep like `chown.*entrypoint` or `chown.*/usr/local/bin/git` to avoid false matches)

- [x] Task 3: Add tests verifying non-root user setup for Podman rootless (AC: #3)
  - [x] 3.1 Existing Story 4-1 tests cover `subuid` and `subgid` (lines 2753-2755) -- do NOT duplicate those
  - [x] 3.2 Add test asserting generated Dockerfile contains `useradd -m -s /bin/bash sandbox` (not currently tested)
  - [x] 3.3 Add test asserting generated Dockerfile contains sandbox user home directory and shell setup

- [x] Task 4: Verify content-hash tests already cover isolation scripts (AC: #1)
  - [x] 4.1 Confirm existing tests at lines 1048-1062 already verify that modifying `scripts/entrypoint.sh` and `scripts/git-wrapper.sh` changes the content hash -- these tests exist and should NOT be duplicated
  - [x] 4.2 Confirm existing tests at lines 1016-1035 verify that missing script files cause `compute_content_hash()` to fail -- do NOT duplicate
  - [x] 4.3 If any gap is found (e.g., one script is not tested), add only the missing assertion

## Dev Notes

### What This Story Is Really About

The isolation script deployment mechanism already exists -- `Dockerfile.template` lines 66-69 COPY the scripts and set permissions, and the non-root user is created at line 72. **This story's primary value is adding comprehensive tests to lock down these behaviors** so future changes cannot accidentally weaken the isolation model.

The key insight: scripts are COPY'd without `--chown` (so they're owned by root), and the container has NO `USER` directive -- it starts as root. The entrypoint.sh drops privileges to the sandbox user via `runuser -u sandbox`. Since the sandbox user cannot write to root-owned files in `/usr/local/bin/`, the git wrapper is tamper-resistant. Tests must verify: (1) scripts are deployed before user creation, (2) no `USER` directive exists, (3) no `chown` targets the script paths.

### What Changes

**tests/test_sandbox.sh** -- MODIFY: Add new test assertions. This is the ONLY file that needs modification.

**No changes to:** `sandbox.sh`, `scripts/entrypoint.sh`, `scripts/git-wrapper.sh`, `Dockerfile.template`. All isolation script baking is already implemented. This story adds the test safety net.

### Current Dockerfile.template Structure (88 lines)

The relevant section (lines 64-87):
```
Line 66: # Copy isolation scripts
Line 67: COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh
Line 68: COPY scripts/git-wrapper.sh /usr/local/bin/git
Line 69: RUN chmod +x /usr/local/bin/entrypoint.sh /usr/local/bin/git
Line 71: # Non-root user for rootless operation
Line 72: RUN useradd -m -s /bin/bash sandbox
Lines 74-77: Rootless Podman subuid/subgid config
Lines 79-83: XDG_RUNTIME_DIR pre-creation (chown sandbox:sandbox /run/user/$UID -- NOT on scripts)
Line 85: # Entrypoint starts as root to fix mount propagation, then drops to sandbox user
Line 86: ENTRYPOINT ["tini", "--"]
Line 87: CMD ["/usr/local/bin/entrypoint.sh"]
```

**Critical details:**
- Scripts deployed (lines 67-69) BEFORE sandbox user created (line 72). Files COPY'd without `--chown` are owned by `root:root`.
- There is NO `USER` directive in the Dockerfile. The container starts as root.
- `entrypoint.sh` drops privileges via `runuser -u sandbox` (line 15 of entrypoint.sh).
- The sandbox user has `rx` on `/usr/local/bin/git` but NOT `w`. This prevents the agent from replacing the git wrapper.
- The `chown sandbox:sandbox` at line 82 targets `/run/user/$UID` only -- NOT the isolation scripts.

### Current scripts/entrypoint.sh (45 lines)

The entrypoint handles:
1. Root-phase initialization (mount propagation fixes, /proc workaround for nested containers)
2. Re-executes itself as the `sandbox` user via `runuser`
3. Podman rootless initialization (XDG_RUNTIME_DIR, podman system migrate)
4. Agent dispatch (claude-code or gemini-cli)

### Current scripts/git-wrapper.sh (11 lines)

Simple push interceptor:
- `git push` -> exit 1 with "fatal: Authentication failed" message
- All other git commands -> passthrough to `/usr/bin/git`

### Content Hash Verification

`compute_content_hash()` at sandbox.sh:213-241 hashes exactly these 4 files:
1. `${CONFIG_PATH}` (config.yaml)
2. `${SCRIPT_DIR}/Dockerfile.template`
3. `${SCRIPT_DIR}/scripts/entrypoint.sh`
4. `${SCRIPT_DIR}/scripts/git-wrapper.sh`

Both isolation scripts are already in the hash. A test should verify that changing either script produces a different hash, locking this behavior.

### Test Pattern

Follow established patterns in `tests/test_sandbox.sh` (2,792 lines, 374 assertions currently passing).

**Dockerfile content capture mechanism:** Tests build the Dockerfile via the build process and then read the generated output from `.sandbox-dockerfile`:
```bash
dockerfile_content="$(cat "${PROJECT_ROOT}/.sandbox-dockerfile")"
```

**Assertion helpers available:**
- `assert_contains "${output}" "expected" "description"`
- `assert_not_contains "${output}" "unexpected" "description"`
- `pass "description"` / `fail "description"`

**For ordering assertions (Task 2):** Use line-number comparison. Extract line numbers of both the COPY directive and the useradd directive from the generated Dockerfile, then assert COPY line < useradd line. Example:
```bash
copy_line="$(echo "${dockerfile_content}" | grep -n "COPY scripts/git-wrapper.sh" | head -1 | cut -d: -f1)"
user_line="$(echo "${dockerfile_content}" | grep -n "useradd.*sandbox" | head -1 | cut -d: -f1)"
if [[ "${copy_line}" -lt "${user_line}" ]]; then
  pass "git-wrapper deployed before sandbox user created (tamper-resistant)"
else
  fail "git-wrapper deployed before sandbox user created (tamper-resistant)"
fi
```

**For content-hash tests (Task 4):** Existing tests at lines 1048-1062 already test hash sensitivity for both isolation scripts. Review those tests to confirm coverage -- do NOT add new hash tests unless a gap is found.

**For the no-USER-directive test (Task 2.2):**
```bash
if echo "${dockerfile_content}" | grep -q "^USER "; then
  fail "generated Dockerfile should not contain USER directive (privilege drop via runuser in entrypoint)"
else
  pass "generated Dockerfile has no USER directive (root ownership of COPY'd scripts preserved)"
fi
```

**For chown path-specific test (Task 2.3):**
```bash
# Note: chown sandbox:sandbox EXISTS for /run/user/ -- only test script paths
assert_not_contains "${dockerfile_content}" "chown.*entrypoint" "no chown on entrypoint script"
assert_not_contains "${dockerfile_content}" "chown.*/usr/local/bin/git" "no chown on git wrapper"
```

### Previous Story Intelligence

**From Story 4-2 (Building and Running Inner Containers):**
- 363 total test assertions at end of 4-2 implementation
- Current count: 374 (11 more added in review cycle)
- Podman rootless init added to entrypoint.sh (XDG_RUNTIME_DIR, system migrate, info check)
- Network isolation tests verify no `-p` flags, no `--network=host` in cmd_run() output
- Test fixtures exist: `tests/fixtures/Dockerfile.inner`, `tests/fixtures/docker-compose.yml`

**From Story 4-1 (Podman Installation and Docker Alias):**
- 15 test assertions for Podman verification already exist (lines 2738-2779)
- These test the GENERATED Dockerfile content for Podman repo setup, packages, rootless config
- Existing tests verify: subuid, subgid, uidmap, fuse-overlayfs, podman-docker, podman-compose
- DO NOT duplicate Podman-specific tests -- they exist. Focus on script deployment and tamper resistance.

**From Story 3-2 (Filesystem and Credential Isolation):**
- Tests at lines 2383-2410 verify no `--privileged` and no Docker socket mount
- These cover the outer container security posture

### What NOT to Do

- Do NOT modify Dockerfile.template -- script deployment is already correct
- Do NOT modify sandbox.sh -- content hash already includes both scripts
- Do NOT modify entrypoint.sh or git-wrapper.sh -- they work correctly
- Do NOT duplicate Story 4-1's Podman installation tests (subuid/subgid/uidmap already tested)
- Do NOT duplicate Story 3-2's --privileged/socket tests
- Do NOT add live container tests -- all assertions are against generated Dockerfile content and hash function behavior

### Architecture Compliance

- **FR42:** System bakes git wrapper and isolation boundary scripts into the image -- verified by testing COPY directives in generated Dockerfile
- **FR31:** Git push blocked via wrapper -- git-wrapper.sh at /usr/local/bin/git (tested in Story 3-1)
- **NFR10:** Git wrapper transparent except for push -- passthrough to /usr/bin/git (tested in Story 3-1)

### Project Structure Notes

- Only `tests/test_sandbox.sh` is modified
- No new files created
- All tests follow existing assertion patterns

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 4, Story 4.3]
- [Source: _bmad-output/planning-artifacts/architecture.md - Isolation Mechanisms: Git Wrapper, Entrypoint]
- [Source: _bmad-output/planning-artifacts/architecture.md - Gap 7: Content-hash cache key composition]
- [Source: _bmad-output/planning-artifacts/prd.md - FR42, FR31, NFR10]
- [Source: Dockerfile.template - Lines 66-69: Script COPY and chmod, Lines 71-83: User setup, Lines 86-87: ENTRYPOINT/CMD]
- [Source: scripts/entrypoint.sh - 45 lines: Root-phase init, Podman rootless setup, agent dispatch]
- [Source: scripts/git-wrapper.sh - 11 lines: Push blocking, passthrough to /usr/bin/git]
- [Source: sandbox.sh - compute_content_hash() lines 213-241: Hash includes both isolation scripts]
- [Source: tests/test_sandbox.sh - 2792 lines, 374 assertions, Dockerfile content capture at line 2738]
- [Source: _bmad-output/implementation-artifacts/4-1-podman-installation-and-docker-alias.md - Podman tests at lines 2738-2779]
- [Source: _bmad-output/implementation-artifacts/4-2-building-and-running-inner-containers.md - Network isolation tests, entrypoint modifications]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None required -- all tests passed on first run.

### Completion Notes List

- Task 1: Added 5 assertions verifying COPY entrypoint.sh, COPY git-wrapper.sh, chmod +x, ENTRYPOINT tini, CMD entrypoint.sh in generated Dockerfile
- Task 2: Added 4 assertions verifying tamper resistance: COPY before useradd (line-number comparison), no USER directive, no chown on entrypoint or git wrapper paths
- Task 3: Added 2 assertions verifying sandbox user created with -m -s /bin/bash (subuid/subgid already tested in 4-1, not duplicated)
- Task 4: Confirmed existing hash sensitivity tests at lines 1048-1062 cover both isolation scripts; missing-file test at 1016-1035 covers git-wrapper.sh. No gaps found, no tests added.
- Total: 11 new test assertions added (374 -> 385 total), all passing, 0 regressions

### Change Log

- 2026-03-25: Added 11 test assertions for isolation script deployment, tamper resistance, and non-root user setup in tests/test_sandbox.sh (Tasks 1-3). Verified existing content-hash tests provide complete coverage (Task 4).

### File List

- tests/test_sandbox.sh (MODIFIED) -- Added Story 4.3 test section with 11 assertions
