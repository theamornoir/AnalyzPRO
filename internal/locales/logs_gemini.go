package locales

// Log messages for the Gemini AI package
const (
	LogGeminiClientInit        = "🔑 Gemini Client initialized with model: %s"
	LogGeminiGenerateCalled    = "📤 GenerateAnalysisSummary called with input length: %d"
	LogGeminiUsingMockResponse = "🧪 Using mock response for analysis"
	LogGeminiBuiltPrompt       = "📝 Built prompt length: %d characters"
	LogGeminiRegularMode       = "👤 Regular mode"
	LogGeminiAthleteMode       = "🏋️ Athlete mode: on course"
	LogGeminiStartRequest      = "🔄 Starting Gemini API request..."
	LogGeminiStartRawRequest   = "🔄 Starting Gemini RAW request..."
	LogGeminiMarshalPayloadErr = "❌ Failed to marshal payload: %v"
	LogGeminiBodySize          = "📦 Request body size: %d bytes"
	LogGeminiResponseStatus    = "📥 Response status: %d"
	// LogGeminiResponseBodySize - логируем ТОЛЬКО статус и размер ответа.
	// Полное тело НЕ логируется: оно содержит персональные мед. данные (PII).
	LogGeminiResponseBodySize  = "📥 Response: status=%d, size=%d bytes"
	LogGeminiResponseBodyShort = "📄 Response body (first 500 chars): %s..."
	LogGeminiResponseBody      = "📄 Response body: %s"
	LogGeminiUnmarshalErr      = "❌ Failed to unmarshal response: %v"
	LogGeminiAPIError          = "❌ Gemini returned error: Code=%d, Message=%s"
	LogGeminiExtractedLen      = "📝 Extracted text length: %d characters"
	LogGeminiEmptyResponse     = "⚠️ Empty response from Gemini"
	LogGeminiFullResponse      = "🔍 Full response: %s"
	LogGeminiSuccess           = "✅ Successfully generated response from Gemini"
	LogGeminiRawJSONLen        = "📝 RAW JSON response length: %d"
	LogGeminiNonOKStatus       = "❌ Non-OK status: %d"
	LogGeminiErrorDetails      = "📋 Gemini error details: Code=%d, Message=%s"
	LogGeminiRequestModel      = "🌐 Request model: %s"
	LogGeminiRequestErr        = "❌ Failed to create request: %v"
	LogGeminiSendingRequest    = "⏳ Sending request to Gemini..."
	LogGeminiHTTPFailed        = "❌ HTTP request failed: %v"
	LogGeminiRequestDuration   = "⏱️ Request completed in %v"
	LogGeminiReadBodyErr       = "❌ Failed to read response body: %v"

	// Mock-related logs
	LogGeminiMockAnalysis         = "🧪 Using mock response for analysis"
	LogGeminiMockAnalysisJSON     = "🧪 Using mock analysis JSON response"
	LogGeminiMockAnalysisFileJSON = "🧪 Using mock analysis JSON response from file"
	LogGeminiMockDossier          = "🧪 Using mock health dossier JSON response"
	LogGeminiMockBioscanJSON      = "🧪 Using mock bioscan JSON response"
	LogGeminiMockFileWithContext  = "🧪 Using mock response for file analysis with context"

	// Fallback logs
	LogGeminiFallbackRateLimit     = "⚠️ Returning rate-limit fallback response"
	LogGeminiFallbackLocationError = "⚠️ Returning location error fallback response"
	LogGeminiFallbackNoKey         = "⚠️ Returning no-key fallback response"
	LogGeminiFallbackUnavailable   = "⚠️ Returning service-unavailable fallback response"
)
