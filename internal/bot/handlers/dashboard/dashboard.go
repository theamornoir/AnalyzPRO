package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/theamornoir/analyzpro/internal/locales"
	apmodels "github.com/theamornoir/analyzpro/internal/models"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/payment"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/report/pdfservice"
)

// MetricsResponse — ответ API для дашборда «Сводка здоровья».
// Данные берутся из реальной истории пользователя (monitoring.Repository),
// а не из моков.
type MetricsResponse struct {
	UserName        string    `json:"userName"`
	UserAge         int       `json:"userAge"`
	NoData          bool      `json:"noData"`
	PremiumRequired bool      `json:"premiumRequired"`
	HealthIndex     int       `json:"healthIndex"`
	EnergyLevel     string    `json:"energyLevel"`
	AnalysisDate    string    `json:"analysisDate"`
	Blood           BloodData `json:"blood"`
	Nutrition       NutData   `json:"nutrition"`
	Activity        ActData   `json:"activity"`
	Trend           TrendData `json:"trend"`
	Recommendations []string  `json:"recommendations"`
}

type BloodData struct {
	Hemoglobin int     `json:"hemoglobin"`
	Leukocytes float64 `json:"leukocytes"`
	Platelets  int     `json:"platelets"`
}

type NutData struct {
	Protein int `json:"protein"`
	Carbs   int `json:"carbs"`
	Fat     int `json:"fat"`
}

type ActData struct {
	Steps    int     `json:"steps"`
	Calories int     `json:"calories"`
	Water    float64 `json:"water"`
}

type TrendData struct {
	Labels []string `json:"labels"`
	Values []int    `json:"values"`
}

// Handler — HTTP-обработчики дашборда «Сводка здоровья».
// Премиум-гейт реализован на уровне API /api/metrics через проверку
// подлинности Telegram initData + статуса Premium пользователя.
type Handler struct {
	pay            *payment.MockPaymentService
	botToken       string
	repo           monitoring.Repository
	reportRenderer *report.Renderer
	pdfConverter   pdfservice.Converter
}

// NewHandler создаёт обработчик дашборда. reportRenderer/pdfConverter нужны,
// чтобы по запросу отдавать сохранённые отчёты как PDF (/api/reports/file).
func NewHandler(pay *payment.MockPaymentService, botToken string, repo monitoring.Repository, reportRenderer *report.Renderer, pdfConverter pdfservice.Converter) *Handler {
	return &Handler{pay: pay, botToken: botToken, repo: repo, reportRenderer: reportRenderer, pdfConverter: pdfConverter}
}

