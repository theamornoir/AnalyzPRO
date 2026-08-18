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
// Разгружено: вместо 6 плоских кнопок - 4 понятных раздела-хаба в сетке 2×2.
// Каждый хаб при входе даёт описание и под-действия. 💎 Premium оставлен
// плоской кнопкой (это точка продажи, её важно держать на виду), остальные
// функции сгруппированы: «Анализы» (лаб. анализы + Bioscan), «Здоровье»
// (Мой профиль/Мониторинг/Консультация ИИ), «Сервис» (Отзывы/О сервисе).
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
			{
				{Text: locales.BtnRegularAnalysis, CallbackData: "section_diag_regular"},
				{Text: locales.BtnRegularAnalysisDemo, CallbackData: "section_diag_regular_demo"},
			},
			{
				{Text: locales.BtnExtendedAnalysis, CallbackData: "section_diag_extended"},
				{Text: locales.BtnExtendedAnalysisDemo, CallbackData: "section_diag_extended_demo"},
			},
			{
				{Text: locales.BtnBioscanBasic, CallbackData: "section_bioscan_basic"},
				{Text: locales.BtnBioscanBasicDemo, CallbackData: "section_bioscan_basic_demo"},
			},
			{
				{Text: locales.BtnBioscanExtended, CallbackData: "section_bioscan_extended"},
				{Text: locales.BtnBioscanExtendedDemo, CallbackData: "section_bioscan_extended_demo"},
			},
			{
				{Text: locales.BtnExtendedAnalysisRepeatDemo, CallbackData: "section_diag_extended_demo2"},
				{Text: locales.BtnBioscanExtendedRepeatDemo, CallbackData: "section_bioscan_extended_demo2"},
			},
		},
	}
}

// HealthHubMenu - раздел-хаб «Здоровье»: под-действия
// (Мой профиль / Консультация ИИ). Кнопка «Назад в меню»
// не нужна: основное меню (reply-клавиатура) и так всегда видно внизу.
// Мониторинг теперь встроен прямо в «Мой профиль» (4-я вкладка
// внутри того же Web App), отдельной кнопки больше нет.
func HealthHubMenu() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: locales.BtnHealthSummary, CallbackData: "section_health_summary"}},
			{{Text: locales.BtnConsultation, CallbackData: "section_consult_start"}},
			{{Text: locales.BtnHealthSummaryDemo, CallbackData: "section_health_summary_demo"}},
		},
	}
}

// ServiceHubMenu - раздел-хаб «Сервис»: под-действия
// (Отзывы и предложения / О сервисе / 🧪 Тест уведомлений). Кнопка
// «Назад в меню» не нужна: основное меню (reply-клавиатура) и так всегда
// видно внизу экрана.
func ServiceHubMenu() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: locales.BtnFeedback, CallbackData: "section_feedback_start"}},
			{{Text: locales.BtnAbout, CallbackData: "section_about"}},
			{{Text: locales.BtnTestNotify, CallbackData: "section_test_notify"}},
		},
	}
}

// TestNotifyMenu - под-меню проверки уведомлений (раздел «Сервис»):
// кнопки планируют отправку реального образца уведомления через 30 секунд,
// а также «Назад» - возврат в хаб «Сервис».
func TestNotifyMenu() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: locales.BtnTestReminder, CallbackData: "test_notify_reminder"}},
			{{Text: locales.BtnTestMotivation, CallbackData: "test_notify_motivation"}},
			{{Text: locales.BtnTestFeature, CallbackData: "test_notify_feature"}},
			{{Text: locales.BtnBack, CallbackData: "test_notify_back"}},
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

// BackCancelMenu - меню анкеты: кнопки "Назад" и "❌ Отмена". Позволяет
// пользователю в любой момент выйти из опросника (❌ Отмена) или вернуться
// на предыдущий вопрос (Назад).
func BackCancelMenu() models.ReplyKeyboardMarkup {
	return models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: locales.BtnBack},
			},
			{
				{Text: locales.BtnCancel},
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
