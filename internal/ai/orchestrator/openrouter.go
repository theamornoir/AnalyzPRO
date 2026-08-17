package orchestrator

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/theamornoir/analyzpro/internal/ai/httpclient"
	"github.com/theamornoir/analyzpro/internal/locales"
)

const (
	// openRouterBaseURL — OpenAI-совместимый endpoint OpenRouter.
	openRouterBaseURL = "https://openrouter.ai/api/v1"

	// defaultOpenRouterModel — бесплатная vision-модель по умолчанию.
	// Обрабатывает и текст, и фото (vision). PDF обрабатывается через
	// извлечение текста (ledongthuc/pdf) и передаётся как обычный текст.
	defaultOpenRouterModel = "google/gemma-4-26b-a4b-it:free"

	// openRouterModelTimeout — макс. время ожидания ответа ОДНОЙ модели.
	// При превышении (free-pool «висит») уходим на следующую модель цепочки,
	// не расходуя общий бюджет и не валя весь провайдер OpenRouter.
	openRouterModelTimeout = 40 * time.Second
)

// openRouterFallbackModels — цепочка бесплатных моделей OpenRouter, к которым
// уходит провайдер, если сконфигурированная (или первая из списка) модель
// недоступна / исчерпала бесплатный лимит (частый кейс для :free-моделей).
var openRouterFallbackModels = []string{
	"nvidia/nemotron-nano-12b-v2-vl:free",
	"google/gemma-4-31b-it:free",
	"nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free",
	"openrouter/free",
}

// OpenRouterProvider — провайдер через OpenRouter (OpenAI-совместимый API).
// Бесплатные модели OpenRouter не имеют гео-блоков (в отличие от Gemini во
// Франции), поэтому это рабочий основной путь для text/photo/PDF-анализа.
type OpenRouterProvider struct {
	client          openai.Client
	configuredModel string
}

// NewOpenRouterProvider создаёт OpenRouterProvider с ключом из окружения.
// Возвращает nil если ключ пустой. Модель берётся из OPENROUTER_MODEL,
// иначе defaultOpenRouterModel.
func NewOpenRouterProvider() *OpenRouterProvider {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return nil
	}
	model := os.Getenv("OPENROUTER_MODEL")
	if model == "" {
		model = defaultOpenRouterModel
	}
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(openRouterBaseURL),
		// Используем общий прокси-совместимый HTTP-клиент (GEMINI_PROXY /
		// системный HTTP_PROXY / direct), как и остальные AI-провайдеры.
		option.WithHTTPClient(httpclient.AIHTTPClient),
		// Рекомендованные OpenRouter заголовки (необязательны, но убирают warnings).
		option.WithHeader("HTTP-Referer", "https://prisma.app"),
		option.WithHeader("X-Title", "Prisma"),
	)
	log.Printf(locales.LogOpenRouterModel, model)
	return &OpenRouterProvider{client: client, configuredModel: model}
}

// candidateModels — сконфигурированная модель + цепочка бесплатных фоллбэков
// (без дублей), в порядке приоритета.
func (p *OpenRouterProvider) candidateModels() []string {
	seen := map[string]bool{}
	var out []string
	if p.configuredModel != "" && !seen[p.configuredModel] {
		out = append(out, p.configuredModel)
		seen[p.configuredModel] = true
	}
	for _, m := range openRouterFallbackModels {
		if !seen[m] {
			out = append(out, m)
			seen[m] = true
		}
	}
	return out
}

// GenerateAnalysisSummary — генерирует текстовый анализ.
func (p *OpenRouterProvider) GenerateAnalysisSummary(ctx context.Context, userInput string) (string, error) {
	return p.complete(ctx,
		"Ты — медицинский аналитик. Проанализируй данные и дай рекомендации.",
		userInput, nil, false, 3000)
}

// GenerateAnalysisJSON — генерирует JSON-анализ.
func (p *OpenRouterProvider) GenerateAnalysisJSON(ctx context.Context, userInput string) (string, error) {
	return p.complete(ctx,
		"Ты — медицинский аналитик. Верни ответ в формате JSON.",
		userInput, nil, true, 4000)
}

