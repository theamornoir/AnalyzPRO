package handlers

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
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

		// Проверяем соглашение через постоянное хранилище
		if agreementStorage.IsAgreed(chatID) {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text: `👋 **Добро пожаловать обратно в AnalyzPRO!**

Выберите действие:`,
				ReplyMarkup: keyboards.MainMenu(),
				ParseMode:   "Markdown",
			})
			return
		}

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text: `👋 **Добро пожаловать в AnalyzPRO!**

Я помогу вам разобраться в медицинских анализах.

🔬 **Что я умею:**
• Анализировать PDF и фото с результатами
• Объяснять показатели простым языком
• Выделять отклонения от нормы
• Давать рекомендации

⚠️ **Важно:** Я НЕ ставлю диагнозы и НЕ заменяю врача!

📝 Для начала работы примите пользовательское соглашение.`,
			ReplyMarkup: keyboards.StartMenu(),
			ParseMode:   "Markdown",
		})
	}
}
