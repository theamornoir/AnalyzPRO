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

func (c *GeminiClient) GenerateBioscanJSON(
	ctx context.Context,
	data []byte,
	mimeType string,
	contextInfo string,
) (string, error) {

	return `{
  "score": 87,
  "level": "Высокий уровень физического развития",
  "summary": "Текущая форма характеризуется хорошим уровнем физического развития, гармоничным соотношением основных параметров тела и наличием потенциала для дальнейшего улучшения. Соотношение мышечной и жировой ткани создаёт спортивный внешний вид.",

  "body": {
    "height": "180 см",
    "weight": "78 кг",
    "muscle_mass": "42.5 кг",
    "fat": "13.8%"
  },

  "composition": "Проведённый анализ показывает благоприятное соотношение основных компонентов тела. Количество мышечной ткани находится на хорошем уровне, а процент жировой массы позволяет сохранять спортивный внешний вид и высокую физическую работоспособность. Текущий этап развития является оптимальным для постепенного улучшения качества тела: увеличения мышечного объёма, повышения силовых показателей и сохранения гармоничных пропорций.",

  "profile": {
    "composition": 88,
    "muscle_development": 82,
    "balance": 90,
    "potential": 85
  },

  "zones": [
    {
      "name": "Плечевой пояс",
      "score": 85,
      "status": "Хороший уровень развития",
      "description": "Плечи являются одной из сильных сторон текущего телосложения. Хорошее развитие данной зоны формирует более выраженный спортивный силуэт и создаёт основу для дальнейшего улучшения пропорций."
    },
    {
      "name": "Грудная клетка",
      "score": 78,
      "status": "Есть потенциал увеличения объёма",
      "description": "Мышечная база присутствует, однако данная зона имеет потенциал дополнительного развития. Увеличение объёма позволит улучшить визуальную плотность верхней части тела."
    },
    {
      "name": "Спина",
      "score": 82,
      "status": "Хорошая ширина и плотность",
      "description": "Текущий уровень развития спины оценивается как хороший. Сохранение баланса между шириной и толщиной мышц позволит поддерживать правильную структуру корпуса."
    },
    {
      "name": "Нижняя часть тела",
      "score": 90,
      "status": "Наиболее развитая зона",
      "description": "Нижняя часть тела показывает высокий уровень развития и является одной из наиболее сильных зон текущей формы."
    }
  ],

  "muscles": [
    {
      "name": "Грудные мышцы",
      "level": "Средне развита",
      "assessment": "Мышечная база присутствует, визуальный тонус хороший, но есть потенциал для увеличения объёма.",
      "symmetry": "Симметрично",
      "recommendation": "Жим штанги лёжа 4x10-12, разводка гантелей 3x12-15, жим гантелей на наклонной скамье 3x10-12"
    },
    {
      "name": "Дельтовидные мышцы",
      "level": "Хорошо развиты",
      "assessment": "Хорошая форма и объём, передние и средние пучки развиты хорошо, задние требуют внимания.",
      "symmetry": "Симметрично",
      "recommendation": "Жим гантелей сидя 4x10-12, тяга к подбородку 3x12-15, разводка гантелей в наклоне 3x12-15"
    },
    {
      "name": "Трапециевидные мышцы",
      "level": "Средне развиты",
      "assessment": "Верхняя часть развита хорошо, средняя и нижняя требуют дополнительного внимания.",
      "symmetry": "Симметрично",
      "recommendation": "Шраги с гантелями 4x12-15, тяга штанги к подбородку 3x10-12, подъёмы рук в стороны в наклоне 3x12-15"
    },
    {
      "name": "Широчайшие мышцы спины",
      "level": "Хорошо развиты",
      "assessment": "Хорошая ширина и толщина, создают V-образный силуэт.",
      "symmetry": "Симметрично",
      "recommendation": "Тяга верхнего блока 4x10-12, тяга гантели в наклоне 3x12-15, тяга нижнего блока 3x12-15"
    },
    {
      "name": "Бицепс и трицепс",
      "level": "Хорошо развиты",
      "assessment": "Соотношение бицепса и трицепса сбалансированное, есть потенциал для увеличения объёма.",
      "symmetry": "Симметрично",
      "recommendation": "Подъём штанги на бицепс 3x10-12, молотковые сгибания 3x12-15, жим лёжа узким хватом 3x10-12, французский жим 3x12-15"
    },
    {
      "name": "Прямая мышца живота",
      "level": "Хорошо развита",
      "assessment": "Выраженная сегментация, рельеф просматривается хорошо.",
      "symmetry": "Симметрично",
      "recommendation": "Скручивания на пресс 3x20-25, подъёмы ног в висе 3x15-20, планка 3x60-90 сек"
    },
    {
      "name": "Косые мышцы живота",
      "level": "Средне развиты",
      "assessment": "Присутствуют, но требуют дополнительной проработки для лучшего визуального эффекта.",
      "symmetry": "Симметрично",
      "recommendation": "Косые скручивания 3x15-20 на каждую сторону, боковая планка 3x45-60 сек"
    },
    {
      "name": "Поясничный отдел",
      "level": "Хорошо развит",
      "assessment": "Достаточный мышечный корсет для поддержки позвоночника.",
      "symmetry": "Симметрично",
      "recommendation": "Гиперэкстензия 3x12-15, становая тяга на прямых ногах 3x10-12"
    },
    {
      "name": "Квадрицепс",
      "level": "Хорошо развит",
      "assessment": "Сильная зона, хороший объём и форма, выраженный рельеф.",
      "symmetry": "Симметрично",
      "recommendation": "Приседания со штангой 4x8-10, выпады с гантелями 3x10-12 на каждую ногу, разгибания ног в тренажёре 3x12-15"
    },
    {
      "name": "Бицепс бедра",
      "level": "Хорошо развит",
      "assessment": "Хороший объём, но есть потенциал для улучшения формы.",
      "symmetry": "Симметрично",
      "recommendation": "Румынская тяга 4x10-12, сгибания ног лёжа 3x12-15, гиперэкстензия с акцентом на ягодицы 3x12-15"
    },
    {
      "name": "Икроножные мышцы",
      "level": "Средне развиты",
      "assessment": "Стандартное развитие, требуется проработка для лучшей формы.",
      "symmetry": "Симметрично",
      "recommendation": "Подъёмы на носки стоя 4x15-20, подъёмы на носки сидя 3x15-20, выпады 3x12-15"
    },
    {
      "name": "Ягодичные мышцы",
      "level": "Хорошо развиты",
      "assessment": "Хорошая форма и плотность, являются сильной зоной.",
      "symmetry": "Симметрично",
      "recommendation": "Приседания 4x8-10, ягодичный мост 3x12-15, выпады с гантелями 3x10-12"
    }
  ],

  "posture": {
    "type": "Нормальная",
    "head": "Нейтральное положение, без наклона вперёд",
    "shoulders": "Нейтральное положение, без сутулости",
    "pelvis": "Нейтральное положение, без перекоса",
    "description": "Осанка оценивается как хорошая. Положение головы, плечевого пояса и таза находится в нейтральной позиции, что свидетельствует о правильной биомеханике тела."
  },

  "attention_zones": [
    {
      "name": "Верхняя часть тела (грудь, плечи, спина)",
      "problem": "Недостаточный объём мышечной массы в верхней части тела по сравнению с нижней",
      "solution": "Увеличить частоту тренировок верхней части тела до 2-3 раз в неделю. Сделать акцент на базовых упражнениях: жим лёжа, тяга штанги, жим гантелей сидя."
    },
    {
      "name": "Мышечный баланс между верхом и низом",
      "problem": "Выраженный дисбаланс: нижняя часть тела значительно превосходит верхнюю по объёму и силе",
      "solution": "Пересмотреть тренировочную программу: уменьшить объём тренировок ног до 1 раза в неделю, увеличить объём для верха тела. Добавить больше изолирующих упражнений для мышц верха."
    },
    {
      "name": "Икроножные мышцы",
      "problem": "Недостаточная проработка, отставание в объёме",
      "solution": "Добавить акцент на икроножные мышцы в каждую тренировку ног. Использовать разные углы и положения для полной проработки."
    }
  ],

  "priorities": [
    {
      "title": "1. Увеличение объёма верхней части тела",
      "description": "Основное направление улучшения связано с развитием плечевого пояса, груди и спины. Это позволит сделать силуэт более выраженным и сбалансировать пропорции. Рекомендуется делать акцент на базовые упражнения и постепенно увеличивать рабочие веса."
    },
    {
      "title": "2. Повышение качества мышечной структуры",
      "description": "Следующий этап развития должен быть направлен не только на увеличение массы тела, но и на улучшение плотности и формы мышц. Добавить больше изолирующих упражнений, работать над пампингом и качеством выполнения каждого движения."
    },
    {
      "title": "3. Сохранение текущего уровня композиции тела",
      "description": "Важно сохранить благоприятное соотношение мышечной ткани и жировой массы во время дальнейшего прогресса. Контролировать питание и калорийность, сохраняя умеренный профицит для роста мышц."
    }
  ],

  "training_days": [
    {
      "day": "День 1 — Верх тела (силовой)",
      "exercises": [
        {"name": "Жим штанги лёжа", "sets": "4", "reps": "8-10"},
        {"name": "Тяга штанги в наклоне", "sets": "4", "reps": "8-10"},
        {"name": "Жим гантелей сидя", "sets": "4", "reps": "10-12"},
        {"name": "Тяга верхнего блока", "sets": "3", "reps": "10-12"},
        {"name": "Разводка гантелей лёжа", "sets": "3", "reps": "12-15"},
        {"name": "Подъём штанги на бицепс", "sets": "3", "reps": "10-12"}
      ]
    },
    {
      "day": "День 2 — Низ тела (силовой)",
      "exercises": [
        {"name": "Приседания со штангой", "sets": "4", "reps": "8-10"},
        {"name": "Румынская тяга", "sets": "4", "reps": "10-12"},
        {"name": "Выпады с гантелями", "sets": "3", "reps": "10-12"},
        {"name": "Сгибания ног лёжа", "sets": "3", "reps": "12-15"},
        {"name": "Подъёмы на носки стоя", "sets": "4", "reps": "15-20"},
        {"name": "Ягодичный мост", "sets": "3", "reps": "12-15"}
      ]
    },
    {
      "day": "День 3 — Верх тела (объёмный)",
      "exercises": [
        {"name": "Жим гантелей лёжа", "sets": "4", "reps": "10-12"},
        {"name": "Тяга гантели в наклоне", "sets": "4", "reps": "10-12"},
        {"name": "Жим гантелей сидя", "sets": "4", "reps": "12-15"},
        {"name": "Тяга нижнего блока", "sets": "3", "reps": "12-15"},
        {"name": "Французский жим", "sets": "3", "reps": "12-15"},
        {"name": "Молотковые сгибания", "sets": "3", "reps": "12-15"}
      ]
    },
    {
      "day": "День 4 — Коррекция и функционал",
      "exercises": [
        {"name": "Планка", "sets": "3", "reps": "60-90 сек"},
        {"name": "Скручивания на пресс", "sets": "3", "reps": "20-25"},
        {"name": "Косые скручивания", "sets": "3", "reps": "15-20"},
        {"name": "Гиперэкстензия", "sets": "3", "reps": "12-15"},
        {"name": "Кардио (интервальное)", "sets": "15", "reps": "мин"},
        {"name": "Стретчинг", "sets": "10", "reps": "мин"}
      ]
    }
  ],

  "nutrition": [
    "Цель: умеренный профицит калорий (+200-300 ккал/день) для набора мышечной массы",
    "Белок: 1.8-2.2 г/кг веса (≈ 140-170 г/день) — куриная грудка, рыба, яйца, творог, протеин",
    "Жиры: 0.8-1 г/кг веса (≈ 60-80 г/день) — оливковое масло, орехи, авокадо, жирная рыба",
    "Углеводы: 4-5 г/кг веса (≈ 310-390 г/день) — рис, гречка, овсянка, картофель, фрукты",
    "Питание: 4-5 приёмов пищи в день, распределение белка равномерно",
    "Водный режим: 2.5-3 литра воды в день",
    "Ограничить: быстрые углеводы, трансжиры, алкоголь",
    "Добавить: BCAA во время тренировки, креатин, омега-3"
  ],

  "recovery": [
    "Сон: 7-8 часов в сутки, ложиться и вставать в одно и то же время",
    "Дни отдыха: 2 полных дня отдыха в неделю для восстановления ЦНС",
    "Стретчинг: 10-15 минут после каждой тренировки",
    "Массаж: 1-2 раза в неделю для улучшения кровообращения и расслабления мышц",
    "Баня/сауна: 1 раз в неделю для ускорения восстановления",
    "Контрастный душ: после тренировки для улучшения тонуса сосудов",
    "Прогулки на свежем воздухе: ежедневно 20-30 минут для снижения стресса"
  ],

  "progress": {
    "recheck": "Через 4-6 недель",
    "targets": [
      "Увеличение мышечной массы на 1-2 кг (чистая масса)",
      "Сохранение процента жира на текущем уровне (13-14%)",
      "Увеличение силовых показателей на 10-15% во всех базовых упражнениях",
      "Улучшение пропорций тела: увеличение объёма грудной клетки на 2-3 см",
      "Улучшение качества мышц: появление более чёткого рельефа",
      "Контроль качества сна и восстановления"
    ]
  }
}`, nil
}

