# Story 12.1: Short `-a` Flag and Positional Agent Argument for `asbox run`

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want to switch agents quickly with either `asbox run -a codex` or `asbox run codex`,
so that I don't have to type `--agent` every time I want to try a different agent.

## Acceptance Criteria

1. **Given** the developer runs `asbox run -a codex`
   **When** the CLI parses flags
   **Then** `codex` is used as the agent override (must be in `installed_agents`) with identical validation and behavior to `--agent codex`

2. **Given** the developer runs `asbox run codex` (positional argument, no flag)
   **When** the CLI parses arguments
   **Then** `codex` is used as the agent override with identical validation and behavior to `--agent codex`

3. **Given** the developer runs `asbox run codex --agent claude` or `asbox run -a claude codex`
   **When** the CLI parses arguments
   **Then** the CLI exits with code 2 and prints exactly `error: agent specified both as positional argument ('codex') and via --agent ('claude') — use only one form` — and no container is started

4. **Given** the developer runs `asbox run notanagent` (positional value not in `installed_agents`)
   **When** the CLI parses arguments
   **Then** the CLI exits with code 1 via the existing `ValidateAgentInstalled` error path, naming the unknown agent and listing the installed agents

5. **Given** the developer runs `asbox run` with neither positional nor flag
   **When** the CLI resolves the agent
   **Then** `cfg.DefaultAgent` is used as before — no behavior change for the default case

6. **Given** the developer runs `asbox run --help`
   **When** the help text is rendered
   **Then** both the short flag (`-a, --agent`) and the positional `[agent]` appear in the usage line, each with a one-line description

## Tasks / Subtasks

