package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mcastellin/asbox/internal/config"
	"github.com/mcastellin/asbox/internal/docker"
	"github.com/mcastellin/asbox/internal/template"
)

// newRootCmd returns a fresh root command tree for isolated testing.
func newRootCmd() *rootCmdResult {
	// Re-initialize from the package-level rootCmd to pick up subcommands,
	// but capture output in a buffer so tests don't leak state.
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	return &rootCmdResult{buf: buf}
}

type rootCmdResult struct {
	buf *bytes.Buffer
}

func (r *rootCmdResult) run(args ...string) error {
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

func (r *rootCmdResult) output() string {
	return r.buf.String()
}

func TestExitCode_mapping(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{"ConfigError", &config.ConfigError{Msg: "bad"}, 1},
		{"TemplateError", &template.TemplateError{Msg: "bad"}, 1},
		{"BuildError", &docker.BuildError{Msg: "build failed"}, 1},
		{"DependencyError", &docker.DependencyError{Msg: "missing"}, 3},
		{"SecretError", &config.SecretError{Msg: "leak"}, 4},
		{"UsageError", &usageError{err: fmt.Errorf("unknown flag")}, 2},
		{"unknown error", fmt.Errorf("unknown"), 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitCode(tt.err); got != tt.expected {
				t.Errorf("exitCode(%T) = %d, want %d", tt.err, got, tt.expected)
			}
		})
	}
}

func TestExitCode_wrappedErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{"wrapped DependencyError", fmt.Errorf("context: %w", &docker.DependencyError{Msg: "missing"}), 3},
		{"wrapped SecretError", fmt.Errorf("context: %w", &config.SecretError{Msg: "leak"}), 4},
		{"wrapped ConfigError", fmt.Errorf("context: %w", &config.ConfigError{Msg: "bad"}), 1},
		{"wrapped TemplateError", fmt.Errorf("context: %w", &template.TemplateError{Msg: "bad"}), 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitCode(tt.err); got != tt.expected {
				t.Errorf("exitCode(wrapped %T) = %d, want %d", tt.err, got, tt.expected)
			}
		})
	}
}

func TestHelp_containsCommands(t *testing.T) {
	r := newRootCmd()
	err := r.run("--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := r.output()
	for _, want := range []string{"init", "build", "run", "-f", "--version"} {
		if !strings.Contains(output, want) {
			t.Errorf("help output missing %q", want)
		}
	}
}

func TestDockerNotFound_returnsDependencyError(t *testing.T) {
	t.Setenv("PATH", "/nonexistent")

	r := newRootCmd()
	err := r.run("build")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var de *docker.DependencyError
	if !errors.As(err, &de) {
		t.Fatalf("expected *docker.DependencyError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "docker not found") {
		t.Errorf("expected 'docker not found' message, got %q", err.Error())
	}
	if got := exitCode(err); got != 3 {
		t.Errorf("exitCode = %d, want 3", got)
	}
}

func TestStubCommands_returnNotImplemented(t *testing.T) {
	// Only init is still a stub; build and run now call config.Parse()
	for _, name := range []string{"init"} {
		t.Run(name, func(t *testing.T) {
			r := newRootCmd()
			err := r.run(name)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "not implemented") {
				t.Errorf("expected 'not implemented', got %q", err.Error())
			}
			var ce *config.ConfigError
			if !errors.As(err, &ce) {
				t.Errorf("expected *config.ConfigError, got %T", err)
			}
			if got := exitCode(err); got != 1 {
				t.Errorf("exitCode = %d, want 1", got)
			}
		})
	}
}

func TestBuildRun_missingConfig_returnsConfigError(t *testing.T) {
	for _, name := range []string{"build", "run"} {
		t.Run(name, func(t *testing.T) {
			r := newRootCmd()
			err := r.run(name)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var ce *config.ConfigError
			if !errors.As(err, &ce) {
				t.Fatalf("expected *ConfigError, got %T: %v", err, err)
			}
			if !strings.Contains(err.Error(), "config file not found") {
				t.Errorf("expected 'config file not found', got %q", err.Error())
			}
			if got := exitCode(err); got != 1 {
				t.Errorf("exitCode = %d, want 1", got)
			}
		})
	}
}

func TestUsageError_returnExitCode2(t *testing.T) {
	r := newRootCmd()
	err := r.run("--nonexistent-flag")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := exitCode(err); got != 2 {
		t.Errorf("exitCode = %d, want 2", got)
	}
}
