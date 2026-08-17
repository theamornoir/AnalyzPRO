package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/theamornoir/analyzpro/internal/ai/httpclient"
	"github.com/theamornoir/analyzpro/internal/ai/orchestrator"
	"github.com/theamornoir/analyzpro/internal/analytics"
	"github.com/theamornoir/analyzpro/internal/bot"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/config"
	"github.com/theamornoir/analyzpro/internal/db"
	"github.com/theamornoir/analyzpro/internal/locales"
	monitoring_sqlrepo "github.com/theamornoir/analyzpro/internal/monitoring/sqlrepo"
	"github.com/theamornoir/analyzpro/internal/payment"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/report/pdfservice"
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

	// Настраиваем AI-HTTP-клиент с учётом прокси (GEMINI_PROXY / системный
	// HTTP_PROXY/HTTPS_PROXY). Должно идти сразу после config.Load(), чтобы
	// прокси подхватился ДО первых AI-вызовов (гео-блок Gemini во Франции и т.п.).
	httpclient.Configure(cfg.GeminiProxy)

	// Проверяем, нужно ли использовать моки
	// Моки используются если:
	// 1. APP_ENV=development и API ключ пустой или содержит "mock"
	// 2. Или явно установлена переменная USE_MOCK=true
	useMock := os.Getenv("USE_MOCK") == "true"

	log.Printf(locales.LogUseMockMode, useMock)
	log.Printf(locales.LogAppEnvironment, cfg.AppEnv)

	// Единая реляционная БД (SQLite по умолчанию, см. internal/db). Хранит
	// профили/диагнозы/курсы/предпочтения И историю мониторинга. Создаётся и
	// мигрируется при старте; данные переживают перезапуск бота.
	dbConn, err := db.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть БД: %w", err)
	}
	if err := db.Migrate(dbConn); err != nil {
		return nil, fmt.Errorf("не удалось применить миграции БД: %w", err)
	}
	log.Printf("🗄️ База данных инициализирована: path=%s", cfg.DBPath)

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

	// Хранилище профилей пользователей / диагнозов / курсов / предпочтений.
	// По умолчанию — реальная БД (SQLite/Postgres через *sql.DB). В режиме
	// USE_MOCK=true — мок (для локальной разработки без БД).
	var appStorage *storage.Storage
	if useMock {
		appStorage = storage.NewMockStorage()
		log.Printf("🗄️ Используется МОК-хранилище (USE_MOCK=true)")
	} else {
		appStorage = storage.NewSQLStorage(dbConn)
		log.Printf("🗄️ SQL-хранилище инициализировано (Users=%T, Diagnoses=%T, Cycles=%T, Preferences=%T)",
			appStorage.Users, appStorage.Diagnoses, appStorage.Cycles, appStorage.Preferences)
	}

	// Репозиторий модуля Мониторинг (проекты + история) поверх той же БД.
	// История анализов/биосканов сохраняется между перезапусками бота.
	monitorRepo := monitoring_sqlrepo.New(dbConn)

	// Сервис платежей (Mock YooKassa)
	paymentService := payment.NewMockPaymentService("./data/premium_users.json")
	log.Printf(locales.LogPaymentServiceInit)

	// Слой аналитики (события: старт, анализ, премиум, ошибки). Персистентный
	// JSONL-файл (ANALYTICS_PATH).
	analytics.Init(cfg.AnalyticsPath)
	log.Printf("📈 Аналитика инициализирована: path=%s", cfg.AnalyticsPath)

	// HTML→PDF конвертер (внешний сервис html2pdf.app по HTML2PDF_API_KEY).
	// Используется для расширенного анализа и Bioscan PRO — отчёты
	// отправляются как PDF (при сбое конвертации — как HTML).
	pdfConverter := pdfservice.New(pdfservice.Config{
		HTML2PDFAPIKey: cfg.HTML2PDFAPIKey,
	})

	telegramBot, err := bot.New(
		cfg.BotToken,
		stateManager,
		analysisService,
		renderer,
		pdfConverter,
		cfg.UploadDir,
		cfg.LoadingStickerID,
		cfg.AdminChatID,
		agreementStorage,
		paymentService,
		appStorage,
		monitorRepo,
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

func (a *App) Run(parent context.Context) {
	// Завершаем работу корректно по SIGINT/SIGTERM: отменяем контекст,
	// HTTP-сервер и long-polling Telegram останавливаются сами.
	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
