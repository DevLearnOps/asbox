package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBmadRepos_mountsAndInstructions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Build asbox binary
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "asbox")
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	buildCmd.Dir = ".." // project root relative to integration/
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	t.Run("logs_mounting_two_repositories", func(t *testing.T) {
		t.Parallel()

		// Create two temp directories as mock repos
		repo1 := t.TempDir()
		repo2 := t.TempDir()
		if err := os.WriteFile(filepath.Join(repo1, "README.md"), []byte("repo1"), 0644); err != nil {
			t.Fatalf("writing repo1 file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repo2, "README.md"), []byte("repo2"), 0644); err != nil {
			t.Fatalf("writing repo2 file: %v", err)
		}

		configDir := t.TempDir()
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf("installed_agents: [claude]\nproject_name: test-bmad\nbmad_repos:\n  - %s\n  - %s\n", repo1, repo2)
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("writing config: %v", err)
		}

		cmd := exec.Command(binPath, "run", "-f", configPath)
		output, _ := cmd.CombinedOutput() // ignore error — Docker run will fail

		outStr := string(output)
		if !strings.Contains(outStr, "bmad_repos: mounting 2 repositories") {
			t.Errorf("missing or incorrect bmad_repos log in output:\n%s", outStr)
		}
	})

	t.Run("mount_flags_contain_workspace_repos_targets", func(t *testing.T) {
		t.Parallel()

		// Create two named mock repos for predictable basenames
		parentDir := t.TempDir()
		repo1 := filepath.Join(parentDir, "frontend")
		repo2 := filepath.Join(parentDir, "backend")
		for _, dir := range []string{repo1, repo2} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("creating dir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644); err != nil {
				t.Fatalf("writing file: %v", err)
			}
		}

		configDir := t.TempDir()
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf("installed_agents: [claude]\nproject_name: test-bmad-mounts\nbmad_repos:\n  - %s\n  - %s\n", repo1, repo2)
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("writing config: %v", err)
		}

		cmd := exec.Command(binPath, "run", "-f", configPath)
		output, _ := cmd.CombinedOutput()

		outStr := string(output)

		if !strings.Contains(outStr, "bmad_repos: mounting 2 repositories") {
			t.Errorf("expected mounting 2 repositories log:\n%s", outStr)
		}
		if !strings.Contains(outStr, "/workspace/repos/frontend") {
			t.Errorf("expected /workspace/repos/frontend mount path in output:\n%s", outStr)
		}
		if !strings.Contains(outStr, "/workspace/repos/backend") {
			t.Errorf("expected /workspace/repos/backend mount path in output:\n%s", outStr)
		}
	})
}
