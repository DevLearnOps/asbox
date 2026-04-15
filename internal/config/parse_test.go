package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
installed_agents:
  - claude
default_agent: claude
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
host_agent_config: true
bmad_repos:
  - /home/user/other-repo
`)

	parsed, err := Parse(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed.InstalledAgents) != 1 || parsed.InstalledAgents[0] != "claude" {
		t.Errorf("InstalledAgents = %v, want [claude]", parsed.InstalledAgents)
	}
	if parsed.DefaultAgent != "claude" {
		t.Errorf("DefaultAgent = %q, want %q", parsed.DefaultAgent, "claude")
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
	if parsed.HostAgentConfig == nil || !*parsed.HostAgentConfig {
		t.Error("HostAgentConfig should be non-nil true")
	}
	if len(parsed.BmadRepos) != 1 || parsed.BmadRepos[0] != "/home/user/other-repo" {
		t.Errorf("BmadRepos = %v, want [/home/user/other-repo]", parsed.BmadRepos)
	}
}

func TestParse_validMinimalConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
installed_agents: [gemini]
sdks:
  nodejs: "22"
`)

	parsed, err := Parse(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed.InstalledAgents) != 1 || parsed.InstalledAgents[0] != "gemini" {
		t.Errorf("InstalledAgents = %v, want [gemini]", parsed.InstalledAgents)
	}
	if parsed.DefaultAgent != "gemini" {
		t.Errorf("DefaultAgent = %q, want %q (should default to first installed)", parsed.DefaultAgent, "gemini")
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

func TestParse_multiAgentConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
installed_agents:
  - claude
  - gemini
default_agent: gemini
sdks:
  nodejs: "22"
`)

	parsed, err := Parse(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed.InstalledAgents) != 2 {
		t.Fatalf("InstalledAgents length = %d, want 2", len(parsed.InstalledAgents))
	}
	if parsed.InstalledAgents[0] != "claude" || parsed.InstalledAgents[1] != "gemini" {
		t.Errorf("InstalledAgents = %v, want [claude gemini]", parsed.InstalledAgents)
	}
	if parsed.DefaultAgent != "gemini" {
		t.Errorf("DefaultAgent = %q, want %q", parsed.DefaultAgent, "gemini")
	}
}

func TestParse_defaultAgentDefaultsToFirst(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
installed_agents:
  - gemini
  - claude
sdks:
  nodejs: "22"
`)

	parsed, err := Parse(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.DefaultAgent != "gemini" {
		t.Errorf("DefaultAgent = %q, want %q (first in installed_agents)", parsed.DefaultAgent, "gemini")
	}
}

func TestParse_defaultAgentNotInstalled(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
installed_agents: [claude]
default_agent: gemini
`)

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

func TestParse_emptyInstalledAgents(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `installed_agents: []`)

	_, err := Parse(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Field != "installed_agents" {
		t.Errorf("Field = %q, want %q", ce.Field, "installed_agents")
	}
}

func TestParse_missingInstalledAgents(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `project_name: foo`)

	_, err := Parse(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Field != "installed_agents" {
		t.Errorf("Field = %q, want %q", ce.Field, "installed_agents")
	}
}

func TestParse_invalidAgentName(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `installed_agents: [chatgpt]`)

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
	if ce.Msg != "unsupported agent 'chatgpt'. Use 'claude', 'gemini', or 'codex'" {
		t.Errorf("Msg = %q", ce.Msg)
	}
}

func TestParse_oldAgentNameRejected(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `installed_agents: [claude-code]`)

	_, err := Parse(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Msg != "unsupported agent 'claude-code'. Use 'claude', 'gemini', or 'codex'" {
		t.Errorf("Msg = %q", ce.Msg)
	}
}

func TestParse_duplicateInstalledAgent(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `installed_agents: [claude, claude]`)

	_, err := Parse(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Field != "installed_agents" {
		t.Errorf("Field = %q, want %q", ce.Field, "installed_agents")
	}
	if ce.Msg != "duplicate agent 'claude'" {
		t.Errorf("Msg = %q", ce.Msg)
	}
}

func TestParse_geminiRequiresNodejs(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `installed_agents: [gemini]`)

	_, err := Parse(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Field != "installed_agents" {
		t.Errorf("Field = %q, want %q", ce.Field, "installed_agents")
	}
	if ce.Msg != "agent 'gemini' requires sdks.nodejs to be configured" {
		t.Errorf("Msg = %q", ce.Msg)
	}
}

func TestParse_hostAgentConfigBoolean(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
installed_agents: [claude]
host_agent_config: false
`)

	parsed, err := Parse(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.HostAgentConfig == nil {
		t.Fatal("HostAgentConfig is nil, want non-nil false")
	}
	if *parsed.HostAgentConfig {
		t.Error("HostAgentConfig = true, want false")
	}
}

