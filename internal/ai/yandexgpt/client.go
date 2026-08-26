package yandexgpt

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/theamornoir/analyzpro/internal/locales"
	"golang.org/x/sync/semaphore"
)

// Дефолтные значения конфигурации клиента. Используются config.Load
// как значения по умолчанию для env-переменных (YANDEX_MODEL и т.п.).
const (
	// DefaultModel - модель YandexGPT по умолчанию (облачный LLM Yandex Cloud).
	DefaultModel = "yandexgpt"
	// DefaultMaxConcurrency - макс. число одновременных запросов к YandexGPT.
	// Низкий предел (3-4) защищает от 429 при невысоком RPM тарифа.
	DefaultMaxConcurrency = 4
	// DefaultTimeout - таймаут одного AI-запроса (PDF/изображения долгие).
	DefaultTimeout = 120 * time.Second

	// yandexCompletionEndpoint - REST YandexGPT Foundation Models (completion).
	yandexCompletionEndpoint = "https://llm.api.cloud.yandex.net/foundationModels/v1/completion"
	// yandexOCREndpoint - REST Yandex Vision OCR для ИЗОБРАЖЕНИЙ (метод
	// recognizeText). Принимает только картинки (image/png, image/jpeg и
	// т.п.) и OCR-текст из них. PDF НЕ принимает (ошибка "Can't decode
	// image"), поэтому PDF маршрутизируется на yandexVisionEndpoint.
	// Использует те же API-ключ и каталог, что и YandexGPT, поэтому не
	// требует отдельной конфигурации. Тело запроса - JSON с полем
	// content (base64 файла), а не multipart (сервер отвергает
	// multipart, пытаясь распарсить тело как JSON).
	yandexOCREndpoint = "https://ocr.api.cloud.yandex.net/ocr/v1/recognizeText"
	// yandexVisionEndpoint - REST Yandex Vision API (batchAnalyze) для
	// ДОКУМЕНТОВ (в т.ч. PDF). Именно этот метод нативно принимает
	// application/pdf (включая «сканированные» многостраничные PDF) через
	// фичу TEXT_DETECTION, в отличие от recognizeText. Использует те же
	// API-ключ и каталог (x-folder-id), что и остальные Yandex API.
	yandexVisionEndpoint = "https://vision.api.cloud.yandex.net/vision/v1/batchAnalyze"
)

// Настройки надёжности пайплайна. Константы транспорта/генерации (не env).
const (
	defaultMaxRetries           = 3
	defaultRetryBackoff         = 400 * time.Millisecond
	defaultJSONTemperature      = 0.2
	defaultTextTemperature      = 0.4
	defaultMaxOutputTokensText  = 4000
	defaultMaxOutputTokensFile  = 6000
	defaultMaxOutputTokensMulti = 8000
	defaultMaxOutputTokensJSON  = 8000
	ocrTextHeader               = "\n\nИзвлечённый текст документа (через распознавание Yandex Vision OCR):\n"
	// maxVisionImageBytes - предельный размер фото, передаваемого модели
	// напрямую (мультимодальный запрос). Yandex ограничивает размер
	// изображения в запросе; берём с запасом под 6 МБ лимит. Более крупные
	// фото в vision-путь не уходят (используется текстовый fallback).
	maxVisionImageBytes = 5 * 1024 * 1024
	// visionOCRMinChars - минимальная длина OCR-текста, чтобы он считался
	// содержательным (скриншот весов/приложения), а не мусором с «голого»
	// фото фигуры (~несколько символов). Такой текст подмешивается в
	// vision-промпт как точные цифры; мусор отбрасывается.
	visionOCRMinChars = 15
)

// Config - параметры YandexGPT-клиента. Все значения передаются явно
// (из config.Config, который читает env), поэтому методы клиента не делают
// os.Getenv и не содержат хардкода - они чистые.
type Config struct {
	// APIKey - ключ API Yandex Cloud (YANDEX_API_KEY). Пустой - вызовы
	// вернут ошибку, бот стартует.
	APIKey string
	// FolderID - идентификатор каталога Yandex Cloud (YANDEX_FOLDER_ID).
	// Используется в modelUri и при аутентификации OCR.
	FolderID string
	// Model - имя модели (напр. yandexgpt, yandexgpt-lite). Читается из
	// YANDEX_MODEL; по умолчанию DefaultModel.
	Model string
	// MaxConcurrency - лимит одновременных запросов.
	MaxConcurrency int
	// Timeout - таймаут одного запроса.
	Timeout time.Duration
}

