# Story 1.4: Dockerfile Generation from Template

Status: done

## Story

As a developer,
I want sandbox to generate a valid Dockerfile from Dockerfile.template using my config values,
so that my sandbox image is built with the SDKs and packages I specified.

## Acceptance Criteria

1. **Given** a config with Node.js 22 and Python 3.12 SDKs and packages [build-essential, curl]
   **When** sandbox processes Dockerfile.template
   **Then** a resolved Dockerfile is produced with Node.js 22 and Python 3.12 install blocks active, Go blocks stripped, and packages included

2. **Given** a config with no SDKs specified
   **When** sandbox processes Dockerfile.template
   **Then** all conditional SDK blocks are stripped and only base tooling remains

3. **Given** a template with mismatched `{{IF_NAME}}` / `{{/IF_NAME}}` tags
   **When** sandbox processes the template
   **Then** the script exits with code 1 and prints an error identifying the unmatched tag

4. **Given** a resolved Dockerfile would contain unresolved `{{PLACEHOLDER}}` values
   **When** sandbox processes the template
   **Then** the script exits with code 1 and prints an error identifying the unresolved placeholder

## Tasks / Subtasks

- [x] Task 1: Create Dockerfile.template with conditional blocks and placeholders (AC: 1, 2)
  - [x]Replace the placeholder Dockerfile.template with a real template
  - [x]Use Ubuntu 24.04 LTS base image pinned to digest as FROM line with `{{BASE_IMAGE}}` placeholder
  - [x]Add tini installation block
  - [x]Add `# {{IF_NODE}}` / `# {{/IF_NODE}}` conditional block for Node.js installation using `{{NODE_VERSION}}`
  - [x]Add `# {{IF_PYTHON}}` / `# {{/IF_PYTHON}}` conditional block for Python installation using `{{PYTHON_VERSION}}`
  - [x]Add `# {{IF_GO}}` / `# {{/IF_GO}}` conditional block for Go installation using `{{GO_VERSION}}`
  - [x]Add `{{PACKAGES}}` placeholder for system packages (space-separated apt-get install list)
  - [x]Add common CLI tools installation (curl, wget, dig, etc.) in base section (not conditional)
  - [x]Add COPY for entrypoint.sh and git-wrapper.sh into image
  - [x]Add non-root user setup section
  - [x]Set entrypoint to tini

- [x] Task 2: Implement `process_template()` function in sandbox.sh (AC: 1, 2, 3, 4)
  - [x]Add function in the "Build functions" section of sandbox.sh
  - [x]Accept no arguments -- reads from `SCRIPT_DIR/Dockerfile.template`, outputs to stdout or a variable
  - [x]Step 1: Read template content into a variable
  - [x]Step 2: Validate matching conditional tags -- every `{{IF_NAME}}` has a `{{/IF_NAME}}`; die with code 1 naming the unmatched tag if not
  - [x]Step 3: Process conditional blocks -- if `CFG_SDK_NODEJS` is non-empty, keep `{{IF_NODE}}` block contents and remove the tag lines; if empty, strip the entire block including contents
  - [x]Step 4: Process conditional blocks for PYTHON and GO similarly
  - [x]Step 5: Substitute value placeholders -- `{{NODE_VERSION}}` -> `CFG_SDK_NODEJS`, `{{PYTHON_VERSION}}` -> `CFG_SDK_PYTHON`, `{{GO_VERSION}}` -> `CFG_SDK_GO`, `{{PACKAGES}}` -> space-joined `CFG_PACKAGES`, `{{BASE_IMAGE}}` -> hardcoded Ubuntu 24.04 digest
  - [x]Step 6: Validate no unresolved `{{...}}` placeholders remain -- die with code 1 naming the first unresolved placeholder
  - [x]Use `sed` for substitutions, not `eval`

- [x] Task 3: Wire `process_template()` into `cmd_build()` (AC: 1, 2)
  - [x]Call `process_template` after `parse_config` in `cmd_build()`
  - [x]Write resolved Dockerfile to a temp location (e.g., `${SCRIPT_DIR}/.sandbox-dockerfile` or a mktemp file)
  - [x]Print info message about generated Dockerfile
  - [x]Keep the "not yet implemented" message for the actual docker build step (story 1-5)

- [x] Task 4: Add tests for template processing (AC: 1, 2, 3, 4)
  - [x]Test: template with Node.js + Python enabled produces Dockerfile with both install blocks and no Go block
  - [x]Test: template with no SDKs strips all conditional blocks
  - [x]Test: template with all three SDKs keeps all blocks
  - [x]Test: packages list is correctly substituted into apt-get install line
  - [x]Test: empty packages list produces valid Dockerfile (no empty apt-get)
  - [x]Test: mismatched `{{IF_NAME}}` without `{{/IF_NAME}}` exits code 1 with error naming the tag
  - [x]Test: unresolved `{{PLACEHOLDER}}` after substitution exits code 1 with error naming the placeholder
  - [x]Test: resolved Dockerfile contains no `{{` markers
  - [x]Test: template processing works end-to-end via `sandbox build -f`

## Dev Notes

### Architecture Compliance

