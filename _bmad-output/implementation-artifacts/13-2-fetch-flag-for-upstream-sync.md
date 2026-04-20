# Story 13.2: `--fetch` Flag for Host-Side Upstream Sync

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want `asbox run --fetch` to fetch upstream state for every repository I'm about to mount,
so that the sandboxed agent has current remote refs on all branches — including branches it isn't checked out on — without needing host credential access.

## Acceptance Criteria

1. **Given** the developer runs `asbox run --fetch` with `bmad_repos` listing three repositories
   **When** the CLI begins launch orchestration
   **Then** before `docker run` is invoked, the CLI runs `git fetch origin` on each of the three `bmad_repos` paths using the developer's host credentials, and on the project directory mount (the config's primary mount — resolved to the directory containing `.asbox/`) if it is a git repository

2. **Given** the fetch phase is about to begin
   **When** workers dispatch
   **Then** a single informational anchor line prints to stdout *before* any per-repo output: `fetching N repositories (timeout 60s each, 4 concurrent)...` where `N` reflects the deduplicated repo count and the timeout/concurrency values reflect actual runtime config. The anchor line must print even if `N == 1`

3. **Given** a repository has no remote named `origin`
   **When** the fetch phase runs against it
   **Then** the path is skipped with an info-level log (`info: no origin remote in <path>, skipping fetch`) and is counted as skipped (not failed) in the summary

4. **Given** the fetch phase runs
   **When** `--fetch` touches any repository
   **Then** only `refs/remotes/origin/*` and the object database are updated — the working tree, index, `HEAD`, local branches (`refs/heads/*`), stash, and any `.git/rebase-*`, `.git/merge-*`, or `.git/cherry-*` state are left unmodified. This is guaranteed by using `git fetch` (never `pull`, `merge`, `rebase`, `checkout`, or `reset`)

5. **Given** `--fetch` is set and one of the repositories is not a git repository (no `.git` entry)
   **When** the fetch phase runs
   **Then** that path is skipped silently and counted as skipped in the summary — no warning, no error

6. **Given** `--fetch` is set and the same canonical path appears more than once across `bmad_repos` and the primary mount
   **When** the fetch phase runs
   **Then** the path is fetched exactly once (deduplication by resolved absolute path via `filepath.EvalSymlinks`)

7. **Given** `--fetch` is set and a fetch does not complete within the per-repo timeout (default 60s, configurable via `ASBOX_FETCH_TIMEOUT` as a Go `time.Duration` string)
   **When** the timeout fires
   **Then** the in-flight `git` process is killed via context cancellation, the repo is reported as failed with reason `timed out after <duration> (override with ASBOX_FETCH_TIMEOUT)` (the env-var hint MUST appear inline in the user-visible failure message), and the launch continues

8. **Given** `--fetch` is set and one of the repositories fails to fetch (network error, auth failure, timeout)
   **When** the fetch phase runs
   **Then** the error is logged as a warning naming the failed repo and the git stderr, and the launch continues — other repos still get fetched and the sandbox still starts

9. **Given** `--fetch` is set with multiple repositories
   **When** the fetch phase runs
   **Then** fetches run concurrently with a bounded worker pool (default 4) and total wall time is close to the longest single fetch, not the sum

10. **Given** concurrent fetches are running
    **When** one repo emits output
    **Then** its stderr is buffered in memory and flushed atomically in a single block when that repo's fetch completes — output from different repos never interleaves line-by-line

11. **Given** `--fetch` is NOT passed
    **When** the CLI begins launch orchestration
    **Then** no fetch runs — the sandbox starts immediately, preserving offline-use behavior

12. **Given** the `--help` output for `asbox run`
    **When** rendered
    **Then** the `--fetch` flag appears with a condensed two-line description: `Run 'git fetch origin' in all mounted repos before launch, using host credentials. Refs-only — never touches working tree. Non-fatal on failure.`

13. **Given** the fetch phase runs
    **When** completed with all repos succeeding and nothing skipped
    **Then** a single short summary line is printed to stdout: `fetched N/N repositories` (e.g., `fetched 3/3 repositories`) — no `(0 failed, 0 skipped)` parenthetical. The summary line always prints, even when `N == 1`

14. **Given** the fetch phase runs
    **When** all repos succeeded but some were skipped (Z > 0)
    **Then** the summary uses the expanded form naming each **non-zero** skip category in user-facing prose: `fetched X/N repositories (1 not a git repo)` or `fetched X/N repositories (1 no origin remote, 1 not a git repo)`. Zero-valued categories are suppressed. Stdout, informational

15. **Given** the fetch phase runs
    **When** one or more repos failed (Y > 0)
    **Then** the summary line is prefixed with `WARNING:` and printed to stderr: `WARNING: fetched X/N repositories (Y failed) — see warnings above`. If non-zero skip categories are also present, they append inside the same parenthetical using the same user-facing prose (e.g. `WARNING: fetched 1/4 repositories (2 failed, 1 no origin remote) — see warnings above`)

## Tasks / Subtasks

