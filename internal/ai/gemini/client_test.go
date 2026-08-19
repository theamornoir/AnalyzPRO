package gemini

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

// okResponse - корректный ответ Gemini с одним текстовым кандидатом.
func okResponse(t *testing.T) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"candidates": []any{
			map[string]any{
				"content": map[string]any{
					"parts": []any{map[string]any{"text": "ok"}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal ok response: %v", err)
	}
	return string(b)
}

// captureServer - тестовый сервер, возвращающий failStatus первые failTimes
// раз, затем okResponse. Запоминает последнее тело запроса и число вызовов.
func captureServer(t *testing.T, failTimes int, failStatus int) (*httptest.Server, *int32, *atomic.Value) {
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
		_, _ = w.Write([]byte(okResponse(t)))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls, &lastBody
}

func newTestClient(t *testing.T, base string) *Client {
	t.Helper()
	c, err := NewClient(Config{
		APIKey:         "test-key",
		Model:          "gemini-2.5-flash",
		APIBase:        base,
		MaxConcurrency: 1,
		Timeout:        0,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

// TestRetryOn5xx: 2 раза 500 -> 3-й раз 200. Успех, число вызовов > 1.
func TestRetryOn5xx(t *testing.T) {
	srv, calls, _ := captureServer(t, 2, http.StatusInternalServerError)
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

// TestRetryExhausted: все попытки 503 -> ошибка ErrGeminiRetryExhausted.
func TestRetryExhausted(t *testing.T) {
	srv, calls, _ := captureServer(t, defaultMaxRetries, http.StatusServiceUnavailable)
	c := newTestClient(t, srv.URL)

	_, err := c.GenerateDossierJSON(context.Background(), "data")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "gemini request failed after retries") {
		t.Fatalf("expected retry-exhausted error, got %v", err)
	}
	if got := atomic.LoadInt32(calls); got != int32(defaultMaxRetries) {
		t.Fatalf("expected %d calls, got %d", defaultMaxRetries, got)
	}
}

// TestNoRetryOn400: 400 (bad request) не повторяется - сразу ошибка, 1 вызов.
func TestNoRetryOn400(t *testing.T) {
	srv, calls, _ := captureServer(t, defaultMaxRetries, http.StatusBadRequest)
	c := newTestClient(t, srv.URL)

	_, err := c.GenerateDossierJSON(context.Background(), "data")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "gemini api error (400)") {
		t.Fatalf("expected 400 api error, got %v", err)
	}
	if got := atomic.LoadInt32(calls); got != 1 {
		t.Fatalf("expected 1 call (no retry on 400), got %d", got)
	}
}

// TestJSONRequestConfig: JSON-метод шлёт responseMimeType, temperature и
// safetySettings BLOCK_NONE.
func TestJSONRequestConfig(t *testing.T) {
	srv, _, lastBody := captureServer(t, 0, http.StatusOK)
	c := newTestClient(t, srv.URL)

	if _, err := c.GenerateDossierJSON(context.Background(), "data"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, ok := lastBody.Load().(string)
	if !ok || raw == "" {
		t.Fatal("no request body captured")
	}
	var req struct {
		GenerationConfig struct {
			Temperature      float64 `json:"temperature"`
			ResponseMimeType string  `json:"responseMimeType"`
		} `json:"generationConfig"`
		SafetySettings []struct {
			Category  string `json:"category"`
			Threshold string `json:"threshold"`
		} `json:"safetySettings"`
	}
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	if req.GenerationConfig.ResponseMimeType != "application/json" {
		t.Fatalf("expected responseMimeType application/json, got %q", req.GenerationConfig.ResponseMimeType)
	}
	if req.GenerationConfig.Temperature != defaultJSONTemperature {
		t.Fatalf("expected temperature %v, got %v", defaultJSONTemperature, req.GenerationConfig.Temperature)
	}
	if len(req.SafetySettings) == 0 {
		t.Fatal("expected safetySettings to be present")
	}
	for _, s := range req.SafetySettings {
		if s.Threshold != "BLOCK_NONE" {
			t.Fatalf("expected BLOCK_NONE, got %q for %q", s.Threshold, s.Category)
		}
	}
}

// TestKeyNotSet: пустой ключ -> ErrGeminiKeyNotSet (без сетевого вызова).
func TestKeyNotSet(t *testing.T) {
	c, err := NewClient(Config{APIKey: "", Model: "m", APIBase: "http://example.invalid", MaxConcurrency: 1})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = c.GenerateDossierJSON(context.Background(), "data")
	if err == nil || err.Error() != locales.ErrGeminiKeyNotSet {
		t.Fatalf("expected ErrGeminiKeyNotSet, got %v", err)
	}
}
