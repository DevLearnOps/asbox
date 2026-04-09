package config

import "encoding/json"

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

// Config represents the top-level asbox configuration.
type Config struct {
	Agent           string            `yaml:"agent"`
	ProjectName     string            `yaml:"project_name"`
	SDKs            SDKConfig         `yaml:"sdks"`
	Packages        []string          `yaml:"packages"`
	MCP             []string          `yaml:"mcp"`
	Mounts          []MountConfig     `yaml:"mounts"`
	Secrets         []string          `yaml:"secrets"`
	Env             map[string]string `yaml:"env"`
	AutoIsolateDeps bool              `yaml:"auto_isolate_deps"`
	HostAgentConfig *MountConfig      `yaml:"host_agent_config"`
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
	for _, m := range c.MCP {
		if m == name {
			return true
		}
	}
	return false
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
