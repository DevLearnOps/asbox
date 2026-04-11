package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutoIsolateDeps_logsVolumeCreation(t *testing.T) {
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

	t.Run("single_package_json", func(t *testing.T) {
		t.Parallel()

		// Create project with package.json
		projectDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(`{"name":"test"}`), 0644); err != nil {
			t.Fatalf("writing package.json: %v", err)
		}

		// Write config with auto_isolate_deps + mount pointing to projectDir
		configDir := t.TempDir()
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf("installed_agents: [claude]\nproject_name: test-isolate\nauto_isolate_deps: true\nmounts:\n  - source: %s\n    target: /workspace\n", projectDir)
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("writing config: %v", err)
		}

		// Run — will fail on Docker run, but scan logs appear first
		cmd := exec.Command(binPath, "run", "-f", configPath)
		output, _ := cmd.CombinedOutput() // ignore error — Docker run will fail

		outStr := string(output)
		if !strings.Contains(outStr, "auto_isolate_deps: scanned 1 mount paths, found 1 package.json files") {
			t.Errorf("missing or incorrect scan summary in output:\n%s", outStr)
		}
		if !strings.Contains(outStr, "isolating:") {
			t.Errorf("missing isolation line in output:\n%s", outStr)
		}
		if !strings.Contains(outStr, "asbox-test-isolate") {
			t.Errorf("missing volume name with project prefix in output:\n%s", outStr)
		}
		if !strings.Contains(outStr, "node_modules") {
			t.Errorf("missing node_modules in isolation output:\n%s", outStr)
		}
	})

	t.Run("monorepo_multiple_package_json", func(t *testing.T) {
		t.Parallel()

		// Create monorepo structure
		projectDir := t.TempDir()
		for _, sub := range []string{"packages/api", "packages/web"} {
			dir := filepath.Join(projectDir, sub)
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("creating dir %s: %v", sub, err)
			}
			if err := os.WriteFile(filepath.Join(dir, "package.json"), fmt.Appendf(nil, `{"name":"%s"}`, filepath.Base(sub)), 0644); err != nil {
				t.Fatalf("writing package.json in %s: %v", sub, err)
			}
		}

		configDir := t.TempDir()
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf("installed_agents: [claude]\nproject_name: test-mono\nauto_isolate_deps: true\nmounts:\n  - source: %s\n    target: /workspace\n", projectDir)
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("writing config: %v", err)
		}

		cmd := exec.Command(binPath, "run", "-f", configPath)
		output, _ := cmd.CombinedOutput()

		outStr := string(output)
		if !strings.Contains(outStr, "found 2 package.json files") {
			t.Errorf("expected 2 package.json files in scan summary:\n%s", outStr)
		}

		// Verify both isolation lines present
		isolateCount := strings.Count(outStr, "isolating:")
		if isolateCount < 2 {
			t.Errorf("expected at least 2 isolating lines, got %d:\n%s", isolateCount, outStr)
		}
	})
}
