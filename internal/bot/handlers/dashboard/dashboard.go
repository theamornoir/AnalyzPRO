package dashboard

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// MetricsResponse — ответ API для дашборда.
type MetricsResponse struct {
	HealthIndex    int       `json:"healthIndex"`
	EnergyLevel    string    `json:"energyLevel"`
	AnalysisDate   string    `json:"analysisDate"`
	Blood          BloodData `json:"blood"`
	Nutrition      NutData   `json:"nutrition"`
	Activity       ActData   `json:"activity"`
	Trend          TrendData `json:"trend"`
	Recommendations []string `json:"recommendations"`
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

// HandleWebApp — отдаёт статический веб-дашборд.
// Открывается только для Premium-пользователей.
func HandleWebApp(w http.ResponseWriter, r *http.Request, isPremium bool) {
	if !isPremium {
		http.Error(w, locales.MsgPremiumRequired, http.StatusForbidden)
		return
	}

	// Проверяем, запрос на главную страницу
	if r.URL.Path != "/" && r.URL.Path != "/dashboard" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>AnalyzPRO — Дашборд</title>
    <style>
        body { font-family: Arial, sans-serif; background: #E9EEEE; margin: 0; padding: 20px; }
        .container { max-width: 800px; margin: 0 auto; background: white; border-radius: 12px; padding: 30px; box-shadow: 0 2px 10px rgba(0,0,0,0.08); }
        h1 { color: #1A2A2A; margin-bottom: 10px; }
        .subtitle { color: #6B7A7A; margin-bottom: 30px; }
        .grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 15px; margin-bottom: 30px; }
        .card { background: #F0F8F8; padding: 20px; border-radius: 10px; text-align: center; }
        .card .label { font-size: 11px; color: #6B7A7A; text-transform: uppercase; margin-bottom: 8px; }
        .card .value { font-size: 32px; font-weight: 800; color: #1FA6A8; }
        .rec { background: #F0F8F8; padding: 15px; border-radius: 8px; margin-bottom: 10px; border-left: 3px solid #1FA6A8; }
        @media (max-width: 600px) { .grid { grid-template-columns: 1fr; } }
    </style>
</head>
<body>
    <div class="container">
        <h1>📊 Мой Дашборд</h1>
        <p class="subtitle">Аналитика здоровья в реальном времени</p>
        
        <div class="grid">
            <div class="card">
                <div class="label">Индекс здоровья</div>
                <div class="value">78</div>
            </div>
            <div class="card">
                <div class="label">Энергия</div>
                <div class="value">Высокий</div>
            </div>
            <div class="card">
                <div class="label">Дата анализа</div>
                <div class="value" style="font-size:16px;">%s</div>
            </div>
        </div>

        <h2 style="margin-bottom:15px;">Рекомендации</h2>
        <div class="rec">💡 Увеличить потребление белка до 1.5г/кг массы тела</div>
        <div class="rec">💡 Добавить 30 минут кардио 3 раза в неделю</div>
        <div class="rec">💡 Контролировать уровень витамина D</div>
        <div class="rec">💡 Нормализовать режим сна (7-8 часов)</div>
    </div>
</body>
</html>`, time.Now().Format("2006-01-02"))
}
