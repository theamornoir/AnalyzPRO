package analytics

import (
	"log"
	"strconv"
	"sync"

	"github.com/posthog/posthog-go"
)

// AnalyticsClient - клиент аналитики PostHog. Оборачивает *posthog.Client.
// Используется для отправки событий активности пользователей (старт бота,
// анализ, биоскан, премиум, открытие Сводки) в дашборд PostHog.
type AnalyticsClient struct {
	client posthog.Client
}

// NewAnalyticsClient создаёт клиент PostHog. Если apiKey пустой - библиотека
// posthog-go возвращает no-op клиент (Enqueue тихо игнорируется), поэтому
// отсутствие ключа в .env не ломает работу бота.
func NewAnalyticsClient(apiKey string) (*AnalyticsClient, error) {
	return &AnalyticsClient{client: posthog.New(apiKey)}, nil
}

// Track отправляет событие в PostHog. userID используется как DistinctId
// (идентификатор пользователя в PostHog). properties - доп. поля события.
func (c *AnalyticsClient) Track(userID int64, event string, properties map[string]interface{}) error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Enqueue(posthog.Capture{
		DistinctId: strconv.FormatInt(userID, 10),
		Event:      event,
		Properties: posthog.Properties(properties),
	})
}

// Close корректно завершает клиент (flush очереди отправки событий).
func (c *AnalyticsClient) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

// Глобальный клиент PostHog. Позволяет вызывать analytics.Track(...) из
// любого хендлера без явного проброса клиента через DI (разрешено ТЗ).
var (
	globalPHMu sync.RWMutex
	defaultPH  *AnalyticsClient
)

// InitPostHog инициализирует глобальный клиент PostHog. Вызывается один раз
// при старте приложения (app.New), рядом с analytics.Init.
func InitPostHog(apiKey string) {
	client, err := NewAnalyticsClient(apiKey)
	if err != nil {
		log.Printf("[ANALYTICS] PostHog: не удалось создать клиент: %v", err)
		return
	}
	globalPHMu.Lock()
	defaultPH = client
	globalPHMu.Unlock()
	log.Printf("[ANALYTICS] PostHog: клиент инициализирован (key_set=%t)", apiKey != "")
}

// Track отправляет событие в PostHog через глобальный клиент. Если клиент не
// инициализирован (нет POSTHOG_API_KEY) - тихо игнорируется.
//
// Название события в PostHog - человекочитаемая русская подпись: для
// предметных событий (user_started, analysis_processed, ...) берётся из
// semanticEventLabels; для событий interaction подпись уже передана вызывающим
// (см. TrackInteraction). Неизвестные ключи остаются как есть. В свойства
// всегда добавляется label (= название события) для удобной группировки в
// дашборде.
func Track(userID int64, event string, properties map[string]interface{}) {
	globalPHMu.RLock()
	c := defaultPH
	globalPHMu.RUnlock()
	if c == nil {
		return
	}

	label := event
	if l, ok := semanticEventLabels[event]; ok {
		label = l
	}
	if properties == nil {
		properties = map[string]interface{}{}
	}
	if _, ok := properties["label"]; !ok {
		properties["label"] = label
	}

	if err := c.Track(userID, label, properties); err != nil {
		log.Printf("[ANALYTICS] PostHog: ошибка отправки события %q (user=%d): %v", label, userID, err)
	}
}

// ClosePostHog закрывает глобальный клиент PostHog (flush очереди событий).
// Вызывается при корректном завершении работы бота.
func ClosePostHog() {
	globalPHMu.Lock()
	c := defaultPH
	defaultPH = nil
	globalPHMu.Unlock()
	if c != nil {
		_ = c.Close()
	}
}

// TrackInteraction отправляет универсальное событие на КАЖДОЕ взаимодействие
// пользователя с ботом: нажатие inline/reply-кнопки, команда, текстовое или
// медиа-сообщение. Дополняет предметные события (user_started,
// analysis_processed, bioscan_completed, premium_view, dashboard_opened,
// premium_purchased) и даёт полный clickstream в PostHog - видно буквально
// каждое нажатие, а не только ключевые точки.
//
// Название события в PostHog - человекочитаемая русская подпись, раскрываемая
// из source/action функцией interactionLabel (например, "Открыл раздел
// Анализы", "Нажал Обработать анализы", "Команда /start"). Сами технические
// данные (callback_data, текст команды, вид медиа) сохраняются в свойствах
// source/action/media для фильтрации.
//
// source - "callback" (нажата inline-кнопка), "command" (команда /...),
//
//	"message" (reply-кнопка или свободный текст/медиа).
//
// action - callback_data, либо текст команды/сообщения.
// properties - доп. поля (например, kind медиа: photo/document/voice/...).
func TrackInteraction(userID int64, source, action string, properties map[string]interface{}) {
	props := map[string]interface{}{
		"source": source,
		"action": action,
	}
	for k, v := range properties {
		props[k] = v
	}
	Track(userID, interactionLabel(source, action), props)
}
