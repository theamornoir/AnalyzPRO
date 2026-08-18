package report

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"strings"

	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/models"
)

//go:embed templates/bioscan.html
var bioscanTemplateFiles embed.FS

// healthDossierTemplateFiles - шаблон универсального отчёта-досье здоровья
// (расширенный анализ). Построен на основе присланных анализов и
// 20-вопросного опросника об образе жизни.
//
//go:embed templates/health_dossier.html
var healthDossierTemplateFiles embed.FS

// bodyScanTemplateFiles - шаблон премиального отчёта Bioscan PRO
// (Body Intelligence). Premium print-ready HTML по фото + опроснику.
//
//go:embed templates/body_scan_report.html
var bodyScanTemplateFiles embed.FS

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
	dossierTmpl  *template.Template
	bodyScanTmpl *template.Template
}

// ============================================================================
// Helpers для премиального отчёта Bioscan PRO (Body Intelligence).
// ============================================================================

// donutCirc - длина окружности для донатов (r=80).
const donutCirc = 502.65

// donutDash возвращает stroke-dasharray для доната по оценке 0-100:
// "заполнено остаток" в координатах длины окружности.
func donutDash(score int) string {
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	filled := float64(score) / 100 * donutCirc
	return fmt.Sprintf("%.2f %.2f", filled, donutCirc-filled)
}

// bodyStatusClass возвращает CSS-класс статуса (good/warning/critical).
func bodyStatusClass(status string) string {
	switch status {
	case "good", "normal":
		return "good"
	case "warning", "attention":
		return "warning"
	case "critical", "risk":
		return "critical"
	default:
		return "good"
	}
}

// bodyStatusText - человекочитаемая метка статуса.
func bodyStatusText(status string) string {
	switch status {
	case "good", "normal":
		return locales.RptMsgBodyStatusNormal
	case "warning", "attention":
		return locales.RptMsgBodyStatusWarning
	case "critical", "risk":
		return locales.RptMsgBodyStatusCritical
	default:
		return status
	}
}

// bodyStatusColor - приглушённый цвет статуса (mint/amber/coral).
func bodyStatusColor(status string) string {
	switch status {
	case "good", "normal":
		return "#3FA796"
	case "warning", "attention":
		return "#C99A4E"
	case "critical", "risk":
		return "#C97A6E"
	default:
		return "#3FA796"
	}
}

// nl2p преобразует текст с разделителями "\n\n" в набор HTML-параграфов,
// сохраняя переносы строк внутри абзаца как <br>. Возвращает template.HTML
// (безопасно для вставки в шаблон). Используется для длинных narrative.
func nl2p(s string) template.HTML {
	parts := strings.Split(s, "\n\n")
	var sb strings.Builder
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Внутри абзаца сохраняем одиночные переносы строк.
		withBreaks := strings.ReplaceAll(p, "\n", "<br>")
		sb.WriteString("<p>")
		sb.WriteString(withBreaks)
		sb.WriteString("</p>")
	}
	return template.HTML(sb.String())
}

// zoneDonuts строит сетку мини-донатов (круговых диаграмм) для зон тела -
// визуальная альтернатива «силуэту человечка». Каждый донат показывает
// оценку зоны 0-100 цветом по статусу. Возвращает template.HTML.
func zoneDonuts(zones []models.BodyScanZone) template.HTML {
	if len(zones) == 0 {
		return ""
	}
	const r = 28.0
	circ := 2 * math.Pi * r
	var sb strings.Builder
	sb.WriteString(`<div class="zdonut-grid">`)
	for _, z := range zones {
		score := z.Score
		if score < 0 {
			score = 0
		}
		if score > 100 {
			score = 100
		}
		filled := float64(score) / 100 * circ
		color := bodyStatusColor(z.Status)
		sb.WriteString(`<div class="zdonut">`)
		sb.WriteString(fmt.Sprintf(`<svg viewBox="0 0 72 72" width="72" height="72" role="img" aria-label="%s">`, z.Name))
		sb.WriteString(fmt.Sprintf(`<circle cx="36" cy="36" r="%.1f" fill="none" stroke="#E4EDEC" stroke-width="7"/>`, r))
		sb.WriteString(fmt.Sprintf(`<circle cx="36" cy="36" r="%.1f" fill="none" stroke="%s" stroke-width="7" stroke-linecap="round" stroke-dasharray="%.2f %.2f" transform="rotate(-90 36 36)"/>`, r, color, filled, circ-filled))
		sb.WriteString(fmt.Sprintf(`<text x="36" y="41" text-anchor="middle" font-size="18" font-weight="700" fill="#102F35">%d</text>`, score))
		sb.WriteString(`</svg>`)
		sb.WriteString(fmt.Sprintf(`<div class="zdonut-label">%s</div>`, z.Name))
		sb.WriteString(`</div>`)
	}
	sb.WriteString(`</div>`)
	return template.HTML(sb.String())
}

