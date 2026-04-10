package cmd

import (
	"fmt"

	"github.com/mcastellin/asbox/internal/config"
	"github.com/spf13/cobra"
)

var noCache bool

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the sandbox container image",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Parse(configFile)
		if err != nil {
			return err
		}

		imageRef, built, err := ensureBuild(cfg, cmd, noCache)
		if err != nil {
			return err
		}

		if built {
			fmt.Fprintf(cmd.OutOrStdout(), "image built: %s\n", imageRef)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "image %s is up to date, skipping build\n", imageRef)
		}
		return nil
	},
}

func init() {
	buildCmd.Flags().BoolVar(&noCache, "no-cache", false, "Force a complete rebuild, bypassing content-hash check and Docker layer cache")
	rootCmd.AddCommand(buildCmd)
}