// ServeWebApp отдаёт статический веб-дашборд из embed-файлов.
// Премиум-проверка здесь НЕ нужна: сама страница не содержит данных,
// данные грузит /api/metrics (который и требует Premium).
func (h *Handler) ServeWebApp(w http.ResponseWriter, r *http.Request) {
	// Telegram WebView агрессивно кэширует статики Mini App и неохотно
	// отдаёт свежие файлы (отсюда «пустой/старый» дашборд после правок).
	// Запрещаем кэширование и поддерживаем ?v= для явного сброса по URL.
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	// Убираем префикс /dashboard/
	filePath := r.URL.Path
	if filePath == "/dashboard/" || filePath == "/" {
		filePath = "index.html"
	} else {
		filePath = strings.TrimPrefix(filePath, "/dashboard/")
	}

	// Отрезаем query (?v=...) — файл ищем по чистому имени.
	if i := strings.IndexByte(filePath, '?'); i >= 0 {
		filePath = filePath[:i]
	}

	switch filePath {
	case "index.html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case "style.css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case "app.js", "data.js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	default:
		http.NotFound(w, r)
		return
	}

	data, err := webappFS.ReadFile("webapp_files/" + filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Write(data)
}

// Metrics — обработчик GET /api/metrics. Возвращает реальные метрики
// пользователя, извлечённые из его истории анализов/биосканов.
//
// Доступ только для Premium-пользователей: проверяем initData (подпись
// Telegram) и статус Premium из paymentService. Без Premium → 403.
func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")

	// Демо-режим: возвращаем синтетические «полностью заполненные» метрики,
	// чтобы можно было посмотреть графики Сводки здоровья без реальных
	// анализов и без Premium. Работает и без валидного initData (удобно для
	// локальной отладки в браузере). Премиум-гейт не применяется.
	if r.URL.Query().Get("demo") == "1" {
		log.Printf("[DASHBOARD] /api/metrics (DEMO) отданы синтетические метрики")
		_ = json.NewEncoder(w).Encode(h.buildDemoMetrics())
		return
	}

	initData := r.URL.Query().Get("initData")
	if initData == "" {
		initData = r.Header.Get("X-Telegram-Init-Data")
	}

	telegramID, ok := monitoring.ValidateInitData(initData, h.botToken)
	if !ok || telegramID == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	metrics := h.buildMetrics(r.Context(), telegramID)

	// Премиум-гейт: «богатые» метрики (кровь/питание/активность/тренды)
	// доступны только Premium. Но ОНБОРДИНГ (регистрация профиля) — для
	// ВСЕХ: при отсутствии данных не-Premium пользователь тоже видит форму
	// заполнения, а не пустой экран с «нужна Premium». Поэтому не отдаём
	// 403, а помечаем premiumRequired и скрываем rich-поля, оставляя
	// noData/имя профиля — чтобы Mini App мог показать карточку регистрации.
	isPremium := h.pay.IsPremium(telegramID)
	metrics.PremiumRequired = !isPremium
	if !isPremium {
		metrics.HealthIndex = 0
		metrics.EnergyLevel = "—"
		metrics.Blood = BloodData{}
		metrics.Nutrition = NutData{}
		metrics.Activity = ActData{}
		metrics.Trend = TrendData{}
	}

	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		log.Printf(locales.LogAPIEncodeError, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	log.Printf("[DASHBOARD] /api/metrics отданы для user=%d (noData=%v)", telegramID, metrics.NoData)
}

// Reports — обработчик GET /api/reports. Возвращает последний и предыдущий
// сохранённые отчёты расширенного анализа и Bioscan PRO (из истории
// пользователя) вместе с вычисленной дельтой индекса — чтобы дашборд
// «Сводка здоровья» мог показать графики последнего отчёта и сравнение
// прогресса с предыдущим.
//
// Доступ — как и у /api/metrics: валидация initData (подпись Telegram) +
// Premium-гейт. Без Premium «богатые» данные скрываются (как в /api/metrics).
func (h *Handler) Reports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")

	// Демо-режим (?demo=1): синтетические последний+предыдущий отчёты, чтобы
	// можно было посмотреть карточки «Расширенные анализы» и «Bioscan PRO» и
	// сравнение прогресса без реальных данных и без Premium.
	if r.URL.Query().Get("demo") == "1" {
		log.Printf("[DASHBOARD] /api/reports (DEMO) отданы синтетические отчёты")
		_ = json.NewEncoder(w).Encode(h.buildDemoReports())
		return
	}

	initData := r.URL.Query().Get("initData")
	if initData == "" {
		initData = r.Header.Get("X-Telegram-Init-Data")
	}
	telegramID, ok := monitoring.ValidateInitData(initData, h.botToken)
	if !ok || telegramID == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	data := h.buildReportsData(r.Context(), telegramID)

	// Премиум-гейт: «богатые» данные (индексы/зоны/индикаторы) — только для
	// Premium. Сам факт наличия отчётов и количество (Count) остаются видны.
	isPremium := h.pay.IsPremium(telegramID)
	data.PremiumRequired = !isPremium
	if !isPremium {
		hideGroup := func(g *ReportsGroup) {
			g.Latest.Scores = map[string]int{}
			g.Latest.Zones = nil
			g.Latest.Indicators = nil
			g.Latest.MainScore = 0
			g.Latest.Summary = ""
			g.Previous = ReportBlock{}
			g.Delta = 0
			g.HasComparison = false
		}
		hideGroup(&data.Analysis)
		hideGroup(&data.Bioscan)
	}

	_ = json.NewEncoder(w).Encode(data)
}