// postureRadar строит inline-SVG radar-диаграмму осанки/баланса (7 осей)
// из полей BodyScanPosture. Возвращает template.HTML (безопасно для вставки).
func postureRadar(p models.BodyScanPosture) template.HTML {
	axes := []struct {
		label string
		value int
	}{
		{locales.RptMsgPdfPosture, p.PostureScore},
		{locales.RptMsgRadarAxisSymmetry, p.Symmetry},
		{locales.RptMsgRadarAxisShoulders, p.ShoulderBalance},
		{locales.RptMsgRadarAxisPelvis, p.PelvicBalance},
		{locales.RptMsgRadarAxisSpine, p.SpinalAlignment},
		{locales.RptMsgRadarAxisMobility, p.Mobility},
		{locales.RptMsgRadarAxisStability, p.Stability},
	}
	const size = 230
	cx, cy, R := float64(size)/2, float64(size)/2, 88.0
	n := len(axes)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg viewBox="0 0 %d %d" width="100%%" height="auto" role="img" aria-label="Radar осанки">`, size, size))

	// Концентрические кольца.
	for ring := 1; ring <= 4; ring++ {
		pts := make([]string, n)
		for i := 0; i < n; i++ {
			ang := -math.Pi/2 + float64(i)*2*math.Pi/float64(n)
			r := R * float64(ring) / 4
			x := cx + r*math.Cos(ang)
			y := cy + r*math.Sin(ang)
			pts[i] = fmt.Sprintf("%.1f,%.1f", x, y)
		}
		sb.WriteString(fmt.Sprintf(`<polygon points="%s" fill="none" stroke="#E4EDEC" stroke-width="1"/>`, strings.Join(pts, " ")))
	}

	// Оси + подписи.
	for i, ax := range axes {
		ang := -math.Pi/2 + float64(i)*2*math.Pi/float64(n)
		x := cx + R*math.Cos(ang)
		y := cy + R*math.Sin(ang)
		sb.WriteString(fmt.Sprintf(`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#E4EDEC" stroke-width="1"/>`, cx, cy, x, y))
		// Подпись чуть за пределами кольца.
		lx := cx + (R+14)*math.Cos(ang)
		ly := cy + (R+14)*math.Sin(ang)
		anchor := "middle"
		if lx < cx-5 {
			anchor = "end"
		} else if lx > cx+5 {
			anchor = "start"
		}
		sb.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" font-size="8" fill="#6C7D80" text-anchor="%s">%s</text>`, lx, ly+3, anchor, ax.label))
	}

	// Значения (полигон).
	valPts := make([]string, n)
	for i, ax := range axes {
		ang := -math.Pi/2 + float64(i)*2*math.Pi/float64(n)
		v := float64(ax.value)
		if v < 0 {
			v = 0
		}
		if v > 100 {
			v = 100
		}
		r := R * v / 100
		x := cx + r*math.Cos(ang)
		y := cy + r*math.Sin(ang)
		valPts[i] = fmt.Sprintf("%.1f,%.1f", x, y)
	}
	sb.WriteString(fmt.Sprintf(`<polygon points="%s" fill="rgba(115,214,192,0.22)" stroke="#286A6B" stroke-width="2" stroke-linejoin="round"/>`, strings.Join(valPts, " ")))
	for i, ax := range axes {
		ang := -math.Pi/2 + float64(i)*2*math.Pi/float64(n)
		v := float64(ax.value)
		if v < 0 {
			v = 0
		}
		if v > 100 {
			v = 100
		}
		r := R * v / 100
		x := cx + r*math.Cos(ang)
		y := cy + r*math.Sin(ang)
		sb.WriteString(fmt.Sprintf(`<circle cx="%.1f" cy="%.1f" r="2.6" fill="#286A6B"/>`, x, y))
	}

	sb.WriteString(`</svg>`)
	return template.HTML(sb.String())
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
		// donutColor - цвет дуги доната по статусу (lifestyle/лаб).
		"donutColor": func(status string) string {
			switch status {
			case "good", "normal":
				return "#45D0B0"
			case "warning":
				return "#E8836B"
			case "risk", "critical":
				return "#D97070"
			default:
				return "#45D0B0"
			}
		},
		// pillClass - CSS-класс пилюли статуса (normal/warning/critical).
		"pillClass": func(status string) string {
			switch status {
			case "normal", "good":
				return "normal"
			case "warning":
				return "warning"
			case "critical", "risk":
				return "critical"
			default:
				return "normal"
			}
		},
		// statusLabel - человекочитаемая метка статуса.
		"statusLabel": func(status string) string {
			switch status {
			case "normal", "good":
				return locales.RptMsgStatusLabelNormal
			case "warning":
				return locales.RptMsgStatusLabelWarning
			case "critical", "risk":
				return locales.RptMsgStatusLabelCritical
			default:
				return status
			}
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
		// donutDash - stroke-dasharray для доната по оценке 0-100.
		"donutDash": donutDash,
		// bodyStatusClass - CSS-класс статуса (good/warning/critical).
		"bodyStatusClass": bodyStatusClass,
		// bodyStatusText - метка статуса (в норме/внимание/риск).
		"bodyStatusText": bodyStatusText,
		// bodyStatusColor - приглушённый цвет статуса.
		"bodyStatusColor": bodyStatusColor,
		// radar - SVG radar-диаграмма осанки/баланса.
		"radar": postureRadar,
		// nl2p - текст с \n\n в набор <p> (абзацы).
		"nl2p": nl2p,
		// zoneDonuts - сетка мини-донатов для зон тела (круговые диаграммы).
		"zoneDonuts": zoneDonuts,
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

	dossierTmpl, err := template.New("health_dossier.html").Funcs(funcMap).ParseFS(
		healthDossierTemplateFiles,
		"templates/health_dossier.html",
	)
	if err != nil {
		return nil, fmt.Errorf("parse health dossier template: %w", err)
	}

	bodyScanTmpl, err := template.New("body_scan_report.html").Funcs(funcMap).ParseFS(
		bodyScanTemplateFiles,
		"templates/body_scan_report.html",
	)
	if err != nil {
		return nil, fmt.Errorf("parse body scan report template: %w", err)
	}

	return &Renderer{
		analysisTmpl: analysisTmpl,
		bioscanTmpl:  bioscanTmpl,
		dossierTmpl:  dossierTmpl,
		bodyScanTmpl: bodyScanTmpl,
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

// RenderDossier рендерит универсальный отчёт-досье здоровья (расширенный
// анализ) из структуры models.HealthDossier в богатый print-ready HTML.
func (r *Renderer) RenderDossier(dossier models.HealthDossier) (string, error) {
	var buf bytes.Buffer
	if err := r.dossierTmpl.Execute(&buf, dossier); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderBodyScan рендерит премиальный отчёт Bioscan PRO (Body Intelligence)
// из структуры models.BodyScanReport в подробный print-ready HTML.
func (r *Renderer) RenderBodyScan(report models.BodyScanReport) (string, error) {
	var buf bytes.Buffer
	if err := r.bodyScanTmpl.Execute(&buf, report); err != nil {
		return "", err
	}
	return buf.String(), nil
}
