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

	"github.com/theamornoir/analyzpro/internal/analytics"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	apmodels "github.com/theamornoir/analyzpro/internal/models"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/report/pdfservice"
	"github.com/theamornoir/analyzpro/internal/storage"
)

// renderAndSendReport - рендерит JSON-результат в HTML/PDF и отправляет пользователю.
// saver сохраняет результат в историю пользователя (для модуля Мониторинг).
// appStorage персистит результат как Diagnosis (для профиля пользователя).
func renderAndSendReport(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	reportRenderer *report.Renderer,
	pdfConverter pdfservice.Converter,
	chatID int64,
	loadingMsg *models.Message,
	textMsg *models.Message,
	jsonResult string,
	appStorage *storage.Storage,
	saver monitoring.Repository,
	webAppURL string,
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
		title := monitoring.ExtractTitle(cleanedJSON, locales.MsgUploadDefaultTitleAnalysis)
		if saveErr := saver.SaveResult(ctx, &monitoring.HistoryEntry{
			TelegramID: chatID,
			Type:       "analysis",
			Title:      title,
			Date:       time.Now(),
			JsonData:   cleanedJSON,
			ReportHTML: htmlResult,
		}); saveErr != nil {
			log.Printf(locales.LogUploadMonitoringSaveErr, chatID, saveErr)
		} else {
			log.Printf(locales.LogUploadMonitoringSaved, chatID)
		}
	}

	// Персистим результат как Diagnosis (для профиля/истории пользователя).
	if appStorage != nil {
		if derr := appStorage.SaveDiagnosisForUser(ctx, chatID, "analysis", cleanedJSON, htmlResult); derr != nil {
			log.Printf(locales.LogUploadStorageSaveErr, chatID, derr)
		} else {
			log.Printf(locales.LogUploadStorageSaved, chatID)
		}
	}

	analytics.EmitEvent(ctx, analytics.Event{
		Type:       analytics.EventAnalysis,
		TelegramID: chatID,
		Meta:       map[string]interface{}{"title": monitoring.ExtractTitle(cleanedJSON, locales.MsgUploadDefaultTitleAnalysis)},
	})

	// PostHog: успешная обработка анализа (расширенный/обычный).
	aType := "regular"
	if isExtendedAnalysis(stateManager, chatID) {
		aType = "extended"
	}
	analytics.Track(chatID, "analysis_processed", map[string]interface{}{
		"analysis_type": aType,
	})

	deleteLoadingMessages(ctx, b, chatID, loadingMsg, textMsg)

	// Расширенный анализ конвертируем в PDF и отправляем как PDF-документ.
	// При сбое конвертации (нет ключа html2pdf.app / сервис недоступен) -
	// откат к HTML, чтобы результат не потерялся.
	pdfBytes, convErr := pdfConverter.ConvertHTML(ctx, htmlResult)
	if convErr != nil {
		log.Printf("⚠️ [UPLOAD] не удалось конвертировать анализ в PDF (chatID=%d): %v - отправляю HTML", chatID, convErr)
		_, _ = b.SendDocument(ctx, &tgbot.SendDocumentParams{
			ChatID: chatID,
			Document: &models.InputFileUpload{
				Filename: "Analysis_report.html",
				Data:     bytes.NewReader([]byte(htmlResult)),
			},
			Caption:   locales.MsgUploadReportHTMLCaption,
			ParseMode: "HTML",
		})
		sendAnalysisComplete(ctx, b, stateManager, chatID)
		return
	}

	log.Printf("✅ [UPLOAD] PDF-отчёт (расширенный анализ) отправлен chatID=%d: %d байт", chatID, len(pdfBytes))
	_, _ = b.SendDocument(ctx, &tgbot.SendDocumentParams{
		ChatID: chatID,
		Document: &models.InputFileUpload{
			Filename: "Analysis_report.pdf",
			Data:     bytes.NewReader(pdfBytes),
		},
		Caption:   locales.MsgUploadReportCaption,
		ParseMode: "HTML",
	})

	sendAnalysisCompleteNote(ctx, b, stateManager, chatID, buildReportNote(jsonResult))

	// Сообщаем, что результат сохранён в «Мой профиль», и даём
	// кнопку для мгновенного открытия (все типы анализов хранятся там).
	helpers.SendSavedToSummary(ctx, b, chatID, webAppURL)
}

// cleanJSONReport - очищает текст JSON от markdown-обёртки.
func cleanJSONReport(jsonResult string) string {
	cleaned := strings.TrimSpace(jsonResult)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	return strings.TrimSpace(cleaned)
}