- **Template placeholder format**: Conditional blocks use `# {{IF_NAME}}` / `# {{/IF_NAME}}` on their own lines as Dockerfile comments. Value substitution uses `{{NAME}}` inline. Block names derive from config keys in UPPER_SNAKE: `sdks.nodejs` -> `IF_NODE`, value -> `NODE_VERSION`. [Source: architecture.md#Template Placeholder Format]
- **Validation before substitution**: Template processing MUST validate before substituting: (1) all conditional blocks have matching open/close tags, (2) all value placeholders have corresponding config values or are inside a stripped conditional block. Never produce a Dockerfile with unresolved placeholders. [Source: architecture.md#Gap 5: Template processing error handling]
- **No eval**: Use `sed` or parameter expansion for template substitution. `eval` is an explicit anti-pattern. [Source: architecture.md#Anti-Patterns]
- **Exit code 1**: Template processing errors use exit code 1 (general error). [Source: architecture.md#Exit Codes]
- **No color codes, no spinners** -- plain text only.
- **`set -euo pipefail`** already enforced. Template processing must not trigger `set -u` failures on empty variables.

### Dockerfile.template Design

The template is NOT a valid Dockerfile until processed. It is a human-readable template with markers. Structure:

```dockerfile
FROM {{BASE_IMAGE}}

# Install tini for signal forwarding
RUN apt-get update && apt-get install -y tini

# System packages from config
RUN apt-get update && apt-get install -y {{PACKAGES}} && rm -rf /var/lib/apt/lists/*

# {{IF_NODE}}
RUN curl -fsSL https://deb.nodesource.com/setup_{{NODE_VERSION}}.x | bash - \
    && apt-get install -y nodejs
# {{/IF_NODE}}

# {{IF_PYTHON}}
RUN apt-get update && apt-get install -y python{{PYTHON_VERSION}} python3-pip \
    && rm -rf /var/lib/apt/lists/*
# {{/IF_PYTHON}}

# {{IF_GO}}
RUN curl -fsSL https://go.dev/dl/go{{GO_VERSION}}.linux-amd64.tar.gz | tar -C /usr/local -xzf -
ENV PATH="/usr/local/go/bin:${PATH}"
# {{/IF_GO}}

# Copy isolation scripts
COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY scripts/git-wrapper.sh /usr/local/bin/git

# Non-root user for Podman rootless
RUN useradd -m -s /bin/bash sandbox
USER sandbox

ENTRYPOINT ["tini", "--"]
CMD ["/usr/local/bin/entrypoint.sh"]
```

The exact template content should follow this structure but may differ in specifics (e.g., exact install commands). The critical requirement is that the conditional markers and value placeholders follow the exact `{{...}}` format.

### Base Image

Ubuntu 24.04 LTS pinned to digest. The digest should be hardcoded in `process_template()` as a constant (not in the template itself). This ensures reproducible builds (FR39, NFR12). The `{{BASE_IMAGE}}` placeholder gets resolved to the full `ubuntu:24.04@sha256:...` string.

Use a current digest for ubuntu:24.04 -- the exact value can be obtained via `docker pull ubuntu:24.04` and inspecting the digest, or use a known recent digest. The important thing is it's pinned, not that it's the absolute latest.

### Conditional Block Processing Algorithm

```
1. Read template into variable
2. Extract all IF_* tag names from the template
3. For each tag name:
   a. Check that both opening and closing tags exist -- if not, die with unmatched tag error
4. For each conditional block (IF_NODE, IF_PYTHON, IF_GO):
   a. If corresponding CFG_SDK_* is non-empty: remove the tag lines only (keep content between them)
   b. If corresponding CFG_SDK_* is empty: remove everything from opening tag through closing tag (inclusive)
5. Substitute value placeholders: {{NAME}} -> value
6. Scan for remaining {{...}} patterns -- if found, die with unresolved placeholder error
```

Use `sed` or `grep` + bash string operations. The processing can be done line-by-line or with multi-line sed. Key: process conditionals BEFORE value substitution, so that placeholders inside stripped blocks don't trigger unresolved errors.

### Packages Handling

`CFG_PACKAGES` is a bash array. For the `{{PACKAGES}}` placeholder, join the array elements with spaces: `"${CFG_PACKAGES[*]}"`. If the array is empty, the entire `apt-get install` line with `{{PACKAGES}}` should either be stripped or handled gracefully. Consider wrapping the packages install in a conditional, or handling empty packages in the substitution logic (e.g., if empty, remove the line).

### Previous Story (1-3) Intelligence

**Established patterns to follow:**
- `die()` for errors (message + exit code), `info()` for success messages
- `parse_config()` populates `CFG_*` globals consumed by build/run functions
- `CFG_SDK_NODEJS`, `CFG_SDK_PYTHON`, `CFG_SDK_GO` are empty strings when not configured
- `CFG_PACKAGES` is a bash array (may be empty)
- `cmd_build()` currently calls `parse_config` then prints "not yet implemented"
- Tests use TAP-like format with `pass()`/`fail()`/`assert_exit_code()`/`assert_contains()`
- Tests create temp directories with purpose-built configs and clean up after
- 77 tests currently pass

**Files modified in 1-3:**
- `sandbox.sh` -- Added CFG_* globals, parse_config() function, wired into cmd_build/cmd_run
- `tests/test_sandbox.sh` -- Added 22 parse_config tests (77 total)

**Key learnings from 1-3:**
- yq returns literal `"null"` for missing keys -- always check for both `""` and `"null"`
- Tests use `set +e` / `set -e` around commands that are expected to fail, capturing exit code
- All tests use temp directories for isolation

### Git History Context

Recent commits show sequential story implementation:
- `f0eb322 feat: implement config parsing with review fixes (story 1-3)`
- `654ee5e feat: implement sandbox init with config generation (story 1-2)`
- `72a7ddc feat: add CLI skeleton with dependency validation (story 1-1)`

### Current sandbox.sh Structure (280 lines)

The build functions stub is at lines 174-181:
```bash
# ============================================================================
# Build functions (stub)
# ============================================================================

cmd_build() {
  parse_config
  info "not yet implemented"
}
```

Add `process_template()` in the Build functions section, before `cmd_build()`. Update `cmd_build()` to call `process_template` after `parse_config`.

### Current Dockerfile.template

Currently a one-line placeholder: `# Placeholder -- implemented in a later story`. Replace entirely with the real template.

### Testing Strategy

Tests should:
1. Create minimal config YAML files in temp dirs with specific SDK combinations
2. Create a test Dockerfile.template in a temp dir (or use the real one from the project)
3. Run `sandbox build -f <config>` and verify:
   - Exit code 0 for valid configs
   - The generated Dockerfile contains expected content (Node.js install block present/absent, etc.)
   - Exit code 1 for mismatched tags or unresolved placeholders

For template validation tests (mismatched tags, unresolved placeholders), create intentionally broken templates in temp dirs and point SCRIPT_DIR at them, or test `process_template` more directly.

The generated Dockerfile location should be predictable so tests can read and inspect it. Consider writing it to a temp file and having `cmd_build` print the path, or writing it to a known location relative to SCRIPT_DIR.

### Anti-Patterns to Avoid

- Do NOT use `eval` for template substitution -- use `sed` or bash parameter expansion
- Do NOT parse the template line-by-line with `read` if `sed` can handle it in a single pass
- Do NOT create a separate template engine script -- keep it in sandbox.sh
- Do NOT hardcode SDK installation commands in sandbox.sh -- they belong in Dockerfile.template
- Do NOT skip tag validation -- mismatched tags must be caught before producing output
- Do NOT leave unresolved placeholders -- post-substitution scan is mandatory

### Project Structure Notes

- `Dockerfile.template` -- Replace placeholder with real template (this is the only file change outside sandbox.sh)
- `sandbox.sh` -- Add `process_template()` function, update `cmd_build()`
- `tests/test_sandbox.sh` -- Add template processing tests
- No new files needed

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Dockerfile Generation]
- [Source: _bmad-output/planning-artifacts/architecture.md#Template Placeholder Format]
- [Source: _bmad-output/planning-artifacts/architecture.md#Gap 5: Template processing error handling]
- [Source: _bmad-output/planning-artifacts/architecture.md#Anti-Patterns]
- [Source: _bmad-output/planning-artifacts/architecture.md#Exit Codes]
- [Source: _bmad-output/planning-artifacts/architecture.md#File Responsibilities -- Dockerfile.template]
- [Source: _bmad-output/planning-artifacts/architecture.md#Project Structure & Boundaries]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 1.4]
- [Source: _bmad-output/planning-artifacts/epics.md#FR38-FR40 -- Image Build System]
- [Source: _bmad-output/planning-artifacts/prd.md#Image Build System]
- [Source: _bmad-output/implementation-artifacts/1-3-configuration-parsing.md]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

### Completion Notes List

- Task 1: Created Dockerfile.template with Ubuntu 24.04 base, tini, common CLI tools, conditional SDK blocks (IF_NODE, IF_PYTHON, IF_GO), PACKAGES placeholder, entrypoint/git-wrapper COPY, non-root user setup
- Task 2: Implemented process_template() with tag validation (matching open/close), conditional block processing (keep/strip based on CFG_SDK_* values), value substitution via sed, and unresolved placeholder detection
- Task 3: Wired process_template() into cmd_build() -- writes resolved Dockerfile to .sandbox-dockerfile, prints info message, retains "not yet implemented" for docker build (story 1-5)
- Task 4: Added 36 new tests (113 total): SDK combinations, packages substitution, empty packages, mismatched tag errors, unresolved placeholder errors, no {{ markers check, end-to-end via sandbox build -f

### File List

- Dockerfile.template (modified -- replaced placeholder with full template)
- sandbox.sh (modified -- added process_template(), BASE_IMAGE constant, RESOLVED_DOCKERFILE global, updated cmd_build())
- tests/test_sandbox.sh (modified -- added 36 template processing tests)

### Change Log

- 2026-03-24: Implemented Dockerfile generation from template (story 1-4) -- process_template() with conditional blocks, value substitution, validation
