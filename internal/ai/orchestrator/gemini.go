package orchestrator

import (
	"context"
	"os"

	"github.com/theamornoir/analyzpro/internal/ai/gemini"
)

// GeminiProvider — обёртка над GeminiClient для оркестратора.
type GeminiProvider struct {
	client *gemini.GeminiClient
}

// NewGeminiProvider создаёт GeminiProvider с ключом из окружения.
// Возвращает nil если ключ пустой — провайдер не будет добавлен в оркестратор.
func NewGeminiProvider() *GeminiProvider {
	apiKey := os.Getenv("GOOGLE_GEMINI_API_KEY")
	if apiKey == "" {
		return nil
	}
	model := os.Getenv("GOOGLE_AI_MODEL")
	if model == "" {
		model = "gemini-2.5-flash-latest"
	}
	return &GeminiProvider{
		client: gemini.NewGeminiClient(apiKey, model),
	}
}

// GenerateAnalysisSummary — генерирует текстовый анализ.
func (p *GeminiProvider) GenerateAnalysisSummary(ctx context.Context, userInput string) (string, error) {
	return p.client.GenerateAnalysisSummary(ctx, userInput)
}

// GenerateAnalysisJSON — генерирует JSON-анализ.
func (p *GeminiProvider) GenerateAnalysisJSON(ctx context.Context, userInput string) (string, error) {
	return p.client.GenerateAnalysisJSON(ctx, userInput)
}

// GenerateAnalysisFromFileWithContext — анализирует файл с контекстом.
func (p *GeminiProvider) GenerateAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	return p.client.GenerateAnalysisFromFileWithContext(ctx, data, mimeType, contextText)
}

// GenerateBioscanJSON — генерирует JSON bioscan по фото.
func (p *GeminiProvider) GenerateBioscanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	return p.client.GenerateBioscanJSON(ctx, photosData, mimeType, contextInfo)
}

// GenerateAnalysisFromFileJSON — генерирует JSON-анализ из файла.
func (p *GeminiProvider) GenerateAnalysisFromFileJSON(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	return p.client.GenerateAnalysisFromFileJSON(ctx, data, mimeType, contextText)
}
