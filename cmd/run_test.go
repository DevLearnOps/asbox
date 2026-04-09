package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mcastellin/asbox/internal/config"
)

// writeRunConfig writes a config YAML file in a .asbox subdirectory and returns its path.
func writeRunConfig(t *testing.T, dir, content string) string {
	t.Helper()
	asboxDir := filepath.Join(dir, ".asbox")
	if err := os.MkdirAll(asboxDir, 0o755); err != nil {
		t.Fatalf("failed to create .asbox dir: %v", err)
	}
	path := filepath.Join(asboxDir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	return path
}

func TestRun_nonexistentMountSource_returnsConfigError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeRunConfig(t, dir, `
agent: claude-code
mounts:
  - source: ./nonexistent-dir
    target: /workspace
`)

	old := configFile
	configFile = cfgPath
	t.Cleanup(func() { configFile = old })

	r := newRootCmd()
	err := r.run("run")
	if err == nil {
		t.Fatal("expected error for nonexistent mount source, got nil")
	}

	var ce *config.ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *config.ConfigError, got %T: %v", err, err)
	}
	if got := exitCode(err); got != 1 {
		t.Errorf("exitCode = %d, want 1", got)
	}
}

func TestRun_nonexistentMountSource_errorMessageFormat(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeRunConfig(t, dir, `
agent: claude-code
mounts:
  - source: ./does-not-exist
    target: /workspace
`)

	old := configFile
	configFile = cfgPath
	t.Cleanup(func() { configFile = old })

	r := newRootCmd()
	err := r.run("run")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// The resolved path will be absolute relative to the config file directory
	asboxDir := filepath.Join(dir, ".asbox")
	resolved := filepath.Join(asboxDir, "does-not-exist")
	want := "mount source '" + resolved + "' not found (resolved to " + resolved + "). Check mounts in .asbox/config.yaml"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}
