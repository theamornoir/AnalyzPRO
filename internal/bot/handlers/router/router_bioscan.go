package router

import (
	"context"
	"log"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/bioscan"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// handleBioscanStates - обработка состояний Bioscan. Возвращает true, если обработано.
func (r *router) handleBioscanStates(ctx context.Context, b *tgbot.Bot, chatID int64, text string, update *models.Update) bool {
	state := r.stateManager.GetState(chatID)

	switch state {
	case states.StateWaitingBioscanName:
		log.Printf(locales.LogProcessingBioscanName, chatID)
		bioscan.HandleBioscanName(ctx, b, r.stateManager, chatID, text)
		return true

	case states.StateWaitingBioscanAge:
		log.Printf(locales.LogProcessingBioscanAge, chatID)
		bioscan.HandleBioscanAge(ctx, b, r.stateManager, chatID, text)
		return true

	case states.StateWaitingBioscanHeight:
		log.Printf(locales.LogProcessingBioscanHeight, chatID)
		bioscan.HandleBioscanHeight(ctx, b, r.stateManager, chatID, text)
		return true

	case states.StateWaitingBioscanWeight:
		log.Printf(locales.LogProcessingBioscanWeight, chatID)
		bioscan.HandleBioscanWeight(ctx, b, r.stateManager, chatID, text)
		return true

	case states.StateWaitingBioscanGoal:
		log.Printf(locales.LogProcessingBioscanGoal, chatID)
		bioscan.HandleBioscanGoal(ctx, b, r.stateManager, chatID, text)
		return true

	case states.StateWaitingBioscanPhoto1,
		states.StateWaitingBioscanPhoto2,
		states.StateWaitingBioscanPhoto3,
		states.StateWaitingBioscanPhoto4:

		log.Printf(locales.LogRouterBioscanPhoto, state, chatID)
		if len(update.Message.Photo) > 0 {
			bioscan.HandleBioscanPhoto(ctx, b, r.stateManager, chatID, update.Message.Photo)
		} else {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   locales.MsgBioscanPhotoRequired,
			})
		}
		return true

	case states.StateWaitingBioscanConfirm:
		log.Printf(locales.LogRouterBioscanConfirm, chatID, text)
		switch text {
		case locales.BtnBioscanConfirm:
			bioscan.ProcessBioscanWithPhotos(ctx, b, r.stateManager, r.analysisService, r.uploadDir, r.stickerID, chatID)
		case locales.BtnBioscanRestart:
			bioscan.StartBioscanFlow(ctx, b, r.stateManager, chatID)
		default:
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   locales.MsgBioscanConfirmAction,
			})
		}
		return true
	}

	return false
}
