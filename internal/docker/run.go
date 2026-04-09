package docker

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
)

// RunOptions holds the parameters for running a Docker container.
type RunOptions struct {
	ImageRef      string            // e.g., "asbox-myapp:a1b2c3d4e5f6"
	ContainerName string            // e.g., "asbox-myapp"
	EnvVars       map[string]string // HOST_UID, HOST_GID, secrets, custom env
	Mounts        []string          // mount flags for later stories (e.g., "-v source:target")
	Stdin         io.Reader         // os.Stdin for interactive TTY
	Stdout        io.Writer         // os.Stdout
	Stderr        io.Writer         // os.Stderr
}

// runCmdArgs assembles the docker run command arguments.
func runCmdArgs(opts RunOptions) []string {
	args := []string{"run", "-it", "--rm"}

	if opts.ContainerName != "" {
		args = append(args, "--name", opts.ContainerName)
	}

	for key, val := range opts.EnvVars {
		args = append(args, "--env", key+"="+val)
	}

	for _, m := range opts.Mounts {
		args = append(args, "-v", m)
	}

	args = append(args, opts.ImageRef)
	return args
}

// RunContainer launches a Docker container with interactive TTY mode.
func RunContainer(opts RunOptions) error {
	args := runCmdArgs(opts)
	cmd := exec.Command("docker", args...)
	cmd.Stdin = opts.Stdin
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code := exitErr.ExitCode()
			// 130 = SIGINT (Ctrl+C), 143 = SIGTERM — expected interactive shutdown
			if code == 130 || code == 143 {
				return nil
			}
		}
		return &RunError{Msg: fmt.Sprintf("docker run failed: %s", err)}
	}
	return nil
}
