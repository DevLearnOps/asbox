package config

import "testing"

func TestConfigError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      ConfigError
		expected string
	}{
		{
			name:     "with field",
			err:      ConfigError{Field: "image", Msg: "required"},
			expected: "config field image: required",
		},
		{
			name:     "without field",
			err:      ConfigError{Msg: "not implemented"},
			expected: "not implemented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("ConfigError.Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSecretError_Error(t *testing.T) {
	err := SecretError{Msg: "secret missing"}
	if got := err.Error(); got != "secret missing" {
		t.Errorf("SecretError.Error() = %q, want %q", got, "secret missing")
	}
}
