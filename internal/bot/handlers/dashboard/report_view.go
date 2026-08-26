package dashboard

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/theamornoir/analyzpro/internal/monitoring"
)

// isRichHTML - признак «богатого» print-ready отчёта (досье расширенного
// анализа / Bioscan PRO), сгенерированного через report.Renderer (шаблоны
// A4 с таблицами и @page). Такие отчёты оставляем как есть; для «плоских»
// результатов (обычный анализ, базовый Bioscan, Общая оценка здоровья)
// строим чистый структурированный HTML из JsonData (см. reportHTML).
func isRichHTML(s string) bool {
	return strings.Contains(s, "<table") || strings.Contains(s, "@page")
}

// statusColorHEX возвращает цвет статуса показателя для HTML-отчёта.
func statusColorHEX(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical", "критично", "alert":
		return "#e5484d"
	case "warning", "внимание", "warn":
		return "#e8744a"
	default:
		return "#4f8a6d"
	}
}

func statusLabelRUshort(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical", "критично", "alert":
		return "критично"
	case "warning", "внимание", "warn":
		return "внимание"
	case "good", "ok":
		return "хорошо"
	default:
		return "норма"
	}
}

// statusTint возвращает полупрозрачную подложку-бейдж под цвет статуса
// (для карточек записей отчёта в тёмной теме).
func statusTint(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical", "критично", "alert":
		return "rgba(229,72,77,0.16)"
	case "warning", "внимание", "warn":
		return "rgba(232,116,74,0.16)"
	default:
		return "rgba(79,138,109,0.18)"
	}
}

// scoreStatusLabel возвращает словесную метку уровня по числовому баллу
// (0-100) для показателей, у которых нет явного статуса.
func scoreStatusLabel(s int) string {
	switch {
	case s >= 80:
		return "Хорошо"
	case s >= 60:
		return "Норма"
	case s >= 40:
		return "Внимание"
	default:
		return "Критично"
	}
}

func esc(s string) string {
	return html.EscapeString(s)
}

// reVisualNote - модель иногда добавляет в значение показателя пояснение
// вида "(визуальная оценка)" (для приблизительных оценок по фото). В
// карточках отчёта это раздувает бейдж и ломает вёрстку на узком экране,
// поэтому вырезаем такую «воду» из значений показателей перед рендером.
var reVisualNote = regexp.MustCompile(`(?i)\s*[\(\[]?\s*визуальная\s+оценка\s*[\)\]]?\s*`)

// cleanMetricValue очищает значение показателя от пояснений вроде
// "(визуальная оценка)", оставляя только саму цифру/текст ("около 16").
func cleanMetricValue(s string) string {
	return strings.TrimSpace(reVisualNote.ReplaceAllString(s, " "))
}

