package gitfetch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchAll_happyPath(t *testing.T) {
	repo1, bare1 := initRepoWithOrigin(t)
	repo2, bare2 := initRepoWithOrigin(t)

	wantRemote1 := pushCommitToRemote(t, bare1, "remote-1.txt", "one\n")
	wantRemote2 := pushCommitToRemote(t, bare2, "remote-2.txt", "two\n")

	summary := FetchAll(context.Background(), []string{repo1, repo2}, FetchOptions{})

	if summary.Total != 2 {
		t.Fatalf("summary.Total = %d, want 2", summary.Total)
	}
	if summary.Succeeded != 2 {
		t.Fatalf("summary.Succeeded = %d, want 2", summary.Succeeded)
	}
	if summary.Failed != 0 || summary.SkippedNoOrigin != 0 || summary.SkippedNotGit != 0 {
		t.Fatalf("unexpected counters: %+v", summary)
	}
	if len(summary.Results) != 2 {
		t.Fatalf("len(summary.Results) = %d, want 2", len(summary.Results))
	}

	for i, result := range summary.Results {
		if result.Status != StatusSucceeded {
			t.Fatalf("result[%d].Status = %q, want %q", i, result.Status, StatusSucceeded)
		}
		if result.Err != nil {
			t.Fatalf("result[%d].Err = %v, want nil", i, result.Err)
		}
	}

	if got := gitOutput(t, repo1, "rev-parse", "refs/remotes/origin/main"); got != wantRemote1 {
		t.Fatalf("repo1 origin/main = %q, want %q", got, wantRemote1)
	}
	if got := gitOutput(t, repo2, "rev-parse", "refs/remotes/origin/main"); got != wantRemote2 {
		t.Fatalf("repo2 origin/main = %q, want %q", got, wantRemote2)
	}
}

func TestFetchAll_skippedNoOrigin(t *testing.T) {
	repo := initRepoWithoutOrigin(t)

	summary := FetchAll(context.Background(), []string{repo}, FetchOptions{})

	if summary.Total != 1 {
		t.Fatalf("summary.Total = %d, want 1", summary.Total)
	}
	if summary.SkippedNoOrigin != 1 {
		t.Fatalf("summary.SkippedNoOrigin = %d, want 1", summary.SkippedNoOrigin)
	}
	if len(summary.Results) != 1 {
		t.Fatalf("len(summary.Results) = %d, want 1", len(summary.Results))
	}
	if summary.Results[0].Status != StatusSkippedNoOrigin {
		t.Fatalf("summary.Results[0].Status = %q, want %q", summary.Results[0].Status, StatusSkippedNoOrigin)
	}
}

func TestFetchAll_skippedNotGit(t *testing.T) {
	notGitDir := t.TempDir()
	repoWithOrigin, _ := initRepoWithOrigin(t)

	summary := FetchAll(context.Background(), []string{notGitDir, repoWithOrigin}, FetchOptions{})

	if summary.Total != 2 {
		t.Fatalf("summary.Total = %d, want 2", summary.Total)
	}
	if summary.SkippedNotGit != 1 {
		t.Fatalf("summary.SkippedNotGit = %d, want 1", summary.SkippedNotGit)
	}
	if summary.Succeeded != 1 {
		t.Fatalf("summary.Succeeded = %d, want 1", summary.Succeeded)
	}
	if summary.Results[0].Status != StatusSkippedNotGit {
		t.Fatalf("summary.Results[0].Status = %q, want %q", summary.Results[0].Status, StatusSkippedNotGit)
	}
	if summary.Results[1].Status != StatusSucceeded {
		t.Fatalf("summary.Results[1].Status = %q, want %q", summary.Results[1].Status, StatusSucceeded)
	}
}

func TestFetchAll_deduplication(t *testing.T) {
	repo, _ := initRepoWithOrigin(t)
	linkPath := filepath.Join(t.TempDir(), "repo-link")
	if err := os.Symlink(repo, linkPath); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	summary := FetchAll(context.Background(), []string{repo, repo + string(os.PathSeparator), linkPath}, FetchOptions{})

	if summary.Total != 1 {
		t.Fatalf("summary.Total = %d, want 1", summary.Total)
	}
	if len(summary.Results) != 1 {
		t.Fatalf("len(summary.Results) = %d, want 1", len(summary.Results))
	}
	wantPath, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", repo, err)
	}
	if summary.Results[0].Path != wantPath {
		t.Fatalf("summary.Results[0].Path = %q, want %q", summary.Results[0].Path, wantPath)
	}
}

