package handlers

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func AboutHandler() func(context.Context, *tgbot.Bot, *models.Update) {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text: "ℹ️ О сервисе\n\nAnalyzPro помогает анализировать медицинские показатели и объяснять их простым языком.\n\n• бот использует искусственный интеллект для интерпретации данных;\n• анализ не является медицинским заключением;\n• сервис не заменяет врача и не ставит диагнозы;\n• бот поддерживает PDF и фотографии анализов.",
		})
	}
}
