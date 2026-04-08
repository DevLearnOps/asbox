package template

// TemplateError represents a template processing error.
type TemplateError struct {
	Msg string
}

func (e *TemplateError) Error() string {
	return e.Msg
}
