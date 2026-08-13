package router

import (
	"context"
	"log"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/menu"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// isMainMenuButton - true, если text совпадает с одной из кнопок главного
// меню (4 верхних раздела-хаба). Под-действия хабов (Обычный/Расширенный
// анализ, Bioscan, Сводка, Мониторинг, Консультация, Отзывы, О сервисе)
// доступны через inline-кнопки (callback), а не через кнопки reply-клавиатуры,
// поэтому здесь не перечисляются.
func (r *router) isMainMenuButton(text string) bool {
	switch text {
	case locales.BtnAnalysisHub,
		locales.BtnHealthHub,
		locales.BtnPremium,
		locales.BtnServiceHub:
		return true
	}
	return false
}

// handleMenuButtons - обработка кнопок главного меню (4 раздела-хаба).
// Возвращает true, если обработано.
func (r *router) handleMenuButtons(ctx context.Context, b *tgbot.Bot, chatID int64, text string, update *models.Update) bool {
	if text == "" {
		return true
	}

	log.Printf(locales.LogRouterMenuButton, text, chatID)

	switch text {
	case locales.BtnAnalysisHub:
		return r.handleAnalysisHub(ctx, b, chatID)

	case locales.BtnHealthHub:
		return r.handleHealthHub(ctx, b, chatID)

	case locales.BtnServiceHub:
		return r.handleServiceHub(ctx, b, chatID)

	case locales.BtnPremium:
		log.Printf(locales.LogRouterMenuPremium, chatID)
		menu.PremiumHandler(r.stateManager, r.paymentService)(ctx, b, update)
		return true
	}

	return false
}
