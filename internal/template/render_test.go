package template

import (
	"errors"
	"strings"
	"testing"

	"github.com/mcastellin/asbox/internal/config"
)

func TestRender_baseImage(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(output, "FROM ubuntu:24.04") {
		t.Errorf("expected output to start with Ubuntu 24.04 base image, got: %s", firstLine(output))
	}
	if !strings.Contains(output, "@sha256:") {
		t.Error("expected base image to be pinned to a sha256 digest")
	}
}

func TestRender_tiniInstalled(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "apt-get install") || !strings.Contains(output, "tini") {
		t.Error("expected output to contain apt-get install with tini")
	}
}

func TestRender_sandboxUserCreated(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "useradd") || !strings.Contains(output, "sandbox") {
		t.Error("expected output to contain useradd for sandbox user")
	}
	if !strings.Contains(output, "-u 1000") || !strings.Contains(output, "-g 1000") {
		t.Error("expected sandbox user to have UID/GID 1000")
	}
}

func TestRender_commonPackages(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	packages := []string{"curl", "wget", "dnsutils", "git", "jq", "unzip", "zip", "less", "vim", "ca-certificates", "gnupg", "lsb-release", "sudo", "build-essential", "tini"}
	for _, pkg := range packages {
		if !strings.Contains(output, pkg) {
			t.Errorf("expected output to contain package %q", pkg)
		}
	}
}

func TestRender_entrypoint(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/entrypoint.sh"]`
	if !strings.Contains(output, expected) {
		t.Errorf("expected output to contain ENTRYPOINT directive, got:\n%s", output)
	}
}

func TestRender_envVars(t *testing.T) {
	cfg := &config.Config{
		InstalledAgents: []string{"claude"},
		Env:             map[string]string{"MY_VAR": "value"},
	}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, `ENV MY_VAR="value"`) {
		t.Errorf("expected output to contain ENV MY_VAR=\"value\", got:\n%s", output)
	}
}

func TestRender_envVarsEscaped(t *testing.T) {
	cfg := &config.Config{
		InstalledAgents: []string{"claude"},
		Env: map[string]string{
			"QUOTES":     `it's a "test"`,
			"BACKSLASH":  `C:\path\to\file`,
			"MIXED":      `say \"hello\"`,
			"SAFE_PLAIN": "no_special_chars",
		},
	}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, `ENV QUOTES="it's a \"test\""`) {
		t.Error("expected double-quotes in value to be escaped")
	}
	if !strings.Contains(output, `ENV BACKSLASH="C:\\path\\to\\file"`) {
		t.Error("expected backslashes in value to be escaped")
	}
	if !strings.Contains(output, `ENV MIXED="say \\\"hello\\\""`) {
		t.Error("expected mixed backslash+quote in value to be escaped")
	}
	if !strings.Contains(output, `ENV SAFE_PLAIN="no_special_chars"`) {
		t.Error("expected plain value to pass through unmodified")
	}
}

func TestRender_noEnvVars(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Check there are no user-defined ENV directives (Testcontainers and PATH ENVs are expected)
	knownEnvs := map[string]bool{
		`ENV TERM=xterm-256color`:                          true,
		`ENV COLORTERM=truecolor`:                          true,
		`ENV TESTCONTAINERS_RYUK_DISABLED=true`:            true,
		`ENV TESTCONTAINERS_HOST_OVERRIDE=localhost`:       true,
		`ENV PATH="/usr/local/go/bin:${PATH}"`:             true,
		`ENV NPM_CONFIG_PREFIX=/home/sandbox/.npm-global`:  true,
		`ENV PATH="/home/sandbox/.npm-global/bin:${PATH}"`: true,
		`ENV PATH="/home/sandbox/.local/bin:${PATH}"`:      true,
		`ENV HOME=/home/sandbox`:                           true,
	}
	for line := range strings.SplitSeq(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ENV ") && !knownEnvs[trimmed] {
			t.Errorf("expected no user ENV directives, found: %s", line)
		}
	}
}

