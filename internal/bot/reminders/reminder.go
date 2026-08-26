package reminders

import (
	"context"
	"log"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/botutil"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/storage"
)

// Параметры системы напоминаний. Подобраны так, чтобы рассылка была
// ненавязчивой: не чаще одного сообщения в 2 недели конкретному
// пользователю, а повторное напоминание о повторном анализе - не чаще
// раза в 30 дней.
const (
	// checkInterval - как часто фоновый цикл просматривает всех
	// пользователей. Раз в 6 часов достаточно для ежедневной/еженедельной
	// логики напоминаний без лишней нагрузки на БД.
	checkInterval = 6 * time.Hour

	// firstRunDelay - небольшая пауза перед первым прогоном, чтобы не
	// блокировать старт бота и не слать сообщения в момент деплоя.
	firstRunDelay = 1 * time.Minute

	// motivationInactiveThreshold - порог неактивности, после которого
	// пользователю шлётся короткое разнообразное уведомление из пула
	// (locales.GetRandomReminder). Благодаря дебаунсу (сброс
	// LastActivityDate после отправки) реальная периодичность - ~10-14
	// дней: не чаще, чтобы не надоедать.
	motivationInactiveThreshold = 10 * 24 * time.Hour

	// reminderInactiveThreshold - порог неактивности для напоминания о
	// повторном анализе (более 30 дней без взаимодействия).
	reminderInactiveThreshold = 30 * 24 * time.Hour

	// broadcastThrottle - минимальная пауза между отправками сообщений в
	// циклах рассылки (напоминания/анонсы), чтобы не превышать лимит
	// Telegram (~30 msg/s). 50ms => ~20 msg/s с запасом.
	broadcastThrottle = 50 * time.Millisecond
)

