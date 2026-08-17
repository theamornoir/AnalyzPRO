package bioscan

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/analytics"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/monitoring"
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
	uploadDir string,
	stickerID string,
	chatID int64,
	appStorage *storage.Storage,
	saver monitoring.HistorySaver,
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
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanIncompleteData,
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	contextInfo := fmt.Sprintf(
		"Данные пользователя:\nИмя: %s\nВозраст: %s лет\nРост: %s см\nВес: %s кг\nЦель: %s",
		name, age, height, weight, goal,
	)

	loadingMsg, textMsg := helpers.SendLoadingMessages(ctx, b, chatID, stickerID)

	go animateBioscanStatus(ctx, b, chatID, textMsg.ID)

	// Скачиваем все 4 фотографии и передаём их в AI для полного анализа
	photoIDs := []string{photo1ID, photo2ID, photo3ID, photo4ID}
	photosData := make([][]byte, 0, len(photoIDs))

	for _, photoID := range photoIDs {
		photoData, _, err := helpers.DownloadFileByID(ctx, b, photoID, uploadDir)
		if err != nil {
			helpers.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        locales.MsgBioscanDownloadError,
				ReplyMarkup: keyboards.BackMenu(),
			})
			return
		}
		photosData = append(photosData, photoData)
	}

	if len(photosData) == 0 {
		helpers.SafeDeleteLoadingMsgs(ctx, b, chatID, loadingMsg, textMsg)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanDownloadError,
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	pdfBytes, filename, htmlReport, err := analysisService.HandleBioscanPDF(
		ctx,
		photosData,
		"image/jpeg",
		contextInfo,
	)
	if err != nil {
		helpers.DeleteMessage(ctx, b, chatID, loadingMsg.ID)
		helpers.DeleteMessage(ctx, b, chatID, textMsg.ID)
		log.Printf(locales.LogBioscanError, err)

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        fmt.Sprintf(locales.MsgBioscanProcessingError, err),
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	helpers.DeleteMessage(ctx, b, chatID, loadingMsg.ID)
	helpers.DeleteMessage(ctx, b, chatID, textMsg.ID)

	// Расширенный (Premium) Bioscan -> детальный PDF-отчёт.
	if len(pdfBytes) > 0 {
		_, err = b.SendDocument(
			ctx,
			&tgbot.SendDocumentParams{
				ChatID: chatID,
				Document: &models.InputFileUpload{
					Filename: filename,
					Data:     bytes.NewReader(pdfBytes),
				},
				Caption:   fmt.Sprintf(locales.MsgBioscanReportCaption, name, age, height, weight, goal),
				ParseMode: "Markdown",
			},
		)

		if err != nil {
			log.Printf(locales.LogBioscanSendDocError, err)
		}
	}

	// Авто-сохранение результата биоскана в историю (для Мониторинга).
	if saver != nil {
		if saveErr := saver.SaveResult(ctx, &monitoring.HistoryEntry{
			TelegramID: chatID,
			Type:       "bioscan",
			Title:      "Bioscan",
			Date:       time.Now(),
			JsonData:   "",
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

	sm.SetState(chatID, states.StateIdle)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanDone,
		ReplyMarkup: keyboards.MainMenu(),
		ParseMode:   "Markdown",
	})
}

// animateBioscanStatus - анимация статуса обработки.
func animateBioscanStatus(ctx context.Context, b *tgbot.Bot, chatID int64, messageID int) {
	statuses := []string{
		locales.BioscanStatusAnalyzingProportions,
		locales.BioscanStatusCheckingMuscleBalance,
		locales.BioscanStatusAnalyzingPosture,
		locales.BioscanStatusEvaluatingComposition,
		locales.BioscanStatusFormingProfile,
		locales.BioscanStatusCreatingRecommendations,
	}

	for _, status := range statuses {
		select {
		case <-ctx.Done():
			return
		default:
		}

		time.Sleep(2 * time.Second)

		_, _ = b.EditMessageText(ctx, &tgbot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      fmt.Sprintf(locales.MsgBioscanLoadingStatusText, status),
		})
	}
}
