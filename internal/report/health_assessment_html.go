package report

import (
	"fmt"
	"html"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/theamornoir/analyzpro/internal/models"
)

// healthDashboardCSS - стиль дашборда «Общая оценка здоровья»:
//   - дефолт (без класса на <body>) - тёмный фиолетовый неон для веб-
//     предпросмотра в «Моём профиле» (Mini App);
//   - body.print - ТА ЖЕ тёмная неоново-фиолетовая вёрстка, но с добавленными
//     печатными правилами (page-break-inside:avoid и тёмный фон листа A4) для
//     конвертации HTML->PDF (html2pdf.app). Включается явно через class="print"
//     на <body> ТОЛЬКО при генерации PDF, поэтому не зависит от того,
//     применяет ли конвертер печатные медиа-правила. Цвета/свечения/градиенты
//     остаются тёмными - меняем только то, что мешает печати (разрезание
//     блоков), чтобы файл выглядел «как было»: тёмный фон, неон, красиво.
//
// Кольца заполняются статически через inline stroke-dasharray/stroke-
// dashoffset (конкретные числа, без CSS-переменных и анимаций) и окрашиваются
// inline-цветом из Go - корректно и одинаково рендерятся и в предпросмотре,
// и в PDF (печатные правила цвет не переопределяют).
const healthDashboardCSS = `* { margin:0; padding:0; box-sizing:border-box; }
:root {
  --bg-primary:#0B0A1A; --bg-secondary:#1A1829; --bg-card:rgba(26,24,41,0.6);
  --border-color:rgba(179,136,255,0.2); --text-primary:#E8E6FF; --text-secondary:#A8A6C4;
  --accent-purple:#B388FF; --accent-purple-dark:#7C4DFF; --accent-blue:#651FFF;
  --glow-purple:179,136,255; --glow-blue:101,31,255;
  --success:#00FF88; --warning:#FFB800; --danger:#FF3D7F;
}
/* Для веб-предпросмотра - без полей (фон тянется на весь экран). */
@page { size:A4; margin:0; }
html, body {
  background:#0B0A1A;
  -webkit-print-color-adjust:exact; print-color-adjust:exact;
}
body {
  background:linear-gradient(135deg,var(--bg-primary) 0%,#16132B 100%);
  background-attachment:fixed;
  color:var(--text-primary);
  font-family:'Inter',system-ui,-apple-system,'Segoe UI',Roboto,Arial,sans-serif;
  line-height:1.6; min-height:100vh; padding:40px 20px;
  -webkit-print-color-adjust:exact; print-color-adjust:exact;
}
.container { max-width:1400px; margin:0 auto; }
/* HEADER */
.header { text-align:center; margin-bottom:50px; }
.header-title { display:flex; align-items:center; justify-content:center; gap:15px; font-size:2.1em; font-weight:700; letter-spacing:-1px; }
.header-subtitle { font-size:0.9em; color:var(--text-secondary); font-weight:300; letter-spacing:0.5px; }
/* MAIN HEALTH INDEX */
.main-index {
  background:var(--bg-card); backdrop-filter:blur(20px); border:1px solid var(--border-color);
  border-radius:24px; padding:50px 40px; margin-bottom:50px; text-align:center;
  /* НЕТ box-shadow: в html2pdf он рендерится квадратом позади блока. */
}
.index-circle-container { display:flex; justify-content:center; margin-bottom:30px; }
.index-circle { width:200px; height:200px; position:relative; }
/* НЕТ подложки/свечения позади круга: цифра лежит на прозрачном центре
   кольца (SVG), без квадратных фонов. Любой box-shadow/filter рендерится
   конвертером HTML->PDF как КВАДРАТ позади цифры - поэтому не используем. */
.index-circle svg { width:100%; height:100%; transform:rotate(-90deg); position:relative; z-index:1; }
.index-circle-bg { fill:none; stroke:rgba(179,136,255,0.2); stroke-width:8; }
.index-circle-progress { fill:none; stroke:#B388FF; stroke-width:8; stroke-linecap:round; stroke-dasharray:377; stroke-dashoffset:0; }
.index-value { position:absolute; inset:0; z-index:2; display:flex; flex-direction:column; align-items:center; justify-content:center; text-align:center; }
.index-number { font-family:'Orbitron',monospace; font-size:2.1em; font-weight:900; line-height:1; white-space:nowrap;
  color:#B388FF; letter-spacing:-0.5px; }
.index-status { font-size:1.05em; font-weight:600; margin-top:18px; letter-spacing:0.5px; }
.index-status.status-excellent { color:var(--success); }
.index-status.status-moderate { color:var(--warning); }
.index-status.status-critical { color:var(--danger); }
.index-description { font-size:0.88em; color:var(--text-secondary); margin-top:14px; max-width:480px; margin-left:auto; margin-right:auto; }
/* CARDS GRID */
.cards-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(340px,1fr)); gap:28px; margin-bottom:50px; }
.metric-card { background:var(--bg-card); backdrop-filter:blur(20px); border:1px solid var(--border-color);
  border-radius:16px; padding:30px; }
.card-header { display:flex; justify-content:space-between; align-items:center; margin-bottom:18px; }
.card-title { font-size:1.05em; font-weight:600; display:flex; align-items:center; gap:10px; }
.card-circle-container { width:100%; height:120px; display:flex; justify-content:center; margin-bottom:16px; }
.card-circle { width:120px; height:120px; position:relative; }
.card-circle svg { width:100%; height:100%; transform:rotate(-90deg); position:relative; z-index:1; }
.card-circle-bg { fill:none; stroke:rgba(179,136,255,0.15); stroke-width:6; }
.card-circle-progress { fill:none; stroke:#7C4DFF; stroke-width:6; stroke-linecap:round; stroke-dasharray:314; stroke-dashoffset:0; }
.card-value { position:absolute; inset:0; z-index:2; display:flex; align-items:center; justify-content:center; font-family:'Orbitron',monospace; font-size:1.5em; font-weight:700; line-height:1; color:#FFFFFF; white-space:nowrap; }
.card-status { font-size:0.85em; font-weight:600; text-transform:uppercase; letter-spacing:1px; margin-bottom:10px; height:22px; }
.card-text { font-size:0.82em; color:var(--text-secondary); line-height:1.55; }
.status-excellent { color:var(--success); } .status-good { color:var(--success); }
.status-moderate { color:var(--warning); } .status-poor { color:var(--danger); } .status-critical { color:var(--danger); }
.metric-card.score-excellent .card-circle-progress { stroke:#00FF88; }
.metric-card.score-good .card-circle-progress { stroke:#00D9FF; }
.metric-card.score-moderate .card-circle-progress { stroke:#FFB800; }
.metric-card.score-poor .card-circle-progress { stroke:#FF5C2D; }
.metric-card.score-critical .card-circle-progress { stroke:#FF1A5C; }
/* RECOMMENDATIONS */
.recommendations { background:var(--bg-card); backdrop-filter:blur(20px); border:1px solid var(--border-color);
  border-radius:16px; padding:40px; }
.recommendations-title { display:flex; align-items:center; gap:12px; font-size:1.4em; font-weight:600; margin-bottom:30px; }
.recommendations-list { display:grid; grid-template-columns:repeat(auto-fit,minmax(280px,1fr)); gap:20px; }
.recommendation-item { background:rgba(101,31,255,0.08); border-left:3px solid var(--accent-purple); padding:18px; border-radius:16px; }
.recommendation-number { display:inline-block; width:26px; height:26px; background:linear-gradient(135deg,var(--accent-purple),var(--accent-blue));
  border-radius:50%; text-align:center; line-height:26px; font-weight:700; font-size:0.85em; margin-bottom:8px; color:#0B0A1A; }
.recommendation-text { font-size:0.9em; line-height:1.6; color:var(--text-secondary); }
.recommendation-emphasis { color:var(--accent-purple); font-weight:600; }
/* OVERVIEW (summary) */
.overview { background:var(--bg-card); backdrop-filter:blur(20px); border:1px solid var(--border-color); border-radius:16px; padding:24px 28px; margin-bottom:40px; }
.overview-title { font-size:1.05em; font-weight:600; margin-bottom:12px; color:var(--text-primary); }
.overview-text { font-size:0.88em; color:var(--text-secondary); line-height:1.6; }
/* RISK ZONES */
.risk-zones { background:var(--bg-card); backdrop-filter:blur(20px); border:1px solid rgba(255,61,127,0.25); border-radius:16px; padding:24px 28px; margin-bottom:40px; }
.risk-zones-title { font-size:1.05em; font-weight:600; margin-bottom:16px; color:var(--danger); display:flex; align-items:center; gap:10px; }
.risk-list { display:flex; flex-direction:column; gap:12px; }
.risk-item { display:flex; flex-direction:column; gap:4px; background:rgba(255,61,127,0.07); border-left:3px solid var(--danger); padding:12px 16px; border-radius:16px; }
.risk-head { display:flex; align-items:baseline; gap:8px; flex-wrap:wrap; }
.risk-name { font-size:0.95em; font-weight:600; color:var(--text-primary); }
.risk-level { font-size:0.75em; font-weight:700; text-transform:uppercase; letter-spacing:0.5px; }
.risk-desc { font-size:0.85em; color:var(--text-secondary); line-height:1.5; }
/* LIFESTYLE RADAR CHART (первый экран) - чистый SVG, БЕЗ box-shadow/filter
   (иначе html2pdf рисует квадратные подложки позади фигур). */
.chart-card { background:var(--bg-card); backdrop-filter:blur(20px); border:1px solid var(--border-color); border-radius:16px; padding:24px 28px 28px; margin-bottom:40px; }
.chart-title { font-size:1.05em; font-weight:600; margin-bottom:14px; color:var(--text-primary); text-align:center; }
.radar { display:block; margin:0 auto; width:100%; max-width:420px; height:auto; }
.radar-grid { fill:none; stroke:rgba(179,136,255,0.16); stroke-width:1; }
.radar-axis { stroke:rgba(179,136,255,0.22); stroke-width:1; }
.radar-area { fill:rgba(179,136,255,0.16); stroke:#B388FF; stroke-width:2; stroke-linejoin:round; }
.radar-dot { fill:#B388FF; }
.radar-label { fill:var(--text-secondary); font-size:13px; font-family:'Inter','Segoe UI',sans-serif; }
.radar-label-val { fill:#B388FF; font-weight:700; font-size:13px; }
/* METHODOLOGY (последний экран) - чистые карточки БЕЗ box-shadow/filter
   (конвертер HTML->PDF рисует их квадратами). */
.tech-card { background:var(--bg-card); backdrop-filter:blur(20px); border:1px solid var(--border-color); border-radius:16px; padding:28px 32px 32px; margin-top:8px; }
.tech-title { font-size:1.25em; font-weight:700; margin-bottom:8px; color:var(--accent-purple); display:flex; align-items:center; gap:10px; }
.tech-intro { font-size:0.85em; color:var(--text-secondary); line-height:1.6; margin-bottom:22px; }
.tech-steps { display:grid; grid-template-columns:repeat(auto-fit,minmax(240px,1fr)); gap:16px; }
.tech-step { background:rgba(101,31,255,0.06); border-left:3px solid var(--accent-purple); border-radius:14px; padding:14px 18px; }
.tech-step-num { font-family:'Orbitron',monospace; font-weight:700; color:var(--accent-purple); font-size:0.9em; display:block; margin-bottom:6px; }
.tech-step-name { font-size:0.95em; font-weight:600; color:var(--text-primary); margin-bottom:5px; }
.tech-step-text { font-size:0.8em; color:var(--text-secondary); line-height:1.55; }
@keyframes fadeInDown { from{opacity:0;transform:translateY(-30px)} to{opacity:1;transform:translateY(0)} }
@keyframes fadeInUp { from{opacity:0;transform:translateY(30px)} to{opacity:1;transform:translateY(0)} }
@keyframes slideInLeft { from{opacity:0;transform:translateX(-20px)} to{opacity:1;transform:translateX(0)} }
@media (max-width:768px) {
  .container { padding:24px 16px; }
  .header-title { font-size:1.8em; }
  .main-index { padding:40px 24px; }
  .index-number { font-size:2.6em; }
  .cards-grid { grid-template-columns:1fr; gap:20px; }
  .index-circle { width:140px; height:140px; }
}

/* ===================== ПЕЧАТНАЯ ВЕРСИЯ (PDF / html2pdf.app) =====================
   Включается явно классом "print" на <body> только при конвертации HTML->PDF.
   Сохраняет ТЁМНЫЙ неоново-фиолетовый вид (полностью как в веб-
   предпросмотре «Мой профиль») - именно этот дизайн нужен пользователю.
   Добавляем лишь печатные правила, чтобы контент не обрезался и блоки не
   разрывались при конвертации в A4:
     - тёмный фон листа (background на @page и body);
     - @page A4 с полями;
     - все блоки цельные (page-break-inside:avoid / break-inside:avoid),
       чтобы карточки сфер и рекомендации не резались посередине.
   Форсированные разрывы страниц НЕ ставим - отчёт остаётся слитным тёмным
   потоком, «как было». Цвета колец/текста наследуются из дефолтных
   (тёмных) правил выше, поэтому менять их здесь не нужно. */
@page { size:A4; margin:0; }
body.print { background:#0B0A1A; color:var(--text-primary); padding:16mm 14mm; }
body.print .container { max-width:none; width:100%; }
body.print .header { margin-bottom:50px; page-break-inside:avoid; break-inside:avoid; }
body.print .main-index { page-break-inside:avoid; break-inside:avoid; }
body.print .metric-card { page-break-inside:avoid; break-inside:avoid; }
body.print .overview { page-break-inside:avoid; break-inside:avoid; }
body.print .chart-card { page-break-inside:avoid; break-inside:avoid; }
body.print .risk-zones { page-break-inside:avoid; break-inside:avoid; }
body.print .recommendations { page-break-inside:avoid; break-inside:avoid; }
body.print .recommendation-item { page-break-inside:avoid; break-inside:avoid; }
body.print .tech-card { page-break-inside:avoid; break-inside:avoid; }
`

