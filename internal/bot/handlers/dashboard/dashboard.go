package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/theamornoir/analyzpro/internal/analytics"
	"github.com/theamornoir/analyzpro/internal/locales"
	apmodels "github.com/theamornoir/analyzpro/internal/models"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/payment"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/report/pdfservice"
)

// MetricsResponse - ответ API для дашборда «Мой профиль».
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
	// Groups - адаптивные блоки с РЕАЛЬНЫМИ показателями пользователя,
	// собранными из его анализов/биосканов. Не показываем «общий анализ
	// крови» заглушкой, если у человека нет крови: блок появляется только
	// когда в истории есть соответствующие данные.
	Groups []MetricGroup `json:"groups"`
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

// MetricGroup - один адаптивный блок показателей в «Мой профиль»
// (например, «🩸 Кровь», «🍎 Питание», «✨ Тело (Bioscan PRO)»). Строится
// из РЕАЛЬНЫХ данных пользователя, поэтому пустой блок не рендерится - если
// у человека нет анализа крови, блока «кровь» просто не будет.
type MetricGroup struct {
	Title string       `json:"title"`
	Icon  string       `json:"icon"`
	Items []MetricItem `json:"items"`
}

// MetricItem - одна строка показателя (имя/значение/статус).
type MetricItem struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Status string `json:"status"` // "", "normal", "warning", "critical"
}

// Handler - HTTP-обработчики дашборда «Мой профиль».
// Премиум-гейт реализован на уровне API /api/metrics через проверку
// подлинности Telegram initData + статуса Premium пользователя.
type Handler struct {
	pay            *payment.MockPaymentService
	botToken       string
	repo           monitoring.Repository
	reportRenderer *report.Renderer
	pdfConverter   pdfservice.Converter
	// appEnv - текущее окружение (development/production). Используется,
	// чтобы демо-режим (?demo=1) работал только в development-отладке,
	// а в production требовал реальной авторизации initData.
	appEnv string
}

// NewHandler создаёт обработчик дашборда. reportRenderer/pdfConverter нужны,
// чтобы по запросу отдавать сохранённые отчёты как PDF (/api/reports/file).
func NewHandler(pay *payment.MockPaymentService, botToken string, repo monitoring.Repository, reportRenderer *report.Renderer, pdfConverter pdfservice.Converter, appEnv string) *Handler {
	return &Handler{pay: pay, botToken: botToken, repo: repo, reportRenderer: reportRenderer, pdfConverter: pdfConverter, appEnv: appEnv}
}

// demoMode возвращает true, только если запрошен демо-режим (?demo=1) И
// окружение - development. В production демо-режим не работает: он отдаёт
// синтетику БЕЗ проверки initData, что было бы латентной дырой (обход auth
// и выдача данных без подписи Telegram). Локальная отладка в браузере
// (AppEnv=development) остаётся рабочей.
func (h *Handler) demoMode(r *http.Request) bool {
	return r.URL.Query().Get("demo") == "1" && h.appEnv == "development"
}

// MaskID маскирует Telegram chat ID в логах, чтобы не светить сырой PII
// (идентификатор пользователя) в production-логах. Показывает первые и
// последние 4 цифры, скрывая середину. Экспортирована (заглавная буква),
// чтобы переиспользоваться в других пакетах обработчиков (например,
// router), где тоже логируется сырой chatID.
func MaskID(id int64) string {
	s := strconv.FormatInt(id, 10)
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "***" + s[len(s)-4:]
}

