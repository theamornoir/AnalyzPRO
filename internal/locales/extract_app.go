package locales

// ============================================================================
// App bootstrap / main entry strings (internal/app/app.go, cmd/bot/main.go).
//
// Startup, storage, analytics and instance-lock diagnostics. Centralised so
// the boot logs can be localised without editing the wiring code.
// ============================================================================

const (
	// LogAppStarting - первое сообщение при запуске бинарника.
	LogAppStarting = "🚀 Prisma запускается..."

	// LogDBInitialized - БД открыта и промигрирована.
	LogDBInitialized = "🗄️ База данных инициализирована: path=%s"

	// LogDBErrorClose - ошибка при закрытии соединения с БД на выходе.
	LogDBErrorClose = "⚠️ Ошибка закрытия соединения с БД: %v"

	// LogUsingMockStorage - выбрано мок-хранилище (USE_MOCK=true).
	LogUsingMockStorage = "🗄️ Используется МОК-хранилище (USE_MOCK=true)"

	// LogSQLStorageInit - SQL-хранилище поднято (типы репозиториев).
	// Формат: %T, %T, %T, %T (Users, Diagnoses, Cycles, Preferences).
	LogSQLStorageInit = "🗄️ SQL-хранилище инициализировано (Users=%T, Diagnoses=%T, Cycles=%T, Preferences=%T)"

	// LogAnalyticsInit - файл аналитики готов.
	LogAnalyticsInit = "📈 Аналитика инициализирована: path=%s"

	// LogWebAppURL - URL веб-аппа (кнопка дашборда).
	LogWebAppURL = "🌐 Web App URL (для кнопки дашборда): %s"

	// LogDashboardURL - отдельный Dashboard URL (если отличается).
	LogDashboardURL = "🌐 Dashboard URL: %s"

	// LogDashboardHTTPS - дашборд откроется как Mini App по HTTPS.
	LogDashboardHTTPS = "✅ Дашборд будет открываться как Mini App прямо в Telegram (HTTPS)."

	// LogDashboardLocalhost - запуск локально (localhost) дляDesktop.
	LogDashboardLocalhost = "💡 Дашборд откроется как Mini App в Telegram Desktop на этой же машине (localhost). На телефоне запустите `make tunnel` (cloudflared/ngrok) для HTTPS - бот сам подхватит https-URL."

	// LogDashboardLAN - доступ по локальной сети (http).
	LogDashboardLAN = "💡 Дашборд доступен по локальной сети (http). На телефоне в той же Wi-Fi откройте ссылку в браузере. Для Mini App запустите `make tunnel` (cloudflared/ngrok) и задайте WEBAPP_URL/HTTPS-туннель."

	// LogLaunchCancelled - уже запущен другой экземпляр (flock). %v = ошибка.
	LogLaunchCancelled = "⛔ Запуск отменён: %v\n   Возможно, уже запущен другой экземпляр бота с тем же токеном. Остановите его (Ctrl+C в том терминале) и повторите `make run`."

	// ErrLockAcquire - не удалось заблокировать lock-файл инстанса. %w = причина.
	ErrLockAcquire = "не удалось заблокировать /tmp/analyzpro.lock (уже запущен?): %w"
)
