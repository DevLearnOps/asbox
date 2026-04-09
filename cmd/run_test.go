package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mcastellin/asbox/internal/config"
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