// RunReminderLoop запускает фоновый цикл уведомлений. Блокирующая
// функция - её нужно вызывать в отдельной goroutine (она завершается только
// при отмене ctx, то есть при остановке бота).
//
// Для каждого пользователя проверяется дата последнего взаимодействия
// (LastActivityDate). В зависимости от глубины неактивности шлётся либо
// мотивационное сообщение (есть сохранённые данные, но не заходил ~2
// недели), либо напоминание о повторном анализе (>30 дней).
//
// Параметры:
//   - b: низкоуровневый Telegram-клиент (для отправки сообщений);
//   - store: хранилище пользователей/предпочтений;
//   - monitorRepo: репозиторий истории мониторинга (для проверки, есть ли у
//     пользователя сохранённые данные). Может быть nil - тогда
//     мотивационные напоминания не шлются.
func RunReminderLoop(ctx context.Context, b *tgbot.Bot, store *storage.Storage, monitorRepo monitoring.Repository) {
	if store == nil || b == nil {
		log.Printf("[REMINDERS] не запущен: нужны bot-клиент и storage")
		return
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	firstRun := time.After(firstRunDelay)
	for {
		select {
		case <-ctx.Done():
			log.Printf("[REMINDERS] цикл остановлен")
			return
		case <-firstRun:
			runChecks(ctx, b, store, monitorRepo)
			firstRun = nil // больше не срабатывает (nil-канал блокируется)
		case <-ticker.C:
			runChecks(ctx, b, store, monitorRepo)
		}
	}
}

// runChecks просматривает всех пользователей и шлёт подходящие напоминания.
func runChecks(ctx context.Context, b *tgbot.Bot, store *storage.Storage, monitorRepo monitoring.Repository) {
	users, err := store.GetAllUsers(ctx)
	if err != nil {
		log.Printf("[REMINDERS] не удалось получить пользователей: %v", err)
		return
	}

	now := time.Now()
	for _, u := range users {
		if u == nil {
			continue
		}
		// Уважаем отказ от уведомлений.
		if !notificationsEnabled(ctx, store, u.ID) {
			continue
		}

		last := u.LastActivityDate
		if last.IsZero() {
			last = u.CreatedAt
		}
		inactive := now.Sub(last)
		if inactive < motivationInactiveThreshold {
			continue
		}

		switch {
		case inactive >= reminderInactiveThreshold:
			// Глубокая неактивность (>30 дней) - шлём случайное короткое
			// напоминание из пула, чтобы вернуть пользователя.
			if send(ctx, b, u.TelegramID, locales.GetRandomReminder()) {
				// Дебаунс: после отправки считаем активность «обновлённой»,
				// чтобы следующее уведомление пришло не раньше чем через ~10 дней.
				_ = store.Users.UpdateUserLastActivity(ctx, u.ID, now)
				time.Sleep(broadcastThrottle)
			}
		case inactive >= motivationInactiveThreshold && userHasData(ctx, monitorRepo, u.TelegramID):
			// Пользователь с сохранёнными данными не заходил ~10 дней -
			// шлём случайное короткое напоминание (графики / действие / мотивация).
			if send(ctx, b, u.TelegramID, locales.GetRandomReminder()) {
				_ = store.Users.UpdateUserLastActivity(ctx, u.ID, now)
				time.Sleep(broadcastThrottle)
			}
		}
	}
}

// notificationsEnabled возвращает true, если пользователь не отключал
// уведомления. При отсутствии записи предпочтений (дефолт) - true.
func notificationsEnabled(ctx context.Context, store *storage.Storage, userID uint) bool {
	p, err := store.Preferences.GetPreferences(ctx, userID)
	if err != nil {
		return true
	}
	return p.NotificationsEnabled
}

// userHasData проверяет, есть ли у пользователя сохранённые результаты
// (анализы/биосканы/профиль) в истории мониторинга.
func userHasData(ctx context.Context, monitorRepo monitoring.Repository, telegramID int64) bool {
	if monitorRepo == nil {
		return false
	}
	entries, _, err := monitorRepo.ListHistory(ctx, telegramID, "", 1, 1)
	if err != nil || len(entries) == 0 {
		return false
	}
	return true
}

// send отправляет текстовое уведомление пользователю с клавиатурой главного
// меню (чтобы сразу можно было перейти в «Мой профиль»). Возвращает true,
// если сообщение успешно отправлено.
func send(ctx context.Context, b *tgbot.Bot, chatID int64, text string) bool {
	const maxAttempts = 4
	backoff := 250 * time.Millisecond
	for attempt := 0; attempt < maxAttempts; attempt++ {
		_, err := botutil.SendSafe(ctx, b, tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ParseMode:   "Markdown",
			ReplyMarkup: keyboards.MainMenuInline(),
		})
		if err == nil {
			return true
		}
		if !isTooManyRequests(err) {
			log.Printf("[REMINDERS] не удалось отправить уведомление chatID=%d: %v", chatID, err)
			return false
		}
		// Telegram 429 (Too Many Requests) - экспоненциальный backoff,
		// уважаем отмену ctx, чтобы не блокировать завершение бота.
		select {
		case <-ctx.Done():
			return false
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	log.Printf("[REMINDERS] исчерпаны попытки отправки chatID=%d (429)", chatID)
	return false
}

// isTooManyRequests - true, если ошибка Telegram означает превышение лимита
// запросов (HTTP 429). Библиотека tgbot возвращает ошибку со строкой вида
// "Too Many Requests: retry after N".
func isTooManyRequests(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "retry after") ||
		strings.Contains(msg, "429")
}

// BroadcastFeature рассылает одноразовое уведомление о новой функции всем
// пользователям, не отключившим уведомления. Возвращает число успешно
// доставленных сообщений.
//
// Предназначено для админ-команды /announce и анонсов при релизе фичи.
func BroadcastFeature(ctx context.Context, b *tgbot.Bot, store *storage.Storage, text string) (int, error) {
	if store == nil || b == nil || text == "" {
		return 0, nil
	}

	users, err := store.GetAllUsers(ctx)
	if err != nil {
		return 0, err
	}

	sent := 0
	for _, u := range users {
		if u == nil {
			continue
		}
		if !notificationsEnabled(ctx, store, u.ID) {
			continue
		}
		if send(ctx, b, u.TelegramID, text) {
			sent++
			time.Sleep(broadcastThrottle)
		}
	}
	return sent, nil
}

// SendTestNotification отправляет ТЕСТОВОЕ уведомление указанного типа
// немедленно. Тип kind: "reminder" (напоминание о повторном анализе),
// "motivation" (мотивационное) или "feature" (анонс новой фичи). Любое
// другое значение трактуется как "reminder".
//
// Предназначено для отладки системы уведомлений кнопками в разделе
// «Сервис» (подменю 🧪 Тест уведомлений) - присылает реальный образец
// того, что увидит пользователь в рассылке. Сообщение крепится клавиатура
// главного меню (как и в боевой рассылке).
func SendTestNotification(ctx context.Context, b *tgbot.Bot, chatID int64, kind string) {
	var text string
	switch kind {
	case "motivation", "reminder":
		// Для предпросмотра шлём случайный образец из пула
		// коротких уведомлений.
		text = locales.GetRandomReminder()
	case "feature":
		text = locales.MsgTestFeature
	default:
		text = locales.GetRandomReminder()
	}
	send(ctx, b, chatID, text)
}
