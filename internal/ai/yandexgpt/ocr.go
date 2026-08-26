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
)

// recognizeText - распознаёт текст из изображения или PDF через Yandex Vision.
// Для ИЗОБРАЖЕНИЙ используется OCR-эндпоинт recognizeText. Для PDF -
// Yandex Vision API (batchAnalyze с TEXT_DETECTION), так как recognizeText
// PDF НЕ принимает (ошибка "Can't decode image"), а Vision API нативно
// поддерживает application/pdf, включая многостраничные и «сканированные»
// документы.
//
// Возвращает полный извлечённый текст документа. При пустых данных
// возвращает пустую строку без ошибки (файл просто не содержит байт).
//
// Это эквивалент нативного чтения PDF/фото: вместо мультимодального запроса
// мы сначала получаем текст документа, а затем передаём его текстовой модели
// YandexGPT. Для анализа крови результат полностью совпадает (показатели
// читаются из текста бланка).
func (c *Client) recognizeText(ctx context.Context, data []byte, mimeType string) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	if mimeType == "application/pdf" {
		slog.Info("[OCR] маршрутизация PDF на Vision API (batchAnalyze/TEXT_DETECTION)", "data_len", len(data))
		return c.recognizeDocument(ctx, data, mimeType)
	}
	return c.recognizeImage(ctx, data, mimeType)
}

// recognizeImage - OCR изображения через эндпоинт recognizeText.
func (c *Client) recognizeImage(ctx context.Context, data []byte, mimeType string) (string, error) {
	return c.runOCRWithRetry(ctx, mimeType, func(ctx context.Context) (string, int, error) {
		return c.doImageOCROnce(ctx, data, mimeType)
	})
}

// recognizeDocument - OCR документа (PDF) через Yandex Vision API
// batchAnalyze с TEXT_DETECTION. Нативно поддерживает application/pdf
// (включая «сканированные» многостраничные PDF), в отличие от recognizeText.
func (c *Client) recognizeDocument(ctx context.Context, data []byte, mimeType string) (string, error) {
	return c.runOCRWithRetry(ctx, mimeType, func(ctx context.Context) (string, int, error) {
		return c.doVisionOCROnce(ctx, data, mimeType)
	})
}

// runOCRWithRetry - повторяет вызов OCR-сервиса на транспортную ошибку
// (status == 0), 429 и 5xx. Прочие 4xx (401/403/400) не ретраятся.
func (c *Client) runOCRWithRetry(ctx context.Context, mimeType string, attempt func(ctx context.Context) (string, int, error)) (string, error) {
	backoff := defaultRetryBackoff
	var lastErr error
	for i := 0; i < defaultMaxRetries; i++ {
		if err := ctx.Err(); err != nil {
			slog.Error("[OCR] распознавание прервано по контексту (отмена/таймаут)", "mime", mimeType, "attempt", i+1, "err", err)
			return "", err
		}
		text, status, err := attempt(ctx)
		if err == nil {
			slog.Info("[OCR] распознавание успешно", "mime", mimeType, "chars", len(text))
			return text, nil
		}
		lastErr = err
		// Повторяем на транспортную ошибку (status == 0), 429 и 5xx.
		// Прочие 4xx (401/403/400) не ретраятся.
		retryable := status == 0 || status == http.StatusTooManyRequests || status >= 500
		if !retryable {
			slog.Error("[OCR] ошибка распознавания (НЕ повторяется)", "mime", mimeType, "status", status, "err", err)
			return "", err
		}
		slog.Warn("[OCR] временная ошибка распознавания (повтор через backoff)", "mime", mimeType, "attempt", i+1, "status", status, "err", err, "next_backoff", backoff)
		if !sleepWithCtx(ctx, backoff) {
			slog.Error("[OCR] отмена ожидания повтора по контексту", "mime", mimeType, "attempt", i+1)
			return "", ctx.Err()
		}
		backoff *= 2
	}
	if lastErr == nil {
		lastErr = fmt.Errorf(locales.ErrYandexRetryExhausted, fmt.Errorf("no attempts executed"))
	}
	return "", fmt.Errorf(locales.ErrYandexRetryExhausted, lastErr)
}

