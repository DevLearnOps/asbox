package config

import "testing"

func TestHasMCP_found(t *testing.T) {
	cfg := &Config{MCP: []string{"playwright"}}
	if !cfg.HasMCP("playwright") {
		t.Error("HasMCP(\"playwright\") = false, want true")
	}
}

func TestHasMCP_notFound(t *testing.T) {
	cfg := &Config{MCP: []string{"playwright"}}
	if cfg.HasMCP("unknown") {
		t.Error("HasMCP(\"unknown\") = true, want false")
	}
}

func TestHasMCP_emptySlice(t *testing.T) {
	cfg := &Config{}
	if cfg.HasMCP("playwright") {
		t.Error("HasMCP(\"playwright\") on empty slice = true, want false")
	}
}

func TestMCPManifestJSON_withPlaywright(t *testing.T) {
	cfg := &Config{MCP: []string{"playwright"}}
	got := cfg.MCPManifestJSON()
	want := `{"mcpServers":{"playwright":{"type":"stdio","command":"npx","args":["-y","@playwright/mcp"]}}}`
	if got != want {
		t.Errorf("MCPManifestJSON() = %q, want %q", got, want)
	}
}

func TestMCPManifestJSON_empty(t *testing.T) {
	cfg := &Config{}
	got := cfg.MCPManifestJSON()
	want := `{"mcpServers":{}}`
	if got != want {
		t.Errorf("MCPManifestJSON() = %q, want %q", got, want)
	}
}
