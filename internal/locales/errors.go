package locales

// Error Messages
const (
	ErrEmptyJSONFromAI     = "received empty JSON from AI model"
	ErrParseAnalysisJSON   = "parse analysis report JSON: %w"
	ErrRenderReportHTML    = "render report html: %w"
	ErrGenerateBioscanJSON = "generate bioscan json: %w"
	ErrHTML2PDFKeyNotSet   = "HTML2PDF_API_KEY not set"
	ErrAPIError            = "API error (status %d): %s"
)
