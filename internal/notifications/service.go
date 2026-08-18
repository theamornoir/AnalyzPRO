package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/botutil"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/payment"
	"github.com/theamornoir/analyzpro/internal/storage"
	sm "github.com/theamornoir/analyzpro/internal/storage/models"
)

// Service - фоновая система уведомлений:
//  1. Об окончании Premium-подписки - 4 точных напоминания (за 7/3/1/0 дней
//     до окончания), каждое - не более одного раза на пользователя.
//  2. Об отклонениях в анализах - только для Premium, раз в 3 дня берёт
//     последний анализ, сравнивает каждый показатель с референсным
//     интервалом и шлёт уведомление по показателю, вышедшему за норму
//     (повторно - не раньше чем через 14 дней).
//
// Работает в отдельной горутине (Run), завершается вместе с ctx.
type Service struct {
	db          *sql.DB
	repo        *repo
	store       *storage.Storage
	payment     *payment.MockPaymentService
	monitorRepo monitoring.Repository
	botClient   *tgbot.Bot
	isDev       bool
	// sendFn - опциональная замена отправки для юнит-тестов (без реального
	// Telegram-клиента). Если nil - отправка идёт через botClient.
	sendFn func(ctx context.Context, chatID int64, text string) bool
}

// NewService создаёт сервис уведомлений. botClient может быть задан позже
// через SetBotClient (он создаётся в app.go после бота, поэтому на момент
// NewService его ещё может не быть).
func NewService(
	db *sql.DB,
	store *storage.Storage,
	pay *payment.MockPaymentService,
	monitorRepo monitoring.Repository,
	isDev bool,
) *Service {
	return &Service{
		db:          db,
		repo:        newRepo(db),
		store:       store,
		payment:     pay,
		monitorRepo: monitorRepo,
		isDev:       isDev,
	}
}

// SetBotClient задаёт низкоуровневый Telegram-клиент для отправки
// уведомлений. Вызывается из app.go сразу после создания бота.
func (s *Service) SetBotClient(b *tgbot.Bot) {
	s.botClient = b
}

// Run запускает фоновый цикл уведомлений. Блокирующая функция - вызывать
// в отдельной горутине (завершается при отмене ctx, то есть при остановке
// бота).
func (s *Service) Run(ctx context.Context) {
	if s.store == nil {
		log.Printf("[NOTIF] не запущен: нужен storage")
		return
	}
	go s.dailyLoop(ctx)
}

// dailyLoop - встроенный планировщик: каждый день ровно в 10:00 local time
// проверяет окончание подписок, а анализы - раз в 3 дня (каждый 3-й прогон).
// Запуск после остановки бота невозможен (ctx.Done()).
func (s *Service) dailyLoop(ctx context.Context) {
	// Первая проверка - в ближайшие 10:00.
	select {
	case <-ctx.Done():
		return
	case <-time.After(time.Until(nextAt10())):
	}

	day := 0
	for {
		if ctx.Err() != nil {
			return
		}
		s.runSubscriptionChecks(ctx)
		// Аналитику проверяем раз в 3 дня (каждый 3-й ежедневный прогон).
		if day%3 == 0 {
			s.runAnalyticsChecks(ctx)
		}
		day++

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(nextAt10())):
		}
	}
}

