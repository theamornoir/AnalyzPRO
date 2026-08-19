package bot

import (
	"context"
	"errors"
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
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/ratelimit"
	"github.com/theamornoir/analyzpro/internal/bot/reminders"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/notifications"
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
	paymentService   *payment.PaymentService
	appStorage       *storage.Storage
	webAppURL        string
	dashboardURL     string
	httpAddr         string
	botToken         string
	monitorRepo      monitoring.Repository
	monitorSvc       *monitoring.Service
	// notificationsService - сервис фоновых уведомлений о подписке и
	// аналитике. Задаётся из app.go через SetNotificationsService (после
	// создания бота, т.к. сервису нужен Telegram-клиент для отправки).
	notificationsService *notifications.Service
	// appEnv - окружение (development/production). Пробрасывается в
	// HTTP-обработчики дашборда, чтобы демо-режим (?demo=1) работал
	// только в development.
	appEnv string
	// promoCodes - список действующих промокодов на активацию Premium.
	promoCodes []string
	// promoCodesMonthly - список действующих промокодов на активацию
	// Premium на 30 дней (вместо года). Коды из этого списка при
	// активации дают premium_monthly, а не premium_yearly.
	promoCodesMonthly []string
	// rateLimiter - per-user (по chatID) rate-limit: защита от спама
	// одного пользователя множеством фото/документов/сообщений, которые
	// иначе породили бы неограниченное число горутин и очередь ИИ-запросов.
	rateLimiter *ratelimit.Limiter
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
	paymentService *payment.PaymentService,
	appStorage *storage.Storage,
	monitorRepo monitoring.Repository,
	webAppURL string,
	dashboardURL string,
	httpAddr string,
	appEnv string,
	promoCodes []string,
	promoCodesMonthly []string,
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
		client:            client,
		stateManager:      stateManager,
		analysisService:   analysisService,
		reportRenderer:    reportRenderer,
		pdfConverter:      pdfConverter,
		uploadDir:         uploadDir,
		stickerID:         stickerID,
		adminChatID:       adminChatID,
		agreementStorage:  agreementStorage,
		paymentService:    paymentService,
		appStorage:        appStorage,
		webAppURL:         webAppURL,
		dashboardURL:      dashboardURL,
		httpAddr:          httpAddr,
		botToken:          token,
		monitorRepo:       monitorRepo,
		monitorSvc:        monitoring.NewService(monitorRepo),
		appEnv:            appEnv,
		promoCodes:        promoCodes,
		promoCodesMonthly: promoCodesMonthly,
		// Per-user rate-limit: до 20 событий (сообщений/фото/документов/
		// callback) в окно 10 секунд на один chatID. Админ исключён в
		// обёртке. Значение подобрано так, чтобы легитимный пользователь
		// (несколько фото за анализ) не упирался в лимит, но спам
		// блокировался.
		rateLimiter: ratelimit.New(20, 10*time.Second),
	}

	// Диагностика: логируем id бота из токена, чтобы при ошибках валидации
	// initData можно было сверить - тот ли BOT_TOKEN запущен на сервере, что
	// подписал initData (serverBotID должен совпадать с ботом, с которым общается пользователь).
	bid := token
	if strings.Contains(bid, ":") {
		bid = bid[:strings.Index(bid, ":")]
	}
	log.Printf("[MONITORING] serverBotID=%q (из BOT_TOKEN), botTokenLen=%d", bid, len(token))

	// В development показываем dev-only элементы интерфейса (например,
	// вход в тестовое меню уведомлений «🧪 Тест уведомлений»). В проде
	// они скрыты, чтобы пользователи не видели отладочный инструмент.
	keyboards.SetDevMode(appEnv == "development")

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

