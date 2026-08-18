// Package onboarding реализует минимальный онбординг для новых
// пользователей: слайдер из 4 шагов (inline-кнопка «Дальше») и в конце -
// пользовательское соглашение с кнопкой «Принять». Каждый шаг - отдельное
// сообщение; предыдущее удаляется при переходе (см. router.handleOnboarding),
// чтобы не плодить историю чата.
package onboarding

import (
	"context"
	"fmt"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// Steps - тексты шагов онбординга (индекс 0 = шаг 1). Число шагов
// определяется длиной среза: динамически обрабатываются и кнопки «Дальше»,
// и финальный переход к соглашению (router.handleOnboarding).
var Steps = []string{
	locales.MsgOnboardingStep1,
	locales.MsgOnboardingStep2,
	locales.MsgOnboardingStep3,
	locales.MsgOnboardingStep4,
	locales.MsgOnboardingStep5,
	locales.MsgOnboardingStep6,
	locales.MsgOnboardingStep7,
	locales.MsgOnboardingStep8,
}

// nextCallbackData возвращает callback-данные кнопки на шаге step (1..N,
// где N = len(Steps)). С последнего шага кнопка ведёт к соглашению.
func nextCallbackData(step int) string {
	if step >= 1 && step < len(Steps) {
		return fmt.Sprintf("onboarding_step_%d", step+1)
	}
	return "onboarding_agreement"
}

// stepButtonText - текст кнопки для шага step (последний шаг ведёт к
// соглашению, остальные - «Дальше»). Число шагов = len(Steps).
func stepButtonText(step int) string {
	if step == len(Steps) {
		return locales.BtnOnboardingAgreement
	}
	return locales.BtnOnboardingNext
}

// SendStep отправляет шаг онбординга step (1..4) с inline-кнопкой.
func SendStep(ctx context.Context, b *tgbot.Bot, chatID int64, step int) {
	if step < 1 || step > len(Steps) {
		return
	}
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    chatID,
		Text:      Steps[step-1],
		ParseMode: "Markdown",
		ReplyMarkup: models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: stepButtonText(step), CallbackData: nextCallbackData(step)},
				},
			},
		},
	})
}

// SendAgreement отправляет текст соглашения с inline-кнопкой «Принять».
func SendAgreement(ctx context.Context, b *tgbot.Bot, chatID int64) {
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    chatID,
		Text:      locales.UserAgreementText,
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
