package helpers

import (
	"context"
	"strings"

	tgbot "github.com/go-telegram/bot"
)

// MaxMessageChunk - максимальная длина одного текстового сообщения с запасом
// до лимита Telegram API (4096 символов). Режем по 4000 рун, чтобы
// оставить буфер под служебные символы.
const MaxMessageChunk = 4000

// SplitLongMessage разбивает текст на куски длиной <= maxChunk по границам
// строк, чтобы не упереться в лимит Telegram (4096 символов). Чистая
// функция - удобна для юнит-тестов без бота. Пустая строка возвращает nil.
func SplitLongMessage(text string, maxChunk int) []string {
	if maxChunk <= 0 {
		maxChunk = MaxMessageChunk
	}
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return nil
	}
	if n <= maxChunk {
		return []string{text}
	}

	chunks := []string{}
	for start := 0; start < n; {
		end := start + maxChunk
		if end > n {
			end = n
		}
		chunk := string(runes[start:end])
		if end < n {
			// Сдвигаем границу к последнему переносу строки внутри куска,
			// чтобы не разрывать строку посреди предложения (если возможно).
			if idx := strings.LastIndex(chunk, "\n"); idx > 0 {
				end = start + idx + 1
				chunk = string(runes[start:end])
			}
		}
		chunks = append(chunks, chunk)
		start = end
	}
	return chunks
}

// SendLongMessagePlain отправляет текст в чат, разбивая на куски <= 4000
// символов (лимит Telegram - 4096). Клавиатура не крепится - меню
// присылается отдельным сообщением после сохранения результата. Без этого
// длинные отчёты ИИ (до ~6-10k символов на кириллице) не доставлялись бы в
// чат (Telegram вернул бы 400), хотя и сохранялись бы в «Мой профиль».
func SendLongMessagePlain(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	for _, chunk := range SplitLongMessage(text, MaxMessageChunk) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: chunk})
	}
}
