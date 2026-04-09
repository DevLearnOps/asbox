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
