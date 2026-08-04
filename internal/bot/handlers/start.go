package handlers

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
)

func StartHandler(stateManager states.StateManager) func(context.Context, *tgbot.Bot, *models.Update) {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		chatID := update.Message.Chat.ID
		stateManager.Reset(chatID)

		_, err := b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "🧬 Добро пожаловать в AnalyzPro!\n\nAnalyzPro помогает разобраться в медицинских анализах с помощью искусственного интеллекта.\n\nПоддерживаются:\n\n• PDF\n• фотографии анализов\n\nПосле обработки бот объяснит показатели простым языком и покажет возможные отклонения.\n\n⚠️ Важно\n\nAnalyzPro не заменяет врача и не ставит диагнозы.\n\nВыберите действие.",
			ReplyMarkup: keyboards.MainMenu(),
		})
		if err != nil {
			return
		}
	}
}
