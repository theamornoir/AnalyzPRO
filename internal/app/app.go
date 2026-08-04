package app

import (
	"github.com/theamornoir/analyzpro/internal/bot"
	"github.com/theamornoir/analyzpro/internal/bot/states"
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

	stateManager := states.NewMemoryStateManager()
	telegramBot, err := bot.New(cfg.BotToken, stateManager)
	if err != nil {
		return nil, err
	}

	return &App{
		cfg: cfg,
		bot: telegramBot,
	}, nil
}

func (a *App) Run() {
	a.bot.Start()
}
