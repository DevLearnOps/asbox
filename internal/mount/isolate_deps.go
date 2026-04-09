package mount

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mcastellin/asbox/internal/config"
)

// volumeNameRe matches characters NOT allowed in Docker volume names.
// Docker allows [a-zA-Z0-9][a-zA-Z0-9_.-] — we strip everything else.
var volumeNameRe = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)

// ScanResult represents a discovered dependency directory to isolate.
type ScanResult struct {
	VolumeName    string // e.g., "asbox-myapp-packages-api-node_modules"
	ContainerPath string // e.g., "/workspace/packages/api/node_modules"
}

// ScanDeps walks each mount's source path for package.json files and returns
// volume isolation targets. Returns nil, nil if AutoIsolateDeps is false.
func ScanDeps(cfg *config.Config) ([]ScanResult, error) {
	if !cfg.AutoIsolateDeps {
		return nil, nil
	}

	var results []ScanResult
	for _, m := range cfg.Mounts {
		err := filepath.WalkDir(m.Source, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable directories
			}
			// Skip node_modules subtrees entirely
			if d.IsDir() && d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			// Only care about package.json files
			if d.IsDir() || d.Name() != "package.json" {
				return nil
			}

			dir := filepath.Dir(path)
			rel, _ := filepath.Rel(m.Source, dir)

			volumeName := buildVolumeName(cfg.ProjectName, rel)
			containerPath := buildContainerPath(m.Target, rel)

			results = append(results, ScanResult{
				VolumeName:    volumeName,
				ContainerPath: containerPath,
			})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("auto_isolate_deps: failed to scan %s: %w", m.Source, err)
		}
	}
	return results, nil
}

// buildVolumeName constructs a Docker named volume name from project name and relative path.
// Sanitizes to Docker-valid characters: [a-zA-Z0-9_.-].
func buildVolumeName(projectName, relPath string) string {
	name := "asbox-" + projectName
	if relPath != "" && relPath != "." {
		dashed := strings.ReplaceAll(relPath, string(filepath.Separator), "-")
		name += "-" + dashed
	}
	name += "-node_modules"
	return volumeNameRe.ReplaceAllString(name, "")
}

// buildContainerPath constructs the container-side mount target for a node_modules volume.
func buildContainerPath(mountTarget, relPath string) string {
	if relPath == "" || relPath == "." {
		return mountTarget + "/node_modules"
	}
	// Normalize to forward slashes for container paths
	relPath = strings.ReplaceAll(relPath, string(filepath.Separator), "/")
	return mountTarget + "/" + relPath + "/node_modules"
}

// AssembleIsolateDeps converts scan results into Docker volume flags and
// the AUTO_ISOLATE_VOLUME_PATHS env var value for entrypoint chown.
func AssembleIsolateDeps(results []ScanResult) (volumeFlags []string, autoIsolatePaths string) {
	if len(results) == 0 {
		return nil, ""
	}

	flags := make([]string, len(results))
	paths := make([]string, len(results))
	for i, r := range results {
		flags[i] = r.VolumeName + ":" + r.ContainerPath
		paths[i] = r.ContainerPath
	}
	return flags, strings.Join(paths, ",")
}