// ReportFile — обработчик GET /api/reports/file. Отдаёт сохранённый отчёт
// (расширенный анализ или Bioscan PRO) как PDF-файл для просмотра/скачивания
// прямо из «Сводки здоровья». По id записи из истории берёт сохранённый
// ReportHTML либо перерендеривает HTML из JsonData и конвертирует в PDF через
// pdfservice (html2pdf.app). При недоступности PDF-конвертера — отдаёт сам
// HTML (отчёт не теряется).
//
// Доступ — по подписи initData (проверка подлинности Telegram) + проверке
// владения записью. ПРЕМИУМ-ГЕЙТ НЕ ПРИМЕНЯЕТСЯ: пользователь открывает
// свой собственный сохранённый отчёт, это его данные, и клик по карточке
// отчёта в «Сводке здоровья» должен открывать файл независимо от тарифа
// (иначе кнопка «📄 PDF» ничего не делает для не-Premium).
func (h *Handler) ReportFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")

	// ДЕМО-режим (?demo=1): отдаём синтетический отчёт БЕЗ проверки
	// подлинности сессии и БЕЗ поиска записи в БД (в демо реальных
	// отчётов нет). Позволяет открывать демо-отчёты прямо из Сводки.
	if r.URL.Query().Get("demo") == "1" {
		entryType := r.URL.Query().Get("type")
		if entryType != "analysis" && entryType != "bioscan" {
			http.Error(w, "bad type", http.StatusBadRequest)
			return
		}
		html := h.buildDemoReportHTML(entryType)
		if strings.TrimSpace(html) == "" {
			http.Error(w, "render error", http.StatusInternalServerError)
			return
		}
		filename := "Demo_" + entryType + "_report"
		// PDF если ключ html2pdf.app доступен, иначе сам HTML (inline).
		pdfBytes, convErr := h.pdfConverter.ConvertHTML(r.Context(), html)
		if convErr != nil {
			log.Printf("[DASHBOARD] PDF недоступен для демо-отчёта (%v) — отдаю HTML", convErr)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename+".html"))
			w.Write([]byte(html))
			return
		}
		log.Printf("[DASHBOARD] демо PDF-отчёт отдан: %d байт", len(pdfBytes))
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename+".pdf"))
		w.Write(pdfBytes)
		return
	}

	initData := r.URL.Query().Get("initData")
	if initData == "" {
		initData = r.Header.Get("X-Telegram-Init-Data")
	}
	telegramID, ok := monitoring.ValidateInitData(initData, h.botToken)
	if !ok || telegramID == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	entryType := r.URL.Query().Get("type")
	if entryType != "analysis" && entryType != "bioscan" {
		http.Error(w, "bad type", http.StatusBadRequest)
		return
	}
	entryID, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil || entryID <= 0 {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	entry, err := h.repo.GetHistoryEntry(r.Context(), entryID)
	if err != nil || entry == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	// Защита: запись принадлежит этому пользователю.
	if entry.TelegramID != telegramID || entry.Type != entryType {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	html, renderErr := h.reportHTML(entry)
	if renderErr != nil || strings.TrimSpace(html) == "" {
		log.Printf("[DASHBOARD] не удалось получить HTML отчёта id=%d: %v", entryID, renderErr)
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}

	filename := "Report_" + entryType + "_" + entry.Date.Format("2006-01-02")

	// Конвертация в PDF. При ошибке (нет ключа html2pdf.app / сервис
	// недоступен) — откат к отдаче самого HTML, чтобы отчёт не потерялся.
	pdfBytes, convErr := h.pdfConverter.ConvertHTML(r.Context(), html)
	if convErr != nil {
		log.Printf("[DASHBOARD] PDF-конвертация недоступна для отчёта id=%d: %v — отдаю HTML", entryID, convErr)
		// ВАЖНО: Content-Disposition=inline (НЕ attachment). В Telegram
		// WebView attachment триггерит скачивание, которое в браузере
		// Mini App не открывается (пользователь видит «ничего не произошло»).
		// inline рендерит HTML-страницу отчёта прямо во встроенном
		// просмотрщике (iframe) или in-app браузере.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename+".html"))
		w.Write([]byte(html))
		return
	}

	log.Printf("[DASHBOARD] PDF-отчёт id=%d отдан пользователю (user=%d): %d байт", entryID, telegramID, len(pdfBytes))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename+".pdf"))
	w.Write(pdfBytes)
}

