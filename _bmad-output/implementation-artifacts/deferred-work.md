# Deferred Work

Consolidated from code reviews across all stories. Organized by category. Duplicates merged. Resolved items removed.

Last updated: 2026-04-17 (code review of story 11-6)

---

## Security / Input Validation

- **SDK version string injection** — Malicious values (e.g., `22; rm -rf /`) in SDK version fields could inject shell commands via Dockerfile ARG/RUN rendering. [internal/config/parse.go] _(stories 1-4, 1-5)_
- **Package name injection** — Values with shell metacharacters could inject commands via `apt-get install`. Empty strings in Packages slice produce invalid Dockerfile syntax. [internal/config/parse.go] _(stories 1-4, 1-5)_
- **ENV key/value injection** — ENV keys not validated for shell variable format (spaces, leading digits). YAML multiline strings in env values can inject arbitrary Dockerfile directives via unescaped `ENV {{$k}}={{$v}}`. [internal/config/parse.go] _(story 1-3)_
- **Template injection via unsanitized config inputs** — Config inputs injected directly into Dockerfile RUN/ENV directives without sanitization. [embed/Dockerfile.tmpl] _(story 1-5)_
- **Unsanitized explicit `project_name`** — `sanitizeProjectName()` only runs when name is derived, not when explicitly set. Affects Docker image tags, container names, and named volume names. [internal/config/parse.go] _(stories 1-7, 6-1)_

## Reliability / Correctness

- **Container name collision on concurrent runs** — Deterministic `asbox-<project>` name means second instance fails. [cmd/run.go:49] _(story 1-7, also in future-work.md)_
- **`-it` flag hardcoded** — Fails in non-TTY contexts (CI/CD, piped input) with "the input device is not a TTY." [internal/docker/run.go:22] _(story 1-7)_
- **Comma in directory names breaks `chown_volumes` IFS parsing** — `AUTO_ISOLATE_VOLUME_PATHS` uses comma-separated format. [internal/mount/isolate_deps.go:93, embed/entrypoint.sh:52] _(story 6-1)_
- **Colon in source/target paths breaks Docker `-v` format** — Rare on Linux but possible. [internal/mount/mount.go:23] _(story 2-1)_
- **Duplicate mount targets silently conflict** — Two mounts targeting the same container path; Docker uses last-one-wins. [internal/mount/mount.go] _(story 2-1)_
- **Multiple mounts to same container target produce duplicate volume entries** — Two mounts with same Target and same relative package.json. [internal/mount/isolate_deps.go] _(story 6-1)_
- **Symlinked subdirectories silently skipped** — Monorepos using symlinks won't get isolation volumes. Adding symlink following risks cycles. [internal/mount/isolate_deps.go:27] _(story 6-1)_
- **`os.IsNotExist` misses `ENOTDIR`** — Opaque error when a path component is a file instead of a directory. [internal/config/parse.go:25] _(story 1-2)_
- **Content hash includes runtime-only fields** — `bmad_repos` and other runtime-only config fields trigger unnecessary rebuilds. [cmd/build_helper.go] _(story 8-1)_
- **Background Podman PID not tracked** — `podman system service` launched with `&` but PID not captured. Tini reaps orphans, so low risk. [embed/entrypoint.sh] _(story 4-1)_

## Reproducibility

- **Docker Compose version not pinned** — Fetches `latest` from GitHub API at build time. Non-reproducible, subject to rate limits. [embed/Dockerfile.tmpl] _(stories 1-5, 2-2, 4-2)_
- **npm agent CLIs not version-pinned** — `gemini-cli` and `@openai/codex` install latest at build time. [embed/Dockerfile.tmpl] _(stories 2-2, 1-10)_
- **Base image digest replaced with floating tag** — `FROM ubuntu:24.04` without digest loses reproducibility. [embed/Dockerfile.tmpl:2] _(story 2-2)_
- **Curl-pipe-bash installers without integrity verification** — NodeSource, get-pip.py, Claude Code install script, Docker Compose. [embed/Dockerfile.tmpl] _(story 2-2)_

## Architecture / Maintainability

