package yandexgpt

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// okResponse - корректный ответ YandexGPT completion (result.alternatives[].message.text).
func okResponse(t *testing.T) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"result": map[string]any{
			"alternatives": []any{
				map[string]any{
					"message": map[string]any{"text": "ok"},
					"status":  "ALTERNATIVE_STATUS_FINAL",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal ok response: %v", err)
	}
	return string(b)
}

// ocrResponse - корректный ответ Yandex Vision OCR (result.textAnnotation.fullText).
func ocrResponse(t *testing.T) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"result": map[string]any{
			"textAnnotation": map[string]any{
				"fullText": "Глюкоза 6.4 ммоль/л Холестерин 5.8 ммоль/л",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal ocr response: %v", err)
	}
	return string(b)
}

// visionResponse - корректный ответ Yandex Vision API (batchAnalyze /
// TEXT_DETECTION) для PDF: results[].results[].text_detection.pages[].
// blocks[].lines[].words[].text.
func visionResponse(t *testing.T) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"results": []any{
			map[string]any{
				"results": []any{
					map[string]any{
						"text_detection": map[string]any{
							"pages": []any{
								map[string]any{
									"width":  100,
									"height": 100,
									"blocks": []any{
										map[string]any{
											"lines": []any{
												map[string]any{
													"words": []any{
														map[string]any{"text": "Глюкоза"},
														map[string]any{"text": "6.4"},
														map[string]any{"text": "ммоль/л"},
													},
												},
												map[string]any{
													"words": []any{
														map[string]any{"text": "Холестерин"},
														map[string]any{"text": "5.8"},
														map[string]any{"text": "ммоль/л"},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal vision response: %v", err)
	}
	return string(b)
}

// captureServer - тестовый сервер, возвращающий failStatus первые failTimes
// раз, затем respond. Запоминает последнее тело запроса и число вызовов.
func captureServer(t *testing.T, respond string, failTimes int, failStatus int) (*httptest.Server, *int32, *atomic.Value) {
	t.Helper()
	var calls int32
	var lastBody atomic.Value

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		lastBody.Store(string(body))
		n := atomic.AddInt32(&calls, 1)
		if int(n) <= failTimes {
			w.WriteHeader(failStatus)
			_, _ = w.Write([]byte("temporary failure"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respond))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls, &lastBody
}

func newTestClient(t *testing.T, completionBase string) *Client {
	t.Helper()
	c, err := NewClient(Config{
		APIKey:         "test-key",
		FolderID:       "test-folder",
		Model:          "yandexgpt",
		MaxConcurrency: 1,
		Timeout:        0,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.completionEndpoint = completionBase
	c.ocrEndpoint = completionBase
	return c
}

// TestRetryOn5xx: 2 раза 500 -> 3-й раз 200. Успех, 3 вызова.
func TestRetryOn5xx(t *testing.T) {
	srv, calls, _ := captureServer(t, okResponse(t), 2, http.StatusInternalServerError)
	c := newTestClient(t, srv.URL)

	out, err := c.GenerateDossierJSON(context.Background(), "data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "ok" {
		t.Fatalf("unexpected output: %q", out)
	}
	if got := atomic.LoadInt32(calls); got != 3 {
		t.Fatalf("expected 3 calls (2 retries + 1 success), got %d", got)
	}
}

// TestRetryExhausted: все попытки 503 -> ошибка ErrYandexRetryExhausted.
func TestRetryExhausted(t *testing.T) {
	srv, calls, _ := captureServer(t, okResponse(t), defaultMaxRetries, http.StatusServiceUnavailable)
	c := newTestClient(t, srv.URL)

	_, err := c.GenerateDossierJSON(context.Background(), "data")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "yandex request failed after retries") {
		t.Fatalf("expected retry-exhausted error, got %v", err)
	}
	if got := atomic.LoadInt32(calls); got != int32(defaultMaxRetries) {
		t.Fatalf("expected %d calls, got %d", defaultMaxRetries, got)
	}
}

// TestNoRetryOn400: 400 (bad request) не повторяется - сразу ошибка, 1 вызов.
func TestNoRetryOn400(t *testing.T) {
	srv, calls, _ := captureServer(t, okResponse(t), defaultMaxRetries, http.StatusBadRequest)
	c := newTestClient(t, srv.URL)

	_, err := c.GenerateDossierJSON(context.Background(), "data")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "yandex api error (400)") {
		t.Fatalf("expected 400 api error, got %v", err)
	}
	if got := atomic.LoadInt32(calls); got != 1 {
		t.Fatalf("expected 1 call (no retry on 400), got %d", got)
	}
}

// TestJSONRequestConfig: JSON-метод шлёт modelUri, completionOptions
// (temperature/maxTokens) и messages.
func TestJSONRequestConfig(t *testing.T) {
	srv, _, lastBody := captureServer(t, okResponse(t), 0, http.StatusOK)
	c := newTestClient(t, srv.URL)

	if _, err := c.GenerateDossierJSON(context.Background(), "data"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, ok := lastBody.Load().(string)
	if !ok || raw == "" {
		t.Fatal("no request body captured")
	}
	var req struct {
		ModelURI          string `json:"modelUri"`
		CompletionOptions struct {
			Temperature float64 `json:"temperature"`
			MaxTokens   int     `json:"maxTokens"`
		} `json:"completionOptions"`
		Messages []struct {
			Role string `json:"role"`
			Text string `json:"text"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	if req.ModelURI == "" {
		t.Fatal("expected modelUri in request")
	}
	if req.CompletionOptions.Temperature != defaultJSONTemperature {
		t.Fatalf("expected temperature %v, got %v", defaultJSONTemperature, req.CompletionOptions.Temperature)
	}
	if req.CompletionOptions.MaxTokens != defaultMaxOutputTokensJSON {
		t.Fatalf("expected maxTokens %v, got %v", defaultMaxOutputTokensJSON, req.CompletionOptions.MaxTokens)
	}
	if len(req.Messages) < 1 {
		t.Fatal("expected at least one message")
	}
}

// TestKeyNotSet: пустой ключ -> ErrYandexKeyNotSet (без сетевого вызова).
func TestKeyNotSet(t *testing.T) {
	c, err := NewClient(Config{APIKey: "", FolderID: "f", Model: "yandexgpt", MaxConcurrency: 1})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = c.GenerateDossierJSON(context.Background(), "data")
	if err == nil || err.Error() != locales.ErrYandexKeyNotSet {
		t.Fatalf("expected ErrYandexKeyNotSet, got %v", err)
	}
}

// TestOCRRecognizesText: PDF-адаптер маршрутизируется на Vision API и
// корректно извлекает текст из results[].results[].text_detection.
func TestOCRRecognizesText(t *testing.T) {
	srv, _, _ := captureServer(t, visionResponse(t), 0, http.StatusOK)
	c := newTestClient(t, srv.URL)
	c.visionEndpoint = srv.URL

	out, err := c.recognizeText(context.Background(), []byte("fake pdf bytes"), "application/pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Глюкоза") {
		t.Fatalf("expected OCR text to contain 'Глюкоза', got %q", out)
	}
}

// TestFileMethodUsesOCR: файловый метод сначала зовёт OCR, затем передаёт
// извлечённый текст в YandexGPT (проверяем, что он попал в user-сообщение).
func TestFileMethodUsesOCR(t *testing.T) {
	ocrSrv, _, _ := captureServer(t, visionResponse(t), 0, http.StatusOK)
	compSrv, _, compBody := captureServer(t, okResponse(t), 0, http.StatusOK)

	c := newTestClient(t, compSrv.URL)
	c.visionEndpoint = ocrSrv.URL

	out, err := c.GenerateAnalysisFromFileJSON(context.Background(), []byte("fake"), "application/pdf", "контекст пациента")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "ok" {
		t.Fatalf("unexpected output: %q", out)
	}

	raw, ok := compBody.Load().(string)
	if !ok || raw == "" {
		t.Fatal("no completion request body captured")
	}
	var req struct {
		Messages []struct {
			Role string `json:"role"`
			Text string `json:"text"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("decode completion body: %v", err)
	}
	var userMsg string
	for _, m := range req.Messages {
		if m.Role == "user" {
			userMsg = m.Text
		}
	}
	if !strings.Contains(userMsg, "Глюкоза") {
		t.Fatalf("expected OCR text in user message, got %q", userMsg)
	}
}

// TestVisionBioscanRequest: базовый Bioscan с фото шлёт МУЛЬТИМОДАЛЬНЫЙ
// запрос - в теле completion-запроса есть поле images (base64 фото) и modelUri.
// OCR отдельным сервером возвращает текст, чтобы пройти подготовку данных.
func TestVisionBioscanRequest(t *testing.T) {
	ocrSrv, _, _ := captureServer(t, ocrResponse(t), 0, http.StatusOK)
	compSrv, _, compBody := captureServer(t, okResponse(t), 0, http.StatusOK)

	c := newTestClient(t, compSrv.URL)
	c.ocrEndpoint = ocrSrv.URL

	// 16 байт (PNG-заголовок) - заведомо меньше лимита vision.
	img := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52}
	out, err := c.GenerateBioscanJSON(context.Background(), [][]byte{img}, "image/png", "Пол: Мужской\nВозраст: 29")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "ok" {
		t.Fatalf("unexpected output: %q", out)
	}

	raw, ok := compBody.Load().(string)
	if !ok || raw == "" {
		t.Fatal("no completion request body captured")
	}
	var req struct {
		ModelURI string `json:"modelUri"`
		Messages []struct {
			Role   string   `json:"role"`
			Text   string   `json:"text"`
			Images []string `json:"images"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("decode completion body: %v", err)
	}
	if req.ModelURI == "" {
		t.Fatal("expected modelUri in request")
	}
	var userImages []string
	found := false
	for _, m := range req.Messages {
		if m.Role == "user" {
			userImages = m.Images
			found = true
		}
	}
	if !found {
		t.Fatal("expected user message")
	}
	if len(userImages) != 1 {
		t.Fatalf("expected 1 image in user message, got %d", len(userImages))
	}
	if userImages[0] == "" {
		t.Fatal("expected non-empty base64 image in user message")
	}
}

// TestVisionFallbackOnError: если vision-запрос падает (напр. модель не
// поддерживает изображения), базовый Bioscan откатывается на текстовый путь
// (OCR+текст) и всё равно возвращает результат. Эмулируем сервер, который
// отвечает 400 только на запросы С изображениями (поле images), а на
// обычные текстовые запросы - 200.
func TestVisionFallbackOnError(t *testing.T) {
	ocrSrv, _, _ := captureServer(t, ocrResponse(t), 0, http.StatusOK)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), `"images"`) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("model does not support images"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(okResponse(t)))
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv.URL)
	c.ocrEndpoint = ocrSrv.URL

	img := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52}
	out, err := c.GenerateBioscanJSON(context.Background(), [][]byte{img}, "image/png", "Пол: Мужской")
	if err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}
	if out != "ok" {
		t.Fatalf("unexpected output: %q", out)
	}
}