// func (c *GeminiClient) GenerateBioscanJSON(
// 	ctx context.Context,
// 	data []byte,
// 	mimeType string,
// 	contextInfo string,
// ) (string, error) {

// 	if strings.TrimSpace(c.apiKey) == "" {
// 		return "", fmt.Errorf("empty gemini api key")
// 	}

// 	prompt := `
// Ты анализируешь фото тела для фитнес отчёта.

// Верни ТОЛЬКО JSON.
// Без markdown.
// Без комментариев.

// Формат:

// {
// "score":85,
// "level":"Хорошая форма",
// "summary":"описание",

// "body":{
// "height":"",
// "weight":"",
// "muscle_mass":"",
// "fat":""
// },

// "composition":"",

// "profile":{
// "composition":80,
// "muscle_development":75,
// "balance":85,
// "potential":90
// },

// "zones":[
// {
// "name":"",
// "score":80,
// "status":"",
// "description":""
// }
// ],

// "muscles":[
// {
// "name":"",
// "level":"",
// "assessment":"",
// "symmetry":"",
// "recommendation":""
// }
// ],

// "posture":{
// "type":"",
// "head":"",
// "shoulders":"",
// "pelvis":"",
// "description":""
// },

// "attention_zones":[
// {
// "name":"",
// "problem":"",
// "solution":""
// }
// ],