// renderAndSendDossier - рендерит JSON-досье в HTML, конвертирует в PDF и
// отправляет пользователю как PDF-документ. Сохраняет результат в историю
// (для Мониторинга) и как Diagnosis (для профиля). При ошибке конвертации в
// PDF откатывается к отправке HTML (отчёт не теряется).
func renderAndSendDossier(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	reportRenderer *report.Renderer,
	pdfConverter pdfservice.Converter,
	chatID int64,
	loadingMsg *models.Message,
	textMsg *models.Message,
	jsonResult string,
	appStorage *storage.Storage,
	saver monitoring.Repository,
	webAppURL string,
) {
	cleanedJSON := cleanJSONReport(jsonResult)

	var dossier apmodels.HealthDossier
	if err := json.Unmarshal([]byte(cleanedJSON), &dossier); err != nil {
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

	htmlResult, err := reportRenderer.RenderDossier(dossier)
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
		title := monitoring.ExtractTitle(cleanedJSON, locales.MsgUploadDefaultTitleDossier)
		if saveErr := saver.SaveResult(ctx, &monitoring.HistoryEntry{
			TelegramID: chatID,
			Type:       "analysis",
			Title:      title,
			Date:       time.Now(),
			JsonData:   cleanedJSON,
			ReportHTML: htmlResult,
		}); saveErr != nil {
			log.Printf(locales.LogUploadMonitoringSaveErr, chatID, saveErr)
		} else {
			log.Printf(locales.LogUploadMonitoringSaved, chatID)
		}
	}

	// Персистим результат как Diagnosis (для профиля/истории пользователя).
	if appStorage != nil {
		if derr := appStorage.SaveDiagnosisForUser(ctx, chatID, "analysis", cleanedJSON, htmlResult); derr != nil {
			log.Printf(locales.LogUploadStorageSaveErr, chatID, derr)
		} else {
			log.Printf(locales.LogUploadStorageSaved, chatID)
		}
	}

	analytics.EmitEvent(ctx, analytics.Event{
		Type:       analytics.EventAnalysis,
		TelegramID: chatID,
		Meta:       map[string]interface{}{"title": monitoring.ExtractTitle(cleanedJSON, "Досье здоровья")},
	})

	// PostHog: успешная обработка досье/биоскана PRO (расширенный анализ).
	aType := "regular"
	if isExtendedAnalysis(stateManager, chatID) {
		aType = "extended"
	}
	analytics.Track(chatID, "analysis_processed", map[string]interface{}{
		"analysis_type": aType,
	})

	deleteLoadingMessages(ctx, b, chatID, loadingMsg, textMsg)

	// Отчёт-досье конвертируем в PDF и отправляем как PDF-документ.
	// При сбое конвертации (нет Chrome / сервис недоступен) - откат к HTML,
	// чтобы результат не потерялся.
	pdfBytes, convErr := pdfConverter.ConvertHTML(ctx, htmlResult)
	if convErr != nil {
		log.Printf("⚠️ [UPLOAD] не удалось конвертировать досье в PDF (chatID=%d): %v - отправляю HTML", chatID, convErr)
		_, _ = b.SendDocument(ctx, &tgbot.SendDocumentParams{
			ChatID: chatID,
			Document: &models.InputFileUpload{
				Filename: "Health_profile.html",
				Data:     bytes.NewReader([]byte(htmlResult)),
			},
			Caption:   locales.MsgUploadDossierCaption,
			ParseMode: "HTML",
		})
		sendAnalysisComplete(ctx, b, stateManager, chatID)
		return
	}

	log.Printf("✅ [UPLOAD] PDF-отчёт (досье/Биоскан PRO) отправлен chatID=%d: %d байт", chatID, len(pdfBytes))
	_, _ = b.SendDocument(ctx, &tgbot.SendDocumentParams{
		ChatID: chatID,
		Document: &models.InputFileUpload{
			Filename: "Health_profile.pdf",
			Data:     bytes.NewReader(pdfBytes),
		},
		Caption:   locales.MsgUploadDossierCaption,
		ParseMode: "HTML",
	})

	sendAnalysisCompleteNote(ctx, b, stateManager, chatID, buildReportNote(jsonResult))

	// Сообщаем, что результат сохранён в «Мой профиль», и даём
	// кнопку для мгновенного открытия (все типы анализов хранятся там).
	helpers.SendSavedToSummary(ctx, b, chatID, webAppURL)
}
