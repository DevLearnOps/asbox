package docker

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mcastellin/asbox/internal/config"
)

// BuildOptions holds the parameters for building a Docker image.
type BuildOptions struct {
	RenderedDockerfile string
	BuildArgs          []string
	Tags               []string
	EmbeddedFiles      map[string][]byte // filename -> content for build context
	Stdout             io.Writer
	Stderr             io.Writer
}

// ImageExists checks whether a Docker image with the given reference exists locally.
func ImageExists(imageRef string) (bool, error) {
	cmd := exec.Command("docker", "image", "inspect", imageRef)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1 from docker inspect means image not found
			if exitErr.ExitCode() == 1 {
				return false, nil
			}
		}
		// Any other error (daemon not running, permission denied, etc.) is a real failure
		return false, fmt.Errorf("failed to check image %s: %w", imageRef, err)
	}
	return true, nil
}

// buildCmdArgs assembles the docker build command arguments (exported for testing).
func buildCmdArgs(opts BuildOptions, dockerfilePath, contextDir string) []string {
	args := []string{"build", "-f", dockerfilePath}
	args = append(args, opts.BuildArgs...)
	for _, tag := range opts.Tags {
		args = append(args, "-t", tag)
	}
	args = append(args, contextDir)
	return args
}

// BuildImage builds a Docker image using the rendered Dockerfile and embedded assets as context.
func BuildImage(opts BuildOptions) error {
	// Write rendered Dockerfile to temp file
	tmpDockerfile, err := os.CreateTemp("", "asbox-dockerfile-*")
	if err != nil {
		return &BuildError{Msg: fmt.Sprintf("failed to create temp Dockerfile: %s", err)}
	}
	defer os.Remove(tmpDockerfile.Name())

	if _, err := tmpDockerfile.WriteString(opts.RenderedDockerfile); err != nil {
		tmpDockerfile.Close()
		return &BuildError{Msg: fmt.Sprintf("failed to write temp Dockerfile: %s", err)}
	}
	tmpDockerfile.Close()

	// Write embedded assets to a temp directory as build context
	contextDir, err := os.MkdirTemp("", "asbox-context-*")
	if err != nil {
		return &BuildError{Msg: fmt.Sprintf("failed to create temp context dir: %s", err)}
	}
	defer os.RemoveAll(contextDir)

	for name, content := range opts.EmbeddedFiles {
		if err := os.WriteFile(filepath.Join(contextDir, name), content, 0644); err != nil {
			return &BuildError{Msg: fmt.Sprintf("failed to write context file %s: %s", name, err)}
		}
	}

	args := buildCmdArgs(opts, tmpDockerfile.Name(), contextDir)
	cmd := exec.Command("docker", args...)
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	if err := cmd.Run(); err != nil {
		return &BuildError{Msg: fmt.Sprintf("docker build failed: %s", err)}
	}
	return nil
}

// BuildArgs returns a slice of --build-arg flags for each non-empty SDK version in the config.
func BuildArgs(cfg *config.Config) []string {
	var args []string

	if cfg.SDKs.NodeJS != "" {
		args = append(args, "--build-arg", "NODE_VERSION="+cfg.SDKs.NodeJS)
	}
	if cfg.SDKs.Go != "" {
		args = append(args, "--build-arg", "GO_VERSION="+cfg.SDKs.Go)
	}
	if cfg.SDKs.Python != "" {
		args = append(args, "--build-arg", "PYTHON_VERSION="+cfg.SDKs.Python)
	}

	return args
}