// reportHTML возвращает print-ready HTML сохранённого отчёта: если есть
// готовый ReportHTML — берёт его, иначе перерендеривает из JsonData через
// report.Renderer (в зависимости от типа записи).
func (h *Handler) reportHTML(entry *monitoring.HistoryEntry) (string, error) {
	if strings.TrimSpace(entry.ReportHTML) != "" {
		return entry.ReportHTML, nil
	}
	cleaned := strings.TrimSpace(entry.JsonData)
	if cleaned == "" {
		return "", fmt.Errorf("пустой отчёт")
	}
	if entry.Type == "bioscan" {
		var bs apmodels.BodyScanReport
		if err := json.Unmarshal([]byte(cleaned), &bs); err != nil {
			return "", err
		}
		return h.reportRenderer.RenderBodyScan(bs)
	}
	// analysis: либо models.Report (расширенный анализ), либо
	// models.HealthDossier (досье). Пробуем оба варианта.
	var rep apmodels.Report
	if err := json.Unmarshal([]byte(cleaned), &rep); err == nil {
		if html, rerr := h.reportRenderer.Render(rep); rerr == nil && strings.TrimSpace(html) != "" {
			return html, nil
		}
	}
	var dossier apmodels.HealthDossier
	if err := json.Unmarshal([]byte(cleaned), &dossier); err == nil {
		if html, derr := h.reportRenderer.RenderDossier(dossier); derr == nil && strings.TrimSpace(html) != "" {
			return html, nil
		}
	}
	return "", fmt.Errorf("не удалось перерендерить отчёт")
}

// buildDemoReportHTML возвращает синтетический HTML-отчёт для демо-режима
// (кнопка «📄 PDF» / клик по архиву в демо-Сводке). Не зависит от шаблонов
// report.Renderer и не требует записи в БД — просто аккуратный демо-документ,
// который можно открыть как HTML (или PDF при наличии ключа html2pdf.app).
func (h *Handler) buildDemoReportHTML(kind string) string {
	esc := func(s string) string {
		return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;").Replace(s)
	}
	title := "Расширенный анализ"
	date := time.Now().Format("2006-01-02")
	extraHead := "<th>Статус</th>"
	rows := ""
	if kind == "bioscan" {
		title = "Bioscan PRO"
		extraHead = ""
		zones := [][2]string{{"Плечи", "88"}, {"Пресс", "74"}, {"Ноги", "80"}, {"Осанка", "84"}, {"Таз", "78"}, {"Позвоночник", "86"}}
		for _, z := range zones {
			rows += fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>", esc(z[0]), esc(z[1]))
		}
	} else {
		inds := [][3]string{
			{"Гемоглобин", "152 г/л", "норма"},
			{"Глюкоза", "5.2 ммоль/л", "норма"},
			{"Холестерин", "5.1 ммоль/л", "внимание"},
		}
		for _, i := range inds {
			rows += fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%s</td></tr>", esc(i[0]), esc(i[1]), esc(i[2]))
		}
	}
	html := `<!doctype html>
<html lang="ru"><head><meta charset="utf-8">
<title>` + title + ` (демо)</title>
<style>
body{font-family:system-ui,-apple-system,Segoe UI,Roboto,sans-serif;max-width:720px;margin:32px auto;padding:0 16px;color:#1a2330}
h1{color:#1FA6A8} .sub{color:#6b7785;margin-bottom:24px}
table{width:100%;border-collapse:collapse;margin-top:12px}
td,th{text-align:left;padding:8px 10px;border-bottom:1px solid #e3e8ef}
.demo{margin-top:24px;padding:12px 14px;background:#f4f8fa;border-radius:12px;color:#6b7785}
</style></head><body>
<h1>` + title + `</h1>
<div class="sub">Демонстрационный отчёт · ` + date + `</div>
<table><thead><tr><th>Показатель</th><th>Значение</th>` + extraHead + `</tr></thead>
<tbody>` + rows + `</tbody></table>
<div class="demo">Это демонстрационный отчёт Prisma. Чтобы открыть реальный PDF-отчёт, загрузите анализ или пройдите Bioscan PRO в боте.</div>
</body></html>`
	return html
}