// reportViewCSS - общий стиль чистых HTML-отчётов (светлая тема, удобно для
// просмотра в WebView и печати в PDF). Без «воды»: только фактические
// значения в таблицах/карточках.
const reportViewCSS = `
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Arial,sans-serif;max-width:760px;margin:0 auto;padding:18px 14px 28px;background:#0f1419;color:#e8eef2;line-height:1.5;-webkit-text-size-adjust:100%}
h1{color:#1FA6A8;margin:0 0 4px;font-size:22px}
.sub{color:#8b9aa7;font-size:13px;margin-bottom:18px}
.score{display:inline-flex;align-items:baseline;gap:6px;font-size:40px;font-weight:700;color:#1FA6A8;margin:8px 0 4px}
.score small{font-size:14px;color:#8b9aa7;font-weight:400}
.note{background:#1a212b;border:1px solid rgba(255,255,255,0.06);border-radius:12px;padding:12px 14px;color:#c7d2da;font-size:14px;margin:14px 0;line-height:1.5}
.card{background:#1a212b;border:1px solid rgba(255,255,255,0.06);border-radius:16px;padding:16px;margin:14px 0;box-shadow:0 2px 12px rgba(0,0,0,0.25)}
.card h2{margin:0 0 12px;font-size:16px;color:#e8eef2}
/* Адаптивные карточки записей (зоны тела, показатели, композиция).
   НЕ таблицы: на телефоне (360-420px) колонки не сжимаются в вертикальные
   буквы - каждая запись - отдельная карточка; на узком экране - в 1 колонку. */
.rec-grid{display:grid;grid-template-columns:1fr 1fr;gap:10px;margin-top:4px}
.rec-list{display:flex;flex-direction:column;gap:10px;margin-top:4px}
.rec{background:#222c38;border:1px solid rgba(255,255,255,0.05);border-radius:12px;padding:12px 14px}
.rec-head{display:flex;align-items:center;gap:8px;font-size:14px;font-weight:700;color:#e8eef2;margin:0 0 8px}
.rec-head .ic{font-size:15px;line-height:1}
.rec-badge{display:inline-flex;align-items:baseline;gap:6px;font-size:13px;font-weight:700;border-radius:999px;padding:5px 12px;white-space:nowrap}
.rec-badge .sep{opacity:.55;font-weight:400;margin:0 1px}
.rec-comment{color:#c7d2da;font-size:13px;line-height:1.55;margin-top:8px;overflow-wrap:anywhere}
@media (max-width:520px){.rec-grid{grid-template-columns:1fr}}
/* Сферы образа жизни (Общая оценка здоровья): строки с баром */
.row{background:#222c38;border-radius:10px;padding:10px 12px;margin:8px 0}
.row .top{display:flex;justify-content:space-between;align-items:center;gap:10px}
.row .name{font-weight:600;color:#e8eef2}
.row .sc{font-weight:700;font-size:15px}
.bar{height:8px;border-radius:6px;background:#0f1419;margin-top:6px;overflow:hidden}
.bar > span{display:block;height:100%;border-radius:6px}
.plan li{margin-bottom:6px;line-height:1.5;color:#c7d2da}
.demo{margin-top:18px;padding:12px 14px;background:#1a212b;border-radius:12px;color:#8b9aa7;font-size:12px;line-height:1.5}
/* Таблицы (анализ / общая оценка здоровья): сохраняем, но в тёмной теме */
table{width:100%;border-collapse:collapse;font-size:13px}
th,td{text-align:left;padding:8px 10px;border-bottom:1px solid rgba(255,255,255,0.06);overflow-wrap:anywhere;word-break:normal;color:#c7d2da}
th{color:#8b9aa7;font-weight:600;font-size:12px}
td.val{font-weight:700;white-space:normal;overflow-wrap:anywhere;color:#e8eef2}
.st{font-weight:600}
`

