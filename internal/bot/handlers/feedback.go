package handlers

import (
	"context"
	"fmt"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
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
				Text:   "❌ Извините, функция отзывов временно недоступна.",
			})
			return
		}

		// Формируем сообщение для админа
		userInfo := fmt.Sprintf(
			"📨 **Новый отзыв/предложение!**\n\n"+
				"👤 **Пользователь:** %d\n"+
				"🆔 **Chat ID:** %d\n"+
				"📱 **Username:** @%s\n"+
				"📝 **Сообщение:**\n%s",
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
				Text:   "❌ Извините, не удалось отправить ваш отзыв. Попробуйте позже.",
			})
			return
		}

		// Подтверждение пользователю
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text: "✅ **Спасибо! Ваш отзыв получен!**\n\n" +
				"Я передал его разработчику. Ваше мнение очень важно для нас!\n" +
				"Если потребуется уточнение - мы свяжемся с вами.",
			ParseMode: "Markdown",
		})
	}
}
