package gitfetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/sync/errgroup"
)

// ErrTimeout is wrapped by FetchResult.Err when a per-repo fetch exceeds its
// timeout. Callers can distinguish timeouts via errors.Is.
var ErrTimeout = errors.New("fetch timed out")

// nonInteractiveGitEnv disables interactive credential prompts so a fetch
// against a repo with expired HTTPS creds fails fast instead of hanging
// until the per-repo timeout fires.
var nonInteractiveGitEnv = append(
	os.Environ(),
	"GIT_TERMINAL_PROMPT=0",
	"GIT_ASKPASS=/bin/true",
)

type FetchStatus string

const (
	StatusSucceeded       FetchStatus = "succeeded"
	StatusFailed          FetchStatus = "failed"
	StatusSkippedNotGit   FetchStatus = "skipped-notgit"
	StatusSkippedNoOrigin FetchStatus = "skipped-noorigin"
)

const (
	DefaultTimeout     = 60 * time.Second
	DefaultConcurrency = 4
)

type FetchOptions struct {
	Timeout     time.Duration
	Concurrency int
}

type FetchResult struct {
	Path   string
	Status FetchStatus
	Err    error
	// Stderr captures the full fetch stderr. Per AC#10, callers flush this atomically
	// in a single write after FetchAll returns.
	Stderr string
}

type FetchSummary struct {
	Results         []FetchResult
	Total           int
	Succeeded       int
	Failed          int
	SkippedNotGit   int
	SkippedNoOrigin int
}

var fetchRepoFn = defaultFetchRepo

// DedupPaths returns the unique set of paths that FetchAll will process,
// including those that will be classified as skipped (not a git repo,
// unreadable parent) so callers can size their pre-fetch anchor line to match
// the eventual summary.Total.
func DedupPaths(paths []string) []string {
	preamble, fetchable := preparePaths(paths)
	out := make([]string, 0, len(preamble)+len(fetchable))
	for _, p := range preamble {
		out = append(out, p.Path)
	}
	return append(out, fetchable...)
}

func FetchAll(ctx context.Context, paths []string, opts FetchOptions) FetchSummary {
	if ctx == nil {
		ctx = context.Background()
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	preamble, fetchable := preparePaths(paths)
	results := make([]FetchResult, len(fetchable))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)

	for i, path := range fetchable {
		i := i
		path := path
		g.Go(func() error {
			results[i] = fetchOne(gctx, path, timeout)
			return nil
		})
	}

	_ = g.Wait()

	allResults := append(preamble, results...)
	summary := FetchSummary{
		Results: allResults,
		Total:   len(allResults),
	}

	for _, result := range allResults {
		switch result.Status {
		case StatusSucceeded:
			summary.Succeeded++
		case StatusFailed:
			summary.Failed++
		case StatusSkippedNotGit:
			summary.SkippedNotGit++
		case StatusSkippedNoOrigin:
			summary.SkippedNoOrigin++
		}
	}

	return summary
}

func preparePaths(paths []string) ([]FetchResult, []string) {
	preamble := make([]FetchResult, 0)
	fetchable := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))

	for _, path := range paths {
		if path == "" {
			continue
		}

		canonicalPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				preamble = append(preamble, FetchResult{
					Path:   path,
					Status: StatusSkippedNotGit,
				})
				continue
			}

			preamble = append(preamble, FetchResult{
				Path:   path,
				Status: StatusFailed,
				Err:    err,
			})
			continue
		}

		if _, ok := seen[canonicalPath]; ok {
			continue
		}
		seen[canonicalPath] = struct{}{}
		fetchable = append(fetchable, canonicalPath)
	}

	return preamble, fetchable
}

func fetchOne(ctx context.Context, path string, timeout time.Duration) FetchResult {
	result := FetchResult{Path: path}

	gitPath := filepath.Join(path, ".git")
	if _, err := os.Stat(gitPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			result.Status = StatusSkippedNotGit
			return result
		}
		result.Status = StatusFailed
		result.Err = err
		return result
	}

	if !hasOriginRemote(ctx, path) {
		result.Status = StatusSkippedNoOrigin
		return result
	}

	perRepoCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stderr, err := fetchRepoFn(perRepoCtx, path)
	result.Stderr = stderr

	switch {
	case err == nil:
		result.Status = StatusSucceeded
	case errors.Is(perRepoCtx.Err(), context.DeadlineExceeded), errors.Is(err, context.DeadlineExceeded):
		result.Status = StatusFailed
		result.Err = fmt.Errorf("%w after %s (override with ASBOX_FETCH_TIMEOUT)", ErrTimeout, timeout)
	default:
		result.Status = StatusFailed
		result.Err = err
	}

	return result
}

func hasOriginRemote(ctx context.Context, path string) bool {
	remoteCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	cmd := exec.CommandContext(remoteCtx, "git", "-C", path, "remote", "get-url", "origin")
	cmd.Env = nonInteractiveGitEnv
	return cmd.Run() == nil
}

func defaultFetchRepo(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "fetch", "origin")
	cmd.Dir = path
	cmd.Env = nonInteractiveGitEnv

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stderr.String(), err
}
