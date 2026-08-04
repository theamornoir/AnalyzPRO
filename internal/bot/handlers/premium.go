package handlers

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func PremiumHandler() func(context.Context, *tgbot.Bot, *models.Update) {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "💎 Premium находится в разработке.",
		})
	}
}
