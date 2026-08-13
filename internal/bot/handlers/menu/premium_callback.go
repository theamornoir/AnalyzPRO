package menu

import (
	"context"
	"log"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/botutil"
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

		// update.Message у callback-запросов nil — берём chatID из отправителя.
		chatID := update.CallbackQuery.From.ID
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
					{
						{
							Text:         "✅ Оплатил (симуляция)",
							CallbackData: "premium_confirm_" + tariffID,
						},
					},
				},
			},
			ParseMode: "Markdown",
		})

		return true
	}
}

// HandlePremiumConfirm — обработка симуляции оплаты (кнопка
// «✅ Оплатил (симуляция)» после выбора тарифа). Активирует Premium.
// webAppURL/dashboardURL нужны, чтобы сразу показать кнопки открытия дашборда.
func HandlePremiumConfirm(
	stateManager states.StateManager,
	paymentService *payment.MockPaymentService,
	webAppURL string,
	dashboardURL string,
) func(ctx context.Context, b *tgbot.Bot, update *models.Update, callbackData string) bool {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update, callbackData string) bool {
		// update.Message у callback-запросов nil — берём chatID из отправителя.
		chatID := update.CallbackQuery.From.ID

		log.Printf(locales.LogPaymentConfirmEnter, chatID, callbackData)
		if !strings.HasPrefix(callbackData, "premium_confirm_") {
			log.Printf(locales.LogPaymentConfirmSkip, chatID, callbackData)
			return false
		}

		tariffID := strings.TrimPrefix(callbackData, "premium_confirm_")
		log.Printf(locales.LogPaymentConfirmTarget, chatID, tariffID)

		// Запоминаем, был ли Premium уже активен до этого подтверждения
		// (тогда это смена тарифа, а не первая активация).
		wasPremium := paymentService.IsUserPremium(chatID)

		tariff := payment.GetTariffByID(tariffID)
		tariffName := ""
		if tariff != nil {
			tariffName = tariff.Name
		} else {
			log.Printf(locales.LogPaymentTariffNotFound, tariffID)
		}

		if err := paymentService.ActivatePremiumManually(chatID, tariffID); err != nil {
			log.Printf(locales.LogPaymentActivateFailed, chatID, err)
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        "❌ Не удалось активировать Premium. Попробуйте позже.",
				ReplyMarkup: keyboards.BackMenu(),
			})
			botutil.AnswerLogged(ctx, b, tgbot.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
			})
			return true
		}

		log.Printf(locales.LogPaymentActivated, chatID, tariffID)

		botutil.AnswerLogged(ctx, b, tgbot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "✅ Premium активирован!",
		})

		expiry := ""
		if info := paymentService.GetPremiumInfo(chatID); info != nil {
			expiry = info.PremiumExpiresAt.Format("2006-01-02")
		}

		// Кнопки открытия дашборда: WebApp (десктоп/HTTPS) + ссылка (телефон
		// в той же Wi-Fi сети через браузер). linkURL берём из dashboardURL,
		// который при localhost-конфиге указывает на LAN-IP машины.
		linkURL := dashboardURL
		if linkURL == "" {
			linkURL = webAppURL
		}
		webAppTarget := webAppURL
		if webAppTarget == "" {
			webAppTarget = dashboardURL
		}

		rows := [][]models.InlineKeyboardButton{}
		// WebApp-кнопка требует HTTPS ИЛИ localhost/127.0.0.1 (для тестов на
		// той же машине, например в Telegram Desktop). LAN-IP и прочие http
		// отклоняются API на телефоне (400) и ломают всё сообщение — поэтому
		// добавляем её только для https/localhost. В остальных случаях даём
		// обычную кнопку-ссылку, которая открывается в браузере.
		if strings.HasPrefix(webAppTarget, "https") ||
			strings.HasPrefix(webAppTarget, "http://localhost") ||
			strings.HasPrefix(webAppTarget, "http://127.0.0.1") {
			rows = append(rows, []models.InlineKeyboardButton{
				{Text: "💡 Открыть Сводку здоровья (Mini App)", WebApp: &models.WebAppInfo{URL: webAppTarget}},
			})
		}
		if linkURL != "" {
			rows = append(rows, []models.InlineKeyboardButton{
				{Text: "🌐 Открыть в браузере", URL: linkURL},
			})
		}

		confirmText := "💎 **"
		if wasPremium {
			confirmText += "Тариф изменён!"
		} else {
			confirmText += "Premium активирован!"
		}
		confirmText += "**\n\nТариф: " + tariffName
		if expiry != "" {
			confirmText += "\nДействует до: " + expiry
		}
		confirmText += "\n\nТеперь вам доступна 💡 **Сводка здоровья** — откройте её кнопкой ниже " +
			"или из главного меню."

		msgID, sendErr := botutil.SendSafe(ctx, b, tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   confirmText,
			ReplyMarkup: models.InlineKeyboardMarkup{
				InlineKeyboard: rows,
			},
			ParseMode: "Markdown",
		})
		if sendErr != nil {
			log.Printf(locales.LogPaymentConfirmSendErr, chatID, sendErr)
		} else {
			log.Printf(locales.LogPaymentConfirmSent, chatID, msgID, len(rows))
		}
		return true
	}
}
