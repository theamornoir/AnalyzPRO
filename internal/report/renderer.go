package report

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"

	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/models"
)

//go:embed templates/bioscan.html
var bioscanTemplateFiles embed.FS

// analysisHTMLTemplate - HTML-шаблон для стандартных анализов (не bioscan)
const analysisHTMLTemplate = `<!DOCTYPE html>
<html lang="ru">
<head>
<meta charset="UTF-8">
<title>{{.Title}}</title>
<style>
body { font-family: Arial, sans-serif; margin: 40px; line-height: 1.6; color: #333; }
h1 { color: #2c3e50; border-bottom: 2px solid #3498db; padding-bottom: 10px; }
h2 { color: #34495e; margin-top: 30px; }
.card { border: 1px solid #e0e0e0; border-radius: 8px; padding: 16px; margin: 10px 0; }
.indicator { display: flex; justify-content: space-between; padding: 8px 0; border-bottom: 1px solid #eee; }
.label { font-weight: bold; color: #2c3e50; }
.status-normal { color: #27ae60; }
.status-warning { color: #f39c12; }
.status-critical { color: #e74c3c; }
.disclaimer { font-size: 12px; color: #7f8c8d; margin-top: 30px; padding-top: 10px; border-top: 1px solid #eee; }
</style>
</head>
<body>
<h1>{{.Title}}</h1>
{{if .Summary}}<div class="card">{{.Summary}}</div>{{end}}
{{if .Categories}}
<h2>Категории</h2>
{{range .Categories}}
<div class="card">
<h3>{{.Name}}</h3>
{{range .Indicators}}
<div class="indicator">
<span class="label">{{.Name}}</span>
<span>{{.Value}} {{.Unit}}</span>
<span class="status-{{.Status}}">{{statusText .Status}}</span>
</div>
{{end}}
</div>
{{end}}
{{end}}
{{range .Recommendations}}
<div class="card">✓ {{.}}</div>
{{end}}
{{if .Disclaimer}}
<div class="disclaimer">{{.Disclaimer}}</div>
{{end}}
</body>
</html>`

type Renderer struct {
	analysisTmpl *template.Template
	bioscanTmpl  *template.Template
}

func NewRenderer() (*Renderer, error) {
	funcMap := template.FuncMap{
		"json": func(v interface{}) template.JS {
			a, _ := json.Marshal(v)
			return template.JS(a)
		},
		"add": func(a, b int) int { return a + b },
		"mul": func(a, b int) int { return a * b },
		"sub": func(a, b int) int { return a - b },
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"list": func(items ...interface{}) []interface{} {
			return items
		},
		"statusClass": func(status string) string {
			switch status {
			case "normal":
				return "status-normal"
			case "warning":
				return "status-warning"
			case "critical":
				return "status-critical"
			default:
				return ""
			}
		},
		"statusIcon": func(status string) string {
			switch status {
			case "normal":
				return locales.StatusIconNormal
			case "warning":
				return locales.StatusIconWarning
			case "critical":
				return locales.StatusIconCritical
			default:
				return locales.StatusIconDefault
			}
		},
		"statusText": func(status string) string {
			switch status {
			case "normal":
				return locales.StatusTextNormal
			case "warning":
				return locales.StatusTextWarning
			case "critical":
				return locales.StatusTextCritical
			default:
				return ""
			}
		},
		"scoreColor": func(score int) string {
			if score >= 80 {
				return "#4CAF50"
			} else if score >= 60 {
				return "#FF9800"
			}
			return "#f44336"
		},
		"categoryIcon": func(name string) string {
			if icon, ok := locales.CategoryIcons[name]; ok {
				return icon
			}
			return locales.CategoryIconDefault
		},
		"substr": func(s string, start, length int) string {
			runes := []rune(s)
			if start >= len(runes) {
				return ""
			}
			end := start + length
			if end > len(runes) {
				end = len(runes)
			}
			return string(runes[start:end])
		},
	}

	analysisTmpl, err := template.New("analysis").Funcs(funcMap).Parse(analysisHTMLTemplate)
	if err != nil {
		return nil, err
	}

	bioscanTmpl, err := template.New("bioscan.html").Funcs(funcMap).ParseFS(
		bioscanTemplateFiles,
		"templates/bioscan.html",
	)
	if err != nil {
		return nil, fmt.Errorf("parse bioscan template: %w", err)
	}

	return &Renderer{
		analysisTmpl: analysisTmpl,
		bioscanTmpl:  bioscanTmpl,
	}, nil
}

func (r *Renderer) Render(report models.Report) (string, error) {
	var buf bytes.Buffer

	var err error
	if report.IsBioscan {
		err = r.bioscanTmpl.ExecuteTemplate(&buf, "bioscan.html", report)
	} else {
		err = r.analysisTmpl.Execute(&buf, report)
	}

	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