// assetVersionedRe - версионированные имена статических активов:
// app.<версия>.js / style.<версия>.css. Telegram WebView агрессивно
// кэширует JS/CSS ПО ПУТИ файла и часто игнорирует query-параметр (?v=),
// поэтому версия в пути - единственный надёжный способ сбросить кэш:
// при смене версии меняется сам URL, и старый закэшированный файл
// Telegram отдать не может. Любая версия резолвится в актуальный
// встроенный файл (на случай закэшированного старого index.html).
var assetVersionedRe = regexp.MustCompile(`^(app|style)\.[^.]+\.(js|css)$`)

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

	// Отрезаем query (?v=...) - файл ищем по чистому имени.
	if i := strings.IndexByte(filePath, '?'); i >= 0 {
		filePath = filePath[:i]
	}

	// Версионирование ПУТИ к статике (надёжнее query-параметра ?v=, который
	// Telegram WebView нередко игнорирует при кэшировании по пути файла).
	// Любой app.<версия>.js / style.<версия>.css отдаёт актуальный
	// встроенный файл, поэтому при смене версии меняется САМ URL - Telegram
	// не может отдать закэшированную старую копию. Старые версии тоже
	// резолвятся (на случай закэшированного старого index.html).
	if m := assetVersionedRe.FindStringSubmatch(filePath); m != nil {
		filePath = m[1] + "." + m[2] // app.js / style.css
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

// Metrics - обработчик GET /api/metrics. Возвращает реальные метрики
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
	// чтобы можно было посмотреть графики Мой профиль без реальных
	// анализов и без Premium. Работает и без валидного initData (удобно для
	// локальной отладки в браузере). Премиум-гейт не применяется.
	if h.demoMode(r) {
		log.Printf(locales.LogDashboardMetricsDemo)
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

	// PostHog: открытие Мой профиль реальным пользователем (не демо).
	analytics.Track(telegramID, "dashboard_opened", nil)

	metrics := h.buildMetrics(r.Context(), telegramID)

	// Премиум-гейт «Мой профиль» (как было раньше): без Premium
	// богатые метрики обзора скрываются - пользователь видит только
	// баннер с предложением оформить подписку. Бесплатные результаты
	// (обычный анализ, базовый биоскан) доступны в разделе отчётов
	// (/api/reports), где они не гейтятся и показываются своим карточками.
	isPremium := h.pay.IsPremium(telegramID)
	if !isPremium {
		metrics.Groups = nil
		metrics.Blood = BloodData{}
		metrics.Nutrition = NutData{}
		metrics.Activity = ActData{}
		metrics.Trend = TrendData{}
		metrics.HealthIndex = 0
		metrics.EnergyLevel = "-"
	}
	metrics.PremiumRequired = !isPremium

	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		log.Printf(locales.LogAPIEncodeError, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	log.Printf(locales.LogDashboardMetrics, MaskID(telegramID), metrics.NoData)
}

// Reports - обработчик GET /api/reports. Возвращает последний и предыдущий
// сохранённые отчёты расширенного анализа и Bioscan PRO (из истории
// пользователя) вместе с вычисленной дельтой индекса - чтобы дашборд
// «Мой профиль» мог показать графики последнего отчёта и сравнение
// прогресса с предыдущим.
//
// Доступ - как и у /api/metrics: валидация initData (подпись Telegram) +
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
	if h.demoMode(r) {
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

	// Премиум-гейт «Мой профиль» (как было раньше): без Premium
	// богатые поля отчётов (scores/zones/indicators/сравнение) скрываются.
	// Бесплатные результаты (обычный анализ, базовый биоскан) не содержат
	// этих полей, поэтому их карточки и архив остаются видимыми - это
	// собственные результаты пользователя. Расширенный анализ-досье и
	// Bioscan PRO генерируются по подписке в боте и здесь тоже скрываются
	// (богатый контент очищается, флаг rich сохраняется для фронтенда).
	isPremium := h.pay.IsPremium(telegramID)
	if !isPremium {
		data.Analysis = stripRichReports(data.Analysis)
		data.Bioscan = stripRichReports(data.Bioscan)
	}
	data.PremiumRequired = !isPremium

	_ = json.NewEncoder(w).Encode(data)
}

// stripRichReports очищает богатые поля отчётов (scores/zones/indicators/
// сравнение) для не-Premium пользователей. Флаг Rich у каждого отчёта
// сохраняется намеренно: фронтенд использует его, чтобы спрятать
// премиум-контент без подписки, но показать бесплатный (обычный анализ,
// базовый биоскан), у которого этих полей и так нет.
func stripRichReports(g ReportsGroup) ReportsGroup {
	clear := func(b ReportBlock) ReportBlock {
		b.Scores = nil
		b.Zones = nil
		b.Indicators = nil
		b.MainScore = 0
		return b
	}
	for i := range g.Reports {
		g.Reports[i] = clear(g.Reports[i])
	}
	g.Latest = clear(g.Latest)
	return g
}

// ReportFile - обработчик GET /api/reports/file. Отдаёт сохранённый отчёт
// (расширенный анализ или Bioscan PRO) как PDF-файл для просмотра/скачивания
// прямо из «Мой профиль». По id записи из истории берёт сохранённый
// ReportHTML либо перерендеривает HTML из JsonData и конвертирует в PDF через
// pdfservice (html2pdf.app). При недоступности PDF-конвертера - отдаёт сам
// HTML (отчёт не теряется).
//
// Доступ - по подписи initData (проверка подлинности Telegram) + проверке
// владения записью. ПРЕМИУМ-ГЕЙТ НЕ ПРИМЕНЯЕТСЯ: пользователь открывает
// свой собственный сохранённый отчёт, это его данные, и клик по карточке
// отчёта в «Мой профиль» должен открывать файл независимо от тарифа
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
	if h.demoMode(r) {
		entryType := r.URL.Query().Get("type")
		if entryType != "analysis" && entryType != "bioscan" {
			http.Error(w, "bad type", http.StatusBadRequest)
			return
		}
		entryID, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		html := h.buildDemoReportHTML(entryType, entryID)
		if strings.TrimSpace(html) == "" {
			http.Error(w, "render error", http.StatusInternalServerError)
			return
		}
		filename := "Demo_" + entryType + "_report"
		// Для встроенного просмотра (iframe внутри Mini App, ?view=inline)
		// отдаём сам HTML - он надёжно рендерится в WebView и НЕ триггерит
		// окно «посетить сайт» Telegram (URL того же домена, навигации нет).
		if r.URL.Query().Get("view") == "inline" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename+".html"))
			w.Write([]byte(html))
			return
		}
		// PDF если ключ html2pdf.app доступен, иначе сам HTML (inline).
		pdfBytes, convErr := h.pdfConverter.ConvertHTML(r.Context(), html)
		if convErr != nil {
			log.Printf("[DASHBOARD] PDF недоступен для демо-отчёта (%v) - отдаю HTML", convErr)
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

	// Для встроенного просмотра (iframe внутри Mini App, ?view=inline)
	// отдаём сам HTML - он надёжно рендерится в WebView и НЕ триггерит
	// окно «посетить сайт» Telegram. PDF-конвертацию пропускаем.
	if r.URL.Query().Get("view") == "inline" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename+".html"))
		w.Write([]byte(html))
		return
	}

	// Конвертация в PDF. При ошибке (нет ключа html2pdf.app / сервис
	// недоступен) - откат к отдаче самого HTML, чтобы отчёт не потерялся.
	pdfBytes, convErr := h.pdfConverter.ConvertHTML(r.Context(), html)
	if convErr != nil {
		log.Printf("[DASHBOARD] PDF-конвертация недоступна для отчёта id=%d: %v - отдаю HTML", entryID, convErr)
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

	log.Printf("[DASHBOARD] PDF-отчёт id=%d отдан пользователю (user=%s): %d байт", entryID, MaskID(telegramID), len(pdfBytes))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename+".pdf"))
	w.Write(pdfBytes)
}

