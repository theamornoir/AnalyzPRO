package pdfservice

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const sampleHTML = `<!DOCTYPE html><html><head><meta charset="utf-8">
<style>
@page { size: A4; margin: 10mm; }
body { font-family: sans-serif; }
</style></head><body>
<h1>Prisma — тестовый отчёт</h1>
<p>Это print-ready HTML для проверки конвертации в PDF через html2pdf.com.</p>
<table border="1" cellspacing="0" cellpadding="4">
<tr><th>Показатель</th><th>Значение</th></tr>
<tr><td>Индекс здоровья</td><td>82</td></tr>
<tr><td>Энергия</td><td>77</td></tr>
</table>
</body></html>`

// fakePDF — валидный PDF-ответ (префикс %PDF + достаточный объём, чтобы
// пройти проверку размера в HTML2PDFConverter, которая отсекает заведомо
// не-PDF ответы короче 800 байт).
var fakePDF = []byte("%PDF-1.4\n" + strings.Repeat("0", 900) + "\ntrailer<<>>\n%%EOF")

// TestHTML2PDFConverterSuccess — успешная конвертация через html2pdf.com.
func TestHTML2PDFConverterSuccess(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		gotAuth = r.Header.Get("X-API-Key")
		var req html2pdfRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if req.HTML == "" {
			http.Error(w, "empty html", http.StatusBadRequest)
			return
		}
		gotBody = req.HTML
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(fakePDF)
	}))
	defer srv.Close()

	c := &HTML2PDFConverter{endpoint: srv.URL, apiKey: "test-key", client: &http.Client{Timeout: 5 * time.Second}}
	pdf, err := c.ConvertHTML(context.Background(), sampleHTML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("ожидался PDF-префикс, получено: %q", pdf[:min(8, len(pdf))])
	}
	if gotAuth != "test-key" {
		t.Fatalf("ожидался заголовок X-API-Key: test-key, получено %q", gotAuth)
	}
	if !strings.Contains(gotBody, "Prisma") {
		t.Fatalf("тело запроса не содержит HTML: %q", gotBody[:min(64, len(gotBody))])
	}
}

// TestHTML2PDFConverterErrorOnNonPDF — сервис вернул не-PDF: ошибка.
func TestHTML2PDFConverterErrorOnNonPDF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>boom</html>"))
	}))
	defer srv.Close()

	c := &HTML2PDFConverter{endpoint: srv.URL, apiKey: "k", client: &http.Client{Timeout: 5 * time.Second}}
	if _, err := c.ConvertHTML(context.Background(), sampleHTML); err == nil {
		t.Fatal("ожидалась ошибка при не-PDF ответе")
	}
}

// TestHTML2PDFConverterErrorOnServerError — сервис вернул 5xx: ошибка.
func TestHTML2PDFConverterErrorOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &HTML2PDFConverter{endpoint: srv.URL, apiKey: "k", client: &http.Client{Timeout: 5 * time.Second}}
	if _, err := c.ConvertHTML(context.Background(), sampleHTML); err == nil {
		t.Fatal("ожидалась ошибка при статусе 500")
	}
}

// TestHTML2PDFConverterErrorOnEmptyHTML — пустой HTML: ошибка (до сети).
func TestHTML2PDFConverterErrorOnEmptyHTML(t *testing.T) {
	c := &HTML2PDFConverter{endpoint: "http://127.0.0.1:1", apiKey: "k", client: &http.Client{Timeout: 1 * time.Second}}
	if _, err := c.ConvertHTML(context.Background(), "   "); err == nil {
		t.Fatal("ожидалась ошибка при пустом HTML")
	}
}

// TestErrorConverterReturnsKeyError — при пустом ключе New() отдаёт
// errorConverter, который возвращает ErrHTML2PDFKeyNotSet.
func TestErrorConverterReturnsKeyError(t *testing.T) {
	c := New(Config{HTML2PDFAPIKey: ""})
	if _, err := c.ConvertHTML(context.Background(), sampleHTML); err == nil {
		t.Fatal("ожидалась ошибка отсутствия ключа")
	}
}

// TestNewWithKeyReturnsHTML2PDFConverter — при заданном ключе New() возвращает
// рабочий конвертер (проверяем реальным httptest-сервером).
func TestNewWithKeyReturnsHTML2PDFConverter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(fakePDF)
	}))
	defer srv.Close()

	// Переопределим эндпоинт через env, чтобы не биться об реальный api.html2pdf.app.
	t.Setenv("HTML2PDF_API_URL", srv.URL)
	c := New(Config{HTML2PDFAPIKey: "k"})
	pdf, err := c.ConvertHTML(context.Background(), sampleHTML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("ожидался PDF от конвертера")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
