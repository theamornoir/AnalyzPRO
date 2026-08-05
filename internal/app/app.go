package app

import (
	"context"

	"github.com/theamornoir/analyzpro/internal/ai"
	"github.com/theamornoir/analyzpro/internal/bot"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/config"
	"github.com/theamornoir/analyzpro/internal/report"
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

	// AI клиент
	geminiClient := ai.NewGeminiClient(
		cfg.GoogleGeminiAPIKey,
		cfg.GoogleAIModel,
	)

	reportRenderer, err := report.NewRenderer()

	if err != nil {
		return nil, err
	}

	// HTML Renderer для отчётов
	renderer, err := report.NewRenderer()
	if err != nil {
		return nil, err
	}

	// Сервис анализа
	analysisService := service.NewAnalysisService(
		geminiClient,
		renderer,
	)

	telegramBot, err := bot.New(
		cfg.BotToken,
		stateManager,
		analysisService,
		reportRenderer,
		cfg.UploadDir,
		cfg.LoadingStickerID,
		cfg.AdminChatID,
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
