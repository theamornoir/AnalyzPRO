package bioscan

import (
	"fmt"
	"strings"
)

// BuildBioscanText - строит блок «Данные пользователя» из опросника
// Bioscan PRO (образ жизни, тренировки, травмы, питание, привычки) поверх
// базовых полей (имя/возраст/рост/вес/цель). Этот текст подставляется в
// промпт ИИ вместе с фотографиями, чтобы отчёт Body Intelligence учитывал не
// только фото, но и анкету пользователя.
func BuildBioscanText(userData map[string]string) string {
	var parts []string

	name := strings.TrimSpace(userData["bioscan_name"])
	age := strings.TrimSpace(userData["bioscan_age"])
	height := strings.TrimSpace(userData["bioscan_height"])
	weight := strings.TrimSpace(userData["bioscan_weight"])
	goal := strings.TrimSpace(userData["bioscan_goal"])

	parts = append(parts, "Данные пользователя (опросник Bioscan PRO):")
	if name != "" {
		parts = append(parts, fmt.Sprintf("• Имя: %s", name))
	}
	if age != "" {
		parts = append(parts, fmt.Sprintf("• Возраст: %s лет", age))
	}
	if height != "" {
		parts = append(parts, fmt.Sprintf("• Рост: %s см", height))
	}
	if weight != "" {
		parts = append(parts, fmt.Sprintf("• Вес: %s кг", weight))
	}
	if goal != "" {
		parts = append(parts, fmt.Sprintf("• Цель: %s", goal))
	}

	// Ответы опросника об образе жизни, спорте и здоровье.
	parts = append(parts, buildBioscanQuestionnaireLines(userData)...)

	return strings.Join(parts, "\n")
}

// buildBioscanQuestionnaireLines - собирает строки ответов опросника в том же
// порядке, в каком задавались вопросы. Пустые ответы пропускаются.
func buildBioscanQuestionnaireLines(userData map[string]string) []string {
	labels := map[string]string{
		"bioscan_training_exp":      "Стаж тренировок",
		"bioscan_training_freq":     "Частота тренировок",
		"bioscan_training_type":     "Виды тренировок",
		"bioscan_injuries":          "Травмы и боли",
		"bioscan_posture_issues":    "Проблемы с осанкой",
		"bioscan_improve_zones":     "Зоны для проработки",
		"bioscan_mobility":          "Гибкость и мобильность",
		"bioscan_recovery":          "Восстановление",
		"bioscan_sleep":             "Сон",
		"bioscan_stress":            "Уровень стресса",
		"bioscan_nutrition":         "Питание",
		"bioscan_protein":           "Белок",
		"bioscan_water":             "Питьевой режим",
		"bioscan_smoking":           "Курение",
		"bioscan_alcohol":           "Алкоголь",
		"bioscan_sedentary":         "Сидячий образ жизни",
		"bioscan_body_fat_goal":     "Цель по композиции",
		"bioscan_diet_restrictions": "Ограничения в питании",
	}

	var lines []string
	for _, q := range bioscanQuestionnaire {
		answer := strings.TrimSpace(userData[q.key])
		if answer == "" {
			continue
		}
		if label, ok := labels[q.key]; ok {
			lines = append(lines, fmt.Sprintf("• %s: %s", label, answer))
		} else {
			lines = append(lines, fmt.Sprintf("• %s", answer))
		}
	}
	return lines
}
