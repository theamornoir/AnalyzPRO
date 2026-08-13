package upload

import (
	"context"
	"log"
	"strings"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/service"
)

// handleTextUpload - обрабатывает текст с показателями анализов.
func handleTextUpload(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	uploadDir string,
	stickerID string,
	chatID int64,
	text string,
	saver monitoring.HistorySaver,
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

	loadingMsg, textMsg := helpers.SendLoadingMessages(ctx, b, chatID, stickerID)

	if isExtended {
		log.Printf(locales.LogUploadExtendedTextAnalysis)

		jsonResult, err := analysisService.HandleAnalysisJSON(ctx, payload)
		if err == nil && jsonResult != "" {
			renderAndSendReport(ctx, b, stateManager, reportRenderer, chatID, loadingMsg, textMsg, jsonResult, saver)
			return
		}
	}

	result, err := analysisService.HandleAnalysisWithContext(ctx, payload, contextInfo)

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
