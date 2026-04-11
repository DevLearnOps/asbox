package integration

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMount_hostFilesAccessibleInContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("read_host_file_from_container", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		tempDir := t.TempDir()
		testContent := "hello-from-host"
		if err := os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte(testContent), 0644); err != nil {
			t.Fatalf("writing test file: %v", err)
		}

		image := buildTestImage(t)
		mounts := []testcontainers.ContainerMount{
			{
				Source: testcontainers.GenericBindMountSource{HostPath: tempDir},
				Target: "/workspace",
			},
		}
		container := startTestContainerWithMounts(ctx, t, image, mounts)

		content := fileContentInContainer(ctx, t, container, "/workspace/test.txt")
		if strings.TrimSpace(content) != testContent {
			t.Errorf("expected %q, got %q", testContent, strings.TrimSpace(content))
		}
	})
}

func TestMount_writesFromInsidePersistOnHost(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("write_inside_container_visible_on_host", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		tempDir := t.TempDir()
		image := buildTestImage(t)
		mounts := []testcontainers.ContainerMount{
			{
				Source: testcontainers.GenericBindMountSource{HostPath: tempDir},
				Target: "/workspace",
			},
		}
		container := startTestContainerWithMounts(ctx, t, image, mounts)

		_, exitCode := execInContainer(ctx, t, container, []string{"sh", "-c", "echo written-from-container > /workspace/output.txt"})
		if exitCode != 0 {
			t.Fatalf("expected exit code 0 for write command, got %d", exitCode)
		}

		data, err := os.ReadFile(filepath.Join(tempDir, "output.txt"))
		if err != nil {
			t.Fatalf("reading output file from host: %v", err)
		}
		if strings.TrimSpace(string(data)) != "written-from-container" {
			t.Errorf("expected %q, got %q", "written-from-container", strings.TrimSpace(string(data)))
		}
	})
}

func TestSecrets_availableAsEnvVarsInContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("env_vars_injected_into_container", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		image := buildTestImage(t)

		req := testcontainers.ContainerRequest{
			Image:      image,
			Entrypoint: []string{"tail", "-f", "/dev/null"},
			Env: map[string]string{
				"MY_SECRET":      "secret-value-123",
				"ANOTHER_SECRET": "another-value",
			},
			WaitingFor: wait.ForExec([]string{"true"}).WithStartupTimeout(60 * time.Second),
		}
		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
		if err != nil {
			t.Fatalf("starting container with env vars: %v", err)
		}
		t.Cleanup(func() {
			if err := container.Terminate(ctx); err != nil {
				t.Logf("warning: failed to terminate container: %v", err)
			}
		})

		output1, exitCode1 := execInContainer(ctx, t, container, []string{"printenv", "MY_SECRET"})
		if exitCode1 != 0 {
			t.Errorf("expected exit code 0 for printenv MY_SECRET, got %d", exitCode1)
		}
		if !strings.Contains(output1, "secret-value-123") {
			t.Errorf("expected MY_SECRET to contain %q, got %q", "secret-value-123", output1)
		}

		output2, exitCode2 := execInContainer(ctx, t, container, []string{"printenv", "ANOTHER_SECRET"})
		if exitCode2 != 0 {
			t.Errorf("expected exit code 0 for printenv ANOTHER_SECRET, got %d", exitCode2)
		}
		if !strings.Contains(output2, "another-value") {
			t.Errorf("expected ANOTHER_SECRET to contain %q, got %q", "another-value", output2)
		}
	})
}

func TestSecrets_missingSecretExitsCode4(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("missing_secret_returns_exit_code_4", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		binaryPath := filepath.Join(tmpDir, "asbox")

		// Build the asbox binary from project root
		buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
		buildCmd.Dir = ".." // project root relative to integration/
		out, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("building asbox binary: %v\noutput: %s", err, out)
		}

		// Create a minimal config that declares a secret
		configContent := "installed_agents: [claude]\nproject_name: test-secret-validation\nsecrets:\n  - ASBOX_TEST_NONEXISTENT_SECRET\n"
		configPath := filepath.Join(tmpDir, "config.yaml")
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("writing config file: %v", err)
		}

		// Run the binary with a controlled environment that excludes the test secret
		runCmd := exec.Command(binaryPath, "run", "-f", configPath)
		env := []string{}
		for _, e := range os.Environ() {
			if !strings.HasPrefix(e, "ASBOX_TEST_NONEXISTENT_SECRET=") {
				env = append(env, e)
			}
		}
		runCmd.Env = env

		err = runCmd.Run()
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("expected ExitError, got %v", err)
		}
		if exitErr.ExitCode() != 4 {
			t.Errorf("expected exit code 4, got %d", exitErr.ExitCode())
		}
	})
}
