package claude

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/theamornoir/analyzpro/internal/locales"
	"golang.org/x/sync/semaphore"
)

// Model - модель Claude по умолчанию. Умеет vision (фото) и нативное чтение
// PDF-документов через Messages API.
const Model = "claude-3-5-sonnet-20241022"

// sem ограничивает число одновременных исходящих запросов к Claude.
// При 500 пользователях, разом загрузивших анализы, это предотвращает
// массовые 429 от Anthropic и OOM по памяти (крупные PDF/фото держим в
// памяти только для in-flight задач в рамках окна семафора, а не для всех
// 500 сразу). Запросы, не попавшие в окно, честно ждут в очереди.
var sem = semaphore.NewWeighted(8) // 8 одновременных AI-запросов

// Attachment - одно вложение (изображение или PDF) для мультимодального запроса.
type Attachment struct {
	Data     []byte
	MimeType string
}

// Client - единый мультимодальный Claude-клиент. Полностью заменяет старый
// оркестратор и всех провайдеров (Gemini / DeepSeek / YandexGPT / OpenRouter).
//
// Главное свойство: ВСЕ файлы (фото и PDF) передаются в ОДНОМ запросе к
// Messages API вместе с промптом. Изображения уходят image-блоком, PDF -
// document-блоком (Claude читает PDF нативно, видит все цифры и таблицы).
// Это гарантирует, что ни один анализ / показатель не будет потерян и связь
// между несколькими вложениями (например, 4 фото Bioscan) сохраняется.
type Client struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewClient создаёт Claude-клиент. Ключ берётся из окружения ANTHROPIC_API_KEY.
// Если ключ пуст - клиент всё равно создаётся, но вызовы методов вернут
// понятную ошибку. Это сохраняет прежнее поведение: бот стартует даже без
// сконфигурированного AI, а сам анализ аккуратно «падает» при вызове.
func NewClient() *Client {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Printf("[claude] ANTHROPIC_API_KEY не задан - AI-вызовы будут недоступны")
	}
	return &Client{
		apiKey:     apiKey,
		model:      Model,
		httpClient: newHTTPClient(),
	}
}

// newHTTPClient - HTTP-клиент с учётом системного прокси (HTTP_PROXY /
// HTTPS_PROXY) и увеличенным таймаутом (анализ PDF/изображений долгий).
func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			IdleConnTimeout:       90 * time.Second,
			MaxIdleConns:          50,
		},
	}
}

// Generate - текстовый запрос без вложений.
func (c *Client) Generate(ctx context.Context, systemPrompt, prompt string, maxTokens int) (string, error) {
	return c.GenerateWithFiles(ctx, systemPrompt, prompt, nil, maxTokens)
}

// GenerateWithFiles - единый мультимодальный вызов. Промпт + ВСЕ файлы
// (фото и PDF) помещаются в ОДНО сообщение пользователя. Это критично для
// целостности анализа: Claude видит связь между всеми вложениями сразу.
func (c *Client) GenerateWithFiles(ctx context.Context, systemPrompt, prompt string, files []Attachment, maxTokens int) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	// Ограничиваем конкурентность: не более 8 одновременных запросов к
	// Claude. Лишние ждут в очереди (без блокировки бота - каждый вызов
	// идёт в своей горутине), что защищает от лимитов RPM/TPM и перегрузки.
	if err := sem.Acquire(ctx, 1); err != nil {
		return "", err
	}
	defer sem.Release(1)

	content := []any{
		map[string]any{"type": "text", "text": prompt},
	}
	for _, f := range files {
		if len(f.Data) == 0 {
			continue
		}
		block, err := contentBlock(f)
		if err != nil {
			return "", err
		}
		content = append(content, block)
	}

	body := map[string]any{
		"model":      c.model,
		"max_tokens": maxTokens,
		"system":     systemPrompt,
		"messages": []any{
			map[string]any{"role": "user", "content": content},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("claude api error (%d): %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("claude response parse error: %w (body: %s)", err, string(respBody))
	}

	var sb strings.Builder
	for _, c := range parsed.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		return "", fmt.Errorf("claude returned empty response")
	}

	return text, nil
}