- **Parallel switch statements diverge when adding agents** — `agentCommand`, `agentInstructionTarget`, Dockerfile.tmpl each have independent agent switch/if blocks. [cmd/run.go, embed/Dockerfile.tmpl] _(stories 1-10, 9-6)_
- **Hardcoded supported-agent lists in error messages** — Agent list appears as literal strings in agentCommand, ValidateAgent, and --agent flag help text. Manual maintenance needed. [multiple files] _(story 1-10)_
- **Node.js SDK validation not consolidated** — Separate if-blocks per agent for Node.js requirement. Grows with each npm-based agent. [internal/config/parse.go] _(story 1-10)_
- **Mutable global `AgentConfigRegistry`** — Tests mutate package-level map and restore in t.Cleanup. Data race risk if `t.Parallel()` added. [internal/mount/mount_test.go] _(stories 1-10, 9-6)_
- **bmad_repos instruction mount only targets default agent** — Non-default agents retain generic build-time instructions. [cmd/run.go:81-92] _(story 1-10)_
- **Mount flags lack `:ro` qualifier** — Instruction file and repo mounts are read-write. No mounts use `:ro`. [cmd/run.go:68] _(story 8-1)_
- **`resolvePath` allows `..` traversal** — By design for user-owned host mounts, but undocumented. [internal/config/parse.go] _(story 1-2)_
- **Docker check scope too broad** — `PersistentPreRunE` runs Docker check on all commands including `--help` and `init`. [cmd/root.go] _(stories 1-1, 1-8)_
- **Test helper `newRootCmd()` mutates package-level state** — Fragile under parallel tests. [cmd/root_test.go] _(story 1-1)_
- **`docker/docker` +incompatible in main require** — Test-only dependency in production `require` section. [go.mod] _(story 4-1)_
- **Error message hardcodes `.asbox/config.yaml` path** — Misleading when `--file` flag overrides it. [internal/mount/mount.go:42] _(story 7-1)_
- **Misleading error prefix in render.go** — Both parse and FS-read failures use same "failed to render Dockerfile" prefix. [internal/template/render.go] _(story 5-1)_
- **`setup_codex_home()` untested at integration level** — Symlink logic verified only by inspection; CI constraints justify deferral. [embed/entrypoint.sh] _(story 9-6)_
- **No test for claude/gemini in `agentInstructionTarget()`** — Pre-existing paths moved without behavior change. [cmd/run_test.go] _(story 9-6)_

## Deferred from: code review of story 11-1 (2026-04-15)

- **`TestRunContainerNameMatchesPattern` doesn't exercise production code path** — Test manually constructs the container name instead of exercising the actual `runCmd` code path. Fix approach debatable (refactor container name construction into testable function vs binary invocation test). [cmd/run_test.go:199-205]
- **No binary invocation test for CLI-level name generation** — No test exercises the `asbox run` binary to verify it produces suffixed container names. CLAUDE.md recommends binary invocation tests for CLI-level features. Nice-to-have coverage improvement.
- **Project names with uppercase/underscores not validated** — `cfg.ProjectName` can contain characters outside the `[a-z0-9-]` charset assumed by the container name pattern regex. Pre-existing; story 11-2 will address input sanitization.

## Deferred from: code review of story 11-2 (2026-04-16)

- **Apt release pinning with `/` rejected by package regex** — `packageNameRe` does not include `/`, so release-pinned apt syntax like `vim/jammy-backports` is rejected. Valid but niche; can be added if a user requests it. [internal/config/parse.go:23]
- **Apt tilde `~` versions rejected by package regex** — Debian version strings can contain `~` for pre-release sorting (e.g., `1.0~beta1`). Uncommon in practice for direct package installs; can be added if needed. [internal/config/parse.go:23]

## Deferred from: code review of story 11-6 (2026-04-17)

