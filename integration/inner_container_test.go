package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/docker/docker/client"
	"github.com/testcontainers/testcontainers-go"
)

func TestDockerBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	image := buildTestImage(t)
	ctr := startTestContainerWithEntrypoint(ctx, t, image)

	// Write a minimal Dockerfile inside the container
	_, exitCode := execInContainer(ctx, t, ctr, []string{"sh", "-c", `mkdir -p /tmp/buildtest && cat > /tmp/buildtest/Dockerfile << 'DEOF'
FROM alpine:latest
RUN echo built
CMD ["echo","hello"]
DEOF`})
	if exitCode != 0 {
		t.Fatal("failed to create test Dockerfile inside container")
	}

	t.Run("build_succeeds", func(t *testing.T) {
		output, exitCode := execAsUser(ctx, t, ctr, "sandbox", []string{
			"sh", "-c", "cd /tmp/buildtest && docker build -t testapp .",
		})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0, got %d; output: %s", exitCode, output)
		}
	})

	t.Run("image_appears_in_docker_images", func(t *testing.T) {
		output, exitCode := execAsUser(ctx, t, ctr, "sandbox", []string{
			"docker", "images", "--format", "{{.Repository}}",
		})
		if exitCode != 0 {
			t.Errorf("expected exit code 0, got %d; output: %s", exitCode, output)
		}
		if !strings.Contains(output, "testapp") {
			t.Errorf("expected 'testapp' in docker images output, got %q", output)
		}
	})
}

func TestDockerComposeMultiService(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	image := buildTestImage(t)
	ctr := startTestContainerWithEntrypoint(ctx, t, image)

	// Write docker-compose.yml with two services
	_, exitCode := execInContainer(ctx, t, ctr, []string{"sh", "-c", `mkdir -p /tmp/composetest && cat > /tmp/composetest/docker-compose.yml << 'CEOF'
services:
  web:
    image: alpine:latest
    command: tail -f /dev/null
  client:
    image: alpine:latest
    command: tail -f /dev/null
CEOF`})
	if exitCode != 0 {
		t.Fatal("failed to create docker-compose.yml inside container")
	}

	t.Run("compose_up_succeeds", func(t *testing.T) {
		output, exitCode := execAsUser(ctx, t, ctr, "sandbox", []string{
			"sh", "-c", "cd /tmp/composetest && docker compose up -d",
		})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0 for compose up, got %d; output: %s", exitCode, output)
		}
	})

	t.Run("both_services_running", func(t *testing.T) {
		output, exitCode := execAsUser(ctx, t, ctr, "sandbox", []string{
			"sh", "-c", "cd /tmp/composetest && docker compose ps",
		})
		if exitCode != 0 {
			t.Errorf("expected exit code 0, got %d; output: %s", exitCode, output)
		}
		if !strings.Contains(output, "web") {
			t.Errorf("expected 'web' service in compose ps output, got %q", output)
		}
		if !strings.Contains(output, "client") {
			t.Errorf("expected 'client' service in compose ps output, got %q", output)
		}
	})

	t.Run("dns_resolution_between_services", func(t *testing.T) {
		// Retry nslookup to allow DNS propagation time after compose up
		output, exitCode := execAsUser(ctx, t, ctr, "sandbox", []string{
			"sh", "-c",
			"cd /tmp/composetest && for i in 1 2 3 4 5; do docker compose exec -T client nslookup web 2>&1 && exit 0; sleep 2; done; exit 1",
		})
		if exitCode != 0 {
			t.Errorf("expected nslookup to resolve 'web' service, got exit code %d; output: %s", exitCode, output)
		}
	})
}

func TestInnerContainerPorts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	image := buildTestImage(t)
	ctr := startTestContainerWithEntrypoint(ctx, t, image)

	// Start an inner container with nc serving HTTP on port 3000
	_, exitCode := execAsUser(ctx, t, ctr, "sandbox", []string{
		"docker", "run", "-d", "--name", "porttest", "-p", "3000:3000", "alpine:latest",
		"sh", "-c", `while true; do printf 'HTTP/1.0 200 OK\r\n\r\nok' | nc -l -p 3000; done`,
	})
	if exitCode != 0 {
		t.Fatal("failed to start inner container with port mapping")
	}

	t.Run("reachable_from_sandbox", func(t *testing.T) {
		// Retry curl to allow server startup time
		output, exitCode := execAsUser(ctx, t, ctr, "sandbox", []string{
			"sh", "-c",
			"for i in 1 2 3 4 5; do curl -s --max-time 2 http://localhost:3000/ && exit 0; sleep 2; done; exit 1",
		})
		if exitCode != 0 {
			t.Errorf("expected curl to reach inner container, got exit code %d; output: %s", exitCode, output)
		}
		if !strings.Contains(output, "ok") {
			t.Errorf("expected response containing 'ok', got %q", output)
		}
	})

	t.Run("not_reachable_from_outside", func(t *testing.T) {
		// Inspect the outer container — it should have no published ports,
		// proving inner container ports are isolated on a private network bridge.
		dockerCtr, ok := ctr.(*testcontainers.DockerContainer)
		if !ok {
			t.Fatal("expected *testcontainers.DockerContainer")
		}
		containerID := dockerCtr.GetContainerID()

		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			t.Fatalf("creating Docker client: %v", err)
		}
		defer cli.Close()

		inspect, err := cli.ContainerInspect(ctx, containerID)
		if err != nil {
			t.Fatalf("inspecting container: %v", err)
		}

		for port, bindings := range inspect.NetworkSettings.Ports {
			if len(bindings) > 0 {
				t.Errorf("expected no published ports on outer container, but port %s has bindings: %v", port, bindings)
			}
		}
	})
}

func TestDockerComposePluginPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	image := buildTestImage(t)
	ctr := startTestContainer(ctx, t, image)

	t.Run("plugin_symlink_exists", func(t *testing.T) {
		t.Parallel()
		output, exitCode := execInContainer(ctx, t, ctr, []string{
			"test", "-f", "/usr/local/lib/docker/cli-plugins/docker-compose",
		})
		if exitCode != 0 {
			t.Errorf("expected docker-compose plugin at /usr/local/lib/docker/cli-plugins/docker-compose to exist; output: %s", output)
		}
	})
}
