package report

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"

	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/report/models"
)

// adaptiveHTMLTemplate - шаблон адаптивного отчёта с чистым CSS/SVG и мятным цветом.
const adaptiveHTMLTemplate = `<!DOCTYPE html>
<html lang="ru">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Title}}</title>
<style>
:root {
    --mint: #1FA6A8;
    --mint-light: #73D2D4;
    --mint-bg: #F0F8F8;
    --dark: #1A2A2A;
    --gray: #6B7A7A;
    --border: #E0EAEA;
    --normal: #4F8A6D;
    --warning: #E8744A;
    --critical: #D32F2F;
}

* { margin: 0; padding: 0; box-sizing: border-box; }

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Arial, sans-serif;
    background: #E9EEEE;
    color: var(--dark);
    line-height: 1.6;
    padding: 20px;
}

.container {
    max-width: 800px;
    margin: 0 auto;
    background: white;
    border-radius: 16px;
    box-shadow: 0 4px 20px rgba(0,0,0,0.08);
    overflow: hidden;
}

/* HEADER */
.header {
    background: linear-gradient(135deg, #0A1F2A, #164451);
    color: white;
    padding: 40px 30px;
    text-align: center;
}

.header .brand {
    font-size: 10px;
    letter-spacing: 4px;
    color: var(--mint-light);
    text-transform: uppercase;
    margin-bottom: 12px;
}

.header h1 {
    font-size: 28px;
    font-weight: 300;
    margin-bottom: 8px;
}

.header h1 span {
    font-weight: 800;
    color: var(--mint-light);
}

/* SUMMARY */
.summary {
    padding: 24px 30px;
    background: var(--mint-bg);
    border-left: 4px solid var(--mint);
    margin: 20px 30px;
    border-radius: 8px;
    font-size: 14px;
    color: var(--gray);
}

/* SECTIONS */
.section {
    padding: 24px 30px;
    border-bottom: 1px solid var(--border);
}

.section:last-child {
    border-bottom: none;
}

.section-title {
    font-size: 18px;
    font-weight: 700;
    color: var(--dark);
    margin-bottom: 16px;
    display: flex;
    align-items: center;
    gap: 10px;
}

.section-title .icon {
    font-size: 20px;
}

/* INDICATORS */
.indicator-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
    gap: 12px;
}

.indicator-card {
    background: var(--mint-bg);
    border-radius: 12px;
    padding: 16px;
    border: 1px solid var(--border);
}

.indicator-card .name {
    font-size: 11px;
    color: var(--gray);
    text-transform: uppercase;
    letter-spacing: 0.5px;
    margin-bottom: 4px;
}

.indicator-card .value-row {
    display: flex;
    align-items: baseline;
    gap: 8px;
    margin-bottom: 6px;
}

.indicator-card .value {
    font-size: 24px;
    font-weight: 800;
    color: var(--dark);
}

.indicator-card .unit {
    font-size: 12px;
    color: var(--gray);
}

.indicator-card .status {
    font-size: 11px;
    font-weight: 600;
    padding: 2px 8px;
    border-radius: 4px;
    display: inline-block;
}

.status-normal { background: #E8F5E9; color: var(--normal); }
.status-warning { background: #FFF3E0; color: var(--warning); }
.status-critical { background: #FFEBEE; color: var(--critical); }

.indicator-card .normal {
    font-size: 10px;
    color: var(--gray);
    margin-top: 4px;
}

/* SCORE CIRCLE */
.score-circle {
    width: 120px;
    height: 120px;
    margin: 16px auto;
    position: relative;
}

.score-circle svg {
    transform: rotate(-90deg);
    width: 120px;
    height: 120px;
}

.score-circle circle {
    fill: none;
    stroke-width: 10;
    stroke-linecap: round;
}

.score-circle .bg { stroke: var(--border); }
.score-circle .progress {
    stroke: var(--mint);
    stroke-dasharray: 314.16;
    stroke-dashoffset: calc(314.16 - (314.16 * {{.Score}}) / 100);
}

.score-circle .value {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    font-size: 28px;
    font-weight: 800;
    color: var(--dark);
}

/* LIST ITEMS */
.list-item {
    padding: 12px 16px;
    background: var(--mint-bg);
    border-radius: 8px;
    margin-bottom: 8px;
    border-left: 3px solid var(--mint);
    font-size: 14px;
}

.list-item:last-child {
    margin-bottom: 0;
}

/* WARNING */
.list-item.warning {
    border-left-color: var(--warning);
    background: #FFF8F5;
}

.list-item.critical {
    border-left-color: var(--critical);
    background: #FFF5F5;
}

/* DISCLAIMER */
.disclaimer {
    padding: 20px 30px;
    background: #F5F5F5;
    font-size: 11px;
    color: var(--gray);
    text-align: center;
    border-top: 1px solid var(--border);
}

/* RESPONSIVE */
@media (max-width: 600px) {
    body { padding: 10px; }
    .container { border-radius: 12px; }
    .header { padding: 30px 20px; }
    .header h1 { font-size: 22px; }
    .section { padding: 20px; }
    .indicator-grid { grid-template-columns: 1fr; }
}
</style>
</head>
<body>
<div class="container">

    <div class="header">
        <div class="brand">PRISMA</div>
        <h1>{{.Title}}</h1>
    </div>

    {{if .Summary}}
    <div class="summary">{{.Summary}}</div>
    {{end}}

    {{range .Sections}}
    <div class="section">
        <div class="section-title">
            {{if eq .Type "blood"}}<span class="icon">🩸</span>{{end}}
            {{if eq .Type "lifestyle"}}<span class="icon">🏃</span>{{end}}
            {{if eq .Type "nutrition"}}<span class="icon">🥗</span>{{end}}
            {{if eq .Type "recommendation"}}<span class="icon">💡</span>{{end}}
            {{if eq .Type "warning"}}<span class="icon">⚠️</span>{{end}}
            {{if eq .Type "profile"}}<span class="icon">📊</span>{{end}}
            {{.Title}}
        </div>

        {{if .Indicators}}
        <div class="indicator-grid">
            {{range .Indicators}}
            <div class="indicator-card">
                <div class="name">{{.Name}}</div>
                <div class="value-row">
                    <span class="value">{{.Value}}</span>
                    {{if .Unit}}<span class="unit">{{.Unit}}</span>{{end}}
                </div>
                <span class="status status-{{.Status}}">{{statusText .Status}}</span>
                {{if .Normal}}
                <div class="normal">Норма: {{.Normal}}</div>
                {{end}}
            </div>
            {{end}}
        </div>
        {{end}}

        {{if .Summary}}
        <div style="margin-top:12px;font-size:14px;color:var(--gray);">{{.Summary}}</div>
        {{end}}

        {{if .List}}
        {{range .List}}
        <div class="list-item">{{.}}</div>
        {{end}}
        {{end}}

        {{if .Score}}
        <div class="score-circle">
            <svg viewBox="0 0 120 120">
                <circle class="bg" cx="60" cy="60" r="50"/>
                <circle class="progress" cx="60" cy="60" r="50"/>
            </svg>
            <div class="value">{{.Score}}</div>
        </div>
        {{end}}
    </div>
    {{end}}

    {{if .Disclaimer}}
    <div class="disclaimer">{{.Disclaimer}}</div>
    {{end}}

</div>
</body>
</html>`

// RenderAdaptiveReport - рендерит адаптивный HTML-отчёт из данных.
func RenderAdaptiveReport(data models.AdaptiveReportData) string {
	funcMap := template.FuncMap{
		"statusText": func(status string) string {
			switch status {
			case "normal":
				return locales.RptMsgAdaptiveStatusNormal
			case "warning":
				return locales.RptMsgAdaptiveStatusWarning
			case "critical":
				return locales.RptMsgAdaptiveStatusCritical
			default:
				return ""
			}
		},
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	tmpl, err := template.New("adaptive").Funcs(funcMap).Parse(adaptiveHTMLTemplate)
	if err != nil {
		return fmt.Sprintf(locales.RptErrAdaptiveRender, err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return fmt.Sprintf(locales.RptErrAdaptiveRender, err)
	}

	return sanitizeHTML(buf.String())
}

// sanitizeHTML - базовая очистка от потенциально опасных тегов.
func sanitizeHTML(html string) string {
	// Заменяем <script> теги
	html = strings.ReplaceAll(html, "<script>", "")
	html = strings.ReplaceAll(html, "</script>", "")
	return html
}
