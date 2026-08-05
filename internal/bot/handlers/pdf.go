package handlers

import (
	"strings"
	"time"
)

// GenerateSimpleBioscanPDF - создаёт HTML-отчёт
func GenerateSimpleBioscanPDF(text string) ([]byte, error) {
	if len(text) < 10 {
		text = `BIOSCAN - Body Analysis Report

ОБЩАЯ ОЦЕНКА
Состояние: Анализ не выполнен

Причины:
- Превышен лимит запросов к API
- Фото не удалось обработать
- Временная ошибка сервиса

Рекомендации:
- Подождите 1-2 минуты
- Отправьте другое фото
- Убедитесь, что фото чёткое

Повторите попытку позже.`
	}

	cleanText := cleanTextForHTML(text)
	html := generateMinimalHTMLReport(cleanText)

	return []byte(html), nil
}

// generateMinimalHTMLReport - создаёт минималистичный HTML-отчёт в тёмно-зелёных тонах
func generateMinimalHTMLReport(text string) string {
	lines := strings.Split(text, "\n")

	html := `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>BIOSCAN - Body Analysis Report</title>
<style>
    * {
        margin: 0;
        padding: 0;
        box-sizing: border-box;
    }
    
    body {
        font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
        background: #f0f2f0;
        padding: 50px 30px;
        color: #1a2e1a;
        line-height: 1.7;
    }
    
    .container {
        max-width: 820px;
        margin: 0 auto;
        background: #ffffff;
        padding: 50px 60px 45px 60px;
        box-shadow: 0 1px 4px rgba(0,0,0,0.06);
        border-radius: 2px;
    }
    
    /* HEADER */
    .header {
        border-bottom: 1px solid #d0d8d0;
        padding-bottom: 25px;
        margin-bottom: 30px;
    }
    
    .header-top {
        display: flex;
        justify-content: space-between;
        align-items: flex-start;
        flex-wrap: wrap;
    }
    
    .header-title {
        font-size: 26px;
        font-weight: 300;
        letter-spacing: 4px;
        color: #1a3a1a;
        text-transform: uppercase;
    }
    
    .header-title span {
        font-weight: 600;
        color: #2d5a2d;
    }
    
    .header-subtitle {
        font-size: 13px;
        color: #5a7a5a;
        margin-top: 4px;
        letter-spacing: 1px;
        font-weight: 300;
    }
    
    .header-meta {
        text-align: right;
        font-size: 12px;
        color: #5a7a5a;
        line-height: 1.9;
    }
    
    .header-meta strong {
        color: #1a3a1a;
        font-weight: 500;
    }
    
    .header-divider {
        margin-top: 18px;
        display: flex;
        justify-content: space-between;
        font-size: 11px;
        color: #7a9a7a;
        text-transform: uppercase;
        letter-spacing: 0.5px;
    }
    
    /* CONTENT */
    .content {
        font-size: 14px;
        color: #1a2e1a;
    }
    
    /* SECTION */
    .section {
        margin-bottom: 28px;
    }
    
    .section-title {
        font-size: 16px;
        font-weight: 600;
        color: #1a3a1a;
        letter-spacing: 1px;
        padding-bottom: 8px;
        border-bottom: 2px solid #e0e8e0;
        margin-bottom: 14px;
        text-transform: uppercase;
    }
    
    /* SUB-SECTION */
    .sub-section {
        margin: 16px 0 10px 0;
    }
    
    .sub-section-title {
        font-size: 14px;
        font-weight: 600;
        color: #2d5a2d;
        margin-bottom: 6px;
    }
    
    /* TEXT */
    .text-block {
        margin: 5px 0;
        font-size: 14px;
        color: #1a2e1a;
    }
    
    .text-block .label {
        color: #3a5a3a;
        font-weight: 500;
    }
    
    .text-block .value {
        color: #1a2e1a;
    }
    
    /* STATUS */
    .status-excellent {
        color: #1a6b1a;
        font-weight: 500;
    }
    
    .status-good {
        color: #3a7a3a;
        font-weight: 500;
    }
    
    .status-fair {
        color: #8a7a2a;
        font-weight: 500;
    }
    
    .status-poor {
        color: #8a3a2a;
        font-weight: 500;
    }
    
    .status-neutral {
        color: #4a6a4a;
    }
    
    /* METRIC BOXES */
    .metrics-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
        gap: 10px;
        margin: 10px 0 14px 0;
    }
    
    .metric-box {
        border: 1px solid #e0e8e0;
        padding: 12px 16px;
        background: #f8faf8;
        border-radius: 2px;
    }
    
    .metric-box .metric-label {
        font-size: 10px;
        text-transform: uppercase;
        color: #5a7a5a;
        letter-spacing: 0.5px;
        font-weight: 600;
    }
    
    .metric-box .metric-value {
        font-size: 15px;
        font-weight: 600;
        color: #1a3a1a;
        margin-top: 2px;
    }
    
    .metric-box .metric-detail {
        font-size: 12px;
        color: #4a6a4a;
        margin-top: 2px;
    }
    
    /* LIST */
    .item-list {
        list-style: none;
        padding: 0;
        margin: 6px 0 10px 0;
    }
    
    .item-list li {
        padding: 5px 0 5px 20px;
        position: relative;
        font-size: 14px;
        color: #1a2e1a;
        border-bottom: 1px solid #f0f4f0;
    }
    
    .item-list li:last-child {
        border-bottom: none;
    }
    
    .item-list li::before {
        content: "—";
        position: absolute;
        left: 0;
        color: #2d5a2d;
    }
    
    .item-list li strong {
        color: #1a3a1a;
        font-weight: 600;
    }
    
    /* MUSCLE GROUP */
    .muscle-group {
        border-left: 2px solid #2d5a2d;
        padding: 6px 16px;
        margin: 4px 0;
        font-size: 14px;
        color: #1a2e1a;
        background: #f8faf8;
    }
    
    .muscle-group strong {
        color: #1a3a1a;
        font-weight: 600;
    }
    
    /* RECOMMENDATION */
    .recommendation-box {
        border-left: 3px solid #2d5a2d;
        padding: 12px 18px;
        margin: 10px 0;
        background: #f5f8f5;
    }
    
    .recommendation-box .rec-title {
        font-weight: 600;
        color: #1a3a1a;
        font-size: 13px;
        text-transform: uppercase;
        letter-spacing: 0.5px;
    }
    
    /* INFO / WARNING */
    .info-box {
        border: 1px solid #d0d8d0;
        padding: 12px 18px;
        margin: 10px 0;
        background: #f8faf8;
        font-size: 14px;
        color: #1a2e1a;
    }
    
    .warning-box {
        border: 1px solid #c8c8a0;
        padding: 12px 18px;
        margin: 10px 0;
        background: #fafaf0;
        font-size: 14px;
        color: #5a4a1a;
    }
    
    .error-box {
        border: 1px solid #c8a0a0;
        padding: 12px 18px;
        margin: 10px 0;
        background: #faf0f0;
        font-size: 14px;
        color: #5a2a2a;
    }
    
    /* TABLE */
    .data-table {
        width: 100%;
        border-collapse: collapse;
        margin: 10px 0 14px 0;
        font-size: 13px;
    }
    
    .data-table th {
        background: #1a3a1a;
        color: #ffffff;
        padding: 8px 14px;
        text-align: left;
        font-weight: 500;
        font-size: 11px;
        text-transform: uppercase;
        letter-spacing: 0.5px;
    }
    
    .data-table td {
        padding: 7px 14px;
        border-bottom: 1px solid #e0e8e0;
        color: #1a2e1a;
    }
    
    .data-table tr:nth-child(even) td {
        background: #f8faf8;
    }
    
    /* PARAGRAPH */
    .paragraph {
        font-size: 14px;
        color: #1a2e1a;
        margin: 8px 0;
        line-height: 1.8;
    }
    
    .paragraph strong {
        color: #1a3a1a;
        font-weight: 600;
    }
    
    .paragraph em {
        color: #3a5a3a;
        font-style: italic;
    }
    
    /* FOOTER */
    .footer {
        border-top: 1px solid #d0d8d0;
        padding-top: 25px;
        margin-top: 30px;
        display: flex;
        justify-content: space-between;
        align-items: center;
        flex-wrap: wrap;
        font-size: 11px;
        color: #5a7a5a;
        text-transform: uppercase;
        letter-spacing: 0.5px;
    }
    
    .footer .footer-brand {
        font-weight: 600;
        color: #1a3a1a;
    }
    
    .footer .footer-disclaimer {
        color: #7a9a7a;
        font-size: 10px;
        text-transform: none;
        text-align: right;
        line-height: 1.6;
        max-width: 350px;
    }
    
    /* RESPONSIVE */
    @media (max-width: 768px) {
        body {
            padding: 20px 10px;
        }
        
        .container {
            padding: 25px 20px;
        }
        
        .header-top {
            flex-direction: column;
        }
        
        .header-meta {
            text-align: left;
            margin-top: 10px;
            width: 100%;
        }
        
        .footer {
            flex-direction: column;
            text-align: center;
        }
        
        .footer .footer-disclaimer {
            text-align: center;
            margin-top: 10px;
        }
        
        .metrics-grid {
            grid-template-columns: 1fr 1fr;
        }
    }
    
    @media print {
        body {
            background: white;
            padding: 0;
        }
        
        .container {
            box-shadow: none;
            padding: 40px 50px;
        }
    }
</style>
</head>
<body>
<div class="container">
    <!-- HEADER -->
    <div class="header">
        <div class="header-top">
            <div>
                <div class="header-title">BIOSCAN<span> PRO</span></div>
                <div class="header-subtitle">Professional Body Composition Analysis</div>
            </div>
            <div class="header-meta">
                <div><strong>Report ID:</strong> BSC-` + time.Now().Format("20060102") + `-` + time.Now().Format("1504") + `</div>
                <div><strong>Date:</strong> ` + time.Now().Format("02 January 2006") + `</div>
                <div><strong>Time:</strong> ` + time.Now().Format("15:04") + `</div>
            </div>
        </div>
        <div class="header-divider">
            <span>Analysis performed by AnalyzPRO AI System</span>
            <span>Version 2.0 · Confidential</span>
        </div>
    </div>
    
    <!-- CONTENT -->
    <div class="content">`

	inList := false
	inMetrics := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if inList {
				html += `</ul>`
				inList = false
			}
			if inMetrics {
				html += `</div>`
				inMetrics = false
			}
			continue
		}

		// ==========================================
		// ERROR / WARNING / INFO BOXES
		// ==========================================
		if strings.Contains(line, "Сервис временно") || strings.Contains(line, "превышен лимит") {
			if inList {
				html += `</ul>`
				inList = false
			}
			html += `<div class="error-box"><strong>Error</strong> — ` + line + `</div>`
			continue
		}

		if strings.Contains(line, "Что делать:") || strings.Contains(line, "Попробуйте") {
			if inList {
				html += `</ul>`
				inList = false
			}
			html += `<div class="warning-box">` + line + `</div>`
			continue
		}

		// ==========================================
		// SECTION HEADERS
		// ==========================================
		if isHeaderLine(line) {
			if inList {
				html += `</ul>`
				inList = false
			}
			if inMetrics {
				html += `</div>`
				inMetrics = false
			}
			html += `<div class="section"><div class="section-title">` + line + `</div>`
			continue
		}

		// ==========================================
		// SUB-SECTION HEADERS
		// ==========================================
		if isSubHeaderLine(line) {
			if inList {
				html += `</ul>`
				inList = false
			}
			if inMetrics {
				html += `</div>`
				inMetrics = false
			}
			html += `<div class="sub-section"><div class="sub-section-title">` + line + `</div>`
			continue
		}

		// ==========================================
		// RECOMMENDATIONS
		// ==========================================
		if strings.Contains(line, "Рекомендация:") || strings.Contains(line, "Рекомендации:") {
			if inList {
				html += `</ul>`
				inList = false
			}
			if inMetrics {
				html += `</div>`
				inMetrics = false
			}
			text := strings.Replace(line, "Рекомендация:", "", 1)
			text = strings.Replace(text, "Рекомендации:", "", 1)
			html += `<div class="recommendation-box"><div class="rec-title">Recommendation</div>` + strings.TrimSpace(text) + `</div>`
			continue
		}

		// ==========================================
		// STATUS LINES
		// ==========================================
		if strings.Contains(line, "Состояние:") {
			if inList {
				html += `</ul>`
				inList = false
			}
			statusValue := strings.TrimPrefix(line, "Состояние:")
			statusValue = strings.TrimSpace(statusValue)
			statusClass := "status-neutral"
			if strings.Contains(statusValue, "отличн") {
				statusClass = "status-excellent"
			} else if strings.Contains(statusValue, "хорош") {
				statusClass = "status-good"
			} else if strings.Contains(statusValue, "удовлетвор") {
				statusClass = "status-fair"
			} else if strings.Contains(statusValue, "плох") || strings.Contains(statusValue, "критич") {
				statusClass = "status-poor"
			}
			html += `<div class="text-block"><span class="label">Status</span> — <span class="` + statusClass + `">` + statusValue + `</span></div>`
			continue
		}

		if strings.Contains(line, "Симметрия:") {
			if inList {
				html += `</ul>`
				inList = false
			}
			val := strings.TrimPrefix(line, "Симметрия:")
			html += `<div class="text-block"><span class="label">Symmetry</span> — <span class="value">` + strings.TrimSpace(val) + `</span></div>`
			continue
		}

		if strings.Contains(line, "Оценка:") {
			if inList {
				html += `</ul>`
				inList = false
			}
			val := strings.TrimPrefix(line, "Оценка:")
			html += `<div class="text-block"><span class="label">Overall Score</span> — <span class="value">` + strings.TrimSpace(val) + `</span></div>`
			continue
		}

		// ==========================================
		// METRICS
		// ==========================================
		if isMetricLine(line) {
			if !inMetrics {
				if inList {
					html += `</ul>`
					inList = false
				}
				html += `<div class="metrics-grid">`
				inMetrics = true
			}
			html += `<div class="metric-box">` + formatMetric(line) + `</div>`
			continue
		}

		// ==========================================
		// MUSCLE GROUPS
		// ==========================================
		if isMuscleGroup(line) {
			if inList {
				html += `</ul>`
				inList = false
			}
			if inMetrics {
				html += `</div>`
				inMetrics = false
			}
			html += `<div class="muscle-group"><strong>` + line + `</strong></div>`
			continue
		}

		// ==========================================
		// LIST ITEMS
		// ==========================================
		if strings.HasPrefix(line, "•") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") {
			if !inList {
				if inMetrics {
					html += `</div>`
					inMetrics = false
				}
				html += `<ul class="item-list">`
				inList = true
			}
			cleanLine := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(line, "•"), "-"), "*")
			cleanLine = strings.TrimSpace(cleanLine)
			if cleanLine != "" {
				html += `<li>` + cleanLine + `</li>`
			}
			continue
		}

		// ==========================================
		// TABLE
		// ==========================================
		if strings.Contains(line, "|") && strings.Count(line, "|") >= 2 {
			if inList {
				html += `</ul>`
				inList = false
			}
			if inMetrics {
				html += `</div>`
				inMetrics = false
			}
			html += parseTableLine(line)
			continue
		}

		// ==========================================
		// REGULAR TEXT
		// ==========================================
		if inList {
			html += `</ul>`
			inList = false
		}
		if inMetrics {
			html += `</div>`
			inMetrics = false
		}
		if len(line) > 2 {
			html += `<p class="paragraph">` + line + `</p>`
		}
	}

	// Close open tags
	if inList {
		html += `</ul>`
	}
	if inMetrics {
		html += `</div>`
	}

	html += `
    </div>
    
    <!-- FOOTER -->
    <div class="footer">
        <div>
            <span class="footer-brand">AnalyzPRO</span> · AI-Powered Analysis
        </div>
        <div class="footer-disclaimer">
            This report is for informational purposes only.<br>
            Consult a qualified healthcare professional for medical advice.
        </div>
    </div>
</div>
</body>
</html>`

	return html
}

