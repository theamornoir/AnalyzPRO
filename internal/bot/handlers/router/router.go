package router

import (
	"context"
	"log"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/service"
	"github.com/theamornoir/analyzpro/internal/storage"
)

// router - маршрутизатор сообщений с зависимостями.
type router struct {
	stateManager     states.StateManager
	analysisService  service.AnalysisService
	reportRenderer   *report.Renderer
	uploadDir        string
	stickerID        string
	adminChatID      int64
	agreementStorage *storage.AgreementStorage
}

// MessageRouter - главный маршрутизатор сообщений бота.
func MessageRouter(
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	uploadDir string,
	stickerID string,
	adminChatID int64,
	agreementStorage *storage.AgreementStorage,
) func(context.Context, *tgbot.Bot, *models.Update) {

	r := &router{
		stateManager:     stateManager,
		analysisService:  analysisService,
		reportRenderer:   reportRenderer,
		uploadDir:        uploadDir,
		stickerID:        stickerID,
		adminChatID:      adminChatID,
		agreementStorage: agreementStorage,
	}

	return r.handle
}

// handle - точка входа маршрутизатора.
func (r *router) handle(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	text := strings.TrimSpace(update.Message.Text)

	currentState := r.stateManager.GetState(chatID)
	agreed := r.agreementStorage.IsAgreed(chatID)
	analysisType := r.stateManager.GetUserData(chatID, "analysis_type")
	analysisSubtype := r.stateManager.GetUserData(chatID, "analysis_subtype")

	log.Printf(locales.LogDiagnosticInfo,
		chatID, text, currentState, agreed, analysisType, analysisSubtype)

	// Проверка соглашения
	if r.handleAgreement(ctx, b, chatID, text) {
		return
	}

	// Обработка состояний Bioscan
	if r.handleBioscanStates(ctx, b, chatID, text, update) {
		return
	}

	// Обработка кнопки "Назад"
	if r.handleBack(ctx, b, chatID, text) {
		return
	}

	// Обработка состояний опросника
	if r.handleQuestionnaireStates(ctx, b, chatID, text) {
		return
	}

	// Обработка загрузки файлов
	if r.handleUpload(ctx, b, chatID, text, update) {
		return
	}

	// Обработка кнопок меню
	if r.handleMenuButtons(ctx, b, chatID, text, update) {
		return
	}

	// Обработка обычного текста
	r.handleText(ctx, b, chatID, text)
}
