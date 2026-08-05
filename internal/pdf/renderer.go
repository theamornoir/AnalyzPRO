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
		return nil, fmt.Errorf(
			"parse report template: %w",
			err,
		)
	}

	fmt.Println("========== TEMPLATES ==========")

	for _, t := range tmpl.Templates() {
		fmt.Println("NAME:", t.Name())
		fmt.Println("LEN:", len(t.Tree.Root.String()))
	}

	fmt.Println("===============================")

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

		return "",
			fmt.Errorf(
				"render report: %w",
				err,
			)
	}

	html := buffer.String()

	fmt.Println("===================================")
	fmt.Println("HTML RESULT LEN:", len(html))

	if len(html) > 200 {
		fmt.Println(html[:200])
	}

	fmt.Println("===================================")

	return html, nil
}
