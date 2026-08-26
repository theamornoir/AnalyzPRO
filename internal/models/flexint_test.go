package models

import (
	"encoding/json"
	"testing"
)

func TestFlexIntUnmarshal(t *testing.T) {
	cases := []struct {
		name string
		json string
		want FlexInt
	}{
		{"int", `82`, 82},
		{"string int", `"82"`, 82},
		{"string float", `"7.5"`, 7},
		{"string float zero", `"7.0"`, 7},
		{"quoted spaced", `" 90 "`, 90},
		{"null", `null`, 0},
		{"empty", `""`, 0},
		{"negative", `-5`, -5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var f FlexInt
			if err := json.Unmarshal([]byte(c.json), &f); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if f != c.want {
				t.Fatalf("got %d, want %d", f, c.want)
			}
		})
	}
}

// TestFlexIntInReport проверяет, что models.Report переживает score,
// присланный моделью строкой (реальный баг: json: cannot unmarshal string
// into Go struct field Zone.zones.score of type int).
func TestFlexIntInReport(t *testing.T) {
	const raw = `{
		"score": "0",
		"level": "Недостаточно данных",
		"zones": [
			{"name": "Плечи", "score": "88", "status": "good"}
		]
	}`
	var rep Report
	if err := json.Unmarshal([]byte(raw), &rep); err != nil {
		t.Fatalf("unmarshal with string score: %v", err)
	}
	if rep.Zones[0].Score != 88 {
		t.Fatalf("zone score = %d, want 88", rep.Zones[0].Score)
	}
}
