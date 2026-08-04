package handlers

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func HistoryHandler() func(context.Context, *tgbot.Bot, *models.Update) {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "📊 История анализов пока недоступна.\nОна появится после подключения базы данных.",
		})
	}
}
