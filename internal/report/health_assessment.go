package report

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/theamornoir/analyzpro/internal/models"
)

// stripJSONFences убирает markdown-ограждения (```json ... ```) и случайный
// текст вокруг JSON, которые модель иногда добавляет даже при явном запросе
// «строго JSON». Без этого json.Unmarshal падает, и структурированный отчёт
// не парсится.
func stripHealthJSONFences(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "```json", "")
	s = strings.ReplaceAll(s, "```", "")
	s = strings.TrimSpace(s)
	if start := strings.Index(s, "{"); start > 0 {
		s = s[start:]
	}
	if end := strings.LastIndex(s, "}"); end >= 0 && end < len(s)-1 {
		s = s[:end+1]
	}
	return strings.TrimSpace(s)
}

// ParseHealthAssessmentJSON парсит сырой JSON-ответ ИИ в структуру
// HealthAssessment. Устойчив к markdown-обёртке и поясняющему тексту вокруг.
func ParseHealthAssessmentJSON(raw string) (models.HealthAssessment, error) {
	cleaned := stripHealthJSONFences(raw)
	if cleaned == "" {
		return models.HealthAssessment{}, fmt.Errorf("empty health assessment JSON from AI")
	}
	var ha models.HealthAssessment
	if err := json.Unmarshal([]byte(cleaned), &ha); err != nil {
		return models.HealthAssessment{}, fmt.Errorf("failed to parse health assessment JSON: %w", err)
	}
	return ha, nil
}

// RenderHealthAssessmentText формирует читаемый текстовый отчёт «Общая оценка
// здоровья» для вывода в чат Telegram (без markdown-разметки). Использует
// данные структуры HealthAssessment: общий индекс, разбор образа жизни,
// зоны риска и план на 3 месяца.
func RenderHealthAssessmentText(ha models.HealthAssessment) string {
	var b strings.Builder

	b.WriteString("🩺 Общая оценка здоровья\n\n")

	idx := ha.HealthIndex
	if idx < 0 {
		idx = 0
	}
	if idx > 100 {
		idx = 100
	}
	b.WriteString(fmt.Sprintf("📊 Общий индекс здоровья: %d из 100\n", idx))
	b.WriteString(levelLabel(idx))
	b.WriteString("\n")

	if strings.TrimSpace(ha.Summary) != "" {
		b.WriteString("🧭 Разбор образа жизни\n")
		b.WriteString(ha.Summary)
		b.WriteString("\n\n")
	}

	// Разбор по сферам (сон, питание, движение, стресс, энергия).
	b.WriteString("🌿 Оценка по сферам\n")
	order := []string{"sleep", "nutrition", "movement", "activity", "stress", "energy", "wellbeing"}
	labels := map[string]string{
		"sleep":     "Сон",
		"nutrition": "Питание",
		"movement":  "Движение",
		"activity":  "Активность",
		"stress":    "Стресс",
		"energy":    "Энергия",
		"wellbeing": "Самочувствие",
	}
	seen := map[string]bool{}
	for _, key := range order {
		if seen[key] {
			continue
		}
		dim, ok := ha.Lifestyle[key]
		if !ok {
			continue
		}
		seen[key] = true
		label := labels[key]
		if label == "" {
			label = key
		}
		b.WriteString(fmt.Sprintf("\n• %s: %d/100\n", label, dim.Score))
		if strings.TrimSpace(dim.Comment) != "" {
			b.WriteString(trimSpaces(dim.Comment) + "\n")
		}
	}
	// Любые прочие ключи lifestyle, не попавшие в известный порядок.
	for key, dim := range ha.Lifestyle {
		if seen[key] {
			continue
		}
		seen[key] = true
		label := labels[key]
		if label == "" {
			label = key
		}
		b.WriteString(fmt.Sprintf("\n• %s: %d/100\n", label, dim.Score))
		if strings.TrimSpace(dim.Comment) != "" {
			b.WriteString(trimSpaces(dim.Comment) + "\n")
		}
	}

	if len(ha.RiskZones) > 0 {
		b.WriteString("\n⚠️ Зоны риска\n")
		for _, z := range ha.RiskZones {
			name := strings.TrimSpace(z.Name)
			if name == "" {
				name = "Внимание"
			}
			b.WriteString(fmt.Sprintf("\n• %s", name))
			if strings.TrimSpace(z.Level) != "" {
				b.WriteString(fmt.Sprintf(" (%s)", z.Level))
			}
			b.WriteString("\n")
			if strings.TrimSpace(z.Description) != "" {
				b.WriteString(trimSpaces(z.Description) + "\n")
			}
		}
	}

	b.WriteString("\n🗓 Персональный план на 3 месяца\n")
	plan := ha.Plan
	if strings.TrimSpace(plan.Sleep) != "" {
		b.WriteString("\n😴 Сон\n" + trimSpaces(plan.Sleep) + "\n")
	}
	if strings.TrimSpace(plan.Nutrition) != "" {
		b.WriteString("\n🥗 Питание\n" + trimSpaces(plan.Nutrition) + "\n")
	}
	if strings.TrimSpace(plan.Movement) != "" {
		b.WriteString("\n🏃 Движение\n" + trimSpaces(plan.Movement) + "\n")
	}
	if strings.TrimSpace(plan.Stress) != "" {
		b.WriteString("\n🧘 Стресс\n" + trimSpaces(plan.Stress) + "\n")
	}

	b.WriteString("\n" + disclaimText)
	return b.String()
}

const disclaimText = "Результат носит информационный характер и не является медицинским диагнозом. При ухудшении состояния обратитесь к врачу."

// levelLabel возвращает словесную оценку общего индекса здоровья.
func levelLabel(idx int) string {
	switch {
	case idx >= 80:
		return "Отличный уровень - держите планку."
	case idx >= 65:
		return "Хороший уровень - есть куда расти."
	case idx >= 50:
		return "Средний уровень - стоит подкорректировать образ жизни."
	case idx >= 35:
		return "Сниженный уровень - нужны заметные изменения."
	default:
		return "Низкий уровень - рекомендуется обратиться к врачу."
	}
}

func trimSpaces(s string) string {
	return strings.TrimSpace(s)
}
