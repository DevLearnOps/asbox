package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/mcastellin/asbox/internal/config"
	"github.com/mcastellin/asbox/internal/docker"
	"github.com/mcastellin/asbox/internal/mount"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the sandbox container",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Parse(configFile)
		if err != nil {
			return err
		}

		// Validate mounts and secrets before building (fail fast on config errors)
		mountFlags, err := mount.AssembleMounts(cfg)
		if err != nil {
			return err
		}

		envVars, err := buildEnvVars(cfg)
		if err != nil {
			return err
		}

		// Build if needed (same hash logic as cmd/build.go)
		imageRef, _, err := ensureBuild(cfg, cmd)
		if err != nil {
			return err
		}

		containerName := "asbox-" + cfg.ProjectName

		fmt.Fprintf(cmd.OutOrStdout(), "launching sandbox %s...\n", containerName)

		opts := docker.RunOptions{
			ImageRef:      imageRef,
			ContainerName: containerName,
			EnvVars:       envVars,
			Mounts:        mountFlags,
			Stdin:         os.Stdin,
			Stdout:        cmd.OutOrStdout(),
			Stderr:        cmd.ErrOrStderr(),
		}

		return docker.RunContainer(opts)
	},
}

// buildEnvVars assembles container environment variables with priority:
// 1. cfg.Env (lowest) 2. secrets from host env 3. HOST_UID/HOST_GID (highest).
// Returns an error if a declared secret is not set in the host environment.
func buildEnvVars(cfg *config.Config) (map[string]string, error) {
	envVars := make(map[string]string)

	// Custom env vars from config (lowest priority)
	for k, v := range cfg.Env {
		envVars[k] = v
	}

	// Validate and inject secrets (overwrite any cfg.Env collision)
	for _, secret := range cfg.Secrets {
		val, ok := os.LookupEnv(secret)
		if !ok {
			return nil, &config.SecretError{
				Msg: fmt.Sprintf("secret '%s' not set in host environment. Export it or remove from .asbox/config.yaml secrets list", secret),
			}
		}
		envVars[secret] = val
	}

	// HOST_UID/HOST_GID (highest priority, cannot be overridden)
	envVars["HOST_UID"] = strconv.Itoa(os.Getuid())
	envVars["HOST_GID"] = strconv.Itoa(os.Getgid())

	return envVars, nil
}

func init() {
	rootCmd.AddCommand(runCmd)
}
