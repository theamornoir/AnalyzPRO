package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/theamornoir/analyzpro/internal/locales"
	"golang.org/x/net/proxy"
	"golang.org/x/sync/semaphore"
)

// Дефолтные значения конфигурации клиента. Используются config.Load
// как значения по умолчанию для env-переменных (GOOGLE_AI_MODEL и т.п.).
const (
	// DefaultModel - модель по умолчанию: умеет vision (фото) и нативное
	// чтение PDF через generateContent API.
	DefaultModel = "gemini-2.5-flash"
	// DefaultAPIBase - базовый URL Generative Language API (REST
	// generateContent).
	DefaultAPIBase = "https://generativelanguage.googleapis.com/v1beta/models"
	// DefaultMaxConcurrency - макс. число одновременных запросов к Gemini.
	// Низкий предел (3-4) защищает от 429 при невысоком RPM тарифа Gemini:
	// лишние запросы честно встают в очередь семафора, а не сыплют ошибки.
	DefaultMaxConcurrency = 4
	// DefaultTimeout - таймаут одного AI-запроса (PDF/изображения долгие).
	DefaultTimeout = 120 * time.Second
)

// Настройки надёжности пайплайна. Не содержат пользовательского текста -
// это константы транспорта/генерации (не env: ключи/модели/прокси уже в
// config, а тюнинг повторов/температуры не требует выноса в env).
const (
	// defaultMaxRetries - число повторов на 429/5xx и сетевую ошибку.
	defaultMaxRetries = 3
	// defaultRetryBackoff - начальный backoff перед повтором (×2 каждый шаг).
	defaultRetryBackoff = 400 * time.Millisecond
	// defaultJSONTemperature - пониженная температура для JSON-методов:
	// детерминированный, стабильный JSON без «шатающихся» полей.
	defaultJSONTemperature = 0.2
	// defaultMaxOutputTokensJSON - потолок токенов выхода для JSON-отчётов
	// (Bioscan PRO / досье могут быть длинными; Flash умеет до 65k).
	defaultMaxOutputTokensJSON = 16384
	// responseMimeJSON - режим генерации чистого JSON без markdown-обёртки.
	responseMimeJSON = "application/json"
)

// safetySettings - отключаем фильтры безопасности. Медицинский контент
// (анализы, фото тела) Gemini по умолчанию ложно блокирует, возвращая
// пустой ответ. BLOCK_NONE гарантирует, что модель ответит. Это
// структурная конфигурация API, а не пользовательский текст.
var safetySettings = []map[string]any{
	{"category": "HARM_CATEGORY_HARASSMENT", "threshold": "BLOCK_NONE"},
	{"category": "HARM_CATEGORY_HATE_SPEECH", "threshold": "BLOCK_NONE"},
	{"category": "HARM_CATEGORY_SEXUALLY_EXPLICIT", "threshold": "BLOCK_NONE"},
	{"category": "HARM_CATEGORY_DANGEROUS_CONTENT", "threshold": "BLOCK_NONE"},
}

// Config - параметры Gemini-клиента. Все значения передаются явно
// (из config.Config, который читает env), поэтому методы клиента не делают
// os.Getenv и не содержат хардкода - они чистые.
type Config struct {
	// APIKey - ключ GOOGLE_GEMINI_API_KEY. Пустой - вызовы вернут ошибку.
	APIKey string
	// Model - имя модели (напр. gemini-2.5-flash).
	Model string
	// Proxy - egress-прокси (socks5/http/https) для запросов к Gemini.
	Proxy string
	// APIBase - базовый URL Generative Language API.
	APIBase string
	// MaxConcurrency - лимит одновременных запросов.
	MaxConcurrency int
	// Timeout - таймаут одного запроса.
	Timeout time.Duration
}

// Attachment - одно вложение (изображение или PDF) для мультимодального запроса.
type Attachment struct {
	Data     []byte
	MimeType string
}

// genOptions - внутренние параметры генерации (чистые, без env/текста).
type genOptions struct {
	maxTokens    int
	temperature  *float64
	responseMime string
}

// Client - единый мультимодальный Gemini-клиент (единственный провайдер ИИ).
// Все файлы (фото и PDF) передаются в ОДНОМ запросе к generateContent вместе
// с промптом (inlineData). Gemini читает PDF нативно и принимает изображения
// напрямую, поэтому ни один показатель не теряется.
type Client struct {
	apiKey     string
	model      string
	apiBase    string
	httpClient *http.Client
	sem        *semaphore.Weighted
}