// SetNotificationsService задаёт сервис фоновых уведомлений (dev-команды
// /test_sub, /test_analytics). Вызывается из app.go сразу после создания
// бота, т.к. сервису нужен Telegram-клиент для отправки сообщений.
func (b *Bot) SetNotificationsService(s *notifications.Service) {
	b.notificationsService = s
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
		dashHandler := dashboard.NewHandler(b.paymentService, b.botToken, b.monitorRepo, b.reportRenderer, b.pdfConverter, b.appEnv, b.notificationsService)
		mux.HandleFunc("/dashboard/", dashHandler.ServeWebApp)
		mux.HandleFunc("/api/metrics", dashHandler.Metrics)
		mux.HandleFunc("/api/profile", dashHandler.SaveProfile)
		mux.HandleFunc("/api/reports", dashHandler.Reports)
		// Открыть сохранённый отчёт как PDF прямо из «Мой профиль».
		mux.HandleFunc("/api/reports/file", dashHandler.ReportFile)
		// Удалить запись истории (анализ/биоскан/профиль) из «Мой
		// профиль» - чтобы пользователь мог удалять свои данные.
		mux.HandleFunc("/api/reports/delete", dashHandler.DeleteEntry)
		// Включить/отключить ВСЕ уведомления прямо из «Мой профиль»
		// (общий флаг Preferences.NotificationsEnabled).
		mux.HandleFunc("/api/notifications", dashHandler.Notifications)

		// Вебхук YooKassa: реальные колбэки об успешной оплате. Подпись
		// X-YooKassa-Signature проверяется внутри HandleWebhook
		// (HMAC-SHA256), чтобы нельзя было подделать «успешный платёж»
		// и получить Premium бесплатно. URL должен быть настроен в
		// личном кабинете YooKassa и доступен по HTTPS извне.
		mux.HandleFunc("/api/payment/webhook", b.paymentService.HandleWebhook)

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
		mux.HandleFunc("/api/monitoring/", monitoring.NewAPIHandler(b.monitorSvc, b.botToken, func(id int64) bool {
			return b.paymentService.IsPremium(id)
		}).Handler())

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

	// Периодическая очистка устаревших записей rate-limiter'а, чтобы
	// карта в памяти не росла бесконечно (окно лимитера - 10с).
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				b.rateLimiter.Cleanup()
			}
		}
	}()

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
		{Command: "notifications", Description: "Управление уведомлениями об анализах: on / off / status"},
	}
	// Dev-команды тестовой отправки уведомлений показываем в списке
	// команд ТОЛЬКО в development, чтобы не светить их в проде.
	if b.appEnv == "development" {
		commands = append(commands,
			models.BotCommand{Command: "test_sub_7d", Description: "Тест уведомления о подписке: за 7 дней (dev)"},
			models.BotCommand{Command: "test_sub_3d", Description: "Тест уведомления о подписке: за 3 дня (dev)"},
			models.BotCommand{Command: "test_sub_1d", Description: "Тест уведомления о подписке: за 1 день (dev)"},
			models.BotCommand{Command: "test_sub_today", Description: "Тест уведомления о подписке: в день окончания (dev)"},
			models.BotCommand{Command: "test_analytics_check", Description: "Тест проверки анализов (без отправки, dev)"},
			models.BotCommand{Command: "test_analytics_send", Description: "Тест отправки уведомлений по анализам (dev)"},
		)
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
// Premium для текущего аккаунта. Коды из cfg.PromoCodes дают год Premium
// (premium_yearly), из cfg.PromoCodesMonthly - месяц (premium_monthly).
// Каждый код можно использовать только один раз на аккаунт
// (хранится в used_promocodes).
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

	// Нормализуем регистр и ищем код в годовых, затем в месячных списках.
	// По принадлежности кода к списку определяем тариф активации.
	code = strings.ToUpper(code)
	tariffID := ""
	for _, c := range b.promoCodes {
		if strings.EqualFold(c, code) {
			tariffID = "premium_yearly"
			code = c
			break
		}
	}
	if tariffID == "" {
		for _, c := range b.promoCodesMonthly {
			if strings.EqualFold(c, code) {
				tariffID = "premium_monthly"
				code = c
				break
			}
		}
	}
	if tariffID == "" {
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

	// Активируем Premium через сервис платежей - он источник истины
	// для Premium-гейтов в дашборде/базе. Тариф зависит от типа кода.
	if err := b.paymentService.ActivatePremiumManually(chatID, tariffID); err != nil {
		log.Printf("⚠️ не удалось активировать Premium по промокоду для chatID=%d (tariff=%s): %v", chatID, tariffID, err)
		_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgPromoInvalid,
		})
		return
	}

	// Синхронизируем флаг is_premium и дату окончания в БД
	// (историческое поле). Длительность зависит от тарифа.
	expiresAt := time.Now().AddDate(1, 0, 0)
	activatedMsg := locales.MsgPromoActivated
	if tariffID == "premium_monthly" {
		expiresAt = time.Now().AddDate(0, 1, 0)
		activatedMsg = locales.MsgPromoActivatedMonth
	}
	if u, gerr := b.appStorage.Users.GetUserByTelegramID(ctx, chatID); gerr == nil {
		_ = b.appStorage.Users.UpdateUserPremiumStatus(ctx, u.ID, true, expiresAt, tariffID)
	}

	// Фиксируем использование кода (нельзя применить повторно).
	if err := b.appStorage.Users.MarkPromoCodeUsed(ctx, chatID, code); err != nil {
		log.Printf("⚠️ не удалось пометить промокод использованным для chatID=%d: %v", chatID, err)
	}

	_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   activatedMsg,
	})
}