func TestRender_copyScripts(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	scripts := []string{
		"COPY entrypoint.sh /usr/local/bin/entrypoint.sh",
		"COPY git-wrapper.sh /usr/local/bin/git",
		"COPY healthcheck-poller.sh /usr/local/bin/healthcheck-poller.sh",
	}
	for _, s := range scripts {
		if !strings.Contains(output, s) {
			t.Errorf("expected output to contain %q", s)
		}
	}
	if !strings.Contains(output, "chmod +x /usr/local/bin/entrypoint.sh /usr/local/bin/git /usr/local/bin/healthcheck-poller.sh") {
		t.Error("expected output to contain chmod +x for all scripts")
	}
}

func TestRender_minimalConfig(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should produce valid Dockerfile with no SDK blocks
	if !strings.Contains(output, "FROM ubuntu:24.04") {
		t.Error("expected valid Dockerfile with FROM directive")
	}
	if !strings.Contains(output, "ENTRYPOINT") {
		t.Error("expected valid Dockerfile with ENTRYPOINT directive")
	}
	// USER sandbox is expected when claude is installed (install script runs as sandbox user)
	if !strings.Contains(output, "USER sandbox") {
		t.Error("expected USER sandbox for claude agent install")
	}
	if !strings.Contains(output, "WORKDIR /workspace") {
		t.Error("expected valid Dockerfile with WORKDIR /workspace")
	}
}

func TestRender_errorType(t *testing.T) {
	// Verify that Render returns *TemplateError on failure.
	// We cannot easily force a template execution error with a valid embedded template,
	// so this test documents the error contract for valid input only.
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	_, err := Render(cfg)
	if err != nil {
		var tmplErr *TemplateError
		if !errors.As(err, &tmplErr) {
			t.Errorf("expected *TemplateError, got %T", err)
		}
	}
}

func TestRender_nodejsOnly(t *testing.T) {
	cfg := &config.Config{
		InstalledAgents: []string{"claude"},
		SDKs:            config.SDKConfig{NodeJS: "22"},
	}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "deb.nodesource.com/setup_${NODE_VERSION}.x") {
		t.Error("expected NodeSource setup script in output")
	}
	if !strings.Contains(output, "apt-get install -y nodejs") {
		t.Error("expected nodejs install in output")
	}
	if !strings.Contains(output, "ARG NODE_VERSION=22") {
		t.Error("expected ARG NODE_VERSION=22 in output")
	}
	if strings.Contains(output, "go.dev/dl/go") {
		t.Error("expected no Go block when Go SDK not configured")
	}
	if strings.Contains(output, "deadsnakes") {
		t.Error("expected no Python block when Python SDK not configured")
	}
}

func TestRender_pythonOnly(t *testing.T) {
	cfg := &config.Config{
		InstalledAgents: []string{"claude"},
		SDKs:            config.SDKConfig{Python: "3.12"},
	}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "deadsnakes/ppa") {
		t.Error("expected deadsnakes PPA in output")
	}
	if !strings.Contains(output, "python${PYTHON_VERSION}") {
		t.Error("expected python${PYTHON_VERSION} install in output")
	}
	if !strings.Contains(output, "ARG PYTHON_VERSION=3.12") {
		t.Error("expected ARG PYTHON_VERSION=3.12 in output")
	}
	if strings.Contains(output, "nodesource") {
		t.Error("expected no Node.js block when NodeJS not configured")
	}
	if strings.Contains(output, "go.dev/dl/go") {
		t.Error("expected no Go block when Go SDK not configured")
	}
}

func TestRender_goOnly(t *testing.T) {
	cfg := &config.Config{
		InstalledAgents: []string{"claude"},
		SDKs:            config.SDKConfig{Go: "1.23"},
	}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "go.dev/dl/go${GO_VERSION}.linux-$(dpkg --print-architecture).tar.gz") {
		t.Error("expected Go tarball download in output with arch detection")
	}
	if !strings.Contains(output, `ENV PATH="/usr/local/go/bin:${PATH}"`) {
		t.Error("expected Go PATH setup in output")
	}
	if !strings.Contains(output, "ARG GO_VERSION=1.23") {
		t.Error("expected ARG GO_VERSION=1.23 in output")
	}
	if strings.Contains(output, "nodesource") {
		t.Error("expected no Node.js block when NodeJS not configured")
	}
	if strings.Contains(output, "deadsnakes") {
		t.Error("expected no Python block when Python not configured")
	}
}

