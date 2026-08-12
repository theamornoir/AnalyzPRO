package router

import (
	"context"
	"log"

	tgbot "github.com/go-telegram/bot"

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
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        keyboards.UserAgreementText(),
			ReplyMarkup: keyboards.AgreementMenu(),
			ParseMode:   "Markdown",
		})
		return true

	case locales.BtnAcceptAgreement:
		log.Printf(locales.LogRouterAgreeAccept, chatID)
		r.agreementStorage.SetAgreed(chatID)
		r.stateManager.SetState(chatID, states.StateIdle)

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgAgreementAccepted,
			ReplyMarkup: keyboards.MainMenu(),
			ParseMode:   "Markdown",
		})
		return true
	}

	if text != "" && text != "/start" {
		log.Printf(locales.LogRouterAgreePrompt, chatID)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgAgreementPrompt,
			ReplyMarkup: keyboards.StartMenu(),
			ParseMode:   "Markdown",
		})
		return true
	}

	return false
}
