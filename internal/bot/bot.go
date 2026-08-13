package bot

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/dashboard"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/menu"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/router"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/payment"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/service"
	"github.com/theamornoir/analyzpro/internal/storage"
)

type Bot struct {
	client           *tgbot.Bot
	stateManager     states.StateManager
	analysisService  service.AnalysisService
	reportRenderer   *report.Renderer
	uploadDir        string
	stickerID        string
	adminChatID      int64
	agreementStorage *storage.AgreementStorage
	paymentService   *payment.MockPaymentService
	webAppURL        string
	dashboardURL     string
	httpAddr         string
	botToken         string
	monitorRepo      monitoring.Repository
	monitorSvc       *monitoring.Service
}

func New(
	token string,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	uploadDir string,
	stickerID string,
	adminChatID int64,
	agreementStorage *storage.AgreementStorage,
	paymentService *payment.MockPaymentService,
	monitorRepo monitoring.Repository,
	webAppURL string,
	dashboardURL string,
	httpAddr string,
) (*Bot, error) {

	if stateManager == nil {
		stateManager = states.NewMemoryStateManager("")
	}

	if analysisService == nil {
		return nil, fmt.Errorf("analysis service is required")
	}

	client, err := tgbot.New(token)
	if err != nil {
		return nil, err
	}

	botInstance := &Bot{
		client:           client,
		stateManager:     stateManager,
		analysisService:  analysisService,
		reportRenderer:   reportRenderer,
		uploadDir:        uploadDir,
		stickerID:        stickerID,
		adminChatID:      adminChatID,
		agreementStorage: agreementStorage,
		paymentService:   paymentService,
		webAppURL:        webAppURL,
		dashboardURL:     dashboardURL,
		httpAddr:         httpAddr,
		botToken:         token,
		monitorRepo:      monitorRepo,
		monitorSvc:       monitoring.NewService(monitorRepo),
	}

	// Диагностика: логируем id бота из токена, чтобы при ошибках валидации
	// initData можно было сверить — тот ли BOT_TOKEN запущен на сервере, что
	// подписал initData (serverBotID должен совпадать с ботом, с которым общается пользователь).
	bid := token
	if strings.Contains(bid, ":") {
		bid = bid[:strings.Index(bid, ":")]
	}
	log.Printf("[MONITORING] serverBotID=%q (из BOT_TOKEN), botTokenLen=%d", bid, len(token))

	botInstance.registerHandlers()

	return botInstance, nil
}

func (b *Bot) Start(ctx context.Context) {
	listenAddr := b.httpAddr
	if listenAddr == "" {
		listenAddr = ":8080"
	}
	log.Printf("🌐 HTTP-сервер для Web App запускается на %s", listenAddr)

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/dashboard/", http.StatusMovedPermanently)
		})
		mux.HandleFunc("/dashboard/", func(w http.ResponseWriter, r *http.Request) {
			dashboard.HandleWebApp(w, r, true)
		})
		mux.HandleFunc("/api/metrics", dashboard.HandleAPIMetrics)

		// Мониторинг: веб-апп (статика) + API с защитой initData.
		mux.HandleFunc("/monitoring", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/monitoring/", http.StatusMovedPermanently)
		})
		mux.HandleFunc("/monitoring/", monitoring.ServeWebApp)
		mux.HandleFunc("/api/monitoring/", monitoring.NewAPIHandler(b.monitorSvc, b.botToken).Handler())

		log.Printf("❌ HTTP-сервер ошибка: %v", http.ListenAndServe(listenAddr, mux))
	}()

	// Настраиваем Menu Button (кнопка рядом с полем ввода внизу чата) так,
	// чтобы она открывала Дашборд как Telegram Mini App. Это то же самое, что
	// предлагают делать вручную в BotFather → Menu Button, но выполняется
	// программно через Bot API (setChatMenuButton), поэтому BotFather не
	// нужен, а URL перенастраивается при каждом старте (актуально для
	// динамического https-туннеля `make mini`). Запускаем в горутине — сетевой
	// вызов к API не должен блокировать старт бота; ошибки логируются, но не
	// фатальны.
	go b.SetupMenuButton(ctx)

	b.client.Start(ctx)
}

