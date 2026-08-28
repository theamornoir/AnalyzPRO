package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/models"
)

func TestRenderHealthAssessmentPDF(t *testing.T) {
	ha := models.HealthAssessment{
		HealthIndex: 72,
		Summary:     "Общий образ жизни в целом здоровый, есть зоны для улучшения сна и движения.",
		Lifestyle: map[string]models.LifestyleDim{
			"sleep":     {Score: 68, Comment: "Сон недостаточно глубокий, стоит ложиться раньше."},
			"nutrition": {Score: 74, Comment: "Питание сбалансировано, не хватает белка."},
			"wellbeing": {Score: 70, Comment: "Самочувствие stable."},
			"stress":    {Score: 55, Comment: "Уровень стресса повышен в будни."},
			"habits":    {Score: 80, Comment: "Вредных привычек почти нет."},
		},
		RiskZones: []models.RiskZone{
			{Name: "Хронический стресс", Level: "средний", Description: "Постоянное напряжение в течение дня."},
			{Name: "Нерегулярный сон", Level: "высокий", Description: "Поздние подъёмы и отбой."},
		},
		Plan: models.HealthPlan{
			Sleep:     "Ложиться до 23:00, 7-8 часов сна.",
			Nutrition: "Добавить 20 г белка в день.",
			Wellbeing: "Прогулка 30 минут на свежем воздухе.",
			Stress:    "Дыхательные практики 5 минут утром.",
		},
	}

	pdf, err := RenderHealthAssessmentPDF(ha)
	if err != nil {
		t.Fatalf("RenderHealthAssessmentPDF: %v", err)
	}
	if len(pdf) < 500 {
		t.Fatalf("health PDF too small: %d bytes", len(pdf))
	}
	if !strings.HasPrefix(string(pdf[:5]), "%PDF") {
		head := string(pdf)
		if len(head) > 5 {
			head = head[:5]
		}
		t.Fatalf("output is not a PDF (prefix=%q)", head)
	}
}

func TestRenderBioscanPDFFromMock(t *testing.T) {
	var rep models.Report
	if err := json.Unmarshal([]byte(locales.MockBioscanJSONText), &rep); err != nil {
		t.Fatalf("unmarshal mock bioscan JSON: %v", err)
	}
	rep.IsBioscan = true

	pdf, err := RenderBioscanPDF(rep)
	if err != nil {
		t.Fatalf("RenderBioscanPDF: %v", err)
	}
	if len(pdf) < 100 {
		t.Fatalf("PDF too small: %d bytes", len(pdf))
	}
	if !strings.HasPrefix(string(pdf[:5]), "%PDF") {
		head := string(pdf)
		if len(head) > 5 {
			head = head[:5]
		}
		t.Fatalf("output is not a PDF (prefix=%q)", head)
	}
}

func TestRenderBioscanPlainTextFromMock(t *testing.T) {
	var rep models.Report
	if err := json.Unmarshal([]byte(locales.MockBioscanJSONText), &rep); err != nil {
		t.Fatalf("unmarshal mock bioscan JSON: %v", err)
	}
	rep.IsBioscan = true

	text := RenderBioscanPlainText(rep)
	if !strings.Contains(text, "BIOSCAN") {
		t.Fatalf("plain text missing header: %q", text)
	}
	if strings.Contains(text, "**") {
		t.Fatalf("plain text contains markdown: %q", text)
	}
}