// handleDevTestCommand обрабатывает dev-команды тестовой отправки
// уведомлений: /test_sub_7d, /test_sub_3d, /test_sub_1d, /test_sub_today
// (подписка) и /test_analytics_check, /test_analytics_send (анализы).
// Доступны ТОЛЬКО в development (регистрируются только там). Отправляют
// реальный образец уведомления (без ожидания 10 сек - команды для быстрой
// проверки без кнопок).
func (b *Bot) handleDevTestCommand(ctx context.Context, tb *tgbot.Bot, update *models.Update, kind string) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID

	s := b.notificationsService
	if s == nil {
		_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: locales.MsgNotifTestInvalid})
		return
	}

	switch kind {
	case "sub_7d", "sub_3d", "sub_1d", "sub_today":
		var days int
		switch kind {
		case "sub_7d":
			days = 7
		case "sub_3d":
			days = 3
		case "sub_1d":
			days = 1
		case "sub_today":
			days = 0
		}
		// ВАЖНО: SendSubscriptionTest САМ отправляет уведомление в Telegram,
		// поэтому здесь НЕ шлём text повторно - иначе пользователь
		// получит одно и то же уведомление ДВАЖДЫ. Проверяем только ошибку.
		if _, err := s.SendSubscriptionTest(ctx, chatID, days); err != nil {
			_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: locales.MsgNotifTestInvalid})
			return
		}
	case "analytics_check":
		findings, err := s.RunAnalyticsDryRun(ctx, chatID)
		if errors.Is(err, notifications.ErrNoAnalysisData) {
			_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: locales.MsgNotifAnalyticsNoData})
			return
		}
		if err != nil {
			_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: locales.MsgNotifTestInvalid})
			return
		}
		_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: s.DryRunMessage(findings), ParseMode: "Markdown"})
	case "analytics_send":
		n, err := s.SendAnalyticsTest(ctx, chatID)
		if errors.Is(err, notifications.ErrNoAnalysisData) {
			_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: locales.MsgNotifAnalyticsNoData})
			return
		}
		if err != nil {
			_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: locales.MsgNotifTestInvalid})
			return
		}
		_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: fmt.Sprintf(locales.MsgNotifAnalyticsSent, n), ParseMode: "Markdown"})
	default:
		_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: locales.MsgNotifTestInvalid})
	}
}

// handleNotificationsCommand обрабатывает команду /notifications
// (on|off|status) - управление уведомлениями об отклонениях в анализах
// для конкретного пользователя. Доступна всем (не только в dev).
func (b *Bot) handleNotificationsCommand(ctx context.Context, tb *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	action := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/notifications")))

	if b.notificationsService == nil {
		_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: locales.MsgNotificationsError})
		return
	}

	switch action {
	case "on":
		if err := b.notificationsService.SetUserNotificationsEnabled(ctx, chatID, true); err != nil {
			_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: locales.MsgNotificationsError})
			return
		}
		_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: locales.MsgNotificationsOn})
	case "off":
		if err := b.notificationsService.SetUserNotificationsEnabled(ctx, chatID, false); err != nil {
			_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: locales.MsgNotificationsError})
			return
		}
		_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: locales.MsgNotificationsOff})
	case "status", "":
		enabled, err := b.notificationsService.GetUserNotificationsEnabled(ctx, chatID)
		if err != nil {
			_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: locales.MsgNotificationsError})
			return
		}
		text := locales.MsgNotificationsStatusDisabled
		if enabled {
			text = locales.MsgNotificationsStatusEnabled
		}
		_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: text})
	default:
		_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: locales.MsgNotificationsUsage})
	}
}

// updateChatID извлекает идентификатор чата (пользователя) из апдейта - для
// ключа rate-limiter'а. Для callback-запросов берём ID отправителя
// (CallbackQuery.From.ID) - он всегда доступен и уникален на пользователя.
// (Поле Message в CallbackQuery имеет тип MaybeInaccessibleMessage без
// Chat, поэтому chatID для кнопок извлекаем именно по From.ID.)
func updateChatID(update *models.Update) int64 {
	if update == nil {
		return 0
	}
	if update.Message != nil {
		return update.Message.Chat.ID
	}
	if update.EditedMessage != nil {
		return update.EditedMessage.Chat.ID
	}
	if update.CallbackQuery != nil {
		return update.CallbackQuery.From.ID
	}
	return 0
}