// DeleteEntry - обработчик DELETE /api/reports/delete. Удаляет запись
// истории (анализ/биоскан/профиль) по ID - чтобы пользователь мог
// удалять свои данные прямо из «Мой профиль». Доступ по подписи
// initData (проверка подлинности Telegram) + проверке владения записью.
// ПРЕМИУМ-ГЕЙТ НЕ ПРИМЕНЯЕТСЯ: пользователь удаляет свои собственные
// данные, это должно работать на любом тарифе.
func (h *Handler) DeleteEntry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")

	// Демо-режим: синтетические отчёты не привязаны к БД, удалять нечего.
	if h.demoMode(r) {
		http.Error(w, "demo mode: nothing to delete", http.StatusBadRequest)
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

	entryID, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil || entryID <= 0 {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	entry, err := h.repo.GetHistoryEntry(r.Context(), entryID)
	if err != nil || entry == nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		return
	}
	// Защита: запись принадлежит этому пользователю.
	if entry.TelegramID != telegramID {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
		return
	}

	if err := h.repo.DeleteHistoryEntry(r.Context(), entryID); err != nil {
		log.Printf("[DASHBOARD] не удалось удалить запись id=%d user=%s: %v", entryID, MaskID(telegramID), err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal"})
		return
	}
	log.Printf("[DASHBOARD] удалена запись id=%d user=%s type=%s", entryID, MaskID(telegramID), entry.Type)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// reportHTML возвращает print-ready HTML сохранённого отчёта: если есть
// готовый ReportHTML - берёт его, иначе перерендеривает из JsonData через
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
// report.Renderer и не требует записи в БД - просто аккуратный демо-документ,
// который можно открыть как HTML (или PDF при наличии ключа html2pdf.app).
// demoReportBlock returns the synthetic demo ReportBlock for a specific id.
func (h *Handler) demoReportBlock(kind string, id int64) ReportBlock {
	demo := h.buildDemoReports()
	group := demo.Analysis
	if kind == "bioscan" {
		group = demo.Bioscan
	}
	for _, b := range group.Reports {
		if b.ID == id {
			return b
		}
	}
	return group.Latest
}

// buildDemoReportHTML returns a synthetic demo HTML report for a specific id.
func (h *Handler) buildDemoReportHTML(kind string, id int64) string {
	esc := func(s string) string {
		return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;").Replace(s)
	}
	block := h.demoReportBlock(kind, id)
	title := block.Title
	if title == "" {
		title = "Расширенный анализ"
		if kind == "bioscan" {
			title = "Bioscan PRO"
		}
	}
	date := block.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	mainScore := block.MainScore
	if mainScore <= 0 {
		mainScore = 75
	}
	scoresRows := ""
	keys := make([]string, 0, len(block.Scores))
	for k := range block.Scores {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		scoresRows += fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>", esc(k), esc(fmt.Sprintf("%d", block.Scores[k])))
	}
	extra := ""
	if kind == "bioscan" {
		if len(block.Zones) > 0 {
			zrows := ""
			for _, z := range block.Zones {
				zrows += fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>", esc(z.Name), esc(fmt.Sprintf("%d", z.Score)))
			}
			extra = `<h2 style="color:#1FA6A8;margin-top:24px">Оценка зон</h2>
<table><thead><tr><th>Зона</th><th>Балл</th></tr></thead><tbody>` + zrows + `</tbody></table>`
		}
	} else {
		if len(block.Indicators) > 0 {
			irows := ""
			for _, i := range block.Indicators {
				status := i.Status
				if status == "normal" {
					status = "норма"
				} else if status == "warning" {
					status = "внимание"
				}
				irows += fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%s</td></tr>", esc(i.Name), esc(i.Value), esc(status))
			}
			extra = `<h2 style="color:#1FA6A8;margin-top:24px">Показатели</h2>
<table><thead><tr><th>Показатель</th><th>Значение</th><th>Статус</th></tr></thead><tbody>` + irows + `</tbody></table>`
		}
	}
	html := `<!doctype html>
<html lang="ru"><head><meta charset="utf-8">
<title>` + title + ` (демо)</title>
<style>
body{font-family:system-ui,-apple-system,Segoe UI,Roboto,sans-serif;max-width:720px;margin:32px auto;padding:0 16px;color:#1a2330}
h1{color:#1FA6A8} .sub{color:#6b7785;margin-bottom:8px}
.score{font-size:40px;font-weight:700;color:#1FA6A8;margin:16px 0}
table{width:100%;border-collapse:collapse;margin-top:12px}
td,th{text-align:left;padding:8px 10px;border-bottom:1px solid #e3e8ef}
.demo{margin-top:24px;padding:12px 14px;background:#f4f8fa;border-radius:12px;color:#6b7785}
</style></head><body>
<h1>` + title + `</h1>
<div class="sub">Демонстрационный отчёт · ` + date + `</div>
<div class="score">` + fmt.Sprintf("%d", mainScore) + ` <span style="font-size:16px;color:#6b7785">` + esc(block.ScoreLabel) + `</span></div>
<h2 style="color:#1FA6A8;margin-top:24px">Компоненты</h2>
<table><thead><tr><th>Компонент</th><th>Балл</th></tr></thead>
<tbody>` + scoresRows + `</tbody></table>
` + extra + `
<div class="demo">Это демонстрационный отчёт Prisma. Чтобы открыть реальный отчёт, загрузите анализ или пройдите Bioscan PRO в боте.</div>
</body></html>`
	return html
}

// buildDemoReports - синтетические «последний» и «предыдущий» отчёты для
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
			{Name: "Гемоглобин", Value: "152 г/л", Status: "normal", Normal: "120-160 г/л", Num: 152, RefMin: 120, RefMax: 160},
			{Name: "Глюкоза", Value: "5.2 ммоль/л", Status: "normal", Normal: "3.9-6.1 ммоль/л", Num: 5.2, RefMin: 3.9, RefMax: 6.1},
			{Name: "Холестерин", Value: "5.1 ммоль/л", Status: "warning", Normal: "0-5.0 ммоль/л", Num: 5.1, RefMin: 0, RefMax: 5.0},
		},
		Summary: "Показатели улучшились по сравнению с предыдущим анализом.",
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
			Count:   3,
			Reports: []ReportBlock{latestA, prevA, olderA},
			Latest:  latestA,
		},
		Bioscan: ReportsGroup{
			Count:   3,
			Reports: []ReportBlock{latestB, prevB, olderB},
			Latest:  latestB,
		},
	}
}