// Client - клиент YandexGPT (LLM Yandex Cloud). Реализует тот же
// интерфейс AIClient. Для документов (анализы крови и т.п.) текстовая
// модель получает текст, предварительно распознанный через Yandex Vision
// OCR (тот же API-ключ и каталог). Для фото тела (базовый Bioscan) клиент
// также умеет строить МУЛЬТИМОДАЛЬНЫЕ запросы: само изображение кодируется
// в base64 и передаётся модели напрямую (поле messages[].images), чтобы она
// реально «видела» фигуру, а не только текст OCR.
type Client struct {
	apiKey             string
	folderID           string
	modelURI           string
	completionEndpoint string
	ocrEndpoint        string
	visionEndpoint     string
	httpClient         *http.Client
	sem                *semaphore.Weighted
}

// NewClient создаёт YandexGPT-клиент из явной Config (без чтения env и без
// логирования внутри - чистый конструктор). Пустой API-ключ не является
// ошибкой: вызовы вернут ErrYandexKeyNotSet, бот продолжит работу.
func NewClient(cfg Config) (*Client, error) {
	conc := cfg.MaxConcurrency
	if conc <= 0 {
		conc = DefaultMaxConcurrency
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}
	return &Client{
		apiKey:             cfg.APIKey,
		folderID:           cfg.FolderID,
		modelURI:           fmt.Sprintf("gpt://%s/%s", cfg.FolderID, model),
		completionEndpoint: yandexCompletionEndpoint,
		ocrEndpoint:        yandexOCREndpoint,
		visionEndpoint:     yandexVisionEndpoint,
		httpClient:         &http.Client{Timeout: timeout},
		sem:                semaphore.NewWeighted(int64(conc)),
	}, nil
}

// generateText - ядро генерации текста через YandexGPT completion API.
// Ограничивает конкурентность семафором, собирает запрос (modelUri,
// completionOptions, messages) и повторяет его при 429/5xx/сетевой ошибке.
// truncateBody обрезает тело HTTP-ответа для логов до безопасной длины,
// чтобы не засорять логи большими JSON-ответами модели и не раскрывать
// лишние данные. На ошибках Yandex тело - это короткое сообщение об ошибке,
// его полезно видеть целиком для диагностики (напр. "folder ID does not
// match"), поэтому лимит достаточно велик.
func truncateBody(b []byte) string {
	const limit = 800
	s := string(b)
	if len(s) > limit {
		return s[:limit] + fmt.Sprintf("... (обрезано, всего %d байт)", len(s))
	}
	return s
}

