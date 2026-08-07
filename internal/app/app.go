package app

import (
	"context"
	"log"
	"os"

	"github.com/theamornoir/analyzpro/internal/ai"
	"github.com/theamornoir/analyzpro/internal/bot"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/config"
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
		(cfg.AppEnv == "development" && (cfg.GoogleGeminiAPIKey == "" || cfg.GoogleGeminiAPIKey == "mock" || cfg.GoogleGeminiAPIKey == "AQ.Ab8RN6JFdJB_j-vWwX4Rk08YF1yY51dPLOz19FY1Ui5T2YLJDAgo"))

	log.Printf("🧪 Use mock mode: %v", useMock)
	log.Printf("📌 App Environment: %s", cfg.AppEnv)

	stateManager := states.NewMemoryStateManager()

	// AI клиент
	var geminiClient *ai.GeminiClient
	if useMock {
		// Используем мок
		geminiClient = ai.NewGeminiClient("mock", cfg.GoogleAIModel)
		log.Printf("🧪 Running in MOCK mode - all AI responses will be mocked")
	} else {
		// Используем реальный API
		geminiClient = ai.NewGeminiClient(
			cfg.GoogleGeminiAPIKey,
			cfg.GoogleAIModel,
		)
	}

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

	agreementStorage := storage.NewAgreementStorage("./data/agreements.json")

	telegramBot, err := bot.New(
		cfg.BotToken,
		stateManager,
		analysisService,
		reportRenderer,
		cfg.UploadDir,
		cfg.LoadingStickerID,
		cfg.AdminChatID,
		agreementStorage,
	)
	if err != nil {
		return nil, err
	}

	log.Printf("✅ App initialized successfully")
	log.Printf("📌 Configuration:")
	log.Printf("   - App Env: %s", cfg.AppEnv)
	log.Printf("   - Bot Token: %s...", cfg.BotToken[:10])
	log.Printf("   - Gemini Model: %s", cfg.GoogleAIModel)
	log.Printf("   - Upload Dir: %s", cfg.UploadDir)
	log.Printf("   - Mock Mode: %v", useMock)
	log.Printf("   - Admin Chat ID: %d", cfg.AdminChatID)

	return &App{
		cfg: cfg,
		bot: telegramBot,
	}, nil
}

func (a *App) Run(ctx context.Context) {
	log.Printf("🚀 Bot is running...")
	a.bot.Start(ctx)
}
