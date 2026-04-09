package cmd

import (
	"fmt"
	"os"

	asboxEmbed "github.com/mcastellin/asbox/embed"
	"github.com/mcastellin/asbox/internal/config"
	"github.com/mcastellin/asbox/internal/docker"
	"github.com/mcastellin/asbox/internal/hash"
	"github.com/mcastellin/asbox/internal/template"
	"github.com/spf13/cobra"
)

// ensureBuild computes the content hash and builds the image if it does not
// already exist. It returns the image reference and whether a build was performed.
func ensureBuild(cfg *config.Config, cmd *cobra.Command) (string, bool, error) {
	rawConfig, err := os.ReadFile(configFile)
	if err != nil {
		return "", false, fmt.Errorf("failed to read config file: %w", err)
	}

	renderedDockerfile, err := template.Render(cfg)
	if err != nil {
		return "", false, err
	}

	scriptNames := []string{"entrypoint.sh", "git-wrapper.sh", "healthcheck-poller.sh"}
	scripts := make([]string, len(scriptNames))
	for i, name := range scriptNames {
		data, err := asboxEmbed.Assets.ReadFile(name)
		if err != nil {
			return "", false, fmt.Errorf("failed to read embedded script %s: %w", name, err)
		}
		scripts[i] = string(data)
	}

	contentHash := hash.Compute(renderedDockerfile, scripts[0], scripts[1], scripts[2], string(rawConfig))
	imageRef := fmt.Sprintf("asbox-%s:%s", cfg.ProjectName, contentHash)
	latestRef := fmt.Sprintf("asbox-%s:latest", cfg.ProjectName)

	exists, err := docker.ImageExists(imageRef)
	if err != nil {
		return "", false, err
	}
	if exists {
		return imageRef, false, nil
	}

	// Read all embedded files for build context
	embeddedFiles := make(map[string][]byte)
	allFiles := []string{"entrypoint.sh", "git-wrapper.sh", "healthcheck-poller.sh", "agent-instructions.md.tmpl", "config.yaml"}
	for _, name := range allFiles {
		data, err := asboxEmbed.Assets.ReadFile(name)
		if err != nil {
			return "", false, fmt.Errorf("failed to read embedded file %s: %w", name, err)
		}
		embeddedFiles[name] = data
	}

	opts := docker.BuildOptions{
		RenderedDockerfile: renderedDockerfile,
		BuildArgs:          docker.BuildArgs(cfg),
		Tags:               []string{imageRef, latestRef},
		EmbeddedFiles:      embeddedFiles,
		Stdout:             cmd.OutOrStdout(),
		Stderr:             cmd.ErrOrStderr(),
	}

	if err := docker.BuildImage(opts); err != nil {
		return "", false, err
	}

	return imageRef, true, nil
}
