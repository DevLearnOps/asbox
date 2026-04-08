package docker

import "testing"

func TestDependencyError_Error(t *testing.T) {
	err := DependencyError{Msg: "docker not found. Install Docker Engine 20.10+ or Docker Desktop"}
	expected := "docker not found. Install Docker Engine 20.10+ or Docker Desktop"
	if got := err.Error(); got != expected {
		t.Errorf("DependencyError.Error() = %q, want %q", got, expected)
	}
}
