package gemini

import (
	"log"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// noKeyFallback - сообщение при отсутствии API-ключа. Это НЕ маскируется под
// успешный ответ: gemini_file.go возвращает его текстом вместе с ошибкой
// (err != nil), чтобы оркестратор мог переключиться на другого провайдера.
func noKeyFallback() string {
	log.Printf(locales.LogGeminiFallbackNoKey)
	return locales.MsgFallbackNoKey
}
