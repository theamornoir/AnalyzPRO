package router

import (
	"context"
	"log"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// handleText - обработка обычного текста через AI.
func (r *router) handleText(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	log.Printf(locales.LogRouterTextProcessing, chatID)
	result, err := r.analysisService.HandleAnalysis(ctx, text)
	if err != nil {
		log.Printf(locales.LogRouterTextError, chatID, err)
		r.stateManager.SetState(chatID, states.StateIdle)

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgTextProcessingError,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return
	}

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   result,
	})
}
