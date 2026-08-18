package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/theamornoir/analyzpro/internal/ai/claude"
	"github.com/theamornoir/analyzpro/internal/analytics"
	"github.com/theamornoir/analyzpro/internal/bot"
	"github.com/theamornoir/analyzpro/internal/bot/reminders"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/config"
	"github.com/theamornoir/analyzpro/internal/db"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/logging"
	monitoring_sqlrepo "github.com/theamornoir/analyzpro/internal/monitoring/sqlrepo"
	"github.com/theamornoir/analyzpro/internal/payment"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/report/pdfservice"
	"github.com/theamornoir/analyzpro/internal/service"
	"github.com/theamornoir/analyzpro/internal/storage"
)

type App struct {
	cfg    *config.Config
	bot    *bot.Bot
	dbConn *sql.DB
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
	log.Printf(locales.LogDBInitialized, cfg.DBPath)

	stateManager := states.NewMemoryStateManager("./data/states.json")

	// Единый мультимодальный Claude-клиент (все файлы в одном запросе).
	aiClient := claude.NewClient()

	// HTML Renderer для отчётов
	renderer, err := report.NewRenderer()
	if err != nil {
		return nil, err
	}

	// Сервис анализа
	analysisService := service.NewAnalysisService(
		aiClient,
		renderer,
	)

	agreementStorage := storage.NewAgreementStorage("./data/agreements.json")

	// Хранилище профилей пользователей / диагнозов / курсов / предпочтений.
	// По умолчанию - реальная БД (SQLite/Postgres через *sql.DB). В режиме
	// USE_MOCK=true - мок (для локальной разработки без БД).
	var appStorage *storage.Storage
	if useMock {
		appStorage = storage.NewMockStorage()
		log.Printf(locales.LogUsingMockStorage)
	} else {
		appStorage = storage.NewSQLStorage(dbConn)
		log.Printf(locales.LogSQLStorageInit,
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
	log.Printf(locales.LogAnalyticsInit, cfg.AnalyticsPath)

	// Клиент аналитики PostHog (события активности в дашборд app.posthog.com).
	// При отсутствии POSTHOG_API_KEY клиент работает как no-op.
	analytics.InitPostHog(cfg.PostHogAPIKey)

	// HTML→PDF конвертер (внешний сервис html2pdf.app по HTML2PDF_API_KEY).
	// Используется для расширенного анализа и Bioscan PRO - отчёты
	// отправляются как PDF (при сбое конвертации - как HTML).
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
	log.Printf(locales.LogClaudeModel, claude.Model)
	log.Printf(locales.LogUploadDir, cfg.UploadDir)
	log.Printf(locales.LogMockMode, useMock)
	log.Printf(locales.LogAdminChatID, cfg.AdminChatID)
	log.Printf(locales.LogWebAppURL, cfg.WebAppURL)
	if cfg.DashboardURL != cfg.WebAppURL {
		log.Printf(locales.LogDashboardURL, cfg.DashboardURL)
	}
	if strings.HasPrefix(cfg.WebAppURL, "https") {
		log.Printf(locales.LogDashboardHTTPS)
	} else if strings.Contains(cfg.WebAppURL, "localhost") || strings.Contains(cfg.WebAppURL, "127.0.0.1") {
		log.Printf(locales.LogDashboardLocalhost)
	} else {
		log.Printf(locales.LogDashboardLAN)
	}

	return &App{
		cfg:    cfg,
		bot:    telegramBot,
		dbConn: dbConn,
	}, nil
}

func (a *App) Run(parent context.Context) {
	// Централизованная настройка slog с уровнем из LOG_LEVEL (перенаправляет
	// и стандартный пакет log на slog). Идемпотентна - если уже настроена в
	// main, повторный вызов безопасен.
	logging.SetupLogging(a.cfg.LogLevel)

	// Завершаем работу корректно по SIGINT/SIGTERM: отменяем контекст,
	// HTTP-сервер и long-polling Telegram останавливаются сами.
	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Корректно закрываем клиент аналитики PostHog (flush очереди событий)
	// при выходе из Run (после остановки бота).
	defer analytics.ClosePostHog()

	// Корректно закрываем соединение с БД при выходе (сброс WAL, освобождение
	// файловых дескрипторов SQLite). bot.Start(ctx) блокирует выполнение до
	// отмены контекста, поэтому закрытие происходит уже после остановки бота.
	if a.dbConn != nil {
		defer func() {
			if err := a.dbConn.Close(); err != nil {
				log.Printf(locales.LogDBErrorClose, err)
			}
		}()
	}

	// Запрещаем запуск второго экземпляра с тем же токеном. Два long-polling
	// инстанса конкурируют за обновления Telegram - из-за этого часть ответов
	// (например, подтверждение Premium или сообщение дашборда) «молча» не
	// доходит до пользователя. Блокировка снимается при выходе процесса
	// (flock), поэтому зависших lock-файлов не остаётся.
	if err := acquireInstanceLock(); err != nil {
		log.Fatalf(locales.LogLaunchCancelled, err)
	}

	log.Printf(locales.LogBotRunning)

	// Фоновая система ненавязчивых уведомлений: напоминание о повторном
	// анализе при >30 дней неактивности и мягкие мотивационные сообщения.
	// Запускается в горутине и завершается вместе с ctx (остановка бота).
	go reminders.RunReminderLoop(ctx, a.bot.Client(), a.bot.Storage(), a.bot.MonitorRepo())

	a.bot.Start(ctx)
}

// acquireInstanceLock блокирует файл /tmp/analyzpro.lock через flock. Если
// блокировка уже занята другим живым процессом - возвращает ошибку.
func acquireInstanceLock() error {
	f, err := os.OpenFile("/tmp/analyzpro.lock", os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return fmt.Errorf(locales.ErrLockAcquire, err)
	}
	// оставляем fd открытым на время жизни процесса - lock держится до выхода
	return nil
}
