package upload

import (
	"context"
	"log"
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

// CancelUpload - отмена загрузки из inline-кнопки «Отмена»: очищает
// накопленные файлы и возвращает в шаг ожидания файла.
func CancelUpload(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	chatID int64,
) {
	// Удаляем реальные файлы с диска: при отмене загрузки они больше не
	// нужны, иначе «висели» бы на диске навсегда (утечка места). Делаем ДО
	// очистки ссылок в состоянии, чтобы прочитать список файлов.
	cleanupUploadedFilesByChat(stateManager, chatID)
	stateManager.SetUserData(chatID, "uploaded_files", "")
	stateManager.SetUserData(chatID, "file_count", "")
	stateManager.SetState(chatID, states.StateWaitingAnalysisFile)
	// Персистентное (не самоудаляющееся) сообщение: иначе после удаления
	// кнопки по глобальному правилу внизу чата осталось бы «пустое дно».
	// Тот же ключ трекинга, что и у главного меню (helpers.MainMenuMsgKey) -
	// при возврате в меню/входе в раздел старое сообщение подчистится.
	helpers.ShowPersistentMessage(ctx, b, stateManager, chatID, helpers.MainMenuMsgKey, tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgUploadCancelled,
		ReplyMarkup: keyboards.MainMenu(),
	})
}

// handleUploadConfirm - обрабатывает подтверждение/отмену загрузки.
func handleUploadConfirm(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	pdfConverter pdfservice.Converter,
	uploadDir string,
	stickerID string,
	chatID int64,
	message *models.Message,
	appStorage *storage.Storage,
	saver monitoring.Repository,
	webAppURL string,
) {
	text := strings.TrimSpace(strings.ToLower(message.Text))

	if message.Document != nil {
		log.Printf(locales.LogUploadAddAnotherDoc)
		handleFileUpload(ctx, b, stateManager, uploadDir, chatID, message.Document)
		return
	}

	if message.Photo != nil {
		log.Printf(locales.LogUploadAddAnotherPhoto)
		handlePhotoUpload(ctx, b, stateManager, uploadDir, chatID, message.Photo)
		return
	}

	if text == locales.BtnProcessAnalysisLower || text == locales.BtnProcessAnalysisLowerShort {
		log.Printf(locales.LogUploadStartAnalysis)
		startAnalysis(ctx, b, stateManager, analysisService, reportRenderer, pdfConverter, uploadDir, stickerID, chatID, appStorage, saver, webAppURL)
		return
	}

	if text == locales.BtnCancelLower || text == locales.BtnCancelLowerShort {
		log.Printf(locales.LogUploadCancel)
		// Удаляем реальные файлы с диска (см. обоснование в CancelUpload),
		// затем очищаем ссылки на них в состоянии.
		cleanupUploadedFilesByChat(stateManager, chatID)
		stateManager.SetUserData(chatID, "uploaded_files", "")
		stateManager.SetUserData(chatID, "file_count", "")
		stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

		// Персистентное (не самоудаляющееся) сообщение - см. обоснование в
		// CancelUpload. Тот же ключ трекинга, что и у главного меню.
		helpers.ShowPersistentMessage(ctx, b, stateManager, chatID, helpers.MainMenuMsgKey, tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgUploadCancelled,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return
	}

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgUploadComplete,
		ReplyMarkup: keyboards.MainMenu(),
		ParseMode:   "HTML",
	})
}