// ProfileRequest - тело запроса регистрации профиля из Mini App «Мой
// профиль». Минимальный набор полей, чтобы дашборд перестал быть пустым.
type ProfileRequest struct {
	Name   string `json:"name"`
	Age    int    `json:"age"`
	Gender string `json:"gender"`
	Height int    `json:"height"`
	Weight int    `json:"weight"`
	Goal   string `json:"goal"`
}

// SaveProfile - обработчик POST /api/profile. Принимает минимальный профиль
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
		log.Printf("[DASHBOARD] не удалось сохранить профиль user=%s: %v", MaskID(telegramID), err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal"})
		return
	}
	log.Printf("[DASHBOARD] профиль сохранён user=%s name=%q", MaskID(telegramID), "***")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// profileRecommendations - стартовые рекомендации на основе цели пользователя.
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
		resp.EnergyLevel = "-"
		resp.Recommendations = []string{
			"Загрузите первый анализ или пройдите биоскан, чтобы увидеть свой профиль.",
		}
		return resp
	}

	// Дата «Последний анализ» и показатели крови/питания/активности берём
	// ТОЛЬКО из реального анализа (тип "analysis"), а не из любой записи
	// истории (профиль/биоскан). Иначе после заполнения профиля в шапке
	// висит дата профиля, выдаваемая за «последний анализ», который
	// пользователь не загружал. Если анализа нет - дата пустая (фронт
	// покажет «Нет загруженных анализов»), а показатели - «-».
	analysisEntries, _, _ := h.repo.ListHistory(ctx, telegramID, "analysis", 1, 1)
	var report reportData
	if len(analysisEntries) > 0 {
		resp.AnalysisDate = analysisEntries[0].Date.Format("2006-01-02")
		report = parseReport(analysisEntries[0].JsonData)
		resp.UserName = report.Name
		resp.UserAge = report.Age

		// Показатели крови - из последнего анализа.
		resp.Blood.Hemoglobin = intOrZero(report.findIndicator("гемоглобин", "hemoglobin"))
		resp.Blood.Leukocytes = report.findIndicator("лейкоцит", "leukocyte", "wbc")
		resp.Blood.Platelets = intOrZero(report.findIndicator("тромбоцит", "platelet"))

		// Питание/активность - из последнего анализа при наличии.
		resp.Nutrition.Protein = intOrZero(report.findIndicator("белок", "protein"))
		resp.Nutrition.Carbs = intOrZero(report.findIndicator("углевод", "carb"))
		resp.Nutrition.Fat = intOrZero(report.findIndicator("жир", "fat"))
		resp.Activity.Steps = intOrZero(report.findIndicator("шаг", "step"))
		resp.Activity.Calories = intOrZero(report.findIndicator("калор", "calorie"))
		resp.Activity.Water = report.findIndicator("вода", "water")
	}

	// Имя/возраст: из анализа, если он есть, иначе из последней записи
	// истории (профиля) - чтобы шапка сводки показывала пользователя ещё
	// до загрузки первого анализа. Сама «дата последнего анализа» при этом
	// берётся выше ТОЛЬКО из анализа (см. analysisEntries), поэтому дата
	// профиля не выдаётся за дату анализа.
	profileReport := parseReport(entries[0].JsonData)
	if resp.UserName == "" {
		resp.UserName = profileReport.Name
	}
	if resp.UserAge == 0 {
		resp.UserAge = profileReport.Age
	}

	// Рекомендации - из последнего анализа, иначе дефолтные.
	if len(report.Recommendations) > 0 {
		resp.Recommendations = report.Recommendations
	} else {
		resp.Recommendations = []string{
			"Поддерживайте водный баланс (≈2 л воды в день).",
			"Регулярно повторяйте анализы для отслеживания динамики.",
		}
	}

	// Реальные замеры здоровья (анализ/биоскан) - без анкеты-профиля,
	// которая тоже хранится в истории, но замером не является. Индекс и
	// тренд считаем ТОЛЬКО по ним, иначе профиль «раздувает» число точек
	// и прячет подсказку «Пока один замер» при наличии одного анализа.
	measurements := make([]monitoring.HistoryEntry, 0, len(entries))
	for _, e := range entries {
		if e.Type == "analysis" || e.Type == "bioscan" {
			measurements = append(measurements, e)
		}
	}

	// Индекс здоровья: отражает вовлечённость + базовый уровень.
	resp.HealthIndex = healthIndex(len(measurements), report)
	resp.EnergyLevel = energyLevel(report, len(measurements))

	// Адаптивные блоки РЕАЛЬНЫХ показателей (кровь/биохимия/тело и т.п.)
	// - строятся только из того, что есть у пользователя в истории.
	resp.Groups = h.buildAdaptiveGroups(ctx, telegramID)

	// Тренд: по одной точке на РЕАЛЬНЫЙ замер, от старых к новым.
	labels := make([]string, 0, len(measurements))
	values := make([]int, 0, len(measurements))
	// measurements идут свежие→старые; строим тренд старые→свежие.
	for i := len(measurements) - 1; i >= 0; i-- {
		e := measurements[i]
		labels = append(labels, e.Date.Format("01.02"))
		values = append(values, healthIndex(i+1, parseReport(e.JsonData)))
	}
	resp.Trend = TrendData{Labels: labels, Values: values}

	return resp
}

