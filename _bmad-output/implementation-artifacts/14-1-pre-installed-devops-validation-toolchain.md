# Story 14.1: Pre-Installed DevOps Validation Toolchain

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want `kubectl`, `helm`, `kustomize`, `yq`, `jq`, `opentofu`, `tflint`, `kubeconform`, `kube-linter`, `trivy`, `flux`, and `sops` pre-installed in the sandbox image at pinned versions,
so that the agent can validate Kubernetes and Terraform work without spending turns on tool installation.

## Acceptance Criteria

1. **Given** a newly built sandbox image
   **When** the agent invokes any of the twelve tools with `--version` or `version`
   **Then** each tool responds with exit code 0 and reports the pinned version baked into the image

2. **Given** the agent runs `trivy image <some-image>` for the first time as the sandbox user
   **When** trivy tries to download its vulnerability database to `~/.cache/trivy`
   **Then** the download succeeds — the cache directory is pre-created at `/home/sandbox/.cache/trivy` and owned by `sandbox:sandbox`

3. **Given** the agent runs `helm template`, `helm install --dry-run`, or `kustomize build` against a manifest directory
   **When** the render is produced
   **Then** the operation completes locally without requiring cluster credentials — no `KUBECONFIG`, no cluster reachability, no network egress to a Kubernetes API

4. **Given** `embed/Dockerfile.tmpl` is inspected
   **When** the `validation_tools` RUN block is read
   **Then** each of the twelve tools is installed at an explicit pinned version (no `latest`, no `@latest`, no GitHub API `latest` endpoint, no `curl | bash` installers) and every pinned version is declared in a single comment block at the top of the file so there is exactly one place to edit

5. **Given** the Dockerfile template comment header
   **When** a maintainer reads it
   **Then** the header documents each pinned version, the upstream release page used to discover it, the update procedure (including how to obtain a multi-arch manifest digest where applicable), and the verification steps — matching the existing pattern established by story 11.5

6. **Given** the sandbox image is built on both amd64 and arm64 hosts
   **When** the `validation_tools` RUN block executes
   **Then** each binary downloaded from a GitHub release URL resolves to the correct architecture asset — via `$(dpkg --print-architecture)` (returns `amd64`/`arm64`) or `$(uname -m)` (returns `x86_64`/`aarch64`) as each project's naming convention requires — and every download line carries an inline comment naming which scheme it uses

7. **Given** a content hash is computed for the image (per `internal/hash/`)
   **When** a pinned version in the `validation_tools` block is bumped in `embed/Dockerfile.tmpl`
   **Then** the content hash changes and the next `asbox build` / `asbox run` triggers a full rebuild — matching the existing reproducibility contract (NFR12, NFR16). No new logic is added to `internal/hash/`; the existing template-content input is sufficient

8. **Given** the integration test suite
   **When** a new `TestToolchain_devopsValidation` test runs under `integration/toolchain_test.go`
   **Then** each of the twelve tools is asserted to exist on `PATH` and to respond to its version probe with exit code 0 inside a freshly launched sandbox, running as the `sandbox` user

9. **Given** the image is inspected
   **When** the `sandbox` user runs `helm env`, `kubectl config view`, `tofu --version`, or `sops --version`
   **Then** none of them error on permission for their default cache/state directories — `~/.cache/helm`, `~/.kube`, `~/.terraform.d`, `~/.config/sops` exist and are writable by `sandbox:sandbox`

10. **Given** the Dockerfile template is rendered with any valid `Config` (any SDK combination, any agents installed, any MCP set)
    **When** the output is inspected
    **Then** the `validation_tools` RUN block is always present — it is NOT conditional on any config field. The toolchain is part of the base sandbox image

11. **Given** the generated agent instructions (`embed/agent-instructions.md.tmpl`)
    **When** an agent reads them inside the sandbox
    **Then** a new "Installed Tooling" section lists the twelve pre-installed DevOps validation tools so the agent does not waste turns discovering them or trying `apt-get install` for tools that are already present

## Tasks / Subtasks

