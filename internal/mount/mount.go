package mount

import (
	"fmt"
	"os"

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
