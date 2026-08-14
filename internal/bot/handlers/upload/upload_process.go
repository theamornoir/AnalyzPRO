package upload

import (
	"context"
	"fmt"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/service"
	"github.com/theamornoir/analyzpro/internal/storage"
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
	appStorage *storage.Storage,
	saver monitoring.HistorySaver,
) {
	fileData, err := file.readData()
	if err != nil {
		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg)
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
			renderAndSendReport(ctx, b, stateManager, reportRenderer, chatID, loadingMsg, textMsg, jsonResult, appStorage, saver)
			return
		}

		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg)
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
		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg)
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
	appStorage *storage.Storage,
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
		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg)
		return
	}

	combinedPayload := strings.Join(collectedTexts, "\n\n")

	if isExtended {
		jsonResult, err := analysisService.HandleAnalysisJSON(ctx, combinedPayload)
		if err == nil && jsonResult != "" {
			renderAndSendReport(ctx, b, stateManager, reportRenderer, chatID, loadingMsg, textMsg, jsonResult, appStorage, saver)
			return
		}

		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg)
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

// sendAnalysisError - отправляет сообщение об ошибке обработки анализа
// вместе с главным меню и сбрасывает состояние. Это гарантирует, что после
// неудачной обработки пользователь не «зависает» без клавиатуры и может
// вернуться в меню. Раньше ошибка отправлялась через helpers.SendError без
// клавиатуры и без сброса состояния — из-за этого пропадало меню и не было
// пути назад. loadingMsg/textMsg безопасно обрабатываются, если nil.
func sendAnalysisError(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64, loadingMsg, textMsg *models.Message) {
	deleteLoadingMessages(ctx, b, chatID, loadingMsg, textMsg)
	stateManager.Reset(chatID)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgTextProcessingError,
		ReplyMarkup: keyboards.MainMenu(),
	})
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
