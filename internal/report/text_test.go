package report

import (
	"strings"
	"testing"

	reportmodels "github.com/theamornoir/analyzpro/internal/report/models"
)

// mockAnalysisJSON - чистый JSON от ИИ (без обёртки).
const mockAnalysisJSON = `{
  "title": "Анализ здоровья",
  "date": "14.03.2025",
  "summary": "Картина стабильна: печень и воспаление в норме, глюкоза и почки в порядке. Главная проблема - липидный профиль.",
  "sections": [
    {
      "type": "blood",
      "title": "Общий анализ крови (ОАК)",
      "indicators": [
        {"name": "Гемоглобин", "value": "145", "unit": "г/л", "normal": "130-160", "status": "normal", "description": "Переносит кислород по организму."},
        {"name": "СОЭ", "value": "12", "unit": "мм/ч", "normal": "до 10", "status": "warning", "description": "Маркер воспаления в организме."}
      ]
    },
    {
      "type": "blood",
      "title": "Липиды",
      "indicators": [
        {"name": "Холестерин", "value": "6.4", "unit": "ммоль/л", "normal": "до 5.2", "status": "critical", "description": "Жировое вещество для гормонов и клеток."},
        {"name": "ЛПВП", "value": "1.1", "unit": "ммоль/л", "normal": "более 1.0", "status": "normal", "description": "Хороший холестерин, чистит артерии."}
      ]
    },
    {
      "type": "warning",
      "title": "Зоны внимания",
      "list": ["Липидный профиль выше нормы - обсудить с терапевтом."]
    },
    {
      "type": "recommendation",
      "title": "Рекомендации",
      "list": ["150 мин/нед аэробной нагрузки", "Омега-3: рыба 2-3 раза в неделю"]
    },
    {
      "type": "profile",
      "title": "Профиль здоровья",
      "score": 78,
      "summary": "Общее состояние удовлетворительное: липидный профиль тянет вниз, остальное в норме."
    }
  ],
  "disclaimer": "Разбор информационный, не заменяет консультацию врача"
}`

// mockAnalysisAdaptiveJSON - JSON от ИИ с markdown-обёрткой (как вернёт
// HandleAnalysisFromFileJSON / HandleAnalysisFromFilesJSON).
var mockAnalysisAdaptiveJSON = "```json\n" + mockAnalysisJSON + "\n```"

// TestParseAdaptiveReportJSON очищает markdown-обёртку и парсит JSON.
func TestParseAdaptiveReportJSON(t *testing.T) {
	data, err := ParseAdaptiveReportJSON(mockAnalysisAdaptiveJSON)
	if err != nil {
		t.Fatalf("ParseAdaptiveReportJSON: %v", err)
	}
	if data.Date != "14.03.2025" {
		t.Fatalf("date not parsed: %q", data.Date)
	}
	if len(data.Sections) != 5 {
		t.Fatalf("expected 5 sections, got %d", len(data.Sections))
	}
	if data.Sections[0].Indicators[0].Description == "" {
		t.Fatalf("indicator description not parsed")
	}
}

// TestRenderAnalysisPlainTextGuaranteedFormat проверяет детерминированный
// формат чат-отчёта анализа (как у Bioscan): emoji-маркеры, без markdown,
// разделы через ●, показатели через ✅/❌, рекомендации через ✦.
func TestRenderAnalysisPlainTextGuaranteedFormat(t *testing.T) {
	data, err := ParseAdaptiveReportJSON(mockAnalysisAdaptiveJSON)
	if err != nil {
		t.Fatalf("ParseAdaptiveReportJSON: %v", err)
	}
	text := RenderAnalysisPlainText(data)

	checks := map[string]string{
		"header":         "🔬 АНАЛИЗ КРОВИ · 14.03.2025",
		"section marker": "● ОБЩИЙ АНАЛИЗ КРОВИ (ОАК)",
		"normal marker":  "✅ Гемоглобин",
		"value line":     "145 г/л (130-160)",
		"warning marker": "❌ СОЭ",
		"attention note": "● ЛИПИДЫ (главное внимание)",
		"conclusions":    "● ВЫВОДЫ",
		"attention sec":  "● ЗОНЫ ВНИМАНИЯ",
		"attention item": "⚠️ Липидный профиль выше нормы - обсудить с терапевтом.",
		"reco sec":       "● РЕКОМЕНДАЦИИ",
		"reco item":      "✦ 150 мин/нед аэробной нагрузки",
		"profile sec":    "● ПРОФИЛЬ ЗДОРОВЬЯ",
		"profile score":  "Общая оценка: 78/100",
		"disclaimer":     "Разбор информационный, не заменяет консультацию врача",
	}
	for name, want := range checks {
		if !strings.Contains(text, want) {
			t.Fatalf("'%s': expected text to contain %q, got:\n%s", name, want, text)
		}
	}

	// Гарантии: без markdown-звёздочек.
	if strings.Contains(text, "**") {
		t.Fatalf("text contains markdown: %q", text)
	}
	// Все показатели имеют статус-маркер.
	if !strings.Contains(text, "❌ Холестерин") {
		t.Fatalf("critical indicator missing marker: %q", text)
	}
	// Блок профиля - наверху (до первой кровяной секции), не дублируется в конце.
	idxProfile := strings.Index(text, "● ПРОФИЛЬ ЗДОРОВЬЯ")
	idxFirstBlood := strings.Index(text, "● ОБЩИЙ АНАЛИЗ КРОВИ")
	if idxProfile == -1 || idxFirstBlood == -1 || idxProfile > idxFirstBlood {
		t.Fatalf("profile block must be above blood sections: %q", text)
	}
	if c := strings.Count(text, "Общее состояние удовлетворительное"); c != 1 {
		t.Fatalf("profile summary duplicated %d times, expected 1:\n%s", c, text)
	}
}

// TestRenderAnalysisPlainTextEmpty гарантирует, что пустой разбор не падает.
func TestRenderAnalysisPlainTextEmpty(t *testing.T) {
	var empty reportmodels.AdaptiveReportData
	if s := RenderAnalysisPlainText(empty); s == "" {
		t.Fatalf("expected non-empty fallback (header + disclaimer)")
	}
}
