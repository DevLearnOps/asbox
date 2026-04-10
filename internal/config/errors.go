package config

import "fmt"

// ConfigError represents a configuration or general error.
type ConfigError struct {
	Field string
	Msg   string
}

func (e *ConfigError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("config field %s: %s", e.Field, e.Msg)
	}
	return e.Msg
}

// SecretError represents a secret-related error.
type SecretError struct {
	Msg string
}

func (e *SecretError) Error() string {
	return e.Msg
}
