package menu

import (
	"context"
	"fmt"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/payment"
)

// PremiumHandler — обработчик кнопки Premium.
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

		// Показываем тарифы
		showPremiumMenu(ctx, b, chatID)
	}
}

// showPremiumMenu — показывает меню выбора тарифа.
func showPremiumMenu(ctx context.Context, b *tgbot.Bot, chatID int64) {
	var lines []string
	lines = append(lines, "💎 **Выберите тариф Premium:**")
	lines = append(lines, "")

	for _, tariff := range payment.AvailableTariffs {
		lines = append(lines, "📌 **"+tariff.Name+"**")
		lines = append(lines, tariff.Description)
		lines = append(lines, "💰 "+formatPrice(tariff.Price))
		lines = append(lines, "")
	}

	lines = append(lines, "Выберите тариф кнопкой ниже:")

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        strings.Join(lines, "\n"),
		ReplyMarkup: buildTariffKeyboard(),
		ParseMode:   "Markdown",
	})
}

// buildTariffKeyboard — создаёт inline-клавиатуру с тарифами.
func buildTariffKeyboard() models.InlineKeyboardMarkup {
	buttons := make([][]models.InlineKeyboardButton, 0, len(payment.AvailableTariffs)+1)

	for _, tariff := range payment.AvailableTariffs {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{
				Text:         "💎 " + tariff.Name + " — " + formatPrice(tariff.Price),
				CallbackData: "premium_" + tariff.ID,
			},
		})
	}

	buttons = append(buttons, []models.InlineKeyboardButton{
		{
			Text:         "⬅️ Назад",
			CallbackData: "back_main",
		},
	})

	return models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

// formatPrice — форматирует цену из копеек в рубли.
func formatPrice(priceCents int) string {
	rubles := priceCents / 100
	cents := priceCents % 100
	if cents < 10 {
		return fmt.Sprintf("%d.0%d ₽", rubles, cents)
	}
	return fmt.Sprintf("%d.%d ₽", rubles, cents)
}
