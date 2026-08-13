package menu

import (
	"context"
	"log"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/payment"
)

// HandlePremiumCallback — обработка нажатия на тариф.
func HandlePremiumCallback(
	stateManager states.StateManager,
	paymentService *payment.MockPaymentService,
) func(ctx context.Context, b *tgbot.Bot, update *models.Update, callbackData string) bool {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update, callbackData string) bool {
		if !strings.HasPrefix(callbackData, "premium_") {
			return false
		}

		chatID := update.Message.Chat.ID
		tariffID := strings.TrimPrefix(callbackData, "premium_")

		tariff := payment.GetTariffByID(tariffID)
		if tariff == nil {
			log.Printf(locales.LogPaymentTariffNotFound, tariffID)
			_, _ = b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
				Text:            "Тариф не найден",
				ShowAlert:       true,
			})
			return true
		}

		log.Printf(locales.LogPaymentSelectTariff, chatID, tariff.Name)

		// Отвечаем на callback
		_, _ = b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Перехожу к оплате...",
		})

		// Создаём платеж
		paymentResp, err := paymentService.CreatePayment(payment.PaymentRequest{
			UserID:   chatID,
			TariffID: tariff.ID,
		})
		if err != nil {
			log.Printf(locales.LogPaymentCreateErr, chatID, err)
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        "❌ Ошибка создания платежа. Попробуйте позже.",
				ReplyMarkup: keyboards.BackMenu(),
			})
			return true
		}

		// Отправляем ссылку на оплату
		priceText := formatPrice(tariff.Price)
		featuresText := strings.Join(tariff.Features, "\n• ")

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text: "💳 **Оплата Premium**\n\n" +
				"📌 **Тариф:** " + tariff.Name + "\n" +
				"💰 **Сумма:** " + priceText + "\n\n" +
				"🎁 **Что входит:**\n• " + featuresText + "\n\n" +
				"👇 Нажмите кнопку для оплаты:",
			ReplyMarkup: models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{
					{
						{
							Text: "💳 Оплатить " + priceText,
							URL:  paymentResp.URL,
						},
					},
				},
			},
			ParseMode: "Markdown",
		})

		return true
	}
}
