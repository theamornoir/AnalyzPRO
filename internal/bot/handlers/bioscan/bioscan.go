package bioscan

import (
	"context"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// StartBioscanFlow - начало опроса для Bioscan.
func StartBioscanFlow(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64) {
	// Проверяем соглашение
	if stateManager.GetUserData(chatID, "agreement_accepted") != "yes" {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: keyboards.StartMenu(),
		})
		return
	}

	// Сбрасываем предыдущие данные Bioscan
	ResetBioscanData(stateManager, chatID)

	// Начинаем с вопроса об имени
	stateManager.SetState(chatID, states.StateWaitingBioscanName)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanStart,
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackMenu(),
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
}