// NewClient создаёт Gemini-клиент из явной Config (без чтения env и без
// логирования внутри - чистый конструктор). Ошибка возможна только при
// некорректной настройке прокси (тогда вызывающий логирует и при
// необходимости создаёт клиента повторно без прокси).
func NewClient(cfg Config) (*Client, error) {
	httpClient, err := buildHTTPClient(cfg.Proxy, cfg.Timeout)
	if err != nil {
		return nil, err
	}
	conc := cfg.MaxConcurrency
	if conc <= 0 {
		conc = DefaultMaxConcurrency
	}
	return &Client{
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		apiBase:    cfg.APIBase,
		httpClient: httpClient,
		sem:        semaphore.NewWeighted(int64(conc)),
	}, nil
}

// buildHTTPClient - чистый конструктор HTTP-клиента: на входе прокси и
// таймаут, на выходе клиент. Не читает env, не логирует. При ошибке
// настройки SOCKS5 возвращает ошибку (вызывающий решает, что делать).
func buildHTTPClient(proxyAddr string, timeout time.Duration) (*http.Client, error) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	baseDialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		DialContext:           baseDialer.DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          50,
	}

	if proxyAddr != "" {
		u, err := url.Parse(proxyAddr)
		if err != nil {
			return nil, fmt.Errorf("invalid GEMINI_PROXY %q: %w", proxyAddr, err)
		}
		if u.Scheme == "socks5" {
			sd, serr := proxy.SOCKS5("tcp", u.Host, nil, baseDialer)
			if serr != nil {
				return nil, fmt.Errorf("setup SOCKS5 proxy %s: %w", proxyAddr, serr)
			}
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return sd.Dial(network, addr)
			}
		} else {
			transport.Proxy = http.ProxyURL(u)
		}
	} else {
		transport.Proxy = http.ProxyFromEnvironment
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}, nil
}

// Generate - текстовый запрос без вложений.
func (c *Client) Generate(ctx context.Context, systemPrompt, prompt string, maxTokens int) (string, error) {
	return c.generateWithConfig(ctx, systemPrompt, prompt, nil, genOptions{maxTokens: maxTokens})
}

// GenerateWithFiles - единый мультимодальный вызов. Промпт + ВСЕ файлы
// (фото и PDF) помещаются в ОДНО сообщение пользователя (inlineData). Это
// критично для целостности анализа: Gemini видит связь между всеми
// вложениями сразу. Промпты и тексты ошибок берутся из locales.
func (c *Client) GenerateWithFiles(ctx context.Context, systemPrompt, prompt string, files []Attachment, maxTokens int) (string, error) {
	return c.generateWithConfig(ctx, systemPrompt, prompt, files, genOptions{maxTokens: maxTokens})
}

// generateJSON - JSON-генерация (для всех *_JSON методов): фиксируем
// пониженную температуру (детерминизм) и responseMimeType=application/json
// (чистый JSON без markdown-обёртки), поднимаем потолок токенов выхода,
// чтобы длинные отчёты (Bioscan PRO / досье) не обрезались.
func (c *Client) generateJSON(ctx context.Context, systemPrompt, prompt string, files []Attachment, maxTokens int) (string, error) {
	t := defaultJSONTemperature
	return c.generateWithConfig(ctx, systemPrompt, prompt, files, genOptions{
		maxTokens:    maxTokens,
		temperature:  &t,
		responseMime: responseMimeJSON,
	})
}

// generateWithConfig - ядро генерации: семафор конкурентности, сборка
// запроса (systemInstruction + parts + generationConfig + safetySettings),
// retry с экспоненциальным backoff на 429/5xx/сетевую ошибку. Промпты и
// тексты ошибок - из locales; env/хардкод отсутствуют (чистые функции).
func (c *Client) generateWithConfig(ctx context.Context, systemPrompt, prompt string, files []Attachment, opts genOptions) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf(locales.ErrGeminiKeyNotSet)
	}

	// Ограничиваем конкурентность (защита от 429 и OOM). Лишние запросы
	// честно ждут в очереди, не блокируя бот (каждый вызов в своей горутине).
	if err := c.sem.Acquire(ctx, 1); err != nil {
		return "", err
	}
	defer c.sem.Release(1)

	parts := []any{
		map[string]any{"text": prompt},
	}
	for _, f := range files {
		if len(f.Data) == 0 {
			continue
		}
		if !isSupportedMime(f.MimeType) {
			return "", fmt.Errorf(locales.ErrGeminiUnsupportedMime, f.MimeType)
		}
		parts = append(parts, contentPart(f))
	}

	genConfig := map[string]any{
		"maxOutputTokens": opts.maxTokens,
	}
	if opts.temperature != nil {
		genConfig["temperature"] = *opts.temperature
	}
	if opts.responseMime != "" {
		genConfig["responseMimeType"] = opts.responseMime
	}

	body := map[string]any{
		"systemInstruction": map[string]any{
			"parts": []any{
				map[string]any{"text": systemPrompt},
			},
		},
		"contents": []any{
			map[string]any{
				"role":  "user",
				"parts": parts,
			},
		},
		"generationConfig": genConfig,
		// Отключаем фильтры безопасности - иначе мед. контент блокируется.
		"safetySettings": safetySettings,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	endpoint := fmt.Sprintf("%s/%s:generateContent?key=%s", c.apiBase, c.model, c.apiKey)
	respBody, err := c.doRequestWithRetry(ctx, endpoint, jsonBody)
	if err != nil {
		return "", err
	}

	return parseGenerateResponse(respBody)
}

