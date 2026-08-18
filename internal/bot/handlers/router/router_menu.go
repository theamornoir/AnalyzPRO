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
// анализ, Bioscan, Мой профиль, Мониторинг, Консультация, Отзывы, О сервисе)
// доступны через inline-кнопки (callback), а не через кнопки reply-клавиатуры,
// поэтому здесь не перечисляются.
func (r *router) isMainMenuButton(text string) bool {
	switch text {
	case locales.BtnAnalysisHub,
		locales.BtnHealthHub,
		locales.BtnPremium,
		locales.BtnServiceHub,
		// Под-действия раздела «Анализы» могут прийти как текстовые кнопки
		// (например, из сохранённой клавиатуры). Чтобы они не ушли в
		// анализ текста/ИИ, ловим их до состояний потока.
		locales.BtnRegularAnalysis,
		locales.BtnExtendedAnalysis,
		locales.BtnBioscan,
		locales.BtnBioscanBasic,
		locales.BtnBioscanExtended:
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
		return r.renderHub(ctx, b, chatID, "analysis")

	case locales.BtnHealthHub:
		return r.renderHub(ctx, b, chatID, "health")

	case locales.BtnServiceHub:
		return r.renderHub(ctx, b, chatID, "service")

	case locales.BtnPremium:
		log.Printf(locales.LogRouterMenuPremium, chatID)
		// Уходим из главного меню в экран Premium - убираем закреплённое
		// сообщение главного меню, чтобы оно не висело над экраном тарифов.
		r.deleteMainMenuMessage(ctx, b, chatID)
		r.setCurrentSection(chatID, "premium")
		menu.PremiumHandler(r.stateManager, r.paymentService)(ctx, b, update)
		return true

	// Под-действия раздела «Анализы»: если пришли как текстовые кнопки
	// (старая/сохранённая клавиатура), запускаем соответствующий анализ,
	// а не отправляем текст в ИИ (handleText).
	case locales.BtnRegularAnalysis:
		return r.handleRegularAnalysis(ctx, b, chatID)
	case locales.BtnExtendedAnalysis:
		return r.handleExtendedAnalysis(ctx, b, chatID)
	case locales.BtnBioscan:
		// Устаревшая текстовая кнопка «📸 Bioscan» (из старой клавиатуры) -
		// безопасно направляем в бесплатный базовый режим.
		return r.handleBioscanBasicStart(ctx, b, chatID)
	case locales.BtnBioscanBasic:
		return r.handleBioscanBasicStart(ctx, b, chatID)
	case locales.BtnBioscanExtended:
		return r.handleBioscanExtendedStart(ctx, b, chatID)
	}

	return false
}
