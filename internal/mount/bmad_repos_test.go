package mount

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mcastellin/asbox/internal/config"
)

func TestAssembleBmadRepos_validPaths(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	cfg := &config.Config{
		BmadRepos: []string{dir1, dir2},
	}

	mounts, content, err := AssembleBmadRepos(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mounts) != 2 {
		t.Fatalf("len(mounts) = %d, want 2", len(mounts))
	}

	base1 := filepath.Base(dir1)
	base2 := filepath.Base(dir2)

	want1 := dir1 + ":/workspace/repos/" + base1
	if mounts[0] != want1 {
		t.Errorf("mounts[0] = %q, want %q", mounts[0], want1)
	}
	want2 := dir2 + ":/workspace/repos/" + base2
	if mounts[1] != want2 {
		t.Errorf("mounts[1] = %q, want %q", mounts[1], want2)
	}

	if content == "" {
		t.Error("expected non-empty instruction content")
	}
}

func TestAssembleBmadRepos_emptyRepos(t *testing.T) {
	cfg := &config.Config{}

	mounts, content, err := AssembleBmadRepos(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mounts != nil {
		t.Errorf("got mounts %v, want nil", mounts)
	}
	if content != "" {
		t.Errorf("got content %q, want empty", content)
	}
}

func TestAssembleBmadRepos_nonexistentPath(t *testing.T) {
	cfg := &config.Config{
		BmadRepos: []string{"/nonexistent/path/that/does/not/exist"},
	}

	_, _, err := AssembleBmadRepos(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *config.ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *config.ConfigError, got %T: %v", err, err)
	}

	want := "bmad_repos path '/nonexistent/path/that/does/not/exist' not found. Check bmad_repos in .asbox/config.yaml"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestAssembleBmadRepos_pathIsFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(file, []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cfg := &config.Config{
		BmadRepos: []string{file},
	}

	_, _, err := AssembleBmadRepos(cfg)
	if err == nil {
		t.Fatal("expected error for file path, got nil")
	}

	var ce *config.ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *config.ConfigError, got %T: %v", err, err)
	}

	if !strings.Contains(err.Error(), "is not a directory") {
		t.Errorf("error should mention 'is not a directory', got: %s", err.Error())
	}
}

func TestAssembleBmadRepos_basenameCollision(t *testing.T) {
	// Create two directories with the same basename under different parents
	parent1 := t.TempDir()
	parent2 := t.TempDir()
	dir1 := filepath.Join(parent1, "client")
	dir2 := filepath.Join(parent2, "client")
	os.Mkdir(dir1, 0o755)
	os.Mkdir(dir2, 0o755)

	cfg := &config.Config{
		BmadRepos: []string{dir1, dir2},
	}

	_, _, err := AssembleBmadRepos(cfg)
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}

	var ce *config.ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *config.ConfigError, got %T: %v", err, err)
	}

	msg := err.Error()
	if !strings.Contains(msg, "basename collision") {
		t.Errorf("error should mention 'basename collision', got: %s", msg)
	}
	if !strings.Contains(msg, dir1) || !strings.Contains(msg, dir2) {
		t.Errorf("error should contain both paths, got: %s", msg)
	}
}

func TestAssembleBmadRepos_singleRepo(t *testing.T) {
	dir := t.TempDir()

	cfg := &config.Config{
		BmadRepos: []string{dir},
	}

	mounts, content, err := AssembleBmadRepos(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mounts) != 1 {
		t.Fatalf("len(mounts) = %d, want 1", len(mounts))
	}

	base := filepath.Base(dir)
	want := dir + ":/workspace/repos/" + base
	if mounts[0] != want {
		t.Errorf("mounts[0] = %q, want %q", mounts[0], want)
	}
	if content == "" {
		t.Error("expected non-empty instruction content")
	}
}

func TestAssembleBmadRepos_instructionContentRendered(t *testing.T) {
	dir := t.TempDir()
	// Rename the temp dir to have a predictable basename
	repoDir := filepath.Join(filepath.Dir(dir), "myrepo")
	if err := os.Rename(dir, repoDir); err != nil {
		t.Fatalf("failed to rename temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(repoDir) })

	cfg := &config.Config{
		BmadRepos: []string{repoDir},
	}

	_, content, err := AssembleBmadRepos(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(content, "myrepo") {
		t.Errorf("instruction content should contain repo basename 'myrepo', got:\n%s", content)
	}
	if !strings.Contains(content, "/workspace/repos/myrepo") {
		t.Errorf("instruction content should contain container path, got:\n%s", content)
	}
	if !strings.Contains(content, "Multi-Repo Workspace") {
		t.Errorf("instruction content should contain 'Multi-Repo Workspace' section, got:\n%s", content)
	}
}

func TestAssembleBmadRepos_instructionContentContainsAllBasenames(t *testing.T) {
	parent := t.TempDir()
	dir1 := filepath.Join(parent, "frontend")
	dir2 := filepath.Join(parent, "api")
	os.Mkdir(dir1, 0o755)
	os.Mkdir(dir2, 0o755)

	cfg := &config.Config{
		BmadRepos: []string{dir1, dir2},
	}

	_, content, err := AssembleBmadRepos(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(content, "frontend") {
		t.Errorf("instruction content should contain 'frontend', got:\n%s", content)
	}
	if !strings.Contains(content, "api") {
		t.Errorf("instruction content should contain 'api', got:\n%s", content)
	}
}
