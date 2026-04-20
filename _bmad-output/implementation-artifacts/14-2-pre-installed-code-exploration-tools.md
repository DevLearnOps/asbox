# Story 14.2: Pre-Installed Code Exploration Tools

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want `ripgrep`, `fd`, `ast-grep`, and `universal-ctags` pre-installed in the sandbox image at pinned versions,
so that the agent can navigate and search repositories efficiently without falling back to `find` or building its own symbol maps.

## Acceptance Criteria

1. **Given** a newly built sandbox image
   **When** the agent invokes `rg --version`, `fd --version`, `ast-grep --version`, or `ctags --version`
   **Then** each tool responds with exit code 0 and reports the pinned version baked into the image

2. **Given** the agent runs `rg <pattern>` inside a mounted project with a `.gitignore`
   **When** the search runs
   **Then** `.gitignore` is respected by default (files listed in it are not searched) and the pattern matches across tracked files in the repo

3. **Given** the agent runs `fd <pattern>` inside a mounted project with a `.gitignore`
   **When** the search runs
   **Then** files matching the pattern are returned and `.gitignore` is honored by default

4. **Given** the agent runs `ast-grep run -p '<pattern>' --lang <lang>` inside a mounted project
   **When** the search runs
   **Then** structural matches are returned (AST-based match, not plain-text grep) — this replaces plain-text grep when the agent needs semantic matches

5. **Given** `embed/Dockerfile.tmpl` is inspected
   **When** the `exploration_tools` RUN block is read
   **Then** each of the four tools is installed at an explicit pinned version (no `latest`, no `@latest`, no GitHub API `latest` endpoint, no `curl | bash` installers) and every pinned version is declared in the same comment header block introduced by story 14.1 so there is exactly one place to edit

6. **Given** the integration test suite
   **When** the toolchain smoke test runs
   **Then** each of the four exploration tools is asserted to exist on `PATH` and to respond to its version probe with exit code 0 inside a freshly launched sandbox, running as the `sandbox` user — alongside the DevOps tools asserted by story 14.1

7. **Given** the Dockerfile template is rendered with any valid `Config` (any SDK combination, any agents installed, any MCP set)
   **When** the output is inspected
   **Then** the `exploration_tools` RUN block is always present — it is NOT conditional on any config field. The toolchain is part of the base sandbox image

8. **Given** the generated agent instructions (`embed/agent-instructions.md.tmpl`)
   **When** an agent reads them inside the sandbox
   **Then** the existing "Installed Tooling > Code Exploration" subsection (currently a pending placeholder left by story 14.1) is replaced with a prescriptive list of the four tools plus `git ls-files`, so the agent prefers them over `find` / plain grep and does not waste turns re-discovering them

9. **Given** a content hash is computed for the image (per `internal/hash/`)
   **When** a pinned version in the `exploration_tools` block is bumped in `embed/Dockerfile.tmpl`
   **Then** the content hash changes and the next `asbox build` / `asbox run` triggers a full rebuild — matching the existing reproducibility contract (NFR12, NFR16). No new logic is added to `internal/hash/`; the existing template-content input is sufficient

10. **Given** the Ubuntu-named binary `fdfind` (apt ships `fd-find` under that name to avoid a collision with the legacy `fd` tool)
    **When** the agent invokes `fd` (the canonical upstream command name)
    **Then** `fd` resolves to the same binary — a symlink `/usr/local/bin/fd → /usr/bin/fdfind` is created at image build time so the agent uses the upstream-documented name

## Tasks / Subtasks

