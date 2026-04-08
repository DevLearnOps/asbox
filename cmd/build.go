package cmd

import (
	"github.com/spf13/cobra"
	"github.com/mcastellin/asbox/internal/config"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the sandbox container image",
	RunE: func(cmd *cobra.Command, args []string) error {
		return &config.ConfigError{Msg: "not implemented"}
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
