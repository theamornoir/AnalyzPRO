package handlers

import (
	"context"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/service"
)

type AnalysisHandler struct {
	stateManager states.StateManager
	service      service.AnalysisService
}

func NewAnalysisHandler(stateManager states.StateManager, analysisService service.AnalysisService) *AnalysisHandler {
	return &AnalysisHandler{
		stateManager: stateManager,
		service:      analysisService,
	}
}

func (h *AnalysisHandler) Handle(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	if update.Message.Document != nil || update.Message.Photo != nil {
		state := h.stateManager.GetState(chatID)
		if state == states.StateWaitingAnalysisFile {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "✅ Анализ получен. Обрабатываю документ и сравниваю показатели…",
			})
			h.stateManager.Reset(chatID)
			return
		}
	}

	if strings.TrimSpace(update.Message.Text) == "" {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Пожалуйста, отправьте PDF или фотографию анализов.",
		})
		return
	}

	result, err := h.service.HandleAnalysis(ctx, update.Message.Text)
	if err != nil {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "⚠️ Не удалось обработать анализ. Попробуйте повторить запрос позже.",
		})
		return
	}

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   result,
	})
}
