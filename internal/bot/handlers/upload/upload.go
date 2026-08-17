package upload

import (
	"context"
	"log"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/report/pdfservice"
	"github.com/theamornoir/analyzpro/internal/service"
	"github.com/theamornoir/analyzpro/internal/storage"
)

// UploadHandler - главный обработчик загрузки файлов и запуска анализа.
// saver сохраняет готовый результат в историю пользователя (для Мониторинга).
// appStorage персистит результат анализа как Diagnosis (Storage).
// webAppURL используется для кнопки «Открыть Сводку здоровья» после отчёта.
func UploadHandler(
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	pdfConverter pdfservice.Converter,
	uploadDir string,
	stickerID string,
	appStorage *storage.Storage,
	saver monitoring.Repository,
	webAppURL string,
) func(context.Context, *tgbot.Bot, *models.Update) {

	return func(
		ctx context.Context,
		b *tgbot.Bot,
		update *models.Update,
	) {

		if update.Message == nil {
			return
		}

		chatID := update.Message.Chat.ID
		log.Printf(locales.LogUploadMessageReceived,
			chatID, update.Message.Text, update.Message.Photo != nil, update.Message.Document != nil)

		state := stateManager.GetState(chatID)
		log.Printf(locales.LogUploadCurrentState, state)

		if state == states.StateWaitingUploadConfirm {
			log.Printf(locales.LogUploadWaitingConfirm)
			handleUploadConfirm(ctx, b, stateManager, analysisService, reportRenderer, pdfConverter, uploadDir, stickerID, chatID, update.Message, appStorage, saver, webAppURL)
			return
		}

		if state != states.StateWaitingAnalysisFile {
			log.Printf(locales.LogUploadNotWaitingState)
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   locales.MsgUploadRequest,
			})
			return
		}

		if update.Message.Document != nil {
			log.Printf(locales.LogUploadHandlingDoc)
			handleFileUpload(ctx, b, stateManager, uploadDir, chatID, update.Message.Document)
			return
		}

		if update.Message.Photo != nil {
			log.Printf(locales.LogUploadHandlingPhoto)
			handlePhotoUpload(ctx, b, stateManager, uploadDir, chatID, update.Message.Photo)
			return
		}

		if update.Message.Text != "" {
			log.Printf(locales.LogProcessingUploadText, update.Message.Text)
			handleTextUpload(ctx, b, stateManager, analysisService, reportRenderer, pdfConverter, uploadDir, stickerID, chatID, update.Message.Text, appStorage, saver, webAppURL)
			return
		}

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgUploadInvalidFiles,
		})
	}
}
