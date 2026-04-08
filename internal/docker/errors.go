package docker

// DependencyError represents an external dependency error.
type DependencyError struct {
	Msg string
}

func (e *DependencyError) Error() string {
	return e.Msg
}
