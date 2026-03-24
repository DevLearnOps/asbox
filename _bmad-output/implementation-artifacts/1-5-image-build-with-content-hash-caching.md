# Story 1.5: Image Build with Content-Hash Caching

Status: done

## Story

As a developer,
I want `sandbox build` to build my container image and skip rebuilds when nothing has changed,
so that I get fast iteration without unnecessary rebuilds.

## Acceptance Criteria

1. **Given** a valid config.yaml and Dockerfile.template
   **When** the developer runs `sandbox build`
   **Then** a Docker image is built and tagged as `sandbox-<project>:<content-hash>` where content-hash is derived from config.yaml, Dockerfile.template, scripts/entrypoint.sh, and scripts/git-wrapper.sh

2. **Given** an image already exists with the current content hash
   **When** the developer runs `sandbox build`
   **Then** the build is skipped and a message indicates the image is up to date

3. **Given** the developer modifies config.yaml
   **When** they run `sandbox build`
   **Then** a new content hash is computed and the image is rebuilt

4. **Given** the base image is pinned to digest in Dockerfile.template
   **When** the image is built
   **Then** the same config always produces the same image regardless of when/where it's built (NFR12)

## Tasks / Subtasks

- [x] Task 1: Implement `compute_content_hash()` function (AC: 1, 3)
  - [x]Add function in the "Build functions" section, before `process_template()`
  - [x]Hash inputs: config.yaml (at `CONFIG_PATH`), `Dockerfile.template`, `scripts/entrypoint.sh`, `scripts/git-wrapper.sh` -- all relative to `SCRIPT_DIR` except config
  - [x]Use `sha256sum` (Linux) with `shasum -a 256` (macOS) fallback for portability
  - [x]Concatenate file contents and hash the combined stream (e.g., `cat file1 file2 ... | sha256sum`)
  - [x]Return first 12 characters of the hex digest as the hash tag
  - [x]Die with error if any hash input file is missing

- [x] Task 2: Implement `compute_image_tag()` function (AC: 1)
  - [x]Derive project name from the directory containing the config file (basename of parent dir, or "sandbox" as fallback)
  - [x]Format: `sandbox-<project>:<content-hash>` (e.g., `sandbox-myapp:a1b2c3d4e5f6`)
  - [x]Store result in a global `IMAGE_TAG` variable

- [x] Task 3: Implement image existence check (AC: 2)
  - [x]Use `docker image inspect "${IMAGE_TAG}" >/dev/null 2>&1` to check if image exists
  - [x]If image exists, print "image up to date: ${IMAGE_TAG}" and return 0
  - [x]If image does not exist, proceed with build

- [x] Task 4: Implement `docker build` execution (AC: 1, 4)
  - [x]Run `docker build -t "${IMAGE_TAG}" -f "${dockerfile_path}" "${SCRIPT_DIR}"` where `dockerfile_path` is the resolved `.sandbox-dockerfile`
  - [x]Build context is `SCRIPT_DIR` (project root with scripts/, Dockerfile.template, etc.)
  - [x]Print info message before build: "building image: ${IMAGE_TAG}"
  - [x]Print success message after build: "image built: ${IMAGE_TAG}"
  - [x]If build fails, `set -e` will propagate the error -- no special handling needed

- [x] Task 5: Update `cmd_build()` to wire everything together (AC: 1, 2, 3)
  - [x]After `process_template()`, call `compute_content_hash` and `compute_image_tag`
  - [x]Write resolved Dockerfile to `.sandbox-dockerfile` (already done)
  - [x]Check if image exists -- if yes, skip build with info message
  - [x]If image does not exist, run `docker build`
  - [x]Remove the `info "not yet implemented"` line

- [x] Task 6: Add tests (AC: 1, 2, 3, 4)
  - [x]Test: `compute_content_hash` produces consistent hash for same files
  - [x]Test: `compute_content_hash` produces different hash when config changes
  - [x]Test: `compute_content_hash` dies if a hash input file is missing
  - [x]Test: `compute_image_tag` produces correct format `sandbox-<project>:<hash>`
  - [x]Test: `cmd_build` skips build when image already exists (mock `docker image inspect` to succeed)
  - [x]Test: `cmd_build` calls docker build when image does not exist (mock `docker` to capture args)
  - [x]Test: content hash includes all four specified files and no others

## Dev Notes

### Architecture Compliance

