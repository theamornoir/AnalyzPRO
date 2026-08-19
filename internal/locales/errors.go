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

	// Ошибки AI-клиента (Gemini).
	ErrGeminiKeyNotSet       = "GOOGLE_GEMINI_API_KEY not set"
	ErrGeminiUnsupportedMime = "unsupported mime type for gemini: %s"
	ErrGeminiAPIError        = "gemini api error (%d): %s"
	ErrGeminiResponseParse   = "gemini response parse error: %w (body: %s)"
	ErrGeminiEmptyResponse   = "gemini returned empty response (finishReason=%s)"
	ErrGeminiUnsupportedFile = "gemini supports only image/pdf files for analysis"
	ErrGeminiNoFileData      = "no supported file data provided"
	ErrGeminiNoPhotoData     = "no photo data provided"
	// Ошибки надёжности (retry/transport). Тексты вынесены в локали - код
	// клиента остаётся чистым.
	ErrGeminiTransport      = "gemini transport error: %w"
	ErrGeminiRetryExhausted = "gemini request failed after retries: %w"
)
