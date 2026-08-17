package gemini

import (
	"context"
	"encoding/base64"
	"log"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// GenerateAnalysisJSON - генерирует JSON-структурированный анализ по тексту.
func (c *GeminiClient) GenerateAnalysisJSON(ctx context.Context, userInput string) (string, error) {
	if c.isMock() {
		log.Printf(locales.LogGeminiMockAnalysisJSON)
		return getMockAnalysisJSON(userInput), nil
	}

	prompt := c.buildAnalysisJSONPrompt(userInput)
	parts := []geminiPart{{Text: prompt}}

	result, err := c.generateRaw(ctx, parts)
	if err != nil {
		return "", err
	}

	return normalizeJSONResponse(result), nil
}

// GenerateAnalysisFromFileJSON - генерирует JSON-анализ из файла.
func (c *GeminiClient) GenerateAnalysisFromFileJSON(
	ctx context.Context,
	data []byte,
	mimeType string,
	contextText string,
) (string, error) {
	if c.isMock() {
		log.Printf(locales.LogGeminiMockAnalysisFileJSON)
		return getMockAnalysisJSON(contextText), nil
	}

	prompt := c.buildAnalysisJSONPrompt(contextText)
	parts := []geminiPart{
		{
			InlineData: &geminiInlineData{
				MimeType: mimeType,
				Data:     base64.StdEncoding.EncodeToString(data),
			},
		},
		{Text: prompt},
	}

	result, err := c.generateRaw(ctx, parts)
	if err != nil {
		return "", err
	}

	return normalizeJSONResponse(result), nil
}

// buildAnalysisJSONPrompt - промпт для JSON-анализа.
func (c *GeminiClient) buildAnalysisJSONPrompt(text string) string {
	return locales.PromptForAnalysisJSON(text)
}

// GenerateDossierJSON - генерирует JSON универсального отчёта-досье здоровья.
func (c *GeminiClient) GenerateDossierJSON(ctx context.Context, userInput string) (string, error) {
	if c.isMock() {
		log.Printf(locales.LogGeminiMockDossier)
		return locales.MockDossierJSON, nil
	}

	prompt := locales.PromptForDossierJSON(userInput)
	parts := []geminiPart{{Text: prompt}}

	result, err := c.generateRaw(ctx, parts)
	if err != nil {
		return "", err
	}

	return normalizeJSONResponse(result), nil
}

// getMockAnalysisJSON - мок-ответ для JSON-анализа.
func getMockAnalysisJSON(_ string) string {
	return locales.MockAnalysisJSON
}