func (c *Client) generateText(ctx context.Context, systemPrompt, userPrompt string, maxTokens int, temperature float64) (string, error) {
	if c.apiKey == "" {
		slog.Error("[YANDEX] API-ключ Yandex не задан - генерация невозможна (проверьте YANDEX_API_KEY в .env)")
		return "", fmt.Errorf(locales.ErrYandexKeyNotSet)
	}

	if err := c.sem.Acquire(ctx, 1); err != nil {
		slog.Error("[YANDEX] не удалось захватить семафор конкурентности", "err", err)
		return "", err
	}
	defer c.sem.Release(1)

	slog.Debug("[YANDEX] генерация текста", "model_uri", c.modelURI, "max_tokens", maxTokens, "temperature", temperature)

	body := map[string]any{
		"modelUri": c.modelURI,
		"completionOptions": map[string]any{
			"stream":      false,
			"temperature": temperature,
			"maxTokens":   maxTokens,
		},
		"messages": []map[string]string{
			{"role": "system", "text": systemPrompt},
			{"role": "user", "text": userPrompt},
		},
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	respBody, err := c.doRequestWithRetry(ctx, c.completionEndpoint, jsonBody)
	if err != nil {
		return "", err
	}
	text, perr := parseCompletion(respBody)
	if perr != nil {
		return "", perr
	}
	slog.Info("[YANDEX] ответ обработан (YandexGPT)", "result_chars", len(text))
	return text, nil
}

// generateMultimodal - генерация текста через МУЛЬТИМОДАЛЬНЫЙ (vision) запрос
// YandexGPT: в последнее сообщение пользователя помимо текста помещаются
// изображения (base64, БЕЗ префикса data URI). Это позволяет модели реально
// «видеть» фото (напр. фигуру пользователя в Bioscan) вместо того, чтобы
// опираться только на текст, извлечённый через OCR. Остальная логика
// (семафор конкурентности, retry на 429/5xx, парсинг ответа) - как в
// generateText. Если модель не поддерживает изображения - запрос вернёт
// ошибку, и вызывающий может откатиться на текстовый путь.
func (c *Client) generateMultimodal(ctx context.Context, systemPrompt, userPrompt string, images []string, maxTokens int, temperature float64) (string, error) {
	if c.apiKey == "" {
		slog.Error("[YANDEX] API-ключ Yandex не задан - генерация невозможна (проверьте YANDEX_API_KEY в .env)")
		return "", fmt.Errorf(locales.ErrYandexKeyNotSet)
	}

	if err := c.sem.Acquire(ctx, 1); err != nil {
		slog.Error("[YANDEX] не удалось захватить семафор конкурентности", "err", err)
		return "", err
	}
	defer c.sem.Release(1)

	slog.Debug("[YANDEX] генерация мультимодального текста", "model_uri", c.modelURI, "images", len(images), "max_tokens", maxTokens, "temperature", temperature)

	userMsg := map[string]any{"role": "user", "text": userPrompt}
	if len(images) > 0 {
		userMsg["images"] = images
	}
	body := map[string]any{
		"modelUri": c.modelURI,
		"completionOptions": map[string]any{
			"stream":      false,
			"temperature": temperature,
			"maxTokens":   maxTokens,
		},
		"messages": []map[string]any{
			{"role": "system", "text": systemPrompt},
			userMsg,
		},
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	respBody, err := c.doRequestWithRetry(ctx, c.completionEndpoint, jsonBody)
	if err != nil {
		return "", err
	}
	text, perr := parseCompletion(respBody)
	if perr != nil {
		return "", perr
	}
	slog.Info("[YANDEX] ответ обработан (YandexGPT vision)", "result_chars", len(text))
	return text, nil
}

// visionImages кодирует фото в base64 для мультимодального запроса,
// пропуская пустые и слишком большие (Yandex ограничивает размер изображения).
func visionImages(photosData [][]byte) []string {
	var imgs []string
	for _, d := range photosData {
		if len(d) == 0 || len(d) > maxVisionImageBytes {
			continue
		}
		imgs = append(imgs, base64.StdEncoding.EncodeToString(d))
	}
	return imgs
}

// parseCompletion - декодирует ответ YandexGPT (result.alternatives[].message.text).
// Пустой текст трактуется как ошибка (модель «успешно» вернула пустоту).
func parseCompletion(respBody []byte) (string, error) {
	var parsed struct {
		Result struct {
			Alternatives []struct {
				Message struct {
					Text string `json:"text"`
				} `json:"message"`
			} `json:"alternatives"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf(locales.ErrYandexResponseParse, err, string(respBody))
	}

	var sb strings.Builder
	for _, alt := range parsed.Result.Alternatives {
		sb.WriteString(alt.Message.Text)
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		return "", fmt.Errorf(locales.ErrYandexEmptyResponse)
	}
	return text, nil
}

// doRequestWithRetry - HTTP POST с повторами на транспортные сбои и
// временные ошибки сервера (429 rate-limit, 5xx). Неретраяемые ошибки
// (2xx и 4xx кроме 429) возвращаются сразу; 400 (bad request) повторять
// бессмысленно. Уважает отмену ctx (прерывает ожидание backoff).
// Логирует каждую попытку, статус и тело ответа Yandex для точной диагностики
// (какой эндпоинт, какой HTTP-статус, что вернул Yandex).
func (c *Client) doRequestWithRetry(ctx context.Context, endpoint string, body []byte) ([]byte, error) {
	backoff := defaultRetryBackoff
	var lastErr error

	for attempt := 0; attempt < defaultMaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			slog.Error("[YANDEX] запрос прерван по контексту (отмена/таймаут)", "endpoint", endpoint, "attempt", attempt+1, "err", err)
			return nil, err
		}

		start := time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			slog.Error("[YANDEX] ошибка сборки HTTP-запроса", "endpoint", endpoint, "err", err)
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Api-Key "+c.apiKey)
		req.Header.Set("x-folder-id", c.folderID)

		slog.Info("[YANDEX] отправка запроса к Yandex", "endpoint", endpoint, "attempt", attempt+1)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf(locales.ErrYandexTransport, err)
			slog.Warn("[YANDEX] транспортная ошибка (повтор через backoff)", "endpoint", endpoint, "attempt", attempt+1, "err", err, "next_backoff", backoff)
			if !sleepWithCtx(ctx, backoff) {
				slog.Error("[YANDEX] отмена ожидания повтора по контексту", "endpoint", endpoint, "attempt", attempt+1)
				return nil, ctx.Err()
			}
			backoff *= 2
			continue
		}

		respBody, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			lastErr = fmt.Errorf(locales.ErrYandexTransport, rerr)
			slog.Warn("[YANDEX] ошибка чтения тела ответа (повтор через backoff)", "endpoint", endpoint, "attempt", attempt+1, "status", resp.StatusCode, "err", rerr, "next_backoff", backoff)
			if !sleepWithCtx(ctx, backoff) {
				return nil, ctx.Err()
			}
			backoff *= 2
			continue
		}

		// 429 (rate limit) и 5xx (временные сбои сервера) - повторяем.
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf(locales.ErrYandexAPIError, resp.StatusCode, string(respBody))
			slog.Warn("[YANDEX] временная ошибка сервера (повтор через backoff)", "endpoint", endpoint, "attempt", attempt+1, "status", resp.StatusCode, "duration", time.Since(start), "body", truncateBody(respBody), "next_backoff", backoff)
			if !sleepWithCtx(ctx, backoff) {
				return nil, ctx.Err()
			}
			backoff *= 2
			continue
		}

		// Неретраяемая ошибка (в т.ч. 400/401/403/404) - возвращаем сразу.
		if resp.StatusCode >= 400 {
			slog.Error("[YANDEX] ошибка API (НЕ повторяется)", "endpoint", endpoint, "status", resp.StatusCode, "duration", time.Since(start), "body", truncateBody(respBody))
			return nil, fmt.Errorf(locales.ErrYandexAPIError, resp.StatusCode, string(respBody))
		}

		slog.Info("[YANDEX] ответ получен от Yandex", "endpoint", endpoint, "status", resp.StatusCode, "duration_ms", time.Since(start).Milliseconds(), "bytes", len(respBody))
		return respBody, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf(locales.ErrYandexRetryExhausted, fmt.Errorf("no attempts executed"))
	}
	slog.Error("[YANDEX] исчерпаны все попытки запроса", "endpoint", endpoint, "attempts", defaultMaxRetries, "last_err", lastErr)
	return nil, fmt.Errorf(locales.ErrYandexRetryExhausted, lastErr)
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

// isSupportedMime сообщает, умеет ли Yandex Vision OCR распознавать данный
// MIME-тип (изображения и PDF).
func isSupportedMime(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/") || mimeType == "application/pdf"
}

// allEmpty сообщает, что все срезы данных пусты.
func allEmpty(data [][]byte) bool {
	for _, d := range data {
		if len(d) > 0 {
			return false
		}
	}
	return true
}

// withExtracted добавляет распознанный текст документа к контексту
// пользователя, если он не пуст.
func withExtracted(contextText, extracted string) string {
	if strings.TrimSpace(extracted) == "" {
		return contextText
	}
	return contextText + ocrTextHeader + extracted
}

// recognizeAll распознаёт все поддерживаемые файлы и объединяет текст.
func (c *Client) recognizeAll(ctx context.Context, data [][]byte, mimeTypes []string) (string, error) {
	var parts []string
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
		ext, err := c.recognizeText(ctx, d, mt)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(ext) != "" {
			parts = append(parts, ext)
		}
	}
	return strings.Join(parts, "\n\n--- Следующий документ ---\n\n"), nil
}

// --- Обёртки под нужды сервиса анализа (имена методов совместимы с интерфейсом) ---

// GenerateAnalysisSummary - текстовый анализ по введённому тексту.
func (c *Client) GenerateAnalysisSummary(ctx context.Context, userInput string) (string, error) {
	slog.Info("[YANDEX] запрос: анализ-сводка (текст)", "input_chars", len(userInput))
	return c.generateText(ctx,
		locales.SystemPromptMedicalAnalyst(),
		locales.PromptForAnalysisSummary(userInput),
		defaultMaxOutputTokensText, defaultTextTemperature)
}

// GenerateAnalysisJSON - структурированный JSON-анализ по тексту.
func (c *Client) GenerateAnalysisJSON(ctx context.Context, userInput string) (string, error) {
	slog.Info("[YANDEX] запрос: структурированный JSON-анализ (текст)", "input_chars", len(userInput))
	return c.generateText(ctx,
		locales.SystemPromptJSON(),
		locales.PromptForAnalysisJSON(userInput),
		defaultMaxOutputTokensJSON, defaultJSONTemperature)
}

// GenerateAnalysisFromFileWithContext - анализ файла (изображение/PDF) с
// контекстом. Файл сначала распознаётся через Yandex Vision OCR, текст
// передаётся модели вместе с контекстом пациента.
func (c *Client) GenerateAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	slog.Info("[YANDEX] запрос: анализ файла (OCR фото/PDF + текст)", "data_len", len(data), "mime", mimeType)
	if !isSupportedMime(mimeType) {
		return "", fmt.Errorf(locales.ErrYandexUnsupportedFile)
	}
	extracted, err := c.recognizeText(ctx, data, mimeType)
	if err != nil {
		return "", err
	}
	userPrompt := locales.PromptForAnalysisFile(withExtracted(contextText, extracted))
	return c.generateText(ctx, locales.SystemPromptAnalysis(), userPrompt, defaultMaxOutputTokensFile, defaultTextTemperature)
}

// GenerateAnalysisFromFileVision - анализ ПРИЛОЖЕННОГО ФОТО (травма/
// проблемная зона и т.п.) через мультимодальный (vision) запрос: само
// изображение кодируется в base64 и передаётся модели напрямую
// (messages[].images), чтобы она реально «видела» снимок, а не только
// OCR-текст (с «живого» фото тела OCR извлекает ~0 символов). Если модель не
// поддерживает изображения (или запрос упал) - откатываемся на OCR-путь.
func (c *Client) GenerateAnalysisFromFileVision(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	slog.Info("[YANDEX] запрос: анализ фото (vision) + текст", "data_len", len(data), "mime", mimeType)
	if !isSupportedMime(mimeType) {
		return "", fmt.Errorf(locales.ErrYandexUnsupportedFile)
	}

	// VISION-ПУТЬ: отправляем САМО фото модели (мультимодальный запрос),
	// чтобы она реально увидела снимок.
	if imgs := visionImages([][]byte{data}); len(imgs) > 0 {
		prompt := locales.PromptForAnalysisVision(contextText)
		slog.Info("[YANDEX] запрос: консультация (vision-фото)", "photos", len(imgs))
		res, err := c.generateMultimodal(ctx, locales.SystemPromptAnalysis(), prompt, imgs, defaultMaxOutputTokensFile, defaultTextTemperature)
		if err == nil {
			slog.Info("[YANDEX] ответ обработан (YandexGPT vision, консультация)", "result_chars", len(res))
			return res, nil
		}
		slog.Warn("[YANDEX] vision-путь консультации не сработал - fallback на OCR+текст", "err", err)
	}

	// ТЕКСТОВЫЙ FALLBACK: OCR-текст фото + контекст (как раньше). С «живого»
	// фото тела OCR даст ~0 символов, но для документов/скриншотов сработает.
	extracted, err := c.recognizeText(ctx, data, mimeType)
	if err != nil {
		return "", err
	}
	userPrompt := locales.PromptForAnalysisFile(withExtracted(contextText, extracted))
	return c.generateText(ctx, locales.SystemPromptAnalysis(), userPrompt, defaultMaxOutputTokensFile, defaultTextTemperature)
}

// GenerateAnalysisFromFilesWithContext - анализ НЕСКОЛЬКИХ файлов (изображения/
// PDF) единым запросом. Все файлы распознаются через OCR, тексты
// объединяются и передаются модели вместе с контекстом.
func (c *Client) GenerateAnalysisFromFilesWithContext(ctx context.Context, data [][]byte, mimeTypes []string, contextText string) (string, error) {
	slog.Info("[YANDEX] запрос: анализ нескольких файлов (OCR + текст)", "files", len(data))
	if allEmpty(data) {
		return "", fmt.Errorf(locales.ErrYandexNoFileData)
	}
	extracted, err := c.recognizeAll(ctx, data, mimeTypes)
	if err != nil {
		return "", err
	}
	userPrompt := locales.PromptForAnalysisFiles(withExtracted(contextText, extracted))
	return c.generateText(ctx, locales.SystemPromptAnalysisMulti(), userPrompt, defaultMaxOutputTokensMulti, defaultTextTemperature)
}

// GenerateAnalysisFromFilesJSON - структурированный JSON-анализ нескольких
// файлов единым запросом (показатели для дашборда «Мой профиль»).
func (c *Client) GenerateAnalysisFromFilesJSON(ctx context.Context, data [][]byte, mimeTypes []string, contextText string) (string, error) {
	slog.Info("[YANDEX] запрос: структурированный JSON-анализ нескольких файлов", "files", len(data))
	if allEmpty(data) {
		return "", fmt.Errorf(locales.ErrYandexNoFileData)
	}
	extracted, err := c.recognizeAll(ctx, data, mimeTypes)
	if err != nil {
		return "", err
	}
	userPrompt := locales.PromptForAnalysisJSON(withExtracted(contextText, extracted))
	return c.generateText(ctx, locales.SystemPromptJSON(), userPrompt, defaultMaxOutputTokensJSON, defaultJSONTemperature)
}

// GenerateBioscanJSON - JSON-результат базового Bioscan по фото. Строит
// персональный отчёт на основе РЕАЛЬНОГО анализа присланного фото моделью:
//
//  1. VISION-ПУТЬ (основной): фото кодируется в base64 и отправляется модели
//     напрямую (мультимодальный запрос, поле messages[].images) - модель
//     реально «видит» фигуру, оценивает композицию, осанку, пропорции, зоны.
//     К промпту также подмешивается OCR-текст со скриншотов умных весов/
//     приложений (точные цифры) и данные мини-опросника.
//  2. ТЕКСТОВЫЙ FALLBACK: если модель не поддерживает изображения (или запрос
//     упал) - откатываемся на прежний путь: OCR-текст фото + опросник.
//
// Если нет ни фото, ни опросника - возвращаем ошибку (отчёт не построить).
func (c *Client) GenerateBioscanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	if allEmpty(photosData) && strings.TrimSpace(contextInfo) == "" {
		return "", fmt.Errorf(locales.ErrYandexNoPhotoData)
	}

	// OCR-текст - для скриншотов умных весов/фитнес-приложений (точные
	// цифры) и как запасной источник, если vision-путь недоступен.
	var ocrParts []string
	for _, d := range photosData {
		if len(d) == 0 {
			continue
		}
		ext, err := c.recognizeText(ctx, d, mimeType)
		if err != nil {
			slog.Error("[OCR] не удалось распознать фото для Bioscan", "err", err)
			return "", err
		}
		if strings.TrimSpace(ext) != "" {
			ocrParts = append(ocrParts, ext)
		}
	}
	ocrText := strings.Join(ocrParts, "\n\n--- Следующее фото ---\n\n")
	slog.Info("[BIOSCAN] OCR-текст из фото распознан", "chars", len(ocrText))

	// VISION-ПУТЬ: отправляем САМО фото модели (мультимодальный запрос),
	// чтобы она реально увидела фигуру пользователя, а не только текст OCR.
	if imgs := visionImages(photosData); len(imgs) > 0 {
		prompt := locales.PromptForBioscanBasicVision(contextInfo, ocrText)
		slog.Info("[YANDEX] запрос: базовый Bioscan (vision-фото + текст)", "photos", len(imgs))
		res, err := c.generateMultimodal(ctx, locales.SystemPromptBioscanVision(), prompt, imgs, defaultMaxOutputTokensJSON, defaultJSONTemperature)
		if err == nil {
			slog.Info("[YANDEX] ответ обработан (YandexGPT vision)", "result_chars", len(res))
			return res, nil
		}
		slog.Warn("[YANDEX] vision-путь базового Bioscan не сработал - fallback на OCR+текст", "err", err)
	}

	// ТЕКСТОВЫЙ FALLBACK: только OCR-текст + опросник (как раньше).
	if len(strings.TrimSpace(ocrText)) < visionOCRMinChars {
		ocrText = ""
	}
	userData := withExtracted(contextInfo, ocrText)
	if strings.TrimSpace(userData) == "" {
		userData = "Данные не предоставлены: на фото нет читаемого текста и опросник не заполнен."
		slog.Info("[BIOSCAN] данные для отчёта отсутствуют - модель вернёт общие рекомендации")
	}
	slog.Info("[YANDEX] запрос: базовый Bioscan (OCR фото + текст)", "photos", len(photosData), "mime", mimeType)
	return c.generateText(ctx, locales.SystemPromptJSON(), locales.PromptForBioscanBasic(userData), defaultMaxOutputTokensJSON, defaultJSONTemperature)
}

// GenerateBodyScanJSON - JSON премиального отчёта Bioscan PRO (Body
// Intelligence). Главное отличие от базового Bioscan - здесь 4 фото (4
// ракурса), и отчёт строится по НИМ напрямую (vision), а не только по
// опроснику:
//
//  1. VISION-ПУТЬ (основной): все 4 фото кодируются в base64 и отправляются
//     модели напрямую (мультимодальный запрос, messages[].images) - модель
//     реально «видит» фигуру пользователя (4 ракурса) и анализирует её
//     (композиция, осанка, пропорции, симметрия, развитость зон). К промпту
//     также подмешивается OCR-текст со скриншотов умных весов/фитнес-
//     приложений (точные цифры) и данные опросника (contextInfo).
//  2. ТЕКСТОВЫЙ FALLBACK: если модель не поддерживает изображения (или запрос
//     упал) - откатываемся на прежний путь: опросник (+ OCR-текст, если он
//     есть). Фото «голой» фигуры OCR не читает, поэтому без vision отчёт
//     строится по замерам из опросника.
//
// Если нет ни фото, ни опросника - возвращаем ошибку (отчёт не построить).
func (c *Client) GenerateBodyScanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	if allEmpty(photosData) {
		return "", fmt.Errorf(locales.ErrYandexNoPhotoData)
	}

	// OCR-текст - для скриншотов умных весов/фитнес-приложений (точные
	// цифры) и как запасной источник, если vision-путь недоступен.
	var ocrParts []string
	for _, d := range photosData {
		if len(d) == 0 {
			continue
		}
		ext, err := c.recognizeText(ctx, d, mimeType)
		if err != nil {
			slog.Error("[OCR] не удалось распознать фото для Bioscan PRO", "err", err)
			return "", err
		}
		if strings.TrimSpace(ext) != "" {
			ocrParts = append(ocrParts, ext)
		}
	}
	ocrText := strings.Join(ocrParts, "\n\n--- Следующее фото ---\n\n")
	slog.Info("[BIOSCAN] OCR-текст из фото распознан (PRO)", "chars", len(ocrText))

	// VISION-ПУТЬ: отправляем САМИ фото модели (мультимодальный запрос),
	// чтобы она реально увидела фигуру пользователя (4 ракурса), а не
	// только текст OCR.
	if imgs := visionImages(photosData); len(imgs) > 0 {
		prompt := locales.PromptForBodyScanVision(contextInfo, ocrText)
		slog.Info("[YANDEX] запрос: Bioscan PRO (vision-фото + текст)", "photos", len(imgs))
		res, err := c.generateMultimodal(ctx, locales.SystemPromptBodyIntelligenceVision(), prompt, imgs, defaultMaxOutputTokensJSON, defaultJSONTemperature)
		if err == nil {
			slog.Info("[YANDEX] ответ обработан (YandexGPT vision, PRO)", "result_chars", len(res))
			return res, nil
		}
		slog.Warn("[YANDEX] vision-путь Bioscan PRO не сработал - fallback на текст по опроснику", "err", err)
	}

	// ТЕКСТОВЫЙ FALLBACK: опросник (+ OCR-текст со скриншотов, если есть) -
	// как раньше. «Голое» фото фигуры OCR не читает, поэтому без vision
	// отчёт строится по замерам из опросника.
	userData := contextInfo
	if len(strings.TrimSpace(ocrText)) >= visionOCRMinChars {
		userData = withExtracted(contextInfo, ocrText)
	}
	if strings.TrimSpace(userData) == "" {
		userData = "Данные не предоставлены: опросник не заполнен."
		slog.Info("[BIOSCAN] PRO: данные для отчёта отсутствуют - модель вернёт общие рекомендации")
	}
	slog.Info("[YANDEX] запрос: Bioscan PRO (текст по замерам/опроснику)", "photos", len(photosData), "mime", mimeType)
	return c.generateText(ctx, locales.SystemPromptBodyIntelligence(), locales.PromptForBodyScanJSON(userData), defaultMaxOutputTokensJSON, defaultJSONTemperature)
}

// GenerateAnalysisFromFileJSON - JSON-анализ из файла (изображение/PDF).
func (c *Client) GenerateAnalysisFromFileJSON(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	slog.Info("[YANDEX] запрос: структурированный JSON-анализ файла (OCR + текст)", "data_len", len(data), "mime", mimeType)
	if !isSupportedMime(mimeType) {
		return "", fmt.Errorf(locales.ErrYandexUnsupportedFile)
	}
	extracted, err := c.recognizeText(ctx, data, mimeType)
	if err != nil {
		return "", err
	}
	userPrompt := locales.PromptForAnalysisJSON(withExtracted(contextText, extracted))
	return c.generateText(ctx, locales.SystemPromptJSON(), userPrompt, defaultMaxOutputTokensJSON, defaultJSONTemperature)
}

// GenerateDossierJSON - JSON универсального отчёта-досье здоровья.
func (c *Client) GenerateDossierJSON(ctx context.Context, userInput string) (string, error) {
	slog.Info("[YANDEX] запрос: досье здоровья (текст)", "input_chars", len(userInput))
	return c.generateText(ctx, locales.SystemPromptDossier(), locales.PromptForDossierJSON(userInput), defaultMaxOutputTokensJSON, defaultJSONTemperature)
}

// GenerateHealthAssessmentJSON - JSON отчёта «Общая оценка здоровья».
// Строится ИСКЛЮЧИТЕЛЬНО по тексту опросника об образе жизни
// пользователя (без медицинских файлов). Зеркален GenerateDossierJSON,
// но использует свой промпт (SystemPromptHealthAssessment /
// PromptForHealthAssessmentJSON) и схему models.HealthAssessment.
func (c *Client) GenerateHealthAssessmentJSON(ctx context.Context, userInput string) (string, error) {
	slog.Info("[YANDEX] запрос: общая оценка здоровья (текст опросника)", "input_chars", len(userInput))
	return c.generateText(ctx, locales.SystemPromptHealthAssessment(), locales.PromptForHealthAssessmentJSON(userInput), defaultMaxOutputTokensJSON, defaultJSONTemperature)
}