// GenerateAnalysisFromFileWithContext — анализирует файл (изображение или PDF) с контекстом.
func (p *OpenRouterProvider) GenerateAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	if isImageMime(mimeType) {
		return p.complete(ctx,
			"Ты — опытный врач-диагност. Проанализируй приложенные медицинские изображения и дай развёрнутый анализ с рекомендациями.",
			contextText, []visionImage{{data: data, mimeType: mimeType}}, false, 4000)
	}
	if mimeType == "application/pdf" {
		text, err := extractPDFText(data)
		if err != nil {
			return "", fmt.Errorf("openrouter: не удалось извлечь текст из PDF: %w", err)
		}
		if strings.TrimSpace(text) == "" {
			return "", fmt.Errorf("openrouter: PDF не содержит извлекаемого текста (возможно, это сканированное изображение)")
		}
		return p.complete(ctx,
			"Ты — опытный врач-диагност. Проанализируй медицинский документ и дай развёрнутый анализ с рекомендациями.",
			contextText+"\n\nТекст документа (PDF):\n"+text, nil, false, 4000)
	}
	return "", fmt.Errorf("openrouter supports only image or pdf files for analysis")
}

// GenerateBioscanJSON — анализирует фото для bioscan и возвращает JSON.
func (p *OpenRouterProvider) GenerateBioscanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	imgs := make([]visionImage, 0, len(photosData))
	for _, d := range photosData {
		if len(d) > 0 {
			imgs = append(imgs, visionImage{data: d, mimeType: mimeType})
		}
	}
	if len(imgs) == 0 {
		return "", fmt.Errorf("no photo data provided")
	}
	return p.complete(ctx,
		"Ты — опытный врач-диагност. Верни ответ строго в формате JSON, без markdown и комментариев.",
		locales.PromptForBioscan(contextInfo), imgs, true, 8000)
}

// GenerateAnalysisFromFileJSON — анализирует файл (изображение или PDF) и возвращает JSON.
func (p *OpenRouterProvider) GenerateAnalysisFromFileJSON(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	if isImageMime(mimeType) {
		return p.complete(ctx,
			"Ты — опытный врач-диагност. Верни ответ строго в формате JSON, без markdown и комментариев.",
			locales.PromptForAnalysisJSON(contextText), []visionImage{{data: data, mimeType: mimeType}}, true, 8000)
	}
	if mimeType == "application/pdf" {
		text, err := extractPDFText(data)
		if err != nil {
			return "", fmt.Errorf("openrouter: не удалось извлечь текст из PDF: %w", err)
		}
		if strings.TrimSpace(text) == "" {
			return "", fmt.Errorf("openrouter: PDF не содержит извлекаемого текста (возможно, это сканированное изображение)")
		}
		return p.complete(ctx,
			"Ты — опытный врач-диагност. Верни ответ строго в формате JSON, без markdown и комментариев.",
			locales.PromptForAnalysisJSON(contextText)+"\n\nТекст документа (PDF):\n"+text, nil, true, 8000)
	}
	return "", fmt.Errorf("openrouter supports only image or pdf files for analysis")
}

// complete — универсальный вызов chat-completions с перебором моделей по
// цепочке фоллбэков. userText — текстовая инструкция/контекст, images — опциональные
// изображения для vision. jsonMode добавляет требование «только JSON» в системный промпт.
// GenerateDossierJSON generates the JSON of the universal health-dossier report.
func (p *OpenRouterProvider) GenerateDossierJSON(ctx context.Context, userInput string) (string, error) {
	return p.complete(ctx,
		"\u0422\u044b - \u043e\u043f\u044b\u0442\u043d\u044b\u0439 \u0432\u0440\u0430\u0447-\u0434\u0438\u0430\u0433\u043d\u043e\u0441\u0442. \u0412\u0435\u0440\u043d\u0438 \u043e\u0442\u0432\u0435\u0442 \u0441\u0442\u0440\u043e\u0433\u043e \u0432 \u0444\u043e\u0440\u043c\u0430\u0442\u0435 JSON, \u0431\u0435\u0437 markdown.",
		locales.PromptForDossierJSON(userInput), nil, true, 8000)
}

