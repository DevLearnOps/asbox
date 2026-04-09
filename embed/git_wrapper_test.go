package embed

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupGitWrapper extracts git-wrapper.sh to a temp directory and returns its path.
// It also creates a fake /usr/bin/git stand-in script that simply echoes "real-git"
// followed by its arguments, so passthrough behavior can be verified without a real git.
func setupGitWrapper(t *testing.T) (wrapperPath, fakeGitPath string) {
	t.Helper()

	dir := t.TempDir()

	// Write the embedded git-wrapper.sh
	data, err := Assets.ReadFile("git-wrapper.sh")
	if err != nil {
		t.Fatalf("reading embedded git-wrapper.sh: %v", err)
	}
	wrapperPath = filepath.Join(dir, "git-wrapper.sh")
	if err := os.WriteFile(wrapperPath, data, 0o755); err != nil {
		t.Fatalf("writing git-wrapper.sh: %v", err)
	}

	// Create a fake git binary that the wrapper will exec into.
	// The wrapper calls "exec /usr/bin/git ...", so we patch the script
	// to use our fake git instead.
	fakeGitPath = filepath.Join(dir, "real-git")
	fakeGit := []byte("#!/usr/bin/env bash\necho \"real-git $*\"\n")
	if err := os.WriteFile(fakeGitPath, fakeGit, 0o755); err != nil {
		t.Fatalf("writing fake git: %v", err)
	}

	// Patch the wrapper to use the fake git instead of /usr/bin/git
	patched := strings.ReplaceAll(string(data), "/usr/bin/git", fakeGitPath)
	if err := os.WriteFile(wrapperPath, []byte(patched), 0o755); err != nil {
		t.Fatalf("patching git-wrapper.sh: %v", err)
	}

	return wrapperPath, fakeGitPath
}

// runWrapper executes git-wrapper.sh with the given arguments and returns
// stdout, stderr, and the exit code.
func runWrapper(t *testing.T, wrapperPath string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	cmd := exec.Command("bash", append([]string{wrapperPath}, args...)...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("running wrapper: %v", err)
		}
	}

	return outBuf.String(), errBuf.String(), exitCode
}

func TestGitWrapper_PushBlocked(t *testing.T) {
	wrapper, _ := setupGitWrapper(t)

	stdout, stderr, exitCode := runWrapper(t, wrapper, "push")

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr, "fatal: Authentication failed") {
		t.Errorf("expected stderr to contain 'fatal: Authentication failed', got %q", stderr)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
}

func TestGitWrapper_PushOriginMainBlocked(t *testing.T) {
	wrapper, _ := setupGitWrapper(t)

	_, stderr, exitCode := runWrapper(t, wrapper, "push", "origin", "main")

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(stderr, "fatal: Authentication failed") {
		t.Errorf("expected stderr to contain 'fatal: Authentication failed', got %q", stderr)
	}
}

func TestGitWrapper_PushVariantsBlocked(t *testing.T) {
	wrapper, _ := setupGitWrapper(t)

	variants := []struct {
		name string
		args []string
	}{
		{"push-force", []string{"push", "--force"}},
		{"push-force-origin-main", []string{"push", "--force", "origin", "main"}},
		{"push-u-origin-main", []string{"push", "-u", "origin", "main"}},
		{"push-all", []string{"push", "--all"}},
		{"push-tags", []string{"push", "--tags"}},
		{"push-origin", []string{"push", "origin"}},
	}

	for _, tc := range variants {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, exitCode := runWrapper(t, wrapper, tc.args...)
			if exitCode != 1 {
				t.Errorf("expected exit code 1, got %d", exitCode)
			}
			if !strings.Contains(stderr, "fatal: Authentication failed") {
				t.Errorf("expected stderr to contain 'fatal: Authentication failed', got %q", stderr)
			}
		})
	}
}

func TestGitWrapper_PassthroughCommands(t *testing.T) {
	wrapper, _ := setupGitWrapper(t)

	commands := []struct {
		name string
		args []string
	}{
		{"add", []string{"add", "."}},
		{"commit", []string{"commit", "-m", "test"}},
		{"log", []string{"log"}},
		{"diff", []string{"diff"}},
		{"branch", []string{"branch"}},
		{"checkout", []string{"checkout", "main"}},
		{"merge", []string{"merge", "feature"}},
		{"stash-push", []string{"stash", "push"}},
		{"pull", []string{"pull"}},
		{"fetch", []string{"fetch"}},
		{"commit-amend", []string{"commit", "--amend"}},
	}

	for _, tc := range commands {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, exitCode := runWrapper(t, wrapper, tc.args...)

			if exitCode != 0 {
				t.Errorf("expected exit code 0, got %d (stderr: %q)", exitCode, stderr)
			}
			if !strings.Contains(stdout, "real-git") {
				t.Errorf("expected stdout to contain 'real-git' (passthrough), got %q", stdout)
			}
		})
	}
}

func TestGitWrapper_StashPushNotBlocked(t *testing.T) {
	// Explicitly verify that "git stash push" is NOT blocked.
	// "stash" is the first non-flag arg, so loop breaks before seeing "push".
	wrapper, _ := setupGitWrapper(t)

	stdout, _, exitCode := runWrapper(t, wrapper, "stash", "push")

	if exitCode != 0 {
		t.Errorf("expected exit code 0 for 'stash push', got %d", exitCode)
	}
	if !strings.Contains(stdout, "real-git") {
		t.Errorf("expected passthrough for 'stash push', got stdout: %q", stdout)
	}
}
