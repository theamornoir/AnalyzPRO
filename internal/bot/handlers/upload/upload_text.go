package upload

import (
	"context"
	"log"
	"strings"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/report/pdfservice"
	"github.com/theamornoir/analyzpro/internal/service"
	"github.com/theamornoir/analyzpro/internal/storage"
)

// handleTextUpload - обрабатывает текст с показателями анализов.
func handleTextUpload(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	pdfConverter pdfservice.Converter,
	uploadDir string,
	stickerID string,
	chatID int64,
	text string,
	appStorage *storage.Storage,
	saver monitoring.Repository,
) {
	payload := strings.TrimSpace(text)
	if payload == "" {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgUploadTextEmpty,
		})
		return
	}

	userData := stateManager.GetAllUserData(chatID)
	contextInfo := helpers.BuildAnalysisText(userData)

	isExtended := isExtendedAnalysis(stateManager, chatID)

	loadingMsg, textMsg := helpers.SendLoadingMessages(ctx, b, chatID, stickerID, nil)

	if isExtended {
		log.Printf(locales.LogUploadExtendedTextAnalysis)

		// Сравнительный контекст: если ранее уже делали расширенный анализ -
		// подставляем предыдущий отчёт, чтобы ИИ построил СРАВНИТЕЛЬНЫЙ
		// отчёт (динамика: что улучшилось / что улучшить), а не «с нуля».
		analysisPayload := payload
		if prevJSON, ok := monitoring.PreviousReportJSON(ctx, saver, chatID, "analysis"); ok {
			analysisPayload = locales.ComparisonContext(prevJSON, "analysis") + "\n\n" + payload
		}

		jsonResult, err := analysisService.HandleAnalysisJSON(ctx, analysisPayload)
		if err == nil && jsonResult != "" {
			renderAndSendReport(ctx, b, stateManager, reportRenderer, pdfConverter, chatID, loadingMsg, textMsg, jsonResult, appStorage, saver)
			return
		}
	}

	result, err := analysisService.HandleAnalysisWithContext(ctx, payload, contextInfo)

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