// doImageOCROnce - один вызов Yandex Vision OCR (recognizeText) для
// ИЗОБРАЖЕНИЯ. Сервер принимает НЕ multipart, а JSON-тело с полем content -
// base64 закодированным содержимым файла (и contentType). multipart-запрос
// сервер отвергает, пытаясь распарсить тело как JSON (возвращает 400
// "invalid character '-' in numeric literal" на границе частей).
// Возвращает (текст, HTTP-статус, ошибку). status == 0 означает
// транспортную ошибку (до получения ответа).
func (c *Client) doImageOCROnce(ctx context.Context, data []byte, mimeType string) (string, int, error) {
	reqBody, err := json.Marshal(map[string]any{
		"content":       base64.StdEncoding.EncodeToString(data),
		"contentType":   ocrContentType(mimeType),
		"languageCodes": []string{"ru", "en"},
		"model":         "page",
	})
	if err != nil {
		return "", 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.ocrEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		slog.Error("[OCR] ошибка сборки HTTP-запроса", "endpoint", c.ocrEndpoint, "err", err)
		return "", 0, err
	}
	req.Header.Set("Authorization", "Api-Key "+c.apiKey)
	req.Header.Set("x-folder-id", c.folderID)
	req.Header.Set("Content-Type", "application/json")

	slog.Info("[OCR] отправка запроса к Yandex Vision OCR (recognizeText)", "content_type", ocrContentType(mimeType), "data_len", len(data))

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Warn("[OCR] транспортная ошибка (повтор)", "endpoint", c.ocrEndpoint, "err", err)
		return "", 0, fmt.Errorf(locales.ErrYandexTransport, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		slog.Error("[OCR] ответ с ошибкой (НЕ повторяется)", "endpoint", c.ocrEndpoint, "status", resp.StatusCode, "duration", time.Since(start), "body", truncateBody(respBody))
		return "", resp.StatusCode, fmt.Errorf(locales.ErrYandexAPIError, resp.StatusCode, string(respBody))
	}

	text, perr := extractOCRText(respBody)
	if perr != nil {
		slog.Error("[OCR] ошибка парсинга ответа", "status", resp.StatusCode, "body", truncateBody(respBody), "err", perr)
		return "", resp.StatusCode, fmt.Errorf(locales.ErrYandexResponseParse, perr, string(respBody))
	}
	slog.Debug("[OCR] запрос успешен", "status", resp.StatusCode, "duration", time.Since(start), "chars", len(strings.TrimSpace(text)))
	return strings.TrimSpace(text), resp.StatusCode, nil
}

// doVisionOCROnce - один вызов Yandex Vision API (batchAnalyze) для ДОКУМЕНТА
// (PDF). Использует ту же аутентификацию (Api-Key + x-folder-id), что и
// остальные Yandex API. Возвращает (текст, HTTP-статус, ошибку).
func (c *Client) doVisionOCROnce(ctx context.Context, data []byte, mimeType string) (string, int, error) {
	reqBody, err := json.Marshal(map[string]any{
		"analyze_specs": []map[string]any{
			{
				"content": base64.StdEncoding.EncodeToString(data),
				"features": []map[string]any{
					{
						"type": "TEXT_DETECTION",
						"text_detection_config": map[string]any{
							"language_codes": []string{"ru", "en"},
							"model":          "page",
						},
					},
				},
				"mime_type": ocrContentType(mimeType),
			},
		},
	})
	if err != nil {
		return "", 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.visionEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		slog.Error("[OCR] ошибка сборки HTTP-запроса", "endpoint", c.visionEndpoint, "err", err)
		return "", 0, err
	}
	req.Header.Set("Authorization", "Api-Key "+c.apiKey)
	req.Header.Set("x-folder-id", c.folderID)
	req.Header.Set("Content-Type", "application/json")

	slog.Info("[OCR] отправка запроса к Yandex Vision API (batchAnalyze/TEXT_DETECTION)", "content_type", ocrContentType(mimeType), "data_len", len(data))

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Warn("[OCR] транспортная ошибка (повтор)", "endpoint", c.visionEndpoint, "err", err)
		return "", 0, fmt.Errorf(locales.ErrYandexTransport, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		slog.Error("[OCR] ответ с ошибкой (НЕ повторяется)", "endpoint", c.visionEndpoint, "status", resp.StatusCode, "duration", time.Since(start), "body", truncateBody(respBody))
		return "", resp.StatusCode, fmt.Errorf(locales.ErrYandexAPIError, resp.StatusCode, string(respBody))
	}

	text, perr := extractVisionOCRText(respBody)
	if perr != nil {
		slog.Error("[OCR] ошибка парсинга ответа Vision API", "status", resp.StatusCode, "body", truncateBody(respBody), "err", perr)
		return "", resp.StatusCode, fmt.Errorf(locales.ErrYandexResponseParse, perr, string(respBody))
	}
	slog.Debug("[OCR] запрос Vision API успешен", "status", resp.StatusCode, "duration", time.Since(start), "chars", len(strings.TrimSpace(text)))
	return strings.TrimSpace(text), resp.StatusCode, nil
}

