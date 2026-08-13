package bot

import (
	"context"
	"fmt"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/menu"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/router"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/payment"
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
	agreementStorage *storage.AgreementStorage
	paymentService   *payment.MockPaymentService
}

func New(
	token string,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	uploadDir string,
	stickerID string,
	adminChatID int64,
	agreementStorage *storage.AgreementStorage,
	paymentService *payment.MockPaymentService,
) (*Bot, error) {

	if stateManager == nil {
		stateManager = states.NewMemoryStateManager("")
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
		agreementStorage: agreementStorage,
		paymentService:   paymentService,
	}

	botInstance.registerHandlers()

	return botInstance, nil
}

func (b *Bot) Start(ctx context.Context) {
	b.client.Start(ctx)
}

func (b *Bot) registerHandlers() {

	router := router.MessageRouter(
		b.stateManager,
		b.analysisService,
		b.reportRenderer,
		b.uploadDir,
		b.stickerID,
		b.adminChatID,
		b.agreementStorage,
		b.paymentService,
	)

	// /start
	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		"/start",
		tgbot.MatchTypeExact,
		menu.StartHandler(b.stateManager, b.agreementStorage),
	)

	// Premium — кнопка меню
	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		locales.BtnPremium,
		tgbot.MatchTypeExact,
		menu.PremiumHandler(b.stateManager, b.paymentService),
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
