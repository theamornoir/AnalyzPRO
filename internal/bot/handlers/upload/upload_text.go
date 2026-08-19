package upload

import (
	"context"
	"fmt"
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
	webAppURL string,
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

		jsonResult, err := analysisService.HandleAnalysisJSON(ctx, payload)
		if err == nil && jsonResult != "" {
			renderAndSendReport(ctx, b, stateManager, reportRenderer, pdfConverter, chatID, loadingMsg, textMsg, jsonResult, appStorage, saver, webAppURL)
			return
		}
	}

	result, err := analysisService.HandleAnalysisWithContext(ctx, payload, contextInfo)

	deleteLoadingMessages(ctx, b, chatID, loadingMsg, textMsg)

	if err != nil {
		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg, err)
		return
	}

	// Пустой результат без явной ошибки - тоже «неверный ответ» ИИ.
	if strings.TrimSpace(result) == "" {
		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg, fmt.Errorf("AI returned empty analysis for text input"))
		return
	}

	// Результат ИИ может быть длиннее лимита Telegram (4096 байт) -
	// отправляем с дроблением по 3500 байт, иначе сообщение не дойдёт,
	// хотя бот уже напишет «готово» и сохранит результат в профиль.
	helpers.SendLongMessagePlain(ctx, b, chatID, result)

	// Дополнительно запрашиваем у ИИ структурированные показатели (sections/
	// categories с indicators), чтобы наполнить блоки «Мой профиль»
	// РЕАЛЬНЫМИ значениями обычного текстового анализа. Не критично: при
	// ошибке сохраняем только текст (текущее поведение).
	indicatorsJSON, _ := analysisService.HandleAnalysisJSON(ctx, payload)

	// Сохраняем ОБЫЧНЫЙ анализ (текстом) в «Мой профиль».
	savePlainResult(ctx, saver, chatID, "analysis", locales.MsgUploadDefaultTitleAnalysis, result, indicatorsJSON)
	sendAnalysisComplete(ctx, b, stateManager, chatID)

	// Сообщаем, что результат сохранён в «Мой профиль», и даём
	// кнопку для мгновенного открытия.
	helpers.SendSavedToSummary(ctx, b, chatID, webAppURL)
}
