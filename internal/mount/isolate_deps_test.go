package mount

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mcastellin/asbox/internal/config"
)

// helper to create a package.json file at the given directory
func createPackageJSON(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create dir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to create package.json in %s: %v", dir, err)
	}
}

func TestScanDeps_disabledReturnsNil(t *testing.T) {
	cfg := &config.Config{
		AutoIsolateDeps: false,
		ProjectName:     "myapp",
		Mounts: []config.MountConfig{
			{Source: t.TempDir(), Target: "/workspace"},
		},
	}

	results, err := ScanDeps(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil, got %v", results)
	}
}

func TestScanDeps_singleRootPackageJSON(t *testing.T) {
	dir := t.TempDir()
	createPackageJSON(t, dir)

	cfg := &config.Config{
		AutoIsolateDeps: true,
		ProjectName:     "myapp",
		Mounts: []config.MountConfig{
			{Source: dir, Target: "/workspace"},
		},
	}

	results, err := ScanDeps(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len = %d, want 1", len(results))
	}
	if results[0].VolumeName != "asbox-myapp-node_modules" {
		t.Errorf("VolumeName = %q, want %q", results[0].VolumeName, "asbox-myapp-node_modules")
	}
	if results[0].ContainerPath != "/workspace/node_modules" {
		t.Errorf("ContainerPath = %q, want %q", results[0].ContainerPath, "/workspace/node_modules")
	}
}

func TestScanDeps_monorepoThreePackageJSONs(t *testing.T) {
	dir := t.TempDir()
	createPackageJSON(t, dir)
	createPackageJSON(t, filepath.Join(dir, "packages", "api"))
	createPackageJSON(t, filepath.Join(dir, "packages", "web"))

	cfg := &config.Config{
		AutoIsolateDeps: true,
		ProjectName:     "myapp",
		Mounts: []config.MountConfig{
			{Source: dir, Target: "/workspace"},
		},
	}

	results, err := ScanDeps(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("len = %d, want 3", len(results))
	}

	// Build a map for order-independent assertions
	byVolume := make(map[string]ScanResult)
	for _, r := range results {
		byVolume[r.VolumeName] = r
	}

	wantVolumes := map[string]string{
		"asbox-myapp-node_modules":              "/workspace/node_modules",
		"asbox-myapp-packages-api-node_modules": "/workspace/packages/api/node_modules",
		"asbox-myapp-packages-web-node_modules": "/workspace/packages/web/node_modules",
	}

	for vol, wantPath := range wantVolumes {
		r, ok := byVolume[vol]
		if !ok {
			t.Errorf("missing volume %q", vol)
			continue
		}
		if r.ContainerPath != wantPath {
			t.Errorf("volume %q: ContainerPath = %q, want %q", vol, r.ContainerPath, wantPath)
		}
	}
}

func TestScanDeps_noPackageJSON(t *testing.T) {
	dir := t.TempDir()

	cfg := &config.Config{
		AutoIsolateDeps: true,
		ProjectName:     "myapp",
		Mounts: []config.MountConfig{
			{Source: dir, Target: "/workspace"},
		},
	}

	results, err := ScanDeps(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("len = %d, want 0", len(results))
	}
}

func TestScanDeps_nodeModulesExcluded(t *testing.T) {
	dir := t.TempDir()
	createPackageJSON(t, dir)
	// Create a package.json inside node_modules (should be ignored)
	createPackageJSON(t, filepath.Join(dir, "node_modules", "some-pkg"))

	cfg := &config.Config{
		AutoIsolateDeps: true,
		ProjectName:     "myapp",
		Mounts: []config.MountConfig{
			{Source: dir, Target: "/workspace"},
		},
	}

	results, err := ScanDeps(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len = %d, want 1 (node_modules package.json should be excluded)", len(results))
	}
	if results[0].VolumeName != "asbox-myapp-node_modules" {
		t.Errorf("VolumeName = %q, want root result only", results[0].VolumeName)
	}
}

