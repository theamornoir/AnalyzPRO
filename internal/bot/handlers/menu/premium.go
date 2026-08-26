package menu

import (
	"context"
	"fmt"
	"log"
	"strconv"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/analytics"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/payment"
)

// Ключи для экранов Premium (якорь с [Назад] и список тарифов).
// Используются обработчиком «Назад» для полного удаления экрана Premium
// при возврате в Главное меню.
const (
	premiumAnchorKey = "premium_anchor_id"
	premiumMsgKey    = "premium_msg_id"
)

// premiumScreenIDs - возвращает id якоря и списка/оплаты экрана Premium.
// Читает СНАЧАЛА из выделенного premiumScreen-map (устойчив к Reset и
// перезапуску), затем - из legacy user-data (миграция старых «висящих»
// экранов, чьи id остались в m.data до этого фикса).
func premiumScreenIDs(stateManager states.StateManager, chatID int64) (anchor, msg string) {
	anchor = stateManager.GetPremiumScreenID(chatID, premiumAnchorKey)
	msg = stateManager.GetPremiumScreenID(chatID, premiumMsgKey)
	if anchor == "" {
		anchor = stateManager.GetUserData(chatID, premiumAnchorKey)
	}
	if msg == "" {
		msg = stateManager.GetUserData(chatID, premiumMsgKey)
	}
	return
}

// ClearPremiumScreen - экспортируемая обёртка clearPremiumScreen для вызова
// из других пакетов (например, роутера при удалении аккаунта). Полностью
// удаляет экран Premium и очищает трекинг его id.
func ClearPremiumScreen(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64) {
	clearPremiumScreen(ctx, b, stateManager, chatID)
}

// clearPremiumScreen - удаляет ранее показанные сообщения экрана Premium
// (якорь с [Назад] и список тарифов/экран оплаты/подтверждения), чтобы при
// повторном входе в раздел (повторное нажатие «💎 Premium» из главного меню,
// либо смена тарифа активного Premium) не накапливались дублирующиеся
// экраны. Безопасно при отсутствии сохранённых id (просто ничего не делает).
//
// Трекинг id (premiumScreen-map) переживает Reset и перезапуск бота, поэтому
// экран гарантированно удаляется даже после /start или рестарта - старые
// сообщения Premium больше не «висят» в чате навсегда.
func clearPremiumScreen(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64) {
	anchor, msg := premiumScreenIDs(stateManager, chatID)
	for _, idStr := range []string{anchor, msg} {
		if id, err := strconv.Atoi(idStr); err == nil && id > 0 {
			helpers.DeleteMessage(ctx, b, chatID, id)
		}
	}
	// Очищаем оба источника трекинга (новый map и legacy user-data).
	stateManager.ClearPremiumScreenIDs(chatID)
	stateManager.SetUserData(chatID, premiumAnchorKey, "0")
	stateManager.SetUserData(chatID, premiumMsgKey, "0")
}

// sendPremiumAnchor - ставит внизу единую Reply-клавиатуру [Назад] перед
// списком тарифов. Inline-кнопки тарифов несовместимы с Reply-клавиатурой в
// одном сообщении, поэтому «якорем» служит отдельное короткое сообщение - оно
// и держит [Назад] на всём протяжении раздела Premium.
func sendPremiumAnchor(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64) {
	msg, err := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.BtnPremium,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
	if err == nil && msg != nil {
		stateManager.SetPremiumScreenID(chatID, premiumAnchorKey, strconv.Itoa(msg.ID))
	} else {
		_ = err
	}
}

