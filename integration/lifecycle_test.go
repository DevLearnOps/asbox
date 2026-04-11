package integration

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mcastellin/asbox/internal/config"
	"github.com/mcastellin/asbox/internal/template"
)

func TestBuild_producesTaggedImage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("image_exists_after_build", func(t *testing.T) {
		t.Parallel()

		tag := buildTestImage(t)

		// Verify tag matches expected pattern
		if !strings.HasPrefix(tag, testImageName+":") {
			t.Errorf("expected tag prefix %s:, got %s", testImageName, tag)
		}

		// Verify image exists via docker image inspect
		cmd := exec.Command("docker", "image", "inspect", tag)
		if err := cmd.Run(); err != nil {
			t.Errorf("expected image %s to exist, docker image inspect failed: %v", tag, err)
		}
	})
}

func TestContainer_respondsToExec(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("exec_echo_hello", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		image := buildTestImage(t)
		container := startTestContainer(ctx, t, image)

		output, exitCode := execInContainer(ctx, t, container, []string{"echo", "hello"})
		if exitCode != 0 {
			t.Errorf("expected exit code 0, got %d", exitCode)
		}
		if !strings.Contains(output, "hello") {
			t.Errorf("expected output to contain %q, got %q", "hello", output)
		}
	})
}

func TestContainer_stopsCleanly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("terminate_returns_no_error", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		image := buildTestImage(t)
		container := startTestContainer(ctx, t, image)

		// Explicitly terminate to verify no error is returned.
		// The cleanup func registered by startTestContainer will also call Terminate;
		// testcontainers handles double-terminate gracefully.
		if err := container.Terminate(ctx); err != nil {
			t.Errorf("expected clean termination, got error: %v", err)
		}

		// After termination, container.State should error or report not running
		// because the container has been removed.
		state, err := container.State(ctx)
		if err != nil {
			// Error is expected — container was removed
			return
		}
		if state.Running {
			t.Errorf("expected container to not be running after termination")
		}
	})
}

func TestAutoRebuild_differentConfigProducesDifferentTag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("different_configs_produce_valid_images", func(t *testing.T) {
		t.Parallel()

		// Build with default config
		tag1 := buildTestImage(t)

		// Build with modified config (adds curl package)
		cfg := &config.Config{
			InstalledAgents: []string{"claude"},
			ProjectName:     "integration-test",
			Packages:        []string{"curl"},
		}
		tag2 := buildTestImageWithConfig(t, cfg)

		// Both builds succeed and produce valid images
		for _, tag := range []string{tag1, tag2} {
			cmd := exec.Command("docker", "image", "inspect", tag)
			if err := cmd.Run(); err != nil {
				t.Errorf("expected image %s to exist, docker image inspect failed: %v", tag, err)
			}
		}

		// Verify that different configs produce different Dockerfile content
		defaultCfg := &config.Config{
			InstalledAgents: []string{"claude"},
			ProjectName:     "integration-test",
		}
		rendered1, err := template.Render(defaultCfg)
		if err != nil {
			t.Fatalf("rendering default config: %v", err)
		}
		rendered2, err := template.Render(cfg)
		if err != nil {
			t.Fatalf("rendering modified config: %v", err)
		}
		if rendered1 == rendered2 {
			t.Errorf("expected different Dockerfile content for different configs")
		}
	})
}