// buildAdaptiveGroups строит адаптивные блоки показателей из РЕАЛЬНОЙ
// истории пользователя. Берёт последний анализ (по типам analysis/bioscan)
// и собирает из него группы (кровь/биохимия/гормоны/...) и блок тела из
// Bioscan PRO. Блок не появляется, если соответствующих данных нет - поэтому
// «общий анализ крови» не висит заглушкой у тех, у кого нет крови.
func (h *Handler) buildAdaptiveGroups(ctx context.Context, telegramID int64) []MetricGroup {
	entries, _, err := h.repo.ListHistory(ctx, telegramID, "", 0, 0)
	if err != nil || len(entries) == 0 {
		return nil
	}
	groups := []MetricGroup{}
	// Только последний анализ (записи отсортированы свежие → старые).
	for _, e := range entries {
		if e.Type == "analysis" {
			if g := groupsFromAnalysis(e.JsonData); len(g) > 0 {
				groups = append(groups, g...)
			}
			break
		}
	}
	// Тело - из последнего Bioscan PRO.
	for _, e := range entries {
		if e.Type == "bioscan" {
			if g := groupFromBioscan(e.JsonData); g != nil {
				groups = append(groups, *g)
			}
			break
		}
	}
	return groups
}

// categoryShape / indicatorJson - форма блока показателей в JSON отчёта
// анализа (categories/sections → indicators).
type categoryShape struct {
	Name       string          `json:"name"`
	Title      string          `json:"title"`
	Indicators []indicatorJson `json:"indicators"`
}

