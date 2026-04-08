# Deferred Work

## Deferred from: code review of story 1-1 (2026-04-08)

- Docker check in `PersistentPreRunE` runs on `--help` and non-docker commands like `init`. Should scope the check to commands that actually use Docker (`build`, `run`). [cmd/root.go:35]
- Test helper `newRootCmd()` mutates package-level `rootCmd` instead of creating a fresh command tree. Works today but fragile if tests are parallelized or grow more complex. [cmd/root_test.go:17]