// nextAt10 возвращает ближайшее наступающее 10:00 local time (строго после now).
func nextAt10() time.Time {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// premiumStatus возвращает Premium-статус и дату окончания подписки из
// ИСТОЧНИКА ИСТИНЫ - MockPaymentService (premium_users.json). SQL-поле
// users.is_premium синхронизируется с моком не всегда (например, обычная
// покупка пишет только в мок), поэтому читать статус из БД нельзя - иначе
// уведомления не уходят реальным платившим пользователям. Если сервис
// платежей недоступен (nil) - откатывается к SQL-полю (для тестов и
// обратной совместимости). Возвращает isPremium=false, если пользователь не
// Premium или срок уже истёк.
func (s *Service) premiumStatus(ctx context.Context, telegramID int64) (bool, time.Time) {
	if s.payment != nil {
		if info := s.payment.GetPremiumInfo(telegramID); info != nil {
			if info.IsPremium && !info.PremiumExpiresAt.IsZero() && info.PremiumExpiresAt.After(time.Now()) {
				return true, info.PremiumExpiresAt
			}
			return false, info.PremiumExpiresAt
		}
	}
	if s.store != nil {
		if u, err := s.store.Users.GetUserByTelegramID(ctx, telegramID); err == nil {
			return u.IsPremium, u.PremiumExpiresAt
		}
	}
	return false, time.Time{}
}

// runSubscriptionChecks - ежедневная проверка окончания Premium-подписок:
// для каждого подходящего пользователя шлёт нужное напоминание (7/3/1/0
// дней), если оно ещё не отправлялось. Premium-статус берётся из
// сервиса платежей (источник истины), а не из SQL.
// rawPremium возвращает «сырые» Premium-статус и дату окончания подписки из
// источника истины (MockPaymentService) БЕЗ проверки, не истекла ли подписка
// на момент вызова. Отличается от premiumStatus тем, что не возвращает
// isPremium=false для истёкшей подписки - это нужно runSubscriptionChecks,
// чтобы отправить напоминание об окончании (kind=0) даже для недавно
// истёкшей подписки (catch-up пропущенного уведомления). Если премиум-данных
// нет - откатывается к SQL (для тестов и обратной совместимости).
func (s *Service) rawPremium(ctx context.Context, telegramID int64) (bool, time.Time) {
	if s.payment != nil {
		if info := s.payment.GetPremiumInfo(telegramID); info != nil {
			return info.IsPremium, info.PremiumExpiresAt
		}
	}
	if s.store != nil {
		if u, err := s.store.Users.GetUserByTelegramID(ctx, telegramID); err == nil {
			return u.IsPremium, u.PremiumExpiresAt
		}
	}
	return false, time.Time{}
}

func (s *Service) runSubscriptionChecks(ctx context.Context) {
	users, err := s.store.GetAllUsers(ctx)
	if err != nil {
		log.Printf("[NOTIF] не удалось получить пользователей: %v", err)
		return
	}
	now := time.Now()
	for _, u := range users {
		if u == nil {
			continue
		}
		// rawPremium возвращает Premium-статус и дату окончания БЕЗ
		// проверки «не истекла ли уже подписка» - чтобы мы могли отправить
		// напоминание об окончании (kind=0) даже для недавно истёкшей
		// подписки (catch-up, см. ниже). В отличие от premiumStatus, здесь
		// важен сам факт наличия подписки, а не только её активность.
		isPrem, expires := s.rawPremium(ctx, u.TelegramID)
		if !isPrem || expires.IsZero() {
			continue
		}
		if !s.notificationsEnabledUser(ctx, u.ID) {
			continue
		}
		daysLeft := daysUntil(expires, now)
		var kind int
		if expires.After(now) {
			// Активная подписка: catch-up среди порогов (7→3→1→0). Берём
			// ближайший порог, для которого осталось ≤ порогу дней И
			// который ещё не отправлялся. Если бот был выключен ровно в
			// день планового напоминания, оно не ушло и не записалось -
			// при следующем прогоне (когда daysLeft всё ещё в окне этого
			// порога) напоминание уйдёт, а не потеряется навсегда.
			kind = -1
			for _, t := range []int{7, 3, 1, 0} {
				if daysLeft > t {
					continue
				}
				if has, _ := s.repo.hasSubscriptionNotification(ctx, u.TelegramID, t); has {
					continue
				}
				kind = t
				break
			}
		} else {
			// Подписка УЖЕ истекла. Шлём ровно одно напоминание об
			// окончании (kind=0, «сегодня/истекла»), если оно ещё не
			// уходило. Более ранние пороги (7/3/1) НЕ шлём - иначе при
			// простое бота пришло бы ложное «за 7 дней до окончания» уже
			// после фактического истечения. Так восстанавливается
			// пропущенное 0-е напоминание, если бот был выключен в день
			// окончания подписки.
			if has, _ := s.repo.hasSubscriptionNotification(ctx, u.TelegramID, 0); has {
				continue
			}
			kind = 0
		}
		if kind < 0 {
			continue
		}
		text := subscriptionText(kind)
		// Таймаут на отправку: один медленный/зависший Telegram-запрос не
		// должен блокировать весь фоновый цикл рассылки.
		sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		ok := s.sendNotification(sendCtx, u.TelegramID, text)
		cancel()
		if ok {
			_ = s.repo.recordSubscriptionNotification(ctx, u.TelegramID, kind, now)
		}
	}
}

// runAnalyticsChecks - проверка анализов на отклонения от нормы (только
// Premium): собирает все показатели последнего анализа, вышедшие за
// референсный интервал, и шлёт ИХ ВСЕ ОДНИМ сообщением (без повтора каждого
// показателя в течение 14 дней). Premium-статус берётся из сервиса платежей.
func (s *Service) runAnalyticsChecks(ctx context.Context) {
	users, err := s.store.GetAllUsers(ctx)
	if err != nil {
		log.Printf("[NOTIF] не удалось получить пользователей: %v", err)
		return
	}
	now := time.Now()
	for _, u := range users {
		if u == nil {
			continue
		}
		isPrem, _ := s.premiumStatus(ctx, u.TelegramID)
		if !isPrem {
			continue
		}
		if !s.notificationsEnabledUser(ctx, u.ID) {
			continue
		}
		inds, ok := s.latestAnalysisIndicators(ctx, u.TelegramID)
		if !ok || len(inds) == 0 {
			continue
		}
		// Собираем показатели, которые вне нормы и ещё не подавлены.
		var toNotify []indicator
		for _, ind := range inds {
			if !isOutOfRange(ind) {
				continue
			}
			if suppressed, _ := s.repo.isSuppressed(ctx, u.TelegramID, ind.Name, now); suppressed {
				continue
			}
			toNotify = append(toNotify, ind)
		}
		if len(toNotify) == 0 {
			continue
		}
		text := formatDeviations(toNotify)
		sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		ok = s.sendNotification(sendCtx, u.TelegramID, text)
		cancel()
		if ok {
			for _, ind := range toNotify {
				_ = s.repo.suppress(ctx, u.TelegramID, ind.Name, now.Add(14*24*time.Hour))
			}
		}
	}
}

// subscriptionKindFor преобразует число оставшихся дней до окончания в тип
// напоминания (7/3/1/0). Если до окончания осталось ровно 7/3/1/0 дней -
// возвращает соответствующее число. Если подписка уже истекла (<0) -
// возвращает 0 (сообщаем об окончании). Иначе -1 (не время напоминать).
// Честный catch-up (чтобы пропущенное из-за простоя бота напоминание не
// терялось) реализован в runSubscriptionChecks через перебор порогов с
// учётом уже отправленных (hasSubscriptionNotification).
func subscriptionKindFor(daysLeft int) int {
	switch daysLeft {
	case 7:
		return 7
	case 3:
		return 3
	case 1:
		return 1
	case 0:
		return 0
	}
	if daysLeft < 0 {
		return 0
	}
	return -1
}

// daysUntil возвращает целое число календарных дней от now до expiry
// (по дате, без времени). Положительное - expiry в будущем, отрицательное -
// уже истекла.
func daysUntil(expiry, now time.Time) int {
	e := time.Date(expiry.Year(), expiry.Month(), expiry.Day(), 0, 0, 0, 0, expiry.Location())
	n := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return int(e.Sub(n).Hours() / 24)
}

// subscriptionText возвращает точный текст напоминания по типу (7/3/1/0).
func subscriptionText(daysBefore int) string {
	switch daysBefore {
	case 7:
		return locales.MsgNotifSub7d
	case 3:
		return locales.MsgNotifSub3d
	case 1:
		return locales.MsgNotifSub1d
	case 0:
		return locales.MsgNotifSubToday
	}
	return ""
}

// indicator - извлечённый из JSON анализа показатель с референсным
// интервалом и единицей измерения.
type indicator struct {
	Name   string // название показателя (например, "Глюкоза")
	Value  string // значение (всегда строка; числа приводятся к строке)
	Normal string // референсный интервал (из любого из refFields)
	Unit   string // единица измерения (если есть в данных)
	Status string // статус: warning/critical/risk или normal/good
}

// refFields - возможные имена поля референсного интервала в JSON показателя.
// Перебираются по порядку; берётся первое непустое строковое значение.
// Поддерживаются все распространённые варианты из разных лабораторий.
var refFields = []string{"normal", "ref_range", "reference", "ref_interval", "norm", "range"}

// unitFields - возможные имена поля единицы измерения в JSON показателя.
var unitFields = []string{"unit", "units"}

// parseIndicators извлекает из JSON-результата анализа плоский список
// показателей. Поддерживаются несколько форматов (пробуем по очереди,
// см. требования):
//  1. Формат 1 (категории): categories[].lab_systems[].indicators[] и
//     вариации categories[].indicators[] / lab_systems[].indicators[].
//  2. Формат 2 (альтернативный): indicators[] на верхнем уровне.
//  3. Формат 3 (плоский): results{ имя: значение, норма_имя: интервал }.
//
// Если ни один формат не дал показателей - логируем предупреждение и
// возвращаем nil (анализ пропускается). Дубли по имени показателя
// игнорируются.
func parseIndicators(jsonStr string) []indicator {
	s := strings.TrimSpace(jsonStr)
	if s == "" {
		return nil
	}
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(s), &root); err != nil {
		log.Printf("[NOTIF] не удалось распарсить JSON анализа: %v", err)
		return nil
	}

	var out []indicator
	if out = parseFormatCategories(root); len(out) > 0 {
		return dedupeIndicators(out)
	}
	if out = parseFormatIndicators(root); len(out) > 0 {
		return dedupeIndicators(out)
	}
	if out = parseFormatResults(root); len(out) > 0 {
		return dedupeIndicators(out)
	}
	log.Printf("[NOTIF] неизвестный формат анализа, пропускаем (начало: %q)", truncate(s, 80))
	return nil
}

