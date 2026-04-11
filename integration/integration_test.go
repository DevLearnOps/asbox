package integration

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	asboxEmbed "github.com/mcastellin/asbox/embed"
	"github.com/mcastellin/asbox/internal/config"
	"github.com/mcastellin/asbox/internal/docker"
	"github.com/mcastellin/asbox/internal/template"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"
)

const testImageName = "asbox-integration-test"

// buildTestImage builds a minimal sandbox image for integration testing.
// It uses the real embedded Dockerfile template with a minimal config.
func buildTestImage(t *testing.T) string {
	t.Helper()

	cfg := &config.Config{
		InstalledAgents: []string{"claude"},
		ProjectName:     "integration-test",
	}

	rendered, err := template.Render(cfg)
	if err != nil {
		t.Fatalf("rendering Dockerfile template: %v", err)
	}

	embeddedFiles := map[string][]byte{}
	entries, err := asboxEmbed.Assets.ReadDir(".")
	if err != nil {
		t.Fatalf("reading embedded assets: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := asboxEmbed.Assets.ReadFile(entry.Name())
		if err != nil {
			t.Fatalf("reading embedded file %s: %v", entry.Name(), err)
		}
		embeddedFiles[entry.Name()] = data
	}

	tag := fmt.Sprintf("%s:%d", testImageName, time.Now().UnixNano())

	var stdout, stderr bytes.Buffer
	err = docker.BuildImage(docker.BuildOptions{
		RenderedDockerfile: rendered,
		Tags:               []string{tag},
		EmbeddedFiles:      embeddedFiles,
		Stdout:             &stdout,
		Stderr:             &stderr,
	})
	if err != nil {
		t.Fatalf("building test image: %v\nstderr: %s", err, stderr.String())
	}

	t.Cleanup(func() {
		_ = exec.Command("docker", "rmi", "-f", tag).Run()
	})

	return tag
}

// startTestContainer starts a container from the given image and returns
// a testcontainers.Container for exec operations.
func startTestContainer(ctx context.Context, t *testing.T, image string) testcontainers.Container {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:      image,
		Entrypoint: []string{"tail", "-f", "/dev/null"},
		WaitingFor: wait.ForExec([]string{"true"}).WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("starting test container: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("warning: failed to terminate container: %v", err)
		}
	})

	return container
}

// execInContainer runs a command inside the container as root and returns
// the combined output and exit code.
func execInContainer(ctx context.Context, t *testing.T, container testcontainers.Container, cmd []string) (string, int) {
	t.Helper()

	exitCode, reader, err := container.Exec(ctx, cmd)
	if err != nil {
		t.Fatalf("exec in container failed: %v", err)
	}

	var buf bytes.Buffer
	if reader != nil {
		_, _ = buf.ReadFrom(reader)
	}

	return buf.String(), exitCode
}

// execAsUser runs a command inside the container as the specified user and returns
// the combined output and exit code.
func execAsUser(ctx context.Context, t *testing.T, container testcontainers.Container, user string, cmd []string) (string, int) {
	t.Helper()

	// Shell-quote each argument to preserve boundaries (e.g., sh -c "complex cmd")
	quoted := make([]string, len(cmd))
	for i, c := range cmd {
		quoted[i] = "'" + strings.ReplaceAll(c, "'", "'\\''") + "'"
	}
	// Source the profile to pick up dynamic env vars set by the entrypoint
	// (docker exec sessions don't inherit entrypoint shell exports).
	shellCmd := ". /etc/profile.d/sandbox-env.sh 2>/dev/null; " + strings.Join(quoted, " ")
	wrapped := []string{"su", "-s", "/bin/bash", "-c", shellCmd, user}
	return execInContainer(ctx, t, container, wrapped)
}

// buildTestImageWithConfig builds a sandbox image from a custom config.
// Use this when tests need specific config options (MCP, auto_isolate_deps, etc.).
func buildTestImageWithConfig(t *testing.T, cfg *config.Config) string {
	t.Helper()

	rendered, err := template.Render(cfg)
	if err != nil {
		t.Fatalf("rendering Dockerfile template: %v", err)
	}

	embeddedFiles := map[string][]byte{}
	entries, err := asboxEmbed.Assets.ReadDir(".")
	if err != nil {
		t.Fatalf("reading embedded assets: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := asboxEmbed.Assets.ReadFile(entry.Name())
		if err != nil {
			t.Fatalf("reading embedded file %s: %v", entry.Name(), err)
		}
		embeddedFiles[entry.Name()] = data
	}

	tag := fmt.Sprintf("%s:%d", testImageName, time.Now().UnixNano())

	var stdout, stderr bytes.Buffer
	err = docker.BuildImage(docker.BuildOptions{
		RenderedDockerfile: rendered,
		Tags:               []string{tag},
		EmbeddedFiles:      embeddedFiles,
		Stdout:             &stdout,
		Stderr:             &stderr,
	})
	if err != nil {
		t.Fatalf("building test image: %v\nstderr: %s", err, stderr.String())
	}

	t.Cleanup(func() {
		_ = exec.Command("docker", "rmi", "-f", tag).Run()
	})

	return tag
}

// fileExistsInContainer checks whether a file exists at the given path inside
// the container. Returns true/false without failing the test.
func fileExistsInContainer(ctx context.Context, t *testing.T, container testcontainers.Container, path string) bool {
	t.Helper()

	exitCode, _, err := container.Exec(ctx, []string{"test", "-f", path})
	if err != nil {
		t.Fatalf("checking file existence in container: %v", err)
	}
	return exitCode == 0
}

// fileContentInContainer reads and returns the content of a file inside
// the container. Fatals if the file does not exist.
func fileContentInContainer(ctx context.Context, t *testing.T, container testcontainers.Container, path string) string {
	t.Helper()

	exitCode, reader, err := container.Exec(ctx, []string{"cat", path}, tcexec.Multiplexed())
	if err != nil {
		t.Fatalf("reading file %s in container: %v", path, err)
	}
	if exitCode != 0 {
		t.Fatalf("file %s does not exist in container (exit code %d)", path, exitCode)
	}

	var buf bytes.Buffer
	if reader != nil {
		_, _ = buf.ReadFrom(reader)
	}
	return buf.String()
}

// startTestContainerWithMounts starts a container with the given bind mounts.
// Useful for mount verification tests.
func startTestContainerWithMounts(ctx context.Context, t *testing.T, image string, mounts []testcontainers.ContainerMount) testcontainers.Container {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:      image,
		Entrypoint: []string{"tail", "-f", "/dev/null"},
		Mounts:     testcontainers.ContainerMounts(mounts),
		WaitingFor: wait.ForExec([]string{"true"}).WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("starting test container with mounts: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("warning: failed to terminate container: %v", err)
		}
	})

	return container
}

// setupGitRepoWithRemote creates a temporary git repository with an initial
// commit and a remote configured. Returns the path to the repository.
// The directory is automatically cleaned up when the test finishes.
func setupGitRepoWithRemote(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	commands := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test User"},
		{"git", "commit", "--allow-empty", "-m", "initial commit"},
		{"git", "remote", "add", "origin", "https://example.com/test.git"},
	}

	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git setup command %v failed: %v\noutput: %s", args, err, out)
		}
	}

	return dir
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