- [x] Task 1: Extend the `Pinned validation tools` subsection in `embed/Dockerfile.tmpl` header with exploration tools (AC: #5)
  - [x] 1.1 In the existing header comment block (`{{- /* ... */ -}}`), add a new "Pinned exploration tools" subsection immediately after "Pinned validation tools" (ends around line 23) listing each of the four tool names and its pinned version. Mirror the style of the validation-tools subsection. Proposed versions (validate at build time, bump to newer patch release if upstream has released one — record the final chosen version in this block):
    - `ripgrep`: `14.1.0-1` (Ubuntu 24.04 apt candidate — verified at creation time)
    - `fd-find`: `9.0.0-1` (Ubuntu 24.04 apt candidate — verified at creation time; binary is `fdfind`, aliased to `fd` via symlink)
    - `universal-ctags`: `5.9.20210829.0-1` (Ubuntu 24.04 apt candidate — verified at creation time)
    - `ast-grep`: `0.42.1` (GitHub release — versioned zip per architecture)
  - [x] 1.2 Extend the "Validation tools update procedure" section into a combined "Toolchain update procedure" — or add an immediately-adjacent "Exploration tools update procedure" subsection — with one entry per exploration tool. Each entry documents: upstream release page, one-liner to check for newer versions, and asset-naming note. Specifically:
    - ripgrep / fd-find / universal-ctags: `apt-cache policy <pkg>` inside a fresh `ubuntu:24.04@<digest>` container. Version format is Debian-style (`14.1.0-1`, not `v14.1.0`) — match what `apt-cache` returns exactly, including the `-N` revision suffix
    - ast-grep: `curl -s https://api.github.com/repos/ast-grep/ast-grep/releases/latest | jq -r .tag_name` (documentation only — the Dockerfile never calls this endpoint at build time). Asset names are `app-x86_64-unknown-linux-gnu.zip` / `app-aarch64-unknown-linux-gnu.zip` — the arch token uses `uname -m`-style names, NOT `dpkg --print-architecture`, so a small case-mapping is required (see Task 2.4)
  - [x] 1.3 Keep the existing "Validation tools update procedure" and "Current pinned versions" subsections intact — do not consolidate or rewrite them. Append the new content so git diffs stay focused on exploration-tools additions (matches 14.1's discipline)
  - [x] 1.4 Final verification line in the header should match 14.1's: `Run go test ./... && go test -v ./integration -count=1 -timeout 20m after every bump.` Do not downgrade this to 10m — the validation+exploration build layer is the reason 14.1 requested 30m timeouts on integration runs

- [x] Task 2: Add the `exploration_tools` RUN block to `embed/Dockerfile.tmpl` (AC: #5, #7, #10)
  - [x] 2.1 Placement: insert immediately **after** the closing `rm -rf /tmp/*` line of the `validation_tools` block (currently `embed/Dockerfile.tmpl:173`) and **before** the `{{- if .SDKs.NodeJS}}` conditional (currently `embed/Dockerfile.tmpl:174`). Rationale: both toolchain blocks must stay grouped at the top of the unconditional section so Docker layer caching reuses them across every config variant (matches architecture.md:340-350 decision)
  - [x] 2.2 Start the block with a single-line comment: `# exploration_tools — pinned code exploration toolchain (FR63, NFR16). See header for version bump procedure.` Mirror the header pattern established by 14.1
  - [x] 2.3 Use a single `RUN set -eux; ...` chain joined with `&&` (one layer for the whole block), matching the shape of the `validation_tools` block. Start with `cd /tmp` and end with `rm -rf /tmp/*` so intermediate archive files do not leak into the image
  - [x] 2.4 Install the four tools at the versions from Task 1.1. Arch mapping is different from 14.1 — ast-grep uses `uname -m`-style tokens, so derive a second local variable:

    ```dockerfile
    # exploration_tools — pinned code exploration toolchain (FR63, NFR16). See header for version bump procedure.
    RUN set -eux; \
        cd /tmp; \
        dpkgArch="$(dpkg --print-architecture)"; \
        case "${dpkgArch}" in \
            amd64) astGrepArch="x86_64" ;; \
            arm64) astGrepArch="aarch64" ;; \
            *) echo "unsupported architecture: ${dpkgArch}" >&2; exit 1 ;; \
        esac; \
        apt-get update && \
        apt-get install -y --no-install-recommends \
            ripgrep=14.1.0-1 \
            fd-find=9.0.0-1 \
            universal-ctags=5.9.20210829.0-1 && \
        rm -rf /var/lib/apt/lists/* && \
        # fd-find installs as /usr/bin/fdfind on Debian/Ubuntu to avoid collision with an older tool.
        # Symlink to /usr/local/bin/fd so the upstream-documented command name works (AC#10).
        ln -sf /usr/bin/fdfind /usr/local/bin/fd && \
        # ast-grep: arch uses uname -m-style tokens => x86_64|aarch64 (mapped from dpkg arch above)
        curl -fsSL "https://github.com/ast-grep/ast-grep/releases/download/0.42.1/app-${astGrepArch}-unknown-linux-gnu.zip" -o /tmp/ast-grep.zip && \
        unzip -q /tmp/ast-grep.zip -d /tmp/ast-grep && \
        install -m 0755 /tmp/ast-grep/ast-grep /usr/local/bin/ast-grep && \
        rm -rf /tmp/*
    ```

  - [x] 2.5 **Invariant — no `curl | bash`:** per AC#5 and architecture.md:347, integrity failures in `curl | sh` flows are silent. `ast-grep` publishes a known-name zip asset with a known binary inside — use the direct URL. Do not substitute an upstream install script even if ast-grep documents one
  - [x] 2.6 **Invariant — no dynamic `latest` lookup:** `curl https://api.github.com/repos/.../releases/latest | jq -r .tag_name` is forbidden inside the Dockerfile. The version `0.42.1` is a literal in the template. Story 14.1's `TestRender_validationToolsNoDynamicLatest` already enforces this globally; Task 3.4 extends the coverage for the exploration block
  - [x] 2.7 **Invariant — apt version pins are exact match including revision suffix:** `ripgrep=14.1.0-1` (with the `-1`) not `ripgrep=14.1.0` — apt treats them as different and will fail with "Version X is not available" if the revision is omitted. Verify by running `apt-cache policy ripgrep` inside `ubuntu:24.04@<digest>` before bumping
  - [x] 2.8 Do NOT attempt to install ast-grep's `sg` alias — upstream ships both `ast-grep` and `sg` in the same zip, but `sg` collides with the POSIX `sg` command (switch group) already on PATH from `/usr/bin/sg`. Install only `/usr/local/bin/ast-grep` (the `install -m 0755` line above copies exactly that one binary). The agent uses `ast-grep` explicitly — `sg` is not advertised anywhere in `agent-instructions.md.tmpl`

- [x] Task 3: Extend unit tests in `internal/template/render_test.go` (AC: #5, #7, #9)
  - [x] 3.1 Add `TestRender_explorationToolsBlockPresent(t *testing.T)`: render with a minimal config (`&config.Config{InstalledAgents: []string{"claude"}}`) and assert `strings.Contains(output, "# exploration_tools")` and `strings.Contains(output, "/usr/local/bin/ast-grep")`. Mirrors `TestRender_validationToolsBlockPresent` at `render_test.go:460`
  - [x] 3.2 Add `TestRender_explorationToolsAllFourPresent(t *testing.T)`: render with minimal config, assert these substrings appear:
    - `ripgrep=14.1.0-1`
    - `fd-find=9.0.0-1`
    - `universal-ctags=5.9.20210829.0-1`
    - `/usr/local/bin/ast-grep` (the final install target — `/usr/local/bin/` appears only in install lines, not in comments)
    - `ln -sf /usr/bin/fdfind /usr/local/bin/fd` (locks the AC#10 symlink)
  - [x] 3.3 Add `TestRender_explorationToolsPinnedVersions(t *testing.T)`: table-driven `{tool, versionSubstring}` pairs for the four tools (`ripgrep 14.1.0-1`, `fd-find 9.0.0-1`, `universal-ctags 5.9.20210829.0-1`, `ast-grep 0.42.1`). Pattern matches `TestRender_validationToolsPinnedVersions` at `render_test.go:502`
  - [x] 3.4 Extend `TestRender_validationToolsNoDynamicLatest` — rename to `TestRender_toolchainNoDynamicLatest` (or add a parallel `TestRender_explorationToolsNoDynamicLatest`, whichever keeps the diff narrow) so the `!api.github.com` / `!releases/latest` / `!| bash` assertions now cover both blocks. If kept as two tests, the exploration variant should use `&config.Config{}` for the same reason the 14.1 variant does (see the in-line comment at `render_test.go:534-539` about template-comment stripping)
  - [x] 3.5 Add `TestRender_explorationToolsUnconditional(t *testing.T)`: render with three configs — (a) `{}`, (b) minimal claude-only, (c) claude+gemini+codex+playwright+python+nodejs+go full config. Assert the `# exploration_tools` substring appears in all three. Mirrors `TestRender_validationToolsUnconditional` at `render_test.go:555` (AC#7)
  - [x] 3.6 Add `TestRender_fdSymlinkCreated(t *testing.T)`: assert the render contains `ln -sf /usr/bin/fdfind /usr/local/bin/fd`. Locks AC#10 independently so a refactor of the install commands cannot silently drop the symlink
  - [x] 3.7 Do NOT duplicate the `TestRender_cacheDirsChowned` assertion logic for exploration tools — exploration tools are stateless (no `~/.cache/ast-grep`, no `~/.ripgrepconfig` requirement at build time) and need no new chown lines. If the dev agent finds itself writing such a test, stop — something has drifted

- [x] Task 4: Extend `integration/toolchain_test.go` with exploration-tool smoke tests (AC: #1, #2, #3, #4, #6)
  - [x] 4.1 Add `TestToolchain_codeExploration(t *testing.T)`: table-driven, one subtest per tool with a `{name, command, expectSubstring}` row. Use `execAsUser(ctx, t, container, "sandbox", cmd)` per 14.1's pattern. Reuse `buildTestImage(t)` and `startTestContainer(ctx, t, image)` — DO NOT introduce a second image-build path. Rows:

    ```go
    {name: "ripgrep",          command: []string{"rg", "--version"},        expectSubstring: "ripgrep 14.1.0"},
    {name: "fd",               command: []string{"fd", "--version"},        expectSubstring: "fd 9.0.0"},
    {name: "ast-grep",         command: []string{"ast-grep", "--version"},  expectSubstring: "ast-grep 0.42.1"},
    {name: "ctags",            command: []string{"ctags", "--version"},     expectSubstring: "Universal Ctags"},
    ```

    The `ctags --version` first line reads `Universal Ctags 5.9.0(p6.1.20210829.0-...)` — asserting `Universal Ctags` is both version-robust-ish and disambiguates from `exuberant-ctags`. Follow the same "fail loudly on version drift" discipline 14.1 established — when a version bumps, this assertion must be updated in the same diff as the Dockerfile and the header comment, which prevents silent version drift. Mark subtests with `t.Parallel()` per the existing pattern in `TestToolchain_devopsValidation` at `integration/toolchain_test.go:40-53`
  - [x] 4.2 Add `TestToolchain_rgRespectsGitignore(t *testing.T)`: create a temp project inside the container using `bash -lc` + heredoc (same shape as `TestToolchain_helmTemplateOffline` at `integration/toolchain_test.go:107-129`):
    - `mkdir -p $tmp/src && echo 'needle' > $tmp/src/hit.txt && echo 'needle' > $tmp/ignored.log && echo '*.log' > $tmp/.gitignore && (cd $tmp && git init -q && rg -n --no-heading needle)`
    - Assert exit code 0 AND stdout contains `src/hit.txt` AND stdout does NOT contain `ignored.log`. Locks AC#2
    - Use `su sandbox` via `execAsUser` so gitignore is applied with the sandbox user's HOME, not root's
  - [x] 4.3 Add `TestToolchain_fdRespectsGitignore(t *testing.T)`: same shape — create `$tmp/src/keep.go`, `$tmp/ignored.log`, `$tmp/.gitignore` containing `*.log`, `git init`, then run `fd -e go` from inside `$tmp`. Assert `keep.go` is in output, `ignored.log` is NOT. Locks AC#3
  - [x] 4.4 Add `TestToolchain_astGrepStructuralMatch(t *testing.T)`: create a minimal JS file `$tmp/demo.js` with `console.log("hello"); console.warn("hi");`, then run `ast-grep run -p 'console.log($X)' --lang js $tmp`. Assert exit code 0 AND stdout contains `demo.js` AND stdout does NOT contain the `console.warn` line (because the pattern is structural, not substring). Locks AC#4. If ast-grep's exit code is non-zero when no matches are found, pick a pattern that matches by design in the fixture
  - [x] 4.5 Do NOT extend `TestToolchain_devopsValidation` to include rg/fd/ast-grep/ctags — keep exploration tests in their own test function so the subtest names stay semantic (`code_exploration/rg`, not `devops_validation/rg`). Future maintainers reading a test failure should see the category in the failure name
  - [x] 4.6 The existing `TestToolchain_devopsValidation` runs subtests in parallel (12 tools × `t.Parallel()`). Exploration-tools subtests are also safe to parallelize — they are read-only version probes against pre-installed binaries

- [x] Task 5: Update `embed/agent-instructions.md.tmpl` Code Exploration subsection (AC: #8)
  - [x] 5.1 Locate the existing `### Code Exploration` subsection at `embed/agent-instructions.md.tmpl:35-37`. It currently reads:

    ```markdown
    ### Code Exploration

    Additional code-exploration tooling is pending and will be documented here when available.
    ```

  - [x] 5.2 Replace the entire subsection (including the placeholder sentence) with a prescriptive list matching the DevOps Validation style (each line is `- `<name>` — ≤10-word purpose note`). Target content:

    ```markdown
    ### Code Exploration

    - `rg` (ripgrep) — Fast content search; respects `.gitignore` by default.
    - `fd` — Fast file-name search; respects `.gitignore` by default.
    - `ast-grep` — Structural code search via AST; use `ast-grep run -p '<pattern>' --lang <lang>`.
    - `ctags` (universal-ctags) — Symbol indexer; generate tags with `ctags -R .`.
    - `git ls-files` — Enumerate tracked files; prefer over `find` for clean repo traversal.
    ```

  - [x] 5.3 Keep phrasing prescriptive — the agent should read "use `X` for Y" and immediately know which tool to reach for. Do NOT add installation instructions, version numbers, or manpage links — those belong in the Dockerfile header and the architecture docs
  - [x] 5.4 `embed/agent-instructions.md.tmpl` is used for ALL three agents (Claude, Gemini, Codex) — see Dockerfile lines 256, 259, 262 where the same template renders into `CLAUDE.md`, `GEMINI.md`, and `.codex/AGENTS.md`. Do NOT introduce agent-specific branches; the tooling list is identical across agents
  - [x] 5.5 `git ls-files` is deliberately called out because git was installed as part of the base apt block (line 103 of the Dockerfile template, via FR23) and the agent historically reaches for `find` or raw `ls -R` when it wants "what files exist in this repo". This line is prescriptive — it tells the agent what to reach for, not what is installed. Do NOT add a separate `## Installed Tooling` entry for git; the base image already ships it and it is already covered by the existing `## Available Tools` and `## Constraints > Git push is disabled` sections

- [x] Task 6: Update the smoke-test checklist (AC: none directly — operational hygiene)
  - [x] 6.1 Append entries to `_bmad-output/implementation-artifacts/smoke-test-checklist.md` under a new "Story 14.2 — Code exploration toolchain" section below the existing "Story 14.1 — DevOps validation toolchain" section (currently at `smoke-test-checklist.md:79-83`):
    - `rg <pattern>` in a real mounted project respects `.gitignore` (spot-check against `node_modules/`)
    - `fd <glob>` in a real mounted project respects `.gitignore`
    - `ast-grep run -p '<pattern>' --lang <lang>` in a real mounted project returns expected AST matches (pick a real pattern like `console.log($X)` or `fmt.Println($X)` from the agent-facing repo)
    - `ctags -R .` generates a `tags` file without error in a real mounted project
  - [x] 6.2 These are manual checks that complement the CI-level integration tests in Task 4. The integration tests use synthetic fixtures; the smoke checks confirm behavior against real-world repo sizes and layouts

- [x] Task 7: Verify no impact to content hash, docker build, or orchestration code (AC: #9)
  - [x] 7.1 Run `go test ./internal/hash/ -v` — existing tests pass unchanged. The hash inputs already include the rendered Dockerfile per `architecture.md:243`, so extending the template automatically extends the hash (same reasoning as 14.1)
  - [x] 7.2 Run `go test ./internal/template/ -v` — confirms Task 3 tests pass
  - [x] 7.3 Run `go test ./cmd/ -v` — no changes expected; confirms no regression
  - [x] 7.4 Run `go vet ./... && gofmt -l . | tee /tmp/gofmt.out && test ! -s /tmp/gofmt.out` — no unformatted Go files
  - [x] 7.5 Run `go test -v ./integration -count=1 -timeout 30m` — full integration suite incl. Task 4. **Cold-build impact:** four additional package installs + one GitHub release download add ~10-30s to a clean build. Keep the 30m timeout from 14.1 — it still absorbs this comfortably. Subsequent test runs reuse the cached image layer and are unaffected
  - [x] 7.6 Run `go test ./...` as the final gate

### Review Findings

Code review run: 2026-04-20. Three-layer review (Blind Hunter, Edge Case Hunter, Acceptance Auditor). 16 raw findings → 2 patches, 4 deferrals, 10 dismissed.

**Patches (actionable):**

- [x] [Review][Patch] `TestToolchain_fdRespectsGitignore` does not actually verify gitignore behavior [integration/toolchain_test.go:191-193] — Fixture creates `keep.go` + gitignored `ignored.log`, then runs `fd -e go`. Because `-e go` filters by extension, `.log` is excluded by the extension filter alone — independently of `.gitignore`. The negative assertion passes even if `fd` completely ignores `.gitignore`. Fix: use `ignored.go` (matching extension) and add `ignored.go` (or glob pattern) to `.gitignore`. Compare to `TestToolchain_rgRespectsGitignore` where both files contain `needle` and only `.gitignore` can exclude the ignored one. **Fixed 2026-04-20.**
- [x] [Review][Patch] Smoke-test checklist drops ast-grep example pattern [_bmad-output/implementation-artifacts/smoke-test-checklist.md:89] — Spec Task 6.1 prescribes: "pick a real pattern like `console.log($X)` or `fmt.Println($X)` from the agent-facing repo". Actual entry omits the parenthetical. Fix: append the guidance so an operator running the smoke list knows what to type. **Fixed 2026-04-20.**

**Deferred (tracked in `deferred-work.md`):**

- [x] [Review][Defer] `TestToolchain_astGrepStructuralMatch` negative assertion brittle to future ast-grep output-format changes [integration/toolchain_test.go:301-303] — deferred, pre-existing; no break at pinned 0.42.1. Hardening would move `console.warn` line further from the match or into a separate file.
- [x] [Review][Defer] `ast-grep` zip-extraction assumes flat layout at `/tmp/ast-grep/ast-grep` [embed/Dockerfile.tmpl:222-223] — deferred, pre-existing; integration tests pass at 0.42.1 so current layout is confirmed. Future upstream version bumps could change layout (e.g., nested under `ast-grep-<version>/`). A local `find /tmp/ast-grep -name ast-grep -type f` probe would harden for bumps.
- [x] [Review][Defer] `TestRender_toolchainNoDynamicLatest` depends on `{{- /* ... */ -}}` wrapper stripping maintenance notes [internal/template/render_test.go:590-610] — deferred, pre-existing; works today because the header is inside a single Go-template comment. Splitting/relocating the maintenance notes outside the wrapper would flip the negative assertions red without any functional regression.
- [x] [Review][Defer] Integration tests don't source `/etc/profile.d/sandbox-env.sh`; latent for any future test that runs `git commit` [integration/toolchain_test.go, `startTestContainer` helper] — deferred, pre-existing; `git init -q` works without identity. If a future test calls `git commit`, it fails with "Author identity unknown" because `startTestContainer` replaces the entrypoint with `tail -f /dev/null` and `persist_env` never runs.

**Dismissed (10, not written — summary):** fd `fdfind 9.0.0` assertion deviation (reality-matching; Ubuntu's binary prints its own name); rg gitignore speculation; git init identity speculation; apt pin fragility (architectural decision accepted by spec); ast-grep tag format (validated by passing build); apt cache cross-layer (standard pattern); unit tests passing if RUN commented out (implausible); narrow `releases/latest` assertion; smoke-test version pinning (intentionally version-agnostic); heading case mismatch (internally consistent with 14.1 sibling).

## Dev Notes

### Developer Context — Why This Story Exists

Today the agent inside a sandbox reaches for `grep -r` or `find` whenever it needs to locate code, and falls back to plain-text grep for structural refactors (renaming all callers of a function, finding every invocation of a method on a specific type, etc.). Both are slow on mid-sized repos, both ignore `.gitignore` unless the agent remembers to exclude `node_modules/` explicitly, and neither understands ASTs — so the agent ends up emitting more tokens of "let me try a different pattern" than tokens of actual work.

This story ships the same tools a senior engineer would reach for: `ripgrep` for fast content search, `fd` for fast file-name search, `ast-grep` for AST-aware pattern matching (rename, refactor, find-by-structure), and `universal-ctags` when a symbol index is genuinely wanted. The epic framing in `epics.md:1697` calls this out alongside the DevOps toolchain: "Agents spending turns on `apt-get install kubectl` or discovering that `fd` isn't installed are agents losing time to infrastructure instead of solving the problem."

### Critical Architectural Compliance Points

- **architecture.md:340-350 "Pre-Installed Validation & Exploration Toolchain"** is the authoritative design. It explicitly mandates two new RUN blocks: `validation_tools` (delivered by 14.1) and `exploration_tools` (this story). Do not merge the two blocks into one — keep them adjacent but separate so the rationale and ownership stays readable in the Dockerfile
- **architecture.md:345-346 "Installation strategy per tool":** exploration tools follow a **mixed** install strategy — apt for `ripgrep`, `fd-find`, `universal-ctags` (Ubuntu 24.04 candidates are acceptable per epic notes at `epics.md:1783`); versioned GitHub-release zip for `ast-grep` (apt does not ship it). Do not attempt to replace the apt installs with binary downloads for "consistency" — apt gives you transactional install + correct dpkg database entries that future apt operations can reason about, and the Ubuntu candidate versions are modern enough (14.1.0 ripgrep, 9.0.0 fd)
- **architecture.md:193 "Multi-architecture support"** mandates: `$(dpkg --print-architecture)` for Debian-style arch names (`amd64`/`arm64`), `$(uname -m)` for kernel-style names (`x86_64`/`aarch64`). `ast-grep` is the second-ever case in this codebase where the kernel-style naming is required — inline the mapping in the RUN block via `case "${dpkgArch}" in amd64) astGrepArch="x86_64" ;; arm64) astGrepArch="aarch64" ;; esac` (Task 2.4). Never hardcode `amd64`
- **architecture.md:242-247 "Content-Hash Caching"** means the hash inputs already cover the rendered Dockerfile and embedded scripts. No changes to `internal/hash/` are needed or wanted. Any attempt to add fields is a design regression (same invariant as 14.1)
- **architecture.md:460-463 "Go Template Conventions"** — comments use `{{/* */}}`, conditionals use `{{- if -}}` with whitespace trim. The `exploration_tools` block is NOT inside an `{{if}}` — it's plain Dockerfile content that always renders, matching `validation_tools`

### Scope — What Is And Is Not In This Story

**In scope:**
- New `exploration_tools` RUN block in `embed/Dockerfile.tmpl` (positioned directly after the `validation_tools` block)
- Pinned versions for 4 tools: `ripgrep`, `fd-find` (aliased to `fd`), `universal-ctags`, `ast-grep`
- Symlink `/usr/local/bin/fd → /usr/bin/fdfind` so the agent can invoke `fd` directly per AC#10
- Unit tests in `internal/template/render_test.go` mirroring the 14.1 pattern
- Integration smoke tests in `integration/toolchain_test.go` (new test functions added alongside 14.1's tests)
- Replace the "Code Exploration" placeholder in `embed/agent-instructions.md.tmpl` with the prescriptive list

**NOT in scope (belongs to other stories or explicitly deferred):**
- Any new config flags (`enable_exploration_tools: false`, etc.) — the toolchain is unconditional; if we add user control later it will be its own story
- Runtime wrappers or agent shims for these tools — the agent just invokes them
- Pre-building a ctags index of the mounted project at container start — `ctags -R` is fast enough to run on demand and the project mount is host-owned; pre-indexing creates a file-ownership headache
- Installing `sg` (ast-grep's short alias) — it collides with POSIX `sg` (switch group). Task 2.8 covers this explicitly
- Installing additional exploration tools (`fzf`, `comby`, `pt`, etc.) — the epic names exactly four, keep the scope bounded. If the user wants more later, it's a new story
- Any changes to `internal/hash/`, `internal/docker/`, `internal/config/`, `internal/mount/`, or `cmd/*` — no Go logic is added by this story (mirrors 14.1's file-scope discipline)

### Decision Points The Dev Agent Will Hit

1. **apt vs binary download for ripgrep/fd/ctags:** The epic notes at `epics.md:1783` explicitly allow apt. Ubuntu 24.04 candidates (verified 2026-04-20: `ripgrep 14.1.0-1`, `fd-find 9.0.0-1`, `universal-ctags 5.9.20210829.0-1`) are recent enough. **Stick with apt.** A binary download path would also work but costs more layer size (+~20MB zstd-compressed binaries per arch vs ~8MB apt delta) and loses the dpkg metadata for future upgrades. If a future story demands a newer ripgrep than Ubuntu ships, swap that single line to the binary download pattern — but not preemptively
2. **`fd` vs `fdfind`:** Debian/Ubuntu ship the `fd-find` package with the binary at `/usr/bin/fdfind` to avoid colliding with an ancient tool also named `fd`. Users and every upstream doc use `fd`. Ship a symlink. **Do not alias via `~/.bashrc`** — `~/.bashrc` is per-user and interactive-shell-only, so non-interactive exec sessions (which is how agents invoke commands) would not see it. A symlink in `/usr/local/bin/` is system-wide and PATH-first
3. **`ast-grep` vs `sg`:** ast-grep ships both `ast-grep` and `sg` in its release archive. **Install only `ast-grep`.** `sg` collides with the POSIX `sg` command (switch group, from `/usr/bin/sg`, part of `passwd` package). Overwriting `/usr/bin/sg` or shadowing it with a `/usr/local/bin/sg` is a foot-gun for any script that expected the original. Task 2.4's `install -m 0755 /tmp/ast-grep/ast-grep /usr/local/bin/ast-grep` copies exactly one file out of the extracted zip — which is the correct behavior
4. **Universal ctags vs exuberant ctags:** Ubuntu's `ctags` package is exuberant-ctags (unmaintained since 2009). `universal-ctags` is the actively-maintained fork that every modern editor/IDE expects. Install `universal-ctags`, NOT `ctags` (which would pull in exuberant). Verify with `ctags --version` starting with `Universal Ctags` in Task 4.1's assertion
5. **ast-grep version choice:** `0.42.1` is the latest stable at time of story creation (2026-04-20). ast-grep is active — expect a patch release within weeks. The dev agent should bump to the latest stable at build time if upstream has released one since, and record the final chosen version in Task 1.1's comment block (same pattern as 14.1 for the 12 DevOps tools)
6. **ast-grep binary arch naming:** Unlike most tools in 14.1 which use Debian-style arch tokens, ast-grep publishes zips named `app-x86_64-unknown-linux-gnu.zip` and `app-aarch64-unknown-linux-gnu.zip` — Rust target-triple convention. The arch token is a `uname -m`-style name, not a dpkg arch. This is the SECOND place in the codebase where kernel-style arch naming is required (the first is Docker Compose at Dockerfile line 227, which uses `$(uname -m)` directly). Task 2.4 shows the explicit case mapping — do NOT use `$(uname -m)` directly here because consistency with the validation_tools block's `dpkgArch` variable makes the block easier to read

### Previous Story Intelligence (14.1 — DevOps Validation Toolchain, merged 2026-04-20)

Story 14.1 built the `validation_tools` block, the pinned-version comment header, unit tests, integration tests, and the `Installed Tooling` section of the agent instructions. This story is the sibling that fills in the second half. Critical patterns to reuse from 14.1:

- **Single RUN block per toolchain** — 14.1's block is one `RUN set -eux; ...` chain. This story matches that shape. Do NOT split into multiple RUNs "for readability" — that doubles the layer count for no cache benefit
- **Inline arch-scheme comments on every download line** — 14.1's block has `# kubectl: arch uses $(dpkg --print-architecture) => amd64|arm64` before each tool. Match that discipline for `ast-grep` (the only download-based tool in this story): the inline comment names the `uname -m`-style mapping explicitly
- **Test nomenclature** — 14.1 uses `TestRender_validationTools*` (unit) and `TestToolchain_*` (integration). This story uses `TestRender_explorationTools*` and `TestToolchain_codeExploration*`. Do NOT overload existing test names
- **Negative assertions as version-drift locks** — 14.1's `TestRender_validationToolsNoDynamicLatest` asserts `!api.github.com`, `!releases/latest`, `!| bash`. Either extend that test to cover the exploration block too (preferred — keep the invariants centralized) or add a parallel `TestRender_explorationToolsNoDynamicLatest`. Task 3.4 defers the choice to the dev agent; either is acceptable as long as the coverage exists
- **Version substrings in integration tests are DELIBERATELY version-specific** — Task 4.1's `expectSubstring` matches the pinned version exactly (e.g., `"ripgrep 14.1.0"`). When a version bumps, these assertions must be updated in the SAME diff as the Dockerfile. This is the mechanism that prevents silent version drift. Do not soften to regex or "major version only" matching
- **The `TestToolchain_cacheDirsOwnedBySandbox` test from 14.1 must stay green** — this story adds no cache directories (exploration tools are stateless), but the sandbox user still owns `/home/sandbox` so `ast-grep`'s implicit XDG cache usage (`~/.cache/ast-grep`) works out-of-the-box. Do NOT pre-create `~/.cache/ast-grep` in the Dockerfile — if ast-grep wants it, it will create it under `/home/sandbox/.cache/` which is already owned by `sandbox:sandbox` (via `useradd -m`). Verified by 14.1's existing tests

### Previous Story Intelligence (13.2 — `--fetch` Flag)

13.2 established the current commit-message convention and integration-test timeouts. This story inherits:

- **Commit message convention:** `feat: pre-installed code exploration tools (story 14-2)` for the final commit. Match the 14.1 style exactly: `feat: <short description> (story N-M)`
- **File-scope discipline:** 14.1 touched 5 files plus sprint-status. This story touches exactly the same set (minus `smoke-test-checklist.md` which was already created in 14.1 — this story appends a section). See "File-Change Summary" below
- **No user-facing runtime strings are added** — no em-dash or byte-for-byte string locks apply. Task 3.4's negative assertions are the equivalent invariant lock

### Git Intelligence (Recent Commits)

```
dbfe1e7 feat: pre-installed DevOps validation toolchain (story 14-1)
5500e37 feat: --fetch flag for host-side upstream sync (story 13-2)
12dcb76 feat: short -a flag and positional agent argument for run (story 12-1)
45f59db docs: refine UX for --fetch operation on bmad repos
60fef54 docs: flush future work into PRD and sprint
```

**Takeaways:**
- 14.1 (commit `dbfe1e7`) is the immediate predecessor. Read its diff before starting — every pattern it established applies here
- The agent-instructions placeholder was left intentionally by 14.1 (see 14.1's review note about the placeholder phrasing). Task 5.1-5.2 replaces it — if the placeholder is still present in the file when this story begins, that is expected, not a merge conflict
- No infrastructure cleanup or retrofit is bundled into this story. If apt-install discipline needs refactoring later (e.g. pinning base-image packages too), that is a separate story

### Architecture Compliance Pointers

- **FR63 (PRD line 88):** Fully addressed by this story ("Sandbox image includes pre-installed code exploration tools at pinned versions: ripgrep, fd, ast-grep, universal-ctags")
- **NFR16 (PRD line 111):** "Pre-installed DevOps and exploration tools use explicit pinned versions declared in a single place in `embed/Dockerfile.tmpl`. Version bumps trigger a content-hash rebuild" — the header comment block (extended in Task 1) remains the single source of truth. The exploration block's version strings are also literals in the RUN lines — both must be bumped together per the update procedure
- **Dockerfile generation (architecture.md:183):** Template is an embedded asset. No runtime config affects exploration-tool install — purely static render, same as 14.1
- **Content-hash scoping (architecture.md:243):** Rendered Dockerfile + embedded scripts are hash inputs. Version bumps flow through automatically. Zero new hash-input logic

### File-Change Summary

| File | Change | Why |
|---|---|---|
| `embed/Dockerfile.tmpl` | Edit — extend header comment, add `exploration_tools` RUN block immediately after `validation_tools` | Primary deliverable (Tasks 1-2) |
| `embed/agent-instructions.md.tmpl` | Edit — replace "Code Exploration" placeholder subsection with prescriptive tool list | AC#8 (Task 5) |
| `internal/template/render_test.go` | Edit — add ~6 new regression tests; optionally extend `TestRender_validationToolsNoDynamicLatest` to cover both blocks | Lock invariants (Task 3) |
| `integration/toolchain_test.go` | Edit — add `TestToolchain_codeExploration`, `TestToolchain_rgRespectsGitignore`, `TestToolchain_fdRespectsGitignore`, `TestToolchain_astGrepStructuralMatch` | Container-side smoke tests (Task 4) |
| `_bmad-output/implementation-artifacts/smoke-test-checklist.md` | Edit — append "Story 14.2" section | Operational hygiene (Task 6) |
| `_bmad-output/implementation-artifacts/sprint-status.yaml` | Edit (by workflow) — `14-2-*` → `ready-for-dev`, then `review` after code-review, then `done` | Sprint tracking |

**No changes to:** `cmd/*`, `internal/config/*`, `internal/docker/*`, `internal/hash/*`, `internal/mount/*`, `internal/gitfetch/*`, `embed/embed.go`, `embed/entrypoint.sh`, `embed/git-wrapper.sh`, `embed/healthcheck-poller.sh`, `go.mod`, `go.sum`. If you find yourself editing any of these, stop and re-read the scope — something has drifted.

### Exit Code Impact

None. This story adds zero new error paths. All failures surface through existing `docker.BuildError` (build-time image creation) and `docker.RunError` (runtime tool invocation). No new error types, no `exitCode()` table updates.

### Content Hash Impact

Rendered Dockerfile changes ⇒ hash changes ⇒ cache miss ⇒ full rebuild. This is correct per NFR12, NFR16 and architecture.md:349. Existing cached images from before this story do NOT auto-invalidate on binary upgrade (they still work, just without the new toolchain) — users upgrade by running `asbox build` or `asbox run --no-cache` after pulling the new `asbox` binary. Matches 14.1's impact exactly.

### CLAUDE.md Compliance Checklist

- [ ] **Error handling:** No new error types. No `exitCode()` changes. Zero uses of bare `==` for errors (this story does not touch Go error-comparison code at all)
- [ ] **Testing:** Table-driven where pure function-ish (`TestToolchain_codeExploration`, `TestRender_explorationToolsPinnedVersions`). `t.TempDir()` not needed — integration tests create container-side temp dirs via `mktemp -d` inside `bash -lc` heredocs (matches the `TestToolchain_helmTemplateOffline` pattern from 14.1). `t.Cleanup` not `defer`. Stdlib `testing` only — no testify
- [ ] **Code organization:** Embedded assets live in `embed/` with `//go:embed` directives in `embed/embed.go` — no new `//go:embed` lines needed (`Dockerfile.tmpl` and `agent-instructions.md.tmpl` are already covered by the existing directive)
- [ ] **Agent registry invariant:** Untouched. Exploration tooling is agent-agnostic, matching 14.1
- [ ] **Import alias:** `asboxEmbed` alias not needed in new tests — `integration/toolchain_test.go` already exists and already imports what it needs via `buildTestImage(t)`

### Source Hints for Fast Navigation

| Artifact | Path | Relevant Lines |
|---|---|:---:|
| Dockerfile template | `embed/Dockerfile.tmpl` | 1-98 (comment header), 118-173 (validation_tools block — insertion point for exploration_tools is at line 174, immediately before `{{- if .SDKs.NodeJS}}`) |
| Agent instructions template | `embed/agent-instructions.md.tmpl` | 35-37 (Code Exploration placeholder — replace) |
| Embed directive | `embed/embed.go` | 5 (already covers both template files) |
| Template render | `internal/template/render.go` | 24-42 (Render function — no changes) |
| Existing validation-tools tests | `internal/template/render_test.go` | 460-603 (validation_tools test suite — extend-after pattern for Task 3) |
| Integration test scaffold | `integration/integration_test.go` | 26-74 (`buildTestImage`), 78-102 (`startTestContainer`), 124-137 (`execAsUser`) |
| Validation-tools integration tests | `integration/toolchain_test.go` | 10-54 (`TestToolchain_devopsValidation` — pattern reference), 56-94 (`TestToolchain_cacheDirsOwnedBySandbox`), 96-173 (`TestToolchain_helmTemplateOffline` — heredoc pattern for Task 4.2-4.4) |
| Content hash | `internal/hash/hash.go` | entire (read-only for this story — no changes) |
| Architecture decision | `_bmad-output/planning-artifacts/architecture.md` | 340-350 |
| FR63 / NFR16 | `_bmad-output/planning-artifacts/prd.md` | 88, 111 |
| Epic story | `_bmad-output/planning-artifacts/epics.md` | 1749-1786 |
| Predecessor story (same epic) | `_bmad-output/implementation-artifacts/14-1-pre-installed-devops-validation-toolchain.md` | entire (pattern reference — tasks, tests, agent-instructions section) |
| Story 13.2 (prior feature story) | `_bmad-output/implementation-artifacts/13-2-fetch-flag-for-upstream-sync.md` | entire (test naming, commit convention) |
| Smoke-test checklist | `_bmad-output/implementation-artifacts/smoke-test-checklist.md` | 79-83 (Story 14.1 section — append after) |

### Testing Standards Summary

- **Unit tests** (`internal/template/render_test.go`): fast, pure, run on every push. No container builds. Table-driven where it fits. Substring-level assertions against rendered Dockerfile content. Assertions lock both positive content (versions present, install paths correct) and negative invariants (no `api.github.com`, no `| bash`)
- **Integration tests** (`integration/toolchain_test.go`): build the image once per test file via `buildTestImage(t)`, start one container, run parallel subtests via `t.Run(..., t.Parallel())`. All container commands go through `execAsUser(ctx, t, container, "sandbox", cmd)` to exercise real user-level access
- **No mocks for Docker/Podman:** integration tests hit real Docker. If the machine lacks Docker, `testing.Short()` skip prevents false failures — every integration test already checks this
- **Version-string assertions in integration tests are DELIBERATELY version-specific** (Task 4.1) so a silent version drift between the Dockerfile and reality breaks the test loudly

### Project Structure Notes

- **Alignment:** All changes are within existing file boundaries. No new files are created (`integration/toolchain_test.go` was created by 14.1; this story appends test functions to it). Matches the "additive" shape of 14.1
- **No new Go packages.** No new module dependencies (`go.mod`/`go.sum` untouched). No new `//go:embed` declarations
- **Detected conflicts:** None. The story is additive and the agent-instructions placeholder was left specifically for this story by 14.1's dev agent

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic-14 - Story 14.2 acceptance criteria and implementation notes (lines 1749-1786)]
- [Source: _bmad-output/planning-artifacts/architecture.md#Pre-Installed-Validation-Exploration-Toolchain - decision record (lines 340-350)]
- [Source: _bmad-output/planning-artifacts/architecture.md#Dockerfile-Generation - multi-arch support rules (line 193)]
- [Source: _bmad-output/planning-artifacts/architecture.md#Content-Hash-Caching - hash input definition (lines 242-247)]
- [Source: _bmad-output/planning-artifacts/prd.md - FR63 (line 88), NFR16 (line 111)]
- [Source: embed/Dockerfile.tmpl - header comment block (lines 1-98), validation_tools block (lines 118-173), insertion point at line 174]
- [Source: embed/agent-instructions.md.tmpl - Installed Tooling > Code Exploration placeholder (lines 35-37)]
- [Source: internal/template/render_test.go - validation_tools test suite (lines 460-603), pattern to mirror for exploration_tools tests]
- [Source: integration/integration_test.go - buildTestImage (line 26), startTestContainer (line 78), execAsUser (line 124)]
- [Source: integration/toolchain_test.go - TestToolchain_devopsValidation (lines 10-54), TestToolchain_helmTemplateOffline heredoc pattern (lines 96-173)]
- [Source: _bmad-output/implementation-artifacts/14-1-pre-installed-devops-validation-toolchain.md - full pattern reference for this sibling story]
- [Source: _bmad-output/implementation-artifacts/11-5-pinned-build-dependencies.md - original pinning discipline and comment-header pattern]

## Dev Agent Record

### Agent Model Used

gpt-5.4

### Debug Log References

- `env GOCACHE=/tmp/asbox-go-build go test ./internal/hash/ -v`
- `env GOCACHE=/tmp/asbox-go-build go test ./internal/template/ -v`
- `env GOCACHE=/tmp/asbox-go-build go test ./cmd/ -v`
- `env GOCACHE=/tmp/asbox-go-build go vet ./...`
- `env GOCACHE=/tmp/asbox-go-build gofmt -l .`
- `env GOCACHE=/tmp/asbox-go-build go test -v ./integration -count=1 -timeout 30m`
- `env GOCACHE=/tmp/asbox-go-build go test ./...`

### Completion Notes List

- Added an unconditional `exploration_tools` Docker layer with pinned `ripgrep`, `fd-find`, `universal-ctags`, and `ast-grep`, including the `fd` symlink and the dpkg-to-uname arch mapping for the `ast-grep` asset.
- Extended the Dockerfile maintenance header with pinned exploration-tool versions and an exploration-tools update procedure; confirmed `ast-grep` remained at `0.42.1`.
- Replaced the Code Exploration placeholder in `embed/agent-instructions.md.tmpl` with prescriptive guidance for `rg`, `fd`, `ast-grep`, `ctags`, and `git ls-files`.
- Added render tests for the exploration block, pinned versions, unconditional rendering, symlink creation, and no-dynamic-latest invariants.
- Added integration coverage for version probes, `.gitignore` behavior for `rg` and `fd`, and structural matching for `ast-grep`.
- Verified the full repo test gates pass. Note: Ubuntu's `fd-find` package prints `fdfind 9.0.0` for `fd --version`, so the integration assertion locks the real package output while still validating the `fd` command path and pinned version.

### File List

- embed/Dockerfile.tmpl
- embed/agent-instructions.md.tmpl
- internal/template/render_test.go
- integration/toolchain_test.go
- _bmad-output/implementation-artifacts/smoke-test-checklist.md
- _bmad-output/implementation-artifacts/sprint-status.yaml
- _bmad-output/implementation-artifacts/14-2-pre-installed-code-exploration-tools.md

### Change Log

- 2026-04-20: Implemented story 14.2 code exploration toolchain, tests, instructions, and status updates.