// contentBlock преобразует вложение в content-блок Anthropic API:
// image/* -> image-блок, application/pdf -> document-блок (нативное чтение).
func contentBlock(att Attachment) (map[string]any, error) {
	base64Data := base64.StdEncoding.EncodeToString(att.Data)

	if strings.HasPrefix(att.MimeType, "image/") {
		return map[string]any{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": att.MimeType,
				"data":       base64Data,
			},
		}, nil
	}

	if att.MimeType == "application/pdf" {
		return map[string]any{
			"type": "document",
			"source": map[string]any{
				"type":       "base64",
				"media_type": "application/pdf",
				"data":       base64Data,
			},
		}, nil
	}

	return nil, fmt.Errorf("unsupported mime type for claude: %s", att.MimeType)
}

// isSupportedMime сообщает, умеет ли Claude анализировать данный MIME-тип.
// Claude поддерживает изображения и PDF-документы напрямую.
func isSupportedMime(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/") || mimeType == "application/pdf"
}

// --- Обёртки под нужды сервиса анализа (имена методов совместимы с интерфейсом) ---

// GenerateAnalysisSummary - текстовый анализ по введённому тексту.
func (c *Client) GenerateAnalysisSummary(ctx context.Context, userInput string) (string, error) {
	return c.Generate(ctx,
		"Ты - медицинский аналитик. Проанализируй данные и дай рекомендации.",
		"Ты - медицинский аналитик. Проанализируй данные и дай рекомендации.\n\n"+userInput,
		3000)
}

// GenerateAnalysisJSON - структурированный JSON-анализ по тексту.
func (c *Client) GenerateAnalysisJSON(ctx context.Context, userInput string) (string, error) {
	return c.Generate(ctx,
		"Ты - медицинский аналитик. Верни ответ строго в формате JSON, без markdown и комментариев.",
		"Ты - медицинский аналитик. Верни ответ строго в формате JSON, без markdown и комментариев.\n\n"+userInput,
		4000)
}

// GenerateAnalysisFromFileWithContext - анализ файла (изображение/PDF) с контекстом.
func (c *Client) GenerateAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	if !isSupportedMime(mimeType) {
		return "", fmt.Errorf("claude supports only image/pdf files for analysis")
	}
	return c.GenerateWithFiles(ctx,
		"Ты - опытный врач-диагност. Проанализируй приложенные медицинские изображения/документы и дай развёрнутый анализ с рекомендациями.",
		contextText,
		[]Attachment{{Data: data, MimeType: mimeType}},
		4000)
}

// GenerateAnalysisFromFilesWithContext - анализ НЕСКОЛЬКИХ файлов (изображения/
// PDF) одним мультимодальным запросом. Все вложения помещаются в ОДНО
// сообщение вместе с промптом - как в Bioscan PRO. Claude видит связь между
// всеми анализами сразу, ничего не теряется.
func (c *Client) GenerateAnalysisFromFilesWithContext(ctx context.Context, data [][]byte, mimeTypes []string, contextText string) (string, error) {
	atts := attachmentsFromPairs(data, mimeTypes)
	if len(atts) == 0 {
		return "", fmt.Errorf("no supported file data provided")
	}
	return c.GenerateWithFiles(ctx,
		"Ты - опытный врач-диагност. Проанализируй приложенные медицинские изображения и документы и дай развёрнутый анализ с рекомендациями. Если файлов несколько - проанализируй их совместно как единый набор анализов одного пациента.",
		contextText,
		atts,
		6000)
}

