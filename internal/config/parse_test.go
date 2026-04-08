package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeConfig writes a config YAML file into dir and returns its path.
func writeConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	return path
}

func TestParse_validFullConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
agent: claude-code
project_name: my-project
sdks:
  nodejs: "20"
  go: "1.25"
  python: "3.12"
packages:
  - curl
  - jq
mcp:
  - playwright
mounts:
  - source: /host/src
    target: /workspace
secrets:
  - GITHUB_TOKEN
env:
  FOO: bar
  BAZ: qux
auto_isolate_deps: true
host_agent_config:
  source: /home/user/.config/agent
  target: /root/.config/agent
bmad_repos:
  - /home/user/other-repo
`)

	parsed, err := Parse(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.Agent != "claude-code" {
		t.Errorf("Agent = %q, want %q", parsed.Agent, "claude-code")
	}
	if parsed.ProjectName != "my-project" {
		t.Errorf("ProjectName = %q, want %q", parsed.ProjectName, "my-project")
	}
	if parsed.SDKs.NodeJS != "20" {
		t.Errorf("SDKs.NodeJS = %q, want %q", parsed.SDKs.NodeJS, "20")
	}
	if parsed.SDKs.Go != "1.25" {
		t.Errorf("SDKs.Go = %q, want %q", parsed.SDKs.Go, "1.25")
	}
	if parsed.SDKs.Python != "3.12" {
		t.Errorf("SDKs.Python = %q, want %q", parsed.SDKs.Python, "3.12")
	}
	if len(parsed.Packages) != 2 || parsed.Packages[0] != "curl" || parsed.Packages[1] != "jq" {
		t.Errorf("Packages = %v, want [curl jq]", parsed.Packages)
	}
	if len(parsed.MCP) != 1 || parsed.MCP[0] != "playwright" {
		t.Errorf("MCP = %v, want [playwright]", parsed.MCP)
	}
	if len(parsed.Mounts) != 1 {
		t.Fatalf("Mounts length = %d, want 1", len(parsed.Mounts))
	}
	if parsed.Mounts[0].Source != "/host/src" {
		t.Errorf("Mounts[0].Source = %q, want %q", parsed.Mounts[0].Source, "/host/src")
	}
	if parsed.Mounts[0].Target != "/workspace" {
		t.Errorf("Mounts[0].Target = %q, want %q", parsed.Mounts[0].Target, "/workspace")
	}
	if len(parsed.Secrets) != 1 || parsed.Secrets[0] != "GITHUB_TOKEN" {
		t.Errorf("Secrets = %v, want [GITHUB_TOKEN]", parsed.Secrets)
	}
	if parsed.Env["FOO"] != "bar" || parsed.Env["BAZ"] != "qux" {
		t.Errorf("Env = %v, want map[FOO:bar BAZ:qux]", parsed.Env)
	}
	if !parsed.AutoIsolateDeps {
		t.Error("AutoIsolateDeps = false, want true")
	}
	if parsed.HostAgentConfig == nil {
		t.Fatal("HostAgentConfig is nil, want non-nil")
	}
	if parsed.HostAgentConfig.Source != "/home/user/.config/agent" {
		t.Errorf("HostAgentConfig.Source = %q, want %q", parsed.HostAgentConfig.Source, "/home/user/.config/agent")
	}
	if len(parsed.BmadRepos) != 1 || parsed.BmadRepos[0] != "/home/user/other-repo" {
		t.Errorf("BmadRepos = %v, want [/home/user/other-repo]", parsed.BmadRepos)
	}
}

func TestParse_validMinimalConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `agent: gemini-cli`)

	parsed, err := Parse(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.Agent != "gemini-cli" {
		t.Errorf("Agent = %q, want %q", parsed.Agent, "gemini-cli")
	}
	// project_name should be derived from parent dir
	if parsed.ProjectName == "" {
		t.Error("ProjectName should be derived, got empty")
	}
	if parsed.Packages != nil {
		t.Errorf("Packages = %v, want nil", parsed.Packages)
	}
	if parsed.Mounts != nil {
		t.Errorf("Mounts = %v, want nil", parsed.Mounts)
	}
	if parsed.HostAgentConfig != nil {
		t.Errorf("HostAgentConfig = %v, want nil", parsed.HostAgentConfig)
	}
	if parsed.AutoIsolateDeps {
		t.Error("AutoIsolateDeps = true, want false")
	}
}

func TestParse_fileNotFound(t *testing.T) {
	_, err := Parse("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Field != "" {
		t.Errorf("Field = %q, want empty", ce.Field)
	}
	want := "config file not found at /nonexistent/path/config.yaml. Run 'asbox init' to create one"
	if ce.Error() != want {
		t.Errorf("error = %q, want %q", ce.Error(), want)
	}
}

func TestParse_invalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `[invalid yaml: {{`)

	_, err := Parse(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Field != "" {
		t.Errorf("Field = %q, want empty", ce.Field)
	}
}

func TestParse_emptyAgent(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `agent: ""`)

	_, err := Parse(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Field != "agent" {
		t.Errorf("Field = %q, want %q", ce.Field, "agent")
	}
	if ce.Msg != "required field is empty. Set agent to 'claude-code' or 'gemini-cli'" {
		t.Errorf("Msg = %q", ce.Msg)
	}
}

func TestParse_invalidAgent(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `agent: chatgpt`)

	_, err := Parse(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Field != "agent" {
		t.Errorf("Field = %q, want %q", ce.Field, "agent")
	}
	if ce.Msg != "unsupported agent 'chatgpt'. Use 'claude-code' or 'gemini-cli'" {
		t.Errorf("Msg = %q", ce.Msg)
	}
}

func TestParse_emptyMountSource(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
agent: claude-code
mounts:
  - source: ""
    target: /workspace
`)

	_, err := Parse(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Field != "mounts[0].source" {
		t.Errorf("Field = %q, want %q", ce.Field, "mounts[0].source")
	}
}

func TestParse_emptyMountTarget(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
agent: claude-code
mounts:
  - source: /host/src
    target: ""
`)

	_, err := Parse(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Field != "mounts[0].target" {
		t.Errorf("Field = %q, want %q", ce.Field, "mounts[0].target")
	}
}

func TestParse_projectNameDerivation(t *testing.T) {
	// Create structure: <tmpdir>/my-cool-project/.asbox/config.yaml
	tmp := t.TempDir()
	projectDir := filepath.Join(tmp, "my-cool-project", ".asbox")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	cfgPath := filepath.Join(projectDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(`agent: claude-code`), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	parsed, err := Parse(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.ProjectName != "my-cool-project" {
		t.Errorf("ProjectName = %q, want %q", parsed.ProjectName, "my-cool-project")
	}
}

func TestParse_projectNameSanitization(t *testing.T) {
	// Create structure: <tmpdir>/My Cool Project!/.asbox/config.yaml
	tmp := t.TempDir()
	projectDir := filepath.Join(tmp, "My Cool Project!", ".asbox")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	cfgPath := filepath.Join(projectDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(`agent: claude-code`), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	parsed, err := Parse(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.ProjectName != "my-cool-project" {
		t.Errorf("ProjectName = %q, want %q", parsed.ProjectName, "my-cool-project")
	}
}

func TestParse_relativeMountPaths(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
agent: claude-code
mounts:
  - source: "."
    target: /workspace
  - source: "../sibling"
    target: /other
`)

	parsed, err := Parse(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	absDir, _ := filepath.Abs(dir)

	if parsed.Mounts[0].Source != absDir {
		t.Errorf("Mounts[0].Source = %q, want %q", parsed.Mounts[0].Source, absDir)
	}
	expectedSibling := filepath.Join(absDir, "..", "sibling")
	if parsed.Mounts[1].Source != expectedSibling {
		t.Errorf("Mounts[1].Source = %q, want %q", parsed.Mounts[1].Source, expectedSibling)
	}
}

func TestParse_absoluteMountPaths(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
agent: claude-code
mounts:
  - source: /absolute/path
    target: /workspace
`)

	parsed, err := Parse(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.Mounts[0].Source != "/absolute/path" {
		t.Errorf("Mounts[0].Source = %q, want %q", parsed.Mounts[0].Source, "/absolute/path")
	}
}

func TestParse_relativeMountTarget_returnsError(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
agent: claude-code
mounts:
  - source: /host/src
    target: workspace
`)

	_, err := Parse(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Field != "mounts[0].target" {
		t.Errorf("Field = %q, want %q", ce.Field, "mounts[0].target")
	}
}

func TestParse_tildeExpansion(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
agent: claude-code
mounts:
  - source: "~/projects"
    target: /workspace
`)

	parsed, err := Parse(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "projects")
	if parsed.Mounts[0].Source != expected {
		t.Errorf("Mounts[0].Source = %q, want %q", parsed.Mounts[0].Source, expected)
	}
}

func TestParse_projectNameFallback(t *testing.T) {
	// Config at root-like path where dir name sanitizes to empty
	tmp := t.TempDir()
	projectDir := filepath.Join(tmp, "!!!", ".asbox")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	cfgPath := filepath.Join(projectDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(`agent: claude-code`), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	parsed, err := Parse(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.ProjectName != "asbox" {
		t.Errorf("ProjectName = %q, want %q", parsed.ProjectName, "asbox")
	}
}

func TestParse_hostAgentConfigValidation(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
agent: claude-code
host_agent_config:
  source: ""
  target: /root/.config
`)

	_, err := Parse(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Field != "host_agent_config.source" {
		t.Errorf("Field = %q, want %q", ce.Field, "host_agent_config.source")
	}
}
