package upload

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	apmodels "github.com/theamornoir/analyzpro/internal/models"
	"github.com/theamornoir/analyzpro/internal/report"
)

// renderAndSendReport - рендерит JSON-результат в HTML/PDF и отправляет пользователю.
// saver сохраняет результат в историю пользователя (для модуля Мониторинг).
func renderAndSendReport(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	reportRenderer *report.Renderer,
	chatID int64,
	loadingMsg *models.Message,
	textMsg *models.Message,
	jsonResult string,
	saver monitoring.HistorySaver,
) {
	cleanedJSON := cleanJSONReport(jsonResult)

	var reportData apmodels.Report
	if err := json.Unmarshal([]byte(cleanedJSON), &reportData); err != nil {
		log.Printf(locales.LogUploadJSONParseErr, err)
		log.Printf(locales.LogUploadReceivedText, cleanedJSON)

		deleteLoadingMessages(ctx, b, chatID, loadingMsg, textMsg)
		stateManager.Reset(chatID)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgUploadJSONError,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return
	}

	htmlResult, err := reportRenderer.Render(reportData)
	if err != nil {
		log.Printf(locales.LogUploadHTMLGenErr, err)

		deleteLoadingMessages(ctx, b, chatID, loadingMsg, textMsg)
		stateManager.Reset(chatID)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgUploadRenderError,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return
	}

	// Авто-сохранение результата в историю пользователя (для модуля Мониторинг).
	if saver != nil {
		title := monitoring.ExtractTitle(cleanedJSON, "Анализ")
		if saveErr := saver.SaveResult(ctx, &monitoring.HistoryEntry{
			TelegramID: chatID,
			Type:       "analysis",
			Title:      title,
			Date:       time.Now(),
			JsonData:   cleanedJSON,
			ReportHTML: htmlResult,
		}); saveErr != nil {
			log.Printf("[MONITORING] не удалось сохранить историю chatID=%d: %v", chatID, saveErr)
		} else {
			log.Printf("[MONITORING] история сохранена chatID=%d type=analysis", chatID)
		}
	}

	pdfData, pdfErr := report.ConvertHTMLToPDF(htmlResult)

	deleteLoadingMessages(ctx, b, chatID, loadingMsg, textMsg)

	if pdfErr == nil && len(pdfData) > 0 {
		_, _ = b.SendDocument(ctx, &tgbot.SendDocumentParams{
			ChatID: chatID,
			Document: &models.InputFileUpload{
				Filename: "Analysis_report.pdf",
				Data:     bytes.NewReader(pdfData),
			},
			Caption:   locales.MsgUploadReportCaption,
			ParseMode: "HTML",
		})
	} else {
		_, _ = b.SendDocument(ctx, &tgbot.SendDocumentParams{
			ChatID: chatID,
			Document: &models.InputFileUpload{
				Filename: "Analysis_report.html",
				Data:     bytes.NewReader([]byte(htmlResult)),
			},
			Caption:   locales.MsgUploadReportHTMLCaption,
			ParseMode: "HTML",
		})
	}

	sendAnalysisComplete(ctx, b, stateManager, chatID)
}

// cleanJSONReport - очищает текст JSON от markdown-обёртки.
func cleanJSONReport(jsonResult string) string {
	cleaned := strings.TrimSpace(jsonResult)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	return strings.TrimSpace(cleaned)
}
