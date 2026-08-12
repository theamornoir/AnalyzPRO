package mock

import "github.com/theamornoir/analyzpro/internal/locales"

// MockBioscanJSON - мок-ответ JSON для bioscan.
func MockBioscanJSON(_ string) string {
	return locales.MockBioscanJSONText
}
