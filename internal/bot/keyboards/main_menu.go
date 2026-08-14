package keyboards

import (
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// ProcessAnalysisMenu - меню с кнопками обработки и возврата
func ProcessAnalysisMenu() models.ReplyKeyboardMarkup {
	return models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: locales.BtnProcessAnalysis},
			},
			{
				{Text: locales.BtnBack},
			},
		},
		ResizeKeyboard: true,
	}
}

// StartMenu - меню после /start (до принятия соглашения)
func StartMenu() models.ReplyKeyboardMarkup {
	return models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: locales.BtnAgreement},
			},
			{
				{Text: locales.BtnAbout},
			},
		},
		ResizeKeyboard: true,
	}
}

// MainMenu - основное меню (после принятия соглашения).
// Разгружено: вместо 6 плоских кнопок — 4 понятных раздела-хаба в сетке 2×2.
// Каждый хаб при входе даёт описание и под-действия. 💎 Premium оставлен
// плоской кнопкой (это точка продажи, её важно держать на виду), остальные
// функции сгруппированы: «Анализы» (лаб. анализы + Bioscan), «Здоровье»
// (Сводка/Мониторинг/Консультация ИИ), «Сервис» (Отзывы/О сервисе).
func MainMenu() models.ReplyKeyboardMarkup {
	return models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: locales.BtnAnalysisHub},
				{Text: locales.BtnHealthHub},
			},
			{
				{Text: locales.BtnPremium},
				{Text: locales.BtnServiceHub},
			},
		},
		ResizeKeyboard: true,
	}
}

// AnalysisHubMenu - раздел-хаб «Анализы»: под-действия
// (Обычный / Расширенный анализ / Bioscan). Кнопка «Назад в меню» не нужна:
// основное меню (reply-клавиатура) и так всегда видно внизу экрана.
func AnalysisHubMenu() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: locales.BtnRegularAnalysis, CallbackData: "section_diag_regular"}},
			{{Text: locales.BtnExtendedAnalysis, CallbackData: "section_diag_extended"}},
			{{Text: locales.BtnBioscan, CallbackData: "section_bioscan"}},
		},
	}
}

// HealthHubMenu - раздел-хаб «Здоровье»: под-действия
// (Сводка здоровья / Мониторинг / Консультация ИИ). Кнопка «Назад в меню»
// не нужна: основное меню (reply-клавиатура) и так всегда видно внизу.
func HealthHubMenu() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: locales.BtnHealthSummary, CallbackData: "section_health_summary"}},
			{{Text: locales.BtnMonitoring, CallbackData: "section_health_monitoring"}},
			{{Text: locales.BtnConsultation, CallbackData: "section_consult_start"}},
		},
	}
}

// ServiceHubMenu - раздел-хаб «Сервис»: под-действия
// (Отзывы и предложения / О сервисе). Кнопка «Назад в меню» не нужна:
// основное меню (reply-клавиатура) и так всегда видно внизу экрана.
func ServiceHubMenu() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: locales.BtnFeedback, CallbackData: "section_feedback_start"}},
			{{Text: locales.BtnAbout, CallbackData: "section_about"}},
		},
	}
}

// UploadConfirm - меню подтверждения загрузки файлов
func UploadConfirm() models.ReplyKeyboardMarkup {
	return models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: locales.BtnProcessAnalysis},
				{Text: locales.BtnCancel},
			},
		},
		ResizeKeyboard:  true,
		OneTimeKeyboard: true,
	}
}

// BackMenu - меню с кнопкой "Назад"
func BackMenu() models.ReplyKeyboardMarkup {
	return models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: locales.BtnBack},
			},
		},
		ResizeKeyboard: true,
	}
}

// AgreementMenu - меню с кнопкой "Принять" для соглашения
func AgreementMenu() models.ReplyKeyboardMarkup {
	return models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: locales.BtnAcceptAgreement},
			},
		},
		ResizeKeyboard: true,
	}
}

// FeedbackMenu - меню режима ввода отзыва (Отмена / Назад).
func FeedbackMenu() models.ReplyKeyboardMarkup {
	return models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: locales.BtnCancel},
			},
			{
				{Text: locales.BtnBack},
			},
		},
		ResizeKeyboard: true,
	}
}

// UserAgreementText - текст пользовательского соглашения
func UserAgreementText() string {
	return locales.UserAgreementText
}
