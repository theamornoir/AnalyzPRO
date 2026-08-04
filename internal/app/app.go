package app

import (
	"github.com/theamornoir/analyzpro/internal/bot"
	"github.com/theamornoir/analyzpro/internal/config"
)

type App struct {
	cfg *config.Config

	bot *bot.Bot
}

func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	telegramBot, err := bot.New(cfg.BotToken)
	if err != nil {
		return nil, err
	}
	app := &App{
		cfg: cfg,
		bot: telegramBot,
	}

	return app, nil
}

func (a *App) Run() {
	a.bot.Start()
}
