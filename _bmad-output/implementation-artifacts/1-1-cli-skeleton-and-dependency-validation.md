# Story 1.1: CLI Skeleton and Dependency Validation

Status: ready-for-dev

## Story

As a developer,
I want to run `sandbox` and see usage help, and have the script validate that Docker and yq are installed,
so that I know the tool is working and will fail clearly if prerequisites are missing.

## Acceptance Criteria

1. **Given** a developer has sandbox.sh on their PATH
   **When** they run `sandbox --help`
   **Then** usage information is displayed showing available commands (init, build, run) and flags (-f, --help)

2. **Given** Docker is not installed on the host
   **When** the developer runs any sandbox command
   **Then** the script exits with code 3 and prints "error: docker not found" to stderr

3. **Given** yq is not installed or is below v4
   **When** the developer runs any sandbox command
   **Then** the script exits with code 3 and prints a clear error about the yq requirement to stderr

4. **Given** bash version is below 4
   **When** the developer runs sandbox.sh
   **Then** the script exits with code 3 and prints a clear error about the bash version requirement

## Tasks / Subtasks

- [ ] Task 1: Create project directory structure (AC: all)
  - [ ] Create `sandbox.sh` at project root with shebang, `set -euo pipefail`, and empty function stubs
  - [ ] Create `scripts/` directory with placeholder `entrypoint.sh` and `git-wrapper.sh`
  - [ ] Create `templates/` directory with placeholder `config.yaml`
  - [ ] Create placeholder `Dockerfile.template`
- [ ] Task 2: Implement utility functions (AC: 2, 3, 4)
  - [ ] Implement `die()` — prints "error: <message>" to stderr, exits with given code
  - [ ] Implement `info()` — prints plain text to stdout
  - [ ] Implement `warn()` — prints "warning: <message>" to stderr
- [ ] Task 3: Implement dependency validation (AC: 2, 3, 4)
  - [ ] Implement `check_dependencies()` function
  - [ ] Bash version check: `BASH_VERSINFO[0] >= 4`, exit 3 on failure
  - [ ] Docker check: `command -v docker`, exit 3 with "error: docker not found"
  - [ ] yq check: `command -v yq`, then parse `yq --version` output to verify v4+, exit 3 on failure
- [ ] Task 4: Implement help and command dispatch (AC: 1)
  - [ ] Implement `show_help()` — displays usage with commands (init, build, run) and flags (-f, --help)
  - [ ] Implement argument parsing: handle `--help`, `-f <path>`, and command dispatch
  - [ ] Route to stub functions for `init`, `build`, `run` (print "not yet implemented" and exit 0)
  - [ ] Unknown command/flag: exit 2 with usage error
- [ ] Task 5: Wire up main entry point (AC: all)
  - [ ] bash version check runs FIRST (before anything else, since other checks may use bash 4+ features)
  - [ ] `check_dependencies()` runs before any command dispatch
  - [ ] `main()` function ties it all together
  - [ ] Make `sandbox.sh` executable

## Dev Notes

### Architecture Compliance

- **Single file**: All logic in `sandbox.sh` — no external modules or sourced files
- **Script organization** (top-to-bottom reading order):
  1. Shebang (`#!/usr/bin/env bash`) and `set -euo pipefail`
  2. Constants and defaults (`DEFAULT_CONFIG_PATH=".sandbox/config.yaml"`, `SANDBOX_VERSION="0.1.0"`)
  3. Utility functions (`die`, `info`, `warn`)
  4. Config parsing functions (stub for now)
  5. Build functions (stub for now)
  6. Run functions (stub for now)
  7. Init function (stub for now)
  8. Command dispatch (argument parsing, route to function)
  9. Main entry point
- **Function naming**: `snake_case` — `check_dependencies`, `show_help`, `parse_args`
- **Variable naming**: locals `lower_snake_case`, constants `UPPER_SNAKE_CASE`
- **Quoting**: Always `"${var}"` — never unquoted `$var`

