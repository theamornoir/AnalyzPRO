package router

import (
	"context"
	"log"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/menu"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// handleMenuButtons - обработка кнопок главного меню. Возвращает true, если обработано.
func (r *router) handleMenuButtons(ctx context.Context, b *tgbot.Bot, chatID int64, text string, update *models.Update) bool {
	if text == "" {
		return true
	}

	log.Printf(locales.LogRouterMenuButton, text, chatID)

	switch text {
	case locales.BtnAgreement:
		return r.handleAgreementButton(ctx, b, chatID)

	case locales.BtnAcceptAgreement:
		return r.handleAcceptAgreement(ctx, b, chatID)

	case locales.BtnAbout:
		log.Printf(locales.LogRouterMenuAbout, chatID)
		menu.AboutHandler()(ctx, b, update)
		return true

	case locales.BtnDiagnostics:
		return r.handleDiagnostics(ctx, b, chatID)

	case locales.BtnRegularAnalysis:
		return r.handleRegularAnalysis(ctx, b, chatID)

	case locales.BtnExtendedAnalysis:
		return r.handleExtendedAnalysis(ctx, b, chatID)

	case locales.BtnBioscan:
		return r.handleBioscanStart(ctx, b, chatID)

	case locales.BtnFeedback:
		log.Printf(locales.LogProcessingFeedback, chatID)
		menu.FeedbackHandler(r.adminChatID)(ctx, b, update)
		return true

	case locales.BtnPremium:
		log.Printf(locales.LogRouterMenuPremium, chatID)
		menu.PremiumHandler()(ctx, b, update)
		return true
	}

	return false
}
