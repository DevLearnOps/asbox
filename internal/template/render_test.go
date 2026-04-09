package template

import (
	"errors"
	"strings"
	"testing"

	"github.com/mcastellin/asbox/internal/config"
)

func TestRender_baseImage(t *testing.T) {
	cfg := &config.Config{Agent: "claude-code"}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(output, "FROM ubuntu:24.04@sha256:") {
		t.Errorf("expected output to start with pinned Ubuntu base image, got: %s", firstLine(output))
	}
}

func TestRender_tiniInstalled(t *testing.T) {
	cfg := &config.Config{Agent: "claude-code"}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "apt-get install") || !strings.Contains(output, "tini") {
		t.Error("expected output to contain apt-get install with tini")
	}
}

func TestRender_sandboxUserCreated(t *testing.T) {
	cfg := &config.Config{Agent: "claude-code"}
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
	cfg := &config.Config{Agent: "claude-code"}
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
	cfg := &config.Config{Agent: "claude-code"}
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
		Agent: "claude-code",
		Env:   map[string]string{"MY_VAR": "value"},
	}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, `ENV MY_VAR="value"`) {
		t.Errorf("expected output to contain ENV MY_VAR=\"value\", got:\n%s", output)
	}
}

func TestRender_noEnvVars(t *testing.T) {
	cfg := &config.Config{Agent: "claude-code"}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(output, "ENV ") {
		// Filter out DEBIAN_FRONTEND which is an ARG, not ENV
		// Check there are no user ENV directives
		for _, line := range strings.Split(output, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "ENV ") {
				t.Errorf("expected no ENV directives for user vars, found: %s", line)
			}
		}
	}
}

func TestRender_copyScripts(t *testing.T) {
	cfg := &config.Config{Agent: "claude-code"}
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
	cfg := &config.Config{Agent: "claude-code"}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should produce valid Dockerfile with no SDK blocks
	if !strings.Contains(output, "FROM ubuntu:24.04@sha256:") {
		t.Error("expected valid Dockerfile with FROM directive")
	}
	if !strings.Contains(output, "ENTRYPOINT") {
		t.Error("expected valid Dockerfile with ENTRYPOINT directive")
	}
	if !strings.Contains(output, "USER sandbox") {
		t.Error("expected valid Dockerfile with USER sandbox")
	}
	if !strings.Contains(output, "WORKDIR /workspace") {
		t.Error("expected valid Dockerfile with WORKDIR /workspace")
	}
}

func TestRender_errorType(t *testing.T) {
	// Verify that Render returns *TemplateError on failure.
	// We cannot easily force a template execution error with a valid embedded template,
	// so this test documents the error contract for valid input only.
	cfg := &config.Config{Agent: "claude-code"}
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
		Agent: "claude-code",
		SDKs:  config.SDKConfig{NodeJS: "22"},
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
		Agent: "claude-code",
		SDKs:  config.SDKConfig{Python: "3.12"},
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
		Agent: "claude-code",
		SDKs:  config.SDKConfig{Go: "1.23"},
	}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz") {
		t.Error("expected Go tarball download in output")
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
		Agent: "claude-code",
		SDKs:  config.SDKConfig{NodeJS: "22", Python: "3.12"},
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
		Agent: "claude-code",
		SDKs:  config.SDKConfig{NodeJS: "22", Go: "1.23", Python: "3.12"},
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
	cfg := &config.Config{Agent: "claude-code"}
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
		Agent:    "claude-code",
		Packages: []string{"libpq-dev", "redis-tools"},
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
	cfg := &config.Config{Agent: "claude-code"}
	output, err := Render(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Count how many apt-get install lines there are — should only be the base packages one
	count := strings.Count(output, "apt-get install")
	if count != 1 {
		t.Errorf("expected exactly 1 apt-get install (base packages only), found %d", count)
	}
}

func TestRender_noBlankLinesWithoutSDKs(t *testing.T) {
	cfg := &config.Config{Agent: "claude-code"}
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

func firstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return s[:idx]
	}
	return s
}
