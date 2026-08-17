package report

import (
	"strconv"
	"strings"

	"github.com/theamornoir/analyzpro/internal/models"
)

// RenderBioscanPlainText формирует plain-text (без markdown/форматирования)
// версию отчёта Bioscan для вывода обычным сообщением в чат. Используется в
// базовом (бесплатном) режиме Bioscan, где результат приходит текстом.
func RenderBioscanPlainText(rep models.Report) string {
	var b strings.Builder

	b.WriteString("BIOSCAN — результат анализа тела\n")
	b.WriteString("————————————————————\n\n")

	if rep.Score > 0 || rep.Level != "" {
		if rep.Score > 0 {
			b.WriteString("Общая оценка: " + strconv.Itoa(rep.Score))
		}
		if rep.Level != "" {
			if rep.Score > 0 {
				b.WriteString("  ·  ")
			}
			b.WriteString(rep.Level)
		}
		b.WriteString("\n\n")
	}

	body := []string{}
	if rep.Body.Height != "" {
		body = append(body, "Рост: "+rep.Body.Height)
	}
	if rep.Body.Weight != "" {
		body = append(body, "Вес: "+rep.Body.Weight)
	}
	if rep.Body.MuscleMass != "" {
		body = append(body, "Мышечная масса: "+rep.Body.MuscleMass)
	}
	if rep.Body.Fat != "" {
		body = append(body, "Процент жира: "+rep.Body.Fat)
	}
	if len(body) > 0 {
		b.WriteString(strings.Join(body, "\n") + "\n\n")
	}

	if rep.Composition != "" {
		b.WriteString("Композиция тела:\n" + rep.Composition + "\n\n")
	}

	if rep.Profile.Composition > 0 || rep.Profile.MuscleDevelopment > 0 ||
		rep.Profile.Balance > 0 || rep.Profile.Potential > 0 {
		b.WriteString("Профиль развития:\n")
		b.WriteString("• Композиция: " + strconv.Itoa(rep.Profile.Composition) + "/100\n")
		b.WriteString("• Развитие мышц: " + strconv.Itoa(rep.Profile.MuscleDevelopment) + "/100\n")
		b.WriteString("• Баланс: " + strconv.Itoa(rep.Profile.Balance) + "/100\n")
		b.WriteString("• Потенциал: " + strconv.Itoa(rep.Profile.Potential) + "/100\n\n")
	}

	if rep.Summary != "" {
		b.WriteString("Резюме:\n" + rep.Summary + "\n\n")
	}

	if len(rep.Zones) > 0 {
		b.WriteString("Оценка зон тела:\n")
		for _, z := range rep.Zones {
			b.WriteString("• " + z.Name + " — " + strconv.Itoa(z.Score) + "/100")
			if z.Status != "" {
				b.WriteString(" (" + z.Status + ")")
			}
			b.WriteString("\n")
			if z.Recommendation != "" {
				b.WriteString("  Рекомендация: " + z.Recommendation + "\n")
			}
		}
		b.WriteString("\n")
	}

	if rep.Posture.Type != "" || rep.Posture.Description != "" {
		b.WriteString("Осанка:\n")
		if rep.Posture.Type != "" {
			b.WriteString("• Тип: " + rep.Posture.Type + "\n")
		}
		if rep.Posture.Head != "" {
			b.WriteString("• Голова: " + rep.Posture.Head + "\n")
		}
		if rep.Posture.Shoulders != "" {
			b.WriteString("• Плечи: " + rep.Posture.Shoulders + "\n")
		}
		if rep.Posture.Pelvis != "" {
			b.WriteString("• Таз: " + rep.Posture.Pelvis + "\n")
		}
		if rep.Posture.Description != "" {
			b.WriteString("• " + rep.Posture.Description + "\n")
		}
		b.WriteString("\n")
	}

	if len(rep.AttentionZones) > 0 {
		b.WriteString("Зоны внимания:\n")
		for _, a := range rep.AttentionZones {
			b.WriteString("• " + a.Name + "\n")
			if a.Problem != "" {
				b.WriteString("  Проблема: " + a.Problem + "\n")
			}
			if a.Solution != "" {
				b.WriteString("  Решение: " + a.Solution + "\n")
			}
		}
		b.WriteString("\n")
	}

	if len(rep.Recommendations) > 0 {
		b.WriteString("Рекомендации:\n")
		for _, r := range rep.Recommendations {
			b.WriteString("• " + r + "\n")
		}
		b.WriteString("\n")
	}

	if len(rep.Progress.Targets) > 0 || rep.Progress.Recheck != "" {
		b.WriteString("Контроль прогресса:\n")
		if rep.Progress.Recheck != "" {
			b.WriteString("• Повторная проверка: " + rep.Progress.Recheck + "\n")
		}
		for _, t := range rep.Progress.Targets {
			b.WriteString("• " + t + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("⚠️ Отчёт носит информационный характер и не заменяет консультацию врача.")

	return b.String()
}