// rateLimited оборачивает обработчик апдейта per-user rate-limit'ом. Админ
// (b.adminChatID) исключён - его действия не дросселируются. При превышении
// лимита пользователю один раз за окно шлётся подсказка «сбавьте темп»,
// остальные лишние события молча игнорируются (чтобы не спамить).
func (b *Bot) rateLimited(handler func(context.Context, *tgbot.Bot, *models.Update)) func(context.Context, *tgbot.Bot, *models.Update) {
	return func(ctx context.Context, tb *tgbot.Bot, update *models.Update) {
		chatID := updateChatID(update)
		if chatID != 0 && chatID != b.adminChatID {
			if !b.rateLimiter.Allow(chatID) {
				// Чтобы у избыточного клика по inline-кнопке не «крутился»
				// спиннер, отвечаем на callback сразу (без алерта). Иначе
				// превышение лимита оставило бы кнопку в состоянии загрузки.
				if update.CallbackQuery != nil {
					_, _ = tb.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
						CallbackQueryID: update.CallbackQuery.ID,
						Text:            locales.MsgRateLimit,
						ShowAlert:       false,
					})
				} else if b.rateLimiter.ShouldWarn(chatID) {
					_, _ = tb.SendMessage(ctx, &tgbot.SendMessageParams{
						ChatID: chatID,
						Text:   locales.MsgRateLimit,
					})
				}
				return
			}
		}
		handler(ctx, tb, update)
	}
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
		&b.notificationsService,
		b.webAppURL,
		b.dashboardURL,
		b.appEnv,
	)

	// /start - запуск бота / онбординг / главное меню.
	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		"/start",
		tgbot.MatchTypeExact,
		menu.StartHandler(b.stateManager, b.agreementStorage, b.appStorage),
	)

	// Примечание: кнопка меню 💎 Premium НЕ регистрируется здесь как
	// отдельный обработчик. Если бы она шла напрямую в menu.PremiumHandler,
	// это обходило бы router.handle и, как следствие, ветку
	// router_menu.go «case BtnPremium» - единственное место, где ставится
	// current_section="premium" и убирается закреплённое сообщение главного
	// меню. Без current_section="premium" кнопка «Назад» позже не попадала
	// бы в премиум-ветку backToParent (экран тарифов «висел» бы в чате).
	// Поэтому кнопка 💎 Premium идёт через общий роутер (обработчик "" ниже)
	// и попадает в handleMenuButtons → BtnPremium правильным путём.

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

	// Управление уведомлениями об отклонениях в анализах для
	// конкретного пользователя: /notifications on|off|status. Доступна
	// всем (не только в dev).
	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		"/notifications",
		tgbot.MatchTypePrefix,
		func(ctx context.Context, tb *tgbot.Bot, update *models.Update) {
			b.handleNotificationsCommand(ctx, tb, update)
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

	// Dev-команды тестовой отправки уведомлений о подписке и аналитике.
	// Регистрируем ТОЛЬКО в development, чтобы не засорять прод и не
	// давать пользователям слать себе тестовые рассылки. Каждая команда
	// присылает реальный образец уведомления (или предпросмотр) сразу.
	if b.appEnv == "development" {
		devTestCommands := map[string]string{
			"/test_sub_7d":          "sub_7d",
			"/test_sub_3d":          "sub_3d",
			"/test_sub_1d":          "sub_1d",
			"/test_sub_today":       "sub_today",
			"/test_analytics_check": "analytics_check",
			"/test_analytics_send":  "analytics_send",
		}
		for cmd, kind := range devTestCommands {
			k := kind
			b.client.RegisterHandler(
				tgbot.HandlerTypeMessageText,
				cmd,
				tgbot.MatchTypeExact,
				func(ctx context.Context, tb *tgbot.Bot, update *models.Update) {
					b.handleDevTestCommand(ctx, tb, update, k)
				},
			)
		}
	}

	// Обычный текст (с per-user rate-limit'ом против спама).
	b.client.RegisterHandler(
		tgbot.HandlerTypeMessageText,
		"",
		tgbot.MatchTypePrefix,
		b.rateLimited(router),
	)

	// Документы (с per-user rate-limit'ом: защита от массовой загрузки
	// файлов, порождающей горутины и очередь ИИ-запросов).
	b.client.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			return update.Message != nil &&
				update.Message.Document != nil
		},
		b.rateLimited(router),
	)

	// Фото (с per-user rate-limit'ом).
	b.client.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			return update.Message != nil &&
				update.Message.Photo != nil
		},
		b.rateLimited(router),
	)

	// Callback-запросы (inline-кнопки: выбор тарифа premium_<id>,
	// подтверждение оплаты premium_confirm_<id>). С per-user rate-limit'ом
	// против спама кнопками.
	// Без этой регистрации router.handle() никогда не получал callback'и,
	// поэтому клики по тарифам/оплате «ничего не делали».
	b.client.RegisterHandler(
		tgbot.HandlerTypeCallbackQueryData,
		"",
		tgbot.MatchTypePrefix,
		b.rateLimited(router),
	)
}