// buildHealthAssessmentHTML строит чистый HTML-отчёт «Общая оценка
// здоровья» из структурированного JsonData (без медицинских файлов).
func buildHealthAssessmentHTML(entry *monitoring.HistoryEntry) string {
	var ha struct {
		Title       string `json:"title"`
		HealthIndex int    `json:"health_index"`
		Summary     string `json:"summary"`
		Lifestyle   map[string]struct {
			Score   int    `json:"score"`
			Comment string `json:"comment"`
		} `json:"lifestyle"`
		RiskZones []struct {
			Name        string `json:"name"`
			Level       string `json:"level"`
			Description string `json:"description"`
		} `json:"risk_zones"`
		Plan struct {
			Sleep     string `json:"sleep"`
			Nutrition string `json:"nutrition"`
			Movement  string `json:"movement"`
			Stress    string `json:"stress"`
		} `json:"plan"`
	}
	_ = json.Unmarshal([]byte(entry.JsonData), &ha)

	title := strings.TrimSpace(ha.Title)
	if title == "" {
		title = "Общая оценка здоровья"
	}
	date := entry.Date.Format("2006-01-02")
	idx := ha.HealthIndex
	if idx < 0 {
		idx = 0
	}
	if idx > 100 {
		idx = 100
	}
	idxColor := "#4f8a6d"
	if idx < 60 {
		idxColor = "#e8744a"
	}
	if idx < 40 {
		idxColor = "#e5484d"
	}

	var b strings.Builder
	b.WriteString("<!doctype html><html lang=\"ru\"><head><meta charset=\"utf-8\"><title>" + esc(title) + "</title><style>" + reportViewCSS + "</style></head><body>")
	b.WriteString("<h1>" + esc(title) + "</h1>")
	b.WriteString("<div class=\"sub\">Prisma · Общая оценка здоровья · " + esc(date) + "</div>")
	b.WriteString("<div class=\"score\" style=\"color:" + idxColor + "\">" + fmt.Sprintf("%d", idx) + "<small>из 100</small></div>")

	if s := strings.TrimSpace(ha.Summary); s != "" {
		b.WriteString("<div class=\"note\">" + esc(s) + "</div>")
	}

	// Сферы образа жизни.
	if len(ha.Lifestyle) > 0 {
		b.WriteString("<div class=\"card\"><h2>Оценка по сферам</h2>")
		order := []string{"sleep", "nutrition", "movement", "activity", "stress", "energy", "wellbeing"}
		labels := map[string]string{
			"sleep": "Сон", "nutrition": "Питание", "movement": "Движение",
			"activity": "Активность", "stress": "Стресс", "energy": "Энергия", "wellbeing": "Самочувствие",
		}
		seen := map[string]bool{}
		add := func(key string, dim struct {
			Score   int    `json:"score"`
			Comment string `json:"comment"`
		}) {
			if seen[key] {
				return
			}
			seen[key] = true
			label := labels[key]
			if label == "" {
				label = key
			}
			sc := dim.Score
			if sc < 0 {
				sc = 0
			}
			if sc > 100 {
				sc = 100
			}
			col := statusColorHEX("normal")
			if sc >= 80 {
				col = statusColorHEX("good")
			} else if sc < 60 {
				col = statusColorHEX("warning")
			}
			comment := strings.TrimSpace(dim.Comment)
			b.WriteString("<div class=\"row\"><div class=\"top\"><span class=\"name\">" + esc(label) + "</span><span class=\"sc\" style=\"color:" + col + "\">" + fmt.Sprintf("%d", sc) + "/100</span></div>")
			b.WriteString("<div class=\"bar\"><span style=\"width:" + fmt.Sprintf("%d", sc) + "%;background:" + col + "\"></span></div>")
			if comment != "" {
				b.WriteString("<div class=\"sub\" style=\"margin:6px 0 0\">" + esc(comment) + "</div>")
			}
			b.WriteString("</div>")
		}
		for _, k := range order {
			if dim, ok := ha.Lifestyle[k]; ok {
				add(k, dim)
			}
		}
		for k, dim := range ha.Lifestyle {
			add(k, dim)
		}
		b.WriteString("</div>")
	}

	// Зоны риска.
	if len(ha.RiskZones) > 0 {
		b.WriteString("<div class=\"card\"><h2>Зоны риска</h2><table><thead><tr><th>Зона</th><th>Уровень</th><th>Описание</th></tr></thead><tbody>")
		for _, z := range ha.RiskZones {
			lvl := strings.TrimSpace(z.Level)
			col := statusColorHEX("warning")
			if strings.EqualFold(lvl, "critical") || strings.EqualFold(lvl, "высокий") {
				col = statusColorHEX("critical")
			} else if strings.EqualFold(lvl, "good") || strings.EqualFold(lvl, "низкий") {
				col = statusColorHEX("normal")
			}
			b.WriteString("<tr><td>" + esc(z.Name) + "</td><td class=\"st\" style=\"color:" + col + "\">" + esc(lvl) + "</td><td>" + esc(z.Description) + "</td></tr>")
		}
		b.WriteString("</tbody></table></div>")
	}

	// План на 3 месяца.
	planItems := []struct {
		Label, Text string
	}{
		{"Сон", ha.Plan.Sleep},
		{"Питание", ha.Plan.Nutrition},
		{"Движение", ha.Plan.Movement},
		{"Стресс", ha.Plan.Stress},
	}
	hasPlan := false
	for _, p := range planItems {
		if strings.TrimSpace(p.Text) != "" {
			hasPlan = true
			break
		}
	}
	if hasPlan {
		b.WriteString("<div class=\"card\"><h2>План улучшения на 3 месяца</h2><ul class=\"plan\">")
		for _, p := range planItems {
			if strings.TrimSpace(p.Text) != "" {
				b.WriteString("<li><b>" + esc(p.Label) + ":</b> " + esc(p.Text) + "</li>")
			}
		}
		b.WriteString("</ul></div>")
	}

	b.WriteString("</body></html>")
	return b.String()
}

