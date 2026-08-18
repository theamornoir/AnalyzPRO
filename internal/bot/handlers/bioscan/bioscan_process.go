package bioscan

import (
	"bytes"
	"context"
	"fmt"
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
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/report/pdfservice"
	"github.com/theamornoir/analyzpro/internal/service"
	"github.com/theamornoir/analyzpro/internal/storage"
)

// ProcessBioscanWithPhotos - обработка подтверждения и отправка в AI.
// saver сохраняет результат биоскана в историю пользователя (для Мониторинга).
// appStorage персистит результат как Diagnosis (для профиля пользователя).
func ProcessBioscanWithPhotos(
	ctx context.Context,
	b *tgbot.Bot,
	sm states.StateManager,
	analysisService service.AnalysisService,
	pdfConverter pdfservice.Converter,
	uploadDir string,
	stickerID string,
	chatID int64,
	appStorage *storage.Storage,
	saver monitoring.Repository,
	webAppURL string,
) {
	// Собираем все данные
	name := sm.GetUserData(chatID, "bioscan_name")
	age := sm.GetUserData(chatID, "bioscan_age")
	height := sm.GetUserData(chatID, "bioscan_height")
	weight := sm.GetUserData(chatID, "bioscan_weight")
	goal := sm.GetUserData(chatID, "bioscan_goal")
	photo1ID := sm.GetUserData(chatID, "bioscan_photo1")
	photo2ID := sm.GetUserData(chatID, "bioscan_photo2")
	photo3ID := sm.GetUserData(chatID, "bioscan_photo3")
	photo4ID := sm.GetUserData(chatID, "bioscan_photo4")

	if name == "" || age == "" || height == "" || weight == "" || goal == "" ||
		photo1ID == "" || photo2ID == "" || photo3ID == "" || photo4ID == "" {
		sm.SetState(chatID, states.StateIdle)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanIncompleteData,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return
	}

	// Контекст для ИИ: базовые поля + весь опросник Bioscan PRO (образ
	// жизни, тренировки, травмы, питание, привычки) - чтобы отчёт Body
	// Intelligence учитывал и анкету, а не только фото.
	contextInfo := BuildBioscanText(sm.GetAllUserData(chatID))

	bioscanSteps := []string{
		locales.BioscanStatusAnalyzingProportions,
		locales.BioscanStatusCheckingMuscleBalance,
		locales.BioscanStatusAnalyzingPosture,
		locales.BioscanStatusEvaluatingComposition,
		locales.BioscanStatusFormingProfile,
		locales.BioscanStatusCreatingRecommendations,
	}
	loadingMsg, textMsg := helpers.SendLoadingMessages(ctx, b, chatID, stickerID, bioscanSteps)

	// Скачиваем все 4 фотографии и передаём их в AI для полного анализа
	photoIDs := []string{photo1ID, photo2ID, photo3ID, photo4ID}
	photosData := make([][]byte, 0, len(photoIDs))

	for _, photoID := range photoIDs {
		photoData, _, err := helpers.DownloadFileByID(ctx, b, photoID, uploadDir)
		if err != nil {
			helpers.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)
			sm.SetState(chatID, states.StateIdle)
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        locales.MsgBioscanDownloadError,
				ReplyMarkup: keyboards.MainMenu(),
			})
			return
		}
		photosData = append(photosData, photoData)
	}

	if len(photosData) == 0 {
		helpers.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)
		sm.SetState(chatID, states.StateIdle)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanDownloadError,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return
	}

	// Расширенный (Premium) Bioscan PRO -> подробный HTML-отчёт Body Intelligence.
	// htmlReport - для отправки пользователю, jsonReport - «чистый» JSON
	// отчёта (сохраняется в историю для графиков дашборда «Мой профиль»
	// и используется при сравнительном повторном анализе).
	//
	// Сравнительный контекст: если ранее уже делали Bioscan PRO - подставляем
	// предыдущий отчёт, чтобы ИИ построил СРАВНИТЕЛЬНЫЙ отчёт (динамика:
	// что улучшилось / что улучшить), а не «с нуля».
	bioscanContext := contextInfo
	if prevJSON, ok := monitoring.PreviousReportJSON(ctx, saver, chatID, "bioscan"); ok {
		bioscanContext = contextInfo + locales.ComparisonContext(prevJSON, "bioscan")
	}

	htmlReport, jsonReport, err := analysisService.HandleBioscanPro(
		ctx,
		photosData,
		"image/jpeg",
		bioscanContext,
	)
	if err != nil {
		helpers.DeleteMessage(ctx, b, chatID, loadingMsg.ID)
		helpers.DeleteMessage(ctx, b, chatID, textMsg.ID)
		log.Printf(locales.LogBioscanError, err)

		sm.SetState(chatID, states.StateIdle)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        fmt.Sprintf(locales.MsgBioscanProcessingError, err),
			ReplyMarkup: keyboards.MainMenu(),
		})
		return
	}

	helpers.DeleteMessage(ctx, b, chatID, loadingMsg.ID)
	helpers.DeleteMessage(ctx, b, chatID, textMsg.ID)

	if len(htmlReport) > 0 {
		// Конвертируем премиальный HTML-отчёт Body Intelligence в PDF и
		// отправляем как PDF. При сбое конвертации - откат к HTML.
		pdfBytes, convErr := pdfConverter.ConvertHTML(ctx, htmlReport)
		if convErr != nil {
			log.Printf("⚠️ [BIOSCAN] не удалось конвертировать PRO-отчёт в PDF (chatID=%d): %v - отправляю HTML", chatID, convErr)
			_, err = b.SendDocument(
				ctx,
				&tgbot.SendDocumentParams{
					ChatID: chatID,
					Document: &models.InputFileUpload{
						Filename: "Body_scan_report.html",
						Data:     bytes.NewReader([]byte(htmlReport)),
					},
					Caption:   locales.MsgBioscanProCaption,
					ParseMode: "Markdown",
				},
			)
		} else {
			_, err = b.SendDocument(
				ctx,
				&tgbot.SendDocumentParams{
					ChatID: chatID,
					Document: &models.InputFileUpload{
						Filename: "Body_scan_report.pdf",
						Data:     bytes.NewReader(pdfBytes),
					},
					Caption:   locales.MsgBioscanProCaption,
					ParseMode: "Markdown",
				},
			)
		}

		if err != nil {
			log.Printf(locales.LogBioscanSendDocError, err)
		}
	}

	// Авто-сохранение результата биоскана в историю (для Мониторинга и
	// графиков дашборда «Мой профиль»).
	if saver != nil {
		if saveErr := saver.SaveResult(ctx, &monitoring.HistoryEntry{
			TelegramID: chatID,
			Type:       "bioscan",
			Title:      "Bioscan PRO",
			Date:       time.Now(),
			JsonData:   jsonReport,
			ReportHTML: htmlReport,
		}); saveErr != nil {
			log.Printf("[MONITORING] не удалось сохранить биоскан chatID=%d: %v", chatID, saveErr)
		} else {
			log.Printf("[MONITORING] история сохранена chatID=%d type=bioscan", chatID)
		}
	}

	// Персистим результат биоскана как Diagnosis (для профиля пользователя).
	if appStorage != nil {
		if derr := appStorage.SaveDiagnosisForUser(ctx, chatID, "bioscan", "", htmlReport); derr != nil {
			log.Printf("[STORAGE] не удалось сохранить диагноз-биоскан chatID=%d: %v", chatID, derr)
		} else {
			log.Printf("[STORAGE] диагноз сохранён chatID=%d type=bioscan", chatID)
		}
	}

	analytics.EmitEvent(ctx, analytics.Event{
		Type:       analytics.EventBioscan,
		TelegramID: chatID,
	})

	// PostHog: успешное завершение Bioscan PRO.
	analytics.Track(chatID, "bioscan_completed", nil)

	sm.SetState(chatID, states.StateIdle)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanDone,
		ReplyMarkup: keyboards.MainMenu(),
		ParseMode:   "Markdown",
	})

	// Прогресс + сравнение с предыдущим отчётом (если это повторный Bioscan PRO).
	// Отдельным сообщением без parse-mode, чтобы summary из отчёта ИИ (где
	// могут быть спецсимволы) не сломал Markdown предыдущего сообщения.
	if note := bioscanReportNote(jsonReport); note != "" {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        note,
			ReplyMarkup: keyboards.MainMenu(),
		})
	}

	// Сообщаем, что результат сохранён в «Мой профиль», и даём
	// кнопку для мгновенного открытия (все типы биосканов хранятся там).
	helpers.SendSavedToSummary(ctx, b, chatID, webAppURL)
}

// bioscanReportNote собирает текст доп. блока для выдачи Bioscan PRO:
// напоминание о сравнении повторных отчётов + краткое сравнение (summary),
// если ИИ сформировал сравнительный отчёт. jsonReport - JSON отчёта.
func bioscanReportNote(jsonReport string) string {
	parts := []string{locales.MsgReportProgressNote}
	if s := monitoring.ParseComparisonSummary(jsonReport); s != "" {
		parts = append(parts, "📈 Сравнение с предыдущим Bioscan PRO: "+s)
	}
	return strings.Join(parts, "\n\n")
}
