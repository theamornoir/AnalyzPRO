package handlers

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func AboutHandler() func(context.Context, *tgbot.Bot, *models.Update) {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		chatID := update.Message.Chat.ID
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text: `ℹ️ О сервисе AnalyzPro

🤖 AnalyzPro — это Telegram-бот, который помогает разобраться в медицинских анализах с помощью искусственного интеллекта.

⚠️ Важно:
• Бот НЕ ставит диагнозы
• Результаты носят информационный характер
• Всегда консультируйтесь с врачом`,
		})
	}
}