- **Content-hash cache key composition**: Hash inputs are exactly: the project's `config.yaml`, `Dockerfile.template`, `scripts/entrypoint.sh`, `scripts/git-wrapper.sh`. Changes to `sandbox.sh`, `README.md`, `templates/config.yaml`, or `LICENSE` do NOT trigger rebuilds. [Source: architecture.md#Gap 7: Content-hash cache key composition]
- **Image tagging**: `sandbox-<project>:<content-hash>` format per FR43. [Source: epics.md#Story 1.5]
- **Reproducible builds**: Base image pinned to digest (already done in story 1-4 via `BASE_IMAGE` constant). Same config always produces same image (NFR12).
- **Exit codes**: Use exit code 1 for general errors (missing files). Exit code 3 if docker is unavailable (already handled by `check_dependencies()`). [Source: architecture.md#Exit Codes]
- **No color codes, no spinners** -- plain text only.
- **`set -euo pipefail`** already enforced.

### Implementation Specifics

**Content hash algorithm:**
```bash
compute_content_hash() {
  local hash_cmd="sha256sum"
  if ! command -v sha256sum >/dev/null 2>&1; then
    hash_cmd="shasum -a 256"
  fi
  # Cat all four files and hash the combined stream
  local hash
  hash="$(cat "${CONFIG_PATH}" \
              "${SCRIPT_DIR}/Dockerfile.template" \
              "${SCRIPT_DIR}/scripts/entrypoint.sh" \
              "${SCRIPT_DIR}/scripts/git-wrapper.sh" \
         | ${hash_cmd} | cut -c1-12)"
  CONTENT_HASH="${hash}"
}
```

**Project name derivation:**
The project name is derived only from the standard config layout (`.sandbox/config.yaml`). When config is at `<project>/.sandbox/config.yaml`, the project name is the basename of `<project>/`. For non-standard config paths (e.g., `-f /tmp/config.yaml` or `-f ./config.yaml`), the project name falls back to `"sandbox"`. The project name is sanitized for Docker tag validity: lowercased and stripped of characters outside `[a-z0-9_.-]`.

**Docker build command:**
```bash
docker build -t "${IMAGE_TAG}" -f "${SCRIPT_DIR}/.sandbox-dockerfile" "${SCRIPT_DIR}"
```
The build context must be `SCRIPT_DIR` because the resolved Dockerfile references `COPY scripts/entrypoint.sh` and `COPY scripts/git-wrapper.sh` relative to the build context.

**Image existence check:**
```bash
if docker image inspect "${IMAGE_TAG}" >/dev/null 2>&1; then
  info "image up to date: ${IMAGE_TAG}"
  return 0
fi
```

### Testing Strategy

Tests cannot run actual Docker builds. Instead:
- **Hash tests**: Create temp files, call `compute_content_hash` with those files, verify deterministic output and that different content produces different hashes.
- **Image tag tests**: Set `CONTENT_HASH` and `CONFIG_PATH`, call `compute_image_tag`, verify format.
- **Build skip/proceed tests**: Mock `docker` with a shell function or script that records arguments. Set PATH to include mock before real docker. Verify:
  - When mock `docker image inspect` returns 0 -> build skipped
  - When mock `docker image inspect` returns 1 -> `docker build` called with correct args
- **Reuse existing test patterns**: temp dirs, `set +e`/`set -e` for exit code capture, `pass()`/`fail()` assertions.

Mock docker approach (from test file patterns):
```bash
# Create a mock docker that logs calls
mock_docker="$(mktemp)"
cat > "${mock_docker}" << 'MOCK'
#!/usr/bin/env bash
echo "docker $*" >> "${MOCK_DOCKER_LOG}"
if [[ "$1" == "image" && "$2" == "inspect" ]]; then
  exit "${MOCK_INSPECT_EXIT:-1}"
fi
exit 0
MOCK
chmod +x "${mock_docker}"
```

### Previous Story (1-4) Intelligence

**Established patterns:**
- `RESOLVED_DOCKERFILE` global stores the processed template output
- `cmd_build()` writes it to `${SCRIPT_DIR}/.sandbox-dockerfile`
- `BASE_IMAGE` is a readonly constant with the pinned Ubuntu 24.04 digest
- `sed_escape_replacement()` utility handles special chars in sed replacements
- `process_template()` reads from `SCRIPT_DIR/Dockerfile.template` and populates `RESOLVED_DOCKERFILE`
- 113 tests currently pass using TAP-like format with `pass()`/`fail()`/`assert_exit_code()`/`assert_contains()`
- Tests mock system binaries (docker, yq) via PATH manipulation and symlinks in temp dirs
- Tests use `set +e` / `set -e` around expected-failure commands

**Files modified in 1-4:**
- `Dockerfile.template` -- Full template with conditional blocks and placeholders
- `sandbox.sh` -- Added `process_template()`, `sed_escape_replacement()`, `BASE_IMAGE` constant, `RESOLVED_DOCKERFILE` global, updated `cmd_build()`
- `tests/test_sandbox.sh` -- Added 36 template processing tests (113 total)

**Key learning from 1-4:**
- sed replacement strings need escaping (`&`, `\`, `|`) -- use `sed_escape_replacement()`
- Template processing must handle conditionals BEFORE value substitution
- yq returns literal `"null"` for missing keys -- always check for both `""` and `"null"`

### Git History Context

Recent commits show sequential story implementation, all following `feat:` prefix convention:
- `073e1f9 feat: implement Dockerfile generation from template with review fixes (story 1-4)`
- `f0eb322 feat: implement config parsing with review fixes (story 1-3)`

### Current sandbox.sh Structure

The build functions section (lines 175-288) contains:
- `sed_escape_replacement()` -- utility for sed escaping
- `BASE_IMAGE` -- readonly constant with pinned digest
- `process_template()` -- template processing function
- `cmd_build()` -- calls `parse_config`, `process_template`, writes Dockerfile, prints "not yet implemented"

New functions (`compute_content_hash`, `compute_image_tag`) should go between `BASE_IMAGE` and `process_template()`. The `cmd_build()` function needs updating to replace the "not yet implemented" stub with actual build logic.

### New globals to add

```bash
CONTENT_HASH=""
IMAGE_TAG=""
```

Add these to the "Config globals" section alongside `RESOLVED_DOCKERFILE=""`.

### Project Structure Notes

- No new files needed -- all changes are in `sandbox.sh` and `tests/test_sandbox.sh`
- `.sandbox-dockerfile` is a build artifact (already in .gitignore or should be)
- Build context is `SCRIPT_DIR` (project root)

### Anti-Patterns to Avoid

- Do NOT hash `sandbox.sh` itself -- it's not part of the cache key
- Do NOT use `md5sum` -- use `sha256sum`/`shasum -a 256` for consistency
- Do NOT run docker build without checking image existence first
- Do NOT hardcode project name -- derive from config path
- Do NOT add `--no-cache` flag -- rely on content hash for cache invalidation
- Do NOT use `eval` for any string operations

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Gap 7: Content-hash cache key composition]
- [Source: _bmad-output/planning-artifacts/architecture.md#File Responsibilities -- sandbox.sh]
- [Source: _bmad-output/planning-artifacts/architecture.md#Exit Codes]
- [Source: _bmad-output/planning-artifacts/architecture.md#Bash Coding Conventions]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 1.5]
- [Source: _bmad-output/planning-artifacts/epics.md#FR43 -- Content-hash image tagging]
- [Source: _bmad-output/implementation-artifacts/1-4-dockerfile-generation-from-template.md]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None required.

### Completion Notes List

- Implemented `compute_content_hash()` using sha256sum with shasum -a 256 fallback for macOS portability. Hashes exactly 4 files: config.yaml, Dockerfile.template, scripts/entrypoint.sh, scripts/git-wrapper.sh. Returns first 12 hex chars.
- Implemented `compute_image_tag()` deriving project name from config path parent directory. Format: `sandbox-<project>:<content-hash>`.
- Added image existence check via `docker image inspect` — skips rebuild when hash matches.
- Added `docker build` execution with correct tag, dockerfile path, and build context.
- Updated `cmd_build()` to wire all components: template processing -> hash computation -> existence check -> conditional build.
- Added 22 new tests (133 total, up from 113): hash determinism, hash sensitivity to each input file, missing file error, image tag format, build skip when image exists, build proceed when image doesn't exist, docker mock argument verification.
- Updated 3 existing tests that checked for "not yet implemented" to verify new build behavior.

### Change Log

- 2026-03-24: Implemented image build with content-hash caching (story 1-5). All 6 tasks completed. 133/133 tests pass.

### File List

- sandbox.sh (modified: added CONTENT_HASH/IMAGE_TAG globals, compute_content_hash(), compute_image_tag(), updated cmd_build())
- tests/test_sandbox.sh (modified: added 22 new tests, updated mock infrastructure, updated 3 existing tests)