func TestRender_multipleSDKs(t *testing.T) {
	cfg := &config.Config{
		InstalledAgents: []string{"claude"},
		SDKs:            config.SDKConfig{NodeJS: "22", Python: "3.12"},
	}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "nodesource") {
		t.Error("expected Node.js block in output")
	}
	if !strings.Contains(output, "deadsnakes") {
		t.Error("expected Python block in output")
	}
	if strings.Contains(output, "go.dev/dl/go") {
		t.Error("expected no Go block when Go SDK not configured")
	}
}

func TestRender_allSDKs(t *testing.T) {
	cfg := &config.Config{
		InstalledAgents: []string{"claude"},
		SDKs:            config.SDKConfig{NodeJS: "22", Go: "1.23", Python: "3.12"},
	}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "nodesource") {
		t.Error("expected Node.js block in output")
	}
	if !strings.Contains(output, "go.dev/dl/go") {
		t.Error("expected Go block in output")
	}
	if !strings.Contains(output, "deadsnakes") {
		t.Error("expected Python block in output")
	}
}

func TestRender_noSDKs(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(output, "nodesource") {
		t.Error("expected no Node.js block when no SDKs configured")
	}
	if strings.Contains(output, "go.dev") {
		t.Error("expected no Go block when no SDKs configured")
	}
	if strings.Contains(output, "deadsnakes") {
		t.Error("expected no Python block when no SDKs configured")
	}
}

func TestRender_additionalPackages(t *testing.T) {
	cfg := &config.Config{
		InstalledAgents: []string{"claude"},
		Packages:        []string{"libpq-dev", "redis-tools"},
	}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "libpq-dev") {
		t.Error("expected libpq-dev in additional packages block")
	}
	if !strings.Contains(output, "redis-tools") {
		t.Error("expected redis-tools in additional packages block")
	}
}

func TestRender_noPackages(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With Podman tooling, there are multiple apt-get install lines (base + uidmap/slirp4netns + podman)
	// Verify there is no additional packages block (the one with range over .Packages)
	// The additional packages block would contain user-specified packages like libpq-dev
	// Just verify no additional packages block content appears
	if strings.Contains(output, "libpq-dev") || strings.Contains(output, "redis-tools") {
		t.Error("expected no additional packages block when no packages configured")
	}
}

func TestRender_noBlankLinesWithoutSDKs(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// When no SDKs are configured, there should be no SDK-related content
	// between the chmod scripts line and the ENTRYPOINT line
	chmodIdx := strings.Index(output, "chmod +x")
	entrypointIdx := strings.Index(output, "ENTRYPOINT")
	if chmodIdx < 0 || entrypointIdx < 0 {
		t.Fatal("expected chmod and ENTRYPOINT in output")
	}
	between := output[chmodIdx:entrypointIdx]
	// Should not contain any SDK-related keywords
	if strings.Contains(between, "nodesource") {
		t.Error("found Node.js block between scripts and ENTRYPOINT when no SDKs configured")
	}
	if strings.Contains(between, "go.dev") {
		t.Error("found Go block between scripts and ENTRYPOINT when no SDKs configured")
	}
	if strings.Contains(between, "deadsnakes") {
		t.Error("found Python block between scripts and ENTRYPOINT when no SDKs configured")
	}
	// Check no extra blank lines in the SDK region (between chmod line end and ENTRYPOINT)
	chmodLineEnd := strings.Index(output[chmodIdx:], "\n") + chmodIdx
	region := output[chmodLineEnd:entrypointIdx]
	if strings.Contains(region, "\n\n\n") {
		t.Errorf("found excessive blank lines in SDK region when no SDKs configured:\n%s", region)
	}
}

