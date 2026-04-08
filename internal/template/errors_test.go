package template

import "testing"

func TestTemplateError_Error(t *testing.T) {
	err := TemplateError{Msg: "parse failed"}
	if got := err.Error(); got != "parse failed" {
		t.Errorf("TemplateError.Error() = %q, want %q", got, "parse failed")
	}
}
