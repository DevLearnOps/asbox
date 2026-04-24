package mount

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mcastellin/asbox/internal/config"
)

func TestAssembleAgentInstructions_validPaths(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	cfg := &config.Config{
		BmadRepos: []string{dir1, dir2},
	}

	mounts, content, err := AssembleAgentInstructions(cfg)
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

func TestAssembleAgentInstructions_emptyRepos(t *testing.T) {
	cfg := &config.Config{}

	mounts, content, err := AssembleAgentInstructions(cfg)
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

func TestAssembleAgentInstructions_nonexistentPath(t *testing.T) {
	cfg := &config.Config{
		BmadRepos: []string{"/nonexistent/path/that/does/not/exist"},
	}

	_, _, err := AssembleAgentInstructions(cfg)
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

func TestAssembleAgentInstructions_pathIsFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(file, []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cfg := &config.Config{
		BmadRepos: []string{file},
	}

	_, _, err := AssembleAgentInstructions(cfg)
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

func TestAssembleAgentInstructions_basenameCollision(t *testing.T) {
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

	_, _, err := AssembleAgentInstructions(cfg)
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

func TestAssembleAgentInstructions_singleRepo(t *testing.T) {
	dir := t.TempDir()

	cfg := &config.Config{
		BmadRepos: []string{dir},
	}

	mounts, content, err := AssembleAgentInstructions(cfg)
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

func TestAssembleAgentInstructions_instructionContentRendered(t *testing.T) {
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

	_, content, err := AssembleAgentInstructions(cfg)
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

func TestAssembleAgentInstructions_instructionContentContainsAllBasenames(t *testing.T) {
	parent := t.TempDir()
	dir1 := filepath.Join(parent, "frontend")
	dir2 := filepath.Join(parent, "api")
	os.Mkdir(dir1, 0o755)
	os.Mkdir(dir2, 0o755)

	cfg := &config.Config{
		BmadRepos: []string{dir1, dir2},
	}

	_, content, err := AssembleAgentInstructions(cfg)
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

func TestAssembleAgentInstructions_extensionFileAppended(t *testing.T) {
	dir := t.TempDir()
	extensionPath := filepath.Join(dir, "AGENT_INSTRUCTIONS.md")
	body := "## Project Rules\n\nAlways run `go fmt`.\n"
	if err := os.WriteFile(extensionPath, []byte(body), 0o644); err != nil {
		t.Fatalf("failed to write extension file: %v", err)
	}

	cfg := &config.Config{
		AgentInstructions: extensionPath,
	}

	_, content, err := AssembleAgentInstructions(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "## Project-Specific Instructions") {
		t.Errorf("instruction content should contain project-specific heading, got:\n%s", content)
	}
	if !strings.Contains(content, body) {
		t.Errorf("instruction content should contain extension body verbatim, got:\n%s", content)
	}
	if strings.Contains(content, "## Multi-Repo Workspace") {
		t.Errorf("instruction content should not contain multi-repo section, got:\n%s", content)
	}
}

func TestAssembleAgentInstructions_extensionMissingFile_returnsConfigError(t *testing.T) {
	cfg := &config.Config{
		AgentInstructions: "/nonexistent/path/agent-instructions.md",
	}

	_, _, err := AssembleAgentInstructions(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *config.ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *config.ConfigError, got %T: %v", err, err)
	}

	want := "agent_instructions path '/nonexistent/path/agent-instructions.md' not found. Check agent_instructions in .asbox/config.yaml"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestAssembleAgentInstructions_extensionUnreadableFile_returnsConfigError(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("chmod 000 read behavior differs outside Linux")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can read chmod 000 files")
	}

	dir := t.TempDir()
	extensionPath := filepath.Join(dir, "AGENT_INSTRUCTIONS.md")
	if err := os.WriteFile(extensionPath, []byte("rules"), 0o644); err != nil {
		t.Fatalf("failed to write extension file: %v", err)
	}
	if err := os.Chmod(extensionPath, 0o000); err != nil {
		t.Fatalf("failed to chmod extension file: %v", err)
	}
	t.Cleanup(func() { os.Chmod(extensionPath, 0o644) })

	cfg := &config.Config{
		AgentInstructions: extensionPath,
	}

	_, _, err := AssembleAgentInstructions(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *config.ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *config.ConfigError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "is not readable") {
		t.Errorf("error should mention 'is not readable', got: %s", err.Error())
	}
}

func TestAssembleAgentInstructions_extensionOnlyNoBmadRepos(t *testing.T) {
	dir := t.TempDir()
	extensionPath := filepath.Join(dir, "AGENT_INSTRUCTIONS.md")
	body := "Use conventional commits.\n"
	if err := os.WriteFile(extensionPath, []byte(body), 0o644); err != nil {
		t.Fatalf("failed to write extension file: %v", err)
	}

	cfg := &config.Config{
		AgentInstructions: extensionPath,
	}

	mounts, content, err := AssembleAgentInstructions(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mounts != nil {
		t.Errorf("mounts = %v, want nil", mounts)
	}
	if content == "" {
		t.Fatal("expected non-empty instruction content")
	}
	if !strings.Contains(content, "## Project-Specific Instructions") {
		t.Errorf("instruction content should contain project-specific heading, got:\n%s", content)
	}
	if strings.Contains(content, "## Multi-Repo Workspace") {
		t.Errorf("instruction content should not contain multi-repo section, got:\n%s", content)
	}
}

func TestAssembleAgentInstructions_bothBmadReposAndExtension(t *testing.T) {
	dir := t.TempDir()
	repo1 := filepath.Join(dir, "frontend")
	repo2 := filepath.Join(dir, "api")
	if err := os.Mkdir(repo1, 0o755); err != nil {
		t.Fatalf("failed to create repo1: %v", err)
	}
	if err := os.Mkdir(repo2, 0o755); err != nil {
		t.Fatalf("failed to create repo2: %v", err)
	}
	extensionPath := filepath.Join(dir, "AGENT_INSTRUCTIONS.md")
	if err := os.WriteFile(extensionPath, []byte("Always run tests.\n"), 0o644); err != nil {
		t.Fatalf("failed to write extension file: %v", err)
	}

	cfg := &config.Config{
		BmadRepos:         []string{repo1, repo2},
		AgentInstructions: extensionPath,
	}

	mounts, content, err := AssembleAgentInstructions(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mounts) != 2 {
		t.Fatalf("len(mounts) = %d, want 2", len(mounts))
	}
	multiRepoIndex := strings.Index(content, "## Multi-Repo Workspace")
	projectIndex := strings.Index(content, "## Project-Specific Instructions")
	if multiRepoIndex == -1 {
		t.Fatalf("instruction content should contain multi-repo section, got:\n%s", content)
	}
	if projectIndex == -1 {
		t.Fatalf("instruction content should contain project-specific section, got:\n%s", content)
	}
	if projectIndex < multiRepoIndex {
		t.Errorf("project-specific section should appear after multi-repo section, got:\n%s", content)
	}
}

func TestAssembleAgentInstructions_extensionBodyVerbatim(t *testing.T) {
	dir := t.TempDir()
	extensionPath := filepath.Join(dir, "AGENT_INSTRUCTIONS.md")
	body := "# Our Conventions\n\n- Always X\n- Never Y\n\n```bash\nmake test\n```\n"
	if err := os.WriteFile(extensionPath, []byte(body), 0o644); err != nil {
		t.Fatalf("failed to write extension file: %v", err)
	}

	cfg := &config.Config{
		AgentInstructions: extensionPath,
	}

	_, content, err := AssembleAgentInstructions(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, body) {
		t.Errorf("instruction content should contain exact extension body, got:\n%s", content)
	}
}
