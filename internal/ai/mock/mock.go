package mock

import "github.com/theamornoir/analyzpro/internal/locales"

// MockAnalysis - базовый мок-ответ анализа.
func MockAnalysis(_ string) string {
	return locales.MockAnalysisText
}
