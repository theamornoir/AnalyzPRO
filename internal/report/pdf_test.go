package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/models"
)

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
