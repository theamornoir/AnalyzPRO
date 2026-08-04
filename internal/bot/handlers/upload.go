package handlers

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/theamornoir/analyzpro/internal/bot/states"
)

func UploadHandler(stateManager states.StateManager) func(context.Context, *tgbot.Bot, *models.Update) {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		chatID := update.Message.Chat.ID
		state := stateManager.GetState(chatID)
		if state != states.StateWaitingAnalysisFile {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "📄 Отправьте PDF-файл или фотографию анализов.",
			})
			return
		}

		if update.Message.Document != nil || update.Message.Photo != nil {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "✅ Анализ получен. Обработка скоро будет реализована.",
			})
			stateManager.Reset(chatID)
			return
		}

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Пожалуйста, отправьте PDF или фотографию анализов.",
		})
	}
}