// parseFormatCategories - Формат 1: вложенные показатели анализа.
// Обрабатывает categories[].lab_systems[].indicators[], а также вариации
// categories[].indicators[] и lab_systems[].indicators[] (без обёртки
// categories). Возвращает все найденные показатели.
func parseFormatCategories(root map[string]interface{}) []indicator {
	var out []indicator
	collectSystems := func(systems []interface{}) {
		for _, sys := range systems {
			sysMap, ok := sys.(map[string]interface{})
			if !ok {
				continue
			}
			if inds, ok := sysMap["indicators"].([]interface{}); ok {
				out = append(out, extractIndicatorsFromList(inds)...)
			}
		}
	}
	if cats, ok := root["categories"].([]interface{}); ok {
		for _, cat := range cats {
			catMap, ok := cat.(map[string]interface{})
			if !ok {
				continue
			}
			if inds, ok := catMap["indicators"].([]interface{}); ok {
				out = append(out, extractIndicatorsFromList(inds)...)
			}
			if syss, ok := catMap["lab_systems"].([]interface{}); ok {
				collectSystems(syss)
			}
		}
	}
	if syss, ok := root["lab_systems"].([]interface{}); ok {
		collectSystems(syss)
	}
	return out
}

// parseFormatIndicators - Формат 2: массив indicators[] на верхнем уровне.
func parseFormatIndicators(root map[string]interface{}) []indicator {
	if inds, ok := root["indicators"].([]interface{}); ok {
		return extractIndicatorsFromList(inds)
	}
	return nil
}

