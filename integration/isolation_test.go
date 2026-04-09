package integration

import (
	"context"
	"strings"
	"testing"
)

func TestIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	image := buildTestImage(t)
	container := startTestContainer(ctx, t, image)

	t.Run("git_push_blocked", func(t *testing.T) {
		t.Parallel()
		output, exitCode := execAsUser(ctx, t, container, "sandbox", []string{"git", "push"})

		if exitCode != 1 {
			t.Errorf("expected exit code 1, got %d", exitCode)
		}
		if !strings.Contains(output, "fatal: Authentication failed") {
			t.Errorf("expected output to contain 'fatal: Authentication failed', got %q", output)
		}
	})

	t.Run("git_operations_pass_through", func(t *testing.T) {
		t.Parallel()
		// Initialize a test repo in a unique temp dir to avoid races with parallel subtests
		output, exitCode := execAsUser(ctx, t, container, "sandbox", []string{
			"sh", "-c",
			"dir=$(mktemp -d /tmp/testrepo-XXXXXX) && cd \"$dir\" && git init && " +
				"git config user.email 'test@test.com' && " +
				"git config user.name 'Test' && " +
				"git add . && " +
				"git commit --allow-empty -m 'test commit' && " +
				"git log --oneline",
		})

		if exitCode != 0 {
			t.Errorf("expected exit code 0, got %d; output: %s", exitCode, output)
		}
		if !strings.Contains(output, "test commit") {
			t.Errorf("expected git log to contain 'test commit', got %q", output)
		}
	})

	t.Run("no_ssh_directory", func(t *testing.T) {
		t.Parallel()
		output, exitCode := execAsUser(ctx, t, container, "sandbox", []string{"ls", "/home/sandbox/.ssh"})

		if exitCode == 0 {
			t.Errorf("expected non-zero exit code for missing .ssh directory, got 0; output: %s", output)
		}
		if !strings.Contains(output, "No such file or directory") {
			t.Errorf("expected 'No such file or directory', got %q", output)
		}
	})

	t.Run("no_aws_directory", func(t *testing.T) {
		t.Parallel()
		output, exitCode := execAsUser(ctx, t, container, "sandbox", []string{"ls", "/home/sandbox/.aws"})

		if exitCode == 0 {
			t.Errorf("expected non-zero exit code for missing .aws directory, got 0; output: %s", output)
		}
		if !strings.Contains(output, "No such file or directory") {
			t.Errorf("expected 'No such file or directory', got %q", output)
		}
	})

	t.Run("git_wrapper_owned_by_root", func(t *testing.T) {
		t.Parallel()
		output, exitCode := execInContainer(ctx, t, container, []string{"stat", "-c", "%u %g", "/usr/local/bin/git"})

		if exitCode != 0 {
			t.Errorf("expected exit code 0, got %d; output: %s", exitCode, output)
		}
		if !strings.Contains(output, "0 0") {
			t.Errorf("expected git wrapper owned by root (0 0), got %q", output)
		}
	})

	t.Run("sandbox_user_exists", func(t *testing.T) {
		t.Parallel()
		output, exitCode := execInContainer(ctx, t, container, []string{"sh", "-c", "grep sandbox /etc/passwd"})

		if exitCode != 0 {
			t.Errorf("expected exit code 0, got %d; output: %s", exitCode, output)
		}
		if !strings.Contains(output, "sandbox") {
			t.Errorf("expected passwd entry for sandbox user, got %q", output)
		}
	})

	t.Run("git_wrapper_not_writable_by_sandbox", func(t *testing.T) {
		t.Parallel()
		// Try to overwrite git wrapper as sandbox user — should fail
		output, exitCode := execAsUser(ctx, t, container, "sandbox", []string{
			"sh", "-c", "echo test > /usr/local/bin/git",
		})

		if exitCode == 0 {
			t.Errorf("expected non-zero exit code when sandbox user tries to modify git wrapper; output: %s", output)
		}
	})
}