// buildAnalysisStructuredHTML строит чистый HTML обычного анализа из
// структурированного JsonData (sections/categories с indicators). Если
// структурированных показателей нет - возвращает "", чтобы reportHTML
// откатился к сохранённому тексту.
func buildAnalysisStructuredHTML(entry *monitoring.HistoryEntry) string {
	var doc struct {
		Title    string `json:"title"`
		Note     string `json:"note"`
		Sections []struct {
			Name       string     `json:"name"`
			Title      string     `json:"title"`
			Indicators []indShape `json:"indicators"`
		} `json:"sections"`
		Categories []struct {
			Name       string     `json:"name"`
			Title      string     `json:"title"`
			Indicators []indShape `json:"indicators"`
		} `json:"categories"`
	}
	if err := json.Unmarshal([]byte(entry.JsonData), &doc); err != nil {
		return ""
	}
	groups := []struct {
		Name       string
		Indicators []indShape
	}{}
	for _, s := range doc.Sections {
		groups = append(groups, struct {
			Name       string
			Indicators []indShape
		}{nameOrTitle(s.Name, s.Title), s.Indicators})
	}
	for _, c := range doc.Categories {
		groups = append(groups, struct {
			Name       string
			Indicators []indShape
		}{nameOrTitle(c.Name, c.Title), c.Indicators})
	}

	total := 0
	for _, g := range groups {
		total += len(g.Indicators)
	}
	if total == 0 {
		return ""
	}

	title := strings.TrimSpace(doc.Title)
	if title == "" {
		title = "Анализ"
	}
	date := entry.Date.Format("2006-01-02")

	var b strings.Builder
	b.WriteString("<!doctype html><html lang=\"ru\"><head><meta charset=\"utf-8\"><title>" + esc(title) + "</title><style>" + reportViewCSS + "</style></head><body>")
	b.WriteString("<h1>" + esc(title) + "</h1>")
	b.WriteString("<div class=\"sub\">Prisma · Анализ · " + esc(date) + "</div>")
	if s := strings.TrimSpace(doc.Note); s != "" {
		b.WriteString("<div class=\"note\">" + esc(s) + "</div>")
	}
	for _, g := range groups {
		if len(g.Indicators) == 0 {
			continue
		}
		b.WriteString("<div class=\"card\"><h2>" + esc(g.Name) + "</h2><table><thead><tr><th>Показатель</th><th>Значение</th><th>Норма</th><th>Статус</th></tr></thead><tbody>")
		for _, ind := range g.Indicators {
			if strings.TrimSpace(ind.Name) == "" {
				continue
			}
			col := statusColorHEX(ind.Status)
			b.WriteString("<tr><td>" + esc(ind.Name) + "</td><td class=\"val\">" + esc(cleanMetricValue(ind.Value)) + "</td><td>" + esc(ind.Normal) + "</td><td class=\"st\" style=\"color:" + col + "\">" + statusLabelRUshort(ind.Status) + "</td></tr>")
		}
		b.WriteString("</tbody></table></div>")
	}
	b.WriteString("</body></html>")
	return b.String()
}

