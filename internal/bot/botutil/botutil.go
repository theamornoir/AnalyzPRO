package botutil

import (
	"context"
	"fmt"
	"log"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// SendSafe sends a message and, if Telegram returns an error (for example a
// Bad Request caused by malformed Markdown or an invalid WebApp button),
// retries with a "safe" version: no ParseMode and no keyboard. This guarantees
// the user receives at least the text instead of silent silence.
//
// In both the success and failure cases it logs enough detail (chatID,
// returned MessageID, or the exact Telegram error) so that a missing/invisible
// message becomes diagnosable from the logs - otherwise SendMessage errors used
// to be silently ignored via `_, _ = ...`.
func SendSafe(ctx context.Context, b *tgbot.Bot, p tgbot.SendMessageParams) (int, error) {
	msg, err := b.SendMessage(ctx, &p)
	if err != nil {
		// Пытаемся понять причину: часто это некорректный Markdown или WebApp-кнопка.
		log.Printf("SEND FAILED (chatID=%d, parseMode=%q): %v - повторяем без форматирования/клавиатуры",
			p.ChatID, p.ParseMode, err)

		plain := tgbot.SendMessageParams{
			ChatID:    p.ChatID,
			Text:      p.Text,
			ParseMode: "",
		}
		msg2, err2 := b.SendMessage(ctx, &plain)
		if err2 != nil {
			log.Printf("SEND RETRY FAILED (chatID=%d): %v", p.ChatID, err2)
			return 0, fmt.Errorf("send retry failed: %w", err2)
		}
		log.Printf("SEND RETRY OK (chatID=%d, messageID=%d)", p.ChatID, msg2.ID)
		return msg2.ID, nil
	}

	log.Printf("SEND OK (chatID=%d, messageID=%d, hasReplyMarkup=%v)",
		p.ChatID, msg.ID, p.ReplyMarkup != nil)
	return msg.ID, nil
}

// SendConfirmed - обёртка, которая шлёт сообщение и явно логирует результат.
// Отличается от SendSafe тем, что НЕ пытается «спасти» сообщение без
// форматирования - используется там, где важен именно оригинальный формат.
func SendConfirmed(ctx context.Context, b *tgbot.Bot, p tgbot.SendMessageParams) (int, error) {
	msg, err := b.SendMessage(ctx, &p)
	if err != nil {
		log.Printf("SEND ERROR (chatID=%d, parseMode=%q): %v", p.ChatID, p.ParseMode, err)
		return 0, err
	}
	log.Printf("SEND OK (chatID=%d, messageID=%d)", p.ChatID, msg.ID)
	return msg.ID, nil
}

// AnswerLogged - отвечает на callback-запрос и логирует статус (важно: если
// на callback не ответить, кнопка «висит» и пользователь видит, что ничего
// не происходит, хотя логика отработала).
func AnswerLogged(ctx context.Context, b *tgbot.Bot, p tgbot.AnswerCallbackQueryParams) {
	if _, err := b.AnswerCallbackQuery(ctx, &p); err != nil {
		log.Printf("ANSWER CALLBACK FAILED (callbackID=%s): %v", p.CallbackQueryID, err)
		return
	}
	log.Printf("ANSWER CALLBACK OK (callbackID=%s, text=%q)", p.CallbackQueryID, p.Text)
}

// ExtractChatID - безопасно извлекает chatID из update (callback или message).
func ExtractChatID(update *models.Update) int64 {
	if update == nil {
		return 0
	}
	if update.CallbackQuery != nil {
		return update.CallbackQuery.From.ID
	}
	if update.Message != nil {
		return update.Message.Chat.ID
	}
	return 0
}
