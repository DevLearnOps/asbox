package mount

import (
	"errors"
	"os"
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

// --- AssembleHostAgentConfig tests ---

func TestAssembleHostAgentConfig_claudeWithExistingDir(t *testing.T) {
	// Create a fake ~/.claude directory to test against
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.Mkdir(claudeDir, 0o755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	// Temporarily override the registry source to use our test dir
	origMapping := config.AgentConfigRegistry["claude"]
	config.AgentConfigRegistry["claude"] = config.AgentConfigMapping{
		Source: claudeDir,
		Target: "/opt/claude-config",
		EnvVar: "CLAUDE_CONFIG_DIR",
		EnvVal: "/opt/claude-config",
	}
	t.Cleanup(func() { config.AgentConfigRegistry["claude"] = origMapping })

	mountFlag, envKey, envVal, err := AssembleHostAgentConfig("claude", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mountFlag != claudeDir+":/opt/claude-config" {
		t.Errorf("mountFlag = %q, want %q", mountFlag, claudeDir+":/opt/claude-config")
	}
	if envKey != "CLAUDE_CONFIG_DIR" {
		t.Errorf("envKey = %q, want %q", envKey, "CLAUDE_CONFIG_DIR")
	}
	if envVal != "/opt/claude-config" {
		t.Errorf("envVal = %q, want %q", envVal, "/opt/claude-config")
	}
}

func TestAssembleHostAgentConfig_disabledExplicitly(t *testing.T) {
	disabled := false
	mountFlag, envKey, envVal, err := AssembleHostAgentConfig("claude", &disabled)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mountFlag != "" || envKey != "" || envVal != "" {
		t.Errorf("expected empty results when disabled, got mount=%q env=%q val=%q", mountFlag, envKey, envVal)
	}
}

func TestAssembleHostAgentConfig_enabledNil(t *testing.T) {
	// When enabled is nil (default), should attempt mount if dir exists
	dir := t.TempDir()
	geminiDir := filepath.Join(dir, ".gemini")
	if err := os.Mkdir(geminiDir, 0o755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	origMapping := config.AgentConfigRegistry["gemini"]
	config.AgentConfigRegistry["gemini"] = config.AgentConfigMapping{
		Source: geminiDir,
		Target: "/opt/gemini-home/.gemini",
		EnvVar: "GEMINI_CLI_HOME",
		EnvVal: "/opt/gemini-home",
	}
	t.Cleanup(func() { config.AgentConfigRegistry["gemini"] = origMapping })

	mountFlag, envKey, envVal, err := AssembleHostAgentConfig("gemini", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mountFlag != geminiDir+":/opt/gemini-home/.gemini" {
		t.Errorf("mountFlag = %q, want %q", mountFlag, geminiDir+":/opt/gemini-home/.gemini")
	}
	if envKey != "GEMINI_CLI_HOME" {
		t.Errorf("envKey = %q, want %q", envKey, "GEMINI_CLI_HOME")
	}
	if envVal != "/opt/gemini-home" {
		t.Errorf("envVal = %q, want %q", envVal, "/opt/gemini-home")
	}
}

func TestAssembleHostAgentConfig_missingDirSilentSkip(t *testing.T) {
	// Point to a non-existent directory — should silently skip (AC9)
	origMapping := config.AgentConfigRegistry["claude"]
	config.AgentConfigRegistry["claude"] = config.AgentConfigMapping{
		Source: "/nonexistent/dir/that/does/not/exist",
		Target: "/opt/claude-config",
		EnvVar: "CLAUDE_CONFIG_DIR",
		EnvVal: "/opt/claude-config",
	}
	t.Cleanup(func() { config.AgentConfigRegistry["claude"] = origMapping })

	mountFlag, envKey, envVal, err := AssembleHostAgentConfig("claude", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mountFlag != "" || envKey != "" || envVal != "" {
		t.Errorf("expected empty results for missing dir, got mount=%q env=%q val=%q", mountFlag, envKey, envVal)
	}
}

func TestAssembleHostAgentConfig_unknownAgent(t *testing.T) {
	mountFlag, envKey, envVal, err := AssembleHostAgentConfig("unknown-agent", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mountFlag != "" || envKey != "" || envVal != "" {
		t.Errorf("expected empty results for unknown agent, got mount=%q env=%q val=%q", mountFlag, envKey, envVal)
	}
}

func TestAssembleHostAgentConfig_sourceIsFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(file, []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	origMapping := config.AgentConfigRegistry["claude"]
	config.AgentConfigRegistry["claude"] = config.AgentConfigMapping{
		Source: file,
		Target: "/opt/claude-config",
		EnvVar: "CLAUDE_CONFIG_DIR",
		EnvVal: "/opt/claude-config",
	}
	t.Cleanup(func() { config.AgentConfigRegistry["claude"] = origMapping })

	mountFlag, envKey, envVal, err := AssembleHostAgentConfig("claude", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// File (not directory) should be silently skipped
	if mountFlag != "" || envKey != "" || envVal != "" {
		t.Errorf("expected empty results when source is a file, got mount=%q env=%q val=%q", mountFlag, envKey, envVal)
	}
}
