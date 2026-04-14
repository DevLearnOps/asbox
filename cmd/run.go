package cmd

import (
	"fmt"
	"maps"
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

		// Agent override via --agent flag
		agentOverride, _ := cmd.Flags().GetString("agent")
		if agentOverride != "" {
			if err := config.ValidateAgent(agentOverride); err != nil {
				return err
			}
			if err := config.ValidateAgentInstalled(agentOverride, cfg.InstalledAgents); err != nil {
				return err
			}
			cfg.DefaultAgent = agentOverride
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

		// Mount host agent config directory for OAuth token sync
		hostMountFlag, envKey, envVal, err := mount.AssembleHostAgentConfig(cfg.DefaultAgent, cfg.HostAgentConfig)
		if err != nil {
			return err
		}
		if hostMountFlag != "" {
			mountFlags = append(mountFlags, hostMountFlag)
			if envKey != "" {
				envVars[envKey] = envVal
			}
		}

		// Mount BMAD multi-repo directories and generate agent instructions
		bmadMounts, instructionContent, err := mount.AssembleBmadRepos(cfg)
		if err != nil {
			return err
		}
		if len(bmadMounts) > 0 {
			mountFlags = append(mountFlags, bmadMounts...)
			fmt.Fprintf(cmd.OutOrStdout(), "bmad_repos: mounting %d repositories\n", len(bmadMounts))
			for _, m := range bmadMounts {
				fmt.Fprintf(cmd.OutOrStdout(), "bmad_repos: %s\n", m)
			}
		}
		if instructionContent != "" {
			tmpFile, err := os.CreateTemp("", "asbox-instructions-*.md")
			if err != nil {
				return fmt.Errorf("bmad_repos: failed to create temp instruction file: %w", err)
			}
			defer os.Remove(tmpFile.Name())
			if _, err := tmpFile.WriteString(instructionContent); err != nil {
				tmpFile.Close()
				return fmt.Errorf("bmad_repos: failed to write instruction file: %w", err)
			}
			tmpFile.Close()

			instructionTarget, err := agentInstructionTarget(cfg.DefaultAgent)
			if err != nil {
				return err
			}
			mountFlags = append(mountFlags, tmpFile.Name()+":"+instructionTarget)
		}

		// Auto-isolate platform dependencies via named volumes
		if cfg.AutoIsolateDeps {
			scanResults, err := mount.ScanDeps(cfg)
			if err != nil {
				return err
			}

			mountCount := len(cfg.Mounts) + len(cfg.BmadRepos)
			fmt.Fprintf(cmd.OutOrStdout(), "auto_isolate_deps: scanned %d mount paths, found %d package.json files\n", mountCount, len(scanResults))

			for _, r := range scanResults {
				fmt.Fprintf(cmd.OutOrStdout(), "isolating: %s (volume: %s)\n", r.ContainerPath, r.VolumeName)
			}

			if len(scanResults) > 0 {
				volumeFlags, autoIsolatePaths := mount.AssembleIsolateDeps(scanResults)
				mountFlags = append(mountFlags, volumeFlags...)
				envVars["AUTO_ISOLATE_VOLUME_PATHS"] = autoIsolatePaths
			}
		}

		agentCmd, err := agentCommand(cfg.DefaultAgent)
		if err != nil {
			return err
		}
		envVars["AGENT_CMD"] = agentCmd

		// Build if needed (same hash logic as cmd/build.go)
		noCacheRun, _ := cmd.Flags().GetBool("no-cache")
		imageRef, _, err := ensureBuild(cfg, cmd, noCacheRun)
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
	maps.Copy(envVars, cfg.Env)

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

// agentCommand maps the configured agent name to the shell command the
// entrypoint should exec into.
func agentCommand(agent string) (string, error) {
	switch agent {
	case "claude":
		return "claude --dangerously-skip-permissions", nil
	case "gemini":
		return "gemini -y", nil
	case "codex":
		return "codex --dangerously-bypass-approvals-and-sandbox", nil
	default:
		return "", fmt.Errorf("unknown agent %q: supported agents are claude, gemini, codex", agent)
	}
}

func agentInstructionTarget(agent string) (string, error) {
	switch agent {
	case "claude":
		return "/home/sandbox/CLAUDE.md", nil
	case "gemini":
		return "/home/sandbox/GEMINI.md", nil
	case "codex":
		return "/home/sandbox/.codex/AGENTS.md", nil
	default:
		return "", fmt.Errorf("bmad_repos: unsupported agent %q for instruction file mount", agent)
	}
}

func init() {
	runCmd.Flags().Bool("no-cache", false, "Force a complete rebuild, bypassing content-hash check and Docker layer cache")
	runCmd.Flags().String("agent", "", "Override default agent for this session (e.g., claude, gemini, codex)")
	rootCmd.AddCommand(runCmd)
}
