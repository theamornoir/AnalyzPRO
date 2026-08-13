package upload

import (
	"context"
	"fmt"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/service"
)

// processSingleFile - обрабатывает один файл.
func processSingleFile(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	chatID int64,
	loadingMsg *models.Message,
	textMsg *models.Message,
	file UploadedFile,
	isExtended bool,
	contextInfo string,
	saver monitoring.HistorySaver,
) {
	fileData, err := file.readData()
	if err != nil {
		helpers.SendError(ctx, b, chatID)
		return
	}

	if isExtended {
		jsonResult, err := analysisService.HandleAnalysisFromFileJSON(
			ctx,
			fileData,
			file.MimeType,
			contextInfo,
		)

		if err == nil && jsonResult != "" {
			renderAndSendReport(ctx, b, stateManager, reportRenderer, chatID, loadingMsg, textMsg, jsonResult, saver)
			return
		}

		deleteLoadingMessages(ctx, b, chatID, loadingMsg, textMsg)
		helpers.SendError(ctx, b, chatID)
		return
	}

	result, err := analysisService.HandleAnalysisFromFileWithContext(
		ctx,
		fileData,
		file.MimeType,
		contextInfo,
	)

	deleteLoadingMessages(ctx, b, chatID, loadingMsg, textMsg)

	if err != nil {
		helpers.SendError(ctx, b, chatID)
		return
	}

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   result,
	})

	sendAnalysisComplete(ctx, b, stateManager, chatID)
}

// processMultipleFiles - обрабатывает несколько файлов.
func processMultipleFiles(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	chatID int64,
	loadingMsg *models.Message,
	textMsg *models.Message,
	files []UploadedFile,
	isExtended bool,
	contextInfo string,
	saver monitoring.HistorySaver,
) {
	var collectedTexts []string

	for i, file := range files {
		fileData, err := file.readData()
		if err != nil {
			continue
		}

		res, err := analysisService.HandleAnalysisFromFileWithContext(
			ctx,
			fileData,
			file.MimeType,
			contextInfo,
		)
		if err == nil && res != "" {
			collectedTexts = append(collectedTexts, fmt.Sprintf("=== Данные из файла %d (%s) ===\n%s", i+1, file.FileName, res))
		}
	}

	if len(collectedTexts) == 0 {
		deleteLoadingMessages(ctx, b, chatID, loadingMsg, textMsg)
		helpers.SendError(ctx, b, chatID)
		return
	}

	combinedPayload := strings.Join(collectedTexts, "\n\n")

	if isExtended {
		jsonResult, err := analysisService.HandleAnalysisJSON(ctx, combinedPayload)
		if err == nil && jsonResult != "" {
			renderAndSendReport(ctx, b, stateManager, reportRenderer, chatID, loadingMsg, textMsg, jsonResult, saver)
			return
		}

		deleteLoadingMessages(ctx, b, chatID, loadingMsg, textMsg)
		helpers.SendError(ctx, b, chatID)
		return
	}

	deleteLoadingMessages(ctx, b, chatID, loadingMsg, textMsg)

	finalResult := fmt.Sprintf(locales.MsgUploadResultFiles, len(files), combinedPayload)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    chatID,
		Text:      finalResult,
		ParseMode: "HTML",
	})

	sendAnalysisComplete(ctx, b, stateManager, chatID)
}

// deleteLoadingMessages - удаляет сообщения о загрузке.
func deleteLoadingMessages(ctx context.Context, b *tgbot.Bot, chatID int64, loadingMsg, textMsg *models.Message) {
	if loadingMsg != nil {
		helpers.DeleteMessage(ctx, b, chatID, loadingMsg.ID)
	}
	if textMsg != nil {
		helpers.DeleteMessage(ctx, b, chatID, textMsg.ID)
	}
}

// sendAnalysisComplete - отправляет сообщение о завершении анализа и сбрасывает состояние.
func sendAnalysisComplete(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64) {
	stateManager.Reset(chatID)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgAnalysisComplete,
		ReplyMarkup: keyboards.MainMenu(),
		ParseMode:   "HTML",
	})
}
