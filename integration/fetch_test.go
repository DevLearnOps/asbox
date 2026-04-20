package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetch_prePhaseAnchorAndSummary_whenFetchSet(t *testing.T) {
	binPath := buildAsboxBinary(t)

	repo1, bare1 := initRepoWithOrigin(t, "repo-one")
	repo2, bare2 := initRepoWithOrigin(t, "repo-two")
	pushCommitToRemote(t, bare1, "remote-one.txt", "one\n")
	pushCommitToRemote(t, bare2, "remote-two.txt", "two\n")

	projectDir := t.TempDir()
	configPath := writeConfigInProject(t, projectDir, fmt.Sprintf(`
installed_agents: [claude]
bmad_repos:
  - %s
  - %s
`, repo1, repo2))

	cmd := exec.Command(binPath, "run", "--fetch", "-f", configPath)
	output, _ := cmd.CombinedOutput()
	outStr := string(output)

	// projectDir is included unconditionally; since it isn't a git repo it
	// surfaces as a "not a git repo" skip in the summary.
	if !strings.Contains(outStr, "fetching 3 repositories (timeout 60s each, 4 concurrent)...") {
		t.Fatalf("missing fetch anchor line:\n%s", outStr)
	}
	if !strings.Contains(outStr, "fetched 2/3 repositories (1 not a git repo)") {
		t.Fatalf("missing fetch summary line:\n%s", outStr)
	}
}

func TestFetch_noFetchFlag_noFetchPhaseOutput(t *testing.T) {
	binPath := buildAsboxBinary(t)

	repo1, _ := initRepoWithOrigin(t, "repo-one")
	repo2, _ := initRepoWithOrigin(t, "repo-two")

	projectDir := t.TempDir()
	configPath := writeConfigInProject(t, projectDir, fmt.Sprintf(`
installed_agents: [claude]
bmad_repos:
  - %s
  - %s
`, repo1, repo2))

	cmd := exec.Command(binPath, "run", "-f", configPath)
	output, _ := cmd.CombinedOutput()
	outStr := string(output)

	if strings.Contains(outStr, "fetching ") || strings.Contains(outStr, "fetched ") {
		t.Fatalf("unexpected fetch output without --fetch:\n%s", outStr)
	}
}

func buildAsboxBinary(t *testing.T) string {
	t.Helper()

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "asbox")
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	buildCmd.Dir = ".."
	buildCmd.Env = os.Environ()
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return binPath
}

func writeConfigInProject(t *testing.T, projectDir, content string) string {
	t.Helper()

	asboxDir := filepath.Join(projectDir, ".asbox")
	if err := os.MkdirAll(asboxDir, 0o755); err != nil {
		t.Fatalf("creating .asbox dir: %v", err)
	}
	configPath := filepath.Join(asboxDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	return configPath
}

func initRepoWithOrigin(t *testing.T, name string) (string, string) {
	t.Helper()

	base := t.TempDir()
	seed := filepath.Join(base, name+"-seed")
	bare := filepath.Join(base, name+"-origin.git")
	clone := filepath.Join(base, name)

	if err := os.MkdirAll(seed, 0o755); err != nil {
		t.Fatalf("creating seed repo dir: %v", err)
	}

	gitRun(t, seed, "init", "-b", "main")
	configureGitUser(t, seed)
	writeFile(t, filepath.Join(seed, "README.md"), name+"\n")
	gitRun(t, seed, "add", "README.md")
	gitRun(t, seed, "commit", "-m", "initial commit")

	gitRun(t, base, "init", "--bare", "-b", "main", bare)
	gitRun(t, seed, "remote", "add", "origin", bare)
	gitRun(t, seed, "push", "-u", "origin", "main")

	gitRun(t, base, "clone", bare, clone)
	configureGitUser(t, clone)
	return clone, bare
}

func pushCommitToRemote(t *testing.T, barePath, filename, content string) {
	t.Helper()

	writer := filepath.Join(t.TempDir(), "writer")
	gitRun(t, filepath.Dir(writer), "clone", barePath, writer)
	configureGitUser(t, writer)
	writeFile(t, filepath.Join(writer, filename), content)
	gitRun(t, writer, "add", filename)
	gitRun(t, writer, "commit", "-m", "update remote")
	gitRun(t, writer, "push", "origin", "main")
}

func configureGitUser(t *testing.T, dir string) {
	t.Helper()
	gitRun(t, dir, "config", "user.name", "Test User")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	gitRun(t, dir, "config", "tag.gpgsign", "false")
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
