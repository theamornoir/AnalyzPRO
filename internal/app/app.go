package app

import (
	"context"
	"log"
	"os"

	"github.com/theamornoir/analyzpro/internal/ai/gemini"
	"github.com/theamornoir/analyzpro/internal/bot"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/config"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/service"
	"github.com/theamornoir/analyzpro/internal/storage"
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

	// Проверяем, нужно ли использовать моки
	// Моки используются если:
	// 1. APP_ENV=development и API ключ пустой или содержит "mock"
	// 2. Или явно установлена переменная USE_MOCK=true
	useMock := os.Getenv("USE_MOCK") == "true" ||
		(cfg.AppEnv == "development" && (cfg.GoogleGeminiAPIKey == "" || cfg.GoogleGeminiAPIKey == "mock"))

	log.Printf(locales.LogUseMockMode, useMock)
	log.Printf(locales.LogAppEnvironment, cfg.AppEnv)

	stateManager := states.NewMemoryStateManager("./data/states.json")

	// AI клиент
	var geminiClient *gemini.GeminiClient
	if useMock {
		// Используем мок
		geminiClient = gemini.NewGeminiClient("mock", cfg.GoogleAIModel)
		log.Printf(locales.LogRunningInMockMode)
	} else {
		// Используем реальный API
		geminiClient = gemini.NewGeminiClient(
			cfg.GoogleGeminiAPIKey,
			cfg.GoogleAIModel,
		)
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

	agreementStorage := storage.NewAgreementStorage("./data/agreements.json")

	telegramBot, err := bot.New(
		cfg.BotToken,
		stateManager,
		analysisService,
		renderer,
		cfg.UploadDir,
		cfg.LoadingStickerID,
		cfg.AdminChatID,
		agreementStorage,
	)
	if err != nil {
		return nil, err
	}

	log.Printf(locales.LogAppInitialized)
	log.Printf(locales.LogConfiguration)
	log.Printf(locales.LogAppEnv, cfg.AppEnv)
	log.Printf(locales.LogBotToken, cfg.BotToken[:10])
	log.Printf(locales.LogGeminiModel, cfg.GoogleAIModel)
	log.Printf(locales.LogUploadDir, cfg.UploadDir)
	log.Printf(locales.LogMockMode, useMock)
	log.Printf(locales.LogAdminChatID, cfg.AdminChatID)

	return &App{
		cfg: cfg,
		bot: telegramBot,
	}, nil
}

func (a *App) Run(ctx context.Context) {
	log.Printf(locales.LogBotRunning)
	a.bot.Start(ctx)
}
