package upload

import (
	"context"
	"encoding/json"
	"fmt"

	tgbot "github.com/go-telegram/bot"

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

// StartAnalysis - экспортируемая обёртка startAnalysis для вызова из
// inline-кнопки «Обработать анализы» (router).
func StartAnalysis(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	pdfConverter pdfservice.Converter,
	uploadDir string,
	stickerID string,
	chatID int64,
	appStorage *storage.Storage,
	saver monitoring.Repository,
	webAppURL string,
) {
	// Защита от устаревших/повторных нажатий inline-кнопки «Обработать»:
	// кнопка действует только если пользователь всё ещё на шаге
	// подтверждения загрузки. Иначе (например, нажатие на старую кнопку из
	// истории после сброса состояния или выхода из раздела) - сообщаем, что
	// кнопка устарела, и предлагаем начать заново, вместо того чтобы
	// запускать анализ с пустым/чужим списком файлов.
	if stateManager.GetState(chatID) != states.StateWaitingUploadConfirm {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgUploadStaleButton,
			ReplyMarkup: keyboards.MainMenuInline(),
		})
		return
	}
	startAnalysis(ctx, b, stateManager, analysisService, reportRenderer, pdfConverter, uploadDir, stickerID, chatID, appStorage, saver, webAppURL)
}

// startAnalysis - запускает анализ всех накопленных файлов.
// saver сохраняет результат в историю (для Мониторинга).
// appStorage персистит результат как Diagnosis.
// webAppURL используется для кнопки «Открыть Мой профиль» после отчёта.
func startAnalysis(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	pdfConverter pdfservice.Converter,
	uploadDir string,
	stickerID string,
	chatID int64,
	appStorage *storage.Storage,
	saver monitoring.Repository,
	webAppURL string,
) {
	filesJSON := stateManager.GetUserData(chatID, "uploaded_files")
	if filesJSON == "" {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgUploadNoFiles,
		})
		stateManager.SetState(chatID, states.StateWaitingAnalysisFile)
		return
	}

	var uploadedFiles []UploadedFile
	if err := json.Unmarshal([]byte(filesJSON), &uploadedFiles); err != nil || len(uploadedFiles) == 0 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgUploadFileError2,
		})
		stateManager.SetState(chatID, states.StateWaitingAnalysisFile)
		return
	}

	stateManager.SetUserData(chatID, "uploaded_files", "")
	stateManager.SetUserData(chatID, "file_count", "")
	stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

	isExtended := isExtendedAnalysis(stateManager, chatID)

	userData := stateManager.GetAllUserData(chatID)
	contextInfo := helpers.BuildAnalysisText(userData)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    chatID,
		Text:      fmt.Sprintf(locales.MsgUploadAnalyzing, len(uploadedFiles)),
		ParseMode: "HTML",
	})

	loadingMsg, textMsg := helpers.SendLoadingMessages(ctx, b, chatID, stickerID, nil)

	defer cleanupUploadedFiles(uploadedFiles)

	if len(uploadedFiles) == 1 {
		file := uploadedFiles[0]
		processSingleFile(ctx, b, stateManager, analysisService, reportRenderer, pdfConverter, chatID, loadingMsg, textMsg, file, isExtended, contextInfo, appStorage, saver, webAppURL)
	} else {
		processMultipleFiles(ctx, b, stateManager, analysisService, reportRenderer, pdfConverter, chatID, loadingMsg, textMsg, uploadedFiles, isExtended, contextInfo, appStorage, saver, webAppURL)
	}
}

// isExtendedAnalysis - определяет, запущен ли расширенный анализ.
func isExtendedAnalysis(stateManager states.StateManager, chatID int64) bool {
	analysisType := stateManager.GetUserData(chatID, "analysis_type")
	analysisSubtype := stateManager.GetUserData(chatID, "analysis_subtype")
	return analysisSubtype == "extended" || analysisType == "extended"
}
