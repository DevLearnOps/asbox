package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	asboxEmbed "github.com/mcastellin/asbox/embed"
	"github.com/mcastellin/asbox/internal/config"
)

func TestInit_createsConfigAtDefaultPath(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, ".asbox", "config.yaml")

	old := configFile
	configFile = target
	t.Cleanup(func() { configFile = old })

	r := newRootCmd()
	err := r.run("init")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(target); err != nil {
		t.Fatalf("config file not created: %v", err)
	}
}

func TestInit_errorsWhenConfigAlreadyExists(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, ".asbox", "config.yaml")

	os.MkdirAll(filepath.Dir(target), 0o755)
	os.WriteFile(target, []byte("existing"), 0o644)

	old := configFile
	configFile = target
	t.Cleanup(func() { configFile = old })

	r := newRootCmd()
	err := r.run("init")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *config.ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *config.ConfigError, got %T: %v", err, err)
	}
	if got := err.Error(); got != "config already exists at "+target {
		t.Errorf("unexpected error message: %q", got)
	}
	if got := exitCode(err); got != 1 {
		t.Errorf("exitCode = %d, want 1", got)
	}
}

func TestInit_createsConfigAtCustomPath(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "custom", "path", "config.yaml")

	r := newRootCmd()
	err := r.run("init", "-f", target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(target); err != nil {
		t.Fatalf("config file not created at custom path: %v", err)
	}
}

func TestInit_createsParentDirectories(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "deep", "nested", "dir", "config.yaml")

	old := configFile
	configFile = target
	t.Cleanup(func() { configFile = old })

	r := newRootCmd()
	err := r.run("init")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(filepath.Dir(target))
	if err != nil {
		t.Fatalf("parent directories not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("parent path is not a directory")
	}
}

func TestInit_generatedFileMatchesEmbeddedTemplate(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, ".asbox", "config.yaml")

	old := configFile
	configFile = target
	t.Cleanup(func() { configFile = old })

	r := newRootCmd()
	err := r.run("init")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}

	want, err := asboxEmbed.Assets.ReadFile("config.yaml")
	if err != nil {
		t.Fatalf("failed to read embedded template: %v", err)
	}

	if string(got) != string(want) {
		t.Error("generated config does not match embedded template")
	}
}

func TestInit_doesNotRequireDocker(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, ".asbox", "config.yaml")

	old := configFile
	configFile = target
	t.Cleanup(func() { configFile = old })

	// Remove docker from PATH
	t.Setenv("PATH", "/nonexistent")

	r := newRootCmd()
	err := r.run("init")
	if err != nil {
		t.Fatalf("init should not require docker, got error: %v", err)
	}
}

func TestInit_successMessageIncludesPath(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, ".asbox", "config.yaml")

	old := configFile
	configFile = target
	t.Cleanup(func() { configFile = old })

	r := newRootCmd()
	err := r.run("init")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := r.output()
	want := "created " + target + "\n"
	if output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
}