// isHeaderLine - проверяет, является ли строка заголовком секции
func isHeaderLine(line string) bool {
	headers := []string{
		"ОБЩАЯ ОЦЕНКА", "ДЕТАЛЬНЫЙ АНАЛИЗ", "ВЕРХНЯЯ ЧАСТЬ",
		"СРЕДНЯЯ ЧАСТЬ", "НИЖНЯЯ ЧАСТЬ", "ОСАНКА",
		"РЕКОМЕНДАЦИИ", "ПРОГРЕСС-ТРЕК", "Тип фигуры",
		"Пропорции", "Общая оценка", "Краткий вывод",
		"Что важно", "Показатели, требующие внимания",
	}

	upperLine := strings.ToUpper(line)
	for _, h := range headers {
		if strings.Contains(upperLine, strings.ToUpper(h)) {
			return true
		}
	}
	return false
}

// isSubHeaderLine - проверяет, является ли строка подзаголовком
func isSubHeaderLine(line string) bool {
	subHeaders := []string{
		"Скелетно-мышечная система", "Кардиореспираторная система",
		"Обмен веществ", "Пропорции тела", "Мышечный баланс",
		"Жировая ткань", "Осанка и позвоночник",
		"Силовые показатели", "Функциональные резервы",
	}

	upperLine := strings.ToUpper(line)
	for _, h := range subHeaders {
		if strings.Contains(upperLine, strings.ToUpper(h)) {
			return true
		}
	}
	return false
}

