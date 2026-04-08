package embed

import "testing"

func TestAssets_containsExpectedFiles(t *testing.T) {
	expectedFiles := []string{
		"Dockerfile.tmpl",
		"entrypoint.sh",
		"git-wrapper.sh",
		"healthcheck-poller.sh",
		"agent-instructions.md.tmpl",
		"config.yaml",
	}

	for _, name := range expectedFiles {
		t.Run(name, func(t *testing.T) {
			data, err := Assets.ReadFile(name)
			if err != nil {
				t.Fatalf("Assets.ReadFile(%q) failed: %v", name, err)
			}
			if len(data) == 0 {
				t.Errorf("Assets.ReadFile(%q) returned empty content", name)
			}
		})
	}
}
