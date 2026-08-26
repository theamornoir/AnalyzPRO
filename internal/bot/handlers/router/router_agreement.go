package router

import (
	"context"
	"log"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// handleAgreement - проверка соглашения. Возвращает true, если сообщение обработано.
func (r *router) handleAgreement(ctx context.Context, b *tgbot.Bot, chatID int64, text string) bool {
	if r.agreementStorage.IsAgreed(chatID) {
		return false
	}

	log.Printf(locales.LogAgreementNotAccepted, chatID)

	switch text {
	case locales.BtnAgreement:
		log.Printf(locales.LogShowingAgreementText)
		// Текст соглашения + инлайн-кнопка «Принять» (onboarding_accept).
		// Reply-клавиатуру НЕ используем - навигация только инлайн, чтобы
		// не плодить «меню в реплай плюс меню в инлайн» (см. общий дизайн).
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   keyboards.UserAgreementText(),
			ReplyMarkup: models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{
					{{Text: locales.BtnAcceptAgreement, CallbackData: "onboarding_accept"}},
				},
			},
			ParseMode: "Markdown",
		})
		return true

	case locales.BtnAcceptAgreement:
		log.Printf(locales.LogRouterAgreeAccept, chatID)
		r.agreementStorage.SetAgreed(chatID)
		r.stateManager.SetState(chatID, states.StateIdle)

		// Снимаем висящую reply-клавиатуру (могла остаться от AgreementMenu),
		// чтобы навигация была ТОЛЬКО inline - без «меню в реплай плюс меню
		// в инлайн». ReplyKeyboardRemove убирает клавиатуру, а главное меню
		// показываем inline (edit-in-place), как после онбординга.
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgAgreementAccepted,
			ReplyMarkup: models.ReplyKeyboardRemove{RemoveKeyboard: true},
			ParseMode:   "Markdown",
		})
		r.showMainMenuMessage(ctx, b, chatID, locales.MsgStartWelcomeBack)
		return true
	}

	if text != "" && text != "/start" {
		log.Printf(locales.LogRouterAgreePrompt, chatID)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgAgreementPrompt,
			ReplyMarkup: models.ReplyKeyboardRemove{RemoveKeyboard: true},
			ParseMode:   "Markdown",
		})
		return true
	}

	return false
}
