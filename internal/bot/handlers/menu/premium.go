package menu

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/payment"
)

// Ключи user-data для экранов Premium (якорь с [Назад] и список тарифов).
// Используются обработчиком «Назад» для полного удаления экрана Premium
// при возврате в Главное меню.
const (
	premiumAnchorKey = "premium_anchor_id"
	premiumMsgKey    = "premium_msg_id"
)

// sendPremiumAnchor - ставит внизу единую Reply-клавиатуру [Назад] перед
// списком тарифов. Inline-кнопки тарифов несовместимы с Reply-клавиатурой в
// одном сообщении, поэтому «якорем» служит отдельное короткое сообщение - оно
// и держит [Назад] на всём протяжении раздела Premium.
func sendPremiumAnchor(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64) {
	msg, err := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "💎 Premium",
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
	if err == nil && msg != nil {
		stateManager.SetUserData(chatID, premiumAnchorKey, strconv.Itoa(msg.ID))
	}
}

// PremiumHandler - обработчик кнопки Premium.
//
// Показывает меню выбора тарифа. Premium активируется только после выбора
// тарифа и нажатия «Оплатил (симуляция)» (callback premium_confirm_<tariffID>).
// Мгновенная выдача Premium убрана - это был тестовый режим.
func PremiumHandler(
	stateManager states.StateManager,
	paymentService *payment.MockPaymentService,
) func(context.Context, *tgbot.Bot, *models.Update) {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		chatID := update.Message.Chat.ID

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
			sendPremiumAnchor(ctx, b, stateManager, chatID)
			showPremiumCurrent(ctx, b, stateManager, chatID, paymentService)
			return
		}

		// Иначе - меню выбора тарифа (выбор → экран оплаты → подтверждение).
		sendPremiumAnchor(ctx, b, stateManager, chatID)
		showPremiumMenu(ctx, b, stateManager, chatID)
	}
}

// showPremiumCurrent - экран активного Premium: показывает текущий тариф,
// дату окончания и кнопку смены тарифа (premium_change).
func showPremiumCurrent(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64, paymentService *payment.MockPaymentService) {
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
		stateManager.SetUserData(chatID, premiumMsgKey, strconv.Itoa(msg.ID))
	}
}

// HandleChangeTariff - обработка кнопки «🔄 Сменить тариф» у активного
// Premium. Показывает меню выбора тарифа (тот же флоу оплаты); после
// подтверждения оплаты тариф пользователя перезаписывается.
func HandleChangeTariff(
	stateManager states.StateManager,
	paymentService *payment.MockPaymentService,
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
		sendPremiumAnchor(ctx, b, stateManager, chatID)
		showPremiumMenu(ctx, b, stateManager, chatID)
		return true
	}
}

// showPremiumMenu - показывает меню выбора тарифа.
func showPremiumMenu(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64) {
	var lines []string
	lines = append(lines, "💎 Выберите тариф Premium:")
	lines = append(lines, "")

	for _, tariff := range payment.AvailableTariffs {
		lines = append(lines, "📌 "+tariff.Name+"")
		lines = append(lines, tariff.Description)
		lines = append(lines, "💰 "+formatPrice(tariff.Price))
		lines = append(lines, "")
	}

	lines = append(lines, "Выберите тариф кнопкой ниже:")

	msg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        strings.Join(lines, "\n"),
		ReplyMarkup: buildTariffKeyboard(),
		ParseMode:   "Markdown",
	})
	if msg != nil {
		stateManager.SetUserData(chatID, premiumMsgKey, strconv.Itoa(msg.ID))
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
