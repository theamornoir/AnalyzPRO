package router

import (
	"context"
	"log"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/upload"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// handleUpload - обработка загрузки файлов и кнопки "✅ Обработать анализы".
// Возвращает true, если сообщение обработано.
func (r *router) handleUpload(ctx context.Context, b *tgbot.Bot, chatID int64, text string, update *models.Update) bool {
	state := r.stateManager.GetState(chatID)

	if text == locales.BtnProcessAnalysis {
		log.Printf(locales.LogUploadButtonPressed, chatID)
		upload.UploadHandler(
			r.stateManager,
			r.analysisService,
			r.reportRenderer,
			r.uploadDir,
			r.stickerID,
		)(ctx, b, update)
		return true
	}

	if state == states.StateWaitingAnalysisFile ||
		state == states.StateWaitingUploadConfirm ||
		len(update.Message.Photo) > 0 ||
		update.Message.Document != nil {
		log.Printf(locales.LogUploadHandleFile, chatID, state)
		upload.UploadHandler(
			r.stateManager,
			r.analysisService,
			r.reportRenderer,
			r.uploadDir,
			r.stickerID,
		)(ctx, b, update)
		return true
	}

	return false
}
