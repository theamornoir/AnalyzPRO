package locales

// extract_ai.go собирает строковые литералы (сообщения, логи, ошибки, промпты),
// извлечённые из пакетов internal/ai/** (orchestrator, gemini, httpclient, mock),
// чтобы централизовать их в пакете locales. НЕ редактируйте существующие файлы
// locales ради этих констант — всё новое добавляется сюда.

// ---------------------------------------------------------------------------
// Errors (провайдер YandexGPT)
// ---------------------------------------------------------------------------
const (
	ErrYandexMarshal             = "yandexgpt marshal error: %w"
	ErrYandexRequestBuild        = "yandexgpt request build error: %w"
	ErrYandexHTTP                = "yandexgpt http error: %w"
	ErrYandexRead                = "yandexgpt read error: %w"
	ErrYandexRequestFailedStatus = "yandexgpt request failed with status %d: %s"
	ErrYandexDecode              = "yandexgpt decode error: %w (body: %s)"
	ErrYandexError               = "yandexgpt error: %s"
	ErrYandexNoAlternatives      = "yandexgpt returned no alternatives"
	ErrYandexEmptyResponse       = "yandexgpt returned empty response"
	ErrYandexImageOnly           = "yandexgpt supports only image files for analysis"
)

// ---------------------------------------------------------------------------
// Errors (общие / переиспользуемые между провайдерами)
// ---------------------------------------------------------------------------
const (
	ErrNoPhotoDataProvided  = "no photo data provided"
	ErrNoImageDataProvided  = "no image data provided"
	ErrNoAttachmentProvided = "no attachment data provided"
)

// ---------------------------------------------------------------------------
// Errors (провайдер DeepSeek)
// ---------------------------------------------------------------------------
const (
	ErrDeepseekImageOnly     = "deepseek supports only image files for analysis"
	ErrDeepseekEmptyResponse = "deepseek returned empty response"
)

// ---------------------------------------------------------------------------
// Errors (провайдер OpenRouter)
// ---------------------------------------------------------------------------
const (
	ErrOpenRouterPDFExtract           = "openrouter: не удалось извлечь текст из PDF: %w"
	ErrOpenRouterPDFNoText            = "openrouter: PDF не содержит извлекаемого текста (возможно, это сканированное изображение)"
	ErrOpenRouterImagePDFOnly         = "openrouter supports only image or pdf files for analysis"
	ErrOpenRouterModelError           = "openrouter: модель %s - %w"
	ErrOpenRouterModelEmpty           = "openrouter: модель %s вернула пустой ответ"
	ErrOpenRouterAllModelsUnavailable = "openrouter: все модели недоступны: %w"
)

// ---------------------------------------------------------------------------
// Errors (провайдер Claude)
// ---------------------------------------------------------------------------
const (
	ErrClaudeClientNotInit   = "claude client not initialized"
	ErrClaudeAPIKeyNotSet    = "claude api key not set"
	ErrClaudeImagePDFOnly    = "claude supports only image/pdf files for analysis"
	ErrClaudeAPIError        = "claude api error (%d): %s"
	ErrClaudeResponseParse   = "claude response parse error: %w (body: %s)"
	ErrClaudeEmptyResponse   = "claude returned empty response"
	ErrClaudeUnsupportedMime = "unsupported mime type for claude: %s"
)

// ---------------------------------------------------------------------------
// Errors (пакет gemini)
// ---------------------------------------------------------------------------
const (
	ErrGeminiEmptyAPIKey         = "empty gemini api key"
	ErrGeminiEmptyFileData       = "empty file data"
	ErrGeminiError               = "gemini error: %s"
	ErrGeminiErrorWithCode       = "gemini error %d: %s"
	ErrGeminiRequestFailedStatus = "gemini request failed with status %d: %s"
)