func TestRender_podmanInstalled(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "devel:kubic:libcontainers:unstable") {
		t.Error("expected Kubic repository setup in output")
	}
	if !strings.Contains(output, "podman podman-docker") {
		t.Error("expected podman and podman-docker install in output")
	}
}

func TestRender_podmanConfig(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "storage.conf") {
		t.Error("expected storage.conf configuration in output")
	}
	if !strings.Contains(output, `driver = "vfs"`) {
		t.Error("expected vfs storage driver in output")
	}
	if !strings.Contains(output, "containers.conf") {
		t.Error("expected containers.conf configuration in output")
	}
	if !strings.Contains(output, `network_backend = "netavark"`) {
		t.Error("expected netavark network backend in output")
	}
	if !strings.Contains(output, `events_logger = "file"`) {
		t.Error("expected file events logger in output")
	}
}

func TestRender_dockerCompose(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "docker/compose/releases") {
		t.Error("expected Docker Compose download from GitHub releases")
	}
	if !strings.Contains(output, "docker-compose-linux-$(uname -m)") {
		t.Error("expected multi-arch Docker Compose download using $(uname -m)")
	}
	if !strings.Contains(output, "/usr/local/bin/docker-compose") {
		t.Error("expected Docker Compose installed to /usr/local/bin/docker-compose")
	}
	if !strings.Contains(output, "/usr/local/lib/docker/cli-plugins/docker-compose") {
		t.Error("expected Docker Compose symlink at cli-plugins path")
	}
	if strings.Contains(output, "api.github.com") {
		t.Error("expected Docker Compose install to avoid dynamic GitHub latest API lookups")
	}
}

func TestRender_validationToolsBlockPresent(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "# validation_tools") {
		t.Error("expected rendered Dockerfile to include validation_tools block")
	}
	if !strings.Contains(output, "/usr/local/bin/kubectl") {
		t.Error("expected rendered Dockerfile to install kubectl into /usr/local/bin")
	}
}

func TestRender_validationToolsAllElevenPresent(t *testing.T) {
	// jq is intentionally omitted: it ships via the base apt-get install block
	// (verified by TestRender_commonPackages). The other eleven tools are installed
	// explicitly in the validation_tools RUN block as /usr/local/bin/<name>.
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, tool := range []string{
		"kubectl",
		"helm",
		"kustomize",
		"yq",
		"tofu",
		"tflint",
		"kubeconform",
		"kube-linter",
		"trivy",
		"flux",
		"sops",
	} {
		if !strings.Contains(output, "/usr/local/bin/"+tool) {
			t.Errorf("expected validation tool %q installed at /usr/local/bin/%s", tool, tool)
		}
	}
}

func TestRender_validationToolsPinnedVersions(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		tool    string
		version string
	}{
		{tool: "kubectl", version: "v1.35.4"},
		{tool: "helm", version: "v3.20.2"},
		{tool: "kustomize", version: "v5.8.1"},
		{tool: "yq", version: "v4.53.2"},
		{tool: "opentofu", version: "v1.11.6"},
		{tool: "tflint", version: "v0.62.0"},
		{tool: "kubeconform", version: "v0.7.0"},
		{tool: "kube-linter", version: "v0.8.3"},
		{tool: "trivy", version: "v0.70.0"},
		{tool: "flux", version: "v2.8.5"},
		{tool: "sops", version: "v3.12.2"},
	}

	for _, tt := range tests {
		if !strings.Contains(output, tt.version) {
			t.Errorf("expected %s version %q in rendered Dockerfile", tt.tool, tt.version)
		}
	}
}

