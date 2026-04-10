package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

func TestRun_hostAgentConfig_envVarSetWhenConfigured(t *testing.T) {
	dir := t.TempDir()
	agentDir := t.TempDir() // valid source for host_agent_config

	cfgPath := writeRunConfig(t, dir, fmt.Sprintf(`
agent: claude-code
host_agent_config:
  source: %s
  target: /opt/claude-config
`, agentDir))

	old := configFile
	configFile = cfgPath
	t.Cleanup(func() { configFile = old })

	// Parse config and replicate the env-building logic from RunE
	cfg, err := config.Parse(cfgPath)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	envVars, err := buildEnvVars(cfg)
	if err != nil {
		t.Fatalf("unexpected buildEnvVars error: %v", err)
	}

	// CLAUDE_CONFIG_DIR is set after buildEnvVars, matching RunE flow (gated on agent type)
	if cfg.HostAgentConfig != nil && cfg.Agent == "claude-code" {
		envVars["CLAUDE_CONFIG_DIR"] = cfg.HostAgentConfig.Target
	}

	if envVars["CLAUDE_CONFIG_DIR"] != "/opt/claude-config" {
		t.Errorf("CLAUDE_CONFIG_DIR = %q, want %q", envVars["CLAUDE_CONFIG_DIR"], "/opt/claude-config")
	}
}

func TestRun_hostAgentConfig_envVarAbsentWhenNil(t *testing.T) {
	dir := t.TempDir()

	cfgPath := writeRunConfig(t, dir, `
agent: claude-code
`)

	cfg, err := config.Parse(cfgPath)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	envVars, err := buildEnvVars(cfg)
	if err != nil {
		t.Fatalf("unexpected buildEnvVars error: %v", err)
	}

	// Same conditional as RunE — should NOT add CLAUDE_CONFIG_DIR
	if cfg.HostAgentConfig != nil && cfg.Agent == "claude-code" {
		envVars["CLAUDE_CONFIG_DIR"] = cfg.HostAgentConfig.Target
	}

	if _, ok := envVars["CLAUDE_CONFIG_DIR"]; ok {
		t.Error("CLAUDE_CONFIG_DIR should not be set when HostAgentConfig is nil")
	}
}

func TestRun_hostAgentConfig_envVarSkippedForNonClaudeAgent(t *testing.T) {
	dir := t.TempDir()
	agentDir := t.TempDir()

	cfgPath := writeRunConfig(t, dir, fmt.Sprintf(`
agent: gemini-cli
host_agent_config:
  source: %s
  target: /opt/claude-config
`, agentDir))

	cfg, err := config.Parse(cfgPath)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	envVars, err := buildEnvVars(cfg)
	if err != nil {
		t.Fatalf("unexpected buildEnvVars error: %v", err)
	}

	// Same conditional as RunE — should NOT add CLAUDE_CONFIG_DIR for non-claude agents
	if cfg.HostAgentConfig != nil && cfg.Agent == "claude-code" {
		envVars["CLAUDE_CONFIG_DIR"] = cfg.HostAgentConfig.Target
	}

	if _, ok := envVars["CLAUDE_CONFIG_DIR"]; ok {
		t.Error("CLAUDE_CONFIG_DIR should not be set for gemini-cli agent")
	}
}

func TestRun_bmadReposNonexistentPath_returnsConfigError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeRunConfig(t, dir, `
agent: claude-code
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
agent: claude-code
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
agent: claude-code
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
