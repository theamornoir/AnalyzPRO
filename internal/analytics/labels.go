package analytics

import (
	"fmt"
	"strings"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// callbackLabels - человекочитаемые русские подписи для callback_data
// inline-кнопок. Используются как название события в PostHog, чтобы в
// дашборде было видно не "section_analysis", а "Открыл раздел Анализы".
var callbackLabels = map[string]string{
	"section_analysis":               "Открыл раздел Анализы",
	"section_health":                 "Открыл раздел Здоровье",
	"section_service":                "Открыл раздел Сервис",
	"section_diag_regular":           "Выбрал Обычный анализ",
	"section_diag_extended":          "Выбрал Расширенный анализ",
	"section_bioscan_basic":          "Выбрал Bioscan (базовый)",
	"section_bioscan_extended":       "Выбрал Bioscan PRO",
	"section_diag_regular_demo":      "Открыл Демо: Обычный анализ",
	"section_diag_extended_demo":     "Открыл Демо: Расширенный анализ",
	"section_bioscan_basic_demo":     "Открыл Демо: Bioscan",
	"section_bioscan_extended_demo":  "Открыл Демо: Bioscan PRO",
	"section_diag_extended_demo2":    "Открыл Демо: повтор. анализ",
	"section_bioscan_extended_demo2": "Открыл Демо: повтор. Bioscan",
	"section_health_summary":         "Открыл Сводку здоровья",
	"section_health_summary_demo":    "Открыл Демо-Сводку",
	"section_health_monitoring":      "Открыл Мониторинг",
	"section_health_monitoring_demo": "Открыл Демо-Мониторинг",
	"section_consult_start":          "Открыл Быструю консультацию",
	"section_feedback_start":         "Открыл Отзывы",
	"section_about":                  "Открыл О сервисе",
	"upload_process":                 "Нажал Обработать анализы",
	"upload_cancel":                  "Отменил загрузку анализа",
	"bioscan_confirm":                "Подтвердил Bioscan",
	"bioscan_restart":                "Перезапустил Bioscan",
	"hub_back":                       "Нажал Назад",
	"msg_back":                       "Нажал Назад",
	"premium_change":                 "Смена тарифа Premium",
}

// buttonLabels - подписи для reply-кнопок (текст кнопки → русская подпись).
// Reply-кнопки приходят как сообщения (source="message", action=текст
// кнопки), поэтому мапим их точный текст на понятную подпись. Ключи берём из
// locales, чтобы не рассинхронизироваться с реальными надписями кнопок.
var buttonLabels = map[string]string{
	locales.BtnAnalysisHub:      "Открыл раздел Анализы",
	locales.BtnHealthHub:        "Открыл раздел Здоровье",
	locales.BtnPremium:          "Открыл Premium",
	locales.BtnServiceHub:       "Открыл раздел Сервис",
	locales.BtnRegularAnalysis:  "Выбрал Обычный анализ",
	locales.BtnExtendedAnalysis: "Выбрал Расширенный анализ",
	locales.BtnBioscan:          "Выбрал Bioscan",
	locales.BtnBioscanBasic:     "Выбрал Bioscan (базовый)",
	locales.BtnBioscanExtended:  "Выбрал Bioscan PRO",
	locales.BtnProcessAnalysis:  "Нажал Обработать анализы",
	locales.BtnBack:             "Нажал Назад",
	locales.BtnCancel:           "Нажал Отмена",
	locales.BtnAgreement:        "Открыл соглашение",
	locales.BtnAbout:            "Открыл О сервисе",
	locales.BtnAcceptAgreement:  "Принял соглашение",
}

// semanticEventLabels - подписи для предметных событий (ключ → русская
// подпись). Трекаются в конкретных хендлерах (старт, анализ, биоскан,
// премиум, сводка, покупка премиум). Название события в PostHog = русская
// подпись, поэтому в дашборде видно сразу понятный текст, а не ключ.
var semanticEventLabels = map[string]string{
	"user_started":       "Запустил бота",
	"analysis_processed": "Анализ обработан",
	"bioscan_completed":  "Bioscan завершён",
	"premium_view":       "Открыл Premium",
	"premium_purchased":  "Купил Premium",
	"dashboard_opened":   "Открыл Сводку здоровья",
}

// interactionLabel возвращает человекочитаемую русскую подпись события
// «interaction» по source/action. action - это callback_data (для inline-
// кнопок), текст команды (для /команд) или текст reply-кнопки/сообщения.
// Динамические префиксы (premium_*, onboarding_*) раскрываются в понятные
// фразы. Свободный текст/медиа без кнопки сворачивается в одну подпись
// «Свободное сообщение» (сам текст остаётся в свойстве action).
func interactionLabel(source, action string) string {
	switch source {
	case "callback":
		if l, ok := callbackLabels[action]; ok {
			return l
		}
		if strings.HasPrefix(action, "premium_confirm_") {
			return "Подтвердил оплату Premium"
		}
		if strings.HasPrefix(action, "premium_") {
			return "Выбрал тариф Premium"
		}
		if strings.HasPrefix(action, "onboarding_step_") {
			var n int
			if _, err := fmt.Sscanf(action, "onboarding_step_%d", &n); err == nil {
				return fmt.Sprintf("Онбординг: шаг %d", n)
			}
			return "Онбординг: шаг"
		}
		switch action {
		case "onboarding_agreement":
			return "Онбординг: соглашение"
		case "onboarding_accept":
			return "Принял соглашение"
		}
		return "Нажатие кнопки: " + action
	case "command":
		return "Команда " + action
	default: // message
		if l, ok := buttonLabels[action]; ok {
			return l
		}
		if strings.TrimSpace(action) != "" {
			return "Свободное сообщение"
		}
		return "Сообщение пользователя"
	}
}
