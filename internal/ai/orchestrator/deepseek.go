package orchestrator

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// DeepSeekProvider - провайдер DeepSeek через OpenAI-совместимый API.
// Поддерживает мультимодальный (vision) анализ изображений через deepseek-chat,
// поэтому используется как рабочий фоллбэк, когда Gemini недоступен (например, гео-блок).
type DeepSeekProvider struct {
	client openai.Client
}

// NewDeepSeekProvider создаёт DeepSeekProvider с ключом из окружения.
// Возвращает nil если ключ пустой.
func NewDeepSeekProvider() *DeepSeekProvider {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return nil
	}
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL("https://api.deepseek.com/v1"),
	)
	return &DeepSeekProvider{client: client}
}

// GenerateAnalysisSummary - генерирует текстовый анализ.
func (p *DeepSeekProvider) GenerateAnalysisSummary(ctx context.Context, userInput string) (string, error) {
	stream := p.client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model: "deepseek-chat",
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("Ты - медицинский аналитик. Проанализируй данные и дай рекомендации."),
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

// GenerateAnalysisJSON - генерирует JSON-анализ.
func (p *DeepSeekProvider) GenerateAnalysisJSON(ctx context.Context, userInput string) (string, error) {
	stream := p.client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model: "deepseek-chat",
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("Ты - медицинский аналитик. Верни ответ в формате JSON."),
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

// GenerateAnalysisFromFileWithContext - анализирует изображение анализа с контекстом.
// DeepSeek vision поддерживает только изображения; для остальных типов вернёт ошибку,
// и оркестратор перейдёт к следующему провайдеру.
func (p *DeepSeekProvider) GenerateAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	if !isImageMime(mimeType) {
		return "", fmt.Errorf("deepseek supports only image files for analysis")
	}
	return p.analyzeImagesVision(ctx, []visionImage{{data: data, mimeType: mimeType}}, contextText, false, 4000)
}

// GenerateBioscanJSON - анализирует фото для bioscan и возвращает JSON.
func (p *DeepSeekProvider) GenerateBioscanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	imgs := make([]visionImage, 0, len(photosData))
	for _, d := range photosData {
		if len(d) > 0 {
			imgs = append(imgs, visionImage{data: d, mimeType: mimeType})
		}
	}
	if len(imgs) == 0 {
		return "", fmt.Errorf("no photo data provided")
	}
	return p.analyzeImagesVision(ctx, imgs, locales.PromptForBioscan(contextInfo), true, 8000)
}

// GenerateBodyScanJSON - генерирует JSON премиального отчёта Bioscan PRO.
func (p *DeepSeekProvider) GenerateBodyScanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	imgs := make([]visionImage, 0, len(photosData))
	for _, d := range photosData {
		if len(d) > 0 {
			imgs = append(imgs, visionImage{data: d, mimeType: mimeType})
		}
	}
	if len(imgs) == 0 {
		return "", fmt.Errorf("no photo data provided")
	}
	return p.analyzeImagesVision(ctx, imgs, locales.PromptForBodyScanJSON(contextInfo), true, 8000)
}

// GenerateAnalysisFromFileJSON - анализирует изображение и возвращает JSON.
func (p *DeepSeekProvider) GenerateAnalysisFromFileJSON(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	if !isImageMime(mimeType) {
		return "", fmt.Errorf("deepseek supports only image files for analysis")
	}
	return p.analyzeImagesVision(ctx, []visionImage{{data: data, mimeType: mimeType}}, locales.PromptForAnalysisJSON(contextText), true, 8000)
}

// GenerateDossierJSON - генерирует JSON универсального отчёта-досье здоровья.
func (p *DeepSeekProvider) GenerateDossierJSON(ctx context.Context, userInput string) (string, error) {
	stream := p.client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model: "deepseek-chat",
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("Ты - опытный врач-диагност и аналитик здоровья. Верни ответ строго в формате JSON, без markdown и комментариев."),
			openai.UserMessage(userInput),
		},
		Temperature: openai.Float(0.1),
		MaxTokens:   openai.Int(8000),
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
	if strings.TrimSpace(fullText) == "" {
		return "", fmt.Errorf("deepseek returned empty response")
	}
	return fullText, nil
}

// analyzeImagesVision - мультимодальный анализ изображений через deepseek-chat.
// prompt - текстовая инструкция; jsonMode включает системную роль «верни только JSON».
func (p *DeepSeekProvider) analyzeImagesVision(ctx context.Context, images []visionImage, prompt string, jsonMode bool, maxTokens int) (string, error) {
	if len(images) == 0 {
		return "", fmt.Errorf("no image data provided")
	}

	contents := []openai.ChatCompletionContentPartUnionParam{
		openai.TextContentPart(prompt),
	}
	for _, img := range images {
		if len(img.data) == 0 {
			continue
		}
		url := "data:" + img.mimeType + ";base64," + base64.StdEncoding.EncodeToString(img.data)
		contents = append(contents, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
			URL: url,
		}))
	}

	system := "Ты - опытный врач-диагност. Проанализируй приложенные медицинские изображения и дай развёрнутый анализ с рекомендациями."
	if jsonMode {
		system = "Ты - опытный врач-диагност. Верни ответ строго в формате JSON, без markdown и комментариев."
	}

	stream := p.client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model: "deepseek-chat",
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(system),
			openai.UserMessage(contents),
		},
		Temperature: openai.Float(0.2),
		MaxTokens:   openai.Int(int64(maxTokens)),
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

	if strings.TrimSpace(fullText) == "" {
		return "", fmt.Errorf("deepseek returned empty response")
	}

	return fullText, nil
}
