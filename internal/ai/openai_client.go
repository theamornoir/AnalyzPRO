package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func NewClient(apiKey string, model string) *Client {
	if model == "" {
		model = "gpt-4o-mini"
	}

	return &Client{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1/chat/completions",
		model:   model,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) GenerateAnalysisSummary(ctx context.Context, userInput string) (string, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return BuildDoctorTemplate("AI не подключён в текущем окружении.", "Для полноценной аналитики нужно настроить OPENAI_API_KEY.", "Проверьте конфигурацию сервиса.", "Подождите, пока API будет подключено.", "Не используйте этот ответ как медицинское заключение."), nil
	}

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
`, userInput)

	payload := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: "Ты опытный врач-диагност. Отвечай только по фиксированному шаблону, без текста вне формата."},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fallback := BuildDoctorTemplate(
			"Сервис ИИ временно недоступен.",
			"Запрос был получен, но обработка сейчас ограничена по квоте или скорости доступа.",
			"На текущий момент стоит повторить запрос чуть позже.",
			"Подождите 1–2 минуты и отправьте анализ повторно.",
			"Этот ответ не является диагнозом и служит только для безопасного уведомления о временной недоступности AI.",
		)
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusInternalServerError {
			return fallback, nil
		}
		return "", fmt.Errorf("openai request failed with status %d", resp.StatusCode)
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Error != nil {
		return "", fmt.Errorf("openai error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		return BuildDoctorTemplate("Сервис анализа получил пустой ответ.", "Проверьте входные данные и конфигурацию AI.", "Повторите запрос позже.", "После проверки данных результат будет подготовлен повторно.", "Не используйте этот ответ как диагноз."), nil
	}

	return normalizeAIResponse(result.Choices[0].Message.Content), nil
}

func normalizeAIResponse(raw string) string {
	clean := strings.TrimSpace(raw)
	clean = strings.Trim(clean, "```")
	clean = strings.TrimSpace(clean)
	return clean
}

func BuildDoctorTemplate(summary, important, risks, recommendation, footnote string) string {
	return fmt.Sprintf("📌 Краткий вывод\n%s\n\n🩺 Что важно\n- %s\n\n⚠️ Показатели, требующие внимания\n- %s\n\n✅ Рекомендации\n1. %s\n\nℹ️ Важно\n%s", summary, important, risks, recommendation, footnote)
}
