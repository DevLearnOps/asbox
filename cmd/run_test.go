package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mcastellin/asbox/internal/config"
	"github.com/mcastellin/asbox/internal/mount"
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
installed_agents: [claude]
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

func TestRun_bmadReposNonexistentPath_returnsConfigError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeRunConfig(t, dir, `
installed_agents: [claude]
bmad_repos:
  - /nonexistent/bmad/repo
`)

	old := configFile
	configFile = cfgPath
	t.Cleanup(func() { configFile = old })

	r := newRootCmd()
	err := r.run("run")
	if err == nil {
		t.Fatal("expected error for nonexistent bmad_repos path, got nil")
	}

	var ce *config.ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *config.ConfigError, got %T: %v", err, err)
	}
	if got := exitCode(err); got != 1 {
		t.Errorf("exitCode = %d, want 1", got)
	}
}

func TestRun_bmadReposConfigured_assembleBmadReposCalled(t *testing.T) {
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "myrepo")
	os.Mkdir(repoDir, 0o755)

	cfgPath := writeRunConfig(t, dir, fmt.Sprintf(`
installed_agents: [claude]
bmad_repos:
  - %s
`, repoDir))

	// Parse and replicate the mount assembly logic from RunE
	cfg, err := config.Parse(cfgPath)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	mountFlags, err := mount.AssembleMounts(cfg)
	if err != nil {
		t.Fatalf("unexpected mount error: %v", err)
	}

	bmadMounts, instructionContent, err := mount.AssembleBmadRepos(cfg)
	if err != nil {
		t.Fatalf("unexpected bmad error: %v", err)
	}

	if len(bmadMounts) != 1 {
		t.Fatalf("len(bmadMounts) = %d, want 1", len(bmadMounts))
	}
	mountFlags = append(mountFlags, bmadMounts...)

	if instructionContent == "" {
		t.Error("expected non-empty instruction content when bmad_repos is configured")
	}

	// Verify mount flag is present
	found := slices.Contains(mountFlags, repoDir+":/workspace/repos/myrepo")
	if !found {
		t.Errorf("expected bmad repo mount flag, got: %v", mountFlags)
	}
}

func TestRun_bmadReposEmpty_noAdditionalMounts(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeRunConfig(t, dir, `
installed_agents: [claude]
`)

	cfg, err := config.Parse(cfgPath)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	bmadMounts, content, err := mount.AssembleBmadRepos(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bmadMounts != nil {
		t.Errorf("expected nil mounts when bmad_repos empty, got %v", bmadMounts)
	}
	if content != "" {
		t.Errorf("expected empty content when bmad_repos empty, got %q", content)
	}
}

func TestRun_nonexistentMountSource_errorMessageFormat(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeRunConfig(t, dir, `
installed_agents: [claude]
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

func TestRandomSuffix(t *testing.T) {
	pattern := regexp.MustCompile(`^[0-9a-f]{6}$`)

	first := randomSuffix()
	second := randomSuffix()

	for _, suffix := range []string{first, second} {
		if len(suffix) != 6 {
			t.Fatalf("randomSuffix() length = %d, want 6", len(suffix))
		}
		if !pattern.MatchString(suffix) {
			t.Fatalf("randomSuffix() = %q, want lowercase hex", suffix)
		}
	}

	if first == second {
		t.Fatal("randomSuffix() produced duplicate consecutive values")
	}
}

func TestRunContainerNameMatchesPattern(t *testing.T) {
	name := "asbox-my-app-" + randomSuffix()
	pattern := regexp.MustCompile(`^asbox-[a-z0-9-]+-[0-9a-f]{6}$`)

	if !pattern.MatchString(name) {
		t.Fatalf("container name %q does not match expected pattern", name)
	}
}

func TestAgentCommand_claude(t *testing.T) {
	cmd, err := agentCommand("claude")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "claude --dangerously-skip-permissions" {
		t.Errorf("agentCommand(claude) = %q, want %q", cmd, "claude --dangerously-skip-permissions")
	}
}

func TestAgentCommand_gemini(t *testing.T) {
	cmd, err := agentCommand("gemini")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "gemini -y" {
		t.Errorf("agentCommand(gemini) = %q, want %q", cmd, "gemini -y")
	}
}

func TestAgentCommand_codex(t *testing.T) {
	cmd, err := agentCommand("codex")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "codex --dangerously-bypass-approvals-and-sandbox" {
		t.Errorf("agentCommand(codex) = %q, want %q", cmd, "codex --dangerously-bypass-approvals-and-sandbox")
	}
}

func TestAgentCommand_unknown(t *testing.T) {
	_, err := agentCommand("chatgpt")
	if err == nil {
		t.Fatal("expected error for unknown agent, got nil")
	}
}

func TestAgentInstructionTarget_codex(t *testing.T) {
	target, err := agentInstructionTarget("codex")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target != "/home/sandbox/.codex/AGENTS.md" {
		t.Errorf("agentInstructionTarget(codex) = %q, want %q", target, "/home/sandbox/.codex/AGENTS.md")
	}
}

func TestAgentInstructionTarget_unknown(t *testing.T) {
	_, err := agentInstructionTarget("chatgpt")
	if err == nil {
		t.Fatal("expected error for unknown agent, got nil")
	}
}

func TestAgentCommand_oldNameRejected(t *testing.T) {
	_, err := agentCommand("claude-code")
	if err == nil {
		t.Fatal("expected error for old agent name, got nil")
	}

	_, err = agentCommand("gemini-cli")
	if err == nil {
		t.Fatal("expected error for old agent name, got nil")
	}
}

func TestAgentCommand_noShellMetacharacters(t *testing.T) {
	agents := []string{"claude", "gemini", "codex"}
	const unsafeChars = ";&|<>$`\\\"'*?(){}[]\n\r\t"

	for _, agent := range agents {
		t.Run(agent, func(t *testing.T) {
			cmd, err := agentCommand(agent)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.ContainsAny(cmd, unsafeChars) {
				t.Fatalf("agentCommand(%s) = %q contains shell metacharacters", agent, cmd)
			}
		})
	}
}
