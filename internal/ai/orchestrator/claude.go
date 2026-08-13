package orchestrator

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
)

// ClaudeProvider — провайдер Claude через langchaingo.
type ClaudeProvider struct {
	llm *anthropic.LLM
}

// NewClaudeProvider создаёт ClaudeProvider с ключом из окружения.
func NewClaudeProvider() *ClaudeProvider {
	client, err := anthropic.New(
		anthropic.WithModel("claude-3-5-sonnet-20241022"),
	)
	if err != nil {
		return &ClaudeProvider{}
	}
	return &ClaudeProvider{llm: client}
}

// GenerateAnalysisSummary — генерирует текстовый анализ.
func (p *ClaudeProvider) GenerateAnalysisSummary(ctx context.Context, userInput string) (string, error) {
	if p.llm == nil {
		return "", fmt.Errorf("claude client not initialized")
	}

	result, err := p.llm.Call(ctx,
		"Ты — медицинский аналитик. Проанализируй данные и дай рекомендации.\n\n"+userInput,
		llms.WithMaxTokens(3000),
		llms.WithTemperature(0.2),
	)
	if err != nil {
		return "", err
	}

	return result, nil
}

// GenerateAnalysisJSON — генерирует JSON-анализ.
func (p *ClaudeProvider) GenerateAnalysisJSON(ctx context.Context, userInput string) (string, error) {
	if p.llm == nil {
		return "", fmt.Errorf("claude client not initialized")
	}

	result, err := p.llm.Call(ctx,
		"Ты — медицинский аналитик. Верни ответ в формате JSON.\n\n"+userInput,
		llms.WithMaxTokens(4000),
		llms.WithTemperature(0.1),
	)
	if err != nil {
		return "", err
	}

	return result, nil
}

// GenerateAnalysisFromFileWithContext — не поддерживается Claude (нет работы с файлами).
func (p *ClaudeProvider) GenerateAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	return "", fmt.Errorf("claude does not support file analysis")
}

// GenerateBioscanJSON — не поддерживается Claude (нет работы с фото).
func (p *ClaudeProvider) GenerateBioscanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	return "", fmt.Errorf("claude does not support bioscan photo analysis")
}

// GenerateAnalysisFromFileJSON — не поддерживается Claude (нет работы с файлами).
func (p *ClaudeProvider) GenerateAnalysisFromFileJSON(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	return "", fmt.Errorf("claude does not support file analysis")
}