func (p *OpenRouterProvider) complete(ctx context.Context, systemPrompt, userText string, images []visionImage, jsonMode bool, maxTokens int) (string, error) {
	if jsonMode {
		systemPrompt += "\nВерни ответ строго в формате JSON, без markdown-разметки и комментариев."
	}

	models := p.candidateModels()
	var lastErr error

	for _, model := range models {
		log.Printf(locales.LogOpenRouterTrying, model)

		params := openai.ChatCompletionNewParams{
			Model:       model,
			Temperature: openai.Float(0.2),
			MaxTokens:   openai.Int(int64(maxTokens)),
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.SystemMessage(systemPrompt),
			},
		}

		if len(images) > 0 {
			parts := []openai.ChatCompletionContentPartUnionParam{
				openai.TextContentPart(userText),
			}
			for _, img := range images {
				if len(img.data) == 0 {
					continue
				}
				url := "data:" + img.mimeType + ";base64," + base64.StdEncoding.EncodeToString(img.data)
				parts = append(parts, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
					URL: url,
				}))
			}
			params.Messages = append(params.Messages, openai.UserMessage(parts))
		} else {
			params.Messages = append(params.Messages, openai.UserMessage(userText))
		}

		modelCtx, modelCancel := context.WithTimeout(ctx, openRouterModelTimeout)
		resp, err := p.client.Chat.Completions.New(modelCtx, params)
		modelCancel()
		if err != nil {
			lastErr = err
			// Модель недоступна / лимит исчерпан → пробуем следующую из цепочки.
			if isOpenRouterFallbackError(err) {
				log.Printf(locales.LogOpenRouterModelFailed, model, err)
				continue
			}
			return "", fmt.Errorf("openrouter: модель %s — %w", model, err)
		}

		if len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
			lastErr = fmt.Errorf("openrouter: модель %s вернула пустой ответ", model)
			log.Printf(locales.LogOpenRouterModelFailed, model, lastErr)
			continue
		}

		return resp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("openrouter: все модели недоступны: %w", lastErr)
}

// isOpenRouterFallbackError сообщает, стоит ли перейти к следующей модели
// (модель не найдена / исчерпан бесплатный лимит / ошибка провайдера).
// Для сетевых/прочих ошибок возвращает false (оркестратор перейдёт к другому провайдеру).
func isOpenRouterFallbackError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *openai.Error
	// Таймаут / отмена контекста (в т.ч. Client.Timeout транспорта при
	// «висении» free-модели) — пробуем следующую модель цепочки, вместо
	// того чтобы валить весь провайдер OpenRouter.
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}

	if errors.As(err, &apiErr) {
		msg := strings.ToLower(apiErr.Message)
		switch apiErr.StatusCode {
		case 404, 429:
			return true
		case 400, 401, 403:
			return strings.Contains(msg, "model") ||
				strings.Contains(msg, "not found") ||
				strings.Contains(msg, "rate") ||
				strings.Contains(msg, "credit") ||
				strings.Contains(msg, "limit") ||
				strings.Contains(msg, "provider")
		case 500, 502, 503:
			return true
		}
	}
	return false
}

// extractPDFText извлекает весь текст из PDF через чистый Go-парсер
// (ledongthuc/pdf). Используется, т.к. OpenAI-совместимые API (в т.ч. OpenRouter)
// не принимают PDF-файлы напрямую — только текст/картинки. Для сканированных
// PDF (только картинки) вернёт пустоту — это обрабатывается вызывающей стороной.
func extractPDFText(data []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	textReader, err := reader.GetPlainText()
	if err != nil {
		return "", err
	}
	text, err := io.ReadAll(textReader)
	if err != nil {
		return "", err
	}
	return string(text), nil
}