func TestRender_validationToolsNoDynamicLatest(t *testing.T) {
	// Rendered against an empty config so SDK/agent blocks that legitimately use
	// `curl | bash` (nodesource, get-pip) or `api.github.com` are not emitted.
	// Note: the Dockerfile header documents these forbidden strings in a
	// Go-template comment (`{{- /* ... */ -}}`) which is stripped at render
	// time, so the assertions below only reflect RUN-block content. If that
	// comment wrapper is ever removed, these assertions will flip — intentionally.
	output, err := Render(&config.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(output, "api.github.com") {
		t.Error("expected validation tools block to avoid api.github.com lookups")
	}
	if strings.Contains(output, "releases/latest") {
		t.Error("expected validation tools block to avoid releases/latest lookups")
	}
	if strings.Contains(output, "| bash") {
		t.Error("expected validation tools block to avoid curl | bash installers")
	}
}

func TestRender_validationToolsUnconditional(t *testing.T) {
	configs := []*config.Config{
		{},
		{InstalledAgents: []string{"claude"}},
		{
			InstalledAgents: []string{"claude", "gemini", "codex"},
			SDKs:            config.SDKConfig{NodeJS: "22", Python: "3.12", Go: "1.23"},
			MCP:             []string{"playwright"},
		},
	}

	for i, cfg := range configs {
		output, err := Render(cfg)
		if err != nil {
			t.Fatalf("config %d: unexpected error: %v", i, err)
		}
		if !strings.Contains(output, "# validation_tools") {
			t.Errorf("config %d: expected validation_tools block in rendered Dockerfile", i)
		}
	}
}

func TestRender_cacheDirsChowned(t *testing.T) {
	output, err := Render(&config.Config{InstalledAgents: []string{"claude"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, dir := range []string{
		"/home/sandbox/.cache/trivy",
		"/home/sandbox/.cache/helm",
		"/home/sandbox/.kube",
		"/home/sandbox/.terraform.d",
		"/home/sandbox/.config/sops",
	} {
		if !strings.Contains(output, dir) {
			t.Errorf("expected cache directory %q to be created in rendered Dockerfile", dir)
		}
	}
	for _, chownRoot := range []string{
		"/home/sandbox/.cache",
		"/home/sandbox/.kube",
		"/home/sandbox/.terraform.d",
		"/home/sandbox/.config",
	} {
		if !strings.Contains(output, "chown -R sandbox:sandbox") || !strings.Contains(output, chownRoot) {
			t.Errorf("expected chown of %q to sandbox:sandbox in rendered Dockerfile", chownRoot)
		}
	}
}

func TestRender_claudeCodeAgent(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "claude.ai/install.sh") {
		t.Error("expected Claude Code install script in output")
	}
	if strings.Contains(output, "npm install -g @google/gemini-cli") {
		t.Error("expected no Gemini CLI install when only claude installed")
	}
}

func TestRender_geminiAgent(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"gemini"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "npm install -g @google/gemini-cli") {
		t.Error("expected Gemini CLI npm install in output")
	}
	if !strings.Contains(output, "@google/gemini-cli@") {
		t.Error("expected Gemini CLI npm install to be version pinned")
	}
	if strings.Contains(output, "claude.ai/install.sh") {
		t.Error("expected no Claude Code install when only gemini installed")
	}
}

func TestRender_npmVersionsPinned(t *testing.T) {
	cfg := &config.Config{
		InstalledAgents: []string{"gemini", "codex"},
		SDKs:            config.SDKConfig{NodeJS: "22"},
		MCP:             []string{"playwright"},
	}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, expected := range []string{
		"@google/gemini-cli@",
		"@openai/codex@",
		"@playwright/mcp@",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("expected version-pinned npm install containing %q", expected)
		}
	}
}

func TestRender_agentInstructions(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "COPY agent-instructions.md.tmpl /tmp/agent-instructions.md") {
		t.Error("expected agent instructions COPY directive in output")
	}
	if !strings.Contains(output, "/home/sandbox/CLAUDE.md") {
		t.Error("expected CLAUDE.md in sandbox home directory")
	}

	// Test gemini agent gets GEMINI.md
	cfg2 := &config.Config{InstalledAgents: []string{"gemini"}}
	output2, err := Render(cfg2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output2, "/home/sandbox/GEMINI.md") {
		t.Error("expected GEMINI.md in sandbox home directory for gemini agent")
	}

	// Test multi-agent gets both instruction files
	cfg3 := &config.Config{InstalledAgents: []string{"claude", "gemini"}}
	output3, err := Render(cfg3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output3, "/home/sandbox/CLAUDE.md") {
		t.Error("expected CLAUDE.md for multi-agent config")
	}
	if !strings.Contains(output3, "/home/sandbox/GEMINI.md") {
		t.Error("expected GEMINI.md for multi-agent config")
	}
}