// buildDemoReports — синтетические «последний» и «предыдущий» отчёты для
// демо-режима дашборда: показывает, как выглядят карточки Расширенного
// анализа и Bioscan PRO с графиками и сравнением прогресса.
func (h *Handler) buildDemoReports() ReportsResponse {
	latestA := ReportBlock{
		ID:         1,
		Available:  true,
		Title:      "Расширенный анализ",
		Date:       time.Now().Format("2006-01-02"),
		MainScore:  78,
		ScoreLabel: "Индекс здоровья",
		Scores:     map[string]int{"Композиция": 74, "Мышцы": 80, "Баланс": 70, "Потенциал": 85},
		Indicators: []IndicatorView{
			{Name: "Гемоглобин", Value: "152 г/л", Status: "normal"},
			{Name: "Глюкоза", Value: "5.2 ммоль/л", Status: "normal"},
			{Name: "Холестерин", Value: "5.1 ммоль/л", Status: "warning"},
		},
		Summary: "Показатели улучшились по сравнению с предыдущим анализом.",
		Comparison: ComparisonView{
			Summary:   "Общая динамика положительная: гемоглобин и глюкоза вернулись в норму.",
			Improved:  []string{"Гемоглобин 145→152", "Глюкоза 6.8→5.2"},
			Unchanged: []string{"Холестерин остаётся повышенным"},
			Worsened:  []string{},
			ToImprove: []string{"Снизить холестерин: меньше насыщенных жиров, кардионагрузки"},
			Metrics: []ComparisonMetricView{
				{Name: "Гемоглобин", Before: "145 г/л", After: "152 г/л", Change: "+7", Trend: "up"},
				{Name: "Глюкоза", Before: "6.8", After: "5.2", Change: "-1.6", Trend: "up"},
				{Name: "Холестерин", Before: "5.9", After: "5.1", Change: "-0.8", Trend: "up"},
			},
		},
	}
	prevA := ReportBlock{
		ID:         2,
		Available:  true,
		Title:      "Расширенный анализ",
		Date:       time.Now().AddDate(0, -1, 0).Format("2006-01-02"),
		MainScore:  71,
		ScoreLabel: "Индекс здоровья",
		Scores:     map[string]int{"Композиция": 68, "Мышцы": 76, "Баланс": 66, "Потенциал": 80},
	}
	olderA := ReportBlock{
		ID:         3,
		Available:  true,
		Title:      "Расширенный анализ",
		Date:       time.Now().AddDate(0, -3, 0).Format("2006-01-02"),
		MainScore:  64,
		ScoreLabel: "Индекс здоровья",
		Scores:     map[string]int{"Композиция": 60, "Мышцы": 70, "Баланс": 62, "Потенциал": 74},
	}

	latestB := ReportBlock{
		ID:         10,
		Available:  true,
		Title:      "Bioscan PRO",
		Date:       time.Now().Format("2006-01-02"),
		MainScore:  86,
		ScoreLabel: "Body Score",
		Scores:     map[string]int{"Осанка": 84, "Симметрия": 82, "Плечи": 80, "Таз": 78, "Позвоночник": 86, "Мобильность": 74, "Стабильность": 81},
		Zones: []ZoneView{
			{Name: "Плечи", Score: 88, Status: "good", Comment: "Сбалансированное развитие."},
			{Name: "Пресс", Score: 74, Status: "warning", Comment: "Нижняя часть требует проработки."},
			{Name: "Ноги", Score: 80, Status: "good", Comment: "Крепкие квадрицепсы."},
		},
		Summary: "Телосложение укрепилось, осанка выровнена.",
		Comparison: ComparisonView{
			Summary:   "Body Score вырос на 4 пункта за 8 недель тренировок.",
			Improved:  []string{"Пресс 64→74", "Осанка 80→84"},
			Unchanged: []string{"Плечи стабильно сильные"},
			Worsened:  []string{},
			ToImprove: []string{"Подтянуть мобильность таза и нижний пресс"},
			Metrics: []ComparisonMetricView{
				{Name: "Body Score", Before: "82", After: "86", Change: "+4", Trend: "up"},
				{Name: "Осанка", Before: "80", After: "84", Change: "+4", Trend: "up"},
				{Name: "Пресс", Before: "64", After: "74", Change: "+10", Trend: "up"},
			},
		},
	}
	prevB := ReportBlock{
		ID:         11,
		Available:  true,
		Title:      "Bioscan PRO",
		Date:       time.Now().AddDate(0, -2, 0).Format("2006-01-02"),
		MainScore:  82,
		ScoreLabel: "Body Score",
		Scores:     map[string]int{"Осанка": 80, "Симметрия": 78, "Плечи": 79, "Таз": 75, "Позвоночник": 83, "Мобильность": 70, "Стабильность": 80},
	}
	olderB := ReportBlock{
		ID:         12,
		Available:  true,
		Title:      "Bioscan PRO",
		Date:       time.Now().AddDate(0, -4, 0).Format("2006-01-02"),
		MainScore:  77,
		ScoreLabel: "Body Score",
		Scores:     map[string]int{"Осанка": 76, "Симметрия": 74, "Плечи": 77, "Таз": 72, "Позвоночник": 79, "Мобильность": 66, "Стабильность": 77},
	}

	return ReportsResponse{
		PremiumRequired: false,
		Analysis: ReportsGroup{
			Count:         3,
			HasComparison: true,
			Delta:         7,
			Reports:       []ReportBlock{latestA, prevA, olderA},
			Latest:        latestA,
			Previous:      prevA,
		},
		Bioscan: ReportsGroup{
			Count:         3,
			HasComparison: true,
			Delta:         4,
			Reports:       []ReportBlock{latestB, prevB, olderB},
			Latest:        latestB,
			Previous:      prevB,
		},
	}
}

