package gemini

import (
	"context"
	"log"
	"strings"

	"github.com/theamornoir/analyzpro/internal/ai/mock"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// GeminiClient - клиент для работы с Gemini API.
type GeminiClient struct {
	model  string
	apiKey string
}

// NewGeminiClient создаёт новый клиент Gemini.
func NewGeminiClient(apiKey, model string) *GeminiClient {
	if model == "" {
		model = "gemini-2.5-flash-latest"
	}

	model = strings.TrimPrefix(model, "models/")

	log.Printf(locales.LogGeminiClientInit, model)

	return &GeminiClient{
		model:  model,
		apiKey: apiKey,
	}
}

// GenerateAnalysisSummary - генерирует текстовый анализ по введённому тексту.
func (c *GeminiClient) GenerateAnalysisSummary(ctx context.Context, userInput string) (string, error) {
	log.Printf(locales.LogGeminiGenerateCalled, len(userInput))

	if c.isMock() {
		log.Printf(locales.LogGeminiMockAnalysis)
		return mock.MockAnalysisFromData(userInput), nil
	}

	prompt := c.buildPromptForContext(userInput)

	log.Printf(locales.LogGeminiBuiltPrompt, len(prompt))

	return c.generate(ctx, []geminiPart{{Text: prompt}})
}

// buildPromptForRegular - промпт для обычного анализа.
func (c *GeminiClient) buildPromptForRegular(text string) string {
	return locales.PromptForRegular(text)
}

// buildPromptForAthlete - промпт для спортсмена/на курсе препаратов.
func (c *GeminiClient) buildPromptForAthlete(text string, courseInfo string) string {
	return locales.PromptForAthlete(text, courseInfo)
}

// buildPromptForContext - выбирает промпт в зависимости от контекста пользователя.
func (c *GeminiClient) buildPromptForContext(userInput string) string {
	if !strings.Contains(userInput, "❗ ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:") {
		return c.buildPromptForRegular(userInput)
	}

	isAthlete := strings.Contains(userInput, "ИСПОЛЬЗУЕТ ПРЕПАРАТЫ") ||
		strings.Contains(userInput, "Вид спорта") ||
		strings.Contains(userInput, "Стаж тренировок")

	if !isAthlete {
		log.Printf(locales.LogGeminiRegularMode)
		return c.buildPromptForRegular(userInput)
	}

	courseInfo := extractCourseInfo(userInput)
	if courseInfo != "" {
		log.Printf(locales.LogGeminiAthleteMode)
		return c.buildPromptForAthlete(userInput, courseInfo)
	}

	log.Printf(locales.LogGeminiRegularMode)
	return c.buildPromptForRegular(userInput)
}

// extractCourseInfo - извлекает информацию о препаратах из контекста.
func extractCourseInfo(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.Contains(line, "❗ ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:") {
			var courseInfo strings.Builder
			for j := i + 1; j < len(lines) && j < i+20; j++ {
				if strings.TrimSpace(lines[j]) == "" {
					continue
				}
				if strings.Contains(lines[j], "•") {
					courseInfo.WriteString(strings.TrimSpace(lines[j]) + "\n")
				}
			}
			return courseInfo.String()
		}
	}
	return ""
}

// isMock - проверяет, используется ли мок-режим.
func (c *GeminiClient) isMock() bool {
	return strings.Contains(c.apiKey, "mock") || c.apiKey == ""
}
