package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/models"
)

// TestRenderBodyScanFromMock checks the premium Bioscan PRO (Body Intelligence)
// report renders from mock JSON without panics and contains the key sections.
func TestRenderBodyScanFromMock(t *testing.T) {
	var rep models.BodyScanReport
	if err := json.Unmarshal([]byte(locales.MockBodyScanJSON), &rep); err != nil {
		t.Fatalf("unmarshal mock body scan JSON: %v", err)
	}
	if rep.Title == "" {
		t.Fatalf("expected non-empty title, got %q", rep.Title)
	}
	if len(rep.CoverMetrics) == 0 {
		t.Fatalf("expected cover metrics, got 0")
	}
	if len(rep.Zones) == 0 {
		t.Fatalf("expected zones, got 0")
	}
	if len(rep.Recommendations) == 0 {
		t.Fatalf("expected recommendations, got 0")
	}

	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	html, err := r.RenderBodyScan(rep)
	if err != nil {
		t.Fatalf("RenderBodyScan: %v", err)
	}

	for _, want := range []string{
		rep.Title,
		rep.Name,
		"Развитый плечевой пояс",
		"Плечи",
		"Осанка",
		"Тренировки",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered body scan missing %q", want)
		}
	}
}
