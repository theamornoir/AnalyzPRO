package ai

import "fmt"

// BuildDoctorTemplate - возвращает шаблон для ошибок
func BuildDoctorTemplate(shortSummary, important, warnings, recommendations, disclaimer string) string {
	return fmt.Sprintf(`📌 Краткий вывод
%s

🩺 Что важно
%s

⚠️ Показатели, требующие внимания
%s

✅ Рекомендации
%s

ℹ️ Важно
%s`, shortSummary, important, warnings, recommendations, disclaimer)
}