// "priorities":[
// {
// "title":"",
// "description":""
// }
// ],

// "training_days":[
// {
// "day":"",
// "exercises":[
// {
// "name":"",
// "sets":"",
// "reps":""
// }
// ]
// }
// ],

// "nutrition":[],
// "recovery":[],

// "progress":{
// "recheck":"",
// "targets":[]
// }

// }

// Данные пользователя:

// ` + contextInfo

// 	parts := []geminiPart{

// 		{
// 			InlineData: &geminiInlineData{
// 				MimeType: mimeType,
// 				Data:     base64.StdEncoding.EncodeToString(data),
// 			},
// 		},

// 		{
// 			Text: prompt,
// 		},
// 	}

// 	result, err := c.generateRaw(ctx, parts)

// 	if err != nil {
// 		return "", err
// 	}

// 	return normalizeJSONResponse(result), nil
// }

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
// func (c *GeminiClient) GenerateBioscanAnalysis(ctx context.Context, data []byte, mimeType string, contextInfo string) (string, error) {
// 	if strings.TrimSpace(c.apiKey) == "" {
// 		return noKeyFallback(), nil
// 	}

// 	if len(data) == 0 {
// 		return "", fmt.Errorf("empty file data")
// 	}

// 	// Проверяем размер фото - для Bioscan нужно качественное фото
// 	if len(data) < 50000 {
// 		return "📸 **Фото слишком маленькое**\n\n" +
// 			"Пожалуйста, отправьте **качественное фото** в полный рост.\n\n" +
// 			"📌 Рекомендации:\n" +
// 			"• Фото в полный рост (анфас)\n" +
// 			"• Хорошее освещение\n" +
// 			"• В обтягивающей одежде или без неё\n" +
// 			"• Стоять прямо, руки вдоль тела\n\n" +
// 			"🔄 Отправьте новое фото для анализа.", nil
// 	}