func TestScanDeps_volumeNamingWithDashedPath(t *testing.T) {
	dir := t.TempDir()
	createPackageJSON(t, filepath.Join(dir, "apps", "frontend"))

	cfg := &config.Config{
		AutoIsolateDeps: true,
		ProjectName:     "proj",
		Mounts: []config.MountConfig{
			{Source: dir, Target: "/workspace"},
		},
	}

	results, err := ScanDeps(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len = %d, want 1", len(results))
	}
	if results[0].VolumeName != "asbox-proj-apps-frontend-node_modules" {
		t.Errorf("VolumeName = %q, want %q", results[0].VolumeName, "asbox-proj-apps-frontend-node_modules")
	}
}

func TestScanDeps_scopedNpmPackageSanitized(t *testing.T) {
	dir := t.TempDir()
	createPackageJSON(t, filepath.Join(dir, "packages", "@myorg", "shared"))

	cfg := &config.Config{
		AutoIsolateDeps: true,
		ProjectName:     "proj",
		Mounts: []config.MountConfig{
			{Source: dir, Target: "/workspace"},
		},
	}

	results, err := ScanDeps(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len = %d, want 1", len(results))
	}
	// @ must be stripped — Docker rejects it in volume names
	if results[0].VolumeName != "asbox-proj-packages-myorg-shared-node_modules" {
		t.Errorf("VolumeName = %q, want %q", results[0].VolumeName, "asbox-proj-packages-myorg-shared-node_modules")
	}
}

// --- Tests for AssembleIsolateDeps ---

func TestAssembleIsolateDeps_volumeFlags(t *testing.T) {
	results := []ScanResult{
		{VolumeName: "asbox-myapp-node_modules", ContainerPath: "/workspace/node_modules"},
		{VolumeName: "asbox-myapp-packages-api-node_modules", ContainerPath: "/workspace/packages/api/node_modules"},
	}

	flags, paths := AssembleIsolateDeps(results)

	if len(flags) != 2 {
		t.Fatalf("len(flags) = %d, want 2", len(flags))
	}
	wantFlag0 := "asbox-myapp-node_modules:/workspace/node_modules"
	if flags[0] != wantFlag0 {
		t.Errorf("flags[0] = %q, want %q", flags[0], wantFlag0)
	}
	wantFlag1 := "asbox-myapp-packages-api-node_modules:/workspace/packages/api/node_modules"
	if flags[1] != wantFlag1 {
		t.Errorf("flags[1] = %q, want %q", flags[1], wantFlag1)
	}

	wantPaths := "/workspace/node_modules,/workspace/packages/api/node_modules"
	if paths != wantPaths {
		t.Errorf("paths = %q, want %q", paths, wantPaths)
	}
}

func TestAssembleIsolateDeps_emptyResults(t *testing.T) {
	flags, paths := AssembleIsolateDeps(nil)
	if flags != nil {
		t.Errorf("flags = %v, want nil", flags)
	}
	if paths != "" {
		t.Errorf("paths = %q, want empty", paths)
	}
}

// --- Tests for bmad_repos scanning ---

func TestScanDeps_bmadRepoRootPackageJSON(t *testing.T) {
	repoDir := t.TempDir()
	createPackageJSON(t, repoDir)

	cfg := &config.Config{
		AutoIsolateDeps: true,
		ProjectName:     "myapp",
		BmadRepos:       []string{repoDir},
	}

	results, err := ScanDeps(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len = %d, want 1", len(results))
	}

	base := filepath.Base(repoDir)
	wantContainer := "/workspace/repos/" + base + "/node_modules"
	if results[0].ContainerPath != wantContainer {
		t.Errorf("ContainerPath = %q, want %q", results[0].ContainerPath, wantContainer)
	}
	wantVolume := "asbox-myapp-repos-" + base + "-node_modules"
	if results[0].VolumeName != wantVolume {
		t.Errorf("VolumeName = %q, want %q", results[0].VolumeName, wantVolume)
	}
}