// isMuscleGroup - проверяет, является ли строка группой мышц
func isMuscleGroup(line string) bool {
	muscles := []string{
		"ГРУДНЫЕ", "ДЕЛЬТОВИДНЫЕ", "ТРАПЕЦИЯ",
		"ШИРОЧАЙШАЯ", "БИЦЕПС", "ТРИЦЕПС",
		"ПРЯМАЯ МЫШЦА", "КОСЫЕ", "КВАДРИЦЕПС",
		"БИЦЕПС БЕДРА", "ИКРОНОЖНЫЕ", "ЯГОДИЦЫ",
		"ПОЯСНИЧНЫЙ",
	}

	upperLine := strings.ToUpper(line)
	for _, m := range muscles {
		if strings.Contains(upperLine, m) {
			return true
		}
	}
	return false
}

// isMetricLine - проверяет, содержит ли строка метрику
func isMetricLine(line string) bool {
	hasNumber := strings.ContainsAny(line, "0123456789")
	hasUnit := strings.Contains(line, "%") ||
		strings.Contains(line, "см") ||
		strings.Contains(line, "кг") ||
		strings.Contains(line, "мм") ||
		strings.Contains(line, "ед") ||
		strings.Contains(line, "балл") ||
		strings.Contains(line, "из 10") ||
		strings.Contains(line, "/10")

	return hasNumber && hasUnit && len(line) < 60
}

