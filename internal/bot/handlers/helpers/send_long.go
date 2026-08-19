package helpers

import (
	"context"
	"unicode/utf8"

	tgbot "github.com/go-telegram/bot"
)

// MaxMessageChunk - максимальный размер ОДНОГО текстового сообщения в
// БАЙТАХ (а не рунах!), с большим запасом до жёсткого лимита Telegram API
// (4096 байт). Telegram считает лимит в байтах, а кириллица/эмодзи занимают
// по 2-4 байта на символ - поэтому режем по 3500 байт, чтобы даже длинная
// «сплошная» строка на кириллице (где негде вставить перенос) гарантированно
// уложилась в 4096 и не вернула 400.
const MaxMessageChunk = 3500

// SplitLongMessage разбивает текст на куски размером <= maxChunk БАЙТ, чтобы
// не упереться в лимит Telegram (4096 байт). Разбиение предпочтительно идёт
// по границам переноса строки (не рвём предложение посреди строки и тем
// более посреди многобайтовой руны). Если одна строка сама длиннее лимита -
// она режется принудительно по байтам (крайний случай: Telegram всё равно
// вернул бы 400 на такой гигант). Чистая функция - удобна для юнит-тестов
// без бота. Пустая строка возвращает nil.
func SplitLongMessage(text string, maxChunk int) []string {
	if maxChunk <= 0 {
		maxChunk = MaxMessageChunk
	}

	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return nil
	}

	// Весь текст влезает в байтовый лимит - вернуть как есть.
	if len(text) <= maxChunk {
		return []string{text}
	}

	chunks := []string{}
	start := 0
	for start < n {
		end := start
		byteLen := 0
		lastNL := -1
		// Набираем кусок, пока влезает по байтам и не разрываем руну.
		for end < n {
			r := runes[end]
			rb := utf8.RuneLen(r)
			if byteLen+rb > maxChunk {
				break
			}
			byteLen += rb
			if r == '\n' {
				lastNL = end
			}
			end++
		}
		// Если внутри набранного куска есть перенос строки - сдвигаем
		// границу к нему, чтобы не разрывать строку посреди предложения.
		if end < n && lastNL > start {
			end = lastNL + 1
		}
		chunks = append(chunks, string(runes[start:end]))
		start = end
	}
	return chunks
}

// SendLongMessagePlain отправляет текст в чат, разбивая на куски <= 3500
// байт (лимит Telegram - 4096 байт). Клавиатура не крепится - меню
// присылается отдельным сообщением после сохранения результата. Без этого
// длинные отчёты ИИ не доставлялись бы в чат (Telegram вернул бы 400), хотя и
// сохранялись бы в «Мой профиль».
func SendLongMessagePlain(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	for _, chunk := range SplitLongMessage(text, MaxMessageChunk) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: chunk})
	}
}