// haClampScore ограничивает балл диапазоном 0-100.
func haClampScore(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// haRing возвращает длину окружности и целевое смещение штриха для
// кольцевого индикатора радиуса r и балла v.
// progress = circ * (1 - v/100): чем выше балл, тем меньше незаполненная
// часть кольца.
func haRing(r float64, v int) (circ, target string) {
	c := 2 * math.Pi * r
	t := c * (1 - float64(haClampScore(v))/100)
	return fmt.Sprintf("%.2f", c), fmt.Sprintf("%.2f", t)
}

// haScoreTier возвращает CSS-класс карточки, id градиента и текстовую метку
// уровня сферы образа жизни по её баллу (0-100).
func haScoreTier(s int) (cls, grad, label, color string) {
	switch {
	case s >= 80:
		return "score-excellent", "gradientGreen", "Отлично", "#00FF88"
	case s >= 65:
		return "score-good", "gradientCyan", "Хорошо", "#00D9FF"
	case s >= 50:
		return "score-moderate", "gradientYellow", "Требует внимания", "#FFB800"
	case s >= 35:
		return "score-poor", "gradientOrange", "Низкое качество", "#FF5C2D"
	default:
		return "score-critical", "gradientRed", "Критично", "#FF1A5C"
	}
}

// haIndexTier возвращает CSS-класс и текстовую метку общего индекса
// здоровья (0-100).
func haIndexTier(idx int) (cls, label, color string) {
	switch {
	case idx >= 80:
		return "status-excellent", "Отличный уровень", "#00FF88"
	case idx >= 50:
		return "status-moderate", "Средний уровень", "#FFB800"
	default:
		return "status-critical", "Низкий уровень", "#FF1A5C"
	}
}

// haRadarItem - точка для radar-диаграммы (подпись + балл 0-100).
type haRadarItem struct {
	label string
	score int
}

// haRadarChartHTML строит неоновую radar-диаграмму (паутинку) по сферам образа
// жизни. Только чистый SVG (кольца сетки, оси, область данных, точки, подписи) -
// без box-shadow/filter/drop-shadow, чтобы конвертер HTML->PDF не рисовал
// квадратных подложек позади фигур. Сам график строится инлайново из баллов.
//
// viewBox специально расширен (520x520, центр 260,260, радиус сетки 120),
// чтобы длинные подписи сфер («Вредные привычки») НЕ обрезались по краям
// SVG при рендере в браузере и в PDF (по умолчанию SVG клиппует всё вне
// viewBox). Балл выносится на отдельную строку под названием сферы, чтобы
// подпись читалась «Сон 58», а не склеивалась в «Сон58».
func haRadarChartHTML(items []haRadarItem) string {
	n := len(items)
	if n == 0 {
		return ""
	}
	cx, cy, R := 260.0, 260.0, 120.0
	labelR := R + 24.0
	axis := func(i int, r float64) (float64, float64) {
		ang := -math.Pi/2 + float64(i)*2*math.Pi/float64(n)
		return cx + r*math.Cos(ang), cy + r*math.Sin(ang)
	}
	var b strings.Builder
	b.WriteString("<div class=\"chart-card\"><div class=\"chart-title\">Профиль образа жизни</div>")
	b.WriteString("<svg viewBox=\"0 0 520 520\" class=\"radar\">")
	// Концентрические кольца сетки (25/50/75/100%).
	for _, lvl := range []float64{0.25, 0.5, 0.75, 1.0} {
		pts := make([]string, n)
		for i := 0; i < n; i++ {
			x, y := axis(i, R*lvl)
			pts[i] = fmt.Sprintf("%.1f,%.1f", x, y)
		}
		b.WriteString("<polygon class=\"radar-grid\" points=\"" + strings.Join(pts, " ") + "\"/>")
	}
	// Оси и подписи (название сферы + балл на отдельной строке).
	for i, it := range items {
		ex, ey := axis(i, R)
		b.WriteString(fmt.Sprintf("<line class=\"radar-axis\" x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\"/>", cx, cy, ex, ey))
		lx, ly := axis(i, labelR)
		anchor := "middle"
		if lx > cx+8 {
			anchor = "start"
		} else if lx < cx-8 {
			anchor = "end"
		}
		// Название сферы, а балл - второй строкой (dy), с тем же выравниванием.
		b.WriteString(fmt.Sprintf("<text class=\"radar-label\" x=\"%.1f\" y=\"%.1f\" text-anchor=\"%s\">%s<tspan class=\"radar-label-val\" x=\"%.1f\" dy=\"15\">%d</tspan></text>",
			lx, ly, anchor, esc(it.label), lx, haClampScore(it.score)))
	}
	// Область данных (многоугольник по баллам).
	dpts := make([]string, n)
	for i, it := range items {
		x, y := axis(i, R*float64(haClampScore(it.score))/100)
		dpts[i] = fmt.Sprintf("%.1f,%.1f", x, y)
	}
	b.WriteString("<polygon class=\"radar-area\" points=\"" + strings.Join(dpts, " ") + "\"/>")
	// Точки данных.
	for i, it := range items {
		x, y := axis(i, R*float64(haClampScore(it.score))/100)
		b.WriteString(fmt.Sprintf("<circle class=\"radar-dot\" cx=\"%.1f\" cy=\"%.1f\" r=\"4\"/>", x, y))
	}
	b.WriteString("</svg></div>")
	return b.String()
}

// riskLevelColorCSS возвращает CSS-цвет для уровня зоны риска, переиспользуя
// общий riskLevelColor (из pdf.go), который возвращает RGB-тройку.
func riskLevelColorCSS(level string) string {
	c := riskLevelColor(level)
	return fmt.Sprintf("rgb(%d,%d,%d)", c[0], c[1], c[2])
}

// RenderHealthAssessmentHTML строит дашборд «Общая оценка здоровья» из
// структурированных данных ИИ (без медицинских файлов): шапка с именем
// пользователя и датой, кольцевой индекс здоровья-герой, карточки 5 сфер
// образа жизни (Сон, Питание, Самочувствие, Стресс, Вредные привычки) и блок
// персональных рекомендаций на 3 месяца.
//
// При forPrint=true на <body> ставится class="print", включающий тёмную
// печатную вёрстку (A4, тёмный фон листа, цельные блоки без разрезания) -
// используется при конвертации HTML->PDF. При forPrint=false применяется тот
// же тёмный неон для веб-предпросмотра в «Моём профиле» (без печатных правил).
// В обоих случаях дизайн тёмный неоново-фиолетовый.
func RenderHealthAssessmentHTML(ha models.HealthAssessment, date time.Time, forPrint bool) string {
	idx := haClampScore(ha.HealthIndex)
	idxCls, idxLabel, idxColor := haIndexTier(idx)

	// Приоритетная сфера для улучшения (низший балл) - короткая подсказка
	// в центре под кольцом индекса.
	indexDesc := "Баланс по сферам образа жизни."
	type sphere struct{ key, label string }
	sphereOrder := []sphere{
		{"sleep", "Сон"},
		{"nutrition", "Питание"},
		{"wellbeing", "Самочувствие"},
		{"stress", "Стресс"},
		{"habits", "Вредные привычки"},
	}
	type cardT struct {
		label, statusText, comment, grad, cls, color string
		score                                        int
	}
	cards := make([]cardT, 0, len(sphereOrder))
	low := -1
	for _, sp := range sphereOrder {
		sc := 0
		var comment string
		if dim, ok := ha.Lifestyle[sp.key]; ok {
			sc = haClampScore(dim.Score)
			comment = strings.TrimSpace(dim.Comment)
		}
		g, c, l, col := haScoreTier(sc)
		cards = append(cards, cardT{sp.label, l, comment, g, c, col, sc})
		if low < 0 || sc < cards[low].score {
			low = len(cards) - 1
		}
	}
	if low >= 0 {
		indexDesc = "Приоритет улучшения: " + cards[low].label + " (" + cards[low].statusText + ")"
	}

	// Точки для radar-диаграммы на первом экране.
	items := make([]haRadarItem, 0, len(cards))
	for _, c := range cards {
		items = append(items, haRadarItem{c.label, c.score})
	}

	// Шапка: имя (если есть) + дата.
	sub := "Отчёт " + date.Format("02.01.2006")
	if name := strings.TrimSpace(ha.Name); name != "" {
		sub = name + " | " + sub
	}

	idxCirc, idxTarget := haRing(60, idx)

	bodyClass := ""
	if forPrint {
		bodyClass = " class=\"print\""
	}

	var b strings.Builder
	b.WriteString("<!doctype html><html lang=\"ru\"><head><meta charset=\"utf-8\">")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">")
	b.WriteString("<title>Prisma · Общая оценка здоровья</title>")
	b.WriteString("<link href=\"https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=Orbitron:wght@400;700;900&display=swap\" rel=\"stylesheet\">")
	b.WriteString("<style>" + healthDashboardCSS + "</style></head><body" + bodyClass + ">")
	b.WriteString("<div class=\"container\">")

	// Заголовок.
	b.WriteString("<div class=\"header\"><div class=\"header-title\"><span>Prisma · Общая оценка здоровья</span></div>")
	b.WriteString("<div class=\"header-subtitle\">" + esc(sub) + "</div></div>")

	// Главный индекс здоровья.
	b.WriteString("<div class=\"main-index\"><div class=\"index-content\">")
	b.WriteString("<div class=\"index-circle-container\"><div class=\"index-circle\">")
	b.WriteString("<svg viewBox=\"0 0 200 200\">")
	b.WriteString("<circle class=\"index-circle-bg\" cx=\"100\" cy=\"100\" r=\"60\"></circle>")
	b.WriteString("<circle class=\"index-circle-progress\" cx=\"100\" cy=\"100\" r=\"60\" style=\"stroke:" + idxColor + ";stroke-dasharray:" + idxCirc + ";stroke-dashoffset:" + idxTarget + "\"></circle>")
	b.WriteString("</svg><div class=\"index-value\"><div class=\"index-number\">" + fmt.Sprintf("%d", idx) + "</div></div>")
	b.WriteString("</div></div>")
	b.WriteString("<div class=\"index-status " + idxCls + "\">" + idxLabel + "</div>")
	b.WriteString("<div class=\"index-description\">" + esc(indexDesc) + "</div>")
	b.WriteString("</div></div>")

	// Неоновая radar-диаграмма по сферам образа жизни - сразу на первом экране.
	b.WriteString(haRadarChartHTML(items))

	// Подробный обзор (summary) - развёрнутый разбор образа жизни.
	if summary := strings.TrimSpace(ha.Summary); summary != "" {
		b.WriteString("<div class=\"overview\"><div class=\"overview-title\">Общая картина</div>")
		b.WriteString("<div class=\"overview-text\">" + esc(summary) + "</div></div>")
	}

	// Карточки сфер.
	b.WriteString("<div class=\"cards-grid\">")
	for _, c := range cards {
		cv, ct := haRing(50, c.score)
		b.WriteString("<div class=\"metric-card " + c.cls + "\">")
		b.WriteString("<div class=\"card-header\"><div class=\"card-title\"><span>" + esc(c.label) + "</span></div></div>")
		b.WriteString("<div class=\"card-circle-container\"><div class=\"card-circle\">")
		b.WriteString("<svg viewBox=\"0 0 120 120\">")
		b.WriteString("<circle class=\"card-circle-bg\" cx=\"60\" cy=\"60\" r=\"50\"></circle>")
		b.WriteString("<circle class=\"card-circle-progress\" cx=\"60\" cy=\"60\" r=\"50\" style=\"stroke:" + c.color + ";stroke-dasharray:" + cv + ";stroke-dashoffset:" + ct + "\"></circle>")
		b.WriteString("</svg><div class=\"card-value\">" + fmt.Sprintf("%d", c.score) + "</div>")
		b.WriteString("</div></div>")
		b.WriteString("<div class=\"card-status " + c.cls + "\">" + esc(c.statusText) + "</div>")
		if c.comment != "" {
			b.WriteString("<div class=\"card-text\">" + esc(c.comment) + "</div>")
		}
		b.WriteString("</div>")
	}
	b.WriteString("</div>")

	// Зоны внимания (risk zones) - красные флаги из ответов опросника.
	if len(ha.RiskZones) > 0 {
		b.WriteString("<div class=\"risk-zones\"><div class=\"risk-zones-title\"><span>Зоны внимания</span></div>")
		b.WriteString("<div class=\"risk-list\">")
		for _, rz := range ha.RiskZones {
			lvl := strings.TrimSpace(rz.Level)
			b.WriteString("<div class=\"risk-item\">")
			b.WriteString("<div class=\"risk-head\"><span class=\"risk-name\">" + esc(rz.Name) + "</span>")
			if lvl != "" {
				b.WriteString("<span class=\"risk-level\" style=\"color:" + riskLevelColorCSS(lvl) + "\">" + esc(lvl) + "</span>")
			}
			b.WriteString("</div>")
			if d := strings.TrimSpace(rz.Description); d != "" {
				b.WriteString("<div class=\"risk-desc\">" + esc(d) + "</div>")
			}
			b.WriteString("</div>")
		}
		b.WriteString("</div></div>")
	}

	// Персональные рекомендации (план на 3 месяца).
	planItems := []struct {
		label, text string
	}{
		{"Сон", ha.Plan.Sleep},
		{"Питание", ha.Plan.Nutrition},
		{"Общее самочувствие", ha.Plan.Wellbeing},
		{"Стресс", ha.Plan.Stress},
	}
	hasPlan := false
	for _, p := range planItems {
		if strings.TrimSpace(p.text) != "" {
			hasPlan = true
			break
		}
	}
	if hasPlan {
		b.WriteString("<div class=\"recommendations\"><div class=\"recommendations-title\"><span>Персональные рекомендации</span></div>")
		b.WriteString("<div class=\"recommendations-list\">")
		n := 0
		for _, p := range planItems {
			if strings.TrimSpace(p.text) == "" {
				continue
			}
			n++
			b.WriteString("<div class=\"recommendation-item\"><div class=\"recommendation-number\">" + fmt.Sprintf("%d", n) + "</div>")
			b.WriteString("<div class=\"recommendation-text\"><span class=\"recommendation-emphasis\">" + esc(p.label) + ":</span> " + esc(p.text) + "</div></div>")
		}
		b.WriteString("</div></div>")
	}

	// Зоны внимания (балл < 65 - умеренное и ниже качество), отсортированные
	// по возрастанию балла, чтобы персонализировать последний шаг блока
	// «Технология разбора PRISMA» конкретными выявленными зонами.
	weak := make([]cardT, 0, len(cards))
	for _, c := range cards {
		if c.score < 65 {
			weak = append(weak, c)
		}
	}
	sort.Slice(weak, func(i, j int) bool { return weak[i].score < weak[j].score })
	weakZones := make([]string, 0, len(weak))
	for _, w := range weak {
		weakZones = append(weakZones, w.label)
	}
	if len(weakZones) == 0 && len(cards) > 0 {
		// Все сферы - отличные: выделяем самую низкую для поддержания.
		minC := cards[0]
		for _, c := range cards[1:] {
			if c.score < minC.score {
				minC = c
			}
		}
		weakZones = append(weakZones, minC.label)
	}

	// Блок «Технология разбора PRISMA» - на последнем экране отчёта.
	b.WriteString(haMethodologyHTML(weakZones))

	b.WriteString("</div></body></html>")
	return b.String()
}

