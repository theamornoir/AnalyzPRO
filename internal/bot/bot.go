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
	"github.com/theamornoir/analyzpro/internal/bot/reminders"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/payment"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/report/pdfservice"
	"github.com/theamornoir/analyzpro/internal/service"
	"github.com/theamornoir/analyzpro/internal/storage"
)

type Bot struct {
	client           *tgbot.Bot
	stateManager     states.StateManager
	analysisService  service.AnalysisService
	reportRenderer   *report.Renderer
	pdfConverter     pdfservice.Converter
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
	// appEnv - окружение (development/production). Пробрасывается в
	// HTTP-обработчики дашборда, чтобы демо-режим (?demo=1) работал
	// только в development.
	appEnv string
	// promoCodes - список действующих промокодов на активацию Premium.
	promoCodes []string
}

func New(
	token string,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	pdfConverter pdfservice.Converter,
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
	appEnv string,
	promoCodes []string,
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
		pdfConverter:     pdfConverter,
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
		appEnv:           appEnv,
		promoCodes:       promoCodes,
	}

	// Диагностика: логируем id бота из токена, чтобы при ошибках валидации
	// initData можно было сверить - тот ли BOT_TOKEN запущен на сервере, что
	// подписал initData (serverBotID должен совпадать с ботом, с которым общается пользователь).
	bid := token
	if strings.Contains(bid, ":") {
		bid = bid[:strings.Index(bid, ":")]
	}
	log.Printf("[MONITORING] serverBotID=%q (из BOT_TOKEN), botTokenLen=%d", bid, len(token))

	botInstance.registerHandlers()

	return botInstance, nil
}

// Client возвращает низкоуровневый Telegram-клиент. Используется
// вспомогательными модулями (например, системой напоминаний), которым нужно
// слать служебные сообщения вне обычного потока обработки апдейтов.
func (b *Bot) Client() *tgbot.Bot {
	return b.client
}

// Storage возвращает хранилище пользователей/предпочтений (для напоминаний).
func (b *Bot) Storage() *storage.Storage {
	return b.appStorage
}

// MonitorRepo возвращает репозиторий истории мониторинга (для проверки
// наличия сохранённых данных у пользователя в мотивационных напоминаниях).
func (b *Bot) MonitorRepo() monitoring.Repository {
	return b.monitorRepo
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
		dashHandler := dashboard.NewHandler(b.paymentService, b.botToken, b.monitorRepo, b.reportRenderer, b.pdfConverter, b.appEnv)
		mux.HandleFunc("/dashboard/", dashHandler.ServeWebApp)
		mux.HandleFunc("/api/metrics", dashHandler.Metrics)
		mux.HandleFunc("/api/profile", dashHandler.SaveProfile)
		mux.HandleFunc("/api/reports", dashHandler.Reports)
		// Открыть сохранённый отчёт как PDF прямо из «Мой профиль».
		mux.HandleFunc("/api/reports/file", dashHandler.ReportFile)
		// Удалить запись истории (анализ/биоскан/профиль) из «Мой
		// профиль» - чтобы пользователь мог удалять свои данные.
		mux.HandleFunc("/api/reports/delete", dashHandler.DeleteEntry)

		// Мониторинг: веб-апп (статика) + API с защитой initData.
		mux.HandleFunc("/monitoring", func(w http.ResponseWriter, r *http.Request) {
			target := "/monitoring/"
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			// 307 (а не 301): редирект НЕ кэшируется - см. обоснование у
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

	// Menu Button (кнопка слева в чате бота) оставляем дефолтной - Мой
	// профиль открывается ТОЛЬКО из клавиатуры бота («📊 Здоровье» → «Открыть»),
	// а не через постоянную кнопку в чате. Сбрасываем в дефолт при каждом
	// старте (актуально для динамического https-туннеля `make mini`) - на
	// случай, если ранее кнопка была настроена на Mini App. Запускаем в
	// горутине - сетевой вызов к API не должен блокировать старт бота;
	// ошибки логируются, но не фатальны.
	go b.SetupMenuButton(ctx)

	// Регистрируем список команд бота, чтобы они отображались в меню «/»
	// Telegram (в т.ч. админ-команды сброса для тестирования онбординга).
	// Запускаем в горутине - сетевой вызов не должен блокировать старт.
	go b.setupCommands(ctx)

	b.client.Start(ctx)
}

// setupCommands регистрирует список команд бота через setMyCommands, чтобы
// они были видны в меню «/» Telegram. В частности, это делает админ-команду
// сброса /resetme (и алиас /reset_premium) ОБНАРУЖИМОЙ - иначе она не
// показывается как «кнопка»/подсказка, и при тестировании онбординга
// кажется, что её нет. Сами команды видны всем пользователям, но /resetme и
// /reset_premium реально срабатывают только для ADMIN_CHAT_ID (для прочих
// вызов в ResetHandler молча игнорируется).
func (b *Bot) setupCommands(ctx context.Context) {
	commands := []models.BotCommand{
		{Command: "start", Description: "Запустить бота / открыть главное меню"},
		{Command: "resetme", Description: "Сбросить Premium и онбординг (тест, только для админа)"},
		{Command: "reset_premium", Description: "Алиас /resetme - сброс статуса (тест)"},
		{Command: "announce", Description: "Анонс новой фичи всем пользователям (только для админа)"},
	}
	if _, err := b.client.SetMyCommands(ctx, &tgbot.SetMyCommandsParams{
		Commands: commands,
	}); err != nil {
		log.Printf("⚠️ Не удалось зарегистрировать команды бота: %v", err)
		return
	}
	log.Printf("🔘 Команды бота зарегистрированы (включая /resetme для тестов).")
}

// SetupMenuButton сбрасывает кнопку меню (слева в чате бота) в дефолтное
// состояние. Мы НЕ делаем из неё постоянную Mini App-кнопку «Мой профиль»:
// доступ к дашборду пользователь получает только из клавиатуры бота
// («📊 Здоровье» → хаб → «Открыть»). Дефолтная кнопка меню у неё показывает
// список команд бота и не мешает интерфейсу.
//
// Глобальная установка (без ChatID) применяется ко всем пользователям бота -
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
	log.Printf("🔘 Menu Button: дефолтная (Мой профиль - только из меню бота).")
}

// handlePromo обрабатывает команду /promo <code>: одноразовая активация
// Premium на 365 дней для текущего аккаунта. Каждый код можно использовать
// только один раз на аккаунт (хранится в used_promocodes).
func (b *Bot) handlePromo(ctx context.Context, tb *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	code := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/promo"))

	if code == "" {
		_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgPromoUsage,
		})
		return
	}

	// Нормализуем регистр и ищем в списке действующих кодов.
	code = strings.ToUpper(code)
	valid := false
	for _, c := range b.promoCodes {
		if strings.EqualFold(c, code) {
			valid = true
			code = c
			break
		}
	}
	if !valid {
		_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgPromoInvalid,
		})
		return
	}

	// Проверяем, не использовал ли пользователь этот код ранее.
	used := b.appStorage.Users.IsPromoCodeUsed(ctx, chatID, code)
	if used {
		_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgPromoAlreadyUsed,
		})
		return
	}

	// Активируем Premium (год) через сервис платежей - он источник истины
	// для Premium-гейтов в дашборде/базе.
	if err := b.paymentService.ActivatePremiumManually(chatID, "premium_yearly"); err != nil {
		log.Printf("⚠️ не удалось активировать Premium по промокоду для chatID=%d: %v", chatID, err)
		_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgPromoInvalid,
		})
		return
	}

	// Синхронизируем флаг is_premium в БД (историческое поле).
	if u, gerr := b.appStorage.Users.GetUserByTelegramID(ctx, chatID); gerr == nil {
		_ = b.appStorage.Users.UpdateUserPremiumStatus(ctx, u.ID, true, time.Now().AddDate(1, 0, 0))
	}

	// Фиксируем использование кода (нельзя применить повторно).
	if err := b.appStorage.Users.MarkPromoCodeUsed(ctx, chatID, code); err != nil {
		log.Printf("⚠️ не удалось пометить промокод использованным для chatID=%d: %v", chatID, err)
	}

	_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   locales.MsgPromoActivated,
	})
}

