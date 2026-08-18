package dashboard

import (
	"testing"
	"time"

	"github.com/theamornoir/analyzpro/internal/monitoring"
)

func TestParseReportBlockBioscan(t *testing.T) {
	jsonStr := `{
		"title": "Bioscan PRO",
		"score": 86,
		"summary": "Телосложение укрепилось.",
		"posture": {"posture_score": 84, "symmetry": 82, "shoulder_balance": 80, "pelvic_balance": 78, "spinal_alignment": 86, "mobility": 74, "stability": 81},
		"zones": [
			{"name": "Плечи", "score": 88, "status": "good", "comment": "Сбалансировано"},
			{"name": "Пресс", "score": 74, "status": "warning", "comment": "Требует проработки"}
		]
	}`

	block := parseReportBlock(monitoring.HistoryEntry{JsonData: jsonStr, Date: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)}, "bioscan")
	if !block.Available {
		t.Fatal("expected block to be available")
	}
	if block.MainScore != 86 {
		t.Errorf("MainScore = %d, want 86", block.MainScore)
	}
	if block.ScoreLabel != "Body Score" {
		t.Errorf("ScoreLabel = %q, want Body Score", block.ScoreLabel)
	}
	if len(block.Zones) != 2 {
		t.Fatalf("Zones len = %d, want 2", len(block.Zones))
	}
	if block.Zones[0].Score != 88 || block.Zones[0].Status != "good" {
		t.Errorf("zone0 = %+v", block.Zones[0])
	}
	if block.Scores["Осанка"] != 84 || block.Scores["Симметрия"] != 82 {
		t.Errorf("posture scores missing: %+v", block.Scores)
	}
}

func TestParseReportBlockAnalysisSections(t *testing.T) {
	jsonStr := `{
		"title": "Расширенный анализ",
		"sections": [
			{"type": "blood", "title": "ОАК", "indicators": [
				{"name": "Гемоглобин", "value": "152", "status": "normal", "score": 85},
				{"name": "Глюкоза", "value": "5.2", "status": "normal", "score": 90}
			]}
		]
	}`

	block := parseReportBlock(monitoring.HistoryEntry{JsonData: jsonStr, Date: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)}, "analysis")
	if !block.Available {
		t.Fatal("expected block to be available")
	}
	if len(block.Indicators) != 2 {
		t.Fatalf("Indicators len = %d, want 2", len(block.Indicators))
	}
	if block.Scores["Гемоглобин"] != 85 || block.Scores["Глюкоза"] != 90 {
		t.Errorf("indicator scores missing: %+v", block.Scores)
	}
	if block.MainScore != 87 {
		t.Errorf("MainScore = %d, want 87 (avg 85/90)", block.MainScore)
	}
}

func TestParseReportBlockEmpty(t *testing.T) {
	block := parseReportBlock(monitoring.HistoryEntry{JsonData: "", Date: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)}, "analysis")
	if block.Available {
		t.Error("empty JSON should not be available")
	}
	if len(block.Scores) != 0 {
		t.Error("scores should be empty")
	}
}

func TestParseRefRange(t *testing.T) {
	cases := []struct {
		in      string
		wantMin float64
		wantMax float64
		wantOK  bool
	}{
		{"0-100", 0, 100, true},
		{"0 - 100", 0, 100, true},
		{"0.00 – 100.00 Ед/мл", 0, 100, true},
		{"(3.9-6.1)", 3.9, 6.1, true},
		{"менее 100", 0, 100, true},
		{"более 200", 200, 400, true},
		{"", 0, 0, false},
		{"нет данных", 0, 0, false},
	}
	for _, c := range cases {
		mn, mx, ok := parseRefRange(c.in)
		if ok != c.wantOK {
			t.Errorf("parseRefRange(%q) ok=%v want %v", c.in, ok, c.wantOK)
			continue
		}
		if ok && (mn != c.wantMin || mx != c.wantMax) {
			t.Errorf("parseRefRange(%q) = (%.2f,%.2f) want (%.2f,%.2f)", c.in, mn, mx, c.wantMin, c.wantMax)
		}
	}
}

func TestParseIndicatorRefImmunoglobulin(t *testing.T) {
	num, refMin, refMax := parseIndicatorRef("255.00 Ед/мл", "0.00 – 100.00 Ед/мл")
	if num != 255 {
		t.Errorf("num = %v, want 255", num)
	}
	if refMin != 0 || refMax != 100 {
		t.Errorf("ref = (%.2f,%.2f), want (0,100)", refMin, refMax)
	}
}

func TestParseAnalysisBlockIndicatorRef(t *testing.T) {
	jsonStr := `{
		"title": "Анализ",
		"categories": [
			{"name": "Иммунология", "indicators": [
				{"name": "Иммуноглобулин E", "value": "255.00 Ед/мл", "status": "critical", "normal": "0.00 – 100.00 Ед/мл"}
			]}
		]
	}`
	block := parseReportBlock(monitoring.HistoryEntry{JsonData: jsonStr, Date: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)}, "analysis")
	if len(block.Indicators) != 1 {
		t.Fatalf("Indicators len = %d, want 1", len(block.Indicators))
	}
	ind := block.Indicators[0]
	if ind.Num != 255 || ind.RefMin != 0 || ind.RefMax != 100 {
		t.Errorf("indicator ref = (%.2f,%.2f,%.2f), want (255,0,100)", ind.Num, ind.RefMin, ind.RefMax)
	}
	if ind.Normal != "0.00 – 100.00 Ед/мл" {
		t.Errorf("Normal = %q", ind.Normal)
	}
}
