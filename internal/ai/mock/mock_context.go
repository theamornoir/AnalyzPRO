package mock

import (
	"fmt"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// MockAnalysisWithContext - мок-ответ для расширенного анализа с контекстом.
func MockAnalysisWithContext(context string) string {
	return fmt.Sprintf(locales.MockAnalysisWithContextTemplate, context)
}
