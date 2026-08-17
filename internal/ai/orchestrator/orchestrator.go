package orchestrator

import (
	"context"
	"fmt"
	"log"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// AIProvider - интерфейс для AI-провайдеров.
type AIProvider interface {
	// GenerateAnalysisSummary генерирует текстовый анализ по введённому тексту.
	GenerateAnalysisSummary(ctx context.Context, userInput string) (string, error)
	// GenerateAnalysisJSON генерирует JSON-структурированный анализ по тексту.
	GenerateAnalysisJSON(ctx context.Context, userInput string) (string, error)
	// GenerateAnalysisFromFileWithContext анализирует файл с учётом контекста.
	GenerateAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextText string) (string, error)
	// GenerateBioscanJSON генерирует JSON-результат bioscan по фотографиям.
	GenerateBioscanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error)
	// GenerateBodyScanJSON генерирует JSON премиального отчёта Bioscan PRO
	// (Body Intelligence) по фотографиям + данным опросника.
	GenerateBodyScanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error)
	// GenerateAnalysisFromFileJSON генерирует JSON-анализ из файла.
	GenerateAnalysisFromFileJSON(ctx context.Context, data []byte, mimeType string, contextText string) (string, error)
	// GenerateDossierJSON генерирует JSON универсального отчёта-досье
	// здоровья (на основе анализов + опросника об образе жизни).
	GenerateDossierJSON(ctx context.Context, userInput string) (string, error)
}

// Orchestrator - оркестратор AI-провайдеров с приоритетами.
type Orchestrator struct {
	providers []AIProvider
}

// NewOrchestrator создаёт оркестратор с провайдерами по умолчанию.
// Провайдеры добавляются только если их API-ключ не пустой.
func NewOrchestrator() *Orchestrator {
	var providers []AIProvider

	// OpenRouter - основной бесплатный провайдер: OpenAI-совместимый API без
	// гео-блоков, умеет text + vision (фото) + PDF (через извлечение текста).
	// Ставим первым, чтобы запросы шли через бесплатные модели OpenRouter.
	if p := NewOpenRouterProvider(); p != nil {
		providers = append(providers, p)
		log.Printf(locales.LogOrchestratorProviderAdd, "OpenRouter")
	}
	// YandexGPT - бесплатный провайдер с квотой (работает из РФ без гео-блоков).
	if p := NewYandexGPTProvider(); p != nil {
		providers = append(providers, p)
		log.Printf(locales.LogOrchestratorProviderAdd, "YandexGPT")
	}
	if p := NewGeminiProvider(); p != nil {
		providers = append(providers, p)
		log.Printf(locales.LogOrchestratorProviderAdd, "Gemini")
	}
	if p := NewDeepSeekProvider(); p != nil {
		providers = append(providers, p)
		log.Printf(locales.LogOrchestratorProviderAdd, "DeepSeek")
	}
	if p := NewClaudeProvider(); p != nil {
		providers = append(providers, p)
		log.Printf(locales.LogOrchestratorProviderAdd, "Claude")
	}

	if len(providers) == 0 {
		log.Printf(locales.LogOrchestratorNoProvider)
	}

	log.Printf(locales.LogOrchestratorTotal, len(providers))

	return &Orchestrator{
		providers: providers,
	}
}

// NewOrchestratorWithProviders создаёт оркестратор с указанными провайдерами.
func NewOrchestratorWithProviders(providers []AIProvider) *Orchestrator {
	return &Orchestrator{
		providers: providers,
	}
}

// tryProvider - пытается выполнить метод на провайдере, при ошибке переходит к следующему.
func (o *Orchestrator) tryProvider(
	ctx context.Context,
	fn func(provider AIProvider) (string, error),
) (string, error) {
	var lastErr error

	for i, provider := range o.providers {
		log.Printf(locales.LogOrchestratorTryProvider, i, providerName(provider))

		result, err := fn(provider)
		if err == nil {
			log.Printf(locales.LogOrchestratorProviderSuccess, providerName(provider))
			return result, nil
		}

		lastErr = err
		log.Printf(locales.LogOrchestratorProviderFailed, providerName(provider), err)
	}

	return "", fmt.Errorf(locales.ErrAllProvidersFailed, lastErr)
}

// GenerateAnalysisSummary - генерирует текстовый анализ.
func (o *Orchestrator) GenerateAnalysisSummary(ctx context.Context, userInput string) (string, error) {
	return o.tryProvider(ctx, func(p AIProvider) (string, error) {
		return p.GenerateAnalysisSummary(ctx, userInput)
	})
}

// GenerateAnalysisJSON - генерирует JSON-анализ.
func (o *Orchestrator) GenerateAnalysisJSON(ctx context.Context, userInput string) (string, error) {
	return o.tryProvider(ctx, func(p AIProvider) (string, error) {
		return p.GenerateAnalysisJSON(ctx, userInput)
	})
}

// GenerateAnalysisFromFileWithContext - анализирует файл с контекстом.
func (o *Orchestrator) GenerateAnalysisFromFileWithContext(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	return o.tryProvider(ctx, func(p AIProvider) (string, error) {
		return p.GenerateAnalysisFromFileWithContext(ctx, data, mimeType, contextText)
	})
}

// GenerateBioscanJSON - генерирует JSON bioscan.
func (o *Orchestrator) GenerateBioscanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	return o.tryProvider(ctx, func(p AIProvider) (string, error) {
		return p.GenerateBioscanJSON(ctx, photosData, mimeType, contextInfo)
	})
}

// GenerateBodyScanJSON - генерирует JSON премиального отчёта Bioscan PRO.
func (o *Orchestrator) GenerateBodyScanJSON(ctx context.Context, photosData [][]byte, mimeType string, contextInfo string) (string, error) {
	return o.tryProvider(ctx, func(p AIProvider) (string, error) {
		return p.GenerateBodyScanJSON(ctx, photosData, mimeType, contextInfo)
	})
}

// GenerateAnalysisFromFileJSON - генерирует JSON-анализ из файла.
func (o *Orchestrator) GenerateAnalysisFromFileJSON(ctx context.Context, data []byte, mimeType string, contextText string) (string, error) {
	return o.tryProvider(ctx, func(p AIProvider) (string, error) {
		return p.GenerateAnalysisFromFileJSON(ctx, data, mimeType, contextText)
	})
}

// GenerateDossierJSON - генерирует JSON универсального отчёта-досье здоровья.
func (o *Orchestrator) GenerateDossierJSON(ctx context.Context, userInput string) (string, error) {
	return o.tryProvider(ctx, func(p AIProvider) (string, error) {
		return p.GenerateDossierJSON(ctx, userInput)
	})
}

// providerName - возвращает имя провайдера для логов.
func providerName(p AIProvider) string {
	switch p.(type) {
	case *YandexGPTProvider:
		return "YandexGPT"
	case *OpenRouterProvider:
		return "OpenRouter"
	case *GeminiProvider:
		return "Gemini"
	case *DeepSeekProvider:
		return "DeepSeek"
	case *ClaudeProvider:
		return "Claude"
	default:
		return "Unknown"
	}
}