// SetupMenuButton настраивает кнопку меню (рядом с полем ввода внизу чата) так,
// чтобы она открывала Дашборд как Telegram Mini App. Работает только для
// HTTPS-URL — Telegram требует https для web_app-кнопки меню. Поэтому Mini App
// в Menu Button имеет смысл только при https-URL (например, при запуске через
// `make mini`); во всех остальных случаях (localhost/LAN, http) кнопка
// сбрасывается в дефолт (команды), чтобы не висела нерабочая http-ссылка.
//
// Глобальная установка (без ChatID) применяется ко всем пользователям бота —
// именно это нужно для «кнопки дашборда у всех».
func (b *Bot) SetupMenuButton(ctx context.Context) {
	if b.webAppURL == "" {
		return
	}

	if !strings.HasPrefix(b.webAppURL, "https") {
		// Нет HTTPS — сбрасываем Menu Button в дефолт, чтобы не висела
		// нерабочая web_app-кнопка с http-ссылкой. ВАЖНО: у индивидуальных
		// типов MenuButton* поле Type не заполняется автоматически (в отличие
		// от обёртки MenuButton), поэтому его нужно ставить явно, иначе
		// сериализуется {"type":""} и Telegram вернёт
		// "can't parse menu button: MenuButton has unsupported type".
		if _, err := b.client.SetChatMenuButton(ctx, &tgbot.SetChatMenuButtonParams{
			MenuButton: &models.MenuButtonDefault{Type: models.MenuButtonTypeDefault},
		}); err != nil {
			log.Printf("⚠️ Не удалось сбросить Menu Button в default: %v", err)
		} else {
			log.Printf("🔘 Menu Button: оставлен дефолтным (нет HTTPS для Mini App).")
		}
		return
	}

	btn := &models.MenuButtonWebApp{
		Type:   models.MenuButtonTypeWebApp,
		Text:   "💡 Сводка здоровья",
		WebApp: models.WebAppInfo{URL: b.webAppURL},
	}
	if _, err := b.client.SetChatMenuButton(ctx, &tgbot.SetChatMenuButtonParams{
		MenuButton: btn,
	}); err != nil {
		log.Printf("⚠️ Не удалось установить Menu Button (Mini App): %v", err)
		return
	}
	log.Printf("🔘 Menu Button настроен: открывает Дашборд как Mini App (%s)", btn.WebApp.URL)
}

func (b *Bot) registerHandlers() {

	router := router.MessageRouter(
		b.stateManager,
		b.analysisService,
		b.reportRenderer,
		b.uploadDir,
		b.stickerID,
		b.adminChatID,
		b.agreementStorage,
		b.paymentService,
		b.monitorRepo,
		b.webAppURL,
		b.dashboardURL,
	)

	// /start
	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		"/start",
		tgbot.MatchTypeExact,
		menu.StartHandler(b.stateManager, b.agreementStorage),
	)

	// Premium — кнопка меню
	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		locales.BtnPremium,
		tgbot.MatchTypeExact,
		menu.PremiumHandler(b.stateManager, b.paymentService),
	)

	// Обычный текст
	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		"",
		tgbot.MatchTypePrefix,
		router,
	)

	// Документы
	b.client.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			return update.Message != nil &&
				update.Message.Document != nil
		},
		router,
	)

	// Фото
	b.client.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			return update.Message != nil &&
				update.Message.Photo != nil
		},
		router,
	)

	// Callback-запросы (inline-кнопки: выбор тарифа premium_<id>,
	// подтверждение оплаты premium_confirm_<id>, «Назад» back_main).
	// Без этой регистрации router.handle() никогда не получал callback'и,
	// поэтому клики по тарифам/оплате «ничего не делали».
	b.client.RegisterHandler(
		tgbot.HandlerTypeCallbackQueryData,
		"",
		tgbot.MatchTypePrefix,
		router,
	)
}