- **Empty/whitespace-only `AGENT_CMD` bypasses `die`** — The `[[ -n "${AGENT_CMD:-}" ]]` guard accepts a value of `" "` (single space). After unquoted word-split, argv is empty and `exec gosu sandbox` fails without the helpful `die "No agent command specified"` diagnostic. Pre-existing branch condition; producer side (`agentCommand` in Go) never emits whitespace-only values. [embed/entrypoint.sh:206]
- **`exec gosu sandbox` invocations lack `--` separator** — Neither the `$@` branch nor the `${AGENT_CMD}` branch inserts `--` between `sandbox` and the command. If a future agent command began with a dash, `gosu` could parse it as its own option. Applies to both branches; out-of-scope for story 11-6. [embed/entrypoint.sh:205,209]
- **`TestAgentCommand_noShellMetacharacters` hardcodes the agent list** — Test iterates the literal slice `{claude, gemini, codex}` rather than a single source of truth. A future agent entry added to `agentCommand()` without a matching test update would escape the metacharacter invariant. Spec explicitly mandated the hardcoded list (Task 2.2). [cmd/run_test.go:275]

## Integration Test Quality

- **`execInContainer` missing `tcexec.Multiplexed()`** — Returns raw Docker multiplexed stream with binary framing. Callers compensate with `strings.Contains`. [integration/integration_test.go:108] _(story 9-1)_
- **Cleanup closures capture caller's ctx** — If a future caller passes a cancellable context, `container.Terminate(ctx)` in `t.Cleanup` will fail silently. [integration/integration_test.go] _(story 9-1)_
- **No context timeout on integration tests** — All use `context.Background()` with no deadline; tests can hang indefinitely. [integration/] _(story 4-2)_
- **nc-based HTTP server has race window** — BusyBox nc exits after each connection; retry loop mitigates but fragile in slow CI. [integration/inner_container_test.go] _(story 4-2)_
- **Locale-dependent error string assertion** — `strings.Contains(output, "No such file or directory")` is locale-dependent. [isolation_test.go] _(story 9-3)_
- **AC #2 doesn't check `/root/.ssh`/`.aws`** — Sandbox user home is checked, root paths are not. [isolation_test.go] _(story 9-3)_
- **FR36 private network bridge coverage partial** — DNS test proves connectivity but not isolation from host. [inner_container_test.go] _(story 9-3)_
- **Cmd integration tests don't exercise full RunE path** — Tests replicate logic inline; needs mock Docker client. [cmd/run_test.go] _(stories 7-1, 8-1)_

## Distribution

- **`go install` path won't work until version tag published** — README instruction will fail. [README.md:42] _(story 10-1)_

## By Design (Documented)

- **`git -c key=val push` bypasses wrapper** — Known per accidental threat model. [embed/git-wrapper.sh] _(story 3-1)_
- **Only `push` blocked; other exfiltration open** — By design per threat model — convenience boundary. _(story 3-1)_
- **No traversal depth limit in `ScanDeps`** — `filepath.WalkDir` doesn't skip `.git`, `vendor`, `.cache`. Performance concern for large repos. [internal/mount/isolate_deps.go] _(story 6-1)_

## Deferred from: code review of 11-5-pinned-build-dependencies (2026-04-17)

- No checksum verification on Docker Compose binary download — `embed/Dockerfile.tmpl` fetches Docker Compose via curl without SHA256 integrity check. Pre-existing pattern, not introduced by version pinning.
- Build-time `npx playwright install` browser versions determined by transitive deps — browser builds fetched at build time are controlled by `@playwright/mcp`'s dependency tree, not explicitly pinned. Pre-existing pattern.

## Deferred from: code review of 12-1-short-flag-and-positional-agent-argument (2026-04-18)

- `resetRunCommandState` does not reset the persistent root `-f`/`file` flag — no test currently uses `-f` in-process, but a future test using it will leak config-file state across subtests. [cmd/run_test.go:31]
- `resetRunCommandState` mutates pflag's `flag.Changed` struct field directly — works but reaches into library internals. Latent breakage if pflag's internal layout changes. [cmd/run_test.go:31-61]
- Package-level `rootCmd`/`runCmd` shared across tests — any future `t.Parallel()` on unit tests would race on `SetArgs` and flag state. Pre-existing architectural constraint. [cmd/root.go]
- Control-character echoing in `ValidateAgent` unsupported-agent error — raw positional interpolated into the `ConfigError` message; inputs with `\n`/`\r` produce multi-line terminal output. Error-path only. [internal/config/parse.go:229]
