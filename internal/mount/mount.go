package mount

import (
	"fmt"
	"os"

	"github.com/mcastellin/asbox/internal/config"
)

// AssembleMounts validates mount source paths exist and returns "source:target" strings
// for passing as Docker -v flags.
func AssembleMounts(cfg *config.Config) ([]string, error) {
	if len(cfg.Mounts) == 0 && cfg.HostAgentConfig == nil {
		return nil, nil
	}

	capacity := len(cfg.Mounts)
	if cfg.HostAgentConfig != nil {
		capacity++
	}

	mounts := make([]string, 0, capacity)
	for _, m := range cfg.Mounts {
		if _, err := os.Stat(m.Source); err != nil {
			if os.IsNotExist(err) {
				return nil, &config.ConfigError{
					Msg: fmt.Sprintf("mount source '%s' not found (resolved to %s). Check mounts in .asbox/config.yaml", m.Source, m.Source),
				}
			}
			return nil, &config.ConfigError{
				Msg: fmt.Sprintf("mount source '%s': %s", m.Source, err),
			}
		}
		mounts = append(mounts, m.Source+":"+m.Target)
	}

	if cfg.HostAgentConfig != nil {
		info, err := os.Stat(cfg.HostAgentConfig.Source)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, &config.ConfigError{
					Msg: fmt.Sprintf("host_agent_config source '%s' not found. Check host_agent_config in .asbox/config.yaml", cfg.HostAgentConfig.Source),
				}
			}
			return nil, &config.ConfigError{
				Msg: fmt.Sprintf("host_agent_config source '%s': %s", cfg.HostAgentConfig.Source, err),
			}
		}
		if !info.IsDir() {
			return nil, &config.ConfigError{
				Msg: fmt.Sprintf("host_agent_config source '%s' is not a directory. Check host_agent_config in .asbox/config.yaml", cfg.HostAgentConfig.Source),
			}
		}
		mounts = append(mounts, cfg.HostAgentConfig.Source+":"+cfg.HostAgentConfig.Target)
	}

	return mounts, nil
}
