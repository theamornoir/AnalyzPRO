package gemini

import (
	"log"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// rateLimitFallback - сообщение при достижении лимита запросов.
func rateLimitFallback() string {
	log.Printf(locales.LogGeminiFallbackRateLimit)
	return locales.MsgFallbackRateLimit
}

// locationErrorFallback - сообщение при недоступности сервиса в регионе.
func locationErrorFallback() string {
	log.Printf(locales.LogGeminiFallbackLocationError)
	return locales.MsgFallbackLocation
}

// noKeyFallback - сообщение при отсутствии API-ключа.
func noKeyFallback() string {
	log.Printf(locales.LogGeminiFallbackNoKey)
	return locales.MsgFallbackNoKey
}

// serviceUnavailableFallback - сообщение при недоступности сервиса.
func serviceUnavailableFallback() string {
	log.Printf(locales.LogGeminiFallbackUnavailable)
	return locales.MsgFallbackServiceUnavailable
}