- [x] Task 1: Reuse the existing `usageError` type — do NOT create `UsageError` or a new `cmd/errors.go` file (AC: #3)
  - [x] 1.1 Read `cmd/root.go:19-25` — the lowercase `usageError` struct already exists and is already mapped to exit code 2 by `exitCode()` (`cmd/root.go:51,60-61`). It has a test row in `cmd/root_test.go:51`. No new type, no new exit-code mapping, no new test row are needed
  - [x] 1.2 The epic's Implementation Notes reference `UsageError` and a `cmd/errors.go` file — treat those as stale wording. The intent (exit code 2 for usage errors) is already satisfied by `usageError`. Reusing it avoids a duplicate error hierarchy and keeps CLAUDE.md's "Error types defined per owning package" invariant
  - [x] 1.3 Construct the mutual-exclusion error as `&usageError{err: fmt.Errorf("agent specified both as positional argument (%q) and via --agent (%q) — use only one form", positional, flagVal)}` — use `%q` for Go-style single-quote rendering identical to the AC#3 expected string

- [x] Task 2: Wire the short flag, positional arg, and resolution logic in `cmd/run.go` (AC: #1, #2, #5, #6)
  - [x] 2.1 Change `runCmd.Use` from `"run"` (`cmd/run.go:19`) to `"run [agent]"` so the positional surfaces in help
  - [x] 2.2 Add `Args: cobra.MaximumNArgs(1)` on the `runCmd` struct literal alongside `Use`/`Short`/`RunE`. Zero or one positional argument; Cobra returns a usage error (auto-wrapped to `usageError` by `rootCmd.SetFlagErrorFunc` — confirm via `cmd/root.go:44-46`) for two or more args, which already yields exit 2
  - [x] 2.3 Verify that `SetFlagErrorFunc` in `cmd/root.go:44-46` also wraps `Args` validation errors. Cobra's default behavior routes `Args` errors through the same error path; if a manual test shows `asbox run a b c` returns a bare error string (exit 1 via `default` in `exitCode()`), add the args error to the usageError wrapping by returning the error from `RunE` wrapped in `&usageError{err: err}` — but only after confirming the default path is insufficient. Do not over-engineer this: start with the default, run `TestRun_tooManyArgs` (Task 4.3), and only intervene if exit code is not 2
  - [x] 2.4 In `cmd/run.go:211-213` (the `init()` flag registration), change `runCmd.Flags().String("agent", "", ...)` to `runCmd.Flags().StringP("agent", "a", "", "Override default agent for this session (e.g., claude, gemini, codex). Also accepted as a positional argument.")`. Keep the same default (`""`) and same long name (`--agent`)
  - [x] 2.5 In `runCmd.RunE` (currently `cmd/run.go:21-37`), replace the existing `agentOverride, _ := cmd.Flags().GetString("agent")` / `if agentOverride != ""` block with the resolution logic:
    ```go
    flagChanged := cmd.Flags().Changed("agent")
    flagVal, _ := cmd.Flags().GetString("agent")
    var positional string
    if len(args) == 1 {
        positional = args[0]
    }

    var agentOverride string
    switch {
    case positional != "" && flagChanged:
        return &usageError{err: fmt.Errorf("agent specified both as positional argument (%q) and via --agent (%q) — use only one form", positional, flagVal)}
    case positional != "":
        agentOverride = positional
    case flagChanged:
        agentOverride = flagVal
    }

    if agentOverride != "" {
        if err := config.ValidateAgent(agentOverride); err != nil {
            return err
        }
        if err := config.ValidateAgentInstalled(agentOverride, cfg.InstalledAgents); err != nil {
            return err
        }
        cfg.DefaultAgent = agentOverride
    }
    ```
  - [x] 2.6 Critical: use `cmd.Flags().Changed("agent")`, NOT `flagVal != ""`. A user running `asbox run -a claude` while `default_agent: claude` in config would have `flagVal == "claude"` and also `positional == "claude"` if they also pass `claude` positionally — but the presence check must detect whether the user typed `-a`/`--agent`, not whether the value is non-empty. `Changed()` returns `true` only when the user explicitly set the flag
  - [x] 2.7 Do NOT duplicate `ValidateAgent` / `ValidateAgentInstalled` — both positional and flag forms flow through the same validation block. That is the one-source-of-truth invariant

- [x] Task 3: Emit the exact error string and verify it matches AC#3 byte-for-byte (AC: #3)
  - [x] 3.1 AC#3 specifies the error as `error: agent specified both as positional argument ('codex') and via --agent ('claude') — use only one form`
  - [x] 3.2 The `error: ` prefix is printed by `cmd/root.go:77` (`fmt.Fprintf(os.Stderr, "error: %s\n", err)`) — `usageError.Error()` must return only the `agent specified both ...` portion, not the `error: ` prefix. This already matches the existing `usageError.Error()` impl (`cmd/root.go:24` returns `e.err.Error()`), so just make sure the wrapped `fmt.Errorf` does not include `error: `
  - [x] 3.3 The `em dash` in the expected string is a real Unicode em-dash (U+2014), not two hyphens. Copy-paste the character from the AC verbatim; do not retype as `--` or `-`. Verify with `len("—") == 3` (3 bytes UTF-8)
  - [x] 3.4 `%q` renders Go strings with double quotes by default (e.g., `"codex"`), but AC#3 expects single quotes (`'codex'`). Use explicit single quotes in the format string: `fmt.Errorf("agent specified both as positional argument ('%s') and via --agent ('%s') — use only one form", positional, flagVal)` — NOT `%q`. The AC is prescriptive about single quotes

- [x] Task 4: Unit tests in `cmd/run_test.go` — one subtest per AC (AC: all)
  - [x] 4.1 Follow the existing CLI-scenario pattern in `cmd/run_test.go:31-57` (`TestRun_nonexistentMountSource_returnsConfigError` — `newRootCmd()` + `r.run(...)`). Each subtest writes a minimal config via `writeRunConfig(t, dir, ...)` with `installed_agents: [claude, gemini, codex]` so ValidateAgentInstalled passes for all three
  - [x] 4.2 Add `TestRun_agentShorthand_equivalentToFlag` — verify `run`-ning with `-a gemini` and with `--agent gemini` both reach the `docker run` orchestration stage (will fail with a `BuildError` or `RunError` because docker build isn't mocked — that's expected). The assertion is that the error class is NOT `*usageError` and NOT `*ConfigError` for agent resolution. This covers AC#1
  - [x] 4.3 Add `TestRun_agentPositional_equivalentToFlag` — same as 4.2 but passing `gemini` as positional arg. Covers AC#2
  - [x] 4.4 Add `TestRun_agentBothPositionalAndFlag_usageError` — pass both (`"run", "codex", "--agent", "claude"` AND separately `"run", "-a", "claude", "codex"`). Assert: `err` is `*usageError`, exit code is 2, error message equals the full expected string from AC#3 byte-for-byte. Covers AC#3
  - [x] 4.5 Add `TestRun_agentPositionalNotInstalled_configError` — pass `"run", "notanagent"` with config listing only `[claude]`. Assert: `err` is `*config.ConfigError`, exit code is 1, message contains "not installed". Covers AC#4
  - [x] 4.6 Add `TestRun_agentDefault_usesConfigDefaultAgent` — assert that with no positional and no flag, the resolved agent is `cfg.DefaultAgent`. Because full RunE execution requires docker, implement this as a pure-Go unit test on the resolution logic (see Task 4.8 if direct testing is blocked). Covers AC#5
  - [x] 4.7 Add `TestRun_helpContainsPositionalAndShortFlag` — run `"run", "--help"`, assert the output contains all of: `run [agent]` (usage line), `-a, --agent` (flag line), and a short positional description. Covers AC#6
  - [x] 4.8 If testing the full `RunE` resolution in 4.2/4.3/4.4 proves hard without stubbing docker, extract the resolution logic into a small unexported helper `resolveAgentOverride(positional string, flagChanged bool, flagVal string) (override string, err error)` in `cmd/run.go` and unit-test it directly. This is the preferred approach — keep `RunE` thin, test the pure function. Use this pattern; it aligns with CLAUDE.md "Pure functions preferred (no I/O, no side effects) for testability"
  - [x] 4.9 Do NOT modify or remove existing tests `TestAgentCommand_claude/gemini/codex/unknown`, `TestAgentCommand_noShellMetacharacters`, or `TestRun_nonexistentMountSource_*`. They must stay green
  - [x] 4.10 Do NOT add a test row for `usageError` to `cmd/root_test.go:40-53` — it is already there at line 51. The existing row covers the exit-code mapping; the new tests cover emission of `usageError` from the specific both-set code path

- [x] Task 5: Integration test (binary invocation) in `integration/multi_agent_test.go` (AC: #1)
  - [x] 5.1 Follow the `TestMultiAgent_uninstalledAgentRejected` pattern in `integration/multi_agent_test.go:70-138`: build the binary via `go build -o binaryPath .`, write a minimal config, `exec.Command(binaryPath, "run", "-a", "gemini", "-f", configPath)`
  - [x] 5.2 The exact assertion is that the `-a` short flag is accepted and produces the same validation failure path as `--agent`. A config with `installed_agents: [claude]` and running `-a gemini` should exit code 1 with `not installed` in output — identical to the existing `uninstalled_agent_exits_code_1` subtest at line 86, just with `-a` instead of `--agent`
  - [x] 5.3 Add the test as a new subtest inside the existing `TestMultiAgent_uninstalledAgentRejected` table (line 86) or as a new top-level test. Prefer a new subtest within the existing table to reuse the test config and binary build
  - [x] 5.4 A full end-to-end "sandbox actually launches with gemini" test is NOT needed — the existing `multi_agent_test.go:86-138` already exercises the happy-path validation flow and the unit tests (Task 4) cover the Go resolution path. Binary invocation here exists only to confirm Cobra flag wiring reaches RunE, not to re-test agent-installation logic

- [x] Task 6: Update PRD/docs text reference to reflect wired short flag (AC: none — dev-completeness)
  - [x] 6.1 `_bmad-output/planning-artifacts/prd.md:300-302` already documents `asbox run [agent]` and the `-a`/`--agent` combo. No change required; the epic and PRD were authored assuming this story's implementation
  - [x] 6.2 Check `README.md` and `embed/config-template.yaml` for stale `--agent`-only wording. If present, update to mention `-a` and the positional form. If neither mentions `--agent`, no change
  - [x] 6.3 Do NOT add a new docs section or a new page. If only one existing line references `--agent`, extend that line to also mention `-a` / positional. Scope discipline — this story is CLI wiring, not documentation expansion

- [x] Task 7: Run full test suite and format (AC: all)
  - [x] 7.1 `gofmt -w cmd/run.go cmd/run_test.go integration/multi_agent_test.go`
  - [x] 7.2 `go vet ./...`
  - [x] 7.3 `go test ./cmd` — expect all tests green
  - [x] 7.4 `go test ./...` — expect all packages green, including `embed/`, `internal/*`
  - [x] 7.5 `go test ./integration -run TestMultiAgent` — expect green (requires docker; skip if docker unavailable and note in Completion Notes)

## Dev Notes

### The Exact Change Set

| File | Change | Lines |
|------|--------|-------|
| `cmd/run.go` | `Use: "run [agent]"`, add `Args: cobra.MaximumNArgs(1)` | 18-21 |
| `cmd/run.go` | Replace `--agent` resolution block with the 4-case switch | 27-37 |
| `cmd/run.go` | Optionally extract `resolveAgentOverride()` pure helper for testability | new func (Task 4.8) |
| `cmd/run.go` | `runCmd.Flags().StringP("agent", "a", "", "...")` | 212 |
| `cmd/run_test.go` | 6 new subtests covering AC#1-6 | append |
| `integration/multi_agent_test.go` | 1 new subtest — `-a` shorthand accepted | append to line 86 table |

### The Go Diff Skeleton for `cmd/run.go:18-37`

Before (current state):

```go
var runCmd = &cobra.Command{
    Use:   "run",
    Short: "Run the sandbox container",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := config.Parse(configFile)
        if err != nil {
            return err
        }

        // Agent override via --agent flag
        agentOverride, _ := cmd.Flags().GetString("agent")
        if agentOverride != "" {
            if err := config.ValidateAgent(agentOverride); err != nil {
                return err
            }
            if err := config.ValidateAgentInstalled(agentOverride, cfg.InstalledAgents); err != nil {
                return err
            }
            cfg.DefaultAgent = agentOverride
        }
```

After (target):

```go
var runCmd = &cobra.Command{
    Use:   "run [agent]",
    Short: "Run the sandbox container",
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := config.Parse(configFile)
        if err != nil {
            return err
        }

        agentOverride, err := resolveAgentOverride(args, cmd.Flags().Changed("agent"), mustGetString(cmd, "agent"))
        if err != nil {
            return err
        }
        if agentOverride != "" {
            if err := config.ValidateAgent(agentOverride); err != nil {
                return err
            }
            if err := config.ValidateAgentInstalled(agentOverride, cfg.InstalledAgents); err != nil {
                return err
            }
            cfg.DefaultAgent = agentOverride
        }
```

Where the helper (new, unexported, testable) is:

```go
// resolveAgentOverride picks the agent override from positional args and the
// --agent flag, returning a usageError when both are set.
// Empty return means no override — caller should use cfg.DefaultAgent.
func resolveAgentOverride(args []string, flagChanged bool, flagVal string) (string, error) {
    var positional string
    if len(args) == 1 {
        positional = args[0]
    }
    switch {
    case positional != "" && flagChanged:
        return "", &usageError{err: fmt.Errorf("agent specified both as positional argument ('%s') and via --agent ('%s') — use only one form", positional, flagVal)}
    case positional != "":
        return positional, nil
    case flagChanged:
        return flagVal, nil
    default:
        return "", nil
    }
}
```

Note: `mustGetString` is a one-line helper; inline `flagVal, _ := cmd.Flags().GetString("agent")` is equivalent and preferred — no need for a helper function.

### Why Reuse `usageError` (Not Create `UsageError`)

The epic and architecture notes at `epics.md:1517` and `architecture.md:300` reference `UsageError` and a `cmd/errors.go` file. Both phrasings are stale — they predate story 11.6's addition of the lowercase `usageError` struct in `cmd/root.go:19-25`, which already:

1. Wraps an underlying error via `err error` field
2. Implements `Error() string` and `Unwrap() error`
3. Is already mapped to exit code 2 in `exitCode()` (`cmd/root.go:60-61`)
4. Is already wired to Cobra's flag-parse errors via `SetFlagErrorFunc` (`cmd/root.go:44-46`)
5. Is already covered by a test row in `cmd/root_test.go:51` and a live test `TestUsageError_returnExitCode2` (`cmd/root_test.go:228-237`)

Creating a second `UsageError` would duplicate the hierarchy and violate CLAUDE.md's "Error types defined per owning package". Reuse is correct.

### Why `Flags().Changed()` Instead of Value Comparison

Consider: `default_agent: claude` in config, user runs `asbox run -a claude codex`. With value comparison (`flagVal != ""`), the resolution sees `flagVal == "claude"` AND `positional == "codex"` AND falsely thinks the flag was set by config (it wasn't — the config fills `cfg.DefaultAgent`, not the CLI flag value). With `Flags().Changed("agent")`, the resolution correctly sees `flagChanged == true` because the user explicitly typed `-a claude`, and returns the mutex usage error.

`Changed()` is the only correct way to distinguish "user provided flag" from "flag has default-from-config value". See Cobra docs: https://pkg.go.dev/github.com/spf13/cobra#Command.Flags.

### The Em-Dash Is Real Unicode

AC#3 requires: `error: agent specified both as positional argument ('codex') and via --agent ('claude') — use only one form`

The `—` is U+2014 EM DASH (3 UTF-8 bytes: `0xE2 0x80 0x94`), NOT two hyphens. Retyping it as `--` or `-` will silently break AC#3. Copy the character byte-for-byte from this story file into the Go source. Verify:

```bash
grep -c '—' cmd/run.go   # should print >= 1
```

### Positional Validation Relies on Existing `ValidateAgent` + `ValidateAgentInstalled`

Whichever form resolves (positional, flag, or none → falling through to `DefaultAgent`), the value flows through the existing validation chain:

1. `config.ValidateAgent(name)` — rejects short names not in `{claude, gemini, codex}` with a `*ConfigError` (exit 1). Enforces FR58 (short-name set)
2. `config.ValidateAgentInstalled(name, cfg.InstalledAgents)` — rejects names not present in the image's installed agent set with a `*ConfigError` (exit 1)

Both already exist at `internal/config/parse.go:227-243`. AC#4 ("notanagent" → exit 1 with "not installed" message) is satisfied by (2) with no new code; the positional value just reaches this validation unchanged. No duplication.

### Cobra Arg Validation Error Routing

`cobra.MaximumNArgs(1)` rejects 2+ positional args with a generic error like `accepts at most 1 arg(s), received 3`. By default, Cobra prints this and returns it up the chain. Because `cmd/root.go:44-46` sets `rootCmd.SetFlagErrorFunc` — which only wraps **flag** errors (not **args** errors) — the resulting error may bypass `usageError` wrapping and hit the `default` arm of `exitCode()`, returning 1 instead of 2.

**Verification step (Task 2.3):** Run `asbox run a b c` manually (after implementing) and check the exit code. If it returns 1, either (a) wrap Cobra's Args error in `runCmd.Args`'s custom wrapper:

```go
Args: func(cmd *cobra.Command, args []string) error {
    if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
        return &usageError{err: err}
    }
    return nil
},
```

or (b) leave it at exit 1 and note it in Completion Notes — ACs #1-6 do not explicitly require exit 2 for "too many args" (only for both-forms-set). Prefer (a) for consistency but only if the verification shows the default path returns 1.

### What the Dev MUST NOT Do

- **Do not** create `cmd/errors.go` or an exported `UsageError` type. Reuse the existing `usageError` in `cmd/root.go` (see "Why Reuse `usageError`" above)
- **Do not** add a new row to `TestExitCode_mapping` in `cmd/root_test.go` — the `UsageError` row at line 51 already covers the mapping
- **Do not** change the semantics of `--agent` (long form) — it must continue to work exactly as before. The short alias is additive
- **Do not** rename the `--agent` flag; only add the `a` shorthand
- **Do not** touch `agentCommand()`, `agentInstructionTarget()`, `AgentConfigRegistry`, `internal/config/parse.go`, `embed/entrypoint.sh`, or any other file outside `cmd/run.go` + tests. This is purely a CLI-surface story
- **Do not** use `%q` in the mutex error format string — it emits double quotes; AC#3 requires single quotes. Use explicit `'%s'`
- **Do not** retype the em-dash as `--` or `-`. Paste the Unicode character verbatim
- **Do not** introduce a precedence where positional silently wins over flag (or vice versa). The spec is explicit: "no implicit precedence between the two forms". Both-set → error, period
- **Do not** add positional-arg support to `build`, `init`, or any command other than `run`. The positional agent is a `run`-specific affordance
- **Do not** inject new validation between positional parsing and the existing `ValidateAgent`/`ValidateAgentInstalled` chain. One validation source of truth
- **Do not** change `cmd/run.go:114` where `agentCmd` is computed — `cfg.DefaultAgent` is already mutated before this point by the override logic; no rewiring needed downstream

### Testing Strategy Summary

| Test | Purpose | Location |
|------|---------|----------|
| NEW `TestResolveAgentOverride_tableDriven` | Cover all 4 resolution branches on the pure helper | `cmd/run_test.go` (append) |
| NEW `TestRun_agentShorthand_equivalentToFlag` | `-a` reaches RunE identically to `--agent` | `cmd/run_test.go` |
| NEW `TestRun_agentPositional_equivalentToFlag` | Positional reaches RunE identically to flag | `cmd/run_test.go` |
| NEW `TestRun_agentBothPositionalAndFlag_usageError` | Mutex → `*usageError`, exit 2, exact message | `cmd/run_test.go` |
| NEW `TestRun_agentPositionalNotInstalled_configError` | Unknown agent via positional → `*ConfigError`, exit 1 | `cmd/run_test.go` |
| NEW `TestRun_helpContainsPositionalAndShortFlag` | `--help` output includes `run [agent]`, `-a, --agent` | `cmd/run_test.go` |
| NEW `TestMultiAgent_shortFlagAccepted` | Binary invocation: `-a gemini` reaches validation, exits 1 with "not installed" | `integration/multi_agent_test.go` |
| Existing `TestAgentCommand_*` | Unchanged — agent command shape lock | `cmd/run_test.go:209-290` |
| Existing `TestExitCode_mapping[UsageError]` | Unchanged — `usageError` → exit 2 mapping | `cmd/root_test.go:51` |

Why a pure-function unit test for the resolver plus thinner RunE tests: the resolver has 4 branches and is the only piece of new logic worth testing in isolation. Full RunE tests require parsing real config, which pulls in `config.Parse` validation of `installed_agents` — fine for 3-4 scenarios, but don't table-drive RunE tests for resolution edge cases. Put the table in the pure function's test.

### Pattern to Follow: Existing Run-Scenario Tests

`cmd/run_test.go:31-57` (`TestRun_nonexistentMountSource_returnsConfigError`) is the template:

1. `dir := t.TempDir()`
2. `cfgPath := writeRunConfig(t, dir, ...)`
3. `configFile = cfgPath; t.Cleanup(func() { configFile = old })`
4. `r := newRootCmd(); err := r.run(...)`
5. `errors.As(err, &target)` — assert error type
6. `exitCode(err)` — assert exit code

Copy this structure for the new RunE-level tests.

### Content Hash Impact

Zero. No embed asset changes. Image cache remains valid; existing images do not rebuild on upgrade.

### Error Handling — No New Types

Per CLAUDE.md "Every new error type must be added to `exitCode()`". This story adds **zero new error types**. All failures route through:

- `usageError` (existing) — mutex case, exit 2
- `*config.ConfigError` (existing) — unknown agent via `ValidateAgent`, uninstalled via `ValidateAgentInstalled`, exit 1

No edits to `cmd/root.go:exitCode()` or `cmd/root_test.go:TestExitCode_mapping`.

### Exit Code Impact

| Scenario | Current | After Story |
|----------|---------|-------------|
| `asbox run` (default) | 0 (or build/run err) | 0 (unchanged) |
| `asbox run --agent claude` | 0 | 0 (unchanged) |
| `asbox run -a claude` | n/a (unknown flag, exit 2) | 0 (new) |
| `asbox run claude` | n/a (unknown command, exit 2) | 0 (new) |
| `asbox run claude --agent claude` | n/a | **2** (new, usageError) |
| `asbox run -a claude claude` | n/a | **2** (new, usageError) |
| `asbox run notanagent` | n/a (exit 2) | **1** (ConfigError via ValidateAgentInstalled) |
| `asbox run a b c` (too many args) | n/a (exit 2) | 2 (Cobra MaximumNArgs — verify per Task 2.3) |

### Architecture Compliance Pointers

- **Two execution domains (architecture.md:75):** Command shape stays in Go; container lifecycle stays in bash. This story edits only `cmd/run.go` (Go CLI surface) — no entrypoint changes
- **CLI Agent Override (architecture.md:297-307):** This section is the authoritative spec for the feature. Resolution order, `Flags().Changed()` detection, error string format are all prescribed
- **FR60/FR61 (PRD:483-484):** Both are fully addressed by this story. No follow-up stories needed for this epic
- **CLAUDE.md Error Handling:** `errors.As()` for error comparison — already in use (`cmd/root_test.go`). Exit code 2 for usage errors — already mapped
- **CLAUDE.md Testing:** Table-driven for `resolveAgentOverride`, individual functions for RunE scenarios, `t.TempDir()` for temp dirs, stdlib `testing` — all followed
- **CLAUDE.md Code Organization:** Stay within `cmd/`. Do not add `cmd/errors.go`. Reuse existing `usageError`

### Project Structure Notes

- Changes: `cmd/run.go` (edit), `cmd/run_test.go` (append), `integration/multi_agent_test.go` (append one subtest)
- No new files. No new packages. No new error types. No `go.mod` changes
- No embed asset changes → image hash unchanged → no forced rebuilds for users upgrading
- Follows the Epic 11 scope-discipline pattern established by stories 11.5 and 11.6: one surface, one concern, no incidental refactors

### Previous Story Intelligence (11.6)

From `_bmad-output/implementation-artifacts/11-6-agent-command-injection-hardening.md:199-217`:

- **Scope discipline:** 11.6 touched exactly one embed file and one test file, plus one new test file. 12.1 follows the same pattern — three files total (`cmd/run.go`, `cmd/run_test.go`, `integration/multi_agent_test.go`)
- **Regression-test-as-invariant-lock:** 11.6's `TestAgentCommand_noShellMetacharacters` locked a safety invariant. 12.1's `TestRun_agentBothPositionalAndFlag_usageError` locks the exact error-message string (AC#3) so future refactors can't silently drop the em-dash or change the phrasing
- **Full test suite is the gate:** 11.6 dev ran `gofmt -w`, `go vet ./...`, `go test ./...`. Apply the same gate here
- **No new error types in Epic 11 stories:** 11.6 added zero. 12.1 also adds zero (reusing existing `usageError`). Consistent with Epic 11's "tighten, don't expand" theme and extends naturally into Epic 12

### Git Intelligence (Recent Commits)

Last 5 commits pattern:
- `45f59db docs: refine UX for --fetch operation on bmad repos`
- `60fef54 docs: flush future work into PRD and sprint`
- `d38f080 docs: update future work`
- `0caf2d1 feat: agent command injection hardening via direct exec (story 11-6)`
- `cd122b2 feat: pinned Dockerfile build dependencies for reproducible builds (story 11-5)`

**Commit message convention:** `feat: <short description> (story N-M)` for feature stories, `docs: <description>` for planning doc edits. Use `feat: short -a flag and positional agent argument for run (story 12-1)` or similar when committing.

### CLAUDE.md Compliance

- **Error handling:** `errors.As` used via wrapped `*usageError` — no bare equality, no type switches. Zero new error types
- **Testing:** Stdlib `testing` only (no testify). Table-driven for `resolveAgentOverride`. `t.TempDir()` for file I/O. No `defer` for cleanup in parallel subtests — use `t.Cleanup`
- **Code organization:** Error types per owning package — reused `usageError` (in `cmd/`), reused `*ConfigError` (in `internal/config`). No cross-package error types created
- **Pure functions preferred:** `resolveAgentOverride` is pure (no I/O, no side effects). Makes RunE thinner and the logic directly testable. This is the single most important design choice in the story
- **Agent registry invariant:** Not touched. No new entries needed — the positional/short-flag surface is agent-agnostic by design

### Source Hints for Fast Navigation

| Artifact | Path | Relevant Lines |
|----------|------|:--------------:|
| `--agent` flag registration | `cmd/run.go` | 212 |
| Agent override resolution block | `cmd/run.go` | 27-37 |
| `runCmd` struct (Use, Short, RunE) | `cmd/run.go` | 18-21 |
| `usageError` type (reuse — do not duplicate) | `cmd/root.go` | 19-25 |
| `exitCode()` mapping (UsageError → 2) | `cmd/root.go` | 50-71 |
| `SetFlagErrorFunc` (flag errors wrap to usageError) | `cmd/root.go` | 44-46 |
| `ValidateAgent` / `ValidateAgentInstalled` | `internal/config/parse.go` | 227-243 |
| Existing exit-code test table | `cmd/root_test.go` | 39-62 |
| Existing `--agent` RunE test template | `cmd/run_test.go` | 31-57 |
| Integration test binary-invocation pattern | `integration/multi_agent_test.go` | 70-138 |
| Architecture: CLI Agent Override decision | `_bmad-output/planning-artifacts/architecture.md` | 297-307 |
| Architecture: FR60/FR61 traceability row | `_bmad-output/planning-artifacts/architecture.md` | 714-715 |
| PRD: FR60, FR61 | `_bmad-output/planning-artifacts/prd.md` | 483-484 |
| PRD: `asbox run [agent]` CLI spec | `_bmad-output/planning-artifacts/prd.md` | 300-302 |
| Epic 12 story spec | `_bmad-output/planning-artifacts/epics.md` | 1477-1521 |

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 12 — Story 12.1 (lines 1477-1521)]
- [Source: _bmad-output/planning-artifacts/architecture.md#CLI Agent Override (Short Flag + Positional) (lines 297-307)]
- [Source: _bmad-output/planning-artifacts/architecture.md#Traceability (lines 714-715)]
- [Source: _bmad-output/planning-artifacts/prd.md#Functional Requirements — FR60, FR61 (lines 483-484)]
- [Source: _bmad-output/planning-artifacts/prd.md#CLI Commands — Positional Arguments (lines 300-302)]
- [Source: cmd/run.go — agent override resolution (lines 27-37), flag registration (line 212)]
- [Source: cmd/root.go — usageError type and exit-code mapping (lines 19-25, 50-71)]
- [Source: internal/config/parse.go — ValidateAgent, ValidateAgentInstalled (lines 227-243)]
- [Source: _bmad-output/implementation-artifacts/11-6-agent-command-injection-hardening.md — Epic 11 scope-discipline pattern]
- [Source: CLAUDE.md — error handling, testing, code organization conventions]

## Dev Agent Record

### Agent Model Used

GPT-5 Codex

### Debug Log References

- `gofmt -w cmd/run.go cmd/run_test.go integration/multi_agent_test.go` completed after the CLI and test updates.
- Initial sandboxed `go vet ./...`, `go test ./cmd`, and `go test ./integration -run TestMultiAgent_agentFlagValidation` failed before execution because Go module downloads were blocked by network restrictions.
- After rerunning outside sandbox restrictions with writable Go caches, `go test ./cmd` first exposed missing `sdks.nodejs` fixtures for `gemini`/`codex` configs and Cobra flag-state leakage between tests; updating the fixtures and adding command-state resets resolved both issues.
- Final unrestricted validation passed with `go vet ./...`, `go test ./cmd`, `go test ./integration -run TestMultiAgent_agentFlagValidation`, and `go test ./...` (including the full `integration` package).

### Completion Notes List

- Updated `cmd/run.go` so `asbox run` now accepts `-a` as a short alias for `--agent`, surfaces `run [agent]` in help, and resolves an optional positional agent argument through a shared `resolveAgentOverride()` helper.
- Reused the existing `usageError` type for both mutual-exclusion failures and too-many-args validation, preserving the established exit-code-2 behavior without introducing a new error type or touching `cmd/root.go`.
- Added CLI tests in `cmd/run_test.go` covering shorthand flag parsing, positional parsing, exact dual-input usage errors, default-agent fallback behavior, help text, and arg-count usage errors, while keeping the existing agent-command and mount regression tests green.
- Added a binary-invocation integration subtest in `integration/multi_agent_test.go` to confirm `asbox run -a gemini -f <config>` reaches the same existing "not installed" validation path as the long flag.
- Checked the planned docs touchpoints: the PRD already documented `run [agent]` and `-a`/`--agent`, while `README.md` and `embed/config.yaml` did not contain stale `--agent`-only wording, so no documentation file changes were required.
- Used a supported-but-uninstalled agent (`gemini`) in the AC#4 regression test to exercise the existing `ValidateAgentInstalled` path while preserving the architecture requirement that unsupported names still flow through the existing `ValidateAgent` check.

### File List

- `cmd/run.go` (modified)
- `cmd/run_test.go` (modified)
- `integration/multi_agent_test.go` (modified)
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (modified)
- `_bmad-output/implementation-artifacts/12-1-short-flag-and-positional-agent-argument.md` (modified)

### Change Log

- 2026-04-18: Implemented Story 12.1 by adding `-a` and positional agent overrides to `asbox run`, enforcing exact mutual-exclusion usage errors, extending CLI/help test coverage, and validating the full Go suite including integration tests.

### Review Findings

_Code review 2026-04-18 (parallel: blind + edge-case + acceptance-auditor). All 6 ACs PASS and all 10 MUST-NOT constraints honored. Triage: 1 decision-needed, 4 patch, 4 defer, ~12 dismissed as noise._

- [x] [Review][Decision] Empty-string inputs (`asbox run ""`, `asbox run -a ""`) silently fall through to `cfg.DefaultAgent`. **Resolved 2026-04-18: accept current behavior** — empty values treated as "not set"; no explicit error path introduced.
- [x] [Review][Patch] Remove `mustGetString` helper — spec Dev Notes line 211 explicitly says the helper is unnecessary and inlining `flagVal, _ := cmd.Flags().GetString("agent")` is preferred. **Fixed 2026-04-18**: helper removed; call site inlined.
- [x] [Review][Patch] Tautological equivalence test: `TestRun_agentShorthand_equivalentToFlag` — both forms converged on a mount error that wouldn't distinguish a broken override. **Fixed 2026-04-18**: test now installs only `[claude]` and asserts both `-a gemini` and `--agent gemini` produce a `"gemini" is not installed` `*ConfigError`, proving the override actually drove resolution.
- [x] [Review][Patch] Tautological default-agent test: `TestRun_agentDefault_usesConfigDefaultAgent` — previously compared two paths that were identical by construction. **Fixed 2026-04-18**: test now verifies plain `run` reaches the post-resolution mount-validation stage without emitting a `usageError` (proving no override was applied and `cfg.DefaultAgent` was honored).
- [x] [Review][Patch] Brittle Cobra string assertion in `TestRun_tooManyArgs_usageError` — asserted byte-exact Cobra internal string. **Fixed 2026-04-18**: loosened to `strings.Contains(err.Error(), "at most 1")`; exit-code 2 and `*usageError` type remain the load-bearing assertions.
- [x] [Review][Defer] `resetRunCommandState` does not reset the persistent root `-f`/`file` flag — no current test uses `-f` in-process (tests set `configFile` directly), but a future test using `-f` will leak state into subsequent tests. [cmd/run_test.go:31-61] — deferred, pre-existing gap in the test harness
- [x] [Review][Defer] `resetRunCommandState` mutates pflag's `flag.Changed` struct field directly — works but reaches into library internals not part of pflag's public contract. [cmd/run_test.go:38,46,54,62] — deferred, pragmatic given the package-global `runCmd` constraint
- [x] [Review][Defer] Package-level `rootCmd`/`runCmd` singletons shared across tests — any future `t.Parallel()` on unit tests would race on `SetArgs`/flag state. Pre-existing architectural constraint forcing the `resetRunCommandState` workaround. [cmd/root.go] — deferred, pre-existing
- [x] [Review][Defer] Control-character echoing in `unsupported agent '...'` error — a positional like `$'foo\nbar'` is interpolated raw into the `ConfigError` message, producing multi-line terminal output. Error-path only, not a security issue. [internal/config/parse.go:229] — deferred, pre-existing in ValidateAgent
