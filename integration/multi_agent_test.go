package integration

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mcastellin/asbox/internal/config"
)

func TestMultiAgent_bothAgentsInstalledInImage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	t.Cleanup(cancel)

	// Build multi-agent image — gemini requires NodeJS
	cfg := &config.Config{
		InstalledAgents: []string{"claude", "gemini"},
		ProjectName:     "integration-test",
		SDKs:            config.SDKConfig{NodeJS: "22"},
	}
	image := buildTestImageWithConfig(t, cfg)
	container := startTestContainer(ctx, t, image)

	t.Run("claude_cli_available", func(t *testing.T) {
		t.Parallel()
		_, exitCode := execInContainer(ctx, t, container, []string{"which", "claude"})
		if exitCode != 0 {
			t.Error("claude CLI not found in container")
		}
	})

	t.Run("gemini_cli_available", func(t *testing.T) {
		t.Parallel()
		_, exitCode := execInContainer(ctx, t, container, []string{"which", "gemini"})
		if exitCode != 0 {
			t.Error("gemini CLI not found in container")
		}
	})

	t.Run("claude_instruction_file_exists", func(t *testing.T) {
		t.Parallel()
		if !fileExistsInContainer(ctx, t, container, "/home/sandbox/CLAUDE.md") {
			t.Error("expected /home/sandbox/CLAUDE.md to exist")
		}
	})

	t.Run("gemini_instruction_file_exists", func(t *testing.T) {
		t.Parallel()
		if !fileExistsInContainer(ctx, t, container, "/home/sandbox/GEMINI.md") {
			t.Error("expected /home/sandbox/GEMINI.md to exist")
		}
	})
}

func TestMultiAgent_agentFlagValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Build binary once for all subtests
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "asbox")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = ".." // project root
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("building asbox binary: %v\noutput: %s", err, out)
	}

	// Config with only claude installed
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.yaml")
	configContent := "installed_agents:\n  - claude\nproject_name: test-multi-agent\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	t.Run("uninstalled_agent_exits_code_1", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(binaryPath, "run", "--agent", "gemini", "-f", configPath)
		output, err := cmd.CombinedOutput()
		outStr := string(output)

		if err == nil {
			t.Fatal("expected non-zero exit, got nil error")
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			t.Errorf("expected exit code 1, got %v", err)
		}
		if !strings.Contains(outStr, "not installed") {
			t.Errorf("expected 'not installed' in output:\n%s", outStr)
		}
	})

	t.Run("invalid_agent_name_exits_code_1", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(binaryPath, "run", "--agent", "invalidname", "-f", configPath)
		output, err := cmd.CombinedOutput()
		outStr := string(output)

		if err == nil {
			t.Fatal("expected non-zero exit, got nil error")
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			t.Errorf("expected exit code 1, got %v", err)
		}
		if !strings.Contains(outStr, "unsupported agent") {
			t.Errorf("expected 'unsupported agent' in output:\n%s", outStr)
		}
	})

	t.Run("old_style_name_rejected", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(binaryPath, "run", "--agent", "claude-code", "-f", configPath)
		output, err := cmd.CombinedOutput()
		outStr := string(output)

		if err == nil {
			t.Fatal("expected non-zero exit, got nil error")
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			t.Errorf("expected exit code 1, got %v", err)
		}
		if !strings.Contains(outStr, "unsupported agent") {
			t.Errorf("expected 'unsupported agent' in output:\n%s", outStr)
		}
	})
}

func TestMultiAgent_hostAgentConfigDisabledNoError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "asbox")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = ".."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("building asbox binary: %v\noutput: %s", err, out)
	}

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.yaml")
	// host_agent_config: false should not cause any config error
	configContent := "installed_agents:\n  - claude\nproject_name: test-host-config\nhost_agent_config: false\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cmd := exec.Command(binaryPath, "run", "-f", configPath)
	output, _ := cmd.CombinedOutput() // ignore error — docker run/build will fail
	outStr := string(output)

	// The command should proceed past config validation. It will fail at
	// docker build/run, but should NOT fail at host_agent_config processing.
	// If host_agent_config: false caused an error, we'd see it here.
	if strings.Contains(outStr, "host_agent_config") || strings.Contains(outStr, "host agent config") {
		t.Errorf("unexpected host_agent_config error in output:\n%s", outStr)
	}
}
