# asbox — Project Conventions

## Error Handling

- Always use `errors.Is()` and `errors.As()` for error comparison — never bare `==` or type switches
- Every new error type must be added to `exitCode()` in `cmd/root.go` and its test table in `cmd/root_test.go`
- Error messages follow the format: what failed + why + fix action
- Exit codes: 0 (success), 1 (config error), 2 (usage error), 3 (missing dependency), 4 (secret validation failure)

## Testing

- Table-driven tests for pure functions; individual test functions for CLI scenarios
- `t.TempDir()` for all temporary directories — never manual cleanup
- Use `t.Cleanup()` instead of `defer` for cleanup in tests with parallel subtests
- Struct construction over YAML parsing for unit tests
- No testify — stdlib `testing` package only
- Integration tests use testcontainers-go in `integration/`
- Binary invocation tests for CLI-level features that produce no container-observable output

## Code Organization

- Error types defined per owning package, not centralized
- Exit code mapping lives only in `cmd/`
- All embedded assets in `embed/` with `//go:embed` directives in `embed/embed.go`
- Import alias `asboxEmbed` for the project's `embed` package (avoids stdlib collision)
- Pure functions preferred (no I/O, no side effects) for testability

## Agent Registry

- New agents are added via `AgentConfigRegistry` entries — one per registry location
- Agent command mapping in `agentCommand()`, instruction target in `agentInstructionTarget()`, Dockerfile install block in `embed/Dockerfile.tmpl`
