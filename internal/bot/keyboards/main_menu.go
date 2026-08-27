package keyboards

import (
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// isDev - флаг окружения development. Управляется из bot.New через
// SetDevMode. Влияет на то, показываются ли dev--only элементы интерфейса
// (например, вход в тестовое меню уведомлений). В продакшене они скрыты.
var isDev bool

// SetDevMode задаёт окружение (development=true). Вызывается один раз при
// старте бота из bot.New.
func SetDevMode(dev bool) {
	isDev = dev
}

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

// MainMenu - основное меню (после принятия соглашения) как reply-клавиатура.
// Используется на РЕЗУЛЬТАТАХ флоу (анализ/биоскан/консультация и т.п.) и в
// опросниках - там нужна «всегда под рукой» нижняя клавиатура. Сама
// навигация по меню (главное меню/хабы/под-действия) идёт через inline и
// редактирует ОДНО сообщение на месте (см. MainMenuInline).
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

// MainMenuInline - основное меню как inline-кнопки. Одно сообщение
// (main_menu_msg_id) редактируется «на месте» при переходах главное меню
// <-> хаб <-> под-действие, поэтому новые сообщения в чате не плодятся.
func MainMenuInline() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: locales.BtnAnalysisHub, CallbackData: "section_analysis"},
				{Text: locales.BtnHealthHub, CallbackData: "section_health"},
			},
			{
				{Text: locales.BtnPremium, CallbackData: "premium_open"},
				{Text: locales.BtnServiceHub, CallbackData: "section_service"},
			},
		},
	}
}

// HubBackRow - inline-кнопка «Назад» для возврата из хаба в главное меню.
func HubBackRow() []models.InlineKeyboardButton {
	return []models.InlineKeyboardButton{
		{Text: locales.BtnBack, CallbackData: "back_to_main"},
	}
}

// BackInline - inline-кнопка «Назад» для возврата из экрана под-действия в
// хаб текущего раздела (Анализы/Здоровье/Сервис). Используется на экранах
// под-действий, премиум-заглушках и экранах ошибок (а также «О сервисе»):
// «Назад» возвращает именно в хаб раздела (через hub_back → backToParent),
// а не сразу в Главное меню - чтобы навигация была равномерной с
// BackCancelInline (у которого «Назад» тоже ведёт в хаб).
//
// ВНИМАНИЕ: callback - hub_back, НЕ back_to_main. back_to_main ведёт в
// Главное меню и предназначен только для кнопки «Назад» на уровне самого
// хаба (HubBackRow). Раньше BackInline ошибочно переиспользовал HubBackRow и
// прыгал в Главное меню - отсюда «неравномерный Назад» на части экранов.
func BackInline() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: locales.BtnBack, CallbackData: "hub_back"},
			},
		},
	}
}

// ConsultFinishMenu - reply-клавиатура ПОСЛЕ ответа консультации (режим
// StateWaitingConsultationFinish). Содержит ТОЛЬКО «Закончить консультацию»
// (+ «Задать ещё вопрос») - общего главного меню в этот момент НЕТ. Это
// гарантирует, что пользователь не может «случайно» выйти в главное меню
// (нажать «Анализы» и т.п.), пока не нажмёт «Закончить консультацию».
// Кнопки - reply (нижняя клавиатура), а НЕ inline, по требованию UX.
func ConsultFinishMenu() models.ReplyKeyboardMarkup {
	return models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: locales.BtnConsultAgain},
			},
			{
				{Text: locales.BtnConsultFinish},
			},
		},
		ResizeKeyboard: true,
	}
}

// BackCancelInline - inline-кнопки «Назад»/«Отмена» для экранов запуска флоу
// (анализ/биоскан/консультация/отзыв): «Назад» возвращает в хаб, «Отмена»
// прерывает флоу и возвращает в хаб текущего раздела.
func BackCancelInline() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: locales.BtnBack, CallbackData: "hub_back"},
				{Text: locales.BtnCancel, CallbackData: "cancel_flow"},
			},
		},
	}
}

// BackQuestionInline - inline-кнопка «Назад» для вопросов опросников
// (Bioscan PRO / базовый Bioscan / загрузка фото), где пользователь вводит
// текст. Inline-клавиатура НЕ блокирует ввод текста, поэтому «Назад»
// доступна параллельно с набором. Возврат на предыдущий вопрос - через
// callback bioscan_question_back (см. router.handleCallback), который
// вызывает backBioscanQuestionnaire. Так вся навигация остаётся ТОЛЬКО
// инлайн (без висящих reply-клавиатур).
func BackQuestionInline() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: locales.BtnBack, CallbackData: "bioscan_question_back"},
			},
		},
	}
}