// ProfileRequest — тело запроса регистрации профиля из Mini App «Сводка
// здоровья». Минимальный набор полей, чтобы дашборд перестал быть пустым.
type ProfileRequest struct {
	Name   string `json:"name"`
	Age    int    `json:"age"`
	Gender string `json:"gender"`
	Height int    `json:"height"`
	Weight int    `json:"weight"`
	Goal   string `json:"goal"`
}

// SaveProfile — обработчик POST /api/profile. Принимает минимальный профиль
// пользователя (регистрация) из Mini App и сохраняет его как запись истории
// типа "questionnaire", чтобы дашборд при следующем открытии был непустым.
//
// Премиум-гейт НЕ применяется: профиль можно заполнить на любом тарифе.
// Выдача самих метрик (/api/metrics) по-прежнему требует Premium.
func (h *Handler) SaveProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	initData := r.URL.Query().Get("initData")
	if initData == "" {
		initData = r.Header.Get("X-Telegram-Init-Data")
	}
	telegramID, ok := monitoring.ValidateInitData(initData, h.botToken)
	if !ok || telegramID == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	var req ProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
		return
	}

	name := strings.TrimSpace(req.Name)
	if utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 50 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid name"})
		return
	}
	if req.Age < 5 || req.Age > 90 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid age"})
		return
	}

	gender := strings.TrimSpace(req.Gender)
	switch strings.ToLower(gender) {
	case "мужской", "м", "male":
		gender = "Мужской"
	case "женский", "ж", "female":
		gender = "Женский"
	default:
		gender = ""
	}

	profile := map[string]interface{}{
		"name":   name,
		"age":    req.Age,
		"gender": gender,
	}
	if req.Height > 0 {
		profile["height"] = req.Height
	}
	if req.Weight > 0 {
		profile["weight"] = req.Weight
	}

	payload := map[string]interface{}{
		"profile":         profile,
		"recommendations": profileRecommendations(strings.TrimSpace(req.Goal)),
	}
	payloadBytes, _ := json.Marshal(payload)

	entry := &monitoring.HistoryEntry{
		TelegramID: telegramID,
		Type:       "questionnaire",
		Title:      "Профиль пользователя",
		Date:       time.Now(),
		JsonData:   string(payloadBytes),
	}
	if err := h.repo.SaveResult(r.Context(), entry); err != nil {
		log.Printf("[DASHBOARD] не удалось сохранить профиль user=%d: %v", telegramID, err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal"})
		return
	}
	log.Printf("[DASHBOARD] профиль сохранён user=%d name=%q", telegramID, name)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// profileRecommendations — стартовые рекомендации на основе цели пользователя.
func profileRecommendations(goal string) []string {
	base := "Регулярно загружайте анализы и биосканы, чтобы отслеживать динамику здоровья."
	if goal == "" {
		return []string{
			base,
			"Укажите цель в профиле, чтобы получать персональные рекомендации.",
		}
	}
	return []string{
		base,
		"Ваша цель: " + goal + ". Зафиксируйте исходные показатели, затем повторяйте замеры раз в 2–4 недели.",
	}
}

// buildMetrics строит метрики на основе реальной истории пользователя.
func (h *Handler) buildMetrics(ctx context.Context, telegramID int64) MetricsResponse {
	resp := MetricsResponse{Recommendations: []string{}}

	entries, _, err := h.repo.ListHistory(ctx, telegramID, "", 1, 0)
	if err != nil || len(entries) == 0 {
		resp.NoData = true
		resp.HealthIndex = 0
		resp.EnergyLevel = "—"
		resp.Recommendations = []string{
			"Загрузите первый анализ или пройдите биоскан, чтобы увидеть сводку здоровья.",
		}
		return resp
	}

	// История отсортирована по убыванию даты (свежие сверху).
	latest := entries[0]
	resp.AnalysisDate = latest.Date.Format("2006-01-02")

	// Извлекаем профиль/показатели из последнего анализа (если есть JSON).
	report := parseReport(latest.JsonData)
	resp.UserName = report.Name
	resp.UserAge = report.Age

	// Показатели крови — из последнего анализа.
	resp.Blood.Hemoglobin = intOrZero(report.findIndicator("гемоглобин", "hemoglobin"))
	resp.Blood.Leukocytes = report.findIndicator("лейкоцит", "leukocyte", "wbc")
	resp.Blood.Platelets = intOrZero(report.findIndicator("тромбоцит", "platelet"))

	// Питание/активность — из последнего анализа при наличии.
	resp.Nutrition.Protein = intOrZero(report.findIndicator("белок", "protein"))
	resp.Nutrition.Carbs = intOrZero(report.findIndicator("углевод", "carb"))
	resp.Nutrition.Fat = intOrZero(report.findIndicator("жир", "fat"))
	resp.Activity.Steps = intOrZero(report.findIndicator("шаг", "step"))
	resp.Activity.Calories = intOrZero(report.findIndicator("калор", "calorie"))
	resp.Activity.Water = report.findIndicator("вода", "water")

	// Рекомендации — из последнего анализа, иначе дефолтные.
	if len(report.Recommendations) > 0 {
		resp.Recommendations = report.Recommendations
	} else {
		resp.Recommendations = []string{
			"Поддерживайте водный баланс (≈2 л воды в день).",
			"Регулярно повторяйте анализы для отслеживания динамики.",
		}
	}

	// Индекс здоровья: отражает вовлечённость + базовый уровень.
	resp.HealthIndex = healthIndex(len(entries), report)
	resp.EnergyLevel = energyLevel(report, len(entries))

	// Тренд: по одной точке на запись, от старых к новым.
	labels := make([]string, 0, len(entries))
	values := make([]int, 0, len(entries))
	// entries идут свежие→старые; строим тренд старые→свежие.
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		labels = append(labels, e.Date.Format("01.02"))
		values = append(values, healthIndex(i+1, parseReport(e.JsonData)))
	}
	resp.Trend = TrendData{Labels: labels, Values: values}

	return resp
}

