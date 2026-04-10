package template

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/mcastellin/asbox/internal/config"

	asboxEmbed "github.com/mcastellin/asbox/embed"
)

// Render renders the embedded Dockerfile template using the provided config.
func Render(cfg *config.Config) (string, error) {
	tmplBytes, err := asboxEmbed.Assets.ReadFile("Dockerfile.tmpl")
	if err != nil {
		return "", &TemplateError{Msg: fmt.Sprintf("failed to render Dockerfile: %s", err)}
	}

	tmpl, err := template.New("Dockerfile").Parse(string(tmplBytes))
	if err != nil {
		return "", &TemplateError{Msg: fmt.Sprintf("failed to render Dockerfile: %s", err)}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", &TemplateError{Msg: fmt.Sprintf("failed to render Dockerfile: %s", err)}
	}

	return buf.String(), nil
}
