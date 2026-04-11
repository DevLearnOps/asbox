package mount

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mcastellin/asbox/internal/config"
)

// AssembleMounts validates mount source paths exist and returns "source:target" strings
// for passing as Docker -v flags.
func AssembleMounts(cfg *config.Config) ([]string, error) {
	if len(cfg.Mounts) == 0 {
		return nil, nil
	}

	mounts := make([]string, 0, len(cfg.Mounts))
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

	return mounts, nil
}

// AssembleHostAgentConfig resolves the host agent config directory mount for the
// given agent. Returns the mount flag ("source:target"), env var key, and env var
// value. All three are empty when the mount should be skipped (disabled, missing
// directory, or unknown agent).
func AssembleHostAgentConfig(agent string, enabled *bool) (string, string, string, error) {
	if enabled != nil && !*enabled {
		return "", "", "", nil
	}
	mapping, ok := config.AgentConfigRegistry[agent]
	if !ok {
		return "", "", "", nil
	}
	source := mapping.Source
	if strings.HasPrefix(source, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", "", nil
		}
		source = filepath.Join(home, source[2:])
	}
	info, err := os.Stat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", "", nil // AC9: silently skip if missing
		}
		return "", "", "", fmt.Errorf("host agent config: cannot access '%s': %w", source, err)
	}
	if !info.IsDir() {
		return "", "", "", nil
	}
	return source + ":" + mapping.Target, mapping.EnvVar, mapping.EnvVal, nil
}