// haMethodologyStep - шаг аналитического конвейера PRISMA.
type haMethodologyStep struct {
	num, name, text string
}

// haMethodologyHTML строит блок «Технология разбора PRISMA» для последнего
// экрана отчёта. Чистая вёрстка (карточки БЕЗ box-shadow/filter) в едином
// тёмном неоновом стиле - описывает профессиональный конвейер разбора
// образа жизни аналитическим движком Prisma. weakZones - названия сфер с
// пониженным баллом (отсортированные по возрастанию), которые подставляются
// в последний шаг, чтобы план выглядел привязанным к реальным данным
// пользователя (персонализация блока).
func haMethodologyHTML(weakZones []string) string {
	// Персональная часть последнего шага: конкретные выявленные зоны
	// внимания, чтобы план не выглядел шаблонным.
	zonesText := "привязанные к выявленным зонам"
	if len(weakZones) > 0 {
		zonesText = "привязанные к выявленным зонам: " + strings.Join(weakZones, ", ")
	}
	steps := []haMethodologyStep{
		{"01", "Сбор данных", "Опросник фиксирует привычки по сну, питанию, активности, стрессу и образу жизни в единой структуре."},
		{"02", "Нормализация", "Ответы переводятся в баллы 0-100 по калиброванным шкалам безопасности без искажения исходных данных."},
		{"03", "Профиль сфер", "Строится многомерный профиль, который выделяет сильные стороны и зоны повышенного внимания."},
		{"04", "Индекс здоровья", "Сферы сворачиваются в общий показатель с приоритизацией самых слабых направлений."},
		{"05", "Персональный план", "Формируются рекомендации на 3 месяца, " + zonesText + "."},
	}
	var b strings.Builder
	b.WriteString("<div class=\"tech-card\">")
	b.WriteString("<div class=\"tech-title\"><span>Технология разбора PRISMA</span></div>")
	b.WriteString("<div class=\"tech-intro\">Аналитический движок Prisma оценивает образ жизни как единую систему показателей. " +
		"Каждая сфера измеряется по собственным нормам, затем сводится в единый индекс здоровья и персональный план действий.</div>")
	b.WriteString("<div class=\"tech-steps\">")
	for _, s := range steps {
		b.WriteString("<div class=\"tech-step\">")
		b.WriteString("<span class=\"tech-step-num\">" + s.num + "</span>")
		b.WriteString("<div class=\"tech-step-name\">" + esc(s.name) + "</div>")
		b.WriteString("<div class=\"tech-step-text\">" + esc(s.text) + "</div>")
		b.WriteString("</div>")
	}
	b.WriteString("</div></div>")
	return b.String()
}

// esc - экранирование спецсимволов HTML.
func esc(s string) string {
	return html.EscapeString(s)
}
