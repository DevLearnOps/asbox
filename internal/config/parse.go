package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var validAgents = map[string]bool{
	"claude": true,
	"gemini": true,
	"codex":  true,
}

var sanitizeRe = regexp.MustCompile(`[^a-z0-9-]+`)
var collapseHyphens = regexp.MustCompile(`-{2,}`)
var sdkVersionRe = regexp.MustCompile(`^[0-9a-zA-Z.\-+]+$`)
var packageNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.\-+=:]*$`)
var envKeyRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

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

	// Validate installed_agents
	if len(cfg.InstalledAgents) == 0 {
		return nil, &ConfigError{
			Field: "installed_agents",
			Msg:   "required field is empty. Set installed_agents to a list of agents (e.g., [claude, gemini])",
		}
	}
	seenAgents := map[string]bool{}
	for _, agent := range cfg.InstalledAgents {
		if err := ValidateAgent(agent); err != nil {
			return nil, err
		}
		if seenAgents[agent] {
			return nil, &ConfigError{
				Field: "installed_agents",
				Msg:   fmt.Sprintf("duplicate agent '%s'", agent),
			}
		}
		seenAgents[agent] = true
	}

	// Validate or default default_agent
	if cfg.DefaultAgent != "" {
		if err := ValidateAgent(cfg.DefaultAgent); err != nil {
			return nil, err
		}
		if err := ValidateAgentInstalled(cfg.DefaultAgent, cfg.InstalledAgents); err != nil {
			return nil, err
		}
	} else {
		cfg.DefaultAgent = cfg.InstalledAgents[0]
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
	if slices.Contains(cfg.InstalledAgents, "gemini") && cfg.SDKs.NodeJS == "" {
		return nil, &ConfigError{
			Field: "installed_agents",
			Msg:   "agent 'gemini' requires sdks.nodejs to be configured",
		}
	}
	if slices.Contains(cfg.InstalledAgents, "codex") && cfg.SDKs.NodeJS == "" {
		return nil, &ConfigError{
			Field: "installed_agents",
			Msg:   "agent 'codex' requires sdks.nodejs to be configured",
		}
	}

	for _, sdk := range []struct {
		field   string
		version string
	}{
		{"sdks.nodejs", cfg.SDKs.NodeJS},
		{"sdks.go", cfg.SDKs.Go},
		{"sdks.python", cfg.SDKs.Python},
	} {
		if sdk.version == "" {
			continue
		}
		if err := validateSDKVersion(sdk.field, sdk.version); err != nil {
			return nil, err
		}
	}

	for i, pkg := range cfg.Packages {
		if err := validatePackageName(i, pkg); err != nil {
			return nil, err
		}
	}

	envKeys := make([]string, 0, len(cfg.Env))
	for key := range cfg.Env {
		envKeys = append(envKeys, key)
	}
	sort.Strings(envKeys)
	for _, key := range envKeys {
		if err := validateEnvKey(key); err != nil {
			return nil, err
		}
		if err := validateEnvValue(key, cfg.Env[key]); err != nil {
			return nil, err
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
		cfg.ProjectName = filepath.Base(parentDir)
	}
	cfg.ProjectName = sanitizeProjectName(cfg.ProjectName)
	if cfg.ProjectName == "" {
		cfg.ProjectName = "asbox"
	}

	// Resolve mount paths relative to config file directory
	for i := range cfg.Mounts {
		cfg.Mounts[i].Source = resolvePath(configDir, cfg.Mounts[i].Source)
	}

	// Resolve bmad_repos paths
	for i := range cfg.BmadRepos {
		cfg.BmadRepos[i] = resolvePath(configDir, cfg.BmadRepos[i])
	}
	if cfg.AgentInstructions != "" {
		cfg.AgentInstructions = resolvePath(configDir, cfg.AgentInstructions)
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

// ValidateAgent checks that an agent name is a supported short name.
func ValidateAgent(agent string) error {
	if !validAgents[agent] {
		return &ConfigError{Field: "agent", Msg: fmt.Sprintf("unsupported agent '%s'. Use 'claude', 'gemini', or 'codex'", agent)}
	}
	return nil
}

// ValidateAgentInstalled checks that an agent is in the installed agents list.
func ValidateAgentInstalled(agent string, installed []string) error {
	if !slices.Contains(installed, agent) {
		return &ConfigError{
			Field: "agent",
			Msg:   fmt.Sprintf("agent '%s' is not installed in the image. Installed agents: %s. Add it to installed_agents in config or choose a different agent", agent, strings.Join(installed, ", ")),
		}
	}
	return nil
}

// sanitizeProjectName lowercases and replaces non-alphanumeric chars with hyphens.
func sanitizeProjectName(name string) string {
	name = strings.ToLower(name)
	name = sanitizeRe.ReplaceAllString(name, "-")
	name = collapseHyphens.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	return name
}

func validateSDKVersion(field, version string) error {
	if sdkVersionRe.MatchString(version) {
		return nil
	}
	return &ConfigError{
		Field: field,
		Msg:   fmt.Sprintf("contains invalid characters %q. Allowed: letters, digits, dots, hyphens, plus signs", version),
	}
}

func validatePackageName(index int, pkg string) error {
	field := fmt.Sprintf("packages[%d]", index)
	if pkg == "" {
		return &ConfigError{
			Field: field,
			Msg:   "package name cannot be empty. Allowed: alphanumeric, hyphens, dots, plus signs, equals signs, colons (apt format)",
		}
	}
	if packageNameRe.MatchString(pkg) {
		return nil
	}
	return &ConfigError{
		Field: field,
		Msg:   fmt.Sprintf("contains invalid characters %q. Allowed: alphanumeric, hyphens, dots, plus signs, equals signs, colons (apt format)", pkg),
	}
}

func validateEnvKey(key string) error {
	if key == "" {
		return &ConfigError{
			Field: "env.",
			Msg:   "empty environment variable key is not allowed",
		}
	}
	if envKeyRe.MatchString(key) {
		return nil
	}
	return &ConfigError{
		Field: "env." + key,
		Msg:   fmt.Sprintf("invalid environment variable key %q. Keys must match shell variable format: start with letter or underscore, followed by letters, digits, or underscores", key),
	}
}

func validateEnvValue(key, value string) error {
	if !strings.ContainsAny(value, "\n\r\x00") {
		return nil
	}
	return &ConfigError{
		Field: "env." + key,
		Msg:   "value contains newline or null byte characters which could inject Dockerfile directives. Remove newlines and null bytes from the value",
	}
}