func TestFetchAll_gitWorktreeMarkerCountsAsRepo(t *testing.T) {
	repo, _ := initRepoWithOrigin(t)
	worktreeDir := filepath.Join(t.TempDir(), "linked-worktree")
	gitRun(t, repo, "worktree", "add", "--detach", worktreeDir)

	info, err := os.Stat(filepath.Join(worktreeDir, ".git"))
	if err != nil {
		t.Fatalf("stat .git file: %v", err)
	}
	if info.IsDir() {
		t.Fatal("expected linked worktree .git to be a file")
	}

	summary := FetchAll(context.Background(), []string{worktreeDir}, FetchOptions{})

	if summary.Total != 1 {
		t.Fatalf("summary.Total = %d, want 1", summary.Total)
	}
	if summary.Results[0].Status != StatusSucceeded {
		t.Fatalf("summary.Results[0].Status = %q, want %q", summary.Results[0].Status, StatusSucceeded)
	}
}

func TestFetchAll_timeout(t *testing.T) {
	repo, _ := initRepoWithOrigin(t)
	swapFetchRepoFn(t, func(ctx context.Context, path string) (string, error) {
		<-ctx.Done()
		return "fetch hung\n", ctx.Err()
	})

	summary := FetchAll(context.Background(), []string{repo}, FetchOptions{Timeout: time.Millisecond})

	if summary.Failed != 1 {
		t.Fatalf("summary.Failed = %d, want 1", summary.Failed)
	}
	result := summary.Results[0]
	if result.Status != StatusFailed {
		t.Fatalf("result.Status = %q, want %q", result.Status, StatusFailed)
	}
	if result.Stderr != "fetch hung\n" {
		t.Fatalf("result.Stderr = %q, want %q", result.Stderr, "fetch hung\n")
	}
	if result.Err == nil {
		t.Fatal("result.Err = nil, want timeout error")
	}
	if !errors.Is(result.Err, ErrTimeout) {
		t.Fatalf("errors.Is(result.Err, ErrTimeout) = false, want true (err = %v)", result.Err)
	}
	want := "timed out after 1ms (override with ASBOX_FETCH_TIMEOUT)"
	if !strings.Contains(result.Err.Error(), want) {
		t.Fatalf("result.Err = %q, want substring %q", result.Err.Error(), want)
	}
}

func TestFetchAll_concurrencyLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrency timing test in short mode")
	}

	_, bare := initRepoWithOrigin(t)
	paths := make([]string, 8)
	for i := range paths {
		paths[i] = cloneRepo(t, bare, fmt.Sprintf("clone-%d", i))
	}

	var current atomic.Int32
	var maxSeen atomic.Int32
	swapFetchRepoFn(t, func(ctx context.Context, path string) (string, error) {
		now := current.Add(1)
		defer current.Add(-1)
		for {
			seen := maxSeen.Load()
			if now <= seen {
				break
			}
			if maxSeen.CompareAndSwap(seen, now) {
				break
			}
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(80 * time.Millisecond):
			return "", nil
		}
	})

	start := time.Now()
	summary := FetchAll(context.Background(), paths, FetchOptions{Concurrency: 4, Timeout: time.Second})
	elapsed := time.Since(start)

	if summary.Succeeded != len(paths) {
		t.Fatalf("summary.Succeeded = %d, want %d", summary.Succeeded, len(paths))
	}
	if got := maxSeen.Load(); got > 4 {
		t.Fatalf("max concurrent fetches = %d, want <= 4", got)
	}
	if elapsed >= 500*time.Millisecond {
		t.Fatalf("elapsed = %s, want under 500ms", elapsed)
	}
}

func TestFetchAll_refsOnlyNoWorkingTreeMutation(t *testing.T) {
	repo, bare := initRepoWithOrigin(t)

	dirtyPath := filepath.Join(repo, "dirty.txt")
	writeFile(t, dirtyPath, "dirty worktree\n")
	gitRun(t, repo, "add", "dirty.txt")

	beforeStatus := gitOutput(t, repo, "status", "--porcelain")
	beforeHead := gitOutput(t, repo, "rev-parse", "HEAD")
	beforeLocalBranch := gitOutput(t, repo, "rev-parse", "refs/heads/main")
	wantRemote := pushCommitToRemote(t, bare, "remote-update.txt", "updated\n")

	summary := FetchAll(context.Background(), []string{repo}, FetchOptions{})

	if summary.Succeeded != 1 {
		t.Fatalf("summary.Succeeded = %d, want 1", summary.Succeeded)
	}
	if got := string(mustReadFile(t, dirtyPath)); got != "dirty worktree\n" {
		t.Fatalf("dirty.txt contents = %q, want %q", got, "dirty worktree\n")
	}
	if got := gitOutput(t, repo, "status", "--porcelain"); got != beforeStatus {
		t.Fatalf("git status changed:\nbefore: %q\nafter:  %q", beforeStatus, got)
	}
	if got := gitOutput(t, repo, "rev-parse", "HEAD"); got != beforeHead {
		t.Fatalf("HEAD changed: before %q after %q", beforeHead, got)
	}
	if got := gitOutput(t, repo, "rev-parse", "refs/heads/main"); got != beforeLocalBranch {
		t.Fatalf("local branch changed: before %q after %q", beforeLocalBranch, got)
	}
	if got := gitOutput(t, repo, "rev-parse", "refs/remotes/origin/main"); got != wantRemote {
		t.Fatalf("origin/main = %q, want %q", got, wantRemote)
	}
}

