package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/models"
)

// TestRenderDossierFromMock checks the universal health-dossier renders from
// mock JSON without panics and contains the key sections.
func TestRenderDossierFromMock(t *testing.T) {
	var d models.HealthDossier
	if err := json.Unmarshal([]byte(locales.MockDossierJSON), &d); err != nil {
		t.Fatalf("unmarshal mock dossier JSON: %v", err)
	}
	if len(d.Survey) != 20 {
		t.Fatalf("expected 20 survey questions, got %d", len(d.Survey))
	}
	if len(d.Lifestyle) != 5 {
		t.Fatalf("expected 5 lifestyle sections, got %d", len(d.Lifestyle))
	}

	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	html, err := r.RenderDossier(d)
	if err != nil {
		t.Fatalf("RenderDossier: %v", err)
	}

	for _, want := range []string{
		"Персональный профиль здоровья",
		"Результаты анкетирования",
		"Сон и восстановление",
		"Углеводный обмен",
		"Гематология",
		"Общая картина",
		"Персональные рекомендации",
		"Научная база",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered dossier missing section %q", want)
		}
	}
}