// buildDemoMetrics возвращает синтетические «полностью заполненные» метрики
// для демо-режима (?demo=1). Позволяет посмотреть графики Сводки здоровья
// (индекс, энергия, показатели крови, динамика) без реальных анализов и
// без Premium. Данные детерминированные, чтобы график был стабильным.
func (h *Handler) buildDemoMetrics() MetricsResponse {
	return MetricsResponse{
		UserName:        "Демо Пользователь",
		UserAge:         34,
		NoData:          false,
		PremiumRequired: false,
		HealthIndex:     82,
		EnergyLevel:     "Высокий",
		AnalysisDate:    time.Now().Format("2006-01-02"),
		Blood: BloodData{
			Hemoglobin: 145,
			Leukocytes: 6.2,
			Platelets:  250,
		},
		Nutrition: NutData{Protein: 92, Carbs: 210, Fat: 65},
		Activity:  ActData{Steps: 8200, Calories: 2400, Water: 2.1},
		Trend: TrendData{
			Labels: []string{"01.03", "08.03", "15.03", "22.03", "29.03", "05.04"},
			Values: []int{60, 64, 63, 70, 75, 82},
		},
		Recommendations: []string{
			"Поддерживайте водный баланс (≈2 л воды в день).",
			"Активность выросла — отличный прогресс за месяц (индекс 60 → 82).",
			"Контролируйте уровень гемоглобина раз в 3–4 недели.",
		},
	}
}

// healthIndex — детерминированный индекс здоровья (0-100), производный от
// числа накопленных анализов и профиля последнего отчёта. Это НЕ медицинская
// оценка, а честный индикатор активности мониторинга + базовый уровень.
func healthIndex(count int, r reportData) int {
	base := 55
	base += count * 5
	if base > 95 {
		base = 95
	}
	if r.Composition > 0 {
		// profile.composition (0-100) из отчёта биоскана/анализа, если есть.
		base = (base + r.Composition) / 2
	}
	return base
}

