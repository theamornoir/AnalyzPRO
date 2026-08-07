package report

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
)

//go:embed template/bioscan_report.html
var bioscanTemplateFiles embed.FS

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
				return "✅"
			case "warning":
				return "⚠️"
			case "critical":
				return "❌"
			default:
				return "ℹ️"
			}
		},
		"statusText": func(status string) string {
			switch status {
			case "normal":
				return "В норме"
			case "warning":
				return "Требует внимания"
			case "critical":
				return "Отклонение"
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
			icons := map[string]string{
				"Общий анализ крови":   "🩸",
				"Биохимический анализ": "🧬",
				"Гормоны":              "💉",
				"Липидный профиль":     "📊",
				"Коагулограмма":        "🧫",
				"Иммунология":          "🛡️",
				"Микроэлементы":        "🔬",
				"Витамины":             "💊",
				"Онкомаркеры":          "🎯",
				"Маркеры воспаления":   "🔥",
			}
			if icon, ok := icons[name]; ok {
				return icon
			}
			return "📋"
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

	bioscanTmpl, err := template.New("bioscan_report.html").Funcs(funcMap).ParseFS(
		bioscanTemplateFiles,
		"template/bioscan_report.html",
	)
	if err != nil {
		return nil, fmt.Errorf("parse bioscan template: %w", err)
	}

	return &Renderer{
		analysisTmpl: analysisTmpl,
		bioscanTmpl:  bioscanTmpl,
	}, nil
}

func (r *Renderer) ConvertToPDF(report Report) ([]byte, error) {
	html, err := r.Render(report)
	if err != nil {
		return nil, err
	}
	return ConvertHTMLToPDF(html)
}

func (r *Renderer) Render(report Report) (string, error) {
	var buf bytes.Buffer

	var err error
	if report.IsBioscan {
		err = r.bioscanTmpl.ExecuteTemplate(&buf, "bioscan_report.html", report)
	} else {
		err = r.analysisTmpl.Execute(&buf, report)
	}

	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

const analysisHTMLTemplate = `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <title>AnalyzPRO Medical Report</title>
    <style>
        @page {
            size: A4;
            margin: 10mm 12mm 10mm 12mm;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        html, body {
            margin: 0; padding: 0;
            background: #e8edf2;
            font-family: "Segoe UI", Arial, sans-serif;
            color: #1a202c;
            font-size: 10pt;
            -webkit-print-color-adjust: exact;
            print-color-adjust: exact;
            color-adjust: exact;
        }
        .page {
            page-break-after: always;
            break-after: page;
            width: 100%;
            min-height: 100vh;
            background: #f7f9fb;
        }
        .page:last-child { page-break-after: avoid; break-after: avoid; }
        .container { width: 100%; max-width: 100%; padding: 0; }

        .header {
            background: linear-gradient(135deg, #0d1b2a 0%, #1b3a5c 40%, #2d6a9f 100%);
            color: white;
            padding: 16px 22px;
            border-radius: 12px;
            margin-bottom: 14px;
            break-inside: avoid;
            page-break-inside: avoid;
            -webkit-print-color-adjust: exact;
            print-color-adjust: exact;
        }
        .header h1 { margin: 0; font-size: 22px; font-weight: 700; }
        .header .subtitle { margin-top: 3px; opacity: 0.8; font-size: 11px; }
        .header .patient-info {
            margin-top: 8px;
            display: flex;
            flex-wrap: wrap;
            gap: 4px 14px;
            font-size: 10px;
            padding-top: 6px;
            border-top: 1px solid rgba(255,255,255,0.15);
        }
        .header .patient-info span {
            background: rgba(255,255,255,0.08);
            padding: 2px 10px;
            border-radius: 10px;
        }

        .section {
            margin-bottom: 12px;
            break-inside: avoid;
            page-break-inside: avoid;
        }
        .section-title {
            font-size: 15px;
            font-weight: 700;
            margin-bottom: 8px;
            color: #0d1b2a;
            padding-bottom: 4px;
            border-bottom: 2px solid #e2e8f0;
            display: flex;
            align-items: center;
            gap: 6px;
        }

        .summary-box {
            background: #eef7ff;
            border-left: 5px solid #2b6cb0;
            border-radius: 10px;
            padding: 12px 16px;
            font-size: 12px;
            line-height: 1.5;
            break-inside: avoid;
            page-break-inside: avoid;
            -webkit-print-color-adjust: exact;
            print-color-adjust: exact;
        }

        .stats-table {
            width: 100%;
            border-collapse: separate;
            border-spacing: 6px;
        }
        .stat {
            background: #f7fafc;
            border-radius: 10px;
            padding: 10px;
            text-align: center;
            border: 1px solid #e2e8f0;
            break-inside: avoid;
            page-break-inside: avoid;
        }
        .stat-number { font-size: 20px; font-weight: 700; }
        .stat-label { font-size: 10px; color: #718096; margin-top: 2px; }
        .green { color: #2f855a; }
        .yellow { color: #b7791f; }
        .red { color: #c53030; }
        .blue { color: #2b6cb0; }

        .category {
            margin-top: 10px;
            break-inside: avoid;
            page-break-inside: avoid;
        }
        .category-title {
            background: #edf2f7;
            padding: 8px 12px;
            border-radius: 8px;
            font-size: 13px;
            font-weight: 700;
            margin-bottom: 8px;
            display: flex;
            align-items: center;
            gap: 6px;
            break-inside: avoid;
            page-break-inside: avoid;
            page-break-after: avoid;
        }
        .category-title .count {
            font-size: 10px;
            font-weight: 400;
            color: #718096;
            margin-left: auto;
        }
        .category-description {
            font-size: 10px;
            color: #475569;
            background: #f8fafc;
            padding: 6px 12px;
            border-radius: 6px;
            margin-bottom: 8px;
            border-left: 3px solid #94a3b8;
            line-height: 1.5;
        }

        .analysis-table {
            width: 100%;
            border-collapse: collapse;
            font-size: 10px;
        }
        .analysis-table th {
            background: #0d1b2a;
            color: white;
            padding: 5px 8px;
            text-align: left;
            font-weight: 600;
            font-size: 9px;
            -webkit-print-color-adjust: exact;
            print-color-adjust: exact;
        }
        .analysis-table td {
            padding: 5px 8px;
            border-bottom: 1px solid #e2e8f0;
            vertical-align: middle;
        }
        .analysis-table tr {
            break-inside: avoid;
            page-break-inside: avoid;
        }
        .analysis-table tr:last-child td { border-bottom: none; }

        .status {
            padding: 2px 8px;
            border-radius: 12px;
            font-size: 8.5px;
            font-weight: 600;
            white-space: nowrap;
            display: inline-block;
        }
        .status-normal { background: #dcfce7; color: #166534; }
        .status-warning { background: #fef9c3; color: #854d0e; }
        .status-critical { background: #fee2e2; color: #991b1b; }

        .indicator-desc { font-size: 8.5px; color: #64748b; display: block; margin-top: 1px; }
        .indicator-role { font-size: 8px; color: #475569; display: block; margin-top: 1px; font-style: italic; }
        .indicator-full-desc {
            font-size: 8.5px;
            color: #475569;
            display: block;
            margin-top: 2px;
            line-height: 1.4;
            background: #f8fafc;
            padding: 3px 6px;
            border-radius: 4px;
            border-left: 2px solid #94a3b8;
        }
        .value-cell { font-weight: 600; font-size: 11px; }
        .value-cell .unit { font-weight: 400; font-size: 8.5px; color: #718096; }

        .charts-grid {
            display: table;
            width: 100%;
            border-collapse: separate;
            border-spacing: 10px;
            margin: 6px 0;
        }
        .charts-grid .chart-cell { display: table-cell; width: 50%; vertical-align: top; }
        .chart-box {
            background: #f7fafc;
            border-radius: 8px;
            padding: 10px;
            border: 1px solid #e2e8f0;
            text-align: center;
            break-inside: avoid;
            page-break-inside: avoid;
            height: 220px;
            min-height: 220px;
            max-height: 220px;
            display: flex;
            flex-direction: column;
        }
        .chart-box h4 { font-size: 10px; font-weight: 600; color: #475569; margin-bottom: 4px; flex-shrink: 0; }
        .chart-box svg { flex: 1; width: 100%; height: 100%; max-height: 160px; }
        .chart-description {
            font-size: 8px;
            color: #64748b;
            text-align: center;
            margin: 3px 0 0 0;
            padding: 3px 6px;
            background: #f1f5f9;
            border-radius: 4px;
            line-height: 1.3;
            flex-shrink: 0;
        }

        .health-potential-box {
            background: linear-gradient(135deg, #e6f7ed, #c8f0d9);
            border-radius: 10px;
            padding: 14px 18px;
            border: 2px solid #38a169;
            break-inside: avoid;
            page-break-inside: avoid;
            -webkit-print-color-adjust: exact;
            print-color-adjust: exact;
            margin-top: 4px;
        }
        .health-potential-box .pot-title {
            font-size: 14px;
            font-weight: 700;
            color: #2f855a;
            display: flex;
            align-items: center;
            gap: 6px;
            margin-bottom: 6px;
        }
        .health-potential-box .pot-subtitle { font-size: 10px; color: #475569; margin-bottom: 8px; line-height: 1.4; }
        .health-potential-box .pot-chart-container { width: 100%; height: 200px; min-height: 200px; max-height: 200px; }
        .health-potential-box .pot-chart-container svg { width: 100%; height: 100%; }
        .health-potential-box .pot-description {
            font-size: 9px;
            color: #2f855a;
            text-align: center;
            margin-top: 5px;
            padding: 5px 10px;
            background: rgba(255,255,255,0.6);
            border-radius: 5px;
            line-height: 1.4;
        }

        .list { padding: 0; margin: 0; }
        .list li {
            list-style: none;
            padding: 6px 12px;
            margin-bottom: 4px;
            border-radius: 8px;
            font-size: 10.5px;
            line-height: 1.4;
            break-inside: avoid;
            page-break-inside: avoid;
        }
        .list-attention li { background: #fef2f2; border-left: 4px solid #e53e3e; color: #7f1d1d; }
        .list-recommend li {
            background: #f0fdf4;
            border-left: 4px solid #38a169;
            color: #14532d;
            counter-increment: rec;
        }
        .list-recommend { counter-reset: rec; padding-left: 0; }
        .list-recommend li::before { content: counter(rec) ". "; font-weight: 700; color: #38a169; }

        .disclaimer {
            margin-top: 10px;
            padding: 8px 14px;
            background: #fefce8;
            border-radius: 8px;
            border-left: 4px solid #d69e2e;
            font-size: 9.5px;
            color: #713f12;
            line-height: 1.4;
            break-inside: avoid;
            page-break-inside: avoid;
            -webkit-print-color-adjust: exact;
            print-color-adjust: exact;
        }

        .footer { display: none; }
        .your-value {
            display: inline-block;
            background: #dbeafe;
            padding: 0 4px;
            border-radius: 3px;
            font-weight: 700;
            color: #1e40af;
        }

        .final-recommendations {
            background: #f8fafc;
            border-radius: 10px;
            padding: 12px 16px;
            border: 1px solid #e2e8f0;
            margin-top: 8px;
            break-inside: avoid;
            page-break-inside: avoid;
        }
        .final-recommendations .rec-item {
            padding: 6px 0;
            border-bottom: 1px solid #e2e8f0;
            font-size: 10px;
            line-height: 1.5;
        }
        .final-recommendations .rec-item:last-child { border-bottom: none; }
        .final-recommendations .rec-item .badge {
            display: inline-block;
            padding: 1px 8px;
            border-radius: 10px;
            font-size: 8px;
            font-weight: 600;
            margin-right: 4px;
        }
        .badge-critical { background: #fee2e2; color: #991b1b; }
        .badge-warning { background: #fef9c3; color: #854d0e; }
        .badge-normal { background: #dcfce7; color: #166534; }

        @media print {
            html, body { background: #e8edf2; }
            .page { background: #f7f9fb; }
            .section, .category, .stat, .summary-box, .list li, .chart-box, .health-potential-box, .final-recommendations {
                break-inside: avoid;
                page-break-inside: avoid;
            }
            .footer { display: none !important; }
        }
    </style>
</head>
<body>
    <div class="container">

        <!-- PAGE 1 -->
        <div class="page">
            <div class="header">
                <h1>AnalyzPRO</h1>
                <div class="subtitle">Персональный анализ лабораторных показателей</div>
                <div class="patient-info">
                    <span>Пациент: {{.Profile.Name}}</span>
                    {{if .Profile.Age}}<span>Возраст: {{.Profile.Age}} лет</span>{{end}}
                    {{if .Profile.Gender}}<span>Пол: {{.Profile.Gender}}</span>{{end}}
                    <span>Дата: {{.Profile.Date}}</span>
                </div>
            </div>

            <div class="section">
                <div class="section-title">Общий вывод</div>
                <div class="summary-box">{{.Summary}}</div>
            </div>

            <div class="section">
                <div class="section-title">Общая статистика</div>
                <table class="stats-table">
                    <tr>
                        {{$total := 0}}{{$normal := 0}}{{$warning := 0}}{{$critical := 0}}
                        {{range .Categories}}{{range .Indicators}}{{$total = add $total 1}}{{if eq .Status "normal"}}{{$normal = add $normal 1}}{{end}}{{if eq .Status "warning"}}{{$warning = add $warning 1}}{{end}}{{if eq .Status "critical"}}{{$critical = add $critical 1}}{{end}}{{end}}{{end}}
                        <td class="stat"><div class="stat-number blue">{{$total}}</div><div class="stat-label">Всего показателей</div></td>
                        <td class="stat"><div class="stat-number green">{{$normal}}</div><div class="stat-label">В норме</div></td>
                        <td class="stat"><div class="stat-number yellow">{{$warning}}</div><div class="stat-label">Требует внимания</div></td>
                        <td class="stat"><div class="stat-number red">{{$critical}}</div><div class="stat-label">Отклонения</div></td>
                    </tr>
                </table>
            </div>

            <!-- CHARTS -->
            <div class="section">
                <div class="section-title">Визуализация анализов</div>
                <div class="charts-grid">
                    <div class="chart-cell">
                        <div class="chart-box">
                            <h4>Статус показателей</h4>
                            <svg viewBox="0 0 300 160" xmlns="http://www.w3.org/2000/svg">
                                {{$total := 0}}{{$normal := 0}}{{$warning := 0}}{{$critical := 0}}
                                {{range .Categories}}{{range .Indicators}}{{$total = add $total 1}}{{if eq .Status "normal"}}{{$normal = add $normal 1}}{{end}}{{if eq .Status "warning"}}{{$warning = add $warning 1}}{{end}}{{if eq .Status "critical"}}{{$critical = add $critical 1}}{{end}}{{end}}{{end}}
                                {{$normalPct := 0}}{{$warningPct := 0}}{{$criticalPct := 0}}
                                {{if gt $total 0}}{{$normalPct = div (mul $normal 100) $total}}{{end}}
                                {{if gt $total 0}}{{$warningPct = div (mul $warning 100) $total}}{{end}}
                                {{if gt $total 0}}{{$criticalPct = div (mul $critical 100) $total}}{{end}}
                                {{$normalDash := div (mul $normalPct 264) 100}}
                                {{$warningDash := div (mul $warningPct 264) 100}}
                                {{$criticalDash := div (mul $criticalPct 264) 100}}
                                {{$warningOffset := sub 0 $normalDash}}
                                {{$criticalOffset := sub 0 (add $normalDash $warningDash)}}
                                <circle cx="95" cy="80" r="42" fill="none" stroke="#edf2f7" stroke-width="18"/>
                                <circle cx="95" cy="80" r="42" fill="none" stroke="#b7797b" stroke-width="18" stroke-dasharray="{{$criticalDash}} 264" stroke-dashoffset="0" stroke-linecap="round"/>
                                <circle cx="95" cy="80" r="42" fill="none" stroke="#d4a05a" stroke-width="18" stroke-dasharray="{{$warningDash}} 264" stroke-dashoffset="{{$warningOffset}}" stroke-linecap="round"/>
                                <circle cx="95" cy="80" r="42" fill="none" stroke="#6b9e7a" stroke-width="18" stroke-dasharray="{{$normalDash}} 264" stroke-dashoffset="{{$criticalOffset}}" stroke-linecap="round"/>
                                <circle cx="95" cy="80" r="30" fill="white"/>
                                {{$score := 0}}{{if gt $total 0}}{{$score = div (mul $normal 100) $total}}{{end}}
                                <text x="95" y="77" text-anchor="middle" font-size="16" font-weight="700" fill="#2d3748">{{$score}}%</text>
                                <text x="95" y="93" text-anchor="middle" font-size="8" fill="#718096">здоровья</text>
                                <rect x="185" y="50" width="10" height="10" rx="2" fill="#6b9e7a"/>
                                <text x="199" y="58" font-size="7.5" fill="#475569">В норме ({{$normal}})</text>
                                <rect x="185" y="68" width="10" height="10" rx="2" fill="#d4a05a"/>
                                <text x="199" y="76" font-size="7.5" fill="#475569">Внимание ({{$warning}})</text>
                                <rect x="185" y="86" width="10" height="10" rx="2" fill="#b7797b"/>
                                <text x="199" y="94" font-size="7.5" fill="#475569">Отклонения ({{$critical}})</text>
                            </svg>
                            <div class="chart-description">Распределение по статусам</div>
                        </div>
                    </div>
                    <div class="chart-cell">
                        <div class="chart-box">
                            <h4>Показатели по категориям</h4>
                            <svg viewBox="0 0 300 160" xmlns="http://www.w3.org/2000/svg">
                                <line x1="18" y1="143" x2="290" y2="143" stroke="#e2e8f0" stroke-width="0.5"/>
                                <line x1="18" y1="110" x2="290" y2="110" stroke="#e2e8f0" stroke-width="0.5" stroke-dasharray="3,3"/>
                                <line x1="18" y1="77" x2="290" y2="77" stroke="#e2e8f0" stroke-width="0.5" stroke-dasharray="3,3"/>
                                <line x1="18" y1="44" x2="290" y2="44" stroke="#e2e8f0" stroke-width="0.5" stroke-dasharray="3,3"/>
                                <text x="14" y="146" text-anchor="end" font-size="6" fill="#94a3b8">0</text>
                                <text x="14" y="113" text-anchor="end" font-size="6" fill="#94a3b8">3</text>
                                <text x="14" y="80" text-anchor="end" font-size="6" fill="#94a3b8">6</text>
                                <text x="14" y="47" text-anchor="end" font-size="6" fill="#94a3b8">9</text>
                                {{$max := 1}}
                                {{range .Categories}}{{if gt (len .Indicators) $max}}{{$max = len .Indicators}}{{end}}{{end}}
                                {{$scale := 0}}{{if gt $max 0}}{{$scale = div 92 $max}}{{end}}
                                {{$x := 26}}
                                {{$colors := list "#5b7c9e" "#5b8c8c" "#b8965a" "#8b7a9e" "#b7797b" "#6b9e7a" "#8a9b9e" "#a8b5b8"}}
                                {{$i := 0}}
                                {{range .Categories}}
                                    {{$value := len .Indicators}}
                                    {{$height := mul $value $scale}}
                                    {{$y := sub 143 $height}}
                                    {{$color := index $colors $i}}
                                    <rect x="{{$x}}" y="{{$y}}" width="28" height="{{$height}}" fill="{{$color}}" rx="3"/>
                                    <text x="{{add $x 14}}" y="{{sub $y 4}}" text-anchor="middle" font-size="7" fill="{{$color}}" font-weight="700">{{$value}}</text>
                                    {{$i = add $i 1}}
                                    {{$x = add $x 36}}
                                {{end}}
                                {{$x = 26}}
                                {{range .Categories}}
                                    {{$name := .Name}}
                                    {{if gt (len $name) 8}}{{$name = printf "%s..." (substr $name 0 6)}}{{end}}
                                    <text x="{{add $x 14}}" y="156" text-anchor="middle" font-size="6" fill="#718096">{{$name}}</text>
                                    {{$x = add $x 36}}
                                {{end}}
                            </svg>
                            <div class="chart-description">Количество в каждой категории</div>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- CATEGORIES PAGES -->
        {{range .Categories}}
        <div class="page">
            <div class="header">
                <h1>AnalyzPRO</h1>
                <div class="subtitle">Персональный анализ лабораторных показателей</div>
                <div class="patient-info">
                    <span>Пациент: {{$.Profile.Name}}</span>
                    {{if $.Profile.Age}}<span>Возраст: {{$.Profile.Age}} лет</span>{{end}}
                    {{if $.Profile.Gender}}<span>Пол: {{$.Profile.Gender}}</span>{{end}}
                    <span>Дата: {{$.Profile.Date}}</span>
                </div>
            </div>

            <div class="section">
                <div class="section-title">Детальный разбор анализов</div>
                <div class="category">
                    <div class="category-title">
                        {{categoryIcon .Name}} {{.Name}}
                        <span class="count">{{len .Indicators}} показателей</span>
                    </div>
                    <div class="category-description">
                        <strong>Что показывает:</strong> {{.Description}}
                        <br><br>
                        <strong>Ваши результаты:</strong>
                        {{$normalCount := 0}}{{$warningCount := 0}}{{$criticalCount := 0}}
                        {{range .Indicators}}{{if eq .Status "normal"}}{{$normalCount = add $normalCount 1}}{{end}}{{if eq .Status "warning"}}{{$warningCount = add $warningCount 1}}{{end}}{{if eq .Status "critical"}}{{$criticalCount = add $criticalCount 1}}{{end}}{{end}}
                        {{$totalCount := len .Indicators}}
                        {{$normalCount}} в норме
                        {{if gt $warningCount 0}}, {{$warningCount}} требуют внимания{{end}}
                        {{if gt $criticalCount 0}}, {{$criticalCount}} критических отклонений{{end}}.
                    </div>
                    <table class="analysis-table">
                        <thead>
                            <tr><th>Показатель</th><th>Функция</th><th>Ваше значение</th><th>Норма</th><th>Статус</th></tr>
                        </thead>
                        <tbody>
                            {{range .Indicators}}
                            <tr>
                                <td>
                                    <strong>{{.Name}}</strong>
                                    <span class="indicator-role">{{.Role}}</span>
                                    <span class="indicator-desc">{{.ShortDesc}}</span>
                                    <span class="indicator-full-desc">{{.FullDesc}}</span>
                                </td>
                                <td style="font-size:8.5px;">{{.Function}}</td>
                                <td class="value-cell"><span class="your-value">{{.Value}}</span> <span class="unit">{{.Unit}}</span></td>
                                <td>{{.Normal}}</td>
                                <td><span class="status {{statusClass .Status}}">{{statusIcon .Status}} {{statusText .Status}}</span></td>
                            </tr>
                            {{end}}
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
        {{end}}

        <!-- HEALTH POTENTIAL PAGE -->
        <div class="page">
            <div class="header">
                <h1>AnalyzPRO</h1>
                <div class="subtitle">Персональный анализ лабораторных показателей</div>
                <div class="patient-info">
                    <span>Пациент: {{.Profile.Name}}</span>
                    {{if .Profile.Age}}<span>Возраст: {{.Profile.Age}} лет</span>{{end}}
                    {{if .Profile.Gender}}<span>Пол: {{.Profile.Gender}}</span>{{end}}
                    <span>Дата: {{.Profile.Date}}</span>
                </div>
            </div>

            <div class="section">
                <div class="section-title">Потенциал улучшения здоровья</div>
                <div class="health-potential-box">
                    <div class="pot-title">
                        Индекс здоровья —
                        {{$total := 0}}{{$normal := 0}}
                        {{range .Categories}}{{range .Indicators}}{{$total = add $total 1}}{{if eq .Status "normal"}}{{$normal = add $normal 1}}{{end}}{{end}}{{end}}
                        {{$score := 0}}{{if gt $total 0}}{{$score = div (mul $normal 100) $total}}{{end}}
                        {{$score}}%
                        <span style="font-size:10px; font-weight:400; color:#475569; margin-left:auto;">Целевой показатель: 95%</span>
                    </div>
                    <div class="pot-subtitle">
                        <strong>На основе ваших анализов:</strong> Общий индекс здоровья составляет <strong>{{$score}}%</strong>.
                        При коррекции выявленных отклонений вы можете достичь <strong>95%</strong>.
                    </div>
                    <div class="pot-chart-container">
                        <svg viewBox="0 0 500 200" xmlns="http://www.w3.org/2000/svg">
                            <defs>
                                <linearGradient id="grad1" x1="0%" y1="0%" x2="0%" y2="100%">
                                    <stop offset="0%" style="stop-color:#4a7c9e;stop-opacity:1"/>
                                    <stop offset="100%" style="stop-color:#6b9ebd;stop-opacity:0.7"/>
                                </linearGradient>
                                <linearGradient id="grad2" x1="0%" y1="0%" x2="0%" y2="100%">
                                    <stop offset="0%" style="stop-color:#5b8c8c;stop-opacity:1"/>
                                    <stop offset="100%" style="stop-color:#7ba8a8;stop-opacity:0.7"/>
                                </linearGradient>
                                <linearGradient id="grad3" x1="0%" y1="0%" x2="0%" y2="100%">
                                    <stop offset="0%" style="stop-color:#b8965a;stop-opacity:1"/>
                                    <stop offset="100%" style="stop-color:#d4b07a;stop-opacity:0.7"/>
                                </linearGradient>
                                <linearGradient id="grad4" x1="0%" y1="0%" x2="0%" y2="100%">
                                    <stop offset="0%" style="stop-color:#8b7a9e;stop-opacity:1"/>
                                    <stop offset="100%" style="stop-color:#ab9abe;stop-opacity:0.7"/>
                                </linearGradient>
                                <linearGradient id="grad5" x1="0%" y1="0%" x2="0%" y2="100%">
                                    <stop offset="0%" style="stop-color:#b7797b;stop-opacity:1"/>
                                    <stop offset="100%" style="stop-color:#d5999b;stop-opacity:0.7"/>
                                </linearGradient>
                                <linearGradient id="grad6" x1="0%" y1="0%" x2="0%" y2="100%">
                                    <stop offset="0%" style="stop-color:#6b9e7a;stop-opacity:1"/>
                                    <stop offset="100%" style="stop-color:#8bbe9a;stop-opacity:0.7"/>
                                </linearGradient>
                                <linearGradient id="grad7" x1="0%" y1="0%" x2="0%" y2="100%">
                                    <stop offset="0%" style="stop-color:#8a9b9e;stop-opacity:1"/>
                                    <stop offset="100%" style="stop-color:#aabbbd;stop-opacity:0.7"/>
                                </linearGradient>
                            </defs>

                            <polygon points="60,170 120,170 120,30 60,30" fill="none" stroke="#e2e8f0" stroke-width="0.5"/>
                            <polygon points="120,170 180,170 180,30 120,30" fill="none" stroke="#e2e8f0" stroke-width="0.5"/>
                            <polygon points="180,170 240,170 240,30 180,30" fill="none" stroke="#e2e8f0" stroke-width="0.5"/>
                            <polygon points="240,170 300,170 300,30 240,30" fill="none" stroke="#e2e8f0" stroke-width="0.5"/>
                            <polygon points="300,170 360,170 360,30 300,30" fill="none" stroke="#e2e8f0" stroke-width="0.5"/>
                            <polygon points="360,170 420,170 420,30 360,30" fill="none" stroke="#e2e8f0" stroke-width="0.5"/>
                            <polygon points="420,170 480,170 480,30 420,30" fill="none" stroke="#e2e8f0" stroke-width="0.5"/>

                            <line x1="60" y1="170" x2="480" y2="170" stroke="#e2e8f0" stroke-width="0.5"/>
                            <line x1="60" y1="135" x2="480" y2="135" stroke="#e2e8f0" stroke-width="0.5" stroke-dasharray="4,4"/>
                            <line x1="60" y1="100" x2="480" y2="100" stroke="#e2e8f0" stroke-width="0.5" stroke-dasharray="4,4"/>
                            <line x1="60" y1="65" x2="480" y2="65" stroke="#e2e8f0" stroke-width="0.5" stroke-dasharray="4,4"/>
                            <line x1="60" y1="30" x2="480" y2="30" stroke="#e2e8f0" stroke-width="0.5"/>

                            <text x="45" y="173" text-anchor="end" font-size="7" fill="#94a3b8">0</text>
                            <text x="45" y="138" text-anchor="end" font-size="7" fill="#94a3b8">25</text>
                            <text x="45" y="103" text-anchor="end" font-size="7" fill="#94a3b8">50</text>
                            <text x="45" y="68" text-anchor="end" font-size="7" fill="#94a3b8">75</text>
                            <text x="45" y="33" text-anchor="end" font-size="7" fill="#94a3b8">100</text>

                            {{$grads := list "grad1" "grad2" "grad3" "grad4" "grad5" "grad6" "grad7"}}
                            {{$x := 70}}
                            {{$i := 0}}
                            {{range .Categories}}
                                {{$totalCat := len .Indicators}}
                                {{$normalCat := 0}}
                                {{range .Indicators}}{{if eq .Status "normal"}}{{$normalCat = add $normalCat 1}}{{end}}{{end}}
                                {{$pct := 0}}{{if gt $totalCat 0}}{{$pct = div (mul $normalCat 100) $totalCat}}{{end}}
                                {{$height := div (mul $pct 140) 100}}
                                {{$y := sub 170 $height}}
                                {{$grad := index $grads $i}}
                                <rect x="{{$x}}" y="{{$y}}" width="32" height="{{$height}}" fill="url(#{{$grad}})" rx="4"/>
                                <text x="{{add $x 16}}" y="{{sub $y 4}}" text-anchor="middle" font-size="7" fill="#475569" font-weight="700">{{$pct}}%</text>
                                {{$i = add $i 1}}
                                {{$x = add $x 60}}
                            {{end}}

                            {{$x = 86}}
                            {{range .Categories}}
                                {{$name := .Name}}
                                {{if gt (len $name) 6}}{{$name = printf "%s" (substr $name 0 4)}}{{end}}
                                <text x="{{$x}}" y="185" text-anchor="middle" font-size="6" fill="#475569">{{$name}}</text>
                                {{$x = add $x 60}}
                            {{end}}

                            <text x="250" y="18" text-anchor="middle" font-size="9" fill="#2f855a" font-weight="600">Текущее состояние здоровья по системам</text>
                        </svg>
                    </div>

                    <div class="pot-description">
                        {{range .Categories}}
                            {{$totalCat := len .Indicators}}
                            {{$normalCat := 0}}
                            {{range .Indicators}}{{if eq .Status "normal"}}{{$normalCat = add $normalCat 1}}{{end}}{{end}}
                            {{$pct := 0}}{{if gt $totalCat 0}}{{$pct = div (mul $normalCat 100) $totalCat}}{{end}}
                            {{$name := .Name}}
                            {{if lt $pct 60}}
                                <strong>Требует внимания:</strong> {{$name}} ({{$pct}}%)
                            {{else if lt $pct 80}}
                                <strong>В зоне риска:</strong> {{$name}} ({{$pct}}%)
                            {{else}}
                                <strong>В норме:</strong> {{$name}} ({{$pct}}%)
                            {{end}}
                        {{end}}
                        <br><br>
                        <strong>Ключевые шаги для улучшения:</strong>
                        Следуйте индивидуальным рекомендациям, корректируйте питание, повышайте физическую активность и регулярно проходите профилактические осмотры.
                    </div>
                </div>
            </div>
        </div>

        <!-- FINAL RECOMMENDATIONS PAGE -->
        <div class="page">
            <div class="header" style="margin-bottom:10px; padding:14px 20px;">
                <div style="display:flex; justify-content:space-between; align-items:flex-start;">
                    <div>
                        <h1 style="font-size:18px;">AnalyzPRO</h1>
                        <div class="subtitle" style="font-size:9px;">Итоговые рекомендации</div>
                    </div>
                    <div style="text-align:right; font-size:8px; opacity:0.8;">
                        <div>{{.Profile.Date}}</div>
                    </div>
                </div>
                <div class="patient-info" style="font-size:8.5px; padding-top:4px; margin-top:4px;">
                    <span>Пациент: {{.Profile.Name}}{{if .Profile.Age}}, {{.Profile.Age}} лет{{end}}</span>
                    {{$total := 0}}{{range .Categories}}{{range .Indicators}}{{$total = add $total 1}}{{end}}{{end}}
                    <span>Всего показателей: {{$total}}</span>
                </div>
            </div>

            {{if .Attention}}
            <div class="section">
                <div class="section-title">На что обратить внимание</div>
                <ul class="list list-attention">
                    {{range .Attention}}<li>{{.}}</li>{{end}}
                </ul>
            </div>
            {{end}}

            {{if .Recommendations}}
            <div class="section">
                <div class="section-title">Рекомендации</div>
                <ul class="list list-recommend">
                    {{range .Recommendations}}<li>{{.}}</li>{{end}}
                </ul>
            </div>
            {{end}}

            <div class="disclaimer" style="margin-top:8px;">
                <strong>Важно:</strong> {{.Disclaimer}}
            </div>
        </div>

    </div>
</body>
</html>`

// const analysisHTMLTemplate = `<!DOCTYPE html>
// <html lang="ru">
// <head>
//     <meta charset="UTF-8">
//     <meta name="viewport" content="width=device-width, initial-scale=1.0">
//     <title>Анализ здоровья - AnalyzPRO</title>
//     <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
//     <style>
//         * { margin: 0; padding: 0; box-sizing: border-box; }
//         body {
//             font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
//             background: #f0f4f8;
//             padding: 20px;
//             color: #1a202c;
//         }
//         .container {
//             max-width: 1200px;
//             margin: 0 auto;
//             background: #ffffff;
//             border-radius: 28px;
//             box-shadow: 0 20px 60px rgba(0,0,0,0.12);
//             overflow: hidden;
//         }
//         .header {
//             background: linear-gradient(135deg, #0d1b2a 0%, #1b3a5c 40%, #2d6a9f 100%);
//             color: white;
//             padding: 30px 40px;
//             position: relative;
//         }
//         .header::after {
//             content: '';
//             position: absolute;
//             bottom: 0;
//             left: 0;
//             right: 0;
//             height: 4px;
//             background: linear-gradient(90deg, #48bb78, #f6e05e, #fc8181, #9f7aea);
//         }
//         .header h1 { font-size: 28px; font-weight: 800; }
//         .header .subtitle { font-size: 14px; opacity: 0.85; font-weight: 300; margin-top: 4px; }
//         .header .patient-info {
//             margin-top: 12px;
//             display: flex;
//             flex-wrap: wrap;
//             gap: 10px 20px;
//             font-size: 13px;
//         }
//         .header .patient-info span {
//             background: rgba(255,255,255,0.1);
//             padding: 4px 16px;
//             border-radius: 20px;
//             backdrop-filter: blur(4px);
//             border: 1px solid rgba(255,255,255,0.08);
//         }
//         .content { padding: 30px 40px; }
//         .section { margin-bottom: 35px; }
//         .section-title {
//             font-size: 20px;
//             font-weight: 700;
//             color: #0d1b2a;
//             margin-bottom: 16px;
//             padding-bottom: 10px;
//             border-bottom: 3px solid #e2e8f0;
//             display: flex;
//             align-items: center;
//             gap: 10px;
//         }
//         .section-title .icon { font-size: 24px; }

//         .stats-row {
//             display: grid;
//             grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
//             gap: 12px;
//             margin: 12px 0;
//         }
//         .stat-card {
//             background: #f7fafc;
//             border-radius: 12px;
//             padding: 14px 16px;
//             text-align: center;
//             border: 1px solid #e2e8f0;
//         }
//         .stat-card .number { font-size: 28px; font-weight: 700; }
//         .stat-card .label { font-size: 12px; color: #718096; margin-top: 4px; }
//         .stat-card .number.green { color: #38a169; }
//         .stat-card .number.yellow { color: #d69e2e; }
//         .stat-card .number.red { color: #e53e3e; }
//         .stat-card .number.blue { color: #3182ce; }

//         .charts-row {
//             display: flex;
//             flex-wrap: wrap;
//             gap: 20px;
//             margin: 16px 0;
//         }
//         .chart-box {
//             background: #f7fafc;
//             border-radius: 14px;
//             padding: 16px;
//             text-align: center;
//             border: 1px solid #e2e8f0;
//             min-height: 250px;
//             display: flex;
//             flex-direction: column;
//             flex: 1 1 calc(50% - 10px);
//             max-width: calc(50% - 10px);
//         }
//         .chart-box h4 { font-size: 13px; font-weight: 600; color: #4a5568; margin-bottom: 10px; }
//         .chart-container {
//             position: relative;
//             width: 100%;
//             height: 180px;
//             min-height: 180px;
//             max-height: 180px;
//         }

//         .card-grid {
//             display: block;
//         }
//         .card {
//             background: #f7fafc;
//             border-radius: 14px;
//             padding: 18px 20px;
//             border-left: 5px solid #667eea;
//             transition: transform 0.2s, box-shadow 0.2s;
//             margin-bottom: 12px;
//             page-break-inside: avoid;
//             page-break-after: avoid;
//             break-inside: avoid;
//             break-after: avoid;
//             overflow: visible;
//         }
//         .card:hover {
//             transform: translateY(-2px);
//             box-shadow: 0 8px 25px rgba(0,0,0,0.06);
//         }
//         .card .card-header {
//             display: flex;
//             justify-content: space-between;
//             align-items: flex-start;
//             flex-wrap: wrap;
//             gap: 8px;
//         }
//         .card .name { font-weight: 700; font-size: 15px; color: #1a202c; }
//         .card .value-row {
//             display: flex;
//             align-items: baseline;
//             gap: 6px;
//             margin: 4px 0;
//             flex-wrap: wrap;
//         }
//         .card .value { font-size: 24px; font-weight: 700; color: #0d1b2a; }
//         .card .unit { font-size: 13px; font-weight: 400; color: #718096; }
//         .card .normal {
//             font-size: 12px;
//             color: #718096;
//             background: #edf2f7;
//             padding: 2px 10px;
//             border-radius: 10px;
//         }
//         .card .desc { font-size: 13px; color: #4a5568; margin-top: 4px; line-height: 1.5; }
//         .card .explanation {
//             font-size: 13px;
//             color: #2d3748;
//             margin-top: 8px;
//             padding: 10px 14px;
//             background: white;
//             border-radius: 10px;
//             border: 1px solid #e2e8f0;
//             line-height: 1.6;
//         }
//         .card .status-badge {
//             display: inline-block;
//             padding: 2px 14px;
//             border-radius: 20px;
//             font-size: 12px;
//             font-weight: 600;
//             white-space: nowrap;
//         }
//         .status-normal { background: #f0fff4; color: #38a169; }
//         .status-warning { background: #fffff0; color: #d69e2e; }
//         .status-critical { background: #fff5f5; color: #e53e3e; }

//         .category-header {
//             font-size: 16px;
//             font-weight: 600;
//             color: #1a202c;
//             margin-bottom: 14px;
//             padding: 12px 18px;
//             background: #edf2f7;
//             border-radius: 10px;
//             display: flex;
//             align-items: center;
//             gap: 10px;
//             page-break-inside: avoid;
//             page-break-after: avoid;
//             break-inside: avoid;
//             break-after: avoid;
//         }
//         .category-header .count {
//             font-size: 12px;
//             font-weight: 400;
//             color: #718096;
//             margin-left: auto;
//         }

//         .category-wrapper {
//             page-break-inside: avoid;
//             break-inside: avoid;
//             margin-bottom: 28px;
//         }

//         .summary-box {
//             background: linear-gradient(135deg, #ebf8ff, #bee3f8);
//             border-radius: 14px;
//             padding: 20px 24px;
//             border-left: 6px solid #2b6cb0;
//             page-break-inside: avoid;
//             break-inside: avoid;
//         }
//         .summary-box p { font-size: 15px; line-height: 1.7; color: #2c5282; }

//         .attentions, .recommendations { padding-left: 0; }
//         .attentions li {
//             list-style: none;
//             padding: 12px 18px;
//             background: #fff5f5;
//             border-radius: 10px;
//             border-left: 4px solid #fc8181;
//             color: #742a2a;
//             margin-bottom: 8px;
//             font-size: 14px;
//             line-height: 1.5;
//             page-break-inside: avoid;
//             break-inside: avoid;
//         }
//         .recommendations li {
//             list-style: none;
//             padding: 12px 18px;
//             background: #f0fff4;
//             border-radius: 10px;
//             border-left: 4px solid #48bb78;
//             color: #22543d;
//             margin-bottom: 8px;
//             font-size: 14px;
//             line-height: 1.5;
//             counter-increment: rec;
//             page-break-inside: avoid;
//             break-inside: avoid;
//         }
//         .recommendations li::before {
//             content: counter(rec) ". ";
//             font-weight: 700;
//             color: #38a169;
//         }
//         .recommendations { counter-reset: rec; padding-left: 0; }

//         .disclaimer {
//             margin-top: 25px;
//             padding: 16px 24px;
//             background: #fefcbf;
//             border-radius: 12px;
//             border-left: 6px solid #d69e2e;
//             font-size: 13px;
//             color: #744210;
//             line-height: 1.6;
//             page-break-inside: avoid;
//             break-inside: avoid;
//         }
//         .disclaimer strong { color: #975a16; }
//         .footer {
//             text-align: center;
//             padding: 20px;
//             color: #a0aec0;
//             font-size: 12px;
//             border-top: 1px solid #e2e8f0;
//         }

//         @media (max-width: 768px) {
//             .header, .content { padding: 16px 20px; }
//             .header h1 { font-size: 22px; }
//             .charts-row { flex-direction: column; }
//             .chart-box { flex: 1 1 100%; max-width: 100%; }
//             .stats-row { grid-template-columns: 1fr 1fr; }
//         }

//         @page {
//             size: A4;
//             margin: 10mm 10mm 10mm 10mm;
//         }

//         @media print {
//             body {
//                 background: white !important;
//                 padding: 0 !important;
//                 -webkit-print-color-adjust: exact !important;
//                 print-color-adjust: exact !important;
//                 color-adjust: exact !important;
//             }

//             .container {
//                 max-width: 100% !important;
//                 border-radius: 0 !important;
//                 box-shadow: none !important;
//                 background: white !important;
//             }

//             .header {
//                 -webkit-print-color-adjust: exact !important;
//                 print-color-adjust: exact !important;
//                 page-break-inside: avoid !important;
//                 break-inside: avoid !important;
//                 padding: 20px 30px !important;
//             }

//             .header::after {
//                 -webkit-print-color-adjust: exact !important;
//                 print-color-adjust: exact !important;
//             }

//             .content {
//                 padding: 20px 30px !important;
//             }

//             .card {
//                 page-break-inside: avoid !important;
//                 break-inside: avoid !important;
//                 page-break-after: avoid !important;
//                 break-after: avoid !important;
//                 margin-bottom: 12px !important;
//                 background: #f7fafc !important;
//                 -webkit-print-color-adjust: exact !important;
//                 print-color-adjust: exact !important;
//                 border: 1px solid #e2e8f0 !important;
//             }

//             .card .status-badge {
//                 -webkit-print-color-adjust: exact !important;
//                 print-color-adjust: exact !important;
//             }

//             .chart-box {
//                 page-break-inside: avoid !important;
//                 break-inside: avoid !important;
//                 page-break-after: avoid !important;
//                 break-after: avoid !important;
//                 min-height: 200px !important;
//                 background: #f7fafc !important;
//                 -webkit-print-color-adjust: exact !important;
//                 print-color-adjust: exact !important;
//                 padding: 12px !important;
//                 flex: 1 1 calc(50% - 10px) !important;
//                 max-width: calc(50% - 10px) !important;
//                 border: 1px solid #e2e8f0 !important;
//             }

//             .chart-container {
//                 height: 150px !important;
//                 min-height: 150px !important;
//                 max-height: 150px !important;
//                 position: relative !important;
//                 width: 100% !important;
//             }

//             .chart-container canvas {
//                 width: 100% !important;
//                 height: 100% !important;
//                 display: block !important;
//                 max-height: 150px !important;
//             }

//             .charts-row {
//                 display: flex !important;
//                 flex-wrap: wrap !important;
//                 gap: 12px !important;
//                 page-break-inside: avoid !important;
//                 break-inside: avoid !important;
//             }

//             .charts-row .chart-box h4 {
//                 font-size: 11px !important;
//                 margin-bottom: 6px !important;
//             }

//             .stats-row {
//                 display: flex !important;
//                 flex-wrap: wrap !important;
//                 gap: 8px !important;
//                 page-break-inside: avoid !important;
//                 break-inside: avoid !important;
//             }

//             .stats-row .stat-card {
//                 flex: 1 0 calc(25% - 8px) !important;
//                 min-width: 100px !important;
//                 page-break-inside: avoid !important;
//                 break-inside: avoid !important;
//                 -webkit-print-color-adjust: exact !important;
//                 print-color-adjust: exact !important;
//                 padding: 10px 12px !important;
//                 border: 1px solid #e2e8f0 !important;
//             }

//             .stats-row .stat-card .number {
//                 font-size: 22px !important;
//             }

//             .category-wrapper {
//                 page-break-inside: avoid !important;
//                 break-inside: avoid !important;
//                 margin-bottom: 20px !important;
//             }

//             .category-header {
//                 page-break-inside: avoid !important;
//                 break-inside: avoid !important;
//                 page-break-after: avoid !important;
//                 break-after: avoid !important;
//                 -webkit-print-color-adjust: exact !important;
//                 print-color-adjust: exact !important;
//             }

//             .summary-box {
//                 -webkit-print-color-adjust: exact !important;
//                 print-color-adjust: exact !important;
//                 page-break-inside: avoid !important;
//                 break-inside: avoid !important;
//             }

//             .attentions li {
//                 -webkit-print-color-adjust: exact !important;
//                 print-color-adjust: exact !important;
//                 page-break-inside: avoid !important;
//                 break-inside: avoid !important;
//             }

//             .recommendations li {
//                 -webkit-print-color-adjust: exact !important;
//                 print-color-adjust: exact !important;
//                 page-break-inside: avoid !important;
//                 break-inside: avoid !important;
//             }

//             .disclaimer {
//                 -webkit-print-color-adjust: exact !important;
//                 print-color-adjust: exact !important;
//                 page-break-inside: avoid !important;
//                 break-inside: avoid !important;
//             }

//             .card:hover {
//                 transform: none !important;
//                 box-shadow: none !important;
//             }

//             * {
//                 animation: none !important;
//                 transition: none !important;
//             }

//             /* Принудительно запрещаем разрыв карточек */
//             .card-grid > .card {
//                 display: block !important;
//                 width: 100% !important;
//                 max-width: 100% !important;
//             }
//         }
//     </style>
// </head>
// <body>
//     <div class="container">
//         <div class="header">
//             <h1>🔬 AnalyzPRO</h1>
//             <div class="subtitle">Полная расшифровка медицинских анализов</div>
//             <div class="patient-info">
//                 <span>👤 {{.Profile.Name}}</span>
//                 {{if .Profile.Age}}<span>📅 {{.Profile.Age}} лет</span>{{end}}
//                 {{if .Profile.Gender}}<span>⚥ {{.Profile.Gender}}</span>{{end}}
//                 <span>📆 {{.Profile.Date}}</span>
//             </div>
//         </div>

//         <div class="content">
//             {{if .Categories}}
//             <div class="section">
//                 <div class="section-title"><span class="icon">📊</span> Общая статистика</div>
//                 <div class="stats-row">
//                     {{$total := 0}}{{$normal := 0}}{{$warning := 0}}{{$critical := 0}}
//                     {{range .Categories}}{{range .Indicators}}{{$total = add $total 1}}{{if eq .Status "normal"}}{{$normal = add $normal 1}}{{end}}{{if eq .Status "warning"}}{{$warning = add $warning 1}}{{end}}{{if eq .Status "critical"}}{{$critical = add $critical 1}}{{end}}{{end}}{{end}}
//                     <div class="stat-card"><div class="number blue">{{$total}}</div><div class="label">Всего показателей</div></div>
//                     <div class="stat-card"><div class="number green">{{$normal}}</div><div class="label">✅ В норме</div></div>
//                     <div class="stat-card"><div class="number yellow">{{$warning}}</div><div class="label">⚠️ Требует внимания</div></div>
//                     <div class="stat-card"><div class="number red">{{$critical}}</div><div class="label">❌ Отклонения</div></div>
//                 </div>
//             </div>

//             <div class="section">
//                 <div class="section-title"><span class="icon">📈</span> Визуализация</div>
//                 <div class="charts-row">
//                     <div class="chart-box">
//                         <h4>Статус показателей</h4>
//                         <div class="chart-container"><canvas id="statusChart"></canvas></div>
//                     </div>
//                     <div class="chart-box">
//                         <h4>Распределение по категориям</h4>
//                         <div class="chart-container"><canvas id="categoryChart"></canvas></div>
//                     </div>
//                     <div class="chart-box">
//                         <h4>Общий профиль здоровья</h4>
//                         <div class="chart-container"><canvas id="radarChart"></canvas></div>
//                     </div>
//                 </div>
//             </div>
//             {{end}}

//             {{if .Summary}}
//             <div class="section">
//                 <div class="section-title"><span class="icon">📋</span> Общее заключение</div>
//                 <div class="summary-box"><p>{{.Summary}}</p></div>
//             </div>
//             {{end}}

//             {{if .Categories}}
//             <div class="section">
//                 <div class="section-title"><span class="icon">🩸</span> Детальный разбор анализов</div>
//                 {{range .Categories}}
//                 <div class="category-wrapper">
//                     <div class="category-header">
//                         <span>{{categoryIcon .Name}}</span>
//                         <span>{{.Name}}</span>
//                         <span class="count">{{len .Indicators}} показателей</span>
//                     </div>
//                     <div class="card-grid">
//                         {{range .Indicators}}
//                         <div class="card" style="border-left-color: {{if eq .Status "normal"}}#48bb78{{else if eq .Status "warning"}}#d69e2e{{else}}#fc8181{{end}};">
//                             <div class="card-header">
//                                 <div class="name">{{.Name}}</div>
//                                 <span class="status-badge {{statusClass .Status}}">{{statusIcon .Status}} {{statusText .Status}}</span>
//                             </div>
//                             <div class="value-row">
//                                 <span class="value">{{.Value}}</span>
//                                 <span class="unit">{{.Unit}}</span>
//                                 <span class="normal">Норма: {{.Normal}}</span>
//                             </div>
//                             <div class="desc">{{.Description}}</div>
//                             {{if .Explanation}}
//                             <div class="explanation"><strong>📖 Что означает:</strong> {{.Explanation}}</div>
//                             {{end}}
//                         </div>
//                         {{end}}
//                     </div>
//                 </div>
//                 {{end}}
//             </div>
//             {{end}}

//             {{if .Attention}}
//             <div class="section">
//                 <div class="section-title"><span class="icon">⚠️</span> На что обратить внимание</div>
//                 <ul class="attentions">{{range .Attention}}<li>{{.}}</li>{{end}}</ul>
//             </div>
//             {{end}}

//             {{if .Recommendations}}
//             <div class="section">
//                 <div class="section-title"><span class="icon">✅</span> Рекомендации</div>
//                 <ul class="recommendations">{{range .Recommendations}}<li>{{.}}</li>{{end}}</ul>
//             </div>
//             {{end}}

//             <div class="disclaimer"><strong>ℹ️ Важно:</strong> {{.Disclaimer}}</div>
//             <div class="footer">AnalyzPRO — интеллектуальный анализ здоровья</div>
//         </div>
//     </div>

//     <script>
//         document.addEventListener('DOMContentLoaded', function() {
//             console.log('=== НАЧАЛО ===');

//             var rawData = {{ json .Categories }};

//             console.log('Данные:', rawData);

//             if (!rawData || rawData.length === 0) {
//                 console.log('Нет данных');
//                 window.reportReady = true;
//                 return;
//             }

//             var normal = 0, warning = 0, critical = 0;
//             var categories = {};

//             for (var i = 0; i < rawData.length; i++) {
//                 var cat = rawData[i];
//                 var indicators = cat.indicators || cat.Indicators || [];
//                 var catName = cat.name || cat.Name || 'Без названия';

//                 categories[catName] = indicators.length;

//                 for (var j = 0; j < indicators.length; j++) {
//                     var ind = indicators[j];
//                     var status = ind.status || ind.Status || 'normal';
//                     if (status === 'normal') normal++;
//                     else if (status === 'warning') warning++;
//                     else if (status === 'critical') critical++;
//                 }
//             }

//             console.log('Норма:', normal, 'Предупреждение:', warning, 'Критические:', critical);
//             console.log('Категории:', categories);

//             var chartsCreated = 0;
//             var totalCharts = 0;

//             var ctx1 = document.getElementById('statusChart');
//             if (ctx1) {
//                 totalCharts++;
//                 new Chart(ctx1, {
//                     type: 'doughnut',
//                     data: {
//                         labels: ['В норме', 'Требует внимания', 'Отклонение'],
//                         datasets: [{
//                             data: [normal, warning, critical],
//                             backgroundColor: ['#48bb78', '#d69e2e', '#fc8181'],
//                             borderWidth: 2,
//                             borderColor: '#fff'
//                         }]
//                     },
//                     options: {
//                         responsive: false,
//                         maintainAspectRatio: false,
//                         plugins: {
//                             legend: {
//                                 position: 'bottom',
//                                 labels: { boxWidth: 10, padding: 8, font: { size: 10 } }
//                             }
//                         },
//                         cutout: '65%',
//                         animation: false
//                     }
//                 });
//                 chartsCreated++;
//                 console.log('✅ Круговая диаграмма создана');
//             }

//             var ctx2 = document.getElementById('categoryChart');
//             if (ctx2) {
//                 totalCharts++;
//                 var labels = Object.keys(categories);
//                 var data = Object.values(categories);
//                 var colors = ['#667eea', '#48bb78', '#d69e2e', '#fc8181', '#9f7aea', '#4299e1', '#ed64a6', '#38b2ac'];
//                 var bgColors = labels.map(function(_, i) { return colors[i % colors.length]; });

//                 if (labels.length > 0) {
//                     new Chart(ctx2, {
//                         type: 'bar',
//                         data: {
//                             labels: labels,
//                             datasets: [{
//                                 label: 'Показателей',
//                                 data: data,
//                                 backgroundColor: bgColors,
//                                 borderRadius: 6
//                             }]
//                         },
//                         options: {
//                             responsive: false,
//                             maintainAspectRatio: false,
//                             plugins: { legend: { display: false } },
//                             scales: {
//                                 y: { beginAtZero: true, ticks: { stepSize: 1 } },
//                                 x: { ticks: { maxRotation: 30, font: { size: 9 } } }
//                             },
//                             animation: false
//                         }
//                     });
//                     chartsCreated++;
//                     console.log('✅ Столбчатая диаграмма создана');
//                 }
//             }

//             var ctx3 = document.getElementById('radarChart');
//             if (ctx3) {
//                 totalCharts++;
//                 var radarLabels = [];
//                 var radarValues = [];

//                 for (var i = 0; i < rawData.length; i++) {
//                     var cat = rawData[i];
//                     var indicators = cat.indicators || cat.Indicators || [];
//                     var total = indicators.length;
//                     var normalCount = 0;

//                     for (var j = 0; j < indicators.length; j++) {
//                         var ind = indicators[j];
//                         var status = ind.status || ind.Status || 'normal';
//                         if (status === 'normal') normalCount++;
//                     }

//                     if (total > 0) {
//                         var catName = cat.name || cat.Name || 'Без названия';
//                         radarLabels.push(catName);
//                         radarValues.push(Math.round((normalCount / total) * 100));
//                     }
//                 }

//                 console.log('Радар метки:', radarLabels);
//                 console.log('Радар значения:', radarValues);

//                 if (radarLabels.length > 0) {
//                     new Chart(ctx3, {
//                         type: 'radar',
//                         data: {
//                             labels: radarLabels,
//                             datasets: [{
//                                 label: 'Процент нормы',
//                                 data: radarValues,
//                                 backgroundColor: 'rgba(43, 108, 176, 0.2)',
//                                 borderColor: '#2b6cb0',
//                                 borderWidth: 2,
//                                 pointBackgroundColor: '#2b6cb0',
//                                 pointBorderColor: '#fff',
//                                 pointBorderWidth: 2,
//                                 pointRadius: 3
//                             }]
//                         },
//                         options: {
//                             responsive: false,
//                             maintainAspectRatio: false,
//                             plugins: { legend: { display: false } },
//                             scales: {
//                                 r: {
//                                     min: 0,
//                                     max: 100,
//                                     ticks: { stepSize: 20, font: { size: 8 } },
//                                     pointLabels: { font: { size: 9 } }
//                                 }
//                             },
//                             animation: false
//                         }
//                     });
//                     chartsCreated++;
//                     console.log('✅ Радар-график создан');
//                 }
//             }

//             setTimeout(function() {
//                 window.reportReady = true;
//                 console.log('✅ Все графики готовы, reportReady = true');
//             }, 300);

//             console.log('=== КОНЕЦ ===');
//         });
//     </script>
// </body>
// </html>`