func TestParse_emptyMountSource(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
installed_agents: [claude]
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
installed_agents: [claude]
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
	if err := os.WriteFile(cfgPath, []byte(`installed_agents: [claude]`), 0644); err != nil {
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
	if err := os.WriteFile(cfgPath, []byte(`installed_agents: [claude]`), 0644); err != nil {
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
installed_agents: [claude]
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
installed_agents: [claude]
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
installed_agents: [claude]
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
installed_agents: [claude]
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
	if err := os.WriteFile(cfgPath, []byte(`installed_agents: [claude]`), 0644); err != nil {
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

func TestParse_sdkVersionValidation(t *testing.T) {
	tests := []struct {
		name      string
		sdkField  string
		value     string
		wantField string
		wantMsg   string
		wantErr   bool
	}{
		{name: "nodejs numeric", sdkField: "nodejs", value: "22"},
		{name: "nodejs dotted", sdkField: "nodejs", value: "22.13.0"},
		{name: "go semver", sdkField: "go", value: "1.23.1"},
		{name: "python dotted", sdkField: "python", value: "3.12"},
		{name: "nodejs rc", sdkField: "nodejs", value: "22-rc1"},
		{name: "go build metadata", sdkField: "go", value: "1.23+build.1"},
		{name: "nodejs shell metacharacters", sdkField: "nodejs", value: "22; rm -rf /", wantField: "sdks.nodejs", wantMsg: `contains invalid characters "22; rm -rf /". Allowed: letters, digits, dots, hyphens, plus signs`, wantErr: true},
		{name: "go command substitution", sdkField: "go", value: "1.23$(curl evil)", wantField: "sdks.go", wantMsg: `contains invalid characters "1.23$(curl evil)". Allowed: letters, digits, dots, hyphens, plus signs`, wantErr: true},
		{name: "python newline injection", sdkField: "python", value: "3.12\nRUN evil", wantField: "sdks.python", wantMsg: "contains invalid characters", wantErr: true},
		{name: "nodejs trailing space", sdkField: "nodejs", value: "22 ", wantField: "sdks.nodejs", wantMsg: `contains invalid characters "22 ". Allowed: letters, digits, dots, hyphens, plus signs`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := writeConfig(t, dir, `
installed_agents: [claude]
sdks:
  `+tt.sdkField+`: "`+tt.value+`"
`)

			parsed, err := Parse(cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				var ce *ConfigError
				if !errors.As(err, &ce) {
					t.Fatalf("expected *ConfigError, got %T: %v", err, err)
				}
				if ce.Field != tt.wantField {
					t.Errorf("Field = %q, want %q", ce.Field, tt.wantField)
				}
				if tt.wantMsg != "" && !strings.Contains(ce.Msg, tt.wantMsg) {
					t.Errorf("Msg = %q, want substring %q", ce.Msg, tt.wantMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if parsed == nil {
				t.Fatal("parsed config is nil")
			}
		})
	}
}

func TestParse_packageNameValidation(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantField string
		wantMsg   string
		wantErr   bool
	}{
		{name: "simple package", value: "vim"},
		{name: "hyphenated package", value: "build-essential"},
		{name: "library package", value: "libpq-dev"},
		{name: "dotted package", value: "python3.12-venv"},
		{name: "plus signs", value: "lib++-dev"},
		{name: "epoch prefix", value: "5:vim"},
		{name: "version pinned", value: "vim=2:8.2.3995-1ubuntu2.22"},
		{name: "empty package", value: "", wantField: "packages[0]", wantMsg: "package name cannot be empty. Allowed: alphanumeric, hyphens, dots, plus signs, equals signs, colons (apt format)", wantErr: true},
		{name: "shell metacharacters", value: "vim; curl evil", wantField: "packages[0]", wantMsg: `contains invalid characters "vim; curl evil". Allowed: alphanumeric, hyphens, dots, plus signs, equals signs, colons (apt format)`, wantErr: true},
		{name: "leading hyphen", value: "-invalid", wantField: "packages[0]", wantMsg: `contains invalid characters "-invalid". Allowed: alphanumeric, hyphens, dots, plus signs, equals signs, colons (apt format)`, wantErr: true},
		{name: "leading dot", value: ".invalid", wantField: "packages[0]", wantMsg: `contains invalid characters ".invalid". Allowed: alphanumeric, hyphens, dots, plus signs, equals signs, colons (apt format)`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := writeConfig(t, dir, `
installed_agents: [claude]
packages:
  - "`+tt.value+`"
`)

			parsed, err := Parse(cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				var ce *ConfigError
				if !errors.As(err, &ce) {
					t.Fatalf("expected *ConfigError, got %T: %v", err, err)
				}
				if ce.Field != tt.wantField {
					t.Errorf("Field = %q, want %q", ce.Field, tt.wantField)
				}
				if tt.wantMsg != "" && !strings.Contains(ce.Msg, tt.wantMsg) {
					t.Errorf("Msg = %q, want substring %q", ce.Msg, tt.wantMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(parsed.Packages) != 1 || parsed.Packages[0] != tt.value {
				t.Errorf("Packages = %v, want [%s]", parsed.Packages, tt.value)
			}
		})
	}
}

func TestParse_explicitProjectNameSanitized(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		wantName string
	}{
		{name: "already sanitized", value: "my-project", wantName: "my-project"},
		{name: "uppercase and underscore", value: "My_Project", wantName: "my-project"},
		{name: "spaces and punctuation", value: "PROJECT 123!", wantName: "project-123"},
		{name: "leading and trailing hyphens", value: "---leading---", wantName: "leading"},
		{name: "sanitizes to empty", value: "!!!", wantName: "asbox"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := writeConfig(t, dir, `
installed_agents: [claude]
project_name: "`+tt.value+`"
`)

			parsed, err := Parse(cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if parsed.ProjectName != tt.wantName {
				t.Errorf("ProjectName = %q, want %q", parsed.ProjectName, tt.wantName)
			}
		})
	}
}

func TestParse_mcpPlaywrightWithoutNodeJS(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
installed_agents: [claude]
mcp:
  - playwright
`)

	_, err := Parse(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Field != "mcp" {
		t.Errorf("Field = %q, want %q", ce.Field, "mcp")
	}
	if ce.Msg != "mcp server 'playwright' requires sdks.nodejs to be configured" {
		t.Errorf("Msg = %q", ce.Msg)
	}
}

func TestParse_mcpPlaywrightWithNodeJS(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
installed_agents: [claude]
sdks:
  nodejs: "22"
mcp:
  - playwright
`)

	parsed, err := Parse(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parsed.MCP) != 1 || parsed.MCP[0] != "playwright" {
		t.Errorf("MCP = %v, want [playwright]", parsed.MCP)
	}
}

func TestParse_mcpUnknownServer(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
installed_agents: [claude]
mcp:
  - unknown-server
`)

	_, err := Parse(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Field != "mcp" {
		t.Errorf("Field = %q, want %q", ce.Field, "mcp")
	}
	if ce.Msg != "unsupported MCP server 'unknown-server'. Supported: playwright" {
		t.Errorf("Msg = %q", ce.Msg)
	}
}

func TestParse_mcpDuplicate(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
installed_agents: [claude]
sdks:
  nodejs: "22"
mcp:
  - playwright
  - playwright
`)

	_, err := Parse(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Field != "mcp" {
		t.Errorf("Field = %q, want %q", ce.Field, "mcp")
	}
	if ce.Msg != "duplicate MCP server 'playwright'" {
		t.Errorf("Msg = %q", ce.Msg)
	}
}

func TestParse_mcpEmptyList(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
installed_agents: [claude]
mcp: []
`)

	parsed, err := Parse(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parsed.MCP) != 0 {
		t.Errorf("MCP = %v, want empty", parsed.MCP)
	}
}

func TestValidateAgent_valid(t *testing.T) {
	if err := ValidateAgent("claude"); err != nil {
		t.Errorf("unexpected error for claude: %v", err)
	}
	if err := ValidateAgent("gemini"); err != nil {
		t.Errorf("unexpected error for gemini: %v", err)
	}
	if err := ValidateAgent("codex"); err != nil {
		t.Errorf("unexpected error for codex: %v", err)
	}
}

func TestValidateAgent_invalid(t *testing.T) {
	err := ValidateAgent("claude-code")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Msg != "unsupported agent 'claude-code'. Use 'claude', 'gemini', or 'codex'" {
		t.Errorf("Msg = %q", ce.Msg)
	}
}

func TestValidateAgentInstalled_installed(t *testing.T) {
	if err := ValidateAgentInstalled("claude", []string{"claude", "gemini"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateAgentInstalled_notInstalled(t *testing.T) {
	err := ValidateAgentInstalled("gemini", []string{"claude"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Msg != "agent 'gemini' is not installed in the image. Installed agents: claude. Add it to installed_agents in config or choose a different agent" {
		t.Errorf("Msg = %q", ce.Msg)
	}
}

func TestParse_codexInValidAgents(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
installed_agents: [codex]
sdks:
  nodejs: "22"
`)

	parsed, err := Parse(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parsed.InstalledAgents) != 1 || parsed.InstalledAgents[0] != "codex" {
		t.Errorf("InstalledAgents = %v, want [codex]", parsed.InstalledAgents)
	}
	if parsed.DefaultAgent != "codex" {
		t.Errorf("DefaultAgent = %q, want %q", parsed.DefaultAgent, "codex")
	}
}

func TestParse_codexRequiresNodejs(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `installed_agents: [codex]`)

	_, err := Parse(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Field != "installed_agents" {
		t.Errorf("Field = %q, want %q", ce.Field, "installed_agents")
	}
	if ce.Msg != "agent 'codex' requires sdks.nodejs to be configured" {
		t.Errorf("Msg = %q", ce.Msg)
	}
}

func TestParse_codexInMultiAgentConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, `
installed_agents:
  - claude
  - gemini
  - codex
default_agent: codex
sdks:
  nodejs: "22"
`)

	parsed, err := Parse(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed.InstalledAgents) != 3 {
		t.Fatalf("InstalledAgents length = %d, want 3", len(parsed.InstalledAgents))
	}
	if parsed.InstalledAgents[2] != "codex" {
		t.Errorf("InstalledAgents[2] = %q, want %q", parsed.InstalledAgents[2], "codex")
	}
	if parsed.DefaultAgent != "codex" {
		t.Errorf("DefaultAgent = %q, want %q", parsed.DefaultAgent, "codex")
	}
}
