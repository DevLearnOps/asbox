package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var validAgents = map[string]bool{
	"claude-code": true,
	"gemini-cli":  true,
}

var sanitizeRe = regexp.MustCompile(`[^a-z0-9-]+`)
var collapseHyphens = regexp.MustCompile(`-{2,}`)

// Parse reads, unmarshals, validates, and resolves a config file.
func Parse(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ConfigError{
				Msg: fmt.Sprintf("config file not found at %s. Run 'asbox init' to create one", configPath),
			}
		}
		return nil, &ConfigError{
			Msg: fmt.Sprintf("cannot read config file: %s", err),
		}
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, &ConfigError{
			Msg: fmt.Sprintf("invalid YAML: %s", err),
		}
	}

	// Validate required field: agent
	if cfg.Agent == "" {
		return nil, &ConfigError{
			Field: "agent",
			Msg:   "required field is empty. Set agent to 'claude-code' or 'gemini-cli'",
		}
	}
	if !validAgents[cfg.Agent] {
		return nil, &ConfigError{
			Field: "agent",
			Msg:   fmt.Sprintf("unsupported agent '%s'. Use 'claude-code' or 'gemini-cli'", cfg.Agent),
		}
	}

	// Validate MCP servers
	supported := make([]string, 0, len(MCPServerRegistry))
	for k := range MCPServerRegistry {
		supported = append(supported, k)
	}
	sort.Strings(supported)
	seen := map[string]bool{}
	for _, mcp := range cfg.MCP {
		if seen[mcp] {
			return nil, &ConfigError{
				Field: "mcp",
				Msg:   fmt.Sprintf("duplicate MCP server '%s'", mcp),
			}
		}
		seen[mcp] = true
		if _, ok := MCPServerRegistry[mcp]; !ok {
			return nil, &ConfigError{
				Field: "mcp",
				Msg:   fmt.Sprintf("unsupported MCP server '%s'. Supported: %s", mcp, strings.Join(supported, ", ")),
			}
		}
	}
	if cfg.HasMCP("playwright") && cfg.SDKs.NodeJS == "" {
		return nil, &ConfigError{
			Field: "mcp",
			Msg:   "mcp server 'playwright' requires sdks.nodejs to be configured",
		}
	}

	// Validate mounts
	for i, m := range cfg.Mounts {
		if m.Source == "" {
			return nil, &ConfigError{
				Field: fmt.Sprintf("mounts[%d].source", i),
				Msg:   "required field is empty. Set source path for each mount entry",
			}
		}
		if m.Target == "" {
			return nil, &ConfigError{
				Field: fmt.Sprintf("mounts[%d].target", i),
				Msg:   "required field is empty. Set target path for each mount entry",
			}
		}
		if !filepath.IsAbs(m.Target) {
			return nil, &ConfigError{
				Field: fmt.Sprintf("mounts[%d].target", i),
				Msg:   fmt.Sprintf("target must be an absolute container path, got %q", m.Target),
			}
		}
	}

	// Validate host_agent_config if set
	if cfg.HostAgentConfig != nil {
		if cfg.HostAgentConfig.Source == "" {
			return nil, &ConfigError{
				Field: "host_agent_config.source",
				Msg:   "required field is empty. Set source path for host_agent_config",
			}
		}
		if cfg.HostAgentConfig.Target == "" {
			return nil, &ConfigError{
				Field: "host_agent_config.target",
				Msg:   "required field is empty. Set target path for host_agent_config",
			}
		}
		if !filepath.IsAbs(cfg.HostAgentConfig.Target) {
			return nil, &ConfigError{
				Field: "host_agent_config.target",
				Msg:   fmt.Sprintf("target must be an absolute container path, got %q", cfg.HostAgentConfig.Target),
			}
		}
	}

	configDir, err := filepath.Abs(filepath.Dir(configPath))
	if err != nil {
		return nil, &ConfigError{
			Msg: fmt.Sprintf("cannot resolve config directory: %s", err),
		}
	}

	// Derive project_name if not set
	if cfg.ProjectName == "" {
		// Parent directory of the config file's directory
		// e.g. for .asbox/config.yaml, configDir is <project>/.asbox, parent is <project>
		parentDir := filepath.Dir(configDir)
		cfg.ProjectName = sanitizeProjectName(filepath.Base(parentDir))
		if cfg.ProjectName == "" {
			cfg.ProjectName = "asbox"
		}
	}

	// Resolve mount paths relative to config file directory
	for i := range cfg.Mounts {
		cfg.Mounts[i].Source = resolvePath(configDir, cfg.Mounts[i].Source)
	}

	// Resolve host_agent_config path
	if cfg.HostAgentConfig != nil {
		cfg.HostAgentConfig.Source = resolvePath(configDir, cfg.HostAgentConfig.Source)
	}

	// Resolve bmad_repos paths
	for i := range cfg.BmadRepos {
		cfg.BmadRepos[i] = resolvePath(configDir, cfg.BmadRepos[i])
	}

	return &cfg, nil
}

// resolvePath resolves a path relative to baseDir. Absolute paths pass through unchanged.
// Tilde prefixes are expanded to the user's home directory.
func resolvePath(baseDir, p string) string {
	if strings.HasPrefix(p, "~/") || p == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[1:])
		}
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(baseDir, p)
}

// sanitizeProjectName lowercases and replaces non-alphanumeric chars with hyphens.
func sanitizeProjectName(name string) string {
	name = strings.ToLower(name)
	name = sanitizeRe.ReplaceAllString(name, "-")
	name = collapseHyphens.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	return name
}
