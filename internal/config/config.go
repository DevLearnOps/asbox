package config

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

// MountConfig specifies a host-to-container mount.
type MountConfig struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}
