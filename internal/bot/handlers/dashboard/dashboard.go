package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

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
	// Убираем префикс /dashboard/
	filePath := r.URL.Path
	if filePath == "/dashboard/" || filePath == "/" {
		filePath = "index.html"
	} else {
		filePath = strings.TrimPrefix(filePath, "/dashboard/")
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

	if !h.pay.IsPremium(telegramID) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "premium_required"})
		return
	}

	metrics := h.buildMetrics(r.Context(), telegramID)

	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		log.Printf(locales.LogAPIEncodeError, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	log.Printf("[DASHBOARD] /api/metrics отданы для user=%d (noData=%v)", telegramID, metrics.NoData)
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
