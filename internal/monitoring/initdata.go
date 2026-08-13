package monitoring

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// initDataUser — часть полезной нагрузки initData (поле "user").
type initDataUser struct {
	ID int64 `json:"id"`
}

// ValidateInitData проверяет подлинность Telegram Web App initData по схеме:
//
//	data_check_str  = отсортированные "key=value" (кроме hash), через "\n"
//	hash            = HEX(HMAC_SHA256(secret_key, data_check_str))
//
// В официальной документации Telegram и в эталонной библиотеке
// telegram-mini-apps/init-data-golang есть расхождение в порядке аргументов
// HMAC для secret_key: один источник утверждает, что secret_key =
// HMAC_SHA256(<bot_token>, "WebAppData") (бот-токен — КЛЮЧ), другой — что
// HMAC_SHA256("WebAppData", <bot_token>). Чтобы не зависеть от версии и не
// гадать, принимаем initData, если совпадает ЛЮБОЙ из двух порядков, и логируем,
// какой именно подошёл.
//
// Дополнительно проверяем auth_date (не старше 24ч), чтобы исключить
// replay-атаки. При неудаче пишем в лог конкретную причину (включая bot_id
// токена сервера и signedUserID из initData) — это помогает отличить
// «неправильный порядок HMAC» от «токен сервера ≠ токену, которым Telegram
// подписал initData».
//
// Подробнее: https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app
func ValidateInitData(initData, botToken string) (int64, bool) {
	id, ok, reason := validateInitDataDetailed(initData, botToken)
	if !ok {
		log.Printf("[INITDATA] валидация НЕ пройдена: %s", reason)
	} else if id == 0 {
		log.Printf("[INITDATA] валидация пройдена, но telegramID=0 (нет поля user/id в initData)")
	} else {
		log.Printf("[INITDATA] валидация ОК: telegramID=%d", id)
	}
	return id, ok
}

// botID возвращает идентификатор бота (часть до первого ':') из токена —
// используется в диагностике, чтобы сверить: тот ли BOT_TOKEN запущен на
// сервере, что подписал initData.
func botID(token string) string {
	if i := strings.Index(token, ":"); i > 0 {
		return token[:i]
	}
	return "<no-colon>"
}

// validateInitDataDetailed — то же, что ValidateInitData, но возвращает
// человекочитаемую причину отказа для логов.
func validateInitDataDetailed(initData, botToken string) (int64, bool, string) {
	if strings.TrimSpace(initData) == "" {
		return 0, false, "initData пустой (не передан из Mini App)"
	}
	// Защитно обрезаем возможные пробелы/переводы строк в токене — частая
	// причина несовпадения hash при чтении BOT_TOKEN из .env.
	botToken = strings.TrimSpace(botToken)
	if botToken == "" {
		return 0, false, "botToken пустой (не сконфигурирован)"
	}

	values, err := url.ParseQuery(initData)
	if err != nil {
		return 0, false, fmt.Sprintf("ошибка разбора initData: %v", err)
	}

	hash := values.Get("hash")
	if hash == "" {
		return 0, false, "в initData отсутствует поле hash"
	}

	// auth_date защита от replay.
	if ad := values.Get("auth_date"); ad != "" {
		if ts, err := strconv.ParseInt(ad, 10, 64); err == nil {
			age := time.Since(time.Unix(ts, 0))
			if age > 24*time.Hour {
				return 0, false, fmt.Sprintf("auth_date слишком старый: ~%d ч назад (>24ч), replay-защита", int64(age.Hours()))
			}
		}
	}

	// Собираем data_check_string из всех полей, кроме hash.
	keys := make([]string, 0, len(values))
	for k := range values {
		if k == "hash" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(values.Get(k))
	}
	dataCheckString := sb.String()

	// Извлекаем telegramID из поля user (нужен и для успеха, и для диагностики).
	userRaw := values.Get("user")
	var u initDataUser
	if userRaw != "" {
		_ = json.Unmarshal([]byte(userRaw), &u)
	}

	// В документации Telegram и в эталонной библиотеке init-data-golang есть
	// расхождение в порядке аргументов HMAC для secret_key. Проверяем оба
	// варианта и принимаем initData, если совпадает любой из них:
	//   A) secret = HMAC_SHA256(<bot_token>, "WebAppData")
	//   B) secret = HMAC_SHA256("WebAppData", <bot_token>)
	secretA := hmacSHA256([]byte(botToken), []byte("WebAppData"))
	computedA := hex.EncodeToString(hmacSHA256(secretA, []byte(dataCheckString)))

	secretB := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	computedB := hex.EncodeToString(hmacSHA256(secretB, []byte(dataCheckString)))

	if computedA == hash {
		return u.ID, true, ""
	}
	if computedB == hash {
		log.Printf("[INITDATA] совпал альтернативный порядок HMAC (B: key=WebAppData) — server botID=%s", botID(botToken))
		return u.ID, true, ""
	}

	// Ни один порядок не подошёл: почти наверняка токен на сервере отличается
	// от токена, с которым Telegram подписал initData. Логируем всё необходимое
	// для сравнения: bot_id сервера, id подписанного пользователя и оба computed.
	return 0, false, fmt.Sprintf("несовпадение hash (оба порядка HMAC не подошли). serverBotID=%q (len=%d) | signedUserID=%d authDate=%s | computedA(key=botToken)=%s computedB(key=WebAppData)=%s expected=%s",
		botID(botToken), len(botToken), u.ID, values.Get("auth_date"), computedA, computedB, hash)
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}
