package mount

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/mcastellin/asbox/internal/config"
)

func TestAssembleMounts_validMounts(t *testing.T) {
	dir := t.TempDir()

	cfg := &config.Config{
		Mounts: []config.MountConfig{
			{Source: dir, Target: "/workspace"},
		},
	}

	got, err := AssembleMounts(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	want := dir + ":/workspace"
	if got[0] != want {
		t.Errorf("got %q, want %q", got[0], want)
	}
}

func TestAssembleMounts_nonexistentSource(t *testing.T) {
	cfg := &config.Config{
		Mounts: []config.MountConfig{
			{Source: "/nonexistent/path/that/does/not/exist", Target: "/workspace"},
		},
	}

	_, err := AssembleMounts(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *config.ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *config.ConfigError, got %T: %v", err, err)
	}
}

func TestAssembleMounts_errorMessageFormat(t *testing.T) {
	src := "/nonexistent/mount/source"
	cfg := &config.Config{
		Mounts: []config.MountConfig{
			{Source: src, Target: "/workspace"},
		},
	}

	_, err := AssembleMounts(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	want := "mount source '/nonexistent/mount/source' not found (resolved to /nonexistent/mount/source). Check mounts in .asbox/config.yaml"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestAssembleMounts_multipleMounts(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	cfg := &config.Config{
		Mounts: []config.MountConfig{
			{Source: dir1, Target: "/workspace"},
			{Source: dir2, Target: "/data"},
		},
	}

	got, err := AssembleMounts(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0] != dir1+":/workspace" {
		t.Errorf("got[0] = %q, want %q", got[0], dir1+":/workspace")
	}
	if got[1] != dir2+":/data" {
		t.Errorf("got[1] = %q, want %q", got[1], dir2+":/data")
	}
}

func TestAssembleMounts_emptyMountsList(t *testing.T) {
	cfg := &config.Config{}

	got, err := AssembleMounts(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestAssembleMounts_failsOnFirstBadMount(t *testing.T) {
	dir := t.TempDir()

	cfg := &config.Config{
		Mounts: []config.MountConfig{
			{Source: dir, Target: "/workspace"},
			{Source: filepath.Join(dir, "nonexistent"), Target: "/data"},
		},
	}

	_, err := AssembleMounts(cfg)
	if err == nil {
		t.Fatal("expected error for nonexistent second mount, got nil")
	}

	var ce *config.ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *config.ConfigError, got %T: %v", err, err)
	}
}