// parseFormatResults - Формат 3: плоский результат results{}, где каждый
// ключ - имя показателя со значением (число или строка), а референсный
// интервал лежит в соседнем ключе норма_<имя> (или norm_<имя>).
func parseFormatResults(root map[string]interface{}) []indicator {
	return extractFromResults(root["results"])
}

// extractFromResults извлекает показатели из плоского объекта results{}.
func extractFromResults(raw interface{}) []indicator {
	results, ok := raw.(map[string]interface{})
	if !ok || len(results) == 0 {
		return nil
	}
	var out []indicator
	for key, val := range results {
		kl := strings.ToLower(strings.TrimSpace(key))
		if strings.HasPrefix(kl, "норма_") || strings.HasPrefix(kl, "norm_") {
			continue // это не показатель, а референсный ключ
		}
		sv, ok := nodeValue(val)
		if !ok {
			continue
		}
		out = append(out, indicator{Name: key, Value: sv, Normal: findNorm(results, key)})
	}
	return out
}

// findNorm ищет референсный интервал для показателя key в плоском объекте
// results{}. Сначала пробует точное совпадение "норма_<имя>" / "norm_<имя>",
// затем - поиск по основе слова без конечных гласных (stem), чтобы
// разбирать реальные лабораторные выгрузки с русским склонением без
// строгой унификации имён (например, "глюкоза" -> "норма_глюкозы").
// Возвращает "" если не найдено.
func findNorm(results map[string]interface{}, key string) string {
	stem := stripTrailingVowels(strings.ToLower(key))
	for ck, cv := range results {
		lower := strings.ToLower(strings.TrimSpace(ck))
		var suffix string
		switch {
		case strings.HasPrefix(lower, "норма_"):
			suffix = lower[len("норма_"):]
		case strings.HasPrefix(lower, "norm_"):
			suffix = lower[len("norm_"):]
		default:
			continue
		}
		if stripTrailingVowels(suffix) == stem {
			if s := asString(cv); s != "" {
				return s
			}
		}
	}
	return ""
}

