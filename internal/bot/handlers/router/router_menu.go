package router

import (
	"context"
	"log"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/menu"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// isMainMenuButton - true, если text совпадает с одной из кнопок главного меню.
func (r *router) isMainMenuButton(text string) bool {
	switch text {
	case locales.BtnAbout,
		locales.BtnDiagnostics,
		locales.BtnRegularAnalysis,
		locales.BtnExtendedAnalysis,
		locales.BtnBioscan,
		locales.BtnFeedback,
		locales.BtnPremium,
		locales.BtnDashboard:
		return true
	}
	return false
}

// handleMenuButtons - обработка кнопок главного меню. Возвращает true, если обработано.
func (r *router) handleMenuButtons(ctx context.Context, b *tgbot.Bot, chatID int64, text string, update *models.Update) bool {
	if text == "" {
		return true
	}

	log.Printf(locales.LogRouterMenuButton, text, chatID)

	switch text {
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
		menu.PremiumHandler(r.stateManager, r.paymentService)(ctx, b, update)
		return true

	case locales.BtnDashboard:
		return r.handleDashboard(ctx, b, chatID)
	}

	return false
}
