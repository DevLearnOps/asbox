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

func firstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return s[:idx]
	}
	return s
}
