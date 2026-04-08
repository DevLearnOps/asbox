package cmd

import (
	"github.com/spf13/cobra"
	"github.com/mcastellin/asbox/internal/config"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the sandbox container",
	RunE: func(cmd *cobra.Command, args []string) error {
		return &config.ConfigError{Msg: "not implemented"}
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
