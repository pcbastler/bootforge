package httpboot

import (
	"bytes"
	"fmt"
	"text/template"
)

// TemplateVars holds variables available to preseed/kickstart templates.
type TemplateVars struct {
	ServerIP string
	HTTPPort int
	MAC      string
	Hostname string
	Custom   map[string]string
}

// RenderTemplate renders a Go text/template string with the given variables.
func RenderTemplate(tmplStr string, vars TemplateVars) (string, error) {
	tmpl, err := template.New("boot").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}
	return buf.String(), nil
}