// doRequestWithRetry - HTTP POST с повторами на транспортные сбои и
// временные ошибки сервера (429 rate-limit, 5xx). Неретраяемые ошибки
// (2xx и 4xx кроме 429) возвращаются сразу; 400 (bad request, напр. неверный
// ключ/модель) повторять бессмысленно. Уважает отмену ctx (прерывает
// ожидание backoff).
func (c *Client) doRequestWithRetry(ctx context.Context, endpoint string, body []byte) ([]byte, error) {
	backoff := defaultRetryBackoff
	var lastErr error

	for attempt := 0; attempt < defaultMaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("content-type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Сетевая ошибка (прокси/таймаут/geo) - повторяем.
			lastErr = fmt.Errorf(locales.ErrGeminiTransport, err)
			if !sleepWithCtx(ctx, backoff) {
				return nil, ctx.Err()
			}
			backoff *= 2
			continue
		}

		respBody, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			lastErr = fmt.Errorf(locales.ErrGeminiTransport, rerr)
			if !sleepWithCtx(ctx, backoff) {
				return nil, ctx.Err()
			}
			backoff *= 2
			continue
		}

		// 429 (rate limit) и 5xx (временные сбои сервера) - повторяем.
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf(locales.ErrGeminiAPIError, resp.StatusCode, string(respBody))
			if !sleepWithCtx(ctx, backoff) {
				return nil, ctx.Err()
			}
			backoff *= 2
			continue
		}

		// Неретраяемая ошибка (в т.ч. 400) - возвращаем сразу, парсинг
		// решит, что с ней делать.
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf(locales.ErrGeminiAPIError, resp.StatusCode, string(respBody))
		}

		return respBody, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf(locales.ErrGeminiRetryExhausted, fmt.Errorf("no attempts executed"))
	}
	return nil, fmt.Errorf(locales.ErrGeminiRetryExhausted, lastErr)
}

// sleepWithCtx - спит backoff секунд либо возвращает false при отмене ctx.
func sleepWithCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// parseGenerateResponse - декодирует тело ответа Gemini, собирает текст из
// всех кандидатов. Пустой текст (блокировка SAFETY / исчерпание лимита
// токенов при успешном статусе) трактуется как ошибка.
func parseGenerateResponse(respBody []byte) (string, error) {
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf(locales.ErrGeminiResponseParse, err, string(respBody))
	}

	var sb strings.Builder
	for _, cand := range parsed.Candidates {
		for _, p := range cand.Content.Parts {
			sb.WriteString(p.Text)
		}
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		// Возможна блокировка по безопасности (SAFETY) или исчерпание лимита
		// токенов - запрос «успешен», но текст пустой.
		reason := "empty"
		if len(parsed.Candidates) > 0 {
			reason = parsed.Candidates[0].FinishReason
		}
		return "", fmt.Errorf(locales.ErrGeminiEmptyResponse, reason)
	}

	return text, nil
}

// contentPart преобразует вложение в inlineData-блок Generative Language API:
// image/* и application/pdf уходят как есть (Gemini читает PDF нативно).
func contentPart(att Attachment) map[string]any {
	return map[string]any{
		"inlineData": map[string]any{
			"mimeType": att.MimeType,
			"data":     base64.StdEncoding.EncodeToString(att.Data),
		},
	}
}

// isSupportedMime сообщает, умеет ли Gemini анализировать данный MIME-тип.
func isSupportedMime(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/") || mimeType == "application/pdf"
}

// --- Обёртки под нужды сервиса анализа (имена методов совместимы с интерфейсом) ---

// GenerateAnalysisSummary - текстовый анализ по введённому тексту.
func (c *Client) GenerateAnalysisSummary(ctx context.Context, userInput string) (string, error) {
	return c.Generate(ctx,
		locales.SystemPromptMedicalAnalyst(),
		locales.PromptForAnalysisSummary(userInput),
		3000)
}

