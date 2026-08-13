package orchestrator

import (
	"context"
	"fmt"
	"os"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// DeepSeekProvider — провайдер DeepSeek через OpenAI-совместимый API.
type DeepSeekProvider struct {
	client openai.Client
}

// NewDeepSeekProvider создаёт DeepSeekProvider с ключом из окружения.
func NewDeepSeekProvider() *DeepSeekProvider {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		apiKey = "sk-dummy" // dummy key for mock mode
	}
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL("https://api.deepseek.com/v1"),
	)
	return &DeepSeekProvider{client: client}
}

// GenerateAnalysisSummary — генерирует текстовый анализ.
func (p *DeepSeekProvider) GenerateAnalysisSummary(ctx context.Context, userInput string) (string, error) {
	stream := p.client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model: "deepseek-chat",
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("Ты — медицинский аналитик. Проанализируй данные и дай рекомендации."),
			openai.UserMessage(userInput),
		},
		Temperature: openai.Float(0.2),
		MaxTokens:   openai.Int(3000),
	})

	var fullText string
	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) > 0 {
			fullText += chunk.Choices[0].Delta.Content
		}
	}

	if err := stream.Err(); err != nil {
		return "", err
	}

	return fullText, nil
}

// GenerateAnalysisJSON — генерирует JSON-анализ.
func (p *DeepSeekProvider) GenerateAnalysisJSON(ctx context.Context, userInput string) (string, error) {
	stream := p.client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model: "deepseek-chat",
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("Ты — медицинский аналитик. Верни ответ в формате JSON."),
			openai.UserMessage(userInput),
		},
		Temperature: openai.Float(0.1),
		MaxTokens:   openai.Int(4000),
	})

	var fullText string
	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) > 0 {
			fullText += chunk.Choices[0].Delta.Content
		}
	}

	if err := stream.Err(); err != nil {
		return "", err
	}

	return fullText, nil
}

// GenerateAnalysisFromFileWithContext — не поддерживается DeepSeek (нет работы с файлами).
func (p *DeepSeekProvider) GenerateAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	return "", fmt.Errorf("deepseek does not support file analysis")
}

// GenerateBioscanJSON — не поддерживается DeepSeek (нет работы с фото).
func (p *DeepSeekProvider) GenerateBioscanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	return "", fmt.Errorf("deepseek does not support bioscan photo analysis")
}

// GenerateAnalysisFromFileJSON — не поддерживается DeepSeek (нет работы с файлами).
func (p *DeepSeekProvider) GenerateAnalysisFromFileJSON(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	return "", fmt.Errorf("deepseek does not support file analysis")
}
