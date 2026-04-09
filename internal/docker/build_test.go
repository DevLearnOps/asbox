package docker

import (
	"testing"

	"github.com/mcastellin/asbox/internal/config"
)

func TestBuildArgs_nodejsOnly(t *testing.T) {
	cfg := &config.Config{
		SDKs: config.SDKConfig{NodeJS: "22"},
	}
	args := BuildArgs(cfg)
	expected := []string{"--build-arg", "NODE_VERSION=22"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, v := range expected {
		if args[i] != v {
			t.Errorf("args[%d] = %q, want %q", i, args[i], v)
		}
	}
}

func TestBuildArgs_allSDKs(t *testing.T) {
	cfg := &config.Config{
		SDKs: config.SDKConfig{NodeJS: "22", Go: "1.23.0", Python: "3.12"},
	}
	args := BuildArgs(cfg)
	if len(args) != 6 {
		t.Fatalf("expected 6 args, got %d: %v", len(args), args)
	}
	expectPairs := [][2]string{
		{"--build-arg", "NODE_VERSION=22"},
		{"--build-arg", "GO_VERSION=1.23.0"},
		{"--build-arg", "PYTHON_VERSION=3.12"},
	}
	for i, pair := range expectPairs {
		if args[i*2] != pair[0] || args[i*2+1] != pair[1] {
			t.Errorf("expected %v at position %d, got [%q, %q]", pair, i*2, args[i*2], args[i*2+1])
		}
	}
}

func TestBuildArgs_noSDKs(t *testing.T) {
	cfg := &config.Config{}
	args := BuildArgs(cfg)
	if len(args) != 0 {
		t.Errorf("expected empty slice, got %v", args)
	}
}

func TestBuildArgs_partialSDKs(t *testing.T) {
	cfg := &config.Config{
		SDKs: config.SDKConfig{Go: "1.23.0", Python: "3.12"},
	}
	args := BuildArgs(cfg)
	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %d: %v", len(args), args)
	}
	if args[1] != "GO_VERSION=1.23.0" {
		t.Errorf("expected GO_VERSION=1.23.0, got %q", args[1])
	}
	if args[3] != "PYTHON_VERSION=3.12" {
		t.Errorf("expected PYTHON_VERSION=3.12, got %q", args[3])
	}
	// Ensure no NODE_VERSION
	for _, a := range args {
		if a == "NODE_VERSION=22" {
			t.Error("unexpected NODE_VERSION arg when NodeJS not configured")
		}
	}
}
