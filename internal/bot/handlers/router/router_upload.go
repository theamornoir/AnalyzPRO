package router

import (
	"context"
	"log"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/upload"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
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
			r.appStorage,
			r.monitorRepo,
		)(ctx, b, update)
		return true
	}

	// Прислали файл/фото прямо из «пустого» состояния (например, открыли
	// раздел «📋 Анализы» и сразу прислали снимок, не нажимая
	// «Обычный анализ»). Сразу начинаем обычный анализ с этого файла,
	// вместо того чтобы просить «отправьте фото».
	if state == states.StateIdle &&
		(len(update.Message.Photo) > 0 || update.Message.Document != nil) {
		if !r.agreementStorage.IsAgreed(chatID) {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        locales.MsgBioscanAgreementRequired,
				ReplyMarkup: keyboards.StartMenu(),
			})
			return true
		}
		r.stateManager.SetUserData(chatID, "analysis_type", "regular")
		r.stateManager.SetUserData(chatID, "analysis_subtype", "regular")
		r.stateManager.SetState(chatID, states.StateWaitingAnalysisFile)
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
			r.appStorage,
			r.monitorRepo,
		)(ctx, b, update)
		return true
	}

	return false
}