type indicatorJson struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Unit   string `json:"unit"`
	Status string `json:"status"`
}

// indicatorRaw - сырая форма показателя для parseReport (читает и name, и
// title, т.к. разные ИИ-отчёты используют разные ключи заголовка группы).
type indicatorRaw struct {
	Name   string `json:"name"`
	Title  string `json:"title"`
	Value  string `json:"value"`
	Status string `json:"status"`
}

// groupsFromAnalysis строит группы показателей из категорий/секций отчёта
// анализа (реальные индикаторы с их статусами из ИИ-отчёта).
func groupsFromAnalysis(jsonStr string) []MetricGroup {
	var doc struct {
		Categories []categoryShape `json:"categories"`
		Sections   []categoryShape `json:"sections"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &doc); err != nil {
		return nil
	}
	var groups []MetricGroup
	add := func(c categoryShape) {
		// Заголовок группы: приоритет у «name», иначе «title»
		// (часть ИИ-отчётов отдаёт секции с title вместо name).
		heading := strings.TrimSpace(c.Name)
		if heading == "" {
			heading = strings.TrimSpace(c.Title)
		}
		if heading == "" {
			return
		}
		items := []MetricItem{}
		for _, ind := range c.Indicators {
			if strings.TrimSpace(ind.Name) == "" {
				continue
			}
			// Подставляем единицу измерения к значению, если модель
			// вернула её отдельным полем (value="145", unit="г/л").
			val := strings.TrimSpace(ind.Value)
			if u := strings.TrimSpace(ind.Unit); u != "" && !strings.Contains(val, u) {
				val = val + " " + u
			}
			items = append(items, MetricItem{
				Name:   ind.Name,
				Value:  val,
				Status: normStatus(ind.Status),
			})
		}
		if len(items) > 0 {
			groups = append(groups, MetricGroup{
				Title: heading,
				Icon:  iconForCategory(heading),
				Items: items,
			})
		}
	}
	for _, c := range doc.Categories {
		add(c)
	}
	for _, s := range doc.Sections {
		add(s)
	}
	return groups
}

// groupFromBioscan строит блок «Тело (Bioscan PRO)» из отчёта биоскана:
// Body Score + оценки зон тела. Возвращает nil, если данных нет.
func groupFromBioscan(jsonStr string) *MetricGroup {
	var doc struct {
		Score   int `json:"score"`
		Posture struct {
			PostureScore int `json:"posture_score"`
		} `json:"posture"`
		Zones []struct {
			Name   string `json:"name"`
			Score  int    `json:"score"`
			Status string `json:"status"`
		} `json:"zones"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &doc); err != nil {
		return nil
	}
	items := []MetricItem{}
	if doc.Score > 0 {
		items = append(items, MetricItem{Name: "Body Score", Value: strconv.Itoa(doc.Score), Status: ""})
	} else if doc.Posture.PostureScore > 0 {
		items = append(items, MetricItem{Name: "Осанка", Value: strconv.Itoa(doc.Posture.PostureScore), Status: ""})
	}
	for _, z := range doc.Zones {
		if strings.TrimSpace(z.Name) == "" {
			continue
		}
		items = append(items, MetricItem{
			Name:   z.Name,
			Value:  strconv.Itoa(z.Score),
			Status: normStatus(z.Status),
		})
	}
	if len(items) == 0 {
		return nil
	}
	return &MetricGroup{Title: "Тело (Bioscan PRO)", Icon: "✨", Items: items}
}

