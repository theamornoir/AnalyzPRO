// Package onboarding реализует короткий онбординг для новых пользователей:
// два сообщения - (1) единое описание всего функционала бота и (2) согласие
// на обработку персональных данных со ссылкой на полный текст. После
// принятия согласия пользователь попадает в главное меню
// (см. router.handleOnboarding).
package onboarding

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// pdAgreementURL - короткая ссылка на полный текст согласия на обработку
// персональных данных (размещён на внешнем ресурсе, без локального HTML).
const pdAgreementURL = "https://clck.ru/3VSSPo"

// SendIntro отправляет 1-е сообщение онбординга: единое описание всего
// функционала бота. Inline-кнопка «📝 Согласие» ведёт ко 2-му сообщению
// (onboarding_agreement).
func SendIntro(ctx context.Context, b *tgbot.Bot, chatID int64) {
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    chatID,
		Text:      locales.MsgOnboardingIntro,
		ParseMode: "Markdown",
		ReplyMarkup: models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: locales.BtnOnboardingAgreement, CallbackData: "onboarding_agreement"},
				},
			},
		},
	})
}

// SendAgreement отправляет 2-е сообщение онбординга: краткое согласие на
// обработку персональных данных + ссылку на полный текст согласия
// (pdAgreementURL). Inline-кнопка «✅ Принять» фиксирует согласие и
// открывает главное меню (onboarding_accept).
func SendAgreement(ctx context.Context, b *tgbot.Bot, chatID int64) {
	text := locales.MsgOnboardingAgreement
	text += "\n\n📄 Полный текст согласия: " + pdAgreementURL
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "Markdown",
		ReplyMarkup: models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: locales.BtnOnboardingAccept, CallbackData: "onboarding_accept"},
				},
			},
		},
	})
}
