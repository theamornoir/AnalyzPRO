package pdfservice

// Package pdfservice конвертирует print-ready HTML-отчёты в PDF.
//
// Конвертация идёт через внешний SaaS html2pdf.app (https://html2pdf.app):
// бот (или любой вызывающий код) шлёт готовый HTML, а сервис возвращает PDF.
// Это и есть описанный пользователем сценарий «отправляем на сервер,
// конвертируем, отдаём PDF».
//
// Шаблоны отчётов (health_dossier.html, body_scan_report.html) самодостаточны
// (нет внешних CDN/шрифтов, только inline CSS/SVG) и PDF-first
// (@page{size:A4}, print-color-adjust), поэтому html2pdf.app рендерит их
// вертикально идентично предпросмотру без запуска headless-браузера.
//
// Требуется ключ HTML2PDF_API_KEY (задаётся в .env). Если ключ не задан -
// ConvertHTML возвращает ошибку, и вызывающая сторона корректно откатывается
// к отправке отчёта как HTML.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// Converter - единый интерфейс конвертации HTML→PDF.
type Converter interface {
	// ConvertHTML принимает готовый HTML (print-ready) и возвращает сырые
	// байты PDF-документа.
	ConvertHTML(ctx context.Context, html string) ([]byte, error)
}

// Config - параметры выбора конвертера.
type Config struct {
	// HTML2PDFAPIKey - API-ключ html2pdf.app (env HTML2PDF_API_KEY).
	// Если пуст - PDF-конвертация недоступна (ConvertHTML вернёт ошибку,
	// вызывающая сторона откатится к HTML).
	HTML2PDFAPIKey string
}

// html2PDFEndpoint - REST-эндпоинт html2pdf.app для конвертации HTML→PDF.
// ВАЖНО: поддомен api.html2pdf.com НЕ существует (не резолвится, NXDOMAIN) -
// поэтому ранее конвертация падала с «no such host». Рабочий сервис это
// html2pdf.app (https://html2pdf.app, путь /v1/generate). Другой путь -
// задайте HTML2PDF_API_URL в окружении.
const html2PDFEndpoint = "https://api.html2pdf.app/v1/generate"

// New выбирает конвертер по конфигу. При наличии ключа возвращает
// HTML2PDFConverter (html2pdf.app), иначе - errorConverter (откат к HTML).
// Никогда не паникует.
func New(cfg Config) Converter {
	apiKey := strings.TrimSpace(cfg.HTML2PDFAPIKey)
	if apiKey == "" {
		log.Printf(locales.LogPDFKeyNotFound)
		return &errorConverter{err: fmt.Errorf("%s", locales.ErrHTML2PDFKeyNotSet)}
	}
	log.Printf("✅ pdfservice: HTML→PDF через html2pdf.app (endpoint %s)", html2PDFEndpoint)
	return NewHTML2PDFConverter(apiKey)
}

// ----------------------------------------------------------------------------
// HTML2PDFConverter - «отправить на сервер», POST HTML → PDF (html2pdf.app).
// ----------------------------------------------------------------------------

// HTML2PDFConverter отправляет HTML на внешний сервис html2pdf.app и читает PDF.
type HTML2PDFConverter struct {
	endpoint string
	apiKey   string
	client   *http.Client
}

// NewHTML2PDFConverter возвращает конвертер html2pdf.app. Эндпоинт можно
// переопределить через env HTML2PDF_API_URL (если документация вашего тарифа
// использует другой путь).
func NewHTML2PDFConverter(apiKey string) *HTML2PDFConverter {
	endpoint := strings.TrimSpace(os.Getenv("HTML2PDF_API_URL"))
	if endpoint == "" {
		endpoint = html2PDFEndpoint
	}
	return &HTML2PDFConverter{
		endpoint: endpoint,
		apiKey:   apiKey,
		client:   &http.Client{Timeout: 120 * time.Second},
	}
}

type html2pdfRequest struct {
	HTML string `json:"html"`
}

// ConvertHTML шлёт HTML на html2pdf.app и возвращает PDF-байты.
// При ошибке (нет ключа / сервис недоступен / не-PDF ответ) возвращает error,
// чтобы вызывающая сторона откатилась к отправке HTML.
func (c *HTML2PDFConverter) ConvertHTML(ctx context.Context, html string) ([]byte, error) {
	if strings.TrimSpace(html) == "" {
		return nil, fmt.Errorf("пустой HTML - конвертация в PDF невозможна")
	}

	log.Printf("📤 [PDF] конвертация HTML→PDF: endpoint=%s, размер HTML=%d байт, ключ задан=%v",
		c.endpoint, len(html), c.apiKey != "")

	body, err := json.Marshal(html2pdfRequest{HTML: html})
	if err != nil {
		return nil, fmt.Errorf("ошибка упаковки запроса: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ошибка формирования запроса: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Авторизация html2pdf.app - через заголовок X-API-Key (именно так сервис
	// принимает ключ; Bearer-схема им не поддерживается). Если HTML2PDF_API_URL
	// указывает на другой сервис с иной схемой авторизации - поменяйте здесь.
	req.Header.Set("X-API-Key", c.apiKey)

	log.Printf("🌐 [PDF] POST %s (тело запроса %d байт)", c.endpoint, len(body))
	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("❌ [PDF] ошибка запроса к html2pdf.app: %v (endpoint=%s)", err, c.endpoint)
		return nil, fmt.Errorf("html2pdf.app недоступен: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("❌ [PDF] ошибка чтения тела ответа html2pdf.app: %v", err)
		return nil, fmt.Errorf("ошибка чтения ответа html2pdf.app: %w", err)
	}

	respPreview := pdfPrefix(data)
	log.Printf("📥 [PDF] ответ html2pdf.app: HTTP %d, content-type=%q, получено байт=%d, префикс=%q",
		resp.StatusCode, resp.Header.Get("Content-Type"), len(data), respPreview)

	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ [PDF] html2pdf.app вернул ошибку HTTP %d. Тело ответа (первые 800 байт): %s",
			resp.StatusCode, firstN(string(data), 800))
		return nil, fmt.Errorf(locales.ErrAPIError, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if len(data) < 800 || !bytes.HasPrefix(data, []byte("%PDF")) {
		log.Printf("❌ [PDF] ответ не похож на PDF: %d байт, префикс %q (ожидался %%PDF). Тело (первые 500 байт): %s",
			len(data), respPreview, firstN(string(data), 500))
		return nil, fmt.Errorf("html2pdf.app вернул невалидный PDF (статус %d, %d байт, префикс %q)", resp.StatusCode, len(data), respPreview)
	}

	log.Printf("✅ [PDF] PDF успешно получен от html2pdf.app: %d байт", len(data))
	return data, nil
}

func pdfPrefix(b []byte) string {
	if len(b) > 5 {
		b = b[:5]
	}
	return string(b)
}

// firstN возвращает первые n символов строки (для логов тел ошибок).
func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ----------------------------------------------------------------------------
// errorConverter - заглушка, когда ключ html2pdf.app не задан (PDF невозможен).
// ----------------------------------------------------------------------------

// errorConverter возвращает ошибку - вызывающая сторона откатывается к HTML.
type errorConverter struct {
	err error
}

func (n *errorConverter) ConvertHTML(ctx context.Context, html string) ([]byte, error) {
	return nil, n.err
}