func TestScanDeps_bmadRepoMonorepoNested(t *testing.T) {
	repoDir := t.TempDir()
	createPackageJSON(t, repoDir)
	createPackageJSON(t, filepath.Join(repoDir, "packages", "core"))
	createPackageJSON(t, filepath.Join(repoDir, "packages", "web"))

	cfg := &config.Config{
		AutoIsolateDeps: true,
		ProjectName:     "myapp",
		BmadRepos:       []string{repoDir},
	}

	results, err := ScanDeps(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("len = %d, want 3", len(results))
	}

	byVolume := make(map[string]ScanResult)
	for _, r := range results {
		byVolume[r.VolumeName] = r
	}

	base := filepath.Base(repoDir)
	wantVolumes := map[string]string{
		"asbox-myapp-repos-" + base + "-node_modules":              "/workspace/repos/" + base + "/node_modules",
		"asbox-myapp-repos-" + base + "-packages-core-node_modules": "/workspace/repos/" + base + "/packages/core/node_modules",
		"asbox-myapp-repos-" + base + "-packages-web-node_modules":  "/workspace/repos/" + base + "/packages/web/node_modules",
	}

	for vol, wantPath := range wantVolumes {
		r, ok := byVolume[vol]
		if !ok {
			t.Errorf("missing volume %q", vol)
			continue
		}
		if r.ContainerPath != wantPath {
			t.Errorf("volume %q: ContainerPath = %q, want %q", vol, r.ContainerPath, wantPath)
		}
	}
}

func TestScanDeps_combinedMountsAndBmadRepos(t *testing.T) {
	mountDir := t.TempDir()
	createPackageJSON(t, mountDir)

	repoDir := t.TempDir()
	createPackageJSON(t, repoDir)
	createPackageJSON(t, filepath.Join(repoDir, "packages", "lib"))

	cfg := &config.Config{
		AutoIsolateDeps: true,
		ProjectName:     "myapp",
		Mounts: []config.MountConfig{
			{Source: mountDir, Target: "/workspace"},
		},
		BmadRepos: []string{repoDir},
	}

	results, err := ScanDeps(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 1 from mount + 2 from bmad repo = 3
	if len(results) != 3 {
		t.Fatalf("len = %d, want 3", len(results))
	}

	// Verify mount result uses /workspace target
	if results[0].ContainerPath != "/workspace/node_modules" {
		t.Errorf("mount result ContainerPath = %q, want /workspace/node_modules", results[0].ContainerPath)
	}

	// Verify bmad results use /workspace/repos/<basename> target
	base := filepath.Base(repoDir)
	wantRepoRoot := "/workspace/repos/" + base + "/node_modules"
	if results[1].ContainerPath != wantRepoRoot {
		t.Errorf("bmad root ContainerPath = %q, want %q", results[1].ContainerPath, wantRepoRoot)
	}
	wantRepoNested := "/workspace/repos/" + base + "/packages/lib/node_modules"
	if results[2].ContainerPath != wantRepoNested {
		t.Errorf("bmad nested ContainerPath = %q, want %q", results[2].ContainerPath, wantRepoNested)
	}

	// Volume names must be distinct between mount and bmad_repo
	if results[0].VolumeName == results[1].VolumeName {
		t.Errorf("mount and bmad_repo root volume names collide: %q", results[0].VolumeName)
	}
}

func TestScanDeps_bmadRepoNoPackageJSON(t *testing.T) {
	repoDir := t.TempDir()

	cfg := &config.Config{
		AutoIsolateDeps: true,
		ProjectName:     "myapp",
		Mounts: []config.MountConfig{
			{Source: t.TempDir(), Target: "/workspace"},
		},
		BmadRepos: []string{repoDir},
	}

	results, err := ScanDeps(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("len = %d, want 0", len(results))
	}
}

func TestScanDeps_bmadRepoVolumeNaming(t *testing.T) {
	repoDir := t.TempDir()
	createPackageJSON(t, filepath.Join(repoDir, "apps", "dashboard"))

	cfg := &config.Config{
		AutoIsolateDeps: true,
		ProjectName:     "proj",
		BmadRepos:       []string{repoDir},
	}

	results, err := ScanDeps(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len = %d, want 1", len(results))
	}
	base := filepath.Base(repoDir)
	wantVolume := "asbox-proj-repos-" + base + "-apps-dashboard-node_modules"
	if results[0].VolumeName != wantVolume {
		t.Errorf("VolumeName = %q, want %q", results[0].VolumeName, wantVolume)
	}
}
