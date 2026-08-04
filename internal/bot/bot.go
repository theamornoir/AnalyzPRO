package bot

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/service"
)

type Bot struct {
	client          *tgbot.Bot
	stateManager    states.StateManager
	analysisService service.AnalysisService
	uploadDir       string
}

func New(
	token string,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	uploadDir string, // <-- Добавьте этот параметр
) (*Bot, error) {

	if stateManager == nil {
		stateManager = states.NewMemoryStateManager()
	}

	if analysisService == nil {
		analysisService = service.NewAnalysisService(nil)
	}

	client, err := tgbot.New(token)
	if err != nil {
		return nil, err
	}

	botInstance := &Bot{
		client:          client,
		stateManager:    stateManager,
		analysisService: analysisService,
		uploadDir:       uploadDir,
	}

	botInstance.registerHandlers()

	return botInstance, nil
}

func (b *Bot) Start(ctx context.Context) {
	b.client.Start(ctx)
}

func (b *Bot) registerHandlers() {
	// Роутер для всех сообщений
	router := handlers.MessageRouter(
		b.stateManager,
		b.analysisService,
		b.uploadDir, // <-- Передаем uploadDir
	)

	// /start
	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		"/start",
		tgbot.MatchTypeExact,
		handlers.StartHandler(b.stateManager),
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