// normStatus приводит статус показателя к каноническому виду для CSS-классов
// фронта (ind-status-normal/warning/critical).
func normStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "normal", "норма", "good", "ok":
		return "normal"
	case "warning", "внимание", "warn":
		return "warning"
	case "critical", "критично", "alert":
		return "critical"
	default:
		return ""
	}
}

// iconForCategory подбирает эмодзи для заголовка группы по названию категории.
func iconForCategory(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.Contains(n, "кров"):
		return "🩸"
	case strings.Contains(n, "биохим"):
		return "🧪"
	case strings.Contains(n, "гормон"):
		return "⚗️"
	case strings.Contains(n, "моч"):
		return "💧"
	case strings.Contains(n, "кал"):
		return "🧫"
	case strings.Contains(n, "иммун"):
		return "🛡️"
	case strings.Contains(n, "витамин"):
		return "💊"
	case strings.Contains(n, "липид"), strings.Contains(n, "холест"):
		return "🫀"
	default:
		return "📋"
	}
}

// buildDemoMetrics возвращает синтетические «полностью заполненные» метрики
// для демо-режима (?demo=1). Позволяет посмотреть графики Мой профиль
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
			"Активность выросла - отличный прогресс за месяц (индекс 60 → 82).",
			"Контролируйте уровень гемоглобина раз в 3–4 недели.",
		},
		// Адаптивные блоки (демо): показываем, как выглядят реальные
		// показатели из анализа и Bioscan PRO.
		Groups: []MetricGroup{
			{
				Title: "Кровь", Icon: "🩸",
				Items: []MetricItem{
					{Name: "Гемоглобин", Value: "145", Status: "normal"},
					{Name: "Лейкоциты", Value: "6.2", Status: "normal"},
					{Name: "Тромбоциты", Value: "250", Status: "normal"},
				},
			},
			{
				Title: "Питание", Icon: "🍎",
				Items: []MetricItem{
					{Name: "Белок", Value: "92 г", Status: "normal"},
					{Name: "Углеводы", Value: "210 г", Status: "normal"},
					{Name: "Жиры", Value: "65 г", Status: "normal"},
				},
			},
			{
				Title: "Тело (Bioscan PRO)", Icon: "✨",
				Items: []MetricItem{
					{Name: "Body Score", Value: "86", Status: ""},
					{Name: "Осанка", Value: "84", Status: "normal"},
					{Name: "Пресс", Value: "74", Status: "warning"},
				},
			},
		},
	}
}

