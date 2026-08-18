package router

import (
	"strings"
	"testing"

	"github.com/theamornoir/analyzpro/internal/report"
)

func TestDemoRenderingsDoNotPanic(t *testing.T) {
	rend, err := report.NewRenderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}

	html, err := rend.Render(demoAnalysisReport())
	if err != nil {
		t.Fatalf("analysis render: %v", err)
	}
	if !strings.Contains(html, "Гемоглобин") {
		t.Fatalf("analysis HTML missing content")
	}

	txt := report.RenderBioscanPlainText(demoBioscanReport())
	if !strings.Contains(txt, "Плечи") {
		t.Fatalf("bioscan text missing zone content")
	}

	pdf, err := report.RenderBioscanPDF(demoBioscanReport())
	if err != nil {
		t.Fatalf("bioscan pdf: %v", err)
	}
	if len(pdf) < 8 || string(pdf[:5]) != "%PDF-" {
		t.Fatalf("bioscan pdf invalid: len=%d", len(pdf))
	}
}
