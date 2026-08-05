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

	// Проверяем, есть ли информация о курсе (для спортсмена)
	var prompt string
	if strings.Contains(userInput, "❗ ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:") &&
		strings.Contains(userInput, "ИСПОЛЬЗУЕТ ПРЕПАРАТЫ") {
		courseInfo := extractCourseInfo(userInput)
		if courseInfo != "" {
			prompt = c.buildPromptForAthlete(userInput, courseInfo)
			log.Printf("🏋️ Athlete mode: on course")
		} else {
			prompt = c.buildPromptForRegular(userInput)
			log.Printf("👤 Regular mode: no course info")
		}
	} else if strings.Contains(userInput, "❗ ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:") {
		// Проверяем, есть ли информация о том, что это спортсмен
		if strings.Contains(userInput, "Вид спорта") || strings.Contains(userInput, "Стаж тренировок") {
			courseInfo := extractCourseInfo(userInput)
			if courseInfo != "" {
				prompt = c.buildPromptForAthlete(userInput, courseInfo)
				log.Printf("🏋️ Athlete mode: sportsman detected")
			} else {
				prompt = c.buildPromptForRegular(userInput)
				log.Printf("👤 Regular mode")
			}
		} else {
			prompt = c.buildPromptForRegular(userInput)
			log.Printf("👤 Regular mode")
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

// GenerateAnalysisFromFileWithContext - анализирует файл с дополнительным контекстом
func (c *GeminiClient) GenerateAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return noKeyFallback(), nil
	}

	if len(data) == 0 {
		return "", fmt.Errorf("empty file data")
	}

	// Проверяем, есть ли информация о спортсмене и препаратах
	var prompt string
	if strings.Contains(contextText, "❗ ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:") {
		if strings.Contains(contextText, "ИСПОЛЬЗУЕТ ПРЕПАРАТЫ") ||
			strings.Contains(contextText, "Вид спорта") ||
			strings.Contains(contextText, "Стаж тренировок") {
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
			MaxOutputTokens: 3000,
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

		if resp.StatusCode == 400 {
			// Проверяем, не связана ли ошибка с геолокацией
			var errResp struct {
				Error struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
					Status  string `json:"status"`
				} `json:"error"`
			}
			if err := json.Unmarshal(respBody, &errResp); err == nil {
				if strings.Contains(errResp.Error.Message, "location is not supported") {
					return locationErrorFallback(), nil
				}
			}
			return serviceUnavailableFallback(), nil
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

// buildPromptForRegular - промпт для обычного пациента
func (c *GeminiClient) buildPromptForRegular(text string) string {
	return fmt.Sprintf(`Ты опытный врач-диагност. Отвечай строго по шаблону.

📌 ПРАВИЛА ФОРМАТИРОВАНИЯ:
1. Используй разделители для каждой категории
2. Каждый показатель на новой строке
3. Четкое выделение отклонений
4. Профессиональным медицинским языком

📋 ШАБЛОН ОТВЕТА:

📊 Результаты анализов
👤 Пациент: [имя]
📅 Дата: [текущая дата]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🩸 Общий анализ крови

[показатель 1] → [значение] (норма: [мин-макс])
ℹ️ [краткое описание]
Статус: ✅ В норме / ⚠️ Требует внимания / ❌ Отклонение

[показатель 2] → [значение] (норма: [мин-макс])
ℹ️ [краткое описание]
Статус: ✅ В норме / ⚠️ Требует внимания / ❌ Отклонение

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🧬 Биохимический анализ

[аналогичный формат]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💉 Гормоны

[аналогичный формат]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📋 Общее заключение

[1-2 предложения итога]

⚠️ На что обратить внимание:
• [пункт 1]
• [пункт 2]

✅ Рекомендации:
1. [рекомендация 1]
2. [рекомендация 2]
3. [рекомендация 3]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

ℹ️ Важно: Этот анализ носит информационный характер и не является диагнозом. Для постановки диагноза обратитесь к врачу.

📌 Инструкция:
- Если показатели в норме → ставь ✅
- Если есть отклонения → ставь ⚠️ или ❌
- Для каждого показателя давай краткое описание
- Если данных нет → пиши "Данные не предоставлены"

Данные анализов пользователя:
%s`, text)
}

// buildPromptForAthlete - промпт для спортсмена
func (c *GeminiClient) buildPromptForAthlete(text string, courseInfo string) string {
	return fmt.Sprintf(`Ты опытный спортивный врач и фармаколог. Отвечай строго по шаблону.

📌 ПРАВИЛА ФОРМАТИРОВАНИЯ:
1. Используй разделители для каждой категории
2. Каждый показатель на новой строке
3. Учитывай влияние препаратов
4. Профессиональный, но понятный язык

📋 ШАБЛОН ОТВЕТА:

📊 Спортивный анализ
👤 Спортсмен: [имя]
🎯 Цель: [цель]
💪 Стаж: [стаж]
🏋️ Вид спорта: [вид спорта]
💊 Препараты: [курс]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🩸 Общий анализ крови

[показатель 1] → [значение] (норма: [мин-макс])
ℹ️ [краткое описание]
📈 Влияние на спорт: [описание]
Статус: ✅ В норме / ⚠️ Требует внимания / ❌ Отклонение

[показатель 2] → [значение] (норма: [мин-макс])
ℹ️ [краткое описание]
📈 Влияние на спорт: [описание]
Статус: ✅ В норме / ⚠️ Требует внимания / ❌ Отклонение

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🧬 Биохимический анализ

[аналогичный формат с учетом препаратов]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

💉 Гормоны (с учетом препаратов)

[аналогичный формат с учетом препаратов]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🏋️ Рекомендации по курсу

1. [рекомендация по препарату 1]
2. [рекомендация по препарату 2]
3. [рекомендация по поддержке]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📋 Общее заключение

[1-2 предложения итога с учетом курса]

⚠️ На что обратить внимание:
• [пункт 1]
• [пункт 2]

✅ Рекомендации:
1. [рекомендация 1]
2. [рекомендация 2]
3. [рекомендация 3]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

ℹ️ Важно: Анализ выполнен с учетом приема препаратов. Не является диагнозом. Для коррекции курса проконсультируйтесь с врачом.

📌 Инструкция:
- Если показатели в норме → ставь ✅
- Если есть отклонения → ставь ⚠️ или ❌
- Учитывай влияние препаратов на показатели
- 📈 - положительное влияние на спорт
- 📉 - негативное влияние на спорт
- Если данных нет → пиши "Данные не предоставлены"

Информация о пациенте и препаратах: %s

Данные анализов пользователя:
%s`, courseInfo, text)
}

// extractCourseInfo - извлекает информацию о курсе из текста
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

// rateLimitFallback - сообщение для ошибки 429 (превышение квоты)
func rateLimitFallback() string {
	log.Printf("⚠️ Returning rate-limit fallback response")
	return `⏳ Сервис временно перегружен

📊 В данный момент я не могу обработать ваш запрос, так как достигнут лимит запросов к сервису искусственного интеллекта.

🔄 Что делать:
• Подождите 1-2 минуты
• Отправьте анализ повторно

💡 Бесплатный тариф имеет ограничение на количество запросов в минуту.

⏰ Обычно лимит восстанавливается через 1-2 минуты. Попробуйте снова через несколько минут.`
}

// locationErrorFallback - сообщение для ошибки геолокации
func locationErrorFallback() string {
	log.Printf("⚠️ Returning location error fallback response")
	return `🌍 Сервис временно недоступен в вашем регионе

Google Gemini API может быть недоступен в некоторых странах.

🔄 Что делать:
• Используйте VPN для доступа к сервису
• Попробуйте позже
• Обратитесь к администратору бота

⏰ В ближайшее время мы работаем над решением этой проблемы.`
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

⏰ Если проблема сохраняется, попробуйте позже.`
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
	// Убираем возможные лишние пробелы
	text = strings.TrimSpace(text)

	// Проверяем наличие основных разделов
	sections := []string{"📊", "🩸", "🧬", "💉", "📋", "ℹ️"}
	hasAllSections := true
	for _, section := range sections {
		if !strings.Contains(text, section) {
			hasAllSections = false
			break
		}
	}

	// Если нет разделов - пытаемся структурировать
	if !hasAllSections {
		// Проверяем, есть ли хоть какой-то контент
		if len(text) < 50 {
			return text + "\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\nℹ️ Важно: Этот ответ не является диагнозом и не заменяет очную консультацию врача."
		}

		// Добавляем недостающие разделы
		if !strings.Contains(text, "📋 Общее заключение") {
			text += "\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n📋 Общее заключение\n\nДанные проанализированы. Для получения детальной интерпретации обратитесь к врачу."
		}
		if !strings.Contains(text, "ℹ️") {
			text += "\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\nℹ️ Важно: Этот ответ не является диагнозом и не заменяет очную консультацию врача."
		}
	}

	// Убираем повторяющиеся пустые строки
	lines := strings.Split(text, "\n")
	var result []string
	var lastEmpty bool
	for _, line := range lines {
		isEmpty := strings.TrimSpace(line) == ""
		if isEmpty && lastEmpty {
			continue
		}
		result = append(result, line)
		lastEmpty = isEmpty
	}

	return strings.Join(result, "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GenerateBioscanAnalysis - анализирует фото тела
func (c *GeminiClient) GenerateBioscanAnalysis(ctx context.Context, data []byte, mimeType string, contextInfo string) (string, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return noKeyFallback(), nil
	}

	if len(data) == 0 {
		return "", fmt.Errorf("empty file data")
	}

	// Проверяем размер фото - для Bioscan нужно качественное фото
	if len(data) < 50000 {
		return "📸 **Фото слишком маленькое**\n\n" +
			"Пожалуйста, отправьте **качественное фото** в полный рост.\n\n" +
			"📌 Рекомендации:\n" +
			"• Фото в полный рост (анфас)\n" +
			"• Хорошее освещение\n" +
			"• В обтягивающей одежде или без неё\n" +
			"• Стоять прямо, руки вдоль тела\n\n" +
			"🔄 Отправьте новое фото для анализа.", nil
	}

	prompt := c.buildBioscanPrompt(contextInfo)

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

// buildBioscanPrompt - промпт для Bioscan
func (c *GeminiClient) buildBioscanPrompt(contextInfo string) string {
	return fmt.Sprintf(`Ты профессиональный фитнес-тренер, специалист по коррекции фигуры и эксперт по биомеханике тела. Проанализируй фото тела и дай максимально детальную оценку.

📌 **ИНФОРМАЦИЯ О ПОЛЬЗОВАТЕЛЕ:**
%s

📋 **ФОРМАТ ОТВЕТА (строго соблюдай структуру):**

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📸 **BIOSCAN - ДЕТАЛЬНЫЙ АНАЛИЗ ФИГУРЫ**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🧍 **ОБЩАЯ ОЦЕНКА ТЕЛОСЛОЖЕНИЯ**

📊 **Тип фигуры:** [эктоморф/мезоморф/эндоморф/смешанный]
📏 **Пропорции:** [оценка пропорций тела]
🎯 **Общая оценка:** [краткая оценка]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
💪 **ДЕТАЛЬНЫЙ АНАЛИЗ МЫШЦ**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

**🔹 ВЕРХНЯЯ ЧАСТЬ ТЕЛА**

┌─────────────────────────────────┐
│          ГРУДНЫЕ МЫШЦЫ           │
├─────────────────────────────────┤
│ Состояние: [развито/слабо/средне]│
│ Симметрия: [симметрично/есть     │
│            асимметрия]           │
│ Оценка: [краткая оценка]         │
│ Рекомендация: [упражнение]       │
└─────────────────────────────────┘

┌─────────────────────────────────┐
│          ДЕЛЬТОВИДНЫЕ МЫШЦЫ      │
├─────────────────────────────────┤
│ Передний пучок: [оценка]         │
│ Средний пучок: [оценка]          │
│ Задний пучок: [оценка]           │
│ Рекомендация: [упражнение]       │
└─────────────────────────────────┘

┌─────────────────────────────────┐
│            ТРАПЕЦИЯ              │
├─────────────────────────────────┤
│ Верхняя часть: [оценка]          │
│ Средняя часть: [оценка]          │
│ Нижняя часть: [оценка]           │
│ Рекомендация: [упражнение]       │
└─────────────────────────────────┘

┌─────────────────────────────────┐
│            ШИРОЧАЙШАЯ            │
├─────────────────────────────────┤
│ Состояние: [развито/слабо/средне]│
│ Симметрия: [симметрично/есть     │
│            асимметрия]           │
│ Рекомендация: [упражнение]       │
└─────────────────────────────────┘

┌─────────────────────────────────┐
│         БИЦЕПС И ТРИЦЕПС         │
├─────────────────────────────────┤
│ Бицепс: [оценка]                 │
│ Трицепс: [оценка]                │
│ Соотношение: [бицепс/трицепс]    │
│ Рекомендация: [упражнение]       │
└─────────────────────────────────┘

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🏋️ **СРЕДНЯЯ ЧАСТЬ ТЕЛА**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

┌─────────────────────────────────┐
│         ПРЯМАЯ МЫШЦА ЖИВОТА      │
├─────────────────────────────────┤
│ Состояние: [развито/слабо/средне]│
│ Сегментация: [выражена/слабая]   │
│ Рекомендация: [упражнение]       │
└─────────────────────────────────┘

┌─────────────────────────────────┐
│       КОСЫЕ МЫШЦЫ ЖИВОТА         │
├─────────────────────────────────┤
│ Состояние: [оценка]              │
│ Симметрия: [симметрично/есть     │
│            асимметрия]           │
│ Рекомендация: [упражнение]       │
└─────────────────────────────────┘

┌─────────────────────────────────┐
│          ПОЯСНИЧНЫЙ ОТДЕЛ        │
├─────────────────────────────────┤
│ Состояние: [оценка]              │
│ Рекомендация: [упражнение]       │
└─────────────────────────────────┘

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🦵 **НИЖНЯЯ ЧАСТЬ ТЕЛА**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

┌─────────────────────────────────┐
│           КВАДРИЦЕПС             │
├─────────────────────────────────┤
│ Состояние: [развито/слабо/средне]│
│ Симметрия: [симметрично/есть     │
│            асимметрия]           │
│ Рекомендация: [упражнение]       │
└─────────────────────────────────┘

┌─────────────────────────────────┐
│          БИЦЕПС БЕДРА            │
├─────────────────────────────────┤
│ Состояние: [оценка]              │
│ Рекомендация: [упражнение]       │
└─────────────────────────────────┘

┌─────────────────────────────────┐
│         ИКРОНОЖНЫЕ МЫШЦЫ         │
├─────────────────────────────────┤
│ Состояние: [оценка]              │
│ Рекомендация: [упражнение]       │
└─────────────────────────────────┘

┌─────────────────────────────────┐
│            ЯГОДИЦЫ               │
├─────────────────────────────────┤
│ Состояние: [развито/слабо/средне]│
│ Симметрия: [симметрично/есть     │
│            асимметрия]           │
│ Рекомендация: [упражнение]       │
└─────────────────────────────────┘

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🦴 **ОСАНКА И ПОЛОЖЕНИЕ ТЕЛА**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

┌─────────────────────────────────┐
│          ОБЩАЯ ОЦЕНКА            │
├─────────────────────────────────┤
│ Тип осанки: [нормальная/сутулая/ │
│             гиперлордоз/        │
│             кифоз]              │
│ Положение головы: [вперед/       │
│                    нейтрально]  │
│ Положение плеч: [вперед/назад/   │
│                  нейтрально]    │
│ Положение таза: [антеверсия/     │
│                  ретроверсия/   │
│                  нейтрально]    │
└─────────────────────────────────┘

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
⚠️ **ЗОНЫ, ТРЕБУЮЩИЕ ВНИМАНИЯ**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

1. **Зона:** [название] — [проблема]
   🔧 Решение: [конкретная рекомендация]

2. **Зона:** [название] — [проблема]
   🔧 Решение: [конкретная рекомендация]

3. **Зона:** [название] — [проблема]
   🔧 Решение: [конкретная рекомендация]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ **ПЕРСОНАЛЬНЫЕ РЕКОМЕНДАЦИИ**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📋 **Приоритетные задачи (по важности):**

1. **[Задача 1]** — [описание]
2. **[Задача 2]** — [описание]
3. **[Задача 3]** — [описание]

🏋️ **Программа тренировок:**

**День 1 (Верх тела):**
• [упражнение 1] — [подходы/повторения]
• [упражнение 2] — [подходы/повторения]
• [упражнение 3] — [подходы/повторения]

**День 2 (Низ тела):**
• [упражнение 1] — [подходы/повторения]
• [упражнение 2] — [подходы/повторения]
• [упражнение 3] — [подходы/повторения]

**День 3 (Коррекция):**
• [упражнение 1] — [подходы/повторения]
• [упражнение 2] — [подходы/повторения]
• [упражнение 3] — [подходы/повторения]

🥗 **Рекомендации по питанию:**
• [рекомендация 1]
• [рекомендация 2]
• [рекомендация 3]

🧘 **Восстановление:**
• [рекомендация 1]
• [рекомендация 2]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 **ПРОГРЕСС-ТРЕК**
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📅 **Рекомендуемый повторный анализ:** через 4-6 недель

📌 **Целевые показатели на следующий анализ:**
1. [показатель 1]
2. [показатель 2]
3. [показатель 3]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

ℹ️ **Важно:** Анализ носит информационный характер и не заменяет консультацию профессионального тренера.

**Используй эмодзи и символы:**
• ✅ — хорошо
• ⚠️ — требует внимания
• ❌ — требует коррекции
• 📈 — прогресс
• 🔧 — решение
• 🏋️ — тренировки
• 🥗 — питание
• 🧘 — восстановление
• 📊 — статистика
• 📸 — анализ

**Оценивай каждую мышечную группу по шкале:**
• Отлично развита
• Хорошо развита
• Средне развита
• Слабо развита
• Требует внимания

Всегда давай конкретные рекомендации с названиями упражнений, количеством подходов и повторений.`, contextInfo)
}
