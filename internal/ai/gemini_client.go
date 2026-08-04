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

	// Проверяем, есть ли информация о курсе
	var prompt string
	if strings.Contains(userInput, "❗ ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:") {
		courseInfo := extractCourseInfo(userInput)
		if courseInfo != "" {
			prompt = c.buildPromptForAthlete(userInput, courseInfo)
			log.Printf("🏋️ Athlete mode: on course")
		} else {
			prompt = c.buildPromptForRegular(userInput)
			log.Printf("👤 Regular mode: no course info")
		}
	} else {
		prompt = c.buildPromptForRegular(userInput)
		log.Printf("👤 Regular mode")
	}

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
		{Text: c.buildPromptForRegular("Содержимое загруженного документа с медицинскими анализами во вложении.")},
	}

	return c.generate(ctx, parts)
}

// GenerateAnalysisFromFileWithContext - анализирует файл с учетом контекста
func (c *GeminiClient) GenerateAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return noKeyFallback(), nil
	}

	if len(data) == 0 {
		return "", fmt.Errorf("empty file data")
	}

	// Проверяем, есть ли информация о курсе
	var prompt string
	if strings.Contains(contextText, "❗ ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:") {
		courseInfo := extractCourseInfo(contextText)
		if courseInfo != "" {
			prompt = c.buildPromptForAthlete(contextText, courseInfo)
			log.Printf("🏋️ Athlete mode: on course")
		} else {
			prompt = c.buildPromptForRegular(contextText)
			log.Printf("👤 Regular mode")
		}
	} else {
		prompt = c.buildPromptForRegular(contextText)
		log.Printf("👤 Regular mode")
	}

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

		if resp.StatusCode == 429 {
			return rateLimitFallback(), nil
		}

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

		if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 500 {
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

// buildPromptForAthlete - промпт для спортсмена на курсе
func (c *GeminiClient) buildPromptForAthlete(text string, courseInfo string) string {
	return fmt.Sprintf(`Ты опытный спортивный врач и фармаколог, специализирующийся на работе со спортсменами, которые находятся на курсах спортивной фармакологии.

ВАЖНО: Пользователь находится на курсе: %s

Твоя задача - проанализировать медицинские показатели с учетом того, что человек на курсе.

Правила анализа:
1. Учитывай влияние препаратов на показатели
2. Оценивай риски и побочные эффекты
3. Давай рекомендации по коррекции курса
4. Обращай внимание на показатели, критичные для спортсменов

Формат ответа (строго следуй этому шаблону):

📌 Краткий вывод
[1-2 предложения о состоянии с учетом курса]

🩺 Ключевые показатели (с учетом курса)
- [показатель 1] - [интерпретация с учетом курса]
- [показатель 2] - [интерпретация с учетом курса]
- [показатель 3] - [интерпретация с учетом курса]

⚠️ Критические отклонения (для спортсмена на курсе)
- [показатель 1] - [опасность и рекомендация]
- [показатель 2] - [опасность и рекомендация]

💊 Рекомендации по коррекции курса
1. [рекомендация по дозировке/препарату]
2. [рекомендация по поддержке/восстановлению]
3. [рекомендация по дополнительным анализам]

🏋️ Спортивные рекомендации
1. [рекомендация по тренировкам]
2. [рекомендация по питанию]
3. [рекомендация по восстановлению]

ℹ️ Важно
Этот ответ не является медицинским диагнозом. Для коррекции курса обязательно проконсультируйтесь с вашим лечащим врачом или спортивным доктором.

Данные анализов пользователя:
%s`, courseInfo, text)
}

// buildPromptForRegular - промпт для обычного человека (стандартный)
func (c *GeminiClient) buildPromptForRegular(text string) string {
	return fmt.Sprintf(`Ты опытный врач-диагност. Отвечай только по фиксированному шаблону, без текста вне формата.

Вот медицинский анализ пользователя:
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
}

// extractCourseInfo - извлекает информацию о курсе из текста
func extractCourseInfo(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.Contains(line, "❗ ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:") {
			var courseInfo strings.Builder
			for j := i + 1; j < len(lines) && j < i+6; j++ {
				if strings.TrimSpace(lines[j]) == "" {
					break
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

func rateLimitFallback() string {
	log.Printf("⚠️ Returning rate-limit fallback response")
	return `⏳ Сервис временно перегружен

📊 В данный момент я не могу обработать ваш запрос, так как достигнут лимит запросов к сервису искусственного интеллекта.

🔄 Что делать:
• Подождите 1-2 минуты
• Отправьте анализ повторно

💡 Бесплатный тариф имеет ограничение на количество запросов в минуту. Это сделано для того, чтобы все пользователи могли пользоваться сервисом.

⏰ Обычно лимит восстанавливается через 1-2 минуты. Попробуйте снова через несколько минут.

ℹ️ Важно
Если проблема повторяется, вы можете написать разработчику бота для увеличения лимитов.`
}

func noKeyFallback() string {
	log.Printf("⚠️ Returning no-key fallback response")
	return `❌ AI не настроен

🔑 В текущем окружении не настроен ключ для доступа к искусственному интеллекту.

🛠️ Что делать:
• Обратитесь к администратору бота
• Убедитесь, что в настройках указан GOOGLE_GEMINI_API_KEY

⏳ Как только ключ будет добавлен, бот снова сможет обрабатывать анализы.`
}

func serviceUnavailableFallback() string {
	log.Printf("⚠️ Returning service-unavailable fallback response")
	return `🔧 Сервис временно недоступен

🌐 В данный момент сервис искусственного интеллекта не отвечает.

🔄 Что делать:
• Проверьте интернет-соединение
• Подождите несколько минут
• Попробуйте отправить анализ повторно

⏰ Если проблема сохраняется, возможно проводятся технические работы. Попробуйте позже.`
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

func normalizeAIResponse(text string) string {
	text = strings.TrimSpace(text)

	sections := []string{"📌 Краткий вывод", "🩺 Что важно", "⚠️ Показатели", "✅ Рекомендации", "ℹ️ Важно"}
	hasAllSections := true
	for _, section := range sections {
		if !strings.Contains(text, section) {
			hasAllSections = false
			break
		}
	}

	if !hasAllSections {
		if !strings.Contains(text, "ℹ️ Важно") {
			text += "\n\nℹ️ Важно\nЭтот ответ не является диагнозом и не заменяет очную консультацию врача."
		}
	}

	return text
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
