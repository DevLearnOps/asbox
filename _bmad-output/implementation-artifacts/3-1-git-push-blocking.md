# Story 3.1: Git Push Blocking

Status: review

## Story

As a developer,
I want git push to be blocked inside the sandbox with a standard error,
so that the agent cannot accidentally push code to remote repositories.

## Acceptance Criteria

1. **Given** a sandbox is running with a mounted git repository that has a remote configured
   **When** the agent runs `git push` (or `git push origin main`, etc.)
   **Then** the command fails with exit code 1 and prints "fatal: Authentication failed" to stderr

2. **Given** a sandbox is running
   **When** the agent runs any other git command (add, commit, log, diff, branch, checkout, merge, amend, pull, fetch)
   **Then** the command passes through to real git and behaves identically to standard git (NFR10)

3. **Given** the git wrapper is at `/usr/local/bin/git`
   **When** the agent inspects the environment
   **Then** the wrapper is transparent -- the agent does not know git is intercepted

## Tasks / Subtasks

- [x] Task 1: Add push-blocking logic to git-wrapper.sh (AC: #1, #2, #3)
  - [x] 1.1 Check if the first argument is `push` -- if so, print "fatal: Authentication failed" to stderr and exit 1
  - [x] 1.2 All non-push commands must continue to use `exec /usr/bin/git "$@"` passthrough unchanged
  - [x] 1.3 Verify the wrapper remains minimal and transparent (no logging, no sandbox-specific errors)
- [x] Task 2: Write tests for git push blocking (AC: #1)
  - [x] 2.1 Test: `git push` is blocked with exit code 1 and stderr contains "fatal: Authentication failed"
  - [x] 2.2 Test: `git push origin main` is blocked (push with arguments)
  - [x] 2.3 Test: `git push --force` is blocked
  - [x] 2.4 Test: `git push --all` is blocked
  - [x] 2.5 Test: `git push -u origin feature-branch` is blocked
- [x] Task 3: Verify non-push operations still pass through (AC: #2)
  - [x] 3.1 Run existing git wrapper tests to confirm no regressions
  - [x] 3.2 Test: `git pull` still passes through to real git
  - [x] 3.3 Test: `git fetch` still passes through to real git
  - [x] 3.4 Test: `git commit --amend` still passes through (not confused with push)
  - [x] 3.5 Test: `git stash push` passes through (subcommand `push` under `stash`, not a top-level `push`)

## Dev Notes

### Exact Change to git-wrapper.sh

The current wrapper is a transparent passthrough:

```bash
#!/usr/bin/env bash
set -euo pipefail
# Transparent passthrough to real git - push blocking added in Story 3-1
exec /usr/bin/git "$@"
```

Replace with push-blocking logic:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Block git push - all other commands pass through to real git
if [[ "${1:-}" == "push" ]]; then
  echo "fatal: Authentication failed for 'https://github.com'" >&2
  exit 1
fi

exec /usr/bin/git "$@"
```

Key implementation details:
- `"${1:-}"` safely handles the case where no arguments are provided (empty default avoids `set -u` failure)
- Only checks the **first argument** for `push` -- this means `git stash push` passes through correctly because `stash` is the first arg
- Error message mimics a real git authentication failure so the agent treats it as a standard git error (FR37, NFR2)
- Exit code 1 matches real `git push` authentication failures
- `exec` for all non-push commands preserves the transparent passthrough behavior (NFR10)
- No sandbox-specific error messages -- the agent should not know it's in a sandbox

### Edge Case: `git stash push`

`git stash push` must NOT be blocked. The wrapper checks only `$1` (the first argument). Since `git stash push` passes `stash` as the first argument, it correctly falls through to real git. No special handling needed.

### Edge Case: `git remote` Commands

Commands like `git remote add`, `git remote set-url` are not blocked. The threat model protects against accidental pushes, not adversarial exfiltration. The agent can configure remotes but cannot push to them.

### What NOT to Change

- **sandbox.sh** -- no changes needed. Content-hash already includes git-wrapper.sh, so the image will automatically rebuild.
- **Dockerfile.template** -- no changes needed. The wrapper is already copied to `/usr/local/bin/git` with correct permissions.
- **scripts/entrypoint.sh** -- no changes needed. PATH already has `/usr/local/bin` before `/usr/bin`.
- **templates/config.yaml** -- no changes needed.

### Files to Modify

Only two files need changes:
1. `scripts/git-wrapper.sh` -- add push-blocking logic (3 lines of new code)
2. `tests/test_sandbox.sh` -- add push-blocking tests

### Test Pattern

Follow the established test pattern from Story 2-4. The existing tests create a mock git binary via sed substitution on the wrapper. For push-blocking tests, no mock is needed for the blocking path since the wrapper never calls real git when push is detected. For passthrough verification, reuse the existing mock pattern.

Example test structure:

```bash
# Test: git push is blocked
output_all="$(bash "${wrapper_copy}" push 2>&1)"
exit_code=$?
assert_exit_code 1 "${exit_code}" "git wrapper blocks push with exit code 1"
assert_contains "${output_all}" "fatal: Authentication failed" "git wrapper returns authentication error on push"

# Test: git push with arguments is blocked
output_all="$(bash "${wrapper_copy}" push origin main 2>&1)"
exit_code=$?
assert_exit_code 1 "${exit_code}" "git wrapper blocks 'push origin main'"

# Test: git stash push passes through (stash is first arg, not push)
output_all="$(bash "${wrapper_ops}" stash push 2>&1)"
exit_code=$?
assert_exit_code 0 "${exit_code}" "git stash push passes through (not confused with push)"
```

Note: For the push-blocking path, the wrapper copy does NOT need the `/usr/bin/git` path replaced because it never reaches the `exec` line. For passthrough tests, continue using the sed mock substitution pattern.

### Previous Story Intelligence

From Story 2-4 implementation:
- 285 tests currently passing (28 added in 2-4)
- Git wrapper tests are in the test file around lines 2112-2203
- Test helpers available: `assert_exit_code`, `assert_contains`, `assert_not_contains`
- Mock pattern: `sed "s|/usr/bin/git|${mock_git}|g"` to replace real git path in wrapper copy
- The wrapper copy variable is `wrapper_copy` (for basic tests) and `wrapper_ops` (for operation loop tests)

### Git Recent Commits

```
b936ee9 feat: implement git wrapper passthrough and CLI toolchain with review fixes (story 2-4)
322198e feat: implement env var injection with validation and review fixes (story 2-3)
80b8cd1 feat: implement secret injection and validation with review fixes (story 2-2)
```

All Epic 2 stories are done. This is the first story in Epic 3.

### Architecture Compliance

- **FR31:** System blocks git push operations via a git wrapper, returning standard "unauthorized" errors
- **FR37:** System returns standard CLI error codes when agent attempts operations outside the boundary
- **NFR2:** Git push operations fail with standard error codes regardless of remote configuration
- **NFR10:** Git wrapper is transparent for all operations except push
- **Threat model:** Accidental, not adversarial -- the wrapper prevents `git push` but does not prevent exfiltration via `curl` or other network tools (this is by design)

### Project Structure Notes

- `scripts/git-wrapper.sh` is already in the correct location per architecture
- Already COPY'd to `/usr/local/bin/git` in Dockerfile.template (line 41)
- Already included in content-hash computation in `sandbox.sh` (line 230)
- No new files needed -- only modifications to existing files

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 3, Story 3.1]
- [Source: _bmad-output/planning-artifacts/architecture.md - Git Wrapper decision, Isolation Mechanisms section]
- [Source: _bmad-output/planning-artifacts/prd.md - FR31, FR37, NFR2, NFR10]
- [Source: _bmad-output/planning-artifacts/architecture.md - Threat Model Boundaries section]
- [Source: scripts/git-wrapper.sh - Current transparent passthrough]
- [Source: _bmad-output/implementation-artifacts/2-4-git-and-cli-toolchain.md - Previous story patterns and test baseline]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None - clean implementation with no debugging needed.

### Completion Notes List

- Added push-blocking logic to git-wrapper.sh: checks `$1` for "push", prints "fatal: Authentication failed for 'https://github.com'" to stderr, exits 1. All other commands pass through via `exec /usr/bin/git "$@"`.
- Added 5 push-blocking tests: bare push, push origin main, push --force, push --all, push -u origin feature-branch — all verify exit code 1 and auth error message.
- Added 4 passthrough verification tests: pull, fetch, commit --amend, stash push — all confirm correct forwarding to real git.
- Fixed existing passthrough loop test to remove `push` from passthrough operations list (push is now blocked).
- All existing tests continue to pass — no regressions. Test count: 285 → 304 (19 new assertions).
- `git stash push` correctly passes through because `stash` is `$1`, not `push`.

### Change Log

- 2026-03-24: Implemented git push blocking (Story 3-1) — 3 lines added to git-wrapper.sh, 19 test assertions added

### File List

- scripts/git-wrapper.sh (modified — added push-blocking logic)
- tests/test_sandbox.sh (modified — added push-blocking tests, passthrough tests, fixed existing loop)
