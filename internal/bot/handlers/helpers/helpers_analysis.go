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
	parts = append(parts, fmt.Sprintf("👤 **Пациент:** %s", name))
	parts = append(parts, "")

	parts = append(parts, "❗ **ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:**")

	if gender := userData["gender"]; gender != "" {
		parts = append(parts, fmt.Sprintf("• Пол: %s", gender))
	}
	if age := userData["age"]; age != "" {
		parts = append(parts, fmt.Sprintf("• Возраст: %s лет", age))
	}

	height := userData["height"]
	weight := userData["weight"]

	if height != "" {
		parts = append(parts, fmt.Sprintf("• Рост: %s см", height))
	}
	if weight != "" {
		parts = append(parts, fmt.Sprintf("• Вес: %s кг", weight))
		if height != "" && weight != "" {
			h, _ := strconv.ParseFloat(height, 64)
			w, _ := strconv.ParseFloat(weight, 64)
			if h > 0 && w > 0 {
				bmi := w / ((h / 100) * (h / 100))
				parts = append(parts, fmt.Sprintf("• ИМТ: %.1f", bmi))
			}
		}
	}

	if chronic := userData["chronic_diseases"]; chronic != "" && strings.ToLower(chronic) != "нет" {
		parts = append(parts, fmt.Sprintf("• Хронические заболевания: %s", chronic))
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
