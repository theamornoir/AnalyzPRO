package menu

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/storage"
)

func StartHandler(
	stateManager states.StateManager,
	agreementStorage *storage.AgreementStorage, // <-- ДОБАВЛЕНО
) func(context.Context, *tgbot.Bot, *models.Update) {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		chatID := update.Message.Chat.ID

		// /start всегда освобождает «зависшее» состояние от прошлых сессий
		// (оно персистится в states.json между перезапусками бота), чтобы
		// пользователь начинал с чистого главного меню, а не из середины
		// старого потока bioscan/анкеты.
		stateManager.Reset(chatID)

		// Проверяем соглашение через постоянное хранилище
		if agreementStorage.IsAgreed(chatID) {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        locales.MsgStartWelcomeBack,
				ReplyMarkup: keyboards.MainMenu(),
				ParseMode:   "Markdown",
			})
			return
		}

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgStartWelcome,
			ReplyMarkup: keyboards.StartMenu(),
			ParseMode:   "Markdown",
		})
	}
}