// BackCancelQuestionInline - inline [Назад / ❌ Отмена] для опросника
// расширенного анализа. «Назад» -> questionnaire_back (предыдущий вопрос),
// «Отмена» -> cancel_flow (выход в хаб «Анализы»). Inline вместо reply -
// чтобы не плодить висящие reply-клавиатуры в чате.
func BackCancelQuestionInline() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: locales.BtnBack, CallbackData: "questionnaire_back"},
				{Text: locales.BtnCancel, CallbackData: "cancel_flow"},
			},
		},
	}
}

// BackCancelQuestionInlineBioscan - inline [Назад / ❌ Отмена] для опросника
// Bioscan PRO. «Назад» -> bioscan_question_back (предыдущий вопрос),
// «Отмена» -> cancel_flow. Аналог BackCancelQuestionInline, но «Назад»
// ведёт в опросник Bioscan (а не анализа).
func BackCancelQuestionInlineBioscan() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: locales.BtnBack, CallbackData: "bioscan_question_back"},
				{Text: locales.BtnCancel, CallbackData: "cancel_flow"},
			},
		},
	}
}

// AnalysisHubMenu - раздел-хаб «Анализы»: под-действия
// (Обычный / Расширенный анализ / Bioscan). Возврат в главное меню - по
// inline-кнопке «Назад» внизу (сообщение редактируется на месте).
func AnalysisHubMenu() models.InlineKeyboardMarkup {
	rows := [][]models.InlineKeyboardButton{
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
		HubBackRow(),
	}
	return models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// HealthHubMenu - раздел-хаб «Здоровье»: под-действия
// (Мой профиль / Консультация ИИ). Мониторинг теперь встроен прямо в
// «Мой профиль» (4-я вкладка внутри того же Web App).
func HealthHubMenu() models.InlineKeyboardMarkup {
	rows := [][]models.InlineKeyboardButton{
		{{Text: locales.BtnHealthSummary, CallbackData: "section_health_summary"}},
		{{Text: locales.BtnConsultation, CallbackData: "section_consult_start"}},
		HubBackRow(),
	}
	return models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// ServiceHubMenu - раздел-хаб «Сервис»: под-действия
// (Отзывы и предложения / О сервисе / 🧪 Тест уведомлений / Удаление аккаунта).
// Тестовое меню уведомлений - ТОЛЬКО в development.
func ServiceHubMenu() models.InlineKeyboardMarkup {
	rows := [][]models.InlineKeyboardButton{
		{{Text: locales.BtnFeedback, CallbackData: "section_feedback_start"}},
		{{Text: locales.BtnAbout, CallbackData: "section_about"}},
		{{Text: locales.BtnDeleteAccount, CallbackData: "section_delete_account"}},
	}
	if isDev {
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: locales.BtnTestNotify, CallbackData: "section_test_notify"},
		})
	}
	rows = append(rows, HubBackRow())
	return models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// DeleteAccountMenu - экран подтверждения удаления аккаунта (раздел
// «Сервис»): «Да, удалить» (необратимо) и «Отмена» (возврат в хаб Сервис).
func DeleteAccountMenu() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: "🗑 Да, удалить", CallbackData: "delete_account_confirm"}},
			{{Text: locales.BtnCancel, CallbackData: "delete_account_cancel"}},
		},
	}
}

// TestNotifyMenu - под-меню проверки уведомлений (раздел «Сервис», только
// в development).
func TestNotifyMenu() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: locales.BtnTestSub7d, CallbackData: "test_sub_7d"}},
			{{Text: locales.BtnTestSub3d, CallbackData: "test_sub_3d"}},
			{{Text: locales.BtnTestSub1d, CallbackData: "test_sub_1d"}},
			{{Text: locales.BtnTestSubToday, CallbackData: "test_sub_today"}},
			{{Text: locales.BtnTestAnalyticsCheck, CallbackData: "test_analytics_check"}},
			{{Text: locales.BtnTestAnalyticsSend, CallbackData: "test_analytics_send"}},
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

// BackMenu - меню с кнопкой "Назад" (reply). Используется на результатах
// флоу и в опросниках - там нужна нижняя клавиатура «всегда под рукой».
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

// ProfileConfirmMenu - inline-клавиатура экрана «Данные уже известны?»:
// «Использовать» подставляет сохранённый профиль (пропускает вопросы
// имя/возраст/пол/рост/вес), «Изменить» запускает опросник заново.
// Callback-данные profile_use / profile_change.
func ProfileConfirmMenu() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: locales.BtnProfileUse, CallbackData: "profile_use"},
				{Text: locales.BtnProfileChange, CallbackData: "profile_change"},
			},
		},
	}
}
