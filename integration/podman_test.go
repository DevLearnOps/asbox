package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// podmanSocketPath is the expected socket path for the default sandbox user (UID 1000).
// Tests do not set HOST_UID, so align_uid_gid is a no-op and UID stays 1000.
const podmanSocketPath = "/run/user/1000/podman/podman.sock"

// startTestContainerWithEntrypoint starts a container using the real entrypoint
// so the Podman socket and environment variables are initialized.
// The container runs "sleep infinity" as its command after entrypoint completes.
func startTestContainerWithEntrypoint(ctx context.Context, t *testing.T, image string) testcontainers.Container {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image: image,
		Cmd:   []string{"sleep", "infinity"},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Privileged = true
		},
		WaitingFor: wait.ForExec([]string{"test", "-S", podmanSocketPath}).
			WithStartupTimeout(120 * time.Second).
			WithPollInterval(2 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("starting test container with entrypoint: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("warning: failed to terminate container: %v", err)
		}
	})

	return container
}

func TestPodmanDockerAlias(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	image := buildTestImage(t)
	container := startTestContainerWithEntrypoint(ctx, t, image)

	t.Run("docker_version_returns_podman", func(t *testing.T) {
		t.Parallel()
		output, exitCode := execAsUser(ctx, t, container, "sandbox", []string{"docker", "--version"})

		if exitCode != 0 {
			t.Errorf("expected exit code 0, got %d; output: %s", exitCode, output)
		}
		outputLower := strings.ToLower(output)
		if !strings.Contains(outputLower, "podman") {
			t.Errorf("expected output to contain 'podman', got %q", output)
		}
	})

	t.Run("storage_driver_is_vfs", func(t *testing.T) {
		t.Parallel()
		output, exitCode := execAsUser(ctx, t, container, "sandbox", []string{
			"docker", "info", "--format", "{{.Store.GraphDriverName}}",
		})

		if exitCode != 0 {
			t.Errorf("expected exit code 0, got %d; output: %s", exitCode, output)
		}
		if !strings.Contains(output, "vfs") {
			t.Errorf("expected storage driver 'vfs', got %q", output)
		}
	})

	t.Run("docker_compose_version", func(t *testing.T) {
		t.Parallel()
		output, exitCode := execAsUser(ctx, t, container, "sandbox", []string{"docker", "compose", "version"})

		if exitCode != 0 {
			t.Errorf("expected exit code 0, got %d; output: %s", exitCode, output)
		}
		// Docker Compose should return a version string
		if !strings.Contains(output, "version") && !strings.Contains(output, "v2") {
			t.Errorf("expected docker compose version output, got %q", output)
		}
	})

	t.Run("docker_host_set", func(t *testing.T) {
		t.Parallel()
		output, exitCode := execAsUser(ctx, t, container, "sandbox", []string{"printenv", "DOCKER_HOST"})

		if exitCode != 0 {
			t.Errorf("expected exit code 0, got %d; output: %s", exitCode, output)
		}
		if !strings.Contains(output, "podman/podman.sock") {
			t.Errorf("expected DOCKER_HOST to contain 'podman/podman.sock', got %q", output)
		}
	})

	t.Run("podman_socket_exists_and_owned_by_sandbox", func(t *testing.T) {
		t.Parallel()
		// Check socket exists
		output, exitCode := execAsUser(ctx, t, container, "sandbox", []string{
			"stat", "-c", "%F %U", podmanSocketPath,
		})

		if exitCode != 0 {
			t.Errorf("expected exit code 0, got %d; output: %s", exitCode, output)
		}
		if !strings.Contains(output, "socket") {
			t.Errorf("expected file type 'socket', got %q", output)
		}
		if !strings.Contains(output, "sandbox") {
			t.Errorf("expected socket owned by 'sandbox', got %q", output)
		}
	})

	t.Run("testcontainers_ryuk_disabled", func(t *testing.T) {
		t.Parallel()
		output, exitCode := execAsUser(ctx, t, container, "sandbox", []string{"printenv", "TESTCONTAINERS_RYUK_DISABLED"})

		if exitCode != 0 {
			t.Errorf("expected exit code 0, got %d; output: %s", exitCode, output)
		}
		if !strings.Contains(output, "true") {
			t.Errorf("expected TESTCONTAINERS_RYUK_DISABLED=true, got %q", output)
		}
	})

	t.Run("testcontainers_host_override", func(t *testing.T) {
		t.Parallel()
		output, exitCode := execAsUser(ctx, t, container, "sandbox", []string{"printenv", "TESTCONTAINERS_HOST_OVERRIDE"})

		if exitCode != 0 {
			t.Errorf("expected exit code 0, got %d; output: %s", exitCode, output)
		}
		if !strings.Contains(output, "localhost") {
			t.Errorf("expected TESTCONTAINERS_HOST_OVERRIDE=localhost, got %q", output)
		}
	})

	t.Run("testcontainers_docker_socket_override", func(t *testing.T) {
		t.Parallel()
		output, exitCode := execAsUser(ctx, t, container, "sandbox", []string{"printenv", "TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE"})

		if exitCode != 0 {
			t.Errorf("expected exit code 0, got %d; output: %s", exitCode, output)
		}
		if !strings.Contains(output, "podman.sock") {
			t.Errorf("expected TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE to contain 'podman.sock', got %q", output)
		}
	})
}

func TestPodmanRunContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	image := buildTestImage(t)
	container := startTestContainerWithEntrypoint(ctx, t, image)

	// No t.Parallel: single subtest with sequential dependency on startTestContainerWithEntrypoint above.
	t.Run("docker_run_alpine_echo", func(t *testing.T) {
		// Run a container inside the sandbox using the docker (podman) alias
		// This validates the full stack: podman-docker alias -> Podman socket -> rootless container execution
		output, exitCode := execAsUser(ctx, t, container, "sandbox", []string{
			"docker", "run", "--rm", "alpine:latest", "echo", "hello",
		})

		if exitCode != 0 {
			t.Errorf("expected exit code 0, got %d; output: %s", exitCode, output)
		}
		if !strings.Contains(output, "hello") {
			t.Errorf("expected output to contain 'hello', got %q", output)
		}
	})
}

func TestContainerNotPrivileged(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	image := buildTestImage(t)
	container := startTestContainer(ctx, t, image)

	// Get the Docker client and container ID for inspection
	dockerContainer, ok := container.(*testcontainers.DockerContainer)
	if !ok {
		t.Fatal("expected *testcontainers.DockerContainer, got different implementation")
	}
	containerID := dockerContainer.GetContainerID()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("creating Docker client: %v", err)
	}
	defer cli.Close()

	inspect, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		t.Fatalf("inspecting container: %v", err)
	}

	t.Run("not_privileged", func(t *testing.T) {
		t.Parallel()
		if inspect.HostConfig.Privileged {
			t.Error("expected container to NOT be privileged, but Privileged=true")
		}
	})

	t.Run("no_docker_socket_mount", func(t *testing.T) {
		t.Parallel()
		for _, mount := range inspect.Mounts {
			if strings.Contains(mount.Source, "docker.sock") {
				t.Errorf("expected no Docker socket mount, found: %s -> %s", mount.Source, mount.Destination)
			}
		}
	})
}
