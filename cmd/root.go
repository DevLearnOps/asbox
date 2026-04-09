package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/mcastellin/asbox/internal/config"
	"github.com/mcastellin/asbox/internal/docker"
	"github.com/mcastellin/asbox/internal/template"
)

var version = "dev"

var configFile string

// usageError wraps flag-parse and argument-validation errors from Cobra.
type usageError struct {
	err error
}

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

var rootCmd = &cobra.Command{
	Use:           "asbox",
	Short:         "AI Sandbox — isolated dev environments for AI agents",
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		_, err := exec.LookPath("docker")
		if err != nil {
			return &docker.DependencyError{Msg: "docker not found. Install Docker Engine 20.10+ or Docker Desktop"}
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "file", "f", ".asbox/config.yaml", "path to config file")
	rootCmd.Version = version
	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return &usageError{err: err}
	})
}

// exitCode maps a command error to the appropriate exit code.
func exitCode(err error) int {
	var ue *usageError
	var de *docker.DependencyError
	var se *config.SecretError
	var ce *config.ConfigError
	var te *template.TemplateError
	var be *docker.BuildError
	var re *docker.RunError

	switch {
	case errors.As(err, &ue):
		return 2
	case errors.As(err, &de):
		return 3
	case errors.As(err, &se):
		return 4
	case errors.As(err, &ce), errors.As(err, &te), errors.As(err, &be), errors.As(err, &re):
		return 1
	default:
		return 1
	}
}

// Execute runs the root command and maps errors to exit codes.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(exitCode(err))
	}
}