// stripTrailingVowels удаляет подряд идущие конечные гласные русского и
// латинского алфавитов (нужно для сопоставления основы слова при поиске
// референсного ключа в плоском формате результатов).
func stripTrailingVowels(s string) string {
	return strings.TrimRight(s, "аеёиоуыэюяaeiouy")
}

// extractIndicatorsFromList извлекает показатели из массива объектов.
func extractIndicatorsFromList(list []interface{}) []indicator {
	var out []indicator
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if ind, ok := extractIndicator(m); ok {
			out = append(out, ind)
		}
	}
	return out
}

// extractIndicator пробует извлечь показатель из объекта. Показатель
// должен иметь непустое имя и значение (строку или число). Референсный
// интервал и единица извлекаются по списку возможных полей (могут быть
// пустыми, если в данных их нет).
func extractIndicator(node map[string]interface{}) (indicator, bool) {
	name, _ := node["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return indicator{}, false
	}
	val, ok := nodeValue(node["value"])
	if !ok || val == "" {
		return indicator{}, false
	}
	return indicator{
		Name:   name,
		Value:  val,
		Normal: findStringField(node, refFields),
		Unit:   findStringField(node, unitFields),
		Status: asString(node["status"]),
	}, true
}

// nodeValue преобразует значение JSON-поля в строку. Поддерживает строки и
// числа (float64/int/int64/json.Number). Возвращает ("", false), если
// значение отсутствует, равно null или не является строкой/числом.
func nodeValue(v interface{}) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "", false
	case string:
		return x, true
	case float64:
		return formatNumber(x), true
	case int:
		return strconv.Itoa(x), true
	case int64:
		return strconv.FormatInt(x, 10), true
	case json.Number:
		return x.String(), true
	case bool:
		return strconv.FormatBool(x), true
	}
	return "", false
}

// formatNumber форматирует число без лишних нулей (7.0 -> "7", 7.2 -> "7.2").
func formatNumber(f float64) string {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return ""
	}
	if f == math.Trunc(f) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// findStringField возвращает первое непустое строковое значение среди
// перечисленных ключей объекта (или "", если ни одного нет).
func findStringField(node map[string]interface{}, fields []string) string {
	for _, f := range fields {
		if v, ok := node[f]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return ""
}

// asString возвращает строковое значение поля или "".
func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// dedupeIndicators убирает дубликаты по имени показателя (оставляя первый).
func dedupeIndicators(in []indicator) []indicator {
	seen := make(map[string]bool, len(in))
	out := make([]indicator, 0, len(in))
	for _, ind := range in {
		if ind.Name == "" || seen[ind.Name] {
			continue
		}
		seen[ind.Name] = true
		out = append(out, ind)
	}
	return out
}

// truncate обрезает строку до n символов (для логов).
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 3 {
		return string(r[:n])
	}
	return string(r[:n-3]) + "..."
}

