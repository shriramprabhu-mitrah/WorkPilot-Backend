package utils

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed templates/*.html
var embeddedTemplates embed.FS

func RenderEmbeddedTemplate(templateName string, data any) (string, error) {
	if templateName == "" {
		return "", fmt.Errorf("template name cannot be empty")
	}

	templateContent, err := embeddedTemplates.ReadFile("templates/" + templateName)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded template %s: %w", templateName, err)
	}

	tmpl, err := template.New(templateName).Parse(string(templateContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse embedded template %s: %w", templateName, err)
	}

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data); err != nil {
		return "", fmt.Errorf("failed to render embedded template %s: %w", templateName, err)
	}

	return rendered.String(), nil
}
