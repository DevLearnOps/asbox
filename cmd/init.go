package cmd

import (
	"github.com/spf13/cobra"
	"github.com/mcastellin/asbox/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new asbox configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		return &config.ConfigError{Msg: "not implemented"}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
