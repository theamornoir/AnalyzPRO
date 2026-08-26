package bioscan

import (
	"context"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// StartBioscanFlow - начало опроса для Bioscan (используется кнопкой
// «Начать заново» на экране подтверждения Bioscan PRO).
// Проверка принятия соглашения выполняется вызывающим
// (router через agreementStorage.IsAgreed), поэтому здесь она не дублируется.
func StartBioscanFlow(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64) {
	// Сбрасываем предыдущие данные Bioscan
	ResetBioscanData(stateManager, chatID)

	// Начинаем с вопроса об имени
	stateManager.SetState(chatID, states.StateWaitingBioscanName)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanStart,
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackQuestionInline(),
	})
}

// ResetBioscanData очищает все данные bioscan для пользователя.
func ResetBioscanData(sm states.StateManager, chatID int64) {
	sm.SetUserData(chatID, "bioscan_photo_count", "0")
	sm.SetUserData(chatID, "bioscan_name", "")
	sm.SetUserData(chatID, "bioscan_age", "")
	sm.SetUserData(chatID, "bioscan_height", "")
	sm.SetUserData(chatID, "bioscan_weight", "")
	sm.SetUserData(chatID, "bioscan_goal", "")
	sm.SetUserData(chatID, "bioscan_photo1", "")
	sm.SetUserData(chatID, "bioscan_photo2", "")
	sm.SetUserData(chatID, "bioscan_photo3", "")
	sm.SetUserData(chatID, "bioscan_photo4", "")
	// Очищаем данные мини-опросника базового Bioscan (пол/возраст/рост/
	// вес/цель/тренировки и текущий шаг). Без этого устаревшие ответы
	// после «Назад»/отмены не могли бы испортить новый опросник.
	sm.SetUserData(chatID, "bioscan_basic_step", "")
	sm.SetUserData(chatID, "bioscan_basic_gender", "")
	sm.SetUserData(chatID, "bioscan_basic_age", "")
	sm.SetUserData(chatID, "bioscan_basic_height", "")
	sm.SetUserData(chatID, "bioscan_basic_weight", "")
	sm.SetUserData(chatID, "bioscan_basic_goal", "")
	sm.SetUserData(chatID, "bioscan_basic_activity", "")
	// Очищаем FileID и ID сообщения присланного фото базового Bioscan
	// (шаг с фото теперь снова активен как первый шаг потока).
	sm.SetUserData(chatID, "bioscan_basic_photo_fileid", "")
	sm.SetUserData(chatID, "bioscan_basic_photo_msgid", "")
	// Очищаем список ID исходных фото-сообщений (трекаются для удаления
	// из чата после обработки Bioscan).
	sm.SetUserData(chatID, bioscanMsgIDsKey, "")
	// Очищаем ответы опросника Bioscan PRO (образ жизни / спорт / здоровье).
	for _, key := range bioscanQuestionnaireKeys() {
		sm.SetUserData(chatID, key, "")
	}
}
