package locales

// Логи для оркестратора AI-провайдеров.
const (
	LogOrchestratorTryProvider   = "🔄 Orchestrator trying provider %d: %s"
	LogOrchestratorProviderSuccess = "✅ Orchestrator provider %s succeeded"
	LogOrchestratorProviderFailed  = "❌ Orchestrator provider %s failed: %v"
)

// Ошибки для оркестратора.
const (
	ErrAllProvidersFailed = "all AI providers failed (last error: %v)"
)
