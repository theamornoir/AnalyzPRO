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
// Сгруппировано в разделы, чтобы главный экран не был перегружен; каждый
// раздел-хаб при входе даёт описание и под-действия. Bioscan и «Отзывы и
// предложения» вынесены отдельными кнопками (Bioscan — это не лабораторный
// анализ, а фотографический анализ тела; отзывы — прямая связь с разработчиком).
func MainMenu() models.ReplyKeyboardMarkup {
	return models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: locales.BtnDiagnostics},
			},
			{
				{Text: locales.BtnBioscan},
			},
			{
				{Text: locales.BtnHealthDynamics},
			},
			{
				{Text: locales.BtnConsultation},
			},
			{
				{Text: locales.BtnFeedback},
			},
			{
				{Text: locales.BtnPremium},
				{Text: locales.BtnAbout},
			},
		},
		ResizeKeyboard: true,
	}
}

// DiagnosticsHubMenu - раздел «Диагностика»: описание + под-действия
// (Обычный анализ / Расширенный анализ) + назад в меню. Bioscan здесь
// намеренно отсутствует — он вынесен отдельной кнопкой в главном меню.
func DiagnosticsHubMenu() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: locales.BtnRegularAnalysis, CallbackData: "section_diag_regular"}},
			{{Text: locales.BtnExtendedAnalysis, CallbackData: "section_diag_extended"}},
			{{Text: "⬅️ Назад в меню", CallbackData: "back_main"}},
		},
	}
}

// HealthDynamicsHubMenu - раздел «Здоровье в динамике»: под-действия
// (Сводка здоровья / Мониторинг) + назад в меню.
func HealthDynamicsHubMenu() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: locales.BtnHealthSummary, CallbackData: "section_health_summary"}},
			{{Text: locales.BtnMonitoring, CallbackData: "section_health_monitoring"}},
			{{Text: "⬅️ Назад в меню", CallbackData: "back_main"}},
		},
	}
}

// ConsultationHubMenu - раздел «Быстрая консультация»: описание + действие
// «Начать консультацию» + назад в меню. Сама проверка соглашения/Premium и
// бесплатной квоты остаётся внутри действия (handleConsultationStart).
func ConsultationHubMenu() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: "💬 Начать консультацию", CallbackData: "section_consult_start"}},
			{{Text: "⬅️ Назад в меню", CallbackData: "back_main"}},
		},
	}
}

// AnalysisTypeMenu - меню выбора типа анализа
func AnalysisTypeMenu() models.ReplyKeyboardMarkup {
	return models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: locales.BtnRegularAnalysis},
				{Text: locales.BtnExtendedAnalysis},
			},
			{
				{Text: locales.BtnBack},
			},
		},
		ResizeKeyboard: true,
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
