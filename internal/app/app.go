package app

import (
	"context"

	"github.com/theamornoir/analyzpro/internal/ai"
	"github.com/theamornoir/analyzpro/internal/bot"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/config"
	"github.com/theamornoir/analyzpro/internal/service"
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

	aiClient := ai.NewClient(cfg.OpenAIAPIKey, "gpt-4o-mini")
	analysisService := service.NewAnalysisService(aiClient)

	telegramBot, err := bot.New(
		cfg.BotToken,
		stateManager,
		analysisService,
	)
	if err != nil {
		return nil, err
	}

	return &App{
		cfg: cfg,
		bot: telegramBot,
	}, nil
}

func (a *App) Run(ctx context.Context) {
	a.bot.Start(ctx)
}