// GenerateAnalysisFromFilesJSON - структурированный JSON-анализ нескольких
// файлов одним запросом (показатели для дашборда «Мой профиль»).
func (c *Client) GenerateAnalysisFromFilesJSON(ctx context.Context, data [][]byte, mimeTypes []string, contextText string) (string, error) {
	atts := attachmentsFromPairs(data, mimeTypes)
	if len(atts) == 0 {
		return "", fmt.Errorf("no supported file data provided")
	}
	return c.GenerateWithFiles(ctx,
		"Ты - опытный врач-диагност. Верни ответ строго в формате JSON, без markdown и комментариев.",
		locales.PromptForAnalysisJSON(contextText),
		atts,
		8000)
}

// GenerateBioscanJSON - JSON-результат Bioscan по фотографиям (все сразу в одном запросе).
func (c *Client) GenerateBioscanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	atts := attachmentsFrom(photosData, mimeType)
	if len(atts) == 0 {
		return "", fmt.Errorf("no photo data provided")
	}
	return c.GenerateWithFiles(ctx,
		"Ты - опытный врач-диагност. Верни ответ строго в формате JSON, без markdown и комментариев.",
		locales.PromptForBioscan(contextInfo),
		atts,
		8000)
}

// GenerateBodyScanJSON - JSON премиального отчёта Bioscan PRO (все фото в одном запросе).
func (c *Client) GenerateBodyScanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	atts := attachmentsFrom(photosData, mimeType)
	if len(atts) == 0 {
		return "", fmt.Errorf("no photo data provided")
	}
	return c.GenerateWithFiles(ctx,
		"Ты - эксперт премиального сервиса биометрической аналитики тела. Верни ответ строго в формате JSON, без markdown и комментариев.",
		locales.PromptForBodyScanJSON(contextInfo),
		atts,
		8000)
}

// GenerateAnalysisFromFileJSON - JSON-анализ из файла (изображение/PDF).
func (c *Client) GenerateAnalysisFromFileJSON(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	if !isSupportedMime(mimeType) {
		return "", fmt.Errorf("claude supports only image/pdf files for analysis")
	}
	return c.GenerateWithFiles(ctx,
		"Ты - опытный врач-диагност. Верни ответ строго в формате JSON, без markdown и комментариев.",
		locales.PromptForAnalysisJSON(contextText),
		[]Attachment{{Data: data, MimeType: mimeType}},
		8000)
}

// GenerateDossierJSON - JSON универсального отчёта-досье здоровья.
func (c *Client) GenerateDossierJSON(ctx context.Context, userInput string) (string, error) {
	return c.Generate(ctx,
		"Ты - опытный врач-диагност и аналитик здоровья. Верни ответ строго в формате JSON, без markdown и комментариев.",
		"Ты - опытный врач-диагност и аналитик здоровья. Верни ответ строго в формате JSON, без markdown и комментариев.\n\n"+userInput,
		8000)
}

// attachmentsFrom собирает непустые фото во вложения для одного запроса.
func attachmentsFrom(photosData [][]byte, mimeType string) []Attachment {
	atts := make([]Attachment, 0, len(photosData))
	for _, d := range photosData {
		if len(d) > 0 {
			atts = append(atts, Attachment{Data: d, MimeType: mimeType})
		}
	}
	return atts
}

// attachmentsFromPairs собирает непустые файлы во вложения для одного
// мультифайлового запроса, сопоставляя каждому блоку данных соответствующий
// MIME-тип из параллельного среза. Игнорирует неподдерживаемые типы
// (Claude умеет только image/* и application/pdf).
func attachmentsFromPairs(data [][]byte, mimeTypes []string) []Attachment {
	atts := make([]Attachment, 0, len(data))
	for i, d := range data {
		if len(d) == 0 {
			continue
		}
		mt := ""
		if i < len(mimeTypes) {
			mt = mimeTypes[i]
		}
		if !isSupportedMime(mt) {
			continue
		}
		atts = append(atts, Attachment{Data: d, MimeType: mt})
	}
	return atts
}