- [x] Task 1: Add `golang.org/x/sync` module dependency (AC: #9)
  - [x] 1.1 Run `go get golang.org/x/sync/errgroup` from the project root — this pulls in the module and updates both `go.mod` and `go.sum`. Do NOT edit `go.mod` by hand
  - [x] 1.2 Verify `go.mod` gained a `require golang.org/x/sync vX.Y.Z` line and `go.sum` gained the matching hashes. No other deps should change
  - [x] 1.3 Run `go mod tidy` at the end of the story (not now — after package code compiles) to drop any unused deps and normalize

- [x] Task 2: Create `internal/gitfetch/` package with `FetchAll()` core (AC: #1, #3, #4, #5, #6, #7, #8, #9, #10)
  - [x] 2.1 Create new file `internal/gitfetch/fetch.go` with package clause `package gitfetch`
  - [x] 2.2 Define the exported types:
    ```go
    type FetchStatus string

    const (
        StatusSucceeded       FetchStatus = "succeeded"
        StatusFailed          FetchStatus = "failed"
        StatusSkippedNotGit   FetchStatus = "skipped-notgit"
        StatusSkippedNoOrigin FetchStatus = "skipped-noorigin"
    )

    type FetchOptions struct {
        Timeout     time.Duration // per-repo timeout; zero means use default
        Concurrency int           // worker-pool limit; zero means use default (4)
    }

    type FetchResult struct {
        Path   string      // canonical absolute path (post-EvalSymlinks)
        Status FetchStatus
        Err    error       // non-nil only when Status == StatusFailed
        Stderr string      // captured git stderr, flushed atomically on completion
    }

    type FetchSummary struct {
        Results              []FetchResult
        Total                int // len(Results) — number of unique paths the function saw
        Succeeded            int
        Failed               int
        SkippedNotGit        int
        SkippedNoOrigin      int
    }
    ```
  - [x] 2.3 Export package-level defaults: `const DefaultTimeout = 60 * time.Second` and `const DefaultConcurrency = 4`. Do NOT expose these as flag defaults in `cmd/run.go` — the package should be self-contained and callable with a zero-valued `FetchOptions` falling back to defaults
  - [x] 2.4 Implement the signature `func FetchAll(ctx context.Context, paths []string, opts FetchOptions) FetchSummary`. Note: **no error return** — per-repo failures live in `FetchResult.Err`, and aggregate outcomes live in `FetchSummary`. The caller uses `summary.Failed > 0` to decide warning treatment. This matches the "non-fatal" contract in AC#8
  - [x] 2.5 Dedup step (AC: #6): before dispatch, iterate `paths`, call `filepath.EvalSymlinks` on each, skip empties, build `seen := map[string]struct{}{}`, collect unique canonical paths into a local slice. Paths that `EvalSymlinks` returns `os.IsNotExist` for are **treated as `StatusSkippedNotGit`** (not a git repo because the path itself doesn't exist) — add a `FetchResult` with the original input path. Other `EvalSymlinks` errors are treated as `StatusFailed` with the error in `.Err`
  - [x] 2.6 Git-repo detection (AC: #5): for each unique path, check `filepath.Join(path, ".git")` with `os.Stat`. If it doesn't exist OR stat fails with `IsNotExist`, append `FetchResult{Path: path, Status: StatusSkippedNotGit}` and continue. A `.git` **file** (worktree marker) counts as a valid repo — do NOT require `info.IsDir()`. Test this with a manually-created `.git` regular file
  - [x] 2.7 Origin-remote detection (AC: #3): run `git -C <path> remote get-url origin` via `exec.CommandContext` with a short context (1s timeout is enough; inherit from outer ctx). Non-zero exit or the stderr containing "No such remote" ⇒ append `FetchResult{Path: path, Status: StatusSkippedNoOrigin}` and print the info line `info: no origin remote in <path>, skipping fetch` to the provided stdout writer (see Task 3.3 for writer plumbing)
  - [x] 2.8 Concurrency (AC: #9, #10): use `golang.org/x/sync/errgroup` — `g, gctx := errgroup.WithContext(ctx); g.SetLimit(concurrency)`. Each worker produces a `FetchResult` into a slice of preallocated indices (use `results := make([]FetchResult, N)` with index-based writes — this avoids mutex overhead and preserves input order for assertions). The `errgroup.Go` closure must not return a non-nil error (that would cancel siblings); the group is only used for its bounded-parallelism semantics, so every closure returns `nil` regardless of fetch outcome
  - [x] 2.9 Per-repo fetch execution (AC: #4, #7, #8, #10): inside each closure:
    - Build `perRepoCtx, cancel := context.WithTimeout(gctx, effectiveTimeout); defer cancel()`
    - `cmd := exec.CommandContext(perRepoCtx, "git", "fetch", "origin"); cmd.Dir = path; var stderr bytes.Buffer; cmd.Stderr = &stderr`
    - `err := cmd.Run()` — capture `stderr.String()` into `FetchResult.Stderr`
    - If `err == nil` ⇒ `StatusSucceeded`
    - Else if `errors.Is(perRepoCtx.Err(), context.DeadlineExceeded)` ⇒ `StatusFailed` with `Err: fmt.Errorf("timed out after %s (override with ASBOX_FETCH_TIMEOUT)", effectiveTimeout)` — the inline env-var hint is LOAD-BEARING per AC#7, do not move it to a separate sentence
    - Else ⇒ `StatusFailed` with `Err: err` (which will be an `*exec.ExitError` with the non-zero exit code)
    - **CRITICAL:** `git fetch` must NEVER be replaced with `git fetch --all`, `git pull`, `git merge`, `git rebase`, `git checkout`, or `git reset`. The argv is literally `["fetch", "origin"]` — do not add flags, do not add `--tags`, do not add `--prune`. AC#4 forbids working-tree mutation. The test suite must include an invariant-lock test (Task 5.9)
  - [x] 2.10 Aggregate: after `g.Wait()` returns, walk `results`, tally counters into `FetchSummary`, return. Do not filter out any category — the caller needs the full list for per-repo warnings
  - [x] 2.11 Where stderr is printed (AC: #10): the package itself does NOT print anything from the closures — it returns results. The `cmd/run.go` caller is responsible for flushing non-succeeded results' `Stderr` as warnings after `FetchAll` returns (Task 3.7). This keeps the package pure (no I/O writer plumbing needed in closures) and satisfies "output flushed atomically in a single block when that repo's fetch completes" because no partial writes ever happen during the fetch

- [x] Task 3: Wire `--fetch` flag + orchestration in `cmd/run.go` (AC: #1, #2, #11, #12, #13, #14, #15)
  - [x] 3.1 Register the flag in the `init()` at `cmd/run.go:238-242`: `runCmd.Flags().Bool("fetch", false, "Run 'git fetch origin' in all mounted repos before launch, using host credentials. Refs-only — never touches working tree. Non-fatal on failure.")`. The help-text wording is EXACT per AC#12 — do not rephrase. The em-dash is `—` (U+2014, 3 UTF-8 bytes), not `--` or `-`
  - [x] 3.2 Read the flag in `RunE` — place the fetch call AFTER mount assembly (line 77 onwards) and AFTER the bmad_repos instruction file setup (lines 84-101), BEFORE the `cfg.AutoIsolateDeps` block at line 104. Orchestration order per architecture.md:337: `build-if-needed → mount assembly → fetch (if --fetch) → secret validation → docker run`. In the current code, `buildEnvVars` (which validates secrets) runs at line 55, so place the fetch call after mount+bmad_repos assembly but before the `docker run` call at line 153. Concretely: place it after the `// Auto-isolate platform dependencies` block (after line 122)
  - [x] 3.3 Build the path list (AC: #1): the input to `gitfetch.FetchAll` is the project directory (parent of `.asbox/`) plus every entry in `cfg.BmadRepos`. Derive the project directory with `filepath.Dir(filepath.Dir(configFile))` — `configFile` is the package-level string defined in `cmd/root.go:17` resolved relative to cwd, but the canonical location is the parent of the config file's parent. Because `configFile` can be a relative path (default `.asbox/config.yaml`), call `filepath.Abs(configFile)` first, then `filepath.Dir` twice. Resist the temptation to extract this into a `config.ProjectDir()` helper — the single caller is here
  - [x] 3.4 Assemble the ordered path slice: `paths := append([]string{projectDir}, cfg.BmadRepos...)`. `cfg.BmadRepos` paths are already resolved to absolute paths by `config.Parse` (`internal/config/parse.go:204-207`), so no additional resolution is needed here. The dedup step inside `FetchAll` will collapse the project directory when it equals a `bmad_repos` entry
  - [x] 3.5 Read timeout from env (AC: #7): resolve `effectiveTimeout` — if `os.Getenv("ASBOX_FETCH_TIMEOUT") != ""`, call `time.ParseDuration`. On parse error, return a `&config.ConfigError{Msg: fmt.Sprintf("invalid ASBOX_FETCH_TIMEOUT value %q: %s. Use a Go duration string like 60s, 2m, 500ms", v, err)}` — this is a user-facing config mistake, not a fetch failure. On success, pass into `FetchOptions{Timeout: d, Concurrency: 4}`. If the env var is unset, pass a zero-valued `FetchOptions{}` and let the package default (60s / 4) apply
  - [x] 3.6 Print anchor line BEFORE calling `FetchAll` (AC: #2). After dedup — `FetchAll` does dedup internally, so expose a tiny helper in the gitfetch package `DedupPaths(paths []string) []string` that returns the canonical unique-path slice. `cmd/run.go` calls `DedupPaths` once, uses its length `N` for the anchor line, and then passes the already-deduped list into `FetchAll` (which must remain idempotent under dedup — double-dedup is cheap). Print `fmt.Fprintf(cmd.OutOrStdout(), "fetching %d repositories (timeout %s each, %d concurrent)...\n", N, effectiveTimeout, effectiveConcurrency)`. Use the *effective* timeout (either env override or default), not the hardcoded 60s, so the anchor line surfaces the actual value in use per AC#2
  - [x] 3.7 Call `summary := gitfetch.FetchAll(cmd.Context(), paths, opts)`. After it returns, iterate `summary.Results` — for each `StatusFailed` result, print to stderr: `fmt.Fprintf(cmd.ErrOrStderr(), "warning: fetch failed for %s: %s\n", r.Path, firstLineOfStderrOrErr(r))`. The `firstLineOfStderrOrErr` helper prefers `r.Stderr` (trimmed, first line only to keep warnings tight) and falls back to `r.Err.Error()` when stderr is empty. For `StatusSkippedNoOrigin`, print the info line `fmt.Fprintf(cmd.OutOrStdout(), "info: no origin remote in %s, skipping fetch\n", r.Path)` (AC#3). `StatusSkippedNotGit` prints nothing per-repo per AC#5
  - [x] 3.8 Print the summary line (AC: #13, #14, #15): inline a small helper `formatFetchSummary(s gitfetch.FetchSummary) (line string, isWarning bool)` co-located in `cmd/run.go` (keep the formatter in `cmd/` not in `internal/gitfetch/` — it's presentation, not domain logic). Logic:
    - Let `x := s.Succeeded; n := s.Total`
    - Build `parens []string` naming only non-zero categories in user-facing prose:
      - `s.Failed` ⇒ `"<n> failed"` where `n` is the count
      - `s.SkippedNoOrigin` ⇒ `"<n> no origin remote"`
      - `s.SkippedNotGit` ⇒ `"<n> not a git repo"`
    - If `len(parens) == 0` ⇒ return `fmt.Sprintf("fetched %d/%d repositories", x, n), false` (AC#13)
    - If `s.Failed == 0` ⇒ return `fmt.Sprintf("fetched %d/%d repositories (%s)", x, n, strings.Join(parens, ", ")), false` (AC#14, informational, stdout)
    - If `s.Failed > 0` ⇒ return `fmt.Sprintf("WARNING: fetched %d/%d repositories (%s) — see warnings above", x, n, strings.Join(parens, ", ")), true` (AC#15, em-dash U+2014)
    - **Order of parenthetical entries:** failed first, then no-origin, then not-a-git-repo. Tests assert this order — do not sort alphabetically, do not reorder. Failed-first matches the urgency of the partial-failure case
    - The returned `isWarning` flag picks the writer: `false → cmd.OutOrStdout()`, `true → cmd.ErrOrStderr()`
  - [x] 3.9 The summary line is ALWAYS printed — even for `N == 1`, even for the all-succeed case (AC#13). Do not gate the print on any condition besides "fetch phase ran" (i.e., `--fetch` was set)
  - [x] 3.10 AC#11 (default path, no `--fetch`): the entire fetch block in `cmd/run.go` runs only when `cmd.Flags().GetBool("fetch")` returns `true`. When unset, no `FetchAll` call, no anchor line, no summary line — the sandbox starts immediately. Do NOT print any "fetch not requested" line. Silence is the spec
  - [x] 3.11 Context: `FetchAll` takes a `context.Context`. Cobra's `cmd.Context()` returns a background context unless `rootCmd.ExecuteContext` is used, which it isn't. For this story, `cmd.Context()` is acceptable; do not introduce cancellation wiring for SIGINT unless a later story requires it. A Ctrl+C during fetch kills the whole `asbox run` process, which terminates child `git` processes through the process group — acceptable blast radius

- [x] Task 4: Unit tests for `internal/gitfetch/` (AC: #1, #3, #4, #5, #6, #7, #9, #10, #13, #14, #15 indirectly)
  - [x] 4.1 New file `internal/gitfetch/fetch_test.go`, package `gitfetch`
  - [x] 4.2 Helper `newLocalRepo(t *testing.T) (repoPath, originPath string)`: creates a bare repo in `t.TempDir()` (`git init --bare`), creates a non-bare repo, sets its origin to the bare repo, creates and pushes an initial commit. This produces a real-world-ish repo with a working `origin` remote and no network dependency. Use `exec.Command("git", "init", "--bare", bare)` etc. Follow `CLAUDE.md`: `t.TempDir()`, `t.Cleanup`, stdlib `testing` only
  - [x] 4.3 Helper `newRepoNoOrigin(t *testing.T) string`: creates a non-bare repo with a commit but no origin remote configured. Returns the repo path
  - [x] 4.4 Helper `newNonGitDir(t *testing.T) string`: creates a `t.TempDir()` with a file inside and no `.git` directory/file. Returns the path
  - [x] 4.5 Helper `newWorktreeLikeRepo(t *testing.T) string`: creates a directory with a regular `.git` **file** (not directory) pointing at another dir. Used to assert AC#5-adjacent: worktree markers count as a valid repo (the subsequent `git remote get-url origin` call will fail because the gitdir target is empty, which should produce `StatusSkippedNoOrigin`, not `StatusSkippedNotGit`). This test locks the "file OR directory" contract in Task 2.6
  - [x] 4.6 `TestFetchAll_happyPath`: 3 repos with origin, `FetchAll` returns `summary.Total == 3`, `summary.Succeeded == 3`, each `Result.Status == StatusSucceeded`, no `Err`. Covers AC#1, #9
  - [x] 4.7 `TestFetchAll_skippedNoOrigin`: pass `[repoWithOrigin, repoNoOrigin]`, assert one result has `StatusSucceeded`, one has `StatusSkippedNoOrigin`, `summary.SkippedNoOrigin == 1`. Covers AC#3
  - [x] 4.8 `TestFetchAll_skippedNotGit`: pass `[nonGitDir, repoWithOrigin]`, assert `StatusSkippedNotGit` + `StatusSucceeded`, `summary.SkippedNotGit == 1`. Covers AC#5
  - [x] 4.9 `TestFetchAll_deduplication`: pass the same canonical path three times with variations (plain path, trailing slash, via symlink created by `os.Symlink`), assert `summary.Total == 1`. Covers AC#6. Use `filepath.EvalSymlinks` manually to confirm the expected canonical form
  - [x] 4.10 `TestFetchAll_timeout`: create a repo whose `origin` points at a path the `git` process will hang on. The simplest hang is a local filesystem remote pointing to a non-existent `:` URL that triggers SSH lookup — but that's flaky. **Preferred approach:** set `GIT_SSH_COMMAND` via `cmd.Env` in the test's subprocess to a slow shell script that sleeps, but that requires plumbing `Env` through `exec.CommandContext`. **Simpler approach:** pass `FetchOptions{Timeout: 1 * time.Millisecond}` with any remote — even a local bare remote — most fetches take >1ms, so the deadline fires. Assert `Result.Status == StatusFailed` AND `strings.Contains(Result.Err.Error(), "timed out after 1ms (override with ASBOX_FETCH_TIMEOUT)")`. The `override with ASBOX_FETCH_TIMEOUT` substring is load-bearing per AC#7 — the test ASSERTS this verbatim. Covers AC#7
  - [x] 4.11 `TestFetchAll_concurrencyLimit`: create 8 repos, record the start time of each worker by writing to a channel at fetch-start, assert at most 4 workers are in flight at any moment. Implementation: inject a fetch-hook via an unexported `fetchFn func(...)` package-level variable that tests can swap out — or simpler, rely on wall-clock: if 8 repos each take ~100ms (fetching against a local bare remote), sequential = 800ms, parallel@4 = ~200ms. Assert total time `< 500ms` (loose but directional). Skip this test under `-short`. Covers AC#9
  - [x] 4.12 `TestFetchAll_stderrBufferingNoInterleave`: this is hard to assert directly without making the package accept an `io.Writer`. Task 2.11 decided the package is pure (no writer). The interleave guarantee therefore lives in `cmd/run.go`'s sequential per-result `Fprintf` loop (Task 3.7), which is trivially non-interleaving. **Skip** a dedicated test for this AC — instead, add a comment in `fetch.go` on the `FetchResult.Stderr` field: `// Stderr captures the full fetch stderr. Per AC#10, callers flush this atomically in a single write after FetchAll returns.` Covers AC#10
  - [x] 4.13 `TestFetchAll_refsOnlyNoWorkingTreeMutation`: the invariant-lock test (Task 2.9 "CRITICAL"). Before `FetchAll`, create a file `dirty.txt` in the repo's working tree with `git add` but no commit. After `FetchAll`, assert:
    - `dirty.txt` still exists in the working tree and has the same content
    - `git status --porcelain` output is identical before and after
    - `HEAD` points at the same commit SHA before and after
    - The local branch ref (e.g., `refs/heads/main`) points at the same commit before and after
    - `refs/remotes/origin/main` DOES change (i.e., the fetch actually happened) — push a new commit to the bare remote before calling `FetchAll` so there's something to fetch
    This test fails LOUDLY if someone mistakenly changes the argv from `["fetch", "origin"]` to anything else. AC#4 compliance
  - [x] 4.14 `TestFetchAll_failureNonFatal`: create one good repo and one "broken" repo (corrupt `.git/HEAD`, or an `origin` URL that fails fast like `ssh://does-not-exist/`), assert that `FetchAll` still returns normally, `summary.Total == 2`, one `StatusSucceeded`, one `StatusFailed` with `Err` non-nil and `Stderr` non-empty. Covers AC#8
  - [x] 4.15 Do NOT use `testing.Short()` to skip the git-calling tests unless they genuinely require network. Local bare-repo fetches run in milliseconds with no network — they belong in the default `go test ./internal/gitfetch/` run. `CLAUDE.md`: "No testify — stdlib testing package only" — use `errors.Is`/`errors.As` for error assertions, no assertion libraries

- [x] Task 5: Unit tests for `cmd/run.go` flag wiring + summary formatter (AC: #2, #11, #12, #13, #14, #15)
  - [x] 5.1 Add tests in `cmd/run_test.go` following the existing `newRootCmd` + `r.run(...)` pattern (see `cmd/run_test.go:63-89`). Use `resetRunCommandState(t)` between subtests because this story adds a new persistent flag to the package-global `runCmd`
  - [x] 5.2 Extend `resetRunCommandState` at `cmd/run_test.go:31-61` to also reset the `fetch` flag: after the `no-cache` reset block, add `if err := runCmd.Flags().Set("fetch", "false"); err != nil { t.Fatalf(...) }` and clear the `Changed` field the same way. Do NOT add a new reset helper — extend the existing one. Other test functions already call `resetRunCommandState`, so they'll automatically be fetch-flag-safe after this change
  - [x] 5.3 `TestRun_fetchFlag_default_isFalse` — `runCmd.Flags().Lookup("fetch")` exists, `DefValue == "false"`, `Usage` is EXACTLY the AC#12 string. Use `strings.Contains` for the usage check if necessary, but prefer exact equality — the test locks AC#12's wording. Covers AC#12
  - [x] 5.4 `TestRun_helpContainsFetchFlag` — run `"run", "--help"`, assert `output` contains `--fetch`, the exact AC#12 description, `Refs-only`, and `Non-fatal on failure`. Follows the 12.1 pattern at `cmd/run_test.go:3?-?? (TestRun_helpContainsPositionalAndShortFlag` — use that test as a template)
  - [x] 5.5 `TestRun_fetchNotPassed_noFetchPhase` — a config with zero `bmad_repos` and a non-existent project directory (so `FetchAll` would produce a `StatusSkippedNotGit` if called). Assert that running `asbox run` WITHOUT `--fetch` produces output that does NOT contain `fetching` or `fetched`. This is the AC#11 lock. Expect the test to fail on docker-build (no docker in CI Go-only) — assert that type of error, not success. Follow the error-classification pattern from `TestRun_nonexistentMountSource_returnsConfigError` at `cmd/run_test.go:63-89`
  - [x] 5.6 `TestFormatFetchSummary_tableDriven` — pure function table test in `cmd/run_test.go` covering:
    | Case | Total | Succeeded | Failed | SkipNoOrigin | SkipNotGit | Expected | isWarning |
    |---|---|---|---|---|---|---|---|
    | All succeed | 3 | 3 | 0 | 0 | 0 | `fetched 3/3 repositories` | false |
    | Single repo succeed | 1 | 1 | 0 | 0 | 0 | `fetched 1/1 repositories` | false |
    | One skip not-git | 3 | 2 | 0 | 0 | 1 | `fetched 2/3 repositories (1 not a git repo)` | false |
    | One skip no-origin | 3 | 2 | 0 | 1 | 0 | `fetched 2/3 repositories (1 no origin remote)` | false |
    | Mixed skips | 3 | 1 | 0 | 1 | 1 | `fetched 1/3 repositories (1 no origin remote, 1 not a git repo)` | false |
    | Single failure | 3 | 2 | 1 | 0 | 0 | `WARNING: fetched 2/3 repositories (1 failed) — see warnings above` | true |
    | Failure + skips | 4 | 1 | 2 | 1 | 0 | `WARNING: fetched 1/4 repositories (2 failed, 1 no origin remote) — see warnings above` | true |
    | Failure + both skips | 5 | 1 | 2 | 1 | 1 | `WARNING: fetched 1/5 repositories (2 failed, 1 no origin remote, 1 not a git repo) — see warnings above` | true |
    - Assert the em-dash is U+2014 in the expected strings — copy-paste the character byte-for-byte (`len("—") == 3`). Retyping as `--` silently breaks AC#15
    - Assert parenthetical ORDER is failed → no-origin → not-a-git-repo (never alphabetical)
    - Covers AC#13, #14, #15
  - [x] 5.7 `TestRun_fetchWithBadTimeoutEnv_returnsConfigError` — set `t.Setenv("ASBOX_FETCH_TIMEOUT", "notaduration")`, run `"run", "--fetch"`, assert `*config.ConfigError` and exit code 1. Message contains `ASBOX_FETCH_TIMEOUT` and `Use a Go duration string`
  - [x] 5.8 Do NOT add fetch-related entries to `cmd/root_test.go:TestExitCode_mapping` — this story adds zero new error types. All fetch path errors reuse `*config.ConfigError` (timeout env parse error). Per `CLAUDE.md`: "Every new error type must be added to `exitCode()`" — we add zero, so no changes to `exitCode()` or its test table

- [x] Task 6: Integration test in `integration/bmad_repos_test.go` (or new `integration/fetch_test.go`) (AC: #1, #11, #13)
  - [x] 6.1 Add a new test file `integration/fetch_test.go` — keep `bmad_repos_test.go` focused on mount assembly. The fetch feature is conceptually a sibling of bmad_repos, not a subset
  - [x] 6.2 Follow the binary-invocation pattern from `integration/bmad_repos_test.go:12-53`: `exec.Command("go", "build", "-o", binPath, ".")` → write config → `exec.Command(binPath, "run", "--fetch", "-f", configPath)` → inspect combined output
  - [x] 6.3 `TestFetch_prePhaseAnchorAndSummary_whenFetchSet`: create 2 temp local repos with bare-remote origins (same pattern as `internal/gitfetch/fetch_test.go` helper, but inline in the integration test since test packages can't import each other's test helpers). Configure `bmad_repos` with both. Run with `--fetch`. Assert output contains:
    - `fetching 2 repositories (timeout 60s each, 4 concurrent)...` (AC#2)
    - `fetched 2/2 repositories` (AC#13)
    The test will still exit non-zero because `docker run` isn't mocked — ignore that. Inspect `combinedOutput` string
  - [x] 6.4 `TestFetch_noFetchFlag_noFetchPhaseOutput`: same setup but omit `--fetch`. Assert output does NOT contain `fetching` or `fetched`. Covers AC#11
  - [x] 6.5 Do NOT write an integration test that asserts `refs/remotes/origin/*` was actually updated inside a launched sandbox — that requires booting docker, which the unit test suite already covers via the pure `FetchAll` test (Task 4.13). The integration test here is only verifying the CLI wiring reaches `FetchAll`
  - [x] 6.6 A `testing.Short()` skip is appropriate only for slow integration tests that boot containers; these binary-invocation tests do not boot docker and run in <1s. Keep them in the default run unless they flake

- [x] Task 7: Docs + changelog (AC: #12)
  - [x] 7.1 Check `README.md` for an asbox-run usage section. If it exists and mentions other flags, add one line for `--fetch` using the same condensed wording as AC#12. If no such section exists, do NOT create one — scope discipline
  - [x] 7.2 Do NOT add a separate docs page for `--fetch`. The `--help` text + AC#12 are the canonical descriptions; the architecture.md section at lines 309-326 is the design reference. One source of truth for each
  - [x] 7.3 Do NOT update `embed/config.yaml` starter — `--fetch` is a CLI flag, not a config field, so the starter doesn't need a new entry

- [x] Task 8: Format, vet, and full test suite (AC: all)
  - [x] 8.1 `gofmt -w cmd/run.go cmd/run_test.go internal/gitfetch/fetch.go internal/gitfetch/fetch_test.go integration/fetch_test.go`
  - [x] 8.2 `go mod tidy` — clean up any unused deps from Task 1
  - [x] 8.3 `go vet ./...` — expect clean
  - [x] 8.4 `go test ./internal/gitfetch/` — expect all unit tests green
  - [x] 8.5 `go test ./cmd/` — expect existing tests (including pre-existing 12.1 tests) and new fetch tests green. If any existing test fails after `resetRunCommandState` extension, fix the reset helper, not the test assertions
  - [x] 8.6 `go test ./...` — full suite green. If an `integration` test requires docker and docker is unavailable, it should skip cleanly via the existing patterns
  - [x] 8.7 `go test ./integration -run TestFetch` — binary-invocation path, expect green

## Dev Notes

### The Exact Change Set

| File | Change | Location |
|------|--------|----------|
| `go.mod` / `go.sum` | Add `golang.org/x/sync` via `go get` | top of file |
| `internal/gitfetch/fetch.go` | **NEW** — `FetchAll`, types, `DedupPaths` helper | entire file |
| `internal/gitfetch/fetch_test.go` | **NEW** — unit tests with local bare-repo fixtures | entire file |
| `cmd/run.go` | Add `--fetch` flag registration | `init()` at line 238-242 |
| `cmd/run.go` | Insert fetch orchestration block after auto_isolate_deps | after line 122 |
| `cmd/run.go` | Add `formatFetchSummary` helper + `firstLineOfStderrOrErr` | new funcs below `buildEnvVars` |
| `cmd/run_test.go` | Extend `resetRunCommandState` for `fetch` flag | lines 31-61 |
| `cmd/run_test.go` | Append fetch tests | append |
| `integration/fetch_test.go` | **NEW** — binary invocation tests | entire file |
| `README.md` | Optional one-line mention of `--fetch` if a flags section exists | TBD |

**Explicitly NOT touched:** `cmd/root.go`, `internal/config/`, `internal/mount/`, `embed/`. This is a pure host-side orchestration addition with no config-schema change, no agent-instruction change, no Dockerfile/image change.

### Package Layout Rationale

The architecture document (lines 551-555) pre-decides the package name `internal/gitfetch/` and anticipates a sibling file `dirty.go` for Story 13.3 (dirty-tree warning). This story ships only `fetch.go` (+ test). Do NOT pre-create `dirty.go` or `DetectDirty` — story 13.3 will add them. Scope discipline matches the Epic 11 / 12 pattern: one story, one surface.

The package is `internal/` because it has no reason to be imported from outside the asbox module. If later reuse is desired (e.g., a standalone tool), promote then — not preemptively.

### Why `FetchAll` Returns No Error

Per AC#8, per-repository failures are non-fatal — the launch continues. If `FetchAll` returned `error`, every call site would need a `return err` that contradicts the non-fatal contract. Instead, all per-repo outcomes live in `FetchSummary.Results[i].Err`, and the aggregate `summary.Failed > 0` tells the caller whether to print with `WARNING:`. This is the same shape `os.ReadDir` uses — one call returns many per-entry results, and the caller decides per-entry severity.

### Why `cmd/run.go` Owns the Presentation Layer

The `internal/gitfetch/` package is a pure data processor: given paths, return results. It does not print anchor lines, per-repo warnings, or summary lines. The rationale:

1. **Testability:** Pure functions without `io.Writer` dependencies test with `reflect.DeepEqual` on `FetchSummary`, not with regex over captured output.
2. **Separation:** The CLI presentation (anchor line wording, summary format, writer choice stdout-vs-stderr) is coupled to the `asbox run` UX. The same `FetchAll` could feed a JSON log in a future `asbox run --json` story without re-implementing the data layer.
3. **CLAUDE.md compliance:** "Pure functions preferred (no I/O, no side effects) for testability."

The one exception is the info-level `info: no origin remote in <path>, skipping fetch` log. Task 3.7 routes this through the caller's `cmd.OutOrStdout()`. The package does not own this write.

### Path Resolution — Do Not Reinvent

`cfg.BmadRepos` entries are already resolved to absolute paths by `config.Parse` at `internal/config/parse.go:204-207`:

```go
for i := range cfg.BmadRepos {
    cfg.BmadRepos[i] = resolvePath(configDir, cfg.BmadRepos[i])
}
```

The project directory (parent of `.asbox/`) is derived in `Parse` as `parentDir := filepath.Dir(configDir)` at line 191, but this value is NOT stored on the Config struct — it's only used to compute `ProjectName`. For this story, re-derive the project directory from `configFile` at the top of the fetch orchestration block:

```go
absCfgPath, err := filepath.Abs(configFile)
if err != nil { return err }
projectDir := filepath.Dir(filepath.Dir(absCfgPath))
```

Do NOT add a `ProjectDir` field to `Config` — one caller does not justify widening the struct. If a second caller appears, promote then.

### Deduplication Edge Cases

`filepath.EvalSymlinks` on a non-existent path returns an `*os.PathError` wrapping `os.ErrNotExist`. Task 2.5 treats this as `StatusSkippedNotGit` — correct: a nonexistent path is effectively not a git repo. But `EvalSymlinks` on a path that exists but is inaccessible (permission error) returns a different error — treat these as `StatusFailed` so the user sees the permission error instead of a silent skip. Use `errors.Is(err, fs.ErrNotExist)` for the not-a-repo branch, fall through to `StatusFailed` otherwise.

### Git Command Safety

Per `CLAUDE.md` "Agent Registry" and Story 11.6 (agent command injection hardening), `os/exec` invocations in this project use argv form, never a shell-joined string. This story's fetch command is literally `exec.CommandContext(ctx, "git", "fetch", "origin")`. User-controlled values (repo paths) flow via `cmd.Dir`, not via the argv. There is no shell involved. `git`'s remote URL comes from the repo's own `.git/config` — asbox never constructs a URL. This is safe.

The remote-detection helper `git -C <path> remote get-url origin` also uses argv form: `exec.CommandContext(ctx, "git", "-C", path, "remote", "get-url", "origin")`. Do not switch either to `exec.Command("sh", "-c", "...")` — this would reintroduce shell-injection risk.

### Error Handling — Zero New Types

Per `CLAUDE.md` "Every new error type must be added to `exitCode()` in `cmd/root.go` and its test table in `cmd/root_test.go`." This story adds **zero new error types**:

- `FetchResult.Err` holds raw `error` values — `*exec.ExitError`, `context.DeadlineExceeded`-wrapped, or `fmt.Errorf` for the timeout message. None are returned to `cmd/run.go` as terminal errors because the whole phase is non-fatal.
- The only path that returns a terminal error from the fetch block is AC#7-adjacent: `ASBOX_FETCH_TIMEOUT` unparseable ⇒ `*config.ConfigError` (existing type, exit code 1). This preserves the invariant.
- No `cmd/root.go` / `cmd/root_test.go` edits.

### Timeout Semantics Under Concurrency

Each worker derives `perRepoCtx` from the shared `gctx` via `context.WithTimeout(gctx, effectiveTimeout)`. If one repo hangs for 60s while others complete in 500ms, the fast ones don't wait — each worker calls `cancel()` on its own `perRepoCtx` via `defer`, releases its pool slot, and `errgroup` dispatches the next unit. Total wall time = longest single fetch + dispatch overhead, as AC#9 requires.

Do NOT use a single shared timeout context for all workers — that would make the timer start from the first dispatch, not from each worker's start, producing confusing behavior where workers queued behind busy slots get less than 60s.

### Output Ordering Under Concurrency

Because worker index-based writes into `results` preserve input order (Task 2.8), the caller's loop over `summary.Results` prints per-repo warnings in the order the user specified `bmad_repos` in config. This is valuable for diagnosing "which repo failed" — the user's mental model tracks their config file, not the order fetches completed. Do NOT replace the indexed writes with `append` on a shared slice under a mutex — the indexed form is both faster (no mutex) and deterministic.

### Why No `--fetch-timeout` Flag

The design uses an environment variable (`ASBOX_FETCH_TIMEOUT`) rather than a CLI flag. Rationale (from epics.md line 1598 and architecture.md line 315):

1. **Sensible defaults:** 60s is correct for ~99% of users. Flags should surface common tuning knobs; env vars should cover rare ones.
2. **Inheritance:** Env vars propagate across `asbox run` invocations in CI scripts without every invocation repeating the flag.
3. **Discoverability:** The failure message includes the env-var name inline (`override with ASBOX_FETCH_TIMEOUT`), so users don't need to `--help` to find it.
4. **Flag hygiene:** `--help` is a scannable inventory. Adding per-flag-of-a-single-flag timeouts crowds sibling flags.

Do not introduce a `--fetch-timeout` flag in this story. If user feedback later shows env-var-only is insufficient, it can be added in a follow-up.

### Why No `--fetch-concurrency` Flag

Same rationale as above, except there's currently no env-var override either. The hardcoded 4 is a sensible default for typical `bmad_repos` sizes (2-5 repos). If a user needs to tune concurrency, they can set `GOMAXPROCS`-level parallelism at the OS, or wait for a future story. Do not add a second env var in this story.

### Integration With Existing Launch Flow

Current launch flow in `cmd/run.go:RunE` (lines 28-154):

```
Parse config → resolve agent override → AssembleMounts → buildEnvVars (validates secrets)
 → AssembleHostAgentConfig → AssembleBmadRepos → AutoIsolateDeps scan
 → ensureBuild → docker.RunContainer
```

Fetch insertion point: between `AutoIsolateDeps` block (ends line 122) and `agentCommand` resolution (line 124). Why there and not earlier?

- **After `buildEnvVars`:** secrets are validated first. If a secret is missing, fail before any network I/O.
- **After `AssembleBmadRepos`:** path resolution and basename collision detection complete before fetch. If bmad_repos is misconfigured, fail fast.
- **Before `agentCommand` / `ensureBuild`:** we want the fetch to happen before any docker work, so if fetch is slow the user sees the anchor line, not a silent pause during image checks.
- **Before `docker run`:** the whole point of `--fetch` is to update refs before the agent starts.

Architecture.md:337 documents this exact order. Follow it.

### `cmd.Context()` Caveat

Cobra's `cmd.Context()` returns `context.Background()` unless `rootCmd.ExecuteContext(ctx)` was called, which `cmd/root.go:Execute` does not do. For this story, `context.Background()` is acceptable — Ctrl+C sends SIGINT to the whole process, which kills child `git` processes via the process group. Do not introduce `rootCmd.ExecuteContext` in this story. If a future story wires signal-aware cancellation, it'll happen at the root, not here.

### Em-Dash Unicode

AC#15 and the `--help` text in AC#12 use `—` (EM DASH, U+2014, 3 UTF-8 bytes `0xE2 0x80 0x94`). Retyping as `--` or `-` silently breaks assertions. Copy the character byte-for-byte from this file. Verify:

```bash
grep -c '—' cmd/run.go internal/gitfetch/fetch.go
```

Story 12.1 locked a similar em-dash invariant; the convention is load-bearing.

### Testing Strategy Summary

| Test | Purpose | Location |
|------|---------|----------|
| NEW `TestFetchAll_happyPath` | Basic multi-repo fetch | `internal/gitfetch/fetch_test.go` |
| NEW `TestFetchAll_skippedNoOrigin` | `git remote get-url` failure path | `internal/gitfetch/fetch_test.go` |
| NEW `TestFetchAll_skippedNotGit` | No `.git` entry ⇒ silent skip | `internal/gitfetch/fetch_test.go` |
| NEW `TestFetchAll_deduplication` | `EvalSymlinks` canonical collapsing | `internal/gitfetch/fetch_test.go` |
| NEW `TestFetchAll_timeout` | Timeout message verbatim hint | `internal/gitfetch/fetch_test.go` |
| NEW `TestFetchAll_concurrencyLimit` | Wall-clock bound under parallelism | `internal/gitfetch/fetch_test.go` |
| NEW `TestFetchAll_refsOnlyNoWorkingTreeMutation` | Invariant lock: fetch never mutates working tree | `internal/gitfetch/fetch_test.go` |
| NEW `TestFetchAll_failureNonFatal` | Broken repo does not abort phase | `internal/gitfetch/fetch_test.go` |
| NEW `TestRun_fetchFlag_default_isFalse` | Flag exists, default false, help text exact | `cmd/run_test.go` |
| NEW `TestRun_helpContainsFetchFlag` | `--help` renders the flag | `cmd/run_test.go` |
| NEW `TestRun_fetchNotPassed_noFetchPhase` | AC#11 silence lock | `cmd/run_test.go` |
| NEW `TestFormatFetchSummary_tableDriven` | All summary-line shapes incl. em-dash | `cmd/run_test.go` |
| NEW `TestRun_fetchWithBadTimeoutEnv_returnsConfigError` | AC#7 env-var parse error | `cmd/run_test.go` |
| NEW `TestFetch_prePhaseAnchorAndSummary_whenFetchSet` | Binary invocation — anchor + summary | `integration/fetch_test.go` |
| NEW `TestFetch_noFetchFlag_noFetchPhaseOutput` | Binary invocation — AC#11 silence | `integration/fetch_test.go` |
| EXISTING `cmd/run_test.go:TestRun_*` | Unchanged — no behavior change for non-fetch paths | `cmd/run_test.go` |
| EXISTING `cmd/root_test.go:TestExitCode_mapping` | Unchanged — zero new error types | `cmd/root_test.go` |

### Content Hash Impact

Zero. No `embed/` asset changes. Image cache remains valid; existing images do not rebuild on upgrade. Users pull this story's binary and their existing `asbox-<project>:<hash>` images keep working.

### Exit Code Impact

| Scenario | After Story |
|----------|-------------|
| `asbox run` (no `--fetch`) | 0 (unchanged) or existing error code |
| `asbox run --fetch` with all repos succeeding | 0 (sandbox starts normally) |
| `asbox run --fetch` with some repos failing | 0 (launch still succeeds — AC#8) — user sees `WARNING:` summary |
| `asbox run --fetch` with `ASBOX_FETCH_TIMEOUT=bogus` | **1** (new — `*config.ConfigError`) |

No new exit codes introduced. The one new error path (timeout env parse) reuses exit code 1 via the existing `*config.ConfigError` handler.

### Architecture Compliance Pointers

- **Two execution domains (architecture.md:75):** Host-side orchestration only. No `embed/` changes, no entrypoint changes, no image-layer changes
- **Multi-Repo Upstream Fetch (architecture.md:309-326):** Authoritative spec — refs-only, origin-only, per-repo timeout, bounded concurrency, dedup, buffered output, three summary shapes
- **FR65 (PRD:496):** Fully addressed by this story
- **CLAUDE.md Error Handling:** `errors.Is`/`errors.As` — already in use. Zero new error types
- **CLAUDE.md Testing:** Table-driven for pure functions (`formatFetchSummary`), individual tests for CLI scenarios, `t.TempDir()` for all temp dirs, `t.Cleanup` not `defer`, stdlib `testing` only, no testify
- **CLAUDE.md Code Organization:** New error types per owning package — zero added here; package boundary at `internal/gitfetch/`; pure functions preferred (`FetchAll`, `DedupPaths`, `formatFetchSummary` all pure)

### Project Structure Notes

- Changes: `cmd/run.go` (edit + 2 new helpers), `cmd/run_test.go` (extend + append tests), `internal/gitfetch/` (new package, 2 files), `integration/fetch_test.go` (new), `go.mod`/`go.sum` (one new dep)
- No new error types. No new exit codes. No `embed/` changes. No `cmd/root.go` changes
- Follows the Epic 11-12 scope-discipline pattern: new capability surfaces cleanly with a new package, existing structure untouched

### Previous Story Intelligence (12.1)

From `_bmad-output/implementation-artifacts/12-1-short-flag-and-positional-agent-argument.md`:

- **Scope discipline pattern:** 12.1 touched 3 files (`cmd/run.go`, `cmd/run_test.go`, `integration/multi_agent_test.go`) and added zero error types. 13.2 adds one package (`internal/gitfetch/`) and one test file (`integration/fetch_test.go`) — still bounded, still zero error types
- **`resetRunCommandState` extension:** 12.1 introduced this helper at `cmd/run_test.go:31-61` because package-global `runCmd` leaks flag state between tests. Every new flag must reset here. Task 5.2 does this for `--fetch`
- **Em-dash lock via test:** 12.1's `TestRun_agentBothPositionalAndFlag_usageError` locked the exact error string byte-for-byte. 13.2's `TestFormatFetchSummary_tableDriven` locks the summary-line strings the same way
- **Pure-function preference:** 12.1 extracted `resolveAgentOverride` as a pure helper. 13.2 applies the same pattern to `formatFetchSummary` and `FetchAll`
- **`Flags().Changed()` pattern:** 12.1 uses `cmd.Flags().Changed("agent")` to distinguish "user set the flag" from "flag has default value". Not needed for 13.2 — `--fetch` is a bool whose default is `false`, and `GetBool` returning `true` is identical to "user set it" because the default is falsy. If a later story adds non-boolean fetch flags, revisit
- **Full test suite is the gate:** 12.1 ran `gofmt -w`, `go vet ./...`, `go test ./...`. Task 8 applies the same gate

### Relationship to Story 13.1

Story 13.1 (Branch-Management Guidance in Generated Agent Instructions) is a **sibling**, not a prerequisite. 13.1 edits `embed/agent-instructions.md.tmpl` and extends `internal/mount/bmad_repos_test.go`. 13.2 adds `internal/gitfetch/` and edits `cmd/run.go`. **Zero file overlap** — they can be implemented in either order, in parallel, or independently.

The epic description (epics.md:1555) notes that 13.1's branch-management guidance explicitly instructs the agent to branch off `origin/<default>`, which "closes the loop" with 13.2's refs update. This coupling is in the USER's workflow (fetch host-side → branch inside sandbox picks up fresh refs) — not in the code. Neither story imports or depends on the other.

**As of 2026-04-18, story 13.1 has not been implemented.** Epic 13 transitions to `in-progress` when this story file is created (per the workflow's epic-status rule). If a developer works on 13.1 concurrently, coordinate via git branches — file overlap is zero, merge conflicts are impossible.

### Git Intelligence (Recent Commits)

```
12dcb76 feat: short -a flag and positional agent argument for run (story 12-1)
45f59db docs: refine UX for --fetch operation on bmad repos
60fef54 docs: flush future work into PRD and sprint
d38f080 docs: update future work
0caf2d1 feat: agent command injection hardening via direct exec (story 11-6)
```

**Commit message convention:** `feat: <short description> (story N-M)` for feature stories, `docs: <description>` for doc-only edits. Use `feat: --fetch flag for host-side upstream sync (story 13-2)` or similar when committing.

Notable: commit `45f59db docs: refine UX for --fetch operation on bmad repos` landed the UX decisions this story implements (anchor line wording, summary shapes, WARNING prefix, em-dash). Trust the current state of epics.md / architecture.md as the canonical design — do not reinvent the UX details.

### CLAUDE.md Compliance Checklist

- [ ] **Error handling:** `errors.Is`/`errors.As` for all error comparisons (use them in `TestFetchAll_timeout`, etc.). Zero new error types → no `exitCode()` changes
- [ ] **Testing:** Stdlib `testing` only. Table-driven `TestFormatFetchSummary_tableDriven`. `t.TempDir()` for all temp dirs. `t.Cleanup` not `defer` in any subtests with `t.Parallel`
- [ ] **Code organization:** Error types per owning package — reuse `*config.ConfigError` only. New package `internal/gitfetch/` contained under `internal/`. Pure functions preferred (`FetchAll`, `DedupPaths`, `formatFetchSummary`)
- [ ] **Agent registry invariant:** Untouched. `--fetch` is agent-agnostic
- [ ] **Import alias:** `asboxEmbed` alias not needed here — this story touches no embed assets

### Source Hints for Fast Navigation

| Artifact | Path | Relevant Lines |
|----------|------|:--------------:|
| `runCmd` struct / orchestration | `cmd/run.go` | 18-154 |
| Flag registration site | `cmd/run.go` | 238-242 |
| `usageError` + `exitCode()` — unchanged | `cmd/root.go` | 19-71 |
| `Config` struct with `BmadRepos` | `internal/config/config.go` | 40-53 |
| `*config.ConfigError` — reuse | `internal/config/errors.go` | 6-16 |
| `resolvePath` — already used for `cfg.BmadRepos` | `internal/config/parse.go` | 204-224 |
| `AssembleBmadRepos` — comparable new-package pattern | `internal/mount/bmad_repos.go` | entire file |
| `writeRunConfig` / `newRootCmd` test pattern | `cmd/run_test.go` | 17-29, 63-89 |
| `resetRunCommandState` — extend here | `cmd/run_test.go` | 31-61 |
| Integration binary-invocation pattern | `integration/bmad_repos_test.go` | 12-53 |
| Architecture: Multi-Repo Upstream Fetch decision | `_bmad-output/planning-artifacts/architecture.md` | 309-326 |
| Architecture: FR65 traceability row | `_bmad-output/planning-artifacts/architecture.md` | 719 |
| Architecture: `internal/gitfetch/` structure | `_bmad-output/planning-artifacts/architecture.md` | 551-555 |
| PRD: FR65 | `_bmad-output/planning-artifacts/prd.md` | 496 |
| Epic 13 narrative + scope boundary | `_bmad-output/planning-artifacts/epics.md` | 1523-1527 |
| Story 13.2 acceptance criteria | `_bmad-output/planning-artifacts/epics.md` | 1566-1642 |
| Previous story 12.1 (Epic 12-13 patterns) | `_bmad-output/implementation-artifacts/12-1-short-flag-and-positional-agent-argument.md` | entire file |

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic 13 — Multi-Repo State Management (lines 1523-1527)]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 13.2 `--fetch` Flag for Host-Side Upstream Sync (lines 1566-1642)]
- [Source: _bmad-output/planning-artifacts/architecture.md#Multi-Repo Upstream Fetch (`--fetch`) (lines 309-326)]
- [Source: _bmad-output/planning-artifacts/architecture.md#Project Layout — internal/gitfetch/ (lines 551-555)]
- [Source: _bmad-output/planning-artifacts/architecture.md#Traceability — FR65 (line 719)]
- [Source: _bmad-output/planning-artifacts/prd.md#Functional Requirements — FR65 (line 496)]
- [Source: cmd/run.go — orchestration sequence (lines 28-154), flag registration (lines 238-242)]
- [Source: cmd/root.go — usageError + exitCode mapping (lines 19-71) — unchanged]
- [Source: internal/config/config.go — `Config.BmadRepos` (lines 40-53)]
- [Source: internal/config/errors.go — `*ConfigError` — reuse for `ASBOX_FETCH_TIMEOUT` parse error]
- [Source: internal/mount/bmad_repos.go — new-package pattern reference]
- [Source: cmd/run_test.go — `resetRunCommandState` extension point (lines 31-61)]
- [Source: integration/bmad_repos_test.go — binary-invocation pattern (lines 12-53)]
- [Source: _bmad-output/implementation-artifacts/12-1-short-flag-and-positional-agent-argument.md — previous story patterns]
- [Source: CLAUDE.md — error handling, testing, code organization conventions]

## Dev Agent Record

### Agent Model Used

Codex (GPT-5)

### Implementation Plan

- Add `internal/gitfetch` with bounded concurrency, per-repo timeout handling, dedup, and pure result aggregation.
- Wire `asbox run --fetch` after mount assembly, emit the required anchor/info/warning/summary lines, and keep failures non-fatal.
- Add unit and integration coverage for fetch behavior, then run formatting, vet, targeted tests, and the full suite.

### Debug Log References

- `go get golang.org/x/sync/errgroup`
- `env GOCACHE=/tmp/asbox-gocache GOMODCACHE=/tmp/asbox-gomodcache go mod tidy`
- `env GOCACHE=/tmp/asbox-gocache GOMODCACHE=/tmp/asbox-gomodcache go vet ./...`
- `env GOCACHE=/tmp/asbox-gocache GOMODCACHE=/tmp/asbox-gomodcache go test ./internal/gitfetch/`
- `env GOCACHE=/tmp/asbox-gocache GOMODCACHE=/tmp/asbox-gomodcache go test ./cmd/`
- `env GOCACHE=/tmp/asbox-gocache GOMODCACHE=/tmp/asbox-gomodcache go test ./integration -run TestFetch`
- `env GOCACHE=/tmp/asbox-gocache GOMODCACHE=/tmp/asbox-gomodcache go test ./integration -run TestAutoIsolateDeps_logsVolumeCreation`
- `env GOCACHE=/tmp/asbox-gocache GOMODCACHE=/tmp/asbox-gomodcache go test ./...`

### Completion Notes List

- Implemented host-side upstream sync in `internal/gitfetch` with `git fetch origin`, per-repo timeout handling, canonical-path deduplication, non-fatal failure aggregation, and refs-only invariants.
- Added `asbox run --fetch` orchestration, anchor/info/warning/summary presentation, `ASBOX_FETCH_TIMEOUT` validation, and project-dir inclusion only when the project root is itself a git repo.
- Added fetch unit and integration coverage and fixed the existing `integration/isolate_deps_test.go` cleanup hazard by precreating `node_modules` mount points so the Docker-backed full suite completes cleanly.

### File List

- `README.md`
- `_bmad-output/implementation-artifacts/13-2-fetch-flag-for-upstream-sync.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`
- `cmd/run.go`
- `cmd/run_test.go`
- `go.mod`
- `go.sum`
- `integration/fetch_test.go`
- `integration/isolate_deps_test.go`
- `internal/gitfetch/fetch.go`
- `internal/gitfetch/fetch_test.go`

### Change Log

- `2026-04-18`: Implemented host-side `--fetch` upstream sync, added fetch package/tests and CLI wiring, updated the run docs, and fixed the Docker-backed auto-isolate integration cleanup needed for a green full suite.

### Review Findings

_Code review 2026-04-20 — 3-layer adversarial (Blind Hunter, Edge Case Hunter, Acceptance Auditor)._

- [x] [Review][Patch] (was Decision, user chose fail-fast) `GIT_TERMINAL_PROMPT=0` + `GIT_ASKPASS=/bin/true` now set via `nonInteractiveGitEnv` on both `defaultFetchRepo` and `hasOriginRemote`. [`internal/gitfetch/fetch.go`]
- [x] [Review][Patch] `DedupPaths` now returns preamble paths alongside canonical fetchable paths, so `summary.Total` matches the anchor-line count and non-existent paths surface as `StatusSkippedNotGit`. [`internal/gitfetch/fetch.go`]
- [x] [Review][Patch] Per-repo failure warning rewritten to `"warning: fetch failed for %s: %s\n"` — first-line only, no trailing stderr dump. [`cmd/run.go`]
- [x] [Review][Patch] Plural branches in `formatCount` removed; singular form is used regardless of count. `formatCount` deleted; `fmt.Sprintf` inlined. Table test extended with n=2 cases for both skip categories. [`cmd/run.go`, `cmd/run_test.go`]
- [x] [Review][Patch] Introduced `gitfetch.ErrTimeout` sentinel and wrap via `%w`; `firstLineOfStderrOrErr` drops the `ASBOX_FETCH_TIMEOUT` substring special case and follows spec order (stderr first, err fallback). Added `TestFirstLineOfStderrOrErr_contract`. [`internal/gitfetch/fetch.go`, `cmd/run.go`, `cmd/run_test.go`]
- [x] [Review][Patch] `ASBOX_FETCH_TIMEOUT` parse error message now matches spec: `invalid ASBOX_FETCH_TIMEOUT value %q: %s. Use a Go duration string like 60s, 2m, 500ms`. Test tightened to assert the spec phrasing. [`cmd/run.go`, `cmd/run_test.go`]
- [x] [Review][Patch] Project-dir auto-inclusion is now unconditional — `hasGitEntry` removed; `FetchAll` classifies non-git project dirs as `StatusSkippedNotGit`. Integration test expectations updated to reflect the new 3-path count. [`cmd/run.go`, `integration/fetch_test.go`]
- [x] [Review][Patch] `hasOriginRemote` simplified to `return cmd.Run() == nil`; locale-fragile stderr substring check removed. [`internal/gitfetch/fetch.go`]
- [x] [Review][Patch] Non-positive `ASBOX_FETCH_TIMEOUT` (e.g. `0s`, `-1s`) now rejected with a `*config.ConfigError` containing "must be positive". `TestRun_fetchWithNonPositiveTimeoutEnv_returnsConfigError` locks the behavior across `0`, `0s`, `-1s`. [`cmd/run.go`, `cmd/run_test.go`]
- [x] [Review][Patch] `TestFirstLineOfStderrOrErr_contract` locks the warning-line contract (stderr first, err fallback, timeout text). Table summary test covers n=2 skip cases. [`cmd/run_test.go`]
- [x] [Review][Defer] Parent ctx cancellation (Ctrl+C) not distinguished from per-repo `DeadlineExceeded` — all in-flight repos report generic "fetch failed". Spec Task 3.11 explicitly accepts this ("acceptable blast radius"). [`internal/gitfetch/fetch.go:179-188`] — deferred, spec-accepted
- [x] [Review][Defer] `fetchRepoFn` is a package-global mutable var with no mutex — latent race if tests ever run in parallel. [`internal/gitfetch/fetch.go:55`] — deferred, test-only surface
- [x] [Review][Defer] Hardcoded 1-second timeout for `remote get-url` is locale/NFS-fragile — slow filesystems or non-English `LANG` could misclassify `StatusSkippedNoOrigin`. Spec Task 2.7 specifies 1s. [`internal/gitfetch/fetch.go:194`] — deferred, spec-prescribed
- [x] [Review][Defer] `projectDir := filepath.Dir(filepath.Dir(absConfigFile))` assumes config lives at `<project>/.asbox/config.yaml` — if user passes `-f /other/path.yaml`, `projectDir` may resolve to an unintended ancestor that happens to be a git repo. [`cmd/run.go:145`] — deferred, spec permits
- [x] [Review][Defer] Full stderr dump in warning admits terminal-escape injection from hostile remotes and can flood terminal on large packfile errors. Partially mitigated by patch P2. [`cmd/run.go:172-178`] — deferred, downstream of P2
- [x] [Review][Defer] `FetchResult.Path` is the raw input for preamble entries but canonical for fetched entries — user-facing warnings display mixed path formats. [`internal/gitfetch/fetch.go:129,136,155`] — deferred, cosmetic
- [x] [Review][Defer] `TestFetchAll_gitWorktreeMarkerCountsAsRepo` uses `git worktree add` (real origin) but Task 4.5 asked for a manually-created `.git` file with empty gitdir target to lock the "file OR directory" contract at a different invariant. [`internal/gitfetch/fetch_test.go:121`] — deferred, existing test still useful
- [x] [Review][Defer] `filepath.Abs(configFile)` returns raw error (not `*config.ConfigError`) — inconsistent with neighboring error-routing. Unlikely in practice. [`cmd/run.go:130-133`] — deferred, not fetch-specific