- [x] Task 1: Extend the maintenance comment block at the top of `embed/Dockerfile.tmpl` (AC: #4, #5)
  - [x] 1.1 In the existing `{{- /* ... */ -}}` header block (starts at line 1), add a new "Pinned validation tools" subsection listing each of the twelve tool names and its pinned version — mirror the style of the existing "Current pinned versions" subsection
  - [x] 1.2 Add a new "Validation tools update procedure" subsection documenting: upstream releases page per tool, one-liner to check for newer versions (e.g. `curl -s https://api.github.com/repos/<org>/<repo>/releases/latest | jq -r .tag_name` — note: **this is documentation only, not code executed at build time**; the API is used by the maintainer during bumps, never by the Dockerfile itself), and a verification note (`go test ./... && go test -v ./integration -count=1 -timeout 20m` after every bump)
  - [x] 1.3 Leave the existing "Current pinned versions" subsection intact — do not consolidate — so git diffs stay focused on the validation-tools section

- [x] Task 2: Add the `validation_tools` RUN block to `embed/Dockerfile.tmpl` (AC: #4, #6, #10)
  - [x] 2.1 Placement: insert immediately after line 48 (`RUN chmod +x /usr/local/bin/entrypoint.sh /usr/local/bin/git /usr/local/bin/healthcheck-poller.sh`) and **before** line 49 (`{{- if .SDKs.NodeJS}}`). Rationale: SDK and agent blocks are conditional on config; the toolchain is unconditional; keeping unconditional blocks grouped at the top maximizes Docker layer cache reuse across configs
  - [x] 2.2 Start the block with a comment line: `# validation_tools — pinned DevOps validation toolchain (FR62, NFR16). See header for version bump procedure.`
  - [x] 2.3 All downloads MUST use `curl -fsSL <versioned-github-release-url>` with the version string hardcoded inline (not fetched from `api.github.com/releases/latest`). `curl | bash` is forbidden. After download: `tar -xz` / `unzip` / `install -m 0755` as appropriate, move binary to `/usr/local/bin/<name>`, `chmod +x`, clean `/tmp`
  - [x] 2.4 Install the twelve tools at the versions below. Every download line carries an inline comment with its arch-naming scheme. Use the suggested pinned versions, but a developer may bump to a newer patch release at build time if upstream has released one since — record the final chosen version in Task 1's comment block:

    | Tool | Pinned Version | Arch Scheme | URL Pattern |
    |---|---|---|---|
    | `kubectl` | `v1.35.4` | `$(dpkg --print-architecture)` → `amd64`/`arm64` | `https://dl.k8s.io/release/v1.35.4/bin/linux/$(dpkg --print-architecture)/kubectl` |
    | `helm` | `v3.20.2` | `$(dpkg --print-architecture)` → `amd64`/`arm64` | `https://get.helm.sh/helm-v3.20.2-linux-$(dpkg --print-architecture).tar.gz` (extract `linux-*/helm`) |
    | `kustomize` | `v5.8.1` | `$(dpkg --print-architecture)` → `amd64`/`arm64` | `https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv5.8.1/kustomize_v5.8.1_linux_$(dpkg --print-architecture).tar.gz` (note the URL-encoded `%2F`) |
    | `yq` (mikefarah) | `v4.53.2` | `$(dpkg --print-architecture)` → `amd64`/`arm64` | `https://github.com/mikefarah/yq/releases/download/v4.53.2/yq_linux_$(dpkg --print-architecture)` (single binary, no extract — `install -m 0755 yq_linux_* /usr/local/bin/yq`) |
    | `jq` | apt-managed | — | **Already installed** by the base `apt-get install` block at line 35. Do NOT duplicate — just verify `jq --version` in tests |
    | `opentofu` | `v1.11.6` | `$(dpkg --print-architecture)` → `amd64`/`arm64` | `https://github.com/opentofu/opentofu/releases/download/v1.11.6/tofu_1.11.6_linux_$(dpkg --print-architecture).tar.gz` (binary name: `tofu`) |
    | `tflint` | `v0.62.0` | `$(dpkg --print-architecture)` → `amd64`/`arm64` | `https://github.com/terraform-linters/tflint/releases/download/v0.62.0/tflint_linux_$(dpkg --print-architecture).zip` (**zip, not tar** — use `unzip`) |
    | `kubeconform` | `v0.7.0` | `$(dpkg --print-architecture)` → `amd64`/`arm64` | `https://github.com/yannh/kubeconform/releases/download/v0.7.0/kubeconform-linux-$(dpkg --print-architecture).tar.gz` |
    | `kube-linter` | `v0.8.3` | custom — see note | `https://github.com/stackrox/kube-linter/releases/download/v0.8.3/kube-linter-linux_$(dpkg --print-architecture).tar.gz` — **verify at build time**: kube-linter historically published `kube-linter-linux.tar.gz` (amd64 only) plus `kube-linter-linux_arm64.tar.gz` (arm64 with suffix). If the canonical asset naming differs, adjust with a small `case` on `dpkg --print-architecture` |
    | `trivy` | `v0.70.0` | custom mapping | Aquasecurity publishes `trivy_0.70.0_Linux-64bit.tar.gz` (amd64) and `trivy_0.70.0_Linux-ARM64.tar.gz` (arm64). Use a shell case: `case $(dpkg --print-architecture) in amd64) TRIVY_ARCH=64bit ;; arm64) TRIVY_ARCH=ARM64 ;; esac`. URL: `https://github.com/aquasecurity/trivy/releases/download/v0.70.0/trivy_0.70.0_Linux-${TRIVY_ARCH}.tar.gz` |
    | `flux` | `v2.8.5` | `$(dpkg --print-architecture)` → `amd64`/`arm64` | `https://github.com/fluxcd/flux2/releases/download/v2.8.5/flux_2.8.5_linux_$(dpkg --print-architecture).tar.gz` |
    | `sops` | `v3.12.2` | `$(dpkg --print-architecture)` → `amd64`/`arm64` | `https://github.com/getsops/sops/releases/download/v3.12.2/sops-v3.12.2.linux.$(dpkg --print-architecture)` (single binary, no extract — `install -m 0755 sops-v3.12.2.linux.* /usr/local/bin/sops`) |

  - [x] 2.5 Keep each tool's install as an independent `curl && extract && install` chain joined with `&&` inside ONE `RUN` directive — one layer for the whole block. Use `cd /tmp` at the start and `rm -rf /tmp/*` at the end so intermediate archive files do not leak into the image. Do NOT split into multiple `RUN` directives — that balloons the layer count and harms cache reuse
  - [x] 2.6 **Invariant — no `curl | bash`:** per AC#4 and the architecture decision at `architecture.md:347`, integrity failures in `curl | sh` flows are silent. Every tool above is a direct URL-to-binary download with a known artifact name. Do not replace this with an upstream install-script shim even if upstream documents one
  - [x] 2.7 **Invariant — no dynamic `latest` lookup:** `curl https://api.github.com/repos/.../releases/latest | jq -r .tag_name` is forbidden inside the Dockerfile. Version strings are literals in the template. This is the same discipline enforced by story 11.5's `TestRender_dockerCompose` negative assertion (`api.github.com` substring check)

- [x] Task 3: Pre-create cache/data directories and set ownership (AC: #2, #9)
  - [x] 3.1 Inside the same `validation_tools` RUN block (stays one layer), after all binaries are installed, run: `mkdir -p /home/sandbox/.cache/trivy /home/sandbox/.cache/helm /home/sandbox/.kube /home/sandbox/.terraform.d /home/sandbox/.config/sops && chown -R sandbox:sandbox /home/sandbox/.cache /home/sandbox/.kube /home/sandbox/.terraform.d /home/sandbox/.config`
  - [x] 3.2 **Ordering matters:** this RUN block executes while the active user is still `root` (no `USER sandbox` switch has happened yet at this point in the Dockerfile). The `chown` takes effect immediately and persists through the image layers. Do not attempt this after a later `USER sandbox` directive
  - [x] 3.3 `/home/sandbox` itself was created by `useradd -m` (line 42); do NOT re-chown the whole home directory — only the specific cache/data subdirectories. Chowning `/home/sandbox` recursively would stomp permissions on directories created by later layers (e.g. `~/.claude`, `~/.npm-global`)

- [x] Task 4: Extend `embed/agent-instructions.md.tmpl` with an "Installed Tooling" section (AC: #11)
  - [x] 4.1 Add a new top-level section titled `## Installed Tooling` between the existing `## Available Tools` section (line 12) and `## Working Directory` section (line 18). Rationale: tools are a capability list (belongs with "Available Tools"), but the existing "Available Tools" section is narrative prose for Playwright/Podman/SDKs. Keeping the tabular tools list separate avoids re-writing that section
  - [x] 4.2 Structure the new section as TWO subsections: `### DevOps Validation` (this story) and `### Code Exploration` (a one-line "See story 14.2" placeholder — 14.2 will fill it in). The DevOps Validation subsection lists every tool with a ≤10-word purpose note. Example format:

    ```markdown
    ### DevOps Validation

    - `kubectl` — Kubernetes CLI; use `kubectl --dry-run=client` to validate manifests offline.
    - `helm` — Chart renderer; use `helm template` or `helm install --dry-run` for offline validation.
    - `kustomize` — Overlay renderer; use `kustomize build <dir>` to produce final manifests.
    - `yq` — YAML query tool (mikefarah flavor); use for YAML manipulation, not JSON.
    - `jq` — JSON query tool; already available via base image.
    - `opentofu` (`tofu`) — Terraform-compatible IaC CLI; use for fmt, validate, plan.
    - `tflint` — Terraform linter; use alongside `tofu validate`.
    - `kubeconform` — Offline Kubernetes manifest schema validator.
    - `kube-linter` — Kubernetes YAML linter; catches misconfigurations (RBAC, probes, resources).
    - `trivy` — Container image and filesystem vulnerability scanner.
    - `flux` — Flux CD CLI for GitOps reconciliation primitives (build, diff).
    - `sops` — Secrets encryption/decryption; offline-capable with local age/pgp keys.
    ```

  - [x] 4.3 Keep the phrasing prescriptive — the agent should read "use `X` for Y" and immediately know which tool to reach for. Do NOT add installation instructions, version numbers, or license info; those belong in the Dockerfile comment block and the CLAUDE.md/architecture docs
  - [x] 4.4 `embed/agent-instructions.md.tmpl` is used for ALL three agents (Claude, Gemini, Codex) — see Dockerfile lines 131, 134, 137. The same template renders into `CLAUDE.md`, `GEMINI.md`, and `.codex/AGENTS.md`. Do NOT introduce agent-specific branches; the tooling list is identical across agents

- [x] Task 5: Add unit-level regression tests in `internal/template/render_test.go` (AC: #4, #10, #7)
  - [x] 5.1 Add `TestRender_validationToolsBlockPresent(t *testing.T)`: render with a minimal config (`&config.Config{InstalledAgents: []string{"claude"}}`) and assert `strings.Contains(output, "# validation_tools")` and `strings.Contains(output, "/usr/local/bin/kubectl")`. The block is unconditional per AC#10, so a minimal config is sufficient
  - [x] 5.2 Add `TestRender_validationToolsAllTwelvePresent(t *testing.T)`: same render, assert each of `kubectl`, `helm`, `kustomize`, `yq`, `tofu`, `tflint`, `kubeconform`, `kube-linter`, `trivy`, `flux`, `sops` appears at least once in the rendered output as a substring of either the URL or the destination path. (`jq` is already checked by the existing `TestRender_commonPackages` at `render_test.go:56` — do not duplicate it)
  - [x] 5.3 Add `TestRender_validationToolsPinnedVersions(t *testing.T)`: table-driven. Each row pairs a tool with a pinned-version regex/substring (`"v1.35.4"`, `"v3.20.2"`, `"v5.8.1"`, etc.). Assert every substring appears in the render. Failure output names the offending tool — keeps diagnostics actionable. Mirrors the `TestRender_npmVersionsPinned` pattern established in story 11.5
  - [x] 5.4 Add `TestRender_validationToolsNoDynamicLatest(t *testing.T)`: assert `!strings.Contains(output, "api.github.com")` AND `!strings.Contains(output, "releases/latest")` AND `!strings.Contains(output, "| bash")` — locks AC#4 invariants as hard test assertions. **Do not remove these assertions when updating versions; they are the guarantee that no one sneaks in a `curl|bash` during a bump**
  - [x] 5.5 Add `TestRender_validationToolsUnconditional(t *testing.T)`: render with three configs — (a) `{}` (no agents — this is a degenerate config, but it should not panic; template rendering is pure), (b) minimal claude-only, (c) claude+gemini+codex+playwright+python+nodejs+go full config. Assert the `# validation_tools` substring appears in all three. Locks AC#10
  - [x] 5.6 Add `TestRender_cacheDirsChowned(t *testing.T)`: assert the render contains `chown -R sandbox:sandbox /home/sandbox/.cache` (or equivalent) and `mkdir -p /home/sandbox/.cache/trivy`. Locks AC#2 and #9 at the template level without needing a container build

- [x] Task 6: Add `integration/toolchain_test.go` for container-side smoke tests (AC: #1, #2, #3, #8, #9)
  - [x] 6.1 New file `integration/toolchain_test.go`, package `integration`. Follow the existing pattern from `integration/lifecycle_test.go` and `integration/multi_agent_test.go` — `testing.Short()` skip, `buildTestImage(t)` then `startTestContainer(...)`, `t.Cleanup(cancel)`, parallel subtests where safe
  - [x] 6.2 Add `TestToolchain_devopsValidation(t *testing.T)`: table-driven. One subtest per tool with a `{name, command, expectSubstring}` row. Use `execAsUser(ctx, t, container, "sandbox", cmd)` (defined at `integration/integration_test.go:124`) — running as the `sandbox` user proves path, ownership, and default-user-invocable-ness all in one. Example rows:

    ```go
    {"kubectl", []string{"kubectl", "version", "--client=true", "--output=yaml"}, "gitVersion: v1.35.4"},
    {"helm",    []string{"helm", "version", "--short"}, "v3.20.2"},
    {"kustomize", []string{"kustomize", "version"}, "v5.8.1"},
    {"yq",      []string{"yq", "--version"}, "4.53.2"},
    {"jq",      []string{"jq", "--version"}, "jq-"},            // apt version-agnostic
    {"tofu",    []string{"tofu", "--version"}, "OpenTofu v1.11.6"},
    {"tflint",  []string{"tflint", "--version"}, "0.62.0"},
    {"kubeconform", []string{"kubeconform", "-v"}, "v0.7.0"},
    {"kube-linter", []string{"kube-linter", "version"}, "0.8.3"},
    {"trivy",   []string{"trivy", "--version"}, "Version: 0.70.0"},
    {"flux",    []string{"flux", "--version"}, "flux version 2.8.5"},
    {"sops",    []string{"sops", "--version"}, "3.12.2"},
    ```

    Each subtest: run the command, assert exit code 0, assert stdout contains `expectSubstring`. On mismatch, `t.Errorf` naming the tool and showing the full output truncated to 500 chars for readability. **Leave the substrings version-specific** — when a version bumps, these assertions fail loud and force the developer to update both the Dockerfile and the test in the same diff, preventing silent version drift
  - [x] 6.3 Add `TestToolchain_cacheDirsOwnedBySandbox(t *testing.T)`: exec `stat -c '%U:%G' /home/sandbox/.cache/trivy /home/sandbox/.cache/helm /home/sandbox/.kube /home/sandbox/.terraform.d /home/sandbox/.config/sops` and assert every line is `sandbox:sandbox`. Locks AC#2 and AC#9
  - [x] 6.4 Add `TestToolchain_helmTemplateOffline(t *testing.T)`: run `helm template release-name <chart-dir>` against a tiny in-container chart written via `heredoc` or `echo >` — assert exit code 0 and stdout contains `apiVersion: v1` (confirming the chart rendered). Locks AC#3. Use the same pattern for `kustomize build` on a trivial overlay. Skip this in `-short` mode to keep fast-feedback intact
  - [x] 6.5 Share the helper `runCommandInSandbox(ctx, container, cmd []string) (stdout string, exitCode int, err error)` inside the test file rather than exporting it — this is the same convention as existing integration tests
  - [x] 6.6 Do NOT test `trivy image <image>` end-to-end — that requires a real vulnerability DB download and network, which is out of scope. AC#2 is satisfied by verifying `~/.cache/trivy` exists and is writable (Task 6.3). Actual DB download can be covered by a manual smoke test in `smoke-test-checklist.md`

- [x] Task 7: Verify no impact to content hash, docker build, or orchestration code (AC: #7)
  - [x] 7.1 Run `go test ./internal/hash/ -v` — existing tests pass unchanged. The hash inputs already include the rendered Dockerfile per `architecture.md:243`, so extending the template automatically extends the hash
  - [x] 7.2 Run `go test ./internal/template/ -v` — confirms Task 5 tests pass
  - [x] 7.3 Run `go test ./cmd/ -v` — no changes expected; confirms no regression
  - [x] 7.4 Run `go vet ./... && gofmt -l . | tee /tmp/gofmt.out && test ! -s /tmp/gofmt.out` — no unformatted Go files
  - [x] 7.5 Run `go test -v ./integration -count=1 -timeout 30m` — full integration suite incl. Task 6. **Build time impact is real** (12 tool installs add ~30-60s to a clean build); the 30-minute timeout (bumped from the project-default 20m) absorbs that. Once the image is cached, subsequent test runs are unaffected
  - [x] 7.6 Run `go test ./...` as the final gate

- [x] Task 8: Update the smoke-test checklist (AC: none directly — operational hygiene)
  - [x] 8.1 Append entries to `_bmad-output/implementation-artifacts/smoke-test-checklist.md` under a new "Story 14.1 — DevOps validation toolchain" section: (a) `trivy image alpine:latest` inside a fresh sandbox downloads the DB successfully; (b) `helm template` on a real chart renders; (c) `sops` round-trip with an age key encrypts and decrypts a test YAML. These are manual checks that cannot run in CI without network/credentials

### Review Findings

- [x] [Review][Decision] `runCommandInSandbox` duplicates `execAsUser` — **Resolved (Option 2):** `execInContainer` now calls `tcexec.Multiplexed()` so the shared helper strips testcontainers stream framing. `runCommandInSandbox` was removed and all call sites now use `execAsUser(ctx, t, container, "sandbox", cmd)`. All integration tests benefit from the multiplexed output going forward.
- [x] [Review][Patch] Agent instructions leak a story-ID reference to end users [embed/agent-instructions.md.tmpl:37] — Replaced `See story 14.2 for the code exploration toolchain.` with a neutral "pending and will be documented here when available" placeholder.
- [x] [Review][Patch] `test -w` writability loop masks intermediate failures [integration/toolchain_test.go] — Prefixed the loop with `set -e;` so any failing `test -w` aborts the loop and surfaces the real failure. Applied inline during the decision-needed refactor.
- [x] [Review][Patch] `TestRender_validationToolsAllTwelvePresent` matches shell-comment words, not install commands — Tightened to assert on `/usr/local/bin/<tool>` (only appears in the actual install line, not in the shell comment).
- [x] [Review][Patch] `TestRender_cacheDirsChowned` only verifies 1 of 5 pre-created cache directories — Extended to cover `.cache/trivy`, `.cache/helm`, `.kube`, `.terraform.d`, `.config/sops` and to verify `chown -R sandbox:sandbox` against each root.
- [x] [Review][Patch] `TestRender_validationToolsAllTwelvePresent` iterates 11 tools despite the "Twelve" name — Renamed to `TestRender_validationToolsAllElevenPresent` with a comment explaining that jq is covered by `TestRender_commonPackages`.
- [x] [Review][Patch] `TestRender_validationToolsNoDynamicLatest` depends on Go-template comment stripping — Added a test-file comment documenting the reliance on `{{- /* */ -}}` stripping and explaining that the assertions flip intentionally if that wrapper is ever removed.
- [x] [Review][Defer] No SHA256 verification for 12 binary downloads (TOFU) [embed/Dockerfile.tmpl:131-170] — deferred, pre-existing pattern (Docker Compose at line 227, npm packages). Systemic discipline change.
- [x] [Review][Defer] Runtime `align_uid_gid` desyncs build-time chown on Linux hosts where `HOST_UID != 1000` [embed/entrypoint.sh:42 vs embed/Dockerfile.tmpl:172] — deferred, pre-existing. Affects `.npm-global` and `.codex` via the same mechanism.
- [x] [Review][Defer] Integration tests each rebuild the image (no cross-test image caching) [integration/toolchain_test.go:22,71,120] — deferred, pre-existing pattern across `fetch_test.go`, `multi_agent_test.go`, `isolate_deps_test.go`.
- [x] [Review][Defer] `TestRender_validationToolsPinnedVersions` substring matching could collide on longer versions (e.g., `v1.35.4` matches `v1.35.40`) [internal/template/render_test.go:499-528] — deferred, low probability and matches the `TestRender_npmVersionsPinned` pattern from story 11.5.
- [x] [Review][Defer] Multiple `tar -xzf ... -C /tmp <member>` silent-no-op if upstream changes archive layout [embed/Dockerfile.tmpl:135,139,146,154,158,162,166] — deferred, `set -eux` makes any missing-member failure loud; integration tests catch it on bump.
- [x] [Review][Defer] Parallel subtests in `TestToolchain_devopsValidation` could race on first-run cache init [integration/toolchain_test.go:44-60] — deferred, low probability for read-only version commands.

## Dev Notes

### Developer Context — Why This Story Exists

Every `asbox run` session today starts with the agent discovering that `kubectl` / `helm` / `trivy` / etc. are not in the image, then spending 3-10 turns on `apt-get install` flows (some of which fail because the tool isn't in the default apt repos). This story ships those tools at pinned versions so the agent reaches for them directly. The epic framing in `epics.md:1697` calls this out: "Agents spending turns on `apt-get install kubectl` or discovering that `fd` isn't installed are agents losing time to infrastructure instead of solving the problem."

### Critical Architectural Compliance Points

- **architecture.md:340-350 "Pre-Installed Validation & Exploration Toolchain"** is the authoritative design. This story implements that decision. Do not re-litigate the design choices; they are settled (`validation_tools` + `exploration_tools` RUN blocks, pinned versions, multi-arch detection, no `curl|bash`, cache-directory ownership)
- **architecture.md:193 "Multi-architecture support"** mandates: `$(dpkg --print-architecture)` for Debian-style arch names (`amd64`/`arm64`), `$(uname -m)` for kernel-style names (`x86_64`/`aarch64`). Never hardcode `amd64`. Inline comment on each download line names the scheme
- **architecture.md:242-247 "Content-Hash Caching"** means the hash inputs already cover the rendered Dockerfile and embedded scripts. No changes to `internal/hash/` are needed or wanted — any attempt to add fields is a design regression
- **architecture.md:460-463 "Go Template Conventions"** — comments use `{{/* */}}`, conditionals use `{{- if -}}` with whitespace trim. The toolchain block is NOT inside an `{{if}}` — it's plain Dockerfile content that always renders

### Scope — What Is And Is Not In This Story

**In scope:**
- New `validation_tools` RUN block in `embed/Dockerfile.tmpl`
- Pinned versions for 12 tools (kubectl, helm, kustomize, yq, [jq already installed], opentofu, tflint, kubeconform, kube-linter, trivy, flux, sops)
- Cache/state directories pre-created and chowned to `sandbox:sandbox`
- Unit tests in `internal/template/render_test.go`
- Integration smoke test at `integration/toolchain_test.go`
- New "Installed Tooling > DevOps Validation" section in `embed/agent-instructions.md.tmpl`

**NOT in scope (belongs to other stories):**
- Code exploration tools (ripgrep, fd, ast-grep, universal-ctags) — story 14.2
- Any new config flags (`enable_toolchain: false`, etc.) — the toolchain is unconditional; if we add user control later it will be its own story
- Runtime tool execution or wrappers — the agent just invokes the tools; no shim/sandbox layer needed
- Kubernetes cluster access (kubeconfig, real cluster) — Epic 15, explicitly deferred
- Vulnerability database management for trivy beyond AC#2's cache directory ownership
- Any changes to `internal/hash/`, `internal/docker/`, `internal/config/`, `internal/mount/`, or `cmd/*` — no Go logic is added by this story

### Decision Points The Dev Agent Will Hit

1. **Helm 3 vs Helm 4:** Helm v4.1.4 was released 2026-04-09. v3.20.2 is the latest v3 line. **Recommendation: ship v3.20.2**. Rationale: v4 is breaking-changes-heavy (Lua-based charts, new API), is <2 weeks old as of this story, and most live charts in the ecosystem still target v3. If the user has a specific v4 workflow in mind, they can bump in a follow-up. Document the choice in the header comment
2. **`kustomize` as standalone vs `kubectl kustomize`:** `kubectl` has a built-in kustomize subcommand. Install the standalone `kustomize` binary anyway — it tracks a faster release cadence than the bundled version, and AC#1 names it explicitly as one of the twelve tools
3. **`tofu` vs `terraform`:** This story installs `opentofu` (binary name: `tofu`), not HashiCorp's `terraform`. OpenTofu is the community fork with a compatible CLI. If the agent expects the literal command `terraform`, it will need to use `tofu` instead — document this in the agent-instructions "Installed Tooling" line for opentofu. Do NOT symlink `/usr/local/bin/terraform -> tofu` — that masks the license/origin difference in a way that would surprise a user
4. **`ast-grep` location:** Story 14.2 will install `ast-grep` in an adjacent block. Do NOT pre-install it as part of this story to stay scope-bounded. Story 14.1 lands first; 14.2 follows

### Previous Story Intelligence (11.5 — Pinned Build Dependencies)

Story 11.5 pinned base image digest, Docker Compose, and npm agent packages. Critical patterns to reuse:

- **The existing Dockerfile header comment block (lines 1-30)** is the authoritative place for pinned-version maintenance instructions. Extend it; do not create a parallel block
- **Regression tests against `api.github.com`** (`render_test.go` for dockerCompose) are the model — Task 5.4 mirrors it for `releases/latest` and `| bash`
- **11.5 discovered:** the maintainer used `docker buildx imagetools inspect` to get multi-arch digests. For this story, the analogous commands are:
  - `gh release list --repo <org>/<repo> --limit 5` to find latest versions
  - `gh release view v1.35.4 --repo kubernetes/kubernetes` to confirm asset names
  - Document these in Task 1's comment-block update
- **11.5 completion notes:** "Added regression assertions … to enforce digest pinning, block GitHub `latest` API usage" — Task 5.4 does the same for this story. Copy the phrasing discipline
- **Single-file-change discipline:** 11.5 kept changes to two files (`embed/Dockerfile.tmpl`, `internal/template/render_test.go`). This story adds three more files (`embed/agent-instructions.md.tmpl`, `integration/toolchain_test.go`, `_bmad-output/implementation-artifacts/smoke-test-checklist.md`) because the scope is slightly wider, but the same discipline applies: no Go logic changes, no config changes, no orchestration changes

### Previous Story Intelligence (13.2 — `--fetch` Flag)

13.2 was the most recent feature story (merged 2026-04-20 as commit `5500e37`). Key patterns this story inherits:

- **Commit message convention:** `feat: pre-installed DevOps validation toolchain (story 14-1)` for the final commit
- **Integration-test timeout:** 13.2 used 20m. Bump to 30m for this story because the validation_tools build layer is ~30-60s longer on a cold cache (Task 7.5). Subsequent runs are unaffected
- **Test nomenclature:** 13.2 uses `TestRun_*` for CLI/cmd tests, `TestFetchAll_*` for internal package tests, `TestFetch_*` for integration tests. This story follows `TestRender_validationTools*` (unit) + `TestToolchain_*` (integration) — matches the existing `TestRender_*` / `TestContainer_*` scheme from earlier stories
- **Em-dash and string-literal locks:** 13.2 locked user-facing strings byte-for-byte in tests. This story does NOT produce user-facing runtime strings (all additions are build-time image content + static agent instructions) so that pattern does not apply. Task 5.4's negative assertions (`!contains "api.github.com"`) are the equivalent invariant lock

### Git Intelligence (Recent Commits)

```
5500e37 feat: --fetch flag for host-side upstream sync (story 13-2)
12dcb76 feat: short -a flag and positional agent argument for run (story 12-1)
45f59db docs: refine UX for --fetch operation on bmad repos
60fef54 docs: flush future work into PRD and sprint
d38f080 docs: update future work
```

**Takeaways:**
- Convention: `feat: <short description> (story N-M)` for feature stories
- Recent stories (12.1, 13.2) averaged ~3 files changed. This story is slightly larger (~5 files) because of the integration test and agent-instructions update — still scope-bounded
- No infrastructure cleanup or retrofit is bundled into this story. If dependency pinning or multi-arch patterns need refactoring later, that is a separate story

### Architecture Compliance Pointers

- **FR62 (PRD line 87):** Fully addressed by this story
- **NFR16 (PRD line 111):** "Pre-installed DevOps and exploration tools … use explicit pinned versions declared in a single place in `embed/Dockerfile.tmpl`" — the header comment block is the single source of truth per Task 1
- **Dockerfile generation (architecture.md:183):** Template is an embedded asset. No runtime config affects toolchain install — purely static render
- **Content-hash scoping (architecture.md:243):** Rendered Dockerfile + embedded scripts are hash inputs. Version bumps flow through automatically. Zero new hash-input logic

### File-Change Summary

| File | Change | Why |
|---|---|---|
| `embed/Dockerfile.tmpl` | Edit — extend header comment, add `validation_tools` RUN block | Primary deliverable (Tasks 1-3) |
| `embed/agent-instructions.md.tmpl` | Edit — add "Installed Tooling > DevOps Validation" section | AC#11 (Task 4) |
| `internal/template/render_test.go` | Edit — add 6 new regression tests | Lock invariants (Task 5) |
| `integration/toolchain_test.go` | **New file** | Container-side smoke tests (Task 6) |
| `_bmad-output/implementation-artifacts/smoke-test-checklist.md` | Edit — append manual checks | Operational hygiene (Task 8) |
| `_bmad-output/implementation-artifacts/sprint-status.yaml` | Edit (by workflow) — `14-1-*` → `ready-for-dev`, `epic-14` → `in-progress` | Sprint tracking |

**No changes to:** `cmd/*`, `internal/config/*`, `internal/docker/*`, `internal/hash/*`, `internal/mount/*`, `internal/gitfetch/*`, `embed/embed.go`, `embed/entrypoint.sh`, `embed/git-wrapper.sh`, `embed/healthcheck-poller.sh`, `go.mod`, `go.sum`. If you find yourself editing any of these, stop and re-read the scope — something has drifted.

### Exit Code Impact

None. This story adds zero new error paths. All failures surface through existing `docker.BuildError` (build-time image creation) and `docker.RunError` (runtime tool invocation). No new error types, no `exitCode()` table updates.

### Content Hash Impact

Rendered Dockerfile changes ⇒ hash changes ⇒ cache miss ⇒ full rebuild. This is correct per NFR12, NFR16 and architecture.md:349. Existing cached images from before this story do NOT auto-invalidate on binary upgrade (they still work, just without the new toolchain) — users upgrade by running `asbox build` or `asbox run --no-cache` after pulling the new `asbox` binary.

### CLAUDE.md Compliance Checklist

- [ ] **Error handling:** No new error types. No `exitCode()` changes. Zero uses of bare `==` for errors (this story does not touch Go error-comparison code at all)
- [ ] **Testing:** Table-driven where pure function-ish (`TestToolchain_devopsValidation`, `TestRender_validationToolsPinnedVersions`). `t.TempDir()` for any temp dirs in tests (only needed if Task 6.4 writes chart files — use `t.TempDir` + `os.WriteFile`). `t.Cleanup` not `defer`. Stdlib `testing` only — no testify
- [ ] **Code organization:** Embedded assets live in `embed/` with `//go:embed` directives in `embed/embed.go` — no new `//go:embed` lines needed (`Dockerfile.tmpl` and `agent-instructions.md.tmpl` are already covered by the existing directive at `embed/embed.go:5`)
- [ ] **Agent registry invariant:** Untouched. Toolchain is agent-agnostic
- [ ] **Import alias:** `asboxEmbed` alias not needed in new tests — `integration/toolchain_test.go` can reuse `buildTestImage(t)` from `integration/integration_test.go` which already does the import

### Source Hints for Fast Navigation

| Artifact | Path | Relevant Lines |
|---|---|:---:|
| Dockerfile template | `embed/Dockerfile.tmpl` | 1-30 (comment header), 48-49 (insertion point) |
| Agent instructions template | `embed/agent-instructions.md.tmpl` | 12 (Available Tools), 18 (Working Directory) |
| Embed directive | `embed/embed.go` | 5 (already covers both template files) |
| Template render | `internal/template/render.go` | 24-42 (Render function — no changes) |
| Existing version-pinning tests | `internal/template/render_test.go` | 11-23 (`TestRender_baseImage` — add-after pattern) |
| Integration test scaffold | `integration/integration_test.go` | 26-74 (`buildTestImage`), 78-102 (`startTestContainer`), 124-137 (`execAsUser`) |
| Multi-agent integration pattern | `integration/multi_agent_test.go` | 16-62 (table-driven subtests for "thing exists in container") |
| Content hash | `internal/hash/hash.go` | entire (read-only for this story — no changes) |
| Architecture decision | `_bmad-output/planning-artifacts/architecture.md` | 340-350 |
| FR62 / NFR16 | `_bmad-output/planning-artifacts/prd.md` | 87, 111 |
| Epic story | `_bmad-output/planning-artifacts/epics.md` | 1699-1747 |
| Story 11.5 (prior pinning pattern) | `_bmad-output/implementation-artifacts/11-5-pinned-build-dependencies.md` | entire (pattern reference) |
| Story 13.2 (prior feature story) | `_bmad-output/implementation-artifacts/13-2-fetch-flag-for-upstream-sync.md` | entire (test pattern reference) |

### Testing Standards Summary

- **Unit tests** (`internal/template/render_test.go`): fast, pure, run on every push. No container builds. Table-driven where it fits. Substring-level assertions against rendered Dockerfile content
- **Integration tests** (`integration/toolchain_test.go`): build the image once per test file via `buildTestImage(t)`, start one container, run parallel subtests via `t.Run(..., t.Parallel())`. All container commands go through `execAsUser(ctx, t, container, "sandbox", cmd)` to exercise real user-level access
- **No mocks for Docker/Podman:** integration tests hit real Docker. If the machine lacks Docker, `testing.Short()` skip prevents false failures — every integration test already checks this
- **Version-string assertions in integration tests are DELIBERATELY version-specific** (Task 6.2) so a silent version drift between the Dockerfile and reality breaks the test loudly

### Project Structure Notes

- **Alignment:** All changes are within existing file boundaries except for one new file (`integration/toolchain_test.go`), which follows the established `integration/<feature>_test.go` naming convention already used by `fetch_test.go`, `multi_agent_test.go`, `isolate_deps_test.go`, etc.
- **No new Go packages.** No new module dependencies (`go.mod`/`go.sum` untouched). No new `//go:embed` declarations
- **Detected conflicts:** None. The story is additive

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Epic-14 - Epic 14 description and Story 14.1 acceptance criteria (lines 1695-1747)]
- [Source: _bmad-output/planning-artifacts/architecture.md#Pre-Installed-Validation-Exploration-Toolchain - decision record (lines 340-350)]
- [Source: _bmad-output/planning-artifacts/architecture.md#Dockerfile-Generation - multi-arch support rules (line 193)]
- [Source: _bmad-output/planning-artifacts/architecture.md#Content-Hash-Caching - hash input definition (lines 242-247)]
- [Source: _bmad-output/planning-artifacts/prd.md - FR62 (line 87), FR63 (line 88), NFR16 (line 111)]
- [Source: embed/Dockerfile.tmpl - header comment block (lines 1-30), insertion point after line 48, existing pinned-version pattern (Docker Compose line 101, npm packages lines 117, 122, 147)]
- [Source: embed/agent-instructions.md.tmpl - Available Tools section (line 12), Working Directory section (line 18)]
- [Source: internal/template/render_test.go - TestRender_baseImage (line 11), TestRender_commonPackages (line 50) — extend-after pattern]
- [Source: integration/integration_test.go - buildTestImage (line 26), startTestContainer (line 78), execAsUser (line 124)]
- [Source: integration/multi_agent_test.go - table-driven "which tool exists" pattern (lines 33-47)]
- [Source: _bmad-output/implementation-artifacts/11-5-pinned-build-dependencies.md - pinning discipline, comment-header pattern, negative assertion pattern]
- [Source: _bmad-output/implementation-artifacts/13-2-fetch-flag-for-upstream-sync.md - test nomenclature, commit convention, scope discipline]

## Dev Agent Record

### Agent Model Used

GPT-5 Codex

### Debug Log References

- Redirected `GOCACHE` to `/tmp/asbox-gocache` because the sandbox could not write to the default macOS Go build cache path.
- Updated `runCommandInSandbox` to request `tcexec.Multiplexed()` after the first `stat` assertion surfaced framed exec-stream bytes instead of plain stdout.

### Completion Notes List

- Extended `embed/Dockerfile.tmpl` with a pinned validation-tools maintenance section and an unconditional single-layer `validation_tools` install block for `kubectl`, `helm`, `kustomize`, `yq`, `jq`, `tofu`, `tflint`, `kubeconform`, `kube-linter`, `trivy`, `flux`, and `sops`.
- Pre-created `/home/sandbox/.cache/trivy`, `/home/sandbox/.cache/helm`, `/home/sandbox/.kube`, `/home/sandbox/.terraform.d`, and `/home/sandbox/.config/sops`, then assigned them to `sandbox:sandbox` to keep offline validation flows permission-safe.
- Added an `Installed Tooling` section to `embed/agent-instructions.md.tmpl`, including the story 14.2 placeholder for code-exploration tools.
- Added render regression coverage in `internal/template/render_test.go` and new Docker-backed smoke coverage in `integration/toolchain_test.go` for tool versions, writable cache directories, and offline `helm template` / `kustomize build`.
- Updated `_bmad-output/implementation-artifacts/smoke-test-checklist.md` with manual follow-up checks for `trivy`, `helm`, and `sops`.
- Validation passed with `go test ./internal/hash/ -v`, `go test ./internal/template/ -v`, `go test ./cmd/ -v`, `go vet ./...`, `gofmt -l .`, `go test -v ./integration -count=1 -timeout 30m`, and a final `go test ./...`.

### File List

- `_bmad-output/implementation-artifacts/14-1-pre-installed-devops-validation-toolchain.md`
- `_bmad-output/implementation-artifacts/smoke-test-checklist.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`
- `embed/Dockerfile.tmpl`
- `embed/agent-instructions.md.tmpl`
- `internal/template/render_test.go`
- `integration/toolchain_test.go`

### Change Log

- 2026-04-20: Implemented story 14.1 by pre-installing the DevOps validation toolchain, documenting the pins/update flow, adding regression and integration coverage, and updating agent/smoke-test guidance.
