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

## Deferred from: code review of story 6-1 (2026-04-09)

- Comma in directory names breaks entrypoint `chown_volumes` IFS parsing — `AUTO_ISOLATE_VOLUME_PATHS` uses comma-separated format; a directory name containing a comma produces an unparseable value. Requires entrypoint change (out of scope per spec). [internal/mount/isolate_deps.go:93, embed/entrypoint.sh:52]
- Symlinked subdirectories in monorepos silently skipped by `filepath.WalkDir` — monorepos using symlinks for shared packages won't get isolation volumes. Adding symlink following risks cycles. [internal/mount/isolate_deps.go:27]
- Multiple mounts to same container target can produce duplicate volume entries — two mounts with same Target and same relative package.json produce two ScanResults for the same container path; Docker uses last-one-wins. [internal/mount/isolate_deps.go:26-56]
- `project_name` special chars amplified to volume names — pre-existing unsanitized explicit project_name (tracked in story 1-7 review) now also affects Docker named volume names, not just container names. [internal/mount/isolate_deps.go:61]

## Deferred from: code review of story 6-1 rework (2026-04-10)

- No traversal depth limit or skip list for non-productive directories — `filepath.WalkDir` recurses without limits and doesn't skip `.git`, `vendor`, `.cache`, `dist`, etc.; performance concern for large mount sources. [internal/mount/isolate_deps.go:59]

## Deferred from: code review of story 7-1 (2026-04-09)

- Tests replicate RunE logic inline instead of exercising actual command handler — pragmatic trade-off since RunE calls `ensureBuild()`/`RunContainer()` which require mock infrastructure that doesn't exist. Future test refactoring should add command-level integration tests. [cmd/run_test.go:52-120]
- Error message hardcodes `.asbox/config.yaml` path — pre-existing pattern across all mount error messages. Config path may be overridden via CLI flag, making the hardcoded path misleading. [internal/mount/mount.go:42]

## Deferred from: code review of story 8-1 (2026-04-10)

- Mount flags lack `:ro` read-only qualifier — instruction file mount and repo mounts are read-write. Pre-existing pattern: no mounts in the codebase use `:ro`. [cmd/run.go:68]
- Content hash implicitly includes bmad_repos config — `cmd/build_helper.go` hashes the entire raw config YAML, so runtime-only fields like `bmad_repos` trigger unnecessary rebuilds. Pre-existing hash granularity issue affecting all runtime-only config fields.
- Cmd integration tests don't exercise full RunE success path — tests replicate RunE logic inline rather than running through `r.run("run")`. Pre-existing test pattern since tests would need to mock Docker. [cmd/run_test.go:177]

## Deferred from: code review of story 9-1 (2026-04-10)

- `execInContainer` missing `tcexec.Multiplexed()` — returns raw Docker multiplexed stream with binary framing headers. Current callers tolerate it via `strings.Contains`, but exact string comparisons would fail. [integration/integration_test.go:108]
- Cleanup closures in `startTestContainer` and `startTestContainerWithMounts` capture caller's `ctx` — if a future caller passes a cancellable context, `container.Terminate(ctx)` in `t.Cleanup` will fail silently. All current callers use `context.Background()`. [integration/integration_test.go:94,237]

## Deferred from: code review of story 9-3 (2026-04-10)

- AC #2 only checks sandbox user home, not root — `/root/.ssh` and `/root/.aws` are not verified absent inside the container. The sandbox user's home is the primary concern, but root paths are unchecked. [isolation_test.go:53-73]
- AC #1 git push test has no remote configured — the git wrapper intercepts before remote evaluation so the test is behaviorally correct, but `setupGitRepoWithRemote` helper exists unused; the test is less realistic than the AC describes. [isolation_test.go:20]
- `not_reachable_from_outside` subtest uses proxy check — inspects outer container's `NetworkSettings.Ports` for no bindings rather than attempting an actual connection from the host. Necessary but not sufficient condition. [inner_container_test.go:144-169]
- `ls` locale-dependent error message in credential path tests — `strings.Contains(output, "No such file or directory")` is locale-dependent; exit code check alone would be more portable. [isolation_test.go:53-69]
- FR36 private network bridge coverage is partial — DNS resolution test proves inter-service connectivity but doesn't assert network isolation from host or other outer containers. [inner_container_test.go:100-109]

## Deferred from: code review of story 10-1 (2026-04-10)

- `go install github.com/mcastellin/asbox@latest` in README assumes the repo is published to that Go module path — instruction will fail until a version tag is pushed. Module path matches `go.mod` but no release exists yet. [README.md:42]
