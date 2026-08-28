package report

import (
	"strings"
	"testing"

	"github.com/theamornoir/analyzpro/internal/models"
)

func TestSanitizeHealthAssessmentStripsName(t *testing.T) {
	ha := &models.HealthAssessment{
		Summary: "Влад спит около 8 часов, но Влад часто нервничает.",
		Lifestyle: map[string]models.LifestyleDim{
			"sleep":     {Score: 60, Comment: "Влад ложится поздно и плохо засыпает."},
			"nutrition": {Score: 70, Comment: "Питание в целом сбалансировано."},
		},
		RiskZones: []models.RiskZone{
			{Name: "Сон", Level: "средний", Description: "У Влада недосып по будням."},
		},
		Plan: models.HealthPlan{
			Sleep:     "Владу стоит ложиться до 23:00.",
			Nutrition: "Пить больше воды.",
			Wellbeing: "Гулять по вечерам.",
			Stress:    "Дыхательные практики.",
		},
	}

	SanitizeHealthAssessment(ha, "Влад")

	for _, c := range []struct {
		field, got string
	}{
		{"summary", ha.Summary},
		{"sleep.comment", ha.Lifestyle["sleep"].Comment},
		{"risk.description", ha.RiskZones[0].Description},
		{"plan.sleep", ha.Plan.Sleep},
	} {
		if strings.Contains(c.got, "Влад") {
			t.Errorf("field %q still contains name: %q", c.field, c.got)
		}
	}
	if strings.Contains(ha.Lifestyle["nutrition"].Comment, "Влад") {
		t.Errorf("field nutrition.comment accidentally touched: %q", ha.Lifestyle["nutrition"].Comment)
	}
}

func TestParseHealthAssessmentJSONStripsJunk(t *testing.T) {
	raw := "Вот ваш отчёт:\n```json\n" +
		`{"health_index":72,"summary":"ok","lifestyle":{"sleep":{"score":68,"comment":"x"}}}` +
		"\n```"
	ha, err := ParseHealthAssessmentJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ha.HealthIndex != 72 {
		t.Errorf("health_index = %d, want 72", ha.HealthIndex)
	}
	if ha.Lifestyle["sleep"].Score != 68 {
		t.Errorf("sleep.score = %d, want 68", ha.Lifestyle["sleep"].Score)
	}
}

func TestParseHealthAssessmentJSONEmpty(t *testing.T) {
	if _, err := ParseHealthAssessmentJSON("   "); err == nil {
		t.Fatal("expected error on empty JSON")
	}
}
