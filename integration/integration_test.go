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
	"github.com/testcontainers/testcontainers-go/wait"
)

const testImageName = "asbox-integration-test"

// buildTestImage builds a minimal sandbox image for integration testing.
// It uses the real embedded Dockerfile template with a minimal config.
func buildTestImage(t *testing.T) string {
	t.Helper()

	cfg := &config.Config{
		Agent:       "claude-code",
		ProjectName: "integration-test",
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
	wrapped := []string{"su", "-s", "/bin/bash", "-c", strings.Join(quoted, " "), user}
	return execInContainer(ctx, t, container, wrapped)
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
