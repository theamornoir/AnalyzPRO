package mock

import (
	"fmt"
	"strings"
	"time"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// MockAnalysisFromData - мок-ответ с 10 показателями разных статусов.
func MockAnalysisFromData(_ string) string {
	var result strings.Builder

	result.WriteString(locales.MockResultsHeader)
	result.WriteString(fmt.Sprintf(locales.MockDateFormat, time.Now().Format("02.01.2006")))

	result.WriteString(locales.MockBloodSection)

	indicators := locales.MockIndicators

	for _, ind := range indicators {
		result.WriteString(ind)
		result.WriteString("\n\n")
	}

	// ЗАКЛЮЧЕНИЕ
	result.WriteString(locales.MockConclusionHeader)
	result.WriteString(locales.MockConclusionText)

	// СТАТИСТИКА
	total := len(indicators)
	normal := 0
	warnings := 0
	critical := 0
	var attention []string

	for _, ind := range indicators {
		lines := strings.Split(ind, "\n")
		name := ""

		if len(lines) > 0 {
			name = strings.TrimSpace(lines[0])
			name = strings.ReplaceAll(name, "✅", "")
			name = strings.ReplaceAll(name, "⚠️", "")
			name = strings.ReplaceAll(name, "❌", "")

			if idx := strings.Index(name, ":"); idx > 0 {
				name = strings.TrimSpace(name[:idx])
			}
		}

		switch {
		case strings.Contains(ind, "❌"):
			critical++
			attention = append(attention, name)
		case strings.Contains(ind, "⚠️"):
			warnings++
			attention = append(attention, name)
		default:
			normal++
		}
	}

	result.WriteString(locales.MockOverallGrade)
	result.WriteString(fmt.Sprintf(locales.MockStatsTotalFormat, total))
	result.WriteString(fmt.Sprintf(locales.MockStatsNormalFormat, normal))

	if warnings > 0 {
		result.WriteString(fmt.Sprintf(locales.MockStatsWarnFormat, warnings))
	}
	if critical > 0 {
		result.WriteString(fmt.Sprintf(locales.MockStatsCritFormat, critical))
	}

	// ЧТО ПОПРАВИТЬ
	if len(attention) > 0 {
		result.WriteString(locales.MockCheckSection)
		for _, item := range attention {
			result.WriteString("• ")
			result.WriteString(item)
			result.WriteString("\n")
		}
	}

	// РЕКОМЕНДАЦИИ
	result.WriteString(locales.MockRecommendations)

	if critical > 0 {
		result.WriteString(locales.MockRecCritical)
	} else if warnings > 0 {
		result.WriteString(locales.MockRecWarnings)
	} else {
		result.WriteString(locales.MockRecNormal)
	}

	result.WriteString(locales.MockDisclaimer)

	return result.String()
}
