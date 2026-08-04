package bot

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/theamornoir/analyzpro/internal/bot/handlers"
)

type Bot struct {
	client *tgbot.Bot
}

func New(token string) (*Bot, error) {
	client, err := tgbot.New(token)
	if err != nil {
		return nil, err
	}

	client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		"/start",
		tgbot.MatchTypeExact,
		handlers.StartHandler,
	)

	return &Bot{
		client: client,
	}, nil
}

func (b *Bot) Start() {
	b.client.Start(context.Background())
}
