package bot

import (
	"context"
	"fmt"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/service"
	"github.com/theamornoir/analyzpro/internal/storage"
)

type Bot struct {
	client           *tgbot.Bot
	stateManager     states.StateManager
	analysisService  service.AnalysisService
	reportRenderer   *report.Renderer
	uploadDir        string
	stickerID        string
	adminChatID      int64
	agreementStorage *storage.AgreementStorage // <-- ДОБАВЛЕНО
}

func New(
	token string,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	uploadDir string,
	stickerID string,
	adminChatID int64,
	agreementStorage *storage.AgreementStorage, // <-- ДОБАВЛЕНО
) (*Bot, error) {

	if stateManager == nil {
		stateManager = states.NewMemoryStateManager()
	}

	if analysisService == nil {
		return nil, fmt.Errorf("analysis service is required")
	}

	client, err := tgbot.New(token)
	if err != nil {
		return nil, err
	}

	botInstance := &Bot{
		client:           client,
		stateManager:     stateManager,
		analysisService:  analysisService,
		reportRenderer:   reportRenderer,
		uploadDir:        uploadDir,
		stickerID:        stickerID,
		adminChatID:      adminChatID,
		agreementStorage: agreementStorage, // <-- ДОБАВЛЕНО
	}

	botInstance.registerHandlers()

	return botInstance, nil
}

func (b *Bot) Start(ctx context.Context) {
	b.client.Start(ctx)
}

func (b *Bot) registerHandlers() {

	router := handlers.MessageRouter(
		b.stateManager,
		b.analysisService,
		b.reportRenderer,
		b.uploadDir,
		b.stickerID,
		b.adminChatID,
		b.agreementStorage, // <-- ДОБАВЛЕНО
	)

	// /start
	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		"/start",
		tgbot.MatchTypeExact,
		handlers.StartHandler(b.stateManager, b.agreementStorage), // <-- ДОБАВЛЕНО
	)

	// Обычный текст
	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		"",
		tgbot.MatchTypePrefix,
		router,
	)

	// Документы
	b.client.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			return update.Message != nil &&
				update.Message.Document != nil
		},
		router,
	)

	// Фото
	b.client.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			return update.Message != nil &&
				update.Message.Photo != nil
		},
		router,
	)
}