// ---------------------------------------------------------------------------
// Logs (провайдер YandexGPT / Gemini / httpclient)
// ---------------------------------------------------------------------------
const (
	LogYandexRequest = "📤 YandexGPT request: model=%s, messages=%d"

	LogGeminiSwitchFallback = "🔄 Gemini: модель %q недоступна - переключаюсь на фоллбэк %q"
	LogGeminiModelNotFound  = "⚠️ Gemini: %q - модель не найдена или недоступна, пробую следующую"
	LogGeminiFallbackWorked = "✅ Gemini: фоллбэк-модель %q сработала (исходная %q недоступна)"

	LogHTTPClientSOCKS5Fallback   = "⚠️ SOCKS5 proxy %s failed (%v), falling back to direct"
	LogHTTPClientProxyInvalid     = "⚠️ explicit proxy %q invalid (%v), falling back to system/direct"
	LogHTTPClientUsingProxy       = "✅ Gemini client using proxy: %s"
	LogHTTPClientUsingProxySystem = "✅ Gemini client using proxy: %s (system HTTP_PROXY/HTTPS_PROXY)"
	LogHTTPClientUsingDirect      = "⚠️ Gemini client using DIRECT connection (no proxy)"
)

// ---------------------------------------------------------------------------
// AI prompts — системные промпты провайдеров (анализ / биоскан / досье).
// ---------------------------------------------------------------------------
const (
	// Только YandexGPT.
	PromptAnalysisSummaryYandex = "Ты - опытный медицинский аналитик. Проанализируй данные и дай практичные рекомендации."
	PromptAnalysisJSONYandex    = "Ты - медицинский аналитик. Верни ответ строго в формате JSON, без markdown-разметки и пояснений."
	PromptDossierJSONYandex     = "Ты - опытный врач-диагност и аналитик здоровья. Верни ответ строго в формате JSON, без markdown-разметки и пояснений."

	// Общие промпты (DeepSeek / OpenRouter / Claude).
	PromptAnalysisSummary  = "Ты - медицинский аналитик. Проанализируй данные и дай рекомендации."
	PromptAnalysisJSON     = "Ты - медицинский аналитик. Верни ответ в формате JSON."
	PromptVisionAnalysis   = "Ты - опытный врач-диагност. Проанализируй приложенные медицинские изображения и дай развёрнутый анализ с рекомендациями."
	PromptJSONAnalysis     = "Ты - опытный врач-диагност. Верни ответ строго в формате JSON, без markdown и комментариев."
	PromptDocumentAnalysis = "Ты - опытный врач-диагност. Проанализируй медицинский документ и дай развёрнутый анализ с рекомендациями."
	PromptDossierJSON      = "Ты - опытный врач-диагност и аналитик здоровья. Верни ответ строго в формате JSON, без markdown и комментариев."

	// Только OpenRouter (досье, unicode-литерал в исходнике).
	PromptDossierJSONOpenRouter = "Ты - опытный врач-диагност. Верни ответ строго в формате JSON, без markdown."

	// Только Claude.
	PromptFileAnalysisClaude    = "Ты - опытный врач-диагност. Проанализируй приложенные медицинские изображения/документы и дай развёрнутый анализ с рекомендации."
	PromptBodyScanPremiumClaude = "Ты - эксперт премиального сервиса биометрической аналитики тела. Верни ответ строго в формате JSON, без markdown и комментариев."
)

// ---------------------------------------------------------------------------
// Литералы, не попавшие в основные группы выше (json-mode суффикс OpenRouter,
// внутренние ошибки http-клиента / SOCKS5-туннеля).
// ---------------------------------------------------------------------------
const (
	// OpenRouter: суффикс-инструкция «только JSON», добавляемая к системному
	// промпту при jsonMode=true (в complete).
	PromptOpenRouterJSONModeSuffix = "\nВерни ответ строго в формате JSON, без markdown-разметки и комментариев."

	// httpclient: сообщение о превышении лимита запросов (429) внутри FetchWithRetry.
	ErrHTTPRateLimitExceeded = "rate limit exceeded"

	// httpclient: внутренние ошибки SOCKS5-туннеля (протокол уровня соединения).
	ErrSOCKS5HandshakeRejected = "socks5 handshake rejected: %v"
	ErrSOCKS5ConnectFailed     = "socks5 connect failed, rep=%d"
)
