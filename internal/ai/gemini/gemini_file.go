package gemini

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"

	"github.com/theamornoir/analyzpro/internal/ai/mock"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// GenerateAnalysisFromFileWithContext - анализ файла с учётом контекста пользователя.
func (c *GeminiClient) GenerateAnalysisFromFileWithContext(
	ctx context.Context,
	data []byte,
	mimeType string,
	contextText string,
) (string, error) {

	if c.isMock() {
		log.Printf(locales.LogGeminiMockFileWithContext)

		if strings.Contains(contextText, "❗ ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:") &&
			(strings.Contains(contextText, "ИСПОЛЬЗУЕТ ПРЕПАРАТЫ") ||
				strings.Contains(contextText, "Вид спорта") ||
				strings.Contains(contextText, "Стаж тренировок")) {
			return mock.MockAnalysisWithContext(contextText), nil
		}

		return getMockAnalysisFromFileData(data, mimeType, contextText), nil
	}

	if strings.TrimSpace(c.apiKey) == "" {
		return noKeyFallback(), nil
	}

	if len(data) == 0 {
		return "", fmt.Errorf("empty file data")
	}

	prompt := c.buildPromptForContext(contextText)

	parts := []geminiPart{
		{
			InlineData: &geminiInlineData{
				MimeType: mimeType,
				Data:     base64.StdEncoding.EncodeToString(data),
			},
		},
		{Text: prompt},
	}

	return c.generate(ctx, parts)
}

// getMockAnalysisFromFileData - мок-ответ для файла.
func getMockAnalysisFromFileData(data []byte, mimeType string, contextText string) string {
	if !strings.Contains(mimeType, "text") && len(data) > 0 {
		return mock.MockAnalysisFromData(contextText)
	}

	content := string(data)
	return mock.MockAnalysisFromData(content)
}