### Exit Codes

| Code | Meaning | Used in this story |
|------|---------|-------------------|
| 0 | Success | Help display, stub commands |
| 2 | Usage error | Unknown command/flag |
| 3 | Dependency error | Docker/yq/bash missing |

### Technical Requirements

- **Bash version check**: Use `BASH_VERSINFO[0]` (integer comparison, no string parsing needed). This check MUST be the very first thing in the script because subsequent code may use bash 4+ features (associative arrays, etc.)
- **Docker detection**: `command -v docker >/dev/null 2>&1` — do NOT check for a running daemon, just binary presence
- **yq version check**: `yq --version` outputs something like `yq (https://github.com/mikefarah/yq/) version v4.44.1`. Extract the major version number after `v` and verify >= 4. Be aware there are two different `yq` tools — Mike Farah's Go version (required) and the Python `yq` wrapper. Mike Farah's version outputs the URL in `--version`; use this to distinguish if needed.
- **macOS bash caveat**: macOS ships bash 3.2. Shebang MUST be `#!/usr/bin/env bash` (not `#!/bin/bash`) so Homebrew-installed bash 4+ is used when available. The bash version check catches the case where a user runs with system bash.
- **No color codes**: Plain text output only. No ANSI escape sequences, spinners, or progress bars.
- **Errors to stderr**: All error/warning output via `>&2`
- **Info to stdout**: Success and informational messages to stdout

### Output Format Examples

```
# Help output (stdout)
Usage: sandbox <command> [options]

Commands:
  init    Generate a starter .sandbox/config.yaml
  build   Build the sandbox container image
  run     Launch an interactive sandbox session

Options:
  -f <path>   Use specified config file (default: .sandbox/config.yaml)
  --help      Show this help message

# Error outputs (stderr)
error: docker not found -- install Docker Desktop or Docker Engine (https://docs.docker.com/get-docker/)
error: yq not found -- install yq v4+ (https://github.com/mikefarah/yq/#install)
error: yq version 3.x detected -- sandbox requires yq v4+ (https://github.com/mikefarah/yq/#install)
error: bash 4+ required (current: 3.2) -- on macOS run: brew install bash
error: unknown command 'foo'
```

### Anti-Patterns to Avoid

- Do NOT use `eval` anywhere
- Do NOT use `echo $var` — always `echo "${var}"`
- Do NOT `exit 1` without a message — always use `die "message"`
- Do NOT put error messages on stdout or info messages on stderr
- Do NOT add color codes, spinners, or progress bars
- Do NOT create helper scripts outside the defined structure
- Do NOT use `#!/bin/bash` — use `#!/usr/bin/env bash`

### Project Structure Notes

This is the first story — it creates the project structure from scratch. All files listed below should be created:

```
sandbox/
├── sandbox.sh                    # CLI entry point — implement fully in this story
├── Dockerfile.template           # Placeholder only (echo comment)
├── templates/
│   └── config.yaml               # Placeholder only (echo comment)
├── scripts/
│   ├── entrypoint.sh             # Placeholder only (shebang + comment)
│   └── git-wrapper.sh            # Placeholder only (shebang + comment)
```

Placeholder files should have a shebang (for .sh files) and a comment indicating they're implemented in a later story. This establishes the project structure without implementing functionality that belongs to other stories.

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Bash Coding Conventions]
- [Source: _bmad-output/planning-artifacts/architecture.md#Exit Codes]
- [Source: _bmad-output/planning-artifacts/architecture.md#Script Organization (sandbox.sh)]
- [Source: _bmad-output/planning-artifacts/architecture.md#Output & Error Formatting]
- [Source: _bmad-output/planning-artifacts/architecture.md#Anti-Patterns]
- [Source: _bmad-output/planning-artifacts/architecture.md#Complete Project Directory Structure]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 1.1]

## Dev Agent Record

### Agent Model Used

### Debug Log References

### Completion Notes List

### File List
