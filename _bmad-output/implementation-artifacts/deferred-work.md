# Deferred Work

## Deferred from: code review of story 1-1 (2026-04-08)

- Docker check in `PersistentPreRunE` runs on `--help` and non-docker commands like `init`. Should scope the check to commands that actually use Docker (`build`, `run`). [cmd/root.go:35]
- Test helper `newRootCmd()` mutates package-level `rootCmd` instead of creating a fresh command tree. Works today but fragile if tests are parallelized or grow more complex. [cmd/root_test.go:17]

## Deferred from: code review of story 1-2 (2026-04-08)

- `os.IsNotExist` check in `Parse()` misses `ENOTDIR` errors — when a path component is a file instead of a directory, the user gets an opaque "cannot read config file" error instead of the actionable "config file not found" hint. [internal/config/parse.go:25]
- `init` stub returns `ConfigError{Msg: "not implemented"}` — semantically incorrect error type for a "not implemented" condition. Story 1-8 will replace this with real init logic. [cmd/init.go]
- `resolvePath` allows `..` traversal outside the project directory. By design for user-owned host mounts, but the decision to not restrict it is undocumented. [internal/config/parse.go:104-106]

## Deferred from: code review of story 1-4 (2026-04-09)

- SDK version strings not validated in `Parse()` — malicious values (e.g., `22; rm -rf /`) could inject shell commands via Dockerfile ARG/RUN rendering. [internal/config/parse.go]
- Package names not validated in `Parse()` — values with shell metacharacters could inject commands via the Packages template block's `apt-get install`. [internal/config/parse.go]
- Empty string in Packages slice produces invalid Dockerfile syntax — `["", "vim"]` renders a bare backslash continuation line. Config parser should reject empty package names. [internal/config/parse.go]

## Deferred from: code review of story 1-3 (2026-04-08)

- ENV key format validation missing in `config.Parse` — env var keys are not checked for valid shell variable name format (e.g., no spaces, no leading digits). Could produce invalid Dockerfile ENV directives.
- ENV value newline injection not blocked by `config.Parse` — YAML multiline strings in env values can inject arbitrary Dockerfile directives via the template's unescaped `ENV {{$k}}={{$v}}` rendering. Validation should reject or sanitize newlines in env values.

## Deferred from: code review of story 1-5 (2026-04-09)

- Docker Compose version not pinned — fetches `latest` from GitHub API at build time, non-reproducible and subject to rate limits. [embed/Dockerfile.tmpl:75]
- Gemini CLI requires Node.js SDK but no config validation enforces it — `npm install` will fail at build time if Node.js not configured. [embed/Dockerfile.tmpl:88]
- Template injection via unsanitized package names and env values — config inputs injected directly into Dockerfile RUN/ENV directives without sanitization. [embed/Dockerfile.tmpl:54-56,112-114]

## Deferred from: code review of story 1-7 (2026-04-09)

- `-it` flag hardcoded in `RunContainer` — fails in non-TTY contexts (CI/CD, piped input) with "the input device is not a TTY." Out of scope for story 1-7 (interactive-only). [internal/docker/run.go:22]
- Unsanitized explicit `project_name` breaks Docker image tags and container names — `sanitizeProjectName()` only runs when name is derived, not when explicitly set. Pre-existing config validation gap. [internal/config/parse.go]
- Container name collision on concurrent `asbox run` invocations — deterministic `asbox-<project>` name means second instance fails. Single-instance by design. [cmd/run.go:49]

## Deferred from: code review of story 1-8 (2026-04-09)

- `PersistentPreRunE` override scope too broad — `initCmd` uses `PersistentPreRunE` instead of `PreRunE` to bypass Docker check, meaning any future subcommand of `init` would also skip the check silently. [cmd/init.go:13-15]

## Deferred from: code review of story 2-1 (2026-04-09)

- `HostAgentConfig` mount not included in `AssembleMounts` — future Story 7-1 scope. The field is parsed and path-resolved but never passed to Docker. [internal/mount/mount.go]
- Colon in source/target paths breaks Docker `-v` format — `source:target` concatenation breaks if either path contains a colon. Pre-existing Docker `-v` limitation; rare on Linux. [internal/mount/mount.go:23]
- Duplicate mount targets silently conflict — two mounts targeting the same container path produce two `-v` flags; Docker uses last-one-wins. No cross-mount validation in config.Parse() or AssembleMounts(). [internal/mount/mount.go]

## Deferred from: code review of story 2-2 (2026-04-09)

## Deferred from: code review of story 3-1 (2026-04-09)

- `git -c key=val push` bypasses wrapper — known limitation per accidental threat model. Loop breaks at first non-flag arg and never sees `push`. Documented in architecture.
- Only `push` is blocked — other exfiltration vectors (git archive, git bundle, curl) remain open. By design per accidental threat model — convenience boundary, not security boundary.
- No explicit test for hash invalidation when git-wrapper.sh changes — `internal/hash/hash.go` includes embedded scripts but no test verifies content change triggers hash change. Could be added in Epic 9.

## Deferred from: code review of story 2-2 (2026-04-09)

- Digest-pinned base image replaced with floating tag — `FROM ubuntu:24.04` without digest loses reproducibility. Multi-arch digests could restore both properties. [embed/Dockerfile.tmpl:2]
- Curl-pipe-bash pattern for multiple installers — NodeSource, get-pip.py, Claude Code install script, Docker Compose download all execute remote code without integrity verification. [embed/Dockerfile.tmpl]
- `gemini-cli` agent requires npm but NodeJS SDK not enforced — config validation does not reject `agent: gemini-cli` without `sdks.nodejs`. Build fails at `npm install -g`. [embed/Dockerfile.tmpl:83]
- Docker Compose version fetched from GitHub API with no pinning — non-reproducible builds, subject to API rate limits. [embed/Dockerfile.tmpl:72]

## Deferred from: code review of story 4-1 (2026-04-09)

- `docker/docker` +incompatible in main require block — test-only dependency (`github.com/docker/docker`) is in the production `require` section of `go.mod`. Could be isolated to a test-only module. [go.mod]
- Background Podman PID not tracked for cleanup — `podman system service` launched with `&` but PID not captured. tini as PID 1 reaps orphans, so low risk. [embed/entrypoint.sh:110]
- `AGENT_CMD` injection via shell expansion — `exec gosu sandbox bash -c "${AGENT_CMD}"` passes unsanitized input through `bash -c`. Pre-existing pattern. [embed/entrypoint.sh:140]

## Deferred from: code review of story 5-1 (2026-04-09)

- Misleading error message prefix in render.go — both `template.Parse` failure and embedded-FS read failure use the same "failed to render Dockerfile" prefix, making parse errors harder to diagnose. Pre-existing pattern. [internal/template/render.go:15-17]

## Deferred from: code review of story 4-2 (2026-04-09)

- nc-based HTTP server has race window between connections — BusyBox nc exits after each connection, leaving a brief unbound window before the while-loop restarts it; retry loop mitigates but fragile in slow CI. [integration/inner_container_test.go:121-124]
- Docker Compose binary and apt packages are unpinned — all packages in the Podman RUN block use latest versions, consistent with existing project convention. [embed/Dockerfile.tmpl:61,72]
- No context timeout on integration tests — all integration test files use context.Background() with no deadline; tests can hang indefinitely. Consistent across the test suite. [integration/inner_container_test.go]