// PremiumHandler - обработчик кнопки Premium.
//
// Показывает меню выбора тарифа. Premium активируется только после выбора
// тарифа и нажатия «Оплатил (симуляция)» (callback premium_confirm_<tariffID>).
// Мгновенная выдача Premium убрана - это был тестовый режим.
func PremiumHandler(
	stateManager states.StateManager,
	paymentService *payment.PaymentService,
) func(context.Context, *tgbot.Bot, *models.Update) {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		// update.Message у callback-запросов (например, premium_open из
		// inline-главного меню) равен nil - берём chatID из отправителя.
		var chatID int64
		if update.Message != nil {
			chatID = update.Message.Chat.ID
		} else if update.CallbackQuery != nil {
			chatID = update.CallbackQuery.From.ID
		} else {
			return
		}

		// PostHog: открытие раздела Premium.
		analytics.Track(chatID, "premium_view", nil)

		// Проверяем, не идёт ли уже процесс
		currentState := stateManager.GetState(chatID)
		if currentState != states.StateIdle {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        locales.MsgUserBusy,
				ReplyMarkup: keyboards.BackMenu(),
			})
			return
		}

		// Если Premium уже активен - показываем текущий тариф и опцию смены.
		if paymentService.IsUserPremium(chatID) {
			log.Printf(locales.LogPremiumChangeTariff, chatID)
			// Запоминаем раздел независимо от точки входа (даже если
			// PremiumHandler вызван мимо роутера) - чтобы кнопка «Назад»
			// гарантированно вернула в Главное меню и почистила экран.
			stateManager.SetUserData(chatID, "current_section", "premium")
			clearPremiumScreen(ctx, b, stateManager, chatID)
			sendPremiumAnchor(ctx, b, stateManager, chatID)
			showPremiumCurrent(ctx, b, stateManager, chatID, paymentService)
			return
		}

		// Иначе - меню выбора тарифа (выбор → экран оплаты → подтверждение).
		// Запоминаем раздел независимо от точки входа (см. выше).
		stateManager.SetUserData(chatID, "current_section", "premium")
		clearPremiumScreen(ctx, b, stateManager, chatID)
		sendPremiumAnchor(ctx, b, stateManager, chatID)
		showPremiumMenu(ctx, b, stateManager, chatID)
	}
}

// showPremiumCurrent - экран активного Premium: показывает текущий тариф,
// дату окончания и кнопку смены тарифа (premium_change).
func showPremiumCurrent(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64, paymentService *payment.PaymentService) {
	tariffName := "-"
	expiry := "-"
	if info := paymentService.GetPremiumInfo(chatID); info != nil {
		if t := payment.GetTariffByID(info.TariffID); t != nil {
			tariffName = t.Name
		}
		expiry = info.PremiumExpiresAt.Format("2006-01-02")
	}

	text := fmt.Sprintf(locales.MsgPremiumCurrent, tariffName, expiry)

	msg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
		ReplyMarkup: models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: locales.BtnPremiumChange, CallbackData: "premium_change"},
				},
			},
		},
		ParseMode: "Markdown",
	})
	if msg != nil {
		stateManager.SetPremiumScreenID(chatID, premiumMsgKey, strconv.Itoa(msg.ID))
	}
}

// HandleChangeTariff - обработка кнопки «🔄 Сменить тариф» у активного
// Premium. Показывает меню выбора тарифа (тот же флоу оплаты); после
// подтверждения оплаты тариф пользователя перезаписывается.
func HandleChangeTariff(
	stateManager states.StateManager,
	paymentService *payment.PaymentService,
) func(ctx context.Context, b *tgbot.Bot, update *models.Update, callbackData string) bool {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update, callbackData string) bool {
		if callbackData != "premium_change" {
			return false
		}

		chatID := update.CallbackQuery.From.ID

		_, _ = b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Выберите новый тариф",
		})

		// Показываем то же меню выбора тарифа, что и при первой покупке.
		// Запоминаем раздел независимо от точки входа (см. PremiumHandler).
		stateManager.SetUserData(chatID, "current_section", "premium")
		clearPremiumScreen(ctx, b, stateManager, chatID)
		sendPremiumAnchor(ctx, b, stateManager, chatID)
		showPremiumMenu(ctx, b, stateManager, chatID)
		return true
	}
}

// showPremiumMenu - показывает меню выбора тарифа.
func showPremiumMenu(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64) {
	msg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgPremiumSubscription,
		ReplyMarkup: buildTariffKeyboard(),
	})
	if msg != nil {
		stateManager.SetPremiumScreenID(chatID, premiumMsgKey, strconv.Itoa(msg.ID))
	}
}

// buildTariffKeyboard - создаёт inline-клавиатуру с тарифами.
func buildTariffKeyboard() models.InlineKeyboardMarkup {
	buttons := make([][]models.InlineKeyboardButton, 0, len(payment.AvailableTariffs)+1)

	for _, tariff := range payment.AvailableTariffs {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{
				Text:         "💎 " + tariff.Name + " - " + formatPrice(tariff.Price),
				CallbackData: "premium_" + tariff.ID,
			},
		})
	}

	return models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

// formatPrice - форматирует цену из копеек в рубли.
func formatPrice(priceCents int) string {
	rubles := priceCents / 100
	cents := priceCents % 100
	if cents < 10 {
		return fmt.Sprintf("%d.0%d ₽", rubles, cents)
	}
	return fmt.Sprintf("%d.%d ₽", rubles, cents)
}
