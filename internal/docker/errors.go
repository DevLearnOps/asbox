package docker

// DependencyError represents an external dependency error.
type DependencyError struct {
	Msg string
}

func (e *DependencyError) Error() string {
	return e.Msg
}

// BuildError represents a Docker build failure.
type BuildError struct {
	Msg string
}

func (e *BuildError) Error() string {
	return e.Msg
}
