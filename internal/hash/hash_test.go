package hash

import (
	"testing"
)

func TestCompute_deterministic(t *testing.T) {
	inputs := []string{"dockerfile content", "entrypoint.sh", "git-wrapper.sh", "healthcheck-poller.sh", "config.yaml"}
	h1 := Compute(inputs...)
	h2 := Compute(inputs...)
	if h1 != h2 {
		t.Errorf("Compute is not deterministic: %q != %q", h1, h2)
	}
}

func TestCompute_length(t *testing.T) {
	h := Compute("some", "inputs")
	if len(h) != 12 {
		t.Errorf("expected 12-char hash, got %d chars: %q", len(h), h)
	}
}

func TestCompute_hexChars(t *testing.T) {
	h := Compute("test")
	for _, c := range h {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex character %q in hash %q", string(c), h)
		}
	}
}

func TestCompute_differentInputs(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
	}{
		{
			name: "different dockerfile",
			a:    []string{"FROM ubuntu:24.04", "entry.sh", "git.sh", "health.sh", "config"},
			b:    []string{"FROM ubuntu:22.04", "entry.sh", "git.sh", "health.sh", "config"},
		},
		{
			name: "different entrypoint",
			a:    []string{"dockerfile", "entrypoint-v1", "git.sh", "health.sh", "config"},
			b:    []string{"dockerfile", "entrypoint-v2", "git.sh", "health.sh", "config"},
		},
		{
			name: "different config",
			a:    []string{"dockerfile", "entry.sh", "git.sh", "health.sh", "config-v1"},
			b:    []string{"dockerfile", "entry.sh", "git.sh", "health.sh", "config-v2"},
		},
		{
			name: "different git-wrapper",
			a:    []string{"dockerfile", "entry.sh", "git-v1.sh", "health.sh", "config"},
			b:    []string{"dockerfile", "entry.sh", "git-v2.sh", "health.sh", "config"},
		},
		{
			name: "different healthcheck",
			a:    []string{"dockerfile", "entry.sh", "git.sh", "health-v1.sh", "config"},
			b:    []string{"dockerfile", "entry.sh", "git.sh", "health-v2.sh", "config"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ha := Compute(tt.a...)
			hb := Compute(tt.b...)
			if ha == hb {
				t.Errorf("expected different hashes for different inputs, both got %q", ha)
			}
		})
	}
}

func TestCompute_emptyInputs(t *testing.T) {
	h := Compute()
	if len(h) != 12 {
		t.Errorf("expected 12-char hash even with no inputs, got %d chars: %q", len(h), h)
	}
}
