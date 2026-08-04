package bot

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/theamornoir/analyzpro/internal/bot/handlers"
	"github.com/theamornoir/analyzpro/internal/bot/states"
)

type Bot struct {
	client       *tgbot.Bot
	stateManager states.StateManager
}

func New(token string, stateManager states.StateManager) (*Bot, error) {
	if stateManager == nil {
		stateManager = states.NewMemoryStateManager()
	}

	client, err := tgbot.New(token)
	if err != nil {
		return nil, err
	}

	botInstance := &Bot{
		client:       client,
		stateManager: stateManager,
	}

	botInstance.registerHandlers()

	return botInstance, nil
}

func (b *Bot) Start() {
	b.client.Start(context.Background())
}

func (b *Bot) registerHandlers() {
	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		"/start",
		tgbot.MatchTypeExact,
		handlers.StartHandler(b.stateManager),
	)

	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		"",
		tgbot.MatchTypePrefix,
		handlers.MessageRouter(b.stateManager),
	)
}
