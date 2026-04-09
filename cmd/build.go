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

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the sandbox container image",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Parse(configFile)
		if err != nil {
			return err
		}

		// Read raw config bytes for hashing
		rawConfig, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}

		// Render Dockerfile template
		renderedDockerfile, err := template.Render(cfg)
		if err != nil {
			return err
		}

		// Read embedded scripts for hashing
		scriptNames := []string{"entrypoint.sh", "git-wrapper.sh", "healthcheck-poller.sh"}
		scripts := make([]string, len(scriptNames))
		for i, name := range scriptNames {
			data, err := asboxEmbed.Assets.ReadFile(name)
			if err != nil {
				return fmt.Errorf("failed to read embedded script %s: %w", name, err)
			}
			scripts[i] = string(data)
		}

		// Compute content hash
		contentHash := hash.Compute(renderedDockerfile, scripts[0], scripts[1], scripts[2], string(rawConfig))

		imageRef := fmt.Sprintf("asbox-%s:%s", cfg.ProjectName, contentHash)
		latestRef := fmt.Sprintf("asbox-%s:latest", cfg.ProjectName)

		// Check if image already exists
		exists, err := docker.ImageExists(imageRef)
		if err != nil {
			return err
		}
		if exists {
			fmt.Fprintf(cmd.OutOrStdout(), "image %s is up to date, skipping build\n", imageRef)
			return nil
		}

		// Read all embedded files for build context
		embeddedFiles := make(map[string][]byte)
		allFiles := []string{"entrypoint.sh", "git-wrapper.sh", "healthcheck-poller.sh", "agent-instructions.md.tmpl", "config.yaml"}
		for _, name := range allFiles {
			data, err := asboxEmbed.Assets.ReadFile(name)
			if err != nil {
				return fmt.Errorf("failed to read embedded file %s: %w", name, err)
			}
			embeddedFiles[name] = data
		}

		// Build the image
		opts := docker.BuildOptions{
			RenderedDockerfile: renderedDockerfile,
			BuildArgs:         docker.BuildArgs(cfg),
			Tags:              []string{imageRef, latestRef},
			EmbeddedFiles:     embeddedFiles,
			Stdout:            cmd.OutOrStdout(),
			Stderr:            cmd.ErrOrStderr(),
		}

		if err := docker.BuildImage(opts); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "image built: %s\n", imageRef)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
