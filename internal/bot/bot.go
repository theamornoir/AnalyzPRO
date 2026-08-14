package bot

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

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
	appStorage       *storage.Storage
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
	appStorage *storage.Storage,
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
		appStorage:       appStorage,
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
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		})
		mux.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
			target := "/dashboard/"
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			// 307 (а не 301): редирект НЕ кэшируется браузером/WebView.
			// Иначе при повторном открытии Mini App Telegram мог бы
			// использовать устаревший закэшированный редирект, и
			// tgWebAppData (параметр запуска) подставился бы из прошлой
			// сессии → initData оказался бы пустым/чужим.
			http.Redirect(w, r, target, http.StatusTemporaryRedirect)
		})
		dashHandler := dashboard.NewHandler(b.paymentService, b.botToken, b.monitorRepo)
		mux.HandleFunc("/dashboard/", dashHandler.ServeWebApp)
		mux.HandleFunc("/api/metrics", dashHandler.Metrics)
		mux.HandleFunc("/api/profile", dashHandler.SaveProfile)

		// Мониторинг: веб-апп (статика) + API с защитой initData.
		mux.HandleFunc("/monitoring", func(w http.ResponseWriter, r *http.Request) {
			target := "/monitoring/"
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			// 307 (а не 301): редирект НЕ кэшируется — см. обоснование у
			// /dashboard. Важно, чтобы tgWebAppData не терялся между
			// открытиями Mini App.
			http.Redirect(w, r, target, http.StatusTemporaryRedirect)
		})
		mux.HandleFunc("/monitoring/", monitoring.ServeWebApp)
		mux.HandleFunc("/api/monitoring/", monitoring.NewAPIHandler(b.monitorSvc, b.botToken).Handler())

		srv := &http.Server{Addr: listenAddr, Handler: mux}
		go func() {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutdownCtx)
		}()
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("❌ HTTP-сервер ошибка: %v", err)
		}
	}()

	// Menu Button (кнопка слева в чате бота) оставляем дефолтной — Сводка
	// здоровья открывается ТОЛЬКО из клавиатуры бота («📊 Здоровье» → «Открыть»),
	// а не через постоянную кнопку в чате. Сбрасываем в дефолт при каждом
	// старте (актуально для динамического https-туннеля `make mini`) — на
	// случай, если ранее кнопка была настроена на Mini App. Запускаем в
	// горутине — сетевой вызов к API не должен блокировать старт бота;
	// ошибки логируются, но не фатальны.
	go b.SetupMenuButton(ctx)

	b.client.Start(ctx)
}

// SetupMenuButton сбрасывает кнопку меню (слева в чате бота) в дефолтное
// состояние. Мы НЕ делаем из неё постоянную Mini App-кнопку «Сводка здоровья»:
// доступ к дашборду пользователь получает только из клавиатуры бота
// («📊 Здоровье» → хаб → «Открыть»). Дефолтная кнопка меню у неё показывает
// список команд бота и не мешает интерфейсу.
//
// Глобальная установка (без ChatID) применяется ко всем пользователям бота —
// сброс нужен на случай, если ранее кнопка была настроена на Mini App (старые
// запуски), иначе у пользователей осталась бы «висеть» web_app-кнопка после
// обновления кода.
//
// ВАЖНО: у индивидуальных типов MenuButton* поле Type не заполняется
// автоматически (в отличие от обёртки MenuButton), поэтому его нужно ставить
// явно, иначе сериализуется {"type":""} и Telegram вернёт
// "can't parse menu button: MenuButton has unsupported type".
func (b *Bot) SetupMenuButton(ctx context.Context) {
	if _, err := b.client.SetChatMenuButton(ctx, &tgbot.SetChatMenuButtonParams{
		MenuButton: &models.MenuButtonDefault{Type: models.MenuButtonTypeDefault},
	}); err != nil {
		log.Printf("⚠️ Не удалось сбросить Menu Button в default: %v", err)
		return
	}
	log.Printf("🔘 Menu Button: дефолтная (Сводка здоровья — только из меню бота).")
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
		b.appStorage,
		b.monitorRepo,
		b.webAppURL,
		b.dashboardURL,
	)

	// /start
	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		"/start",
		tgbot.MatchTypeExact,
		menu.StartHandler(b.stateManager, b.agreementStorage, b.appStorage),
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
