package handlers

import (
	"context"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/service"
)

func MessageRouter(stateManager states.StateManager, analysisService service.AnalysisService, uploadDir string) func(context.Context, *tgbot.Bot, *models.Update) {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		chatID := update.Message.Chat.ID
		state := stateManager.GetState(chatID)
		if state == states.StateWaitingAnalysisFile {
			UploadHandler(stateManager, analysisService, uploadDir)(ctx, b, update)
			return
		}

		text := strings.TrimSpace(update.Message.Text)
		if text == "" {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "Пожалуйста, отправьте текст или загрузите анализ.",
			})
			return
		}

		switch text {
		case "📤 Загрузить анализ":
			stateManager.SetState(chatID, states.StateWaitingAnalysisFile)
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "📄 Отправьте PDF-файл или фотографию анализов.",
			})
		case "📊 История":
			HistoryHandler()(ctx, b, update)
		case "💎 Premium":
			PremiumHandler()(ctx, b, update)
		case "ℹ️ О сервисе":
			AboutHandler()(ctx, b, update)
		default:
			result, err := analysisService.HandleAnalysis(ctx, text)
			if err != nil {
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: chatID,
					Text:   "⚠️ Не удалось обработать анализ. Попробуйте позже.",
				})
				return
			}

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   result,
			})
		}
	}
}