func TestFetchAll_failureNonFatal(t *testing.T) {
	goodRepo, _ := initRepoWithOrigin(t)
	badRepo, _ := initRepoWithOrigin(t)
	gitRun(t, badRepo, "remote", "set-url", "origin", "file:///definitely/missing/remote.git")

	summary := FetchAll(context.Background(), []string{goodRepo, badRepo}, FetchOptions{})

	if summary.Total != 2 {
		t.Fatalf("summary.Total = %d, want 2", summary.Total)
	}
	if summary.Succeeded != 1 {
		t.Fatalf("summary.Succeeded = %d, want 1", summary.Succeeded)
	}
	if summary.Failed != 1 {
		t.Fatalf("summary.Failed = %d, want 1", summary.Failed)
	}
	if summary.Results[0].Status != StatusSucceeded {
		t.Fatalf("summary.Results[0].Status = %q, want %q", summary.Results[0].Status, StatusSucceeded)
	}
	if summary.Results[1].Status != StatusFailed {
		t.Fatalf("summary.Results[1].Status = %q, want %q", summary.Results[1].Status, StatusFailed)
	}
	if summary.Results[1].Err == nil {
		t.Fatal("summary.Results[1].Err = nil, want non-nil")
	}
	if summary.Results[1].Stderr == "" {
		t.Fatal("summary.Results[1].Stderr = empty, want git stderr")
	}
}

func swapFetchRepoFn(t *testing.T, fn func(context.Context, string) (string, error)) {
	t.Helper()
	old := fetchRepoFn
	fetchRepoFn = fn
	t.Cleanup(func() {
		fetchRepoFn = old
	})
}

func initRepoWithOrigin(t *testing.T) (string, string) {
	t.Helper()

	base := t.TempDir()
	seed := filepath.Join(base, "seed")
	bare := filepath.Join(base, "origin.git")
	clone := filepath.Join(base, "clone")

	if err := os.MkdirAll(seed, 0o755); err != nil {
		t.Fatalf("creating seed repo dir: %v", err)
	}

	gitRun(t, seed, "init", "-b", "main")
	configureGitUser(t, seed)
	writeFile(t, filepath.Join(seed, "README.md"), "seed\n")
	gitRun(t, seed, "add", "README.md")
	gitRun(t, seed, "commit", "-m", "initial commit")

	gitRun(t, base, "init", "--bare", "-b", "main", bare)
	gitRun(t, seed, "remote", "add", "origin", bare)
	gitRun(t, seed, "push", "-u", "origin", "main")

	clone = cloneRepo(t, bare, "clone")
	return clone, bare
}

func initRepoWithoutOrigin(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	gitRun(t, repo, "init", "-b", "main")
	configureGitUser(t, repo)
	writeFile(t, filepath.Join(repo, "README.md"), "local\n")
	gitRun(t, repo, "add", "README.md")
	gitRun(t, repo, "commit", "-m", "local commit")
	return repo
}

func cloneRepo(t *testing.T, barePath, name string) string {
	t.Helper()

	parent := t.TempDir()
	clonePath := filepath.Join(parent, name)
	gitRun(t, parent, "clone", barePath, clonePath)
	configureGitUser(t, clonePath)
	return clonePath
}

func pushCommitToRemote(t *testing.T, barePath, filename, content string) string {
	t.Helper()

	writer := cloneRepo(t, barePath, "writer")
	writeFile(t, filepath.Join(writer, filename), content)
	gitRun(t, writer, "add", filename)
	gitRun(t, writer, "commit", "-m", "update remote")
	gitRun(t, writer, "push", "origin", "main")
	return gitOutput(t, writer, "rev-parse", "HEAD")
}

func configureGitUser(t *testing.T, dir string) {
	t.Helper()
	gitRun(t, dir, "config", "user.name", "Test User")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	gitRun(t, dir, "config", "tag.gpgsign", "false")
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, err := gitOutputWithError(t, dir, args...); err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitOutputWithError(t, dir, args...)
	if err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
	return strings.TrimSpace(out)
}

func gitOutputWithError(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return content
}

func TestFetchAll_timeoutHookReturnsContextDeadline(t *testing.T) {
	repo, _ := initRepoWithOrigin(t)
	swapFetchRepoFn(t, func(ctx context.Context, path string) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})

	summary := FetchAll(context.Background(), []string{repo}, FetchOptions{Timeout: time.Millisecond})
	if !errors.Is(summary.Results[0].Err, context.DeadlineExceeded) && !strings.Contains(summary.Results[0].Err.Error(), "timed out after 1ms") {
		t.Fatalf("expected timeout-flavored error, got %v", summary.Results[0].Err)
	}
}