// healthIndex - детерминированный индекс здоровья (0-100), производный от
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
	return "-"
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
// структуру - дашборд корректно покажет «-».
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
			Title      string `json:"title"`
			Indicators []indicatorRaw
		} `json:"categories"`
		// Sections - альтернативная форма группировки показателей
		// (некоторые ИИ-отчёты отдают секции с title вместо name).
		Sections []struct {
			Name       string `json:"name"`
			Title      string `json:"title"`
			Indicators []indicatorRaw
		} `json:"sections"`
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

	// Собираем показатели из categories и sections (заголовок - name,
	// либо title, если name пуст). Первое встреченное значение побеждает.
	collect := func(name, title string, inds []indicatorRaw) {
		heading := strings.TrimSpace(name)
		if heading == "" {
			heading = strings.TrimSpace(title)
		}
		if heading == "" {
			return
		}
		for _, ind := range inds {
			key := normalizeIndicatorName(ind.Name)
			if key == "" {
				continue
			}
			if v, ok := firstNumber(ind.Value); ok {
				if _, exists := out.indicators[key]; !exists {
					out.indicators[key] = v
				}
			}
		}
	}
	for _, cat := range doc.Categories {
		collect(cat.Name, cat.Title, cat.Indicators)
	}
	for _, sec := range doc.Sections {
		collect(sec.Name, sec.Title, sec.Indicators)
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
