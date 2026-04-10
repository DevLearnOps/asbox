package mount

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	asboxEmbed "github.com/mcastellin/asbox/embed"
	"github.com/mcastellin/asbox/internal/config"
)

const bmadRepoMountBase = "/workspace/repos"

// BmadRepoInfo holds metadata for a single BMAD repository mount.
type BmadRepoInfo struct {
	Name          string // basename of the repo directory
	ContainerPath string // e.g., /workspace/repos/frontend
}

// InstructionData is the template data for rendering agent-instructions.md.tmpl.
type InstructionData struct {
	BmadRepos []BmadRepoInfo
}

// AssembleBmadRepos validates bmad_repos paths, detects basename collisions,
// generates mount flags, and renders agent instruction content from the
// embedded template. Returns (mounts, instructionContent, error).
func AssembleBmadRepos(cfg *config.Config) ([]string, string, error) {
	if len(cfg.BmadRepos) == 0 {
		return nil, "", nil
	}

	seen := make(map[string]string) // basename → full path
	mounts := make([]string, 0, len(cfg.BmadRepos))
	repos := make([]BmadRepoInfo, 0, len(cfg.BmadRepos))

	for _, repoPath := range cfg.BmadRepos {
		info, err := os.Stat(repoPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, "", &config.ConfigError{
					Msg: fmt.Sprintf("bmad_repos path '%s' not found. Check bmad_repos in .asbox/config.yaml", repoPath),
				}
			}
			return nil, "", &config.ConfigError{
				Msg: fmt.Sprintf("bmad_repos path '%s': %s", repoPath, err),
			}
		}
		if !info.IsDir() {
			return nil, "", &config.ConfigError{
				Msg: fmt.Sprintf("bmad_repos path '%s' is not a directory. Check bmad_repos in .asbox/config.yaml", repoPath),
			}
		}

		base := filepath.Base(repoPath)
		if base == "/" || base == "." {
			return nil, "", &config.ConfigError{
				Msg: fmt.Sprintf("bmad_repos path '%s' resolves to invalid basename '%s'. Check bmad_repos in .asbox/config.yaml", repoPath, base),
			}
		}
		if existing, ok := seen[base]; ok {
			return nil, "", &config.ConfigError{
				Msg: fmt.Sprintf("bmad_repos basename collision — '%s' resolves from both %s and %s. Rename one directory or use symlinks to disambiguate.", base, existing, repoPath),
			}
		}
		seen[base] = repoPath

		containerPath := bmadRepoMountBase + "/" + base
		mounts = append(mounts, repoPath+":"+containerPath)
		repos = append(repos, BmadRepoInfo{
			Name:          base,
			ContainerPath: containerPath,
		})
	}

	// Render agent instructions template
	tmplBytes, err := asboxEmbed.Assets.ReadFile("agent-instructions.md.tmpl")
	if err != nil {
		return nil, "", fmt.Errorf("bmad_repos: failed to read agent instructions template: %w", err)
	}

	tmpl, err := template.New("agent-instructions").Parse(string(tmplBytes))
	if err != nil {
		return nil, "", fmt.Errorf("bmad_repos: failed to parse agent instructions template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, InstructionData{BmadRepos: repos}); err != nil {
		return nil, "", fmt.Errorf("bmad_repos: failed to render agent instructions: %w", err)
	}

	return mounts, buf.String(), nil
}
