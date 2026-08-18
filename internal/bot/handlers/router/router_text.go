package router

import (
	"context"
	"log"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// handleText - обработка произвольного текста в главном меню.
// Бот НЕ отправляет свободный текст в ИИ (это приводило к
// непредсказуемым ответам на не-медицинские запросы, например на
// «проанализируй соглашение»). Вместо этого предлагаем выбрать
// действие через меню: Анализы (расшифровка файлов/Bioscan) или
// Быстрая консультация (вопрос ИИ).
func (r *router) handleText(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	log.Printf(locales.LogRouterTextProcessing, chatID)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   locales.MsgFreeTextHint,
		ReplyMarkup: models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: locales.BtnAnalysisHub, CallbackData: "section_analysis"},
					{Text: locales.BtnConsultation, CallbackData: "section_consult_start"},
				},
			},
		},
	})
}