// isOutOfRange возвращает true, если показатель вышел за референсную норму.
// Сначала смотрим статус (warning/critical/risk - да, normal/good - нет);
// если статус пустой/неизвестный - сравниваем числовое значение с
// референсным интервалом из normal.
func isOutOfRange(ind indicator) bool {
	status := strings.ToLower(strings.TrimSpace(ind.Status))
	switch status {
	case "warning", "critical", "risk":
		return true
	case "normal", "good":
		return false
	}
	low, high, ok := parseRange(ind.Normal)
	if !ok {
		return false
	}
	val, ok := firstFloat(ind.Value)
	if !ok {
		return false
	}
	return val < low || val > high
}

// parseRange разбирает строку референсного интервала в (нижняя, верхняя
// граница). Поддерживает форматы: "a-b", "до X" (≤X), "≤X"/"<=X",
// "≥X"/">=X", "<X", ">X". Если граница не задана - бесконечность.
func parseRange(s string) (float64, float64, bool) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return 0, 0, false
	}
	s = strings.ReplaceAll(s, "≤", "<=")
	s = strings.ReplaceAll(s, "≥", ">=")
	s = strings.ToLower(s)
	if strings.HasPrefix(s, "до") {
		s = "<=" + strings.TrimSpace(s[len("до"):])
	}

	low := math.Inf(-1)
	high := math.Inf(1)

	switch {
	case strings.HasPrefix(s, "<="):
		if n := floatAfter(s, "<="); n != nil {
			high = *n
		}
	case strings.HasPrefix(s, ">="):
		if n := floatAfter(s, ">="); n != nil {
			low = *n
		}
	case strings.HasPrefix(s, "<"):
		if n := floatAfter(s, "<"); n != nil {
			high = *n
		}
	case strings.HasPrefix(s, ">"):
		if n := floatAfter(s, ">"); n != nil {
			low = *n
		}
	case strings.Contains(s, "-"):
		parts := strings.SplitN(s, "-", 2)
		l, err1 := parseNumber(parts[0])
		h, err2 := parseNumber(parts[1])
		if err1 == nil && err2 == nil {
			return l, h, true
		}
	}

	if math.IsInf(low, -1) && math.IsInf(high, 1) {
		return 0, 0, false
	}
	return low, high, true
}

func floatAfter(s, op string) *float64 {
	if !strings.HasPrefix(s, op) {
		return nil
	}
	n, err := parseNumber(s[len(op):])
	if err != nil {
		return nil
	}
	return &n
}

func parseNumber(s string) (float64, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", ".")
	return strconv.ParseFloat(s, 64)
}

