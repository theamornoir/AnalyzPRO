package menu

import (
	"context"
	"fmt"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// FeedbackHandler - обработчик для отзывов и предложений
func FeedbackHandler(adminChatID int64) func(context.Context, *tgbot.Bot, *models.Update) {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		chatID := update.Message.Chat.ID
		text := update.Message.Text

		// Проверяем, что adminChatID задан
		if adminChatID == 0 {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   locales.MsgFeedbackUnavailable,
			})
			return
		}

		// Формируем сообщение для админа
		userInfo := fmt.Sprintf(locales.MsgFeedbackSentToAdmin,
			update.Message.From.ID,
			chatID,
			update.Message.From.Username,
			text,
		)

		// Отправляем админу
		_, err := b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:    adminChatID,
			Text:      userInfo,
			ParseMode: "Markdown",
		})

		if err != nil {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   locales.MsgFeedbackSendError,
			})
			return
		}

		// Подтверждение пользователю
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:    chatID,
			Text:      locales.MsgFeedbackConfirmed,
			ParseMode: "Markdown",
		})
	}
}