// GenerateAnalysisJSON - структурированный JSON-анализ по тексту.
func (c *Client) GenerateAnalysisJSON(ctx context.Context, userInput string) (string, error) {
	return c.generateJSON(ctx,
		locales.SystemPromptJSON(),
		locales.PromptForAnalysisJSON(userInput),
		nil,
		defaultMaxOutputTokensJSON)
}

// GenerateAnalysisFromFileWithContext - анализ файла (изображение/PDF) с контекстом.
func (c *Client) GenerateAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	if !isSupportedMime(mimeType) {
		return "", fmt.Errorf(locales.ErrGeminiUnsupportedFile)
	}
	return c.GenerateWithFiles(ctx,
		locales.SystemPromptAnalysis(),
		locales.PromptForAnalysisFile(contextText),
		[]Attachment{{Data: data, MimeType: mimeType}},
		4000)
}

// GenerateAnalysisFromFilesWithContext - анализ НЕСКОЛЬКИХ файлов (изображения/
// PDF) одним мультимодальным запросом. Все вложения помещаются в ОДНО
// сообщение вместе с промптом - как в Bioscan PRO. Gemini видит связь между
// всеми анализами сразу, ничего не теряется.
func (c *Client) GenerateAnalysisFromFilesWithContext(ctx context.Context, data [][]byte, mimeTypes []string, contextText string) (string, error) {
	atts := attachmentsFromPairs(data, mimeTypes)
	if len(atts) == 0 {
		return "", fmt.Errorf(locales.ErrGeminiNoFileData)
	}
	return c.GenerateWithFiles(ctx,
		locales.SystemPromptAnalysisMulti(),
		locales.PromptForAnalysisFiles(contextText),
		atts,
		6000)
}

// GenerateAnalysisFromFilesJSON - структурированный JSON-анализ нескольких
// файлов одним запросом (показатели для дашборда «Мой профиль»).
func (c *Client) GenerateAnalysisFromFilesJSON(ctx context.Context, data [][]byte, mimeTypes []string, contextText string) (string, error) {
	atts := attachmentsFromPairs(data, mimeTypes)
	if len(atts) == 0 {
		return "", fmt.Errorf(locales.ErrGeminiNoFileData)
	}
	return c.generateJSON(ctx,
		locales.SystemPromptJSON(),
		locales.PromptForAnalysisJSON(contextText),
		atts,
		defaultMaxOutputTokensJSON)
}

// GenerateBioscanJSON - JSON-результат Bioscan по фотографиям (все сразу в одном запросе).
func (c *Client) GenerateBioscanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	atts := attachmentsFrom(photosData, mimeType)
	if len(atts) == 0 {
		return "", fmt.Errorf(locales.ErrGeminiNoPhotoData)
	}
	return c.generateJSON(ctx,
		locales.SystemPromptJSON(),
		locales.PromptForBioscan(contextInfo),
		atts,
		defaultMaxOutputTokensJSON)
}

// GenerateBodyScanJSON - JSON премиального отчёта Bioscan PRO (все фото в одном запросе).
func (c *Client) GenerateBodyScanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	atts := attachmentsFrom(photosData, mimeType)
	if len(atts) == 0 {
		return "", fmt.Errorf(locales.ErrGeminiNoPhotoData)
	}
	return c.generateJSON(ctx,
		locales.SystemPromptBodyIntelligence(),
		locales.PromptForBodyScanJSON(contextInfo),
		atts,
		defaultMaxOutputTokensJSON)
}

// GenerateAnalysisFromFileJSON - JSON-анализ из файла (изображение/PDF).
func (c *Client) GenerateAnalysisFromFileJSON(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	if !isSupportedMime(mimeType) {
		return "", fmt.Errorf(locales.ErrGeminiUnsupportedFile)
	}
	return c.generateJSON(ctx,
		locales.SystemPromptJSON(),
		locales.PromptForAnalysisJSON(contextText),
		[]Attachment{{Data: data, MimeType: mimeType}},
		defaultMaxOutputTokensJSON)
}

// GenerateDossierJSON - JSON универсального отчёта-досье здоровья.
func (c *Client) GenerateDossierJSON(ctx context.Context, userInput string) (string, error) {
	return c.generateJSON(ctx,
		locales.SystemPromptDossier(),
		locales.PromptForDossierJSON(userInput),
		nil,
		defaultMaxOutputTokensJSON)
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
// (Gemini умеет только image/* и application/pdf).
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
