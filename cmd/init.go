package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	asboxEmbed "github.com/mcastellin/asbox/embed"
	"github.com/mcastellin/asbox/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new asbox configuration",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil // init doesn't need Docker
	},
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	target := configFile

	// Check if file already exists
	if _, err := os.Stat(target); err == nil {
		return &config.ConfigError{Msg: fmt.Sprintf("config already exists at %s", target)}
	} else if !errors.Is(err, os.ErrNotExist) {
		return &config.ConfigError{Msg: fmt.Sprintf("failed to check config at %s: %s", target, err)}
	}

	// Read starter config from embedded assets
	data, err := asboxEmbed.Assets.ReadFile("config.yaml")
	if err != nil {
		return &config.ConfigError{Msg: fmt.Sprintf("failed to read embedded config: %s", err)}
	}

	// Create parent directories
	dir := filepath.Dir(target)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return &config.ConfigError{Msg: fmt.Sprintf("failed to create config at %s: %s", target, err)}
		}
	}

	// Write config file
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return &config.ConfigError{Msg: fmt.Sprintf("failed to create config at %s: %s", target, err)}
	}

	fmt.Fprintln(cmd.OutOrStdout(), "created", target)
	return nil
}