// 	prompt := c.buildBioscanPrompt(contextInfo)

// 	parts := []geminiPart{
// 		{
// 			InlineData: &geminiInlineData{
// 				MimeType: mimeType,
// 				Data:     base64.StdEncoding.EncodeToString(data),
// 			},
// 		},
// 		{Text: prompt},
// 	}

// 	return c.generate(ctx, parts)
// }

// buildBioscanPrompt - промпт для Bioscan
func (c *GeminiClient) buildBioscanPrompt(contextInfo string) string {

	return fmt.Sprintf(`
Ты профессиональный фитнес-тренер и специалист по анализу состава тела.

Проанализируй фото тела.

ВАЖНО:
Ответ должен быть ТОЛЬКО валидным JSON.
Без markdown.
Без комментариев.
Без текста до или после JSON.

Информация пользователя:

%s


ФОРМАТ JSON:

{
  "score": 0,
  "level": "",
  "summary": "",

  "body": {
    "height": "",
    "weight": "",
    "muscle_mass": "",
    "fat": ""
  },


  "composition": "",


  "profile": {
    "composition": 0,
    "muscle_development": 0,
    "balance": 0,
    "potential": 0
  },


  "zones": [

    {
      "name": "",
      "score": 0,
      "status": "",
      "description": "",
      "recommendation": ""
    }

  ],


  "posture": {

    "type": "",
    "head": "",
    "shoulders": "",
    "pelvis": ""

  },


  "weak_points": [

    {
      "zone": "",
      "problem": "",
      "solution": ""
    }

  ],


  "training": [

    {
      "day": "",
      "exercises": [
        ""
      ]
    }

  ],


  "nutrition": [
    ""
  ],


  "recovery": [
    ""
  ],


  "progress": {

    "weeks": "",
    "targets": [
      ""
    ]

  }

}


ПРАВИЛА:

- zones создавай динамически.
- Не ограничивай количество мышечных групп.
- Добавляй только те зоны, которые реально можно оценить по фото.
- score от 0 до 100.
- Пиши коротко и профессионально.
- Если показатель невозможно определить по фото — укажи "нет данных".
- Не придумывай рост, вес и проценты жира.


`, contextInfo)

}