// formatMetric - форматирует метрику в HTML
func formatMetric(line string) string {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) == 2 {
		label := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		return `<div class="metric-label">` + label + `</div>
		        <div class="metric-value">` + value + `</div>`
	}
	return `<div class="metric-label">` + line + `</div>`
}

// parseTableLine - парсит строку таблицы
func parseTableLine(line string) string {
	cells := strings.Split(line, "|")
	var cleanCells []string
	for _, c := range cells {
		c = strings.TrimSpace(c)
		if c != "" {
			cleanCells = append(cleanCells, c)
		}
	}

	if len(cleanCells) == 0 {
		return `<p class="paragraph">` + line + `</p>`
	}

	html := `<table class="data-table"><tr>`
	for _, c := range cleanCells {
		html += `<td>` + c + `</td>`
	}
	html += `</tr></table>`
	return html
}

// cleanTextForHTML - удаляет лишние символы
func cleanTextForHTML(text string) string {
	replacements := map[string]string{
		"**": "", "*": "", "_": "", "`": "", "#": "",
		"━": "", "─": "", "┌": "", "┐": "", "└": "", "┘": "",
		"├": "", "┤": "", "│": "", "☑": "", "✓": "",
		"✗": "", "●": "", "○": "", "◆": "", "◇": "",
		"📌": "", "🩺": "", "⚠️": "", "✅": "", "ℹ️": "",
		"📊": "", "📸": "", "🧍": "", "💪": "", "🦴": "",
		"🏋️": "", "🦵": "", "📋": "", "🎯": "", "🔍": "",
		"🔧": "", "📈": "", "📉": "",
	}

	for old, new := range replacements {
		text = strings.ReplaceAll(text, old, new)
	}

	return strings.TrimSpace(text)
}
