package dashboard

import (
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// MetricsResponse — ответ API для дашборда.
type MetricsResponse struct {
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

// HandleAPIMetrics — обработчик GET /api/metrics.
// Возвращает JSON с метриками пользователя (из mock-репозитория).
func HandleAPIMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Проверяем Content-Type
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// TODO: В будущем заменим на реальные данные из БД
	// Сейчас используем мок-данные
	metrics := generateMockMetrics()

	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		log.Printf(locales.LogAPIEncodeError, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("API /api/metrics requested")
}

// generateMockMetrics — генерирует мок-данные для дашборда.
func generateMockMetrics() MetricsResponse {
	return MetricsResponse{
		HealthIndex:  78,
		EnergyLevel:  "Высокий",
		AnalysisDate: time.Now().Format("2006-01-02"),
		Blood: BloodData{
			Hemoglobin: 145,
			Leukocytes: 6.2,
			Platelets:  250,
		},
		Nutrition: NutData{
			Protein: 85,
			Carbs:   70,
			Fat:     60,
		},
		Activity: ActData{
			Steps:    8500,
			Calories: 2200,
			Water:    2.5,
		},
		Trend: TrendData{
			Labels: []string{"Янв", "Фев", "Мар", "Апр", "Май", "Июн"},
			Values: []int{65, 68, 72, 70, 75, 78},
		},
		Recommendations: []string{
			"Увеличить потребление белка до 1.5г/кг массы тела",
			"Добавить 30 минут кардио 3 раза в неделю",
			"Контролировать уровень витамина D",
			"Нормализовать режим сна (7-8 часов)",
		},
	}
}

//go:embed webapp_files/index.html webapp_files/style.css webapp_files/app.js webapp_files/data.js
var webappFS embed.FS

// GenerateMockMetrics — публичная функция для мок-данных (для тестов)
func GenerateMockMetrics() MetricsResponse {
	return generateMockMetrics()
}

// HandleWebApp — отдаёт статический веб-дашборд из embed-файлов.
// Требует Premium-подписки.
func HandleWebApp(w http.ResponseWriter, r *http.Request, isPremium bool) {
	if !isPremium {
		http.Error(w, locales.MsgPremiumRequired, http.StatusForbidden)
		return
	}

	// Убираем префикс /dashboard/
	filePath := r.URL.Path
	if filePath == "/dashboard/" || filePath == "/" {
		filePath = "index.html"
	} else {
		filePath = filePath[len("/dashboard/"):]
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

// HandleWebAppIndex — сервис-обёртка для роутера Telegram.
func HandleWebAppIndex(w http.ResponseWriter, r *http.Request) {
	HandleWebApp(w, r, true)
}