func (c *GeminiClient) buildBioscanJSONPrompt(
	contextInfo string,
) string {

	return fmt.Sprintf(`
Ты профессиональный фитнес-эксперт и специалист по анализу телосложения.

Проанализируй фотографию тела.

Верни ТОЛЬКО JSON.
Без markdown.
Без комментариев.
Без текста до или после JSON.


Структура ответа строго:


{
"score":0,
"level":"",
"summary":"",

"body":{
"height":"",
"weight":"",
"muscle_mass":"",
"fat":""
},

"composition":"",

"profile":{
"composition":0,
"muscle_development":0,
"balance":0,
"potential":0
},

"zones":[

{
"name":"",
"score":0,
"status":"",
"description":""
}

]

}



Правила:

score — общая оценка 0-100

profile значения — 0-100

zones:
создай столько зон, сколько реально нужно.
НЕ ограничивай количество.

Примеры зон:

- грудные мышцы
- плечи
- спина
- руки
- пресс
- ноги
- ягодицы
- осанка


Оценивай только визуально.

Не ставь медицинские диагнозы.

Информация пользователя:

%s

`, contextInfo)

}

func normalizeJSONResponse(text string) string {
	text = strings.TrimSpace(text)

	// убираем markdown
	text = strings.ReplaceAll(text, "```json", "")
	text = strings.ReplaceAll(text, "```", "")

	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")

	if start >= 0 && end > start {
		text = text[start : end+1]
	}

	return strings.TrimSpace(text)
}

func (c *GeminiClient) generateRaw(ctx context.Context, parts []geminiPart) (string, error) {
	log.Printf("🔄 Starting Gemini RAW request...")

	payload := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{
				Text: "Ты должен вернуть только JSON. Без markdown. Без комментариев.",
			}},
		},
		Contents: []geminiContent{{
			Role:  "user",
			Parts: parts,
		}},
		GenerationConfig: &geminiGenerationConfig{
			Temperature:     0.1,
			MaxOutputTokens: 4000,
			TopP:            0.95,
			TopK:            40,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		c.model,
		c.apiKey,
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(
			"gemini error: %s",
			string(respBody),
		)
	}

	var result geminiResponse

	err = json.Unmarshal(respBody, &result)
	if err != nil {
		return "", err
	}

	text := extractGeminiText(&result)

	log.Printf("📝 RAW JSON response length: %d", len(text))

	return text, nil
}
