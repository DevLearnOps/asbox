package docker

import (
	"github.com/mcastellin/asbox/internal/config"
)

// BuildArgs returns a slice of --build-arg flags for each non-empty SDK version in the config.
func BuildArgs(cfg *config.Config) []string {
	var args []string

	if cfg.SDKs.NodeJS != "" {
		args = append(args, "--build-arg", "NODE_VERSION="+cfg.SDKs.NodeJS)
	}
	if cfg.SDKs.Go != "" {
		args = append(args, "--build-arg", "GO_VERSION="+cfg.SDKs.Go)
	}
	if cfg.SDKs.Python != "" {
		args = append(args, "--build-arg", "PYTHON_VERSION="+cfg.SDKs.Python)
	}

	return args
}
