package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/mcastellin/asbox/internal/config"
)

func TestMCP_manifestExistsWithPlaywrightEntry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := &config.Config{
		Agent:       "claude-code",
		ProjectName: "integration-test",
		MCP:         []string{"playwright"},
		SDKs:        config.SDKConfig{NodeJS: "20"},
	}
	image := buildTestImageWithConfig(t, cfg)
	ctx := context.Background()

	// Must use real entrypoint for MCP merge — tail -f /dev/null skips it
	container := startTestContainerWithEntrypoint(ctx, t, image)

	t.Run("build_time_manifest_exists", func(t *testing.T) {
		t.Parallel()
		content := fileContentInContainer(ctx, t, container, "/etc/sandbox/mcp-servers.json")
		if !strings.Contains(content, `"playwright"`) {
			t.Errorf("manifest missing playwright entry: %s", content)
		}
		if !strings.Contains(content, `"npx"`) {
			t.Errorf("manifest missing npx command: %s", content)
		}
	})

	t.Run("runtime_mcp_json_generated", func(t *testing.T) {
		t.Parallel()
		if !fileExistsInContainer(ctx, t, container, "/home/sandbox/.mcp.json") {
			t.Error("expected /home/sandbox/.mcp.json after entrypoint merge")
		}
	})

	t.Run("merged_config_contains_playwright", func(t *testing.T) {
		t.Parallel()
		content := fileContentInContainer(ctx, t, container, "/home/sandbox/.mcp.json")
		if !strings.Contains(content, `"mcpServers"`) {
			t.Errorf("merged .mcp.json missing mcpServers key: %s", content)
		}
		if !strings.Contains(content, `"playwright"`) {
			t.Errorf("merged .mcp.json missing playwright: %s", content)
		}
	})
}
