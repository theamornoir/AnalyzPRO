package report

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
)

//go:embed template/bioscan_report.html
var templateFiles embed.FS

type Renderer struct {
	template *template.Template
}

func NewRenderer() (*Renderer, error) {

	tmpl, err := template.ParseFS(
		templateFiles,
		"template/bioscan_report.html",
	)

	if err != nil {
		return nil, fmt.Errorf("parse report template: %w", err)
	}

	return &Renderer{
		template: tmpl,
	}, nil
}

func (r *Renderer) Render(report Report) (string, error) {

	var buffer bytes.Buffer

	err := r.template.ExecuteTemplate(
		&buffer,
		"bioscan_report.html",
		report,
	)

	if err != nil {
		return "", fmt.Errorf("render report: %w", err)
	}

	html := buffer.String()

	fmt.Println("RENDER HTML LEN:", len(html))

	return html, nil
}