// extractOCRText извлекает распознанный текст из ответа Yandex Vision OCR
// (recognizeText). Текст лежит в result.textAnnotation.fullText. Для
// многостраничных/документных ответов (если fullText пуст) текст
// восстанавливается постранично (блок -> строка -> слово).
func extractOCRText(respBody []byte) (string, error) {
	var parsed struct {
		Result struct {
			TextAnnotation struct {
				FullText string `json:"fullText"`
				Pages    []struct {
					Blocks []struct {
						Lines []struct {
							Words []struct {
								Text string `json:"text"`
							} `json:"words"`
						} `json:"lines"`
					} `json:"blocks"`
				} `json:"pages"`
			} `json:"textAnnotation"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}

	if strings.TrimSpace(parsed.Result.TextAnnotation.FullText) != "" {
		return parsed.Result.TextAnnotation.FullText, nil
	}

	var sb strings.Builder
	for p, page := range parsed.Result.TextAnnotation.Pages {
		if p > 0 {
			sb.WriteString("\n\n")
		}
		for _, block := range page.Blocks {
			for _, line := range block.Lines {
				for w, word := range line.Words {
					if w > 0 {
						sb.WriteString(" ")
					}
					sb.WriteString(word.Text)
				}
				sb.WriteString("\n")
			}
		}
	}
	return sb.String(), nil
}

// extractVisionOCRText извлекает распознанный текст из ответа Yandex Vision
// API (batchAnalyze/TEXT_DETECTION). Структура: results[].results[].
// text_detection.pages[].blocks[].lines[].words[].text. Слова собираются в
// строки (через пробел), строки - в страницы (через перевод строки),
// страницы разделяются двойным переводом строки.
func extractVisionOCRText(respBody []byte) (string, error) {
	var parsed struct {
		Results []struct {
			Results []struct {
				TextDetection struct {
					Pages []struct {
						Blocks []struct {
							Lines []struct {
								Words []struct {
									Text string `json:"text"`
								} `json:"words"`
							} `json:"lines"`
						} `json:"blocks"`
					} `json:"pages"`
				} `json:"text_detection"`
			} `json:"results"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, spec := range parsed.Results {
		for _, r := range spec.Results {
			for p, page := range r.TextDetection.Pages {
				if p > 0 || sb.Len() > 0 {
					sb.WriteString("\n\n")
				}
				for _, block := range page.Blocks {
					for _, line := range block.Lines {
						for w, word := range line.Words {
							if w > 0 {
								sb.WriteString(" ")
							}
							sb.WriteString(word.Text)
						}
						sb.WriteString("\n")
					}
				}
			}
		}
	}
	return sb.String(), nil
}

// ocrContentType возвращает contentType для запроса OCR по MIME-типу файла.
// Yandex Vision OCR/Vision принимают image/* и application/pdf; для остальных
// используем универсальный octet-stream (сервис сам определит тип по
// содержимому).
func ocrContentType(mimeType string) string {
	switch {
	case strings.HasPrefix(mimeType, "image/"), mimeType == "application/pdf":
		return mimeType
	default:
		return "application/octet-stream"
	}
}