func (b *Bot) registerHandlers() {

	router := router.MessageRouter(
		b.stateManager,
		b.analysisService,
		b.reportRenderer,
		b.pdfConverter,
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

	// /start - запуск бота / онбординг / главное меню.
	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		"/start",
		tgbot.MatchTypeExact,
		menu.StartHandler(b.stateManager, b.agreementStorage, b.appStorage),
	)

	// Premium - кнопка меню
	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		locales.BtnPremium,
		tgbot.MatchTypeExact,
		menu.PremiumHandler(b.stateManager, b.paymentService),
	)

	// Админ-команда сброса Premium/онбординга (только для ADMIN_CHAT_ID).
	// Доступна по двум алиасам: /resetme и /reset_premium.
	resetHandler := menu.ResetHandler(
		b.adminChatID,
		b.stateManager,
		b.agreementStorage,
		b.paymentService,
		b.appStorage,
	)
	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		"/resetme",
		tgbot.MatchTypeExact,
		resetHandler,
	)
	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		"/reset_premium",
		tgbot.MatchTypeExact,
		resetHandler,
	)

	// Промокоды: /promo <code> активирует Premium на 365 дней (одноразово
	// на аккаунт). Список действующих кодов задан в cfg.PromoCodes.
	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		"/promo",
		tgbot.MatchTypePrefix,
		func(ctx context.Context, tb *tgbot.Bot, update *models.Update) {
			b.handlePromo(ctx, tb, update)
		},
	)

	// Админ-команда анонса новой фичи: /announce <текст> рассылает
	// одноразовое уведомление всем пользователям, не отключившим
	// уведомления. Работает только для ADMIN_CHAT_ID.
	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		"/announce",
		tgbot.MatchTypePrefix,
		func(ctx context.Context, tb *tgbot.Bot, update *models.Update) {
			if update.Message == nil {
				return
			}
			if update.Message.Chat.ID != b.adminChatID {
				return
			}
			text := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/announce"))
			if text == "" {
				_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text:   "Укажите текст после /announce, например:\n/announce Добавили графики тренировок в Мой профиль!",
				})
				return
			}
			sent, err := reminders.BroadcastFeature(ctx, tb, b.appStorage, text)
			reply := fmt.Sprintf("📣 Анонс разослан: %d пользователей.", sent)
			if err != nil {
				reply += fmt.Sprintf("\n⚠️ Ошибка: %v", err)
			}
			_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   reply,
			})
		},
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
	// подтверждение оплаты premium_confirm_<id>).
	// Без этой регистрации router.handle() никогда не получал callback'и,
	// поэтому клики по тарифам/оплате «ничего не делали».
	b.client.RegisterHandler(
		tgbot.HandlerTypeCallbackQueryData,
		"",
		tgbot.MatchTypePrefix,
		router,
	)
}
