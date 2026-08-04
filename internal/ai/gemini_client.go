package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type GeminiClient struct {
	model  string
	apiKey string
	client *http.Client
}

func NewGeminiClient(apiKey, model string) *GeminiClient {
	if model == "" {
		model = "gemini-3.6-flash"
	}

	// Убираем "models/" если оно есть
	model = strings.TrimPrefix(model, "models/")

	log.Printf("🔑 Gemini Client initialized with model: %s", model)

	return &GeminiClient{
		model:  model,
		apiKey: apiKey,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *GeminiClient) GenerateAnalysisSummary(ctx context.Context, userInput string) (string, error) {
	log.Printf("📤 GenerateAnalysisSummary called with input length: %d", len(userInput))

	if strings.TrimSpace(c.apiKey) == "" {
		log.Printf("❌ API key is empty or only whitespace")
		return noKeyFallback(), nil
	}

	prompt := c.buildPrompt(userInput)
	log.Printf("📝 Built prompt length: %d characters", len(prompt))

	return c.generate(ctx, []geminiPart{{Text: prompt}})
}

func (c *GeminiClient) GenerateAnalysisFromFile(ctx context.Context, data []byte, mimeType string) (string, error) {
	log.Printf("📤 GenerateAnalysisFromFile called with mimeType: %s, data size: %d bytes", mimeType, len(data))

	if strings.TrimSpace(c.apiKey) == "" {
		log.Printf("❌ API key is empty or only whitespace")
		return noKeyFallback(), nil
	}

	if len(data) == 0 {
		return "", fmt.Errorf("empty file data")
	}

	parts := []geminiPart{
		{
			InlineData: &geminiInlineData{
				MimeType: mimeType,
				Data:     base64.StdEncoding.EncodeToString(data),
			},
		},
		{Text: c.buildPrompt("Содержимое загруженного документа с медицинскими анализами во вложении.")},
	}

	return c.generate(ctx, parts)
}

func (c *GeminiClient) generate(ctx context.Context, parts []geminiPart) (string, error) {
	log.Printf("🔄 Starting Gemini API request...")

	payload := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{
				Text: "Ты опытный врач-диагност. Отвечай только по фиксированному шаблону, без текста вне формата.",
			}},
		},
		Contents: []geminiContent{{
			Role:  "user",
			Parts: parts,
		}},
		GenerationConfig: &geminiGenerationConfig{
			Temperature:     0.2,
			MaxOutputTokens: 2048,
			TopP:            0.95,
			TopK:            40,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("❌ Failed to marshal payload: %v", err)
		return "", err
	}

	log.Printf("📦 Request body size: %d bytes", len(body))

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		c.model,
		c.apiKey,
	)

	// Скрываем ключ в логах
	if len(c.apiKey) > 10 {
		loggedURL := fmt.Sprintf(
			"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s...%s",
			c.model,
			c.apiKey[:10],
			c.apiKey[len(c.apiKey)-4:],
		)
		log.Printf("🌐 Request URL: %s", loggedURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Printf("❌ Failed to create request: %v", err)
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	log.Printf("⏳ Sending request to Gemini...")
	startTime := time.Now()

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("❌ HTTP request failed: %v", err)
		return serviceUnavailableFallback(), nil
	}
	defer resp.Body.Close()

	elapsed := time.Since(startTime)
	log.Printf("⏱️ Request completed in %v", elapsed)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("❌ Failed to read response body: %v", err)
		return "", err
	}

	log.Printf("📥 Response status: %d", resp.StatusCode)
	if len(respBody) > 500 {
		log.Printf("📄 Response body (first 500 chars): %s...", string(respBody[:500]))
	} else {
		log.Printf("📄 Response body: %s", string(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ Non-OK status: %d", resp.StatusCode)

		var errResp struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}

		if err := json.Unmarshal(respBody, &errResp); err == nil {
			log.Printf("📋 Gemini error details: Code=%d, Message=%s, Status=%s",
				errResp.Error.Code,
				errResp.Error.Message,
				errResp.Error.Status)

			if errResp.Error.Code == 429 || errResp.Error.Code == 401 || errResp.Error.Code == 403 || errResp.Error.Code == 500 {
				return serviceUnavailableFallback(), nil
			}

			return "", fmt.Errorf("gemini error %d: %s", errResp.Error.Code, errResp.Error.Message)
		}

		if resp.StatusCode == 429 || resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 500 {
			return serviceUnavailableFallback(), nil
		}
		return "", fmt.Errorf("gemini request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result geminiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		log.Printf("❌ Failed to unmarshal response: %v", err)
		return "", err
	}

	if result.Error != nil {
		log.Printf("❌ Gemini returned error: Code=%d, Message=%s", result.Error.Code, result.Error.Message)
		if result.Error.Code == 429 || result.Error.Code == 401 || result.Error.Code == 403 || result.Error.Code == 500 {
			return serviceUnavailableFallback(), nil
		}
		return "", fmt.Errorf("gemini error: %s", result.Error.Message)
	}

	text := extractGeminiText(&result)
	log.Printf("📝 Extracted text length: %d characters", len(text))

	if text == "" {
		log.Printf("⚠️ Empty response from Gemini")
		log.Printf("🔍 Full response: %s", string(respBody))
		return BuildDoctorTemplate(
			"Сервис анализа получил пустой ответ.",
			"Проверьте входные данные и конфигурацию AI.",
			"Повторите запрос позже.",
			"После проверки данных результат будет подготовлен повторно.",
			"Не используйте этот ответ как диагноз.",
		), nil
	}

	log.Printf("✅ Successfully generated response from Gemini")
	return normalizeAIResponse(text), nil
}

func (c *GeminiClient) buildPrompt(text string) string {
	prompt := fmt.Sprintf(`Ты опытный врач-диагност и медицинский консультант. Отвечай строго по шаблону, без вариативности структуры. Не пиши общие рассуждения. Сохраняй спокойный и профессиональный тон.

Пользователь прислал информацию по анализам:
%s

Верни ответ строго в таком формате:

📌 Краткий вывод
[1–2 предложения]

🩺 Что важно
- [важный пункт 1]
- [важный пункт 2]
- [важный пункт 3]

⚠️ Показатели, требующие внимания
- [показатель 1]
- [показатель 2]

✅ Рекомендации
1. [рекомендация 1]
2. [рекомендация 2]
3. [рекомендация 3]

ℹ️ Важно
Этот ответ не является диагнозом и не заменяет очную консультацию врача. Он помогает понять направление анализа и возможные отклонения.
`, text)

	log.Printf("📝 Built prompt with user input (first 100 chars): %s...", text[:min(100, len(text))])
	return prompt
}

func noKeyFallback() string {
	log.Printf("⚠️ Returning no-key fallback response")
	return BuildDoctorTemplate(
		"AI не подключён в текущем окружении.",
		"Для полноценной аналитики нужно настроить GOOGLE_GEMINI_API_KEY.",
		"Проверьте конфигурацию сервиса.",
		"Подождите, пока API будет подключено.",
		"Не используйте этот ответ как медицинское заключение.",
	)
}

func serviceUnavailableFallback() string {
	log.Printf("⚠️ Returning service-unavailable fallback response")
	return BuildDoctorTemplate(
		"Сервис ИИ временно недоступен.",
		"Запрос был получен, но обработка сейчас ограничена по квоте или скорости доступа.",
		"На текущий момент стоит повторить запрос чуть позже.",
		"Подождите 1–2 минуты и отправьте анализ повторно.",
		"Этот ответ не является диагнозом и служит только для безопасного уведомления о временной недоступности AI.",
	)
}

type geminiRequest struct {
	SystemInstruction *geminiContent          `json:"systemInstruction,omitempty"`
	Contents          []geminiContent         `json:"contents"`
	GenerationConfig  *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inlineData,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type geminiGenerationConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	TopP            float64 `json:"topP,omitempty"`
	TopK            int     `json:"topK,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func extractGeminiText(resp *geminiResponse) string {
	if resp == nil || len(resp.Candidates) == 0 {
		return ""
	}

	var parts []string
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Text != "" {
			parts = append(parts, part.Text)
		}
	}

	return strings.Join(parts, "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
