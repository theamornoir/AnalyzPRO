package helpers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// BuildAnalysisText - формирует текст с учётом данных пользователя.
func BuildAnalysisText(userData map[string]string) string {
	var parts []string

	name := userData["name"]
	if name == "" {
		name = locales.MsgUserDefaultName
	}
	parts = append(parts, fmt.Sprintf(locales.MsgAnalysisPatient, name))
	parts = append(parts, "")

	parts = append(parts, locales.MsgAnalysisImportantInfo)

	if gender := userData["gender"]; gender != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgAnalysisGender, gender))
	}
	if age := userData["age"]; age != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgAnalysisAge, age))
	}

	height := userData["height"]
	weight := userData["weight"]

	if height != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgAnalysisHeight, height))
	}
	if weight != "" {
		parts = append(parts, fmt.Sprintf(locales.MsgAnalysisWeight, weight))
		if height != "" && weight != "" {
			h, _ := strconv.ParseFloat(height, 64)
			w, _ := strconv.ParseFloat(weight, 64)
			if h > 0 && w > 0 {
				bmi := w / ((h / 100) * (h / 100))
				parts = append(parts, fmt.Sprintf(locales.MsgAnalysisBMI, bmi))
			}
		}
	}

	if chronic := userData["chronic_diseases"]; chronic != "" && strings.ToLower(chronic) != "нет" {
		parts = append(parts, fmt.Sprintf(locales.MsgAnalysisChronic, chronic))
	}
	if allergies := userData["allergies"]; allergies != "" && strings.ToLower(allergies) != "нет" {
		parts = append(parts, fmt.Sprintf("• Аллергии: %s", allergies))
	}
	if medications := userData["medications"]; medications != "" && strings.ToLower(medications) != "нет" {
		parts = append(parts, fmt.Sprintf("• Принимаемые лекарства: %s", medications))
	}
	if smoking := userData["smoking"]; smoking != "" && strings.ToLower(smoking) != "нет" {
		parts = append(parts, fmt.Sprintf("• Курение: %s", smoking))
	}
	if alcohol := userData["alcohol"]; alcohol != "" {
		parts = append(parts, fmt.Sprintf("• Алкоголь: %s", alcohol))
	}
	if sport := userData["sport_type"]; sport != "" {
		parts = append(parts, fmt.Sprintf("• Вид спорта: %s", sport))
	}
	if exp := userData["training_experience"]; exp != "" {
		parts = append(parts, fmt.Sprintf("• Стаж тренировок: %s лет", exp))
	}
	if goal := userData["goal"]; goal != "" {
		parts = append(parts, fmt.Sprintf("• Цель: %s", goal))
	}

	// --- Блок образа жизни (20-вопросный опросник) ---
	if sleep := userData["sleep"]; sleep != "" {
		parts = append(parts, fmt.Sprintf("• Сон: %s", sleep))
	}
	if stress := userData["stress"]; stress != "" {
		parts = append(parts, fmt.Sprintf("• Уровень стресса: %s", stress))
	}
	if veg := userData["nutrition_veg"]; veg != "" {
		parts = append(parts, fmt.Sprintf("• Овощи/фрукты: %s", veg))
	}
	if proc := userData["nutrition_processed"]; proc != "" {
		parts = append(parts, fmt.Sprintf("• Ультраобработанные продукты: %s", proc))
	}
	if water := userData["water"]; water != "" {
		parts = append(parts, fmt.Sprintf("• Питьевой режим: %s", water))
	}
	if activity := userData["activity"]; activity != "" {
		parts = append(parts, fmt.Sprintf("• Физическая активность: %s", activity))
	}
	if family := userData["family_history"]; family != "" && strings.ToLower(family) != "нет" {
		parts = append(parts, fmt.Sprintf("• Семейный анамнез: %s", family))
	}
	if digestion := userData["digestion"]; digestion != "" && strings.ToLower(digestion) != "нет" {
		parts = append(parts, fmt.Sprintf("• ЖКТ / пищеварение: %s", digestion))
	}

	onCourse := userData["on_course"]
	if onCourse == "yes" {
		courseInfo := userData["course_info"]
		if courseInfo != "" {
			parts = append(parts, fmt.Sprintf("• ИСПОЛЬЗУЕТ ПРЕПАРАТЫ: %s", courseInfo))
		} else {
			parts = append(parts, "• ИСПОЛЬЗУЕТ ПРЕПАРАТЫ (информация не указана)")
		}
		parts = append(parts, "• Требуется интерпретация с учетом приема препаратов")
		parts = append(parts, "• Оценить влияние на гормональный фон и показатели")
	} else if onCourse == "no" {
		parts = append(parts, "• Без препаратов (естественный фон)")
	}

	return strings.Join(parts, "\n")
}
