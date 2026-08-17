package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/payment"
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
	pay      *payment.MockPaymentService
	botToken string
	repo     monitoring.Repository
}

// NewHandler создаёт обработчик дашборда.
func NewHandler(pay *payment.MockPaymentService, botToken string, repo monitoring.Repository) *Handler {
	return &Handler{pay: pay, botToken: botToken, repo: repo}
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
