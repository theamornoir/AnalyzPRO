package ai

import (
	"fmt"
	"strings"
)

func BuildDoctorTemplate(summary, important, risks, recommendation, footnote string) string {
	return fmt.Sprintf("📌 Краткий вывод\n%s\n\n🩺 Что важно\n- %s\n\n⚠️ Показатели, требующие внимания\n- %s\n\n✅ Рекомендации\n1. %s\n\nℹ️ Важно\n%s", summary, important, risks, recommendation, footnote)
}

func normalizeAIResponse(raw string) string {
	clean := strings.TrimSpace(raw)
	clean = strings.Trim(clean, "```")
	clean = strings.TrimSpace(clean)
	return clean
}
