package dashboard

import (
	"strings"
	"testing"
	"time"

	"github.com/theamornoir/analyzpro/internal/monitoring"
)

func TestBuildHealthAssessmentHTML(t *testing.T) {
	jsonData := `{
		"health_index": 72,
		"summary": "Общий образ жизни здоровый.",
		"lifestyle": {
			"sleep": {"score": 68, "comment": "Спите 6 часов."},
			"nutrition": {"score": 74, "comment": "Ешьте больше овощей."},
			"wellbeing": {"score": 55, "comment": "Энергия нестабильна."},
			"stress": {"score": 40, "comment": "Частый стресс."},
			"habits": {"score": 30, "comment": "Курение."}
		},
		"risk_zones": [{"name": "Сон", "level": "средний", "description": "Недосып."}],
		"plan": {"sleep": "Ложитесь раньше.", "nutrition": "Больше белка.", "wellbeing": "Прогулки.", "stress": "Дыхание."}
	}`
	entry := &monitoring.HistoryEntry{Type: "health_assessment", Title: "Общая оценка здоровья", Date: time.Now(), JsonData: jsonData}
	html := buildHealthAssessmentHTML(entry)
	for _, want := range []string{
		"Health Dashboard", "72",
		"Сон", "Питание", "Самочувствие", "Стресс", "Вредные привычки",
		"Персональные рекомендации", "Ложитесь раньше.",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("health dashboard HTML missing %q", want)
		}
	}
	if !strings.Contains(html, "<!doctype html>") {
		t.Errorf("health dashboard HTML missing doctype")
	}
}

func TestBuildAnalysisStructuredHTML(t *testing.T) {
	jsonData := `{
		"title": "Анализ крови",
		"note": "В целом норма.",
		"sections": [{"name": "Кровь", "indicators": [
			{"name": "Глюкоза", "value": "5.2", "normal": "3.9-6.1", "status": "normal"},
			{"name": "Холестерин", "value": "5.4", "normal": "0-5.0", "status": "warning"}
		]}]
	}`
	entry := &monitoring.HistoryEntry{Type: "analysis", Title: "Анализ крови", Date: time.Now(), JsonData: jsonData}
	html := buildAnalysisStructuredHTML(entry)
	if html == "" {
		t.Fatal("expected non-empty analysis HTML")
	}
	for _, want := range []string{"Глюкоза", "5.2", "Холестерин", "внимание"} {
		if !strings.Contains(html, want) {
			t.Errorf("analysis HTML missing %q", want)
		}
	}
}

func TestBuildBioscanStructuredHTML(t *testing.T) {
	jsonData := `{
		"title": "Bioscan",
		"score": 84,
		"summary": "Тело в норме.",
		"posture": {"posture_score": 84, "symmetry": 80, "mobility": 76},
		"zones": [{"name": "Плечи", "score": 88, "status": "good", "comment": "Ок."}]
	}`
	entry := &monitoring.HistoryEntry{Type: "bioscan", Title: "Bioscan", Date: time.Now(), JsonData: jsonData}
	html := buildBioscanStructuredHTML(entry)
	if html == "" {
		t.Fatal("expected non-empty bioscan HTML")
	}
	for _, want := range []string{"Показатели тела", "Осанка", "84", "Зоны тела", "Плечи"} {
		if !strings.Contains(html, want) {
			t.Errorf("bioscan HTML missing %q", want)
		}
	}
}

func TestBuildAnalysisStructuredHTMLEmpty(t *testing.T) {
	entry := &monitoring.HistoryEntry{Type: "analysis", JsonData: `{"title":"x","note":"только текст"}`}
	if html := buildAnalysisStructuredHTML(entry); html != "" {
		t.Errorf("expected empty HTML for unstructured analysis, got %q", html)
	}
}

func TestBuildBioscanStructuredHTMLEnhanced(t *testing.T) {
	jsonData := `{
		"title": "Bioscan PRO",
		"score": 86,
		"summary": "Тело укреплено.",
		"composition": [{"name":"Жировая масса","value":"18","unit":"%","status":"good","ref":"10-20%"}],
		"zones": [{"name":"Плечи","score":88,"status":"good","comment":"Ок."}],
		"strengths": [{"title":"Осанка","description":"Ровная спина."}],
		"improve": [{"title":"Пресс","description":"Проработать низ."}]
	}`
	entry := &monitoring.HistoryEntry{Type: "bioscan", Title: "Bioscan PRO", Date: time.Now(), JsonData: jsonData}
	html := buildBioscanStructuredHTML(entry)
	if html == "" {
		t.Fatal("expected non-empty bioscan HTML")
	}
	for _, want := range []string{"Композиция тела", "Жировая масса", "18", "Зоны тела", "Сильные стороны", "Осанка", "Зоны роста", "Пресс"} {
		if !strings.Contains(html, want) {
			t.Errorf("bioscan enhanced HTML missing %q", want)
		}
	}
}

func TestBuildBioscanStructuredHTMLBasicNote(t *testing.T) {
	entry := &monitoring.HistoryEntry{Type: "bioscan", Title: "Bioscan", Date: time.Now(), JsonData: `{"title":"Bioscan","note":"Ваши показатели в норме."}`}
	html := buildBioscanStructuredHTML(entry)
	if html == "" {
		t.Fatal("expected non-empty HTML for basic bioscan note")
	}
	if !strings.Contains(html, "Ваши показатели в норме.") {
		t.Errorf("basic bioscan note not rendered")
	}
}

func TestBuildBioscanStructuredHTMLCardFormat(t *testing.T) {
	jsonData := `{
		"title": "Bioscan PRO",
		"score": 86,
		"zones": [
			{"name":"Плечи","score":88,"status":"good","comment":"Сбалансированное развитие передней и задней дельты."},
			{"name":"Пресс","score":52,"status":"warning","comment":"Нижняя часть требует проработки."}
		]
	}`
	entry := &monitoring.HistoryEntry{Type: "bioscan", Title: "Bioscan PRO", Date: time.Now(), JsonData: jsonData}
	html := buildBioscanStructuredHTML(entry)
	if html == "" {
		t.Fatal("expected non-empty bioscan HTML")
	}
	// Нет табличного формата зон (была основная проблема на мобильных).
	if strings.Contains(html, "Зоны тела</h2><table>") {
		t.Errorf("zones still rendered as a 4-column <table>")
	}
	// Карточный формат: заголовок зоны, бейдж «балл · статус», комментарий.
	for _, want := range []string{
		"rec-head", "rec-badge", "Плечи", "88", "·", "хорошо",
		"Пресс", "52", "внимание",
		"Сбалансированное развитие передней и задней дельты.",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("card-format HTML missing %q", want)
		}
	}
}
