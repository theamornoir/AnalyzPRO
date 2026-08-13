package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

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
	paymentService := payment.NewMockPaymentService("./data/premium_users.json")
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
		cfg.WebAppURL,
		cfg.DashboardURL,
		cfg.HTTPAddr,
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
	log.Printf("🌐 Web App URL (для кнопки дашборда): %s", cfg.WebAppURL)
	if cfg.DashboardURL != cfg.WebAppURL {
		log.Printf("🌐 Dashboard URL: %s", cfg.DashboardURL)
	}
	if strings.HasPrefix(cfg.WebAppURL, "https") {
		log.Printf("✅ Дашборд будет открываться как Mini App прямо в Telegram (HTTPS).")
	} else if strings.Contains(cfg.WebAppURL, "localhost") || strings.Contains(cfg.WebAppURL, "127.0.0.1") {
		log.Printf("💡 Дашборд откроется как Mini App в Telegram Desktop на этой же машине (localhost). На телефоне запустите `make tunnel` (cloudflared/ngrok) для HTTPS — бот сам подхватит https-URL.")
	} else {
		log.Printf("💡 Дашборд доступен по локальной сети (http). На телефоне в той же Wi-Fi откройте ссылку в браузере. Для Mini App запустите `make tunnel` (cloudflared/ngrok) и задайте WEBAPP_URL/HTTPS-туннель.")
	}

	return &App{
		cfg: cfg,
		bot: telegramBot,
	}, nil
}

func (a *App) Run(ctx context.Context) {
	// Запрещаем запуск второго экземпляра с тем же токеном. Два long-polling
	// инстанса конкурируют за обновления Telegram — из-за этого часть ответов
	// (например, подтверждение Premium или сообщение дашборда) «молча» не
	// доходит до пользователя. Блокировка снимается при выходе процесса
	// (flock), поэтому зависших lock-файлов не остаётся.
	if err := acquireInstanceLock(); err != nil {
		log.Fatalf("⛔ Запуск отменён: %v\n   Возможно, уже запущен другой экземпляр бота с тем же токеном. Остановите его (Ctrl+C в том терминале) и повторите `make run`.", err)
	}

	log.Printf(locales.LogBotRunning)
	a.bot.Start(ctx)
}

// acquireInstanceLock блокирует файл /tmp/analyzpro.lock через flock. Если
// блокировка уже занята другим живым процессом — возвращает ошибку.
func acquireInstanceLock() error {
	f, err := os.OpenFile("/tmp/analyzpro.lock", os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return fmt.Errorf("не удалось заблокировать /tmp/analyzpro.lock (уже запущен?): %w", err)
	}
	// оставляем fd открытым на время жизни процесса — lock держится до выхода
	return nil
}
