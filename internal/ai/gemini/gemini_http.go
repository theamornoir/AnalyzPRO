package gemini

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	httpclient "github.com/theamornoir/analyzpro/internal/ai/httpclient"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// rawResponse - сырой HTTP-ответ от Gemini.
type rawResponse struct {
	status int
	body   []byte
}

// doRequest - выполняет HTTP-запрос к Gemini API.
//
// Устойчив к «списанию» моделей Google: сначала пробует сконфигурированную
// модель (из .env GOOGLE_AI_MODEL), а при ошибке вида «модель не найдена /
// недоступна» (404, либо 400 с сообщением о модели) автоматически
// переключается на следующую из цепочки фоллбэков (candidateModels).
// Реальный HTTP-статус и тело ошибки пропагируются наверх (без маскировки
// 404 в 503), чтобы в логах было видно настоящую причину.
func (c *GeminiClient) doRequest(ctx context.Context, body []byte) (*rawResponse, error) {
	models := c.candidateModels()
	lastResp := &rawResponse{status: 503, body: nil}

	for i, m := range models {
		if i > 0 {
			log.Printf("🔄 Gemini: модель %q недоступна - переключаюсь на фоллбэк %q", models[i-1], m)
		}

		url := fmt.Sprintf(locales.GeminiAPIURL, m, c.apiKey)
		log.Printf(locales.LogGeminiRequestModel, m)
		log.Printf(locales.LogGeminiSendingRequest)
		startTime := time.Now()

		respBody, err := httpclient.FetchWithRetry(ctx, url, bytes.NewReader(body), 3)
		if err != nil {
			var he *httpclient.HTTPError
			if errors.As(err, &he) {
				lastResp = &rawResponse{status: he.StatusCode, body: []byte(he.Message)}
				// Ошибка вида «модель не найдена/недоступна» - пробуем
				// следующую модель из цепочки фоллбэков.
				if isModelNotFoundError(err) && i < len(models)-1 {
					log.Printf("⚠️ Gemini: %q - модель не найдена или недоступна, пробую следующую", m)
					continue
				}
				log.Printf(locales.LogGeminiHTTPFailed, err)
				return lastResp, nil
			}
			log.Printf(locales.LogGeminiHTTPFailed, err)
			return &rawResponse{status: 503, body: nil}, nil
		}

		elapsed := time.Since(startTime)
		log.Printf(locales.LogGeminiRequestDuration, elapsed)
		if i > 0 {
			log.Printf("✅ Gemini: фоллбэк-модель %q сработала (исходная %q недоступна)", m, models[0])
		}
		return &rawResponse{status: 200, body: respBody}, nil
	}

	return lastResp, nil
}

// candidateModels возвращает список моделей для попытки запроса: сначала
// сконфигурированная (из .env GOOGLE_AI_MODEL / аргумента NewGeminiClient),
// затем цепочка фоллбэков. Google периодически «списывает» неверсионированные
// алиасы (например, models/gemini-2.5-flash становится недоступен для новых
// ключей с 404 "no longer available to new users"), поэтому при такой ошибке
// мы автоматически переключаемся на рабочую модель, не требуя правки .env.
func (c *GeminiClient) candidateModels() []string {
	base := strings.TrimPrefix(c.model, "models/")
	fallbacks := []string{
		"gemini-2.5-flash-latest",
		"gemini-2.5-flash-002",
		"gemini-2.5-flash-001",
		"gemini-2.0-flash",
		"gemini-2.5-pro-latest",
		"gemini-2.5-pro",
	}

	seen := make(map[string]bool)
	out := make([]string, 0, len(fallbacks)+1)
	add := func(m string) {
		m = strings.TrimPrefix(m, "models/")
		if m == "" || seen[m] {
			return
		}
		seen[m] = true
		out = append(out, m)
	}

	add(base)
	for _, f := range fallbacks {
		add(f)
	}
	return out
}

// isModelNotFoundError проверяет, что ошибка запроса означает
// «модель не найдена / недоступна» (404, либо 400 с сообщением о модели).
// В таком случае имеет смысл попробовать другую модель из цепочки фоллбэков.
func isModelNotFoundError(err error) bool {
	var he *httpclient.HTTPError
	if !errors.As(err, &he) {
		return false
	}
	if he.StatusCode == http.StatusNotFound {
		return true
	}
	msg := strings.ToLower(he.Message)
	return he.StatusCode == http.StatusBadRequest &&
		(strings.Contains(msg, "not found") ||
			strings.Contains(msg, "no longer available") ||
			strings.Contains(msg, "is not available") ||
			(strings.Contains(msg, "model") && strings.Contains(msg, "not available")))
}
