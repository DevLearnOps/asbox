package cmd

import (
	"fmt"

	"github.com/mcastellin/asbox/internal/config"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the sandbox container",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := config.Parse(configFile)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "config loaded: %s\n", configFile)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
