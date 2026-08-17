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
	"github.com/theamornoir/analyzpro/internal/report/pdfservice"
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
	pdfConverter pdfservice.Converter,
	chatID int64,
	loadingMsg *models.Message,
	textMsg *models.Message,
	file UploadedFile,
	isExtended bool,
	contextInfo string,
	appStorage *storage.Storage,
	saver monitoring.Repository,
) {
	fileData, err := file.readData()
	if err != nil {
		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg)
		return
	}

	if isExtended {
		// Расширенный анализ -> универсальное отчёт-досье здоровья.
		fileText, ferr := analysisService.HandleAnalysisFromFileWithContext(
			ctx,
			fileData,
			file.MimeType,
			contextInfo,
		)
		if ferr != nil || fileText == "" {
			sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg)
			return
		}
		combined := fileText + "\n\nДанные пациента и опросника об образе жизни:\n" + contextInfo
		// Сравнительный контекст: если ранее уже делали расширенный анализ —
		// подставляем предыдущий отчёт для СРАВНИТЕЛЬНОГО досье.
		if prevJSON, ok := monitoring.PreviousReportJSON(ctx, saver, chatID, "analysis"); ok {
			combined += locales.ComparisonContext(prevJSON, "analysis")
		}
		dossierJSON, derr := analysisService.HandleExtendedDossierJSON(ctx, combined)
		if derr != nil || dossierJSON == "" {
			sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg)
			return
		}
		renderAndSendDossier(ctx, b, stateManager, reportRenderer, pdfConverter, chatID, loadingMsg, textMsg, dossierJSON, appStorage, saver)
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
	pdfConverter pdfservice.Converter,
	chatID int64,
	loadingMsg *models.Message,
	textMsg *models.Message,
	files []UploadedFile,
	isExtended bool,
	contextInfo string,
	appStorage *storage.Storage,
	saver monitoring.Repository,
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
		// Расширенный анализ (несколько файлов) -> универсальное отчёт-досье.
		combined := strings.Join(collectedTexts, "\n\n")
		if combined == "" {
			sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg)
			return
		}
		combined += "\n\nДанные пациента и опросника об образе жизни:\n" + contextInfo
		// Сравнительный контекст: если ранее уже делали расширенный анализ —
		// подставляем предыдущий отчёт для СРАВНИТЕЛЬНОГО досье.
		if prevJSON, ok := monitoring.PreviousReportJSON(ctx, saver, chatID, "analysis"); ok {
			combined += locales.ComparisonContext(prevJSON, "analysis")
		}
		dossierJSON, derr := analysisService.HandleExtendedDossierJSON(ctx, combined)
		if derr != nil || dossierJSON == "" {
			sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg)
			return
		}
		renderAndSendDossier(ctx, b, stateManager, reportRenderer, pdfConverter, chatID, loadingMsg, textMsg, dossierJSON, appStorage, saver)
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
	sendAnalysisCompleteNote(ctx, b, stateManager, chatID, "")
}

// sendAnalysisCompleteNote - как sendAnalysisComplete, но дописывает блок
// дополнительной информации (extra). Используется для расширенного анализа/
// досье: в extra передаём запроса на сравнение с предыдущим отчётом и
// напоминание, что динамику видно в «Сводке здоровья».
func sendAnalysisCompleteNote(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64, extra string) {
	stateManager.Reset(chatID)
	text := locales.MsgAnalysisComplete
	if strings.TrimSpace(extra) != "" {
		text += "\n\n" + extra
	}
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: keyboards.MainMenu(),
		ParseMode:   "HTML",
	})
}

// buildReportNote собирает текст доп. блока для выдачи расширенного отчёта:
// напоминание о сравнении повторных отчётов + краткое сравнение (summary),
// если ИИ сформировал сравнительный отчёт. jsonResult — JSON отчёта.
func buildReportNote(jsonResult string) string {
	parts := []string{locales.MsgReportProgressNote}
	if s := monitoring.ParseComparisonSummary(jsonResult); s != "" {
		parts = append(parts, "📈 Сравнение с предыдущим отчётом: "+s)
	}
	return strings.Join(parts, "\n\n")
}