// firstFloat извлекает первое вещественное число из строки (например, из
// "6.8" или "145"). Возвращает (значение, ok). Для "-" возвращает ok=false.
func firstFloat(s string) (float64, bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", ".")
	re := regexp.MustCompile(`-?\d+(\.\d+)?`)
	m := re.FindString(s)
	if m == "" || m == "-" {
		return 0, false
	}
	f, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// latestAnalysisIndicators возвращает показатели последнего сохранённого
// анализа пользователя (того, где есть показатели с референсами). Берёт
// до 10 последних записей истории, выбирает первую подходящую.
func (s *Service) latestAnalysisIndicators(ctx context.Context, telegramID int64) ([]indicator, bool) {
	if s.monitorRepo == nil {
		return nil, false
	}
	entries, _, err := s.monitorRepo.ListHistory(ctx, telegramID, "", 1, 10)
	if err != nil || len(entries) == 0 {
		return nil, false
	}
	for _, e := range entries {
		if inds := parseIndicators(e.JsonData); len(inds) > 0 {
			return inds, true
		}
	}
	return nil, false
}

// notificationsEnabledUser возвращает true, если пользователь не отключал
// уведомления. При отсутствии записи предпочтений (дефолт) - true.
func (s *Service) notificationsEnabledUser(ctx context.Context, userID uint) bool {
	if s.store == nil {
		return true
	}
	p, err := s.store.Preferences.GetPreferences(ctx, userID)
	if err != nil {
		return true
	}
	return p.NotificationsEnabled
}

// SetUserNotificationsEnabled включает/отключает уведомления об отклонениях
// в анализах для пользователя (команды /notifications on|off). При
// отсутствии записи предпочтений - создаёт её.
func (s *Service) SetUserNotificationsEnabled(ctx context.Context, telegramID int64, enabled bool) error {
	if s.store == nil {
		return fmt.Errorf("storage not initialized")
	}
	u, err := s.store.Users.GetUserByTelegramID(ctx, telegramID)
	if err != nil {
		return err
	}
	p, perr := s.store.Preferences.GetPreferences(ctx, u.ID)
	if perr != nil {
		p = &sm.Preference{UserID: u.ID}
	}
	p.NotificationsEnabled = enabled
	return s.store.Preferences.UpdatePreferences(ctx, p)
}

// GetUserNotificationsEnabled возвращает текущий статус уведомлений
// пользователя. При отсутствии записи предпочтений - true (дефолт).
func (s *Service) GetUserNotificationsEnabled(ctx context.Context, telegramID int64) (bool, error) {
	if s.store == nil {
		return true, fmt.Errorf("storage not initialized")
	}
	u, err := s.store.Users.GetUserByTelegramID(ctx, telegramID)
	if err != nil {
		return true, err
	}
	p, perr := s.store.Preferences.GetPreferences(ctx, u.ID)
	if perr != nil {
		return true, perr
	}
	return p.NotificationsEnabled, nil
}

// sendNotification отправляет текстовое уведомление пользователю. Возвращает
// true, если отправка успешна (или заменена тестовым sendFn).
func (s *Service) sendNotification(ctx context.Context, chatID int64, text string) bool {
	if s.sendFn != nil {
		return s.sendFn(ctx, chatID, text)
	}
	if s.botClient == nil {
		log.Printf("[NOTIF] bot-клиент не задан, пропускаем отправку chatID=%d", chatID)
		return false
	}
	_, err := botutil.SendSafe(ctx, s.botClient, tgbot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "Markdown",
	})
	if err != nil {
		log.Printf("[NOTIF] не удалось отправить chatID=%d: %v", chatID, err)
		return false
	}
	return true
}

// AnalyticsFinding - одно найденное отклонение показателя от нормы
// (для предпросмотра проверки анализов).
type AnalyticsFinding struct {
	Name   string
	Value  string
	Normal string
	Unit   string
}

// formatDeviationText формирует текст уведомления об отклонении показателя:
// «⚠️ {показатель}: {значение}{единица} при норме {норма}{единица}.
// Рекомендуем обновить анализ.» Единица измерения добавляется только при
// наличии в данных анализа (через пробел после значения и нормы).
func formatDeviationText(name, value, norm, unit string) string {
	if unit != "" {
		if value != "" {
			value += " " + unit
		}
		if norm != "" {
			norm += " " + unit
		}
	}
	return fmt.Sprintf(locales.MsgNotifAnalyticsDeviation, name, value, norm)
}

// formatDeviation - обёртка formatDeviationText для извлечённого показателя.
func formatDeviation(ind indicator) string {
	return formatDeviationText(ind.Name, ind.Value, ind.Normal, ind.Unit)
}

// formatDeviationLine формирует ОДНУ строку списка отклонений (без
// ведущего эмодзи и без итогового «Рекомендуем обновить анализ.» - его
// добавляет formatDeviations обёрткой). Единица измерения добавляется при
// наличии (через пробел после значения и нормы).
func formatDeviationLine(name, value, norm, unit string) string {
	if unit != "" {
		if value != "" {
			value += " " + unit
		}
		if norm != "" {
			norm += " " + unit
		}
	}
	return fmt.Sprintf("• %s: %s при норме %s", name, value, norm)
}

// formatDeviations объединяет все вышедшие за норму показатели в ОДНО
// уведомление (вместо отдельного сообщения на каждый показатель), чтобы не
// спамить пользователя при нескольких отклонениях сразу.
func formatDeviations(inds []indicator) string {
	var bld strings.Builder
	for i, ind := range inds {
		if i > 0 {
			bld.WriteString("\n")
		}
		bld.WriteString(formatDeviationLine(ind.Name, ind.Value, ind.Normal, ind.Unit))
	}
	return fmt.Sprintf(locales.MsgNotifAnalyticsDeviationList, bld.String())
}

