package monitoring

import (
	"encoding/json"
	"strings"
)

// ParseComparisonSummary извлекает краткое сравнение (summary) из поля
// "comparison" верхнего уровня JSON-отчёта (analysis/bioscan), если ИИ
// сформировал СРАВНИТЕЛЬНЫЙ отчёт при повторном анализе/биоскане.
// Возвращает "" если поля нет, оно пусто или JSON некорректен.
func ParseComparisonSummary(jsonStr string) string {
	s := strings.TrimSpace(jsonStr)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var doc struct {
		Comparison struct {
			Summary string `json:"summary"`
		} `json:"comparison"`
	}
	if err := json.Unmarshal([]byte(s), &doc); err != nil {
		return ""
	}
	return strings.TrimSpace(doc.Comparison.Summary)
}
