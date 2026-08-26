package locales

// Error Messages
const (
	ErrEmptyJSONFromAI     = "received empty JSON from AI model"
	ErrParseAnalysisJSON   = "parse analysis report JSON: %w"
	ErrRenderReportHTML    = "render report html: %w"
	ErrRenderReportPDF     = "render report pdf: %w"
	ErrGenerateBioscanJSON = "generate bioscan json: %w"
	ErrHTML2PDFKeyNotSet   = "HTML2PDF_API_KEY not set"
	ErrAPIError            = "API error (status %d): %s"

	// Ошибки AI-клиента (YandexGPT).
	ErrYandexKeyNotSet       = "YANDEX_API_KEY not set"
	ErrYandexUnsupportedMime = "unsupported mime type for yandex: %s"
	ErrYandexAPIError        = "yandex api error (%d): %s"
	ErrYandexResponseParse   = "yandex response parse error: %w (body: %s)"
	ErrYandexEmptyResponse   = "yandex returned empty response"
	ErrYandexUnsupportedFile = "yandex supports only image/pdf files for analysis"
	ErrYandexNoFileData      = "no supported file data provided"
	ErrYandexNoPhotoData     = "no photo data provided"
	// Ошибки надёжности (retry/transport). Тексты вынесены в локали - код
	// клиента остаётся чистым.
	ErrYandexTransport      = "yandex transport error: %w"
	ErrYandexRetryExhausted = "yandex request failed after retries: %w"
)
