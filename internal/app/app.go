package app

import (
	"context"
	"log"
	"os"

	"github.com/theamornoir/analyzpro/internal/ai/orchestrator"
	"github.com/theamornoir/analyzpro/internal/bot"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/config"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/payment"
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

	// AI оркестратор (Gemini → DeepSeek → Claude)
	aiOrchestrator := orchestrator.NewOrchestrator()

	// HTML Renderer для отчётов
	renderer, err := report.NewRenderer()
	if err != nil {
		return nil, err
	}

	// Сервис анализа
	analysisService := service.NewAnalysisService(
		aiOrchestrator,
		renderer,
	)

	agreementStorage := storage.NewAgreementStorage("./data/agreements.json")

	// Мок-хранилище (будет заменено на GORM при внедрении реальной БД)
	appStorage := storage.NewMockStorage()
	log.Printf("🗄️ Мок-хранилище инициализировано: Users=%T, Diagnoses=%T, Cycles=%T, Preferences=%T",
		appStorage.Users, appStorage.Diagnoses, appStorage.Cycles, appStorage.Preferences)

	_ = appStorage // Будет передано в хендлеры при следующем этапе

	// Сервис платежей (Mock YooKassa)
	paymentService := payment.NewMockPaymentService()
	log.Printf(locales.LogPaymentServiceInit)

	telegramBot, err := bot.New(
		cfg.BotToken,
		stateManager,
		analysisService,
		renderer,
		cfg.UploadDir,
		cfg.LoadingStickerID,
		cfg.AdminChatID,
		agreementStorage,
		paymentService,
	)
	if err != nil {
		return nil, err
	}

	log.Printf(locales.LogAppInitialized)
	log.Printf(locales.LogConfiguration)
	log.Printf(locales.LogAppEnv, cfg.AppEnv)
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