func TestRender_testcontainersEnv(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "ENV TESTCONTAINERS_RYUK_DISABLED=true") {
		t.Error("expected TESTCONTAINERS_RYUK_DISABLED=true in output")
	}
	if !strings.Contains(output, "ENV TESTCONTAINERS_HOST_OVERRIDE=localhost") {
		t.Error("expected TESTCONTAINERS_HOST_OVERRIDE=localhost in output")
	}
}

func TestRender_mcpPlaywrightBlock(t *testing.T) {
	cfg := &config.Config{
		InstalledAgents: []string{"claude"},
		SDKs:            config.SDKConfig{NodeJS: "22"},
		MCP:             []string{"playwright"},
	}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "@playwright/mcp") {
		t.Error("expected @playwright/mcp install when MCP contains playwright")
	}
	if !strings.Contains(output, "playwright install --with-deps chromium webkit") {
		t.Error("expected playwright install --with-deps chromium webkit when playwright configured")
	}
	if !strings.Contains(output, "PLAYWRIGHT_BROWSERS_PATH=/opt/playwright-browsers") {
		t.Error("expected PLAYWRIGHT_BROWSERS_PATH env var when playwright configured")
	}
	if !strings.Contains(output, "PLAYWRIGHT_MCP_BROWSER=chromium") {
		t.Error("expected PLAYWRIGHT_MCP_BROWSER env var when playwright configured")
	}
	if !strings.Contains(output, "chown -R sandbox:sandbox /opt/playwright-browsers") {
		t.Error("expected chown for playwright browsers directory")
	}
}

func TestRender_mcpManifestAlwaysPresent(t *testing.T) {
	// Manifest should be present even without MCP
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "mcp-servers.json") {
		t.Error("expected mcp-servers.json manifest directive even without MCP")
	}
}

func TestRender_mcpManifestContentWithPlaywright(t *testing.T) {
	cfg := &config.Config{
		InstalledAgents: []string{"claude"},
		SDKs:            config.SDKConfig{NodeJS: "22"},
		MCP:             []string{"playwright"},
	}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, `"playwright"`) {
		t.Error("expected manifest to contain playwright entry")
	}
	if !strings.Contains(output, `"npx"`) {
		t.Error("expected manifest to contain npx command")
	}
}

func TestRender_mcpManifestContentEmpty(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, `{"mcpServers":{}}`) {
		t.Error("expected empty manifest when no MCP configured")
	}
}

func TestRender_noPlaywrightWithoutMCP(t *testing.T) {
	cfg := &config.Config{
		InstalledAgents: []string{"claude"},
		SDKs:            config.SDKConfig{NodeJS: "22"},
	}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(output, "@playwright/mcp") {
		t.Error("expected no Playwright MCP install when MCP not configured")
	}
}

func TestRender_noBlankLinesWithoutTooling(t *testing.T) {
	cfg := &config.Config{InstalledAgents: []string{"claude"}}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify no excessive blank lines in the output
	if strings.Contains(output, "\n\n\n\n") {
		t.Error("found excessive blank lines (4+) in rendered output")
	}
}

func TestRender_goSDKMultiArch(t *testing.T) {
	cfg := &config.Config{
		InstalledAgents: []string{"claude"},
		SDKs:            config.SDKConfig{Go: "1.23"},
	}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "go.dev/dl/go${GO_VERSION}.linux-$(dpkg --print-architecture).tar.gz") {
		t.Error("expected Go SDK download to use $(dpkg --print-architecture) for multi-arch support")
	}
	if strings.Contains(output, "linux-amd64") {
		t.Error("expected no hardcoded linux-amd64 in Go SDK download URL")
	}
}

func firstLine(s string) string {
	if before, _, ok := strings.Cut(s, "\n"); ok {
		return before
	}
	return s
}
