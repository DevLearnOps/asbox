package config

import (
	"encoding/json"
	"slices"
)

// MCPServerEntry represents an MCP server in the manifest.
type MCPServerEntry struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// MCPServerRegistry maps supported MCP server names to their manifest entries.
var MCPServerRegistry = map[string]MCPServerEntry{
	"playwright": {
		Type:    "stdio",
		Command: "npx",
		Args:    []string{"-y", "@playwright/mcp"},
	},
}

// AgentConfigMapping maps an agent to its host config directory, container mount target, and env var.
type AgentConfigMapping struct {
	Source string // host path, e.g. "~/.claude"
	Target string // container mount path, e.g. "/opt/claude-config"
	EnvVar string // env var name, e.g. "CLAUDE_CONFIG_DIR"
	EnvVal string // env var value, e.g. "/opt/claude-config"
}

// AgentConfigRegistry maps supported agent names to their host config mappings.
var AgentConfigRegistry = map[string]AgentConfigMapping{
	"claude": {Source: "~/.claude", Target: "/opt/claude-config", EnvVar: "CLAUDE_CONFIG_DIR", EnvVal: "/opt/claude-config"},
	"gemini": {Source: "~/.gemini", Target: "/opt/gemini-home/.gemini", EnvVar: "GEMINI_CLI_HOME", EnvVal: "/opt/gemini-home"},
}

// Config represents the top-level asbox configuration.
type Config struct {
	InstalledAgents []string          `yaml:"installed_agents"`
	DefaultAgent    string            `yaml:"default_agent"`
	ProjectName     string            `yaml:"project_name"`
	SDKs            SDKConfig         `yaml:"sdks"`
	Packages        []string          `yaml:"packages"`
	MCP             []string          `yaml:"mcp"`
	Mounts          []MountConfig     `yaml:"mounts"`
	Secrets         []string          `yaml:"secrets"`
	Env             map[string]string `yaml:"env"`
	AutoIsolateDeps bool              `yaml:"auto_isolate_deps"`
	HostAgentConfig *bool             `yaml:"host_agent_config"`
	BmadRepos       []string          `yaml:"bmad_repos"`
}

// SDKConfig specifies SDK versions to install in the sandbox.
type SDKConfig struct {
	NodeJS string `yaml:"nodejs"`
	Go     string `yaml:"go"`
	Python string `yaml:"python"`
}

// HasMCP returns true if the named MCP server is in the config.
func (c *Config) HasMCP(name string) bool {
	return slices.Contains(c.MCP, name)
}

// MCPManifestJSON returns the MCP manifest JSON string for embedding in the Dockerfile.
func (c *Config) MCPManifestJSON() string {
	servers := make(map[string]MCPServerEntry)
	for _, name := range c.MCP {
		if entry, ok := MCPServerRegistry[name]; ok {
			servers[name] = entry
		}
	}
	manifest := struct {
		MCPServers map[string]MCPServerEntry `json:"mcpServers"`
	}{MCPServers: servers}
	data, _ := json.Marshal(manifest)
	return string(data)
}

// MountConfig specifies a host-to-container mount.
type MountConfig struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}
