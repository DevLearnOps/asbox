package docker

import (
	"bytes"
	"strings"
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

func TestImageExists_returnsErrorOrFalse_whenDockerUnavailable(t *testing.T) {
	ref := "asbox-myproject:abc123def456"
	exists, err := ImageExists(ref)
	// Without a running Docker daemon, we expect either (false, nil) or (false, error)
	if exists {
		t.Errorf("expected exists=false when docker is unavailable, got true")
	}
	// err may or may not be nil depending on how docker fails — just verify no panic
	_ = err
}

func TestBuildImage_commandAssembly(t *testing.T) {
	tests := []struct {
		name string
		opts BuildOptions
		want []string // substrings expected in assembled command
	}{
		{
			name: "single tag",
			opts: BuildOptions{
				RenderedDockerfile: "FROM ubuntu:24.04\n",
				Tags:              []string{"asbox-test:abc123"},
				Stdout:            &bytes.Buffer{},
				Stderr:            &bytes.Buffer{},
			},
			want: []string{"-t", "asbox-test:abc123"},
		},
		{
			name: "multiple tags",
			opts: BuildOptions{
				RenderedDockerfile: "FROM ubuntu:24.04\n",
				Tags:              []string{"asbox-test:abc123", "asbox-test:latest"},
				Stdout:            &bytes.Buffer{},
				Stderr:            &bytes.Buffer{},
			},
			want: []string{"-t", "asbox-test:abc123", "-t", "asbox-test:latest"},
		},
		{
			name: "with build args",
			opts: BuildOptions{
				RenderedDockerfile: "FROM ubuntu:24.04\n",
				Tags:              []string{"asbox-test:abc123"},
				BuildArgs:         []string{"--build-arg", "NODE_VERSION=22"},
				Stdout:            &bytes.Buffer{},
				Stderr:            &bytes.Buffer{},
			},
			want: []string{"--build-arg", "NODE_VERSION=22", "-t", "asbox-test:abc123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := buildCmdArgs(tt.opts, "/tmp/fake-dockerfile", "/tmp/fake-context")
			joined := strings.Join(args, " ")
			for _, w := range tt.want {
				if !strings.Contains(joined, w) {
					t.Errorf("expected %q in command args %q", w, joined)
				}
			}
		})
	}
}

func TestBuildImage_tagFormatting(t *testing.T) {
	opts := BuildOptions{
		RenderedDockerfile: "FROM ubuntu\n",
		Tags:              []string{"asbox-myproject:a1b2c3d4e5f6", "asbox-myproject:latest"},
		Stdout:            &bytes.Buffer{},
		Stderr:            &bytes.Buffer{},
	}
	args := buildCmdArgs(opts, "/tmp/df", "/tmp/ctx")
	// Count -t flags
	tagCount := 0
	for _, a := range args {
		if a == "-t" {
			tagCount++
		}
	}
	if tagCount != 2 {
		t.Errorf("expected 2 -t flags, got %d in %v", tagCount, args)
	}
}