func energyLevel(r reportData, count int) string {
	if r.Potential > 0 {
		switch {
		case r.Potential >= 75:
			return "Высокий"
		case r.Potential >= 50:
			return "Средний"
		default:
			return "Низкий"
		}
	}
	if count >= 3 {
		return "Высокий"
	}
	if count >= 1 {
		return "Средний"
	}
	return "—"
}

func intOrZero(f float64) int {
	return int(f)
}

// ---------------------------------------------------------------
// Парсинг отчёта анализа (толерантный)
// ---------------------------------------------------------------

type reportData struct {
	Name            string
	Age             int
	Composition     int
	Potential       int
	Recommendations []string
	indicators      map[string]float64
}

func (r reportData) findIndicator(names ...string) float64 {
	for _, n := range names {
		if v, ok := r.indicators[n]; ok {
			return v
		}
	}
	return 0
}

// parseReport извлекает профиль, показатели и рекомендации из JSON отчёта
// анализа (структура models.Report). При ошибке/пустоте возвращает пустую
// структуру — дашборд корректно покажет «—».
func parseReport(jsonStr string) reportData {
	out := reportData{indicators: map[string]float64{}}
	if strings.TrimSpace(jsonStr) == "" {
		return out
	}

	var doc struct {
		Profile struct {
			Name              string `json:"name"`
			Age               int    `json:"age"`
			Composition       int    `json:"composition"`
			MuscleDevelopment int    `json:"muscle_development"`
			Potential         int    `json:"potential"`
		} `json:"profile"`
		Categories []struct {
			Name       string `json:"name"`
			Indicators []struct {
				Name   string `json:"name"`
				Value  string `json:"value"`
				Status string `json:"status"`
			} `json:"indicators"`
		} `json:"categories"`
		Recommendations []string `json:"recommendations"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &doc); err != nil {
		return out
	}

	out.Name = doc.Profile.Name
	out.Age = doc.Profile.Age
	out.Composition = doc.Profile.Composition
	out.Potential = doc.Profile.Potential
	out.Recommendations = doc.Recommendations

	for _, cat := range doc.Categories {
		for _, ind := range cat.Indicators {
			key := normalizeIndicatorName(ind.Name)
			if key == "" {
				continue
			}
			if v, ok := firstNumber(ind.Value); ok {
				// Если один показатель встречается несколько раз, берём
				// первое значение (исторически — самое общее).
				if _, exists := out.indicators[key]; !exists {
					out.indicators[key] = v
				}
			}
		}
	}
	return out
}

// normalizeIndicatorName приводит название показателя к ключу для поиска.
func normalizeIndicatorName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.Contains(n, "гемоглобин"), strings.Contains(n, "hemoglobin"):
		return "гемоглобин"
	case strings.Contains(n, "лейкоцит"), strings.Contains(n, "leukocyte"), strings.Contains(n, "wbc"):
		return "лейкоцит"
	case strings.Contains(n, "тромбоцит"), strings.Contains(n, "platelet"):
		return "тромбоцит"
	case strings.Contains(n, "белок"), strings.Contains(n, "protein"):
		return "белок"
	case strings.Contains(n, "углевод"), strings.Contains(n, "carb"):
		return "углевод"
	case strings.Contains(n, "жир"), strings.Contains(n, "fat"):
		return "жир"
	case strings.Contains(n, "шаг"), strings.Contains(n, "step"):
		return "шаг"
	case strings.Contains(n, "калор"), strings.Contains(n, "calorie"):
		return "калор"
	case strings.Contains(n, "вода"), strings.Contains(n, "water"):
		return "вода"
	default:
		return ""
	}
}

// firstNumber извлекает первое число из строки вида "145", "5.4–6.1", "> 40".
func firstNumber(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	// Оставляем только цифры, точку, запятую и минус.
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' || r == ',' || r == '-' {
			if r == ',' {
				b.WriteRune('.')
			} else {
				b.WriteRune(r)
			}
		} else {
			// Прерываем на первом нечисловом разделителе, кроме '-' в начале.
			if b.Len() > 0 {
				break
			}
		}
	}
	if b.Len() == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(b.String(), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

//go:embed webapp_files/index.html webapp_files/style.css webapp_files/app.js webapp_files/data.js
var webappFS embed.FS
