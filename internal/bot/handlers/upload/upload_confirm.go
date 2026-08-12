package upload

import (
	"context"
	"log"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/service"
)

// handleUploadConfirm - обрабатывает подтверждение/отмену загрузки.
func handleUploadConfirm(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	uploadDir string,
	stickerID string,
	chatID int64,
	message *models.Message,
) {
	text := strings.TrimSpace(strings.ToLower(message.Text))

	if message.Document != nil {
		log.Printf(locales.LogUploadAddAnotherDoc)
		handleFileUpload(ctx, b, stateManager, uploadDir, chatID, message.Document)
		return
	}

	if message.Photo != nil {
		log.Printf(locales.LogUploadAddAnotherPhoto)
		handlePhotoUpload(ctx, b, stateManager, uploadDir, chatID, message.Photo)
		return
	}

	if text == locales.BtnProcessAnalysisLower || text == locales.BtnProcessAnalysisLowerShort {
		log.Printf(locales.LogUploadStartAnalysis)
		startAnalysis(ctx, b, stateManager, analysisService, reportRenderer, uploadDir, stickerID, chatID)
		return
	}

	if text == locales.BtnCancelLower || text == locales.BtnCancelLowerShort {
		log.Printf(locales.LogUploadCancel)
		stateManager.SetUserData(chatID, "uploaded_files", "")
		stateManager.SetUserData(chatID, "file_count", "")
		stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgUploadCancelled,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return
	}

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgUploadComplete,
		ReplyMarkup: keyboards.MainMenu(),
		ParseMode:   "HTML",
	})
}