// SendSubscriptionTest отправляет ТЕСТОВОЕ уведомление об окончании
// подписки заданного типа (daysBefore ∈ {7,3,1,0}) немедленно в указанный
// чат. Используется dev-командами и тестовыми кнопками. Возвращает
// отправленный текст.
func (s *Service) SendSubscriptionTest(ctx context.Context, chatID int64, daysBefore int) (string, error) {
	text := subscriptionText(daysBefore)
	if text == "" {
		return "", fmt.Errorf("unknown subscription test kind %d", daysBefore)
	}
	s.sendNotification(ctx, chatID, text)
	return text, nil
}

// ErrNoAnalysisData - у пользователя нет сохранённых анализов (для
// честного различения «нет анализов» и «анализы в норме»).
var ErrNoAnalysisData = errors.New("no analysis data")

// RunAnalyticsDryRun проверяет сохранённые анализы пользователя и
// возвращает список найденных отклонений БЕЗ реальной отправки и без записи
// в БД. Используется dev-кнопкой «Проверить» (предпросмотр). Возвращает
// ErrNoAnalysisData, если анализов нет; иначе - срез отклонений (возможно
// пустой, если все показатели в норме).
func (s *Service) RunAnalyticsDryRun(ctx context.Context, chatID int64) ([]AnalyticsFinding, error) {
	inds, ok := s.latestAnalysisIndicators(ctx, chatID)
	if !ok || len(inds) == 0 {
		return nil, ErrNoAnalysisData
	}
	var findings []AnalyticsFinding
	for _, ind := range inds {
		if isOutOfRange(ind) {
			findings = append(findings, AnalyticsFinding{Name: ind.Name, Value: ind.Value, Normal: ind.Normal, Unit: ind.Unit})
		}
	}
	return findings, nil
}

// SendAnalyticsTest выполняет реальную проверку и отправку уведомлений об
// отклонениях для указанного пользователя (с учётом подавления на 14 дней).
// Используется dev-кнопкой «Отправить». Возвращает число отправленных
// уведомлений (0, если отклонений нет или все подавлены) и ErrNoAnalysisData,
// если сохранённых анализов нет.
func (s *Service) SendAnalyticsTest(ctx context.Context, chatID int64) (int, error) {
	inds, ok := s.latestAnalysisIndicators(ctx, chatID)
	if !ok || len(inds) == 0 {
		return 0, ErrNoAnalysisData
	}
	now := time.Now()
	var toNotify []indicator
	for _, ind := range inds {
		if !isOutOfRange(ind) {
			continue
		}
		if suppressed, _ := s.repo.isSuppressed(ctx, chatID, ind.Name, now); suppressed {
			continue
		}
		toNotify = append(toNotify, ind)
	}
	if len(toNotify) == 0 {
		return 0, nil
	}
	text := formatDeviations(toNotify)
	sent := 0
	sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	if s.sendNotification(sendCtx, chatID, text) {
		for _, ind := range toNotify {
			_ = s.repo.suppress(ctx, chatID, ind.Name, now.Add(14*24*time.Hour))
		}
		sent = len(toNotify)
	}
	cancel()
	return sent, nil
}

// DryRunMessage формирует текст предпросмотра проверки анализов из списка
// найденных отклонений (используется dev-кнопкой/командой «Проверить»).
func (s *Service) DryRunMessage(findings []AnalyticsFinding) string {
	var body string
	if len(findings) == 0 {
		body = locales.MsgNotifAnalyticsNone
	} else {
		var bld strings.Builder
		for _, f := range findings {
			bld.WriteString("• ")
			bld.WriteString(formatDeviationText(f.Name, f.Value, f.Normal, f.Unit))
			bld.WriteString("\n")
		}
		body = bld.String()
	}
	return fmt.Sprintf(locales.MsgNotifAnalyticsDryRun, body)
}