// buildBioscanStructuredHTML строит чистый HTML отчёта Bioscan (базовый или
// PRO) из структурированного JsonData: показатели тела (posture) и зоны.
func buildBioscanStructuredHTML(entry *monitoring.HistoryEntry) string {
	var doc struct {
		Title   string `json:"title"`
		Score   int    `json:"score"`
		Level   string `json:"level"`
		Summary string `json:"summary"`
		Note    string `json:"note"`
		Posture struct {
			PostureScore    int `json:"posture_score"`
			Symmetry        int `json:"symmetry"`
			ShoulderBalance int `json:"shoulder_balance"`
			PelvicBalance   int `json:"pelvic_balance"`
			SpinalAlignment int `json:"spinal_alignment"`
			Mobility        int `json:"mobility"`
			Stability       int `json:"stability"`
		} `json:"posture"`
		Zones []struct {
			Name    string `json:"name"`
			Score   int    `json:"score"`
			Status  string `json:"status"`
			Comment string `json:"comment"`
		} `json:"zones"`
		Composition []struct {
			Name   string `json:"name"`
			Value  string `json:"value"`
			Unit   string `json:"unit"`
			Status string `json:"status"`
			Ref    string `json:"ref"`
		} `json:"composition"`
		Strengths []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"strengths"`
		Improve []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"improve"`
	}
	if err := json.Unmarshal([]byte(entry.JsonData), &doc); err != nil {
		return ""
	}
	hasBody := doc.Posture.PostureScore > 0 || doc.Posture.Symmetry > 0 || doc.Posture.Mobility > 0
	hasZones := len(doc.Zones) > 0
	hasComp := len(doc.Composition) > 0
	hasStr := len(doc.Strengths) > 0
	hasImp := len(doc.Improve) > 0
	if !hasBody && !hasZones && !hasComp && !hasStr && !hasImp {
		note := strings.TrimSpace(doc.Note)
		if note == "" {
			note = strings.TrimSpace(doc.Summary)
		}
		if note != "" {
			return buildNoteHTML("Bioscan", note)
		}
		return ""
	}

	title := strings.TrimSpace(doc.Title)
	if title == "" {
		title = "Bioscan"
	}
	date := entry.Date.Format("2006-01-02")

	var b strings.Builder
	b.WriteString("<!doctype html><html lang=\"ru\"><head><meta charset=\"utf-8\"><title>" + esc(title) + "</title><style>" + reportViewCSS + "</style></head><body>")
	b.WriteString("<h1>" + esc(title) + "</h1>")
	b.WriteString("<div class=\"sub\">Prisma · Bioscan · " + esc(date) + "</div>")
	if doc.Score > 0 {
		col := statusColorHEX("normal")
		if doc.Score < 60 {
			col = statusColorHEX("warning")
		}
		if doc.Score < 40 {
			col = statusColorHEX("critical")
		}
		b.WriteString("<div class=\"score\" style=\"color:" + col + "\">" + fmt.Sprintf("%d", doc.Score) + "<small>Body Score</small></div>")
	}
	if st := strings.TrimSpace(doc.Summary); st != "" {
		b.WriteString("<div class=\"note\">" + esc(st) + "</div>")
	} else if st := strings.TrimSpace(doc.Note); st != "" {
		b.WriteString("<div class=\"note\">" + esc(st) + "</div>")
	}

	if hasBody {
		b.WriteString("<div class=\"card\"><h2>🧍 Показатели тела</h2><div class=\"rec-grid\">")
		body := []struct {
			Name  string
			Score int
		}{
			{"Осанка", doc.Posture.PostureScore},
			{"Симметрия", doc.Posture.Symmetry},
			{"Плечи", doc.Posture.ShoulderBalance},
			{"Таз", doc.Posture.PelvicBalance},
			{"Позвоночник", doc.Posture.SpinalAlignment},
			{"Мобильность", doc.Posture.Mobility},
			{"Стабильность", doc.Posture.Stability},
		}
		for _, it := range body {
			if it.Score <= 0 {
				continue
			}
			col := statusColorHEX(scoreStatusLabel(it.Score))
			bg := statusTint(scoreStatusLabel(it.Score))
			b.WriteString("<div class=\"rec\"><div class=\"rec-head\"><span class=\"ic\">▦</span>" + esc(it.Name) + "</div>" +
				"<span class=\"rec-badge\" style=\"color:" + col + ";background:" + bg + "\">" +
				fmt.Sprintf("%d", it.Score) + "<span class=\"sep\">·</span>" + scoreStatusLabel(it.Score) + "</span></div>")
		}
		b.WriteString("</div></div>")
	}

	if hasComp {
		b.WriteString("<div class=\"card\"><h2>🧬 Композиция тела</h2><div class=\"rec-list\">")
		for _, c := range doc.Composition {
			col := statusColorHEX(c.Status)
			bg := statusTint(c.Status)
			val := cleanMetricValue(c.Value)
			if c.Unit != "" {
				// Модель может уже включить единицу в значение (например,
				// «около 16 (визуальная оценка) %»). Чтобы не получить
				// «16 %%», убираем уже имеющуюся единицу из значения.
				trimmed := strings.TrimSuffix(val, c.Unit)
				if trimmed != val {
					val = strings.TrimSpace(trimmed)
				}
				// Для процентов склейку без пробела («около 16%»),
				// для остальных единиц - через пробел («50 кг»).
				if c.Unit == "%" {
					val += c.Unit
				} else {
					val += " " + c.Unit
				}
			}
			badge := esc(val)
			if st := strings.TrimSpace(statusLabelRUshort(c.Status)); st != "" {
				badge += "<span class=\"sep\">·</span>" + st
			}
			comment := ""
			if r := strings.TrimSpace(c.Ref); r != "" {
				comment = "Норма: " + r
			}
			b.WriteString("<div class=\"rec\"><div class=\"rec-head\"><span class=\"ic\">◈</span>" + esc(c.Name) + "</div>" +
				"<span class=\"rec-badge\" style=\"color:" + col + ";background:" + bg + "\">" + badge + "</span>")
			if comment != "" {
				b.WriteString("<div class=\"rec-comment\">" + esc(comment) + "</div>")
			}
			b.WriteString("</div>")
		}
		b.WriteString("</div></div>")
	}

	if hasZones {
		b.WriteString("<div class=\"card\"><h2>📍 Зоны тела</h2><div class=\"rec-grid\">")
		for _, z := range doc.Zones {
			col := statusColorHEX(z.Status)
			bg := statusTint(z.Status)
			badge := fmt.Sprintf("%d", z.Score) + "<span class=\"sep\">·</span>" + statusLabelRUshort(z.Status)
			b.WriteString("<div class=\"rec\"><div class=\"rec-head\"><span class=\"ic\">🏷️</span>" + esc(z.Name) + "</div>" +
				"<span class=\"rec-badge\" style=\"color:" + col + ";background:" + bg + "\">" + badge + "</span>")
			if c := strings.TrimSpace(z.Comment); c != "" {
				b.WriteString("<div class=\"rec-comment\">" + esc(c) + "</div>")
			}
			b.WriteString("</div>")
		}
		b.WriteString("</div></div>")
	}

	if hasStr {
		b.WriteString("<div class=\"card\"><h2>💪 Сильные стороны</h2><div class=\"rec-list\">")
		for _, st := range doc.Strengths {
			b.WriteString("<div class=\"rec\"><div class=\"rec-head\"><span class=\"ic\">✓</span>" + esc(st.Title) + "</div>")
			if d := strings.TrimSpace(st.Description); d != "" {
				b.WriteString("<div class=\"rec-comment\">" + esc(d) + "</div>")
			}
			b.WriteString("</div>")
		}
		b.WriteString("</div></div>")
	}

	if hasImp {
		b.WriteString("<div class=\"card\"><h2>🌱 Зоны роста</h2><div class=\"rec-list\">")
		for _, st := range doc.Improve {
			b.WriteString("<div class=\"rec\"><div class=\"rec-head\"><span class=\"ic\">↑</span>" + esc(st.Title) + "</div>")
			if d := strings.TrimSpace(st.Description); d != "" {
				b.WriteString("<div class=\"rec-comment\">" + esc(d) + "</div>")
			}
			b.WriteString("</div>")
		}
		b.WriteString("</div></div>")
	}

	b.WriteString("</body></html>")
	return b.String()
}

// buildNoteHTML - аккуратный HTML из чистого текстового примечания (без
// структурированных показателей). Сохраняет переносы строк.
func buildNoteHTML(title, note string) string {
	body := strings.ReplaceAll(esc(note), "\n", "<br>")
	return "<!doctype html><html lang=\"ru\"><head><meta charset=\"utf-8\"><title>" + esc(title) + "</title><style>" + reportViewCSS + "</style></head><body>" +
		"<h1>" + esc(title) + "</h1>" +
		"<div class=\"note\">" + body + "</div>" +
		"</body></html>"
}

// nameOrTitle возвращает непустой заголовок группы показателей.
func nameOrTitle(name, title string) string {
	if strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return strings.TrimSpace(title)
}

// indShape - форма показателя в структурированном отчёте анализа.
type indShape struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Status string `json:"status"`
	Normal string `json:"normal"`
	Score  int    `json:"score"`
}
