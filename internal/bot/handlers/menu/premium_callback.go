package menu

import (
	"context"
	"log"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/analytics"
	"github.com/theamornoir/analyzpro/internal/bot/botutil"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/payment"
)

// HandlePremiumCallback - обработка нажатия на тариф.
func HandlePremiumCallback(
	stateManager states.StateManager,
	paymentService *payment.MockPaymentService,
) func(ctx context.Context, b *tgbot.Bot, update *models.Update, callbackData string) bool {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update, callbackData string) bool {
		if !strings.HasPrefix(callbackData, "premium_") {
			return false
		}

		// update.Message у callback-запросов nil - берём chatID из отправителя.
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

		msg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text: "💳 Оплата Premium\n\n" +
				"📌 Тариф: " + tariff.Name + "\n" +
				"💰 Сумма: " + priceText + "\n\n" +
				"🎁 Что входит:\n• " + featuresText + "\n\n" +
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
		// Запоминаем id экрана оплаты под тем же ключом, что и список
		// тарифов/экран активного Premium (premium_msg_key). При выходе из
		// Premium (кнопка «Назад») backToParent удаляет именно последний
		// экран - иначе экран оплаты «висел» бы в истории после ухода.
		if msg != nil {
			stateManager.SetPremiumScreenID(chatID, premiumMsgKey, strconv.Itoa(msg.ID))
		}

		return true
	}
}

// HandlePremiumConfirm - обработка симуляции оплаты (кнопка
// «✅ Оплатил (симуляция)» после выбора тарифа). Активирует Premium.
// webAppURL/dashboardURL нужны, чтобы сразу показать кнопки открытия дашборда.
func HandlePremiumConfirm(
	stateManager states.StateManager,
	paymentService *payment.MockPaymentService,
	webAppURL string,
	dashboardURL string,
) func(ctx context.Context, b *tgbot.Bot, update *models.Update, callbackData string) bool {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update, callbackData string) bool {
		// update.Message у callback-запросов nil - берём chatID из отправителя.
		chatID := update.CallbackQuery.From.ID

		log.Printf(locales.LogPaymentConfirmEnter, chatID, callbackData)
		if !strings.HasPrefix(callbackData, "premium_confirm_") {
			log.Printf(locales.LogPaymentConfirmSkip, chatID, callbackData)
			return false
		}

		tariffID := strings.TrimPrefix(callbackData, "premium_confirm_")
		log.Printf(locales.LogPaymentConfirmTarget, chatID, tariffID)

		// Защита от повторного нажатия кнопки «Оплатил (симуляция)» из
		// старого сообщения или двойного клика: если Premium уже активен
		// ровно на этом тарифе - повторная активация не нужна (на реальном
		// платёжном шлюзе это предотвратило бы повторное списание средств).
		if paymentService.IsUserPremium(chatID) {
			if info := paymentService.GetPremiumInfo(chatID); info != nil && info.TariffID == tariffID {
				botutil.AnswerLogged(ctx, b, tgbot.AnswerCallbackQueryParams{
					CallbackQueryID: update.CallbackQuery.ID,
					Text:            "✅ Premium уже активен на этом тарифе",
				})
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:      chatID,
					Text:        locales.MsgPremiumConfirmReplay,
					ReplyMarkup: keyboards.MainMenu(),
				})
				return true
			}
		}

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

		analytics.EmitEvent(ctx, analytics.Event{
			Type:       analytics.EventPremium,
			TelegramID: chatID,
			Meta:       map[string]interface{}{"tariff": tariffID, "changed": wasPremium},
		})

		// PostHog: событие покупки Premium (русская подпись "Купил Premium").
		// changed=true - это смена тарифа уже активного Premium, false -
		// первая активация.
		analytics.Track(chatID, "premium_purchased", map[string]interface{}{
			"tariff":  tariffID,
			"changed": wasPremium,
		})

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
		webAppTarget := webAppURL
		if webAppTarget == "" {
			webAppTarget = dashboardURL
		}

		// Только Mini App - без ссылок и «открыть в браузере».
		rows := [][]models.InlineKeyboardButton{}
		if webAppTarget != "" {
			rows = append(rows, []models.InlineKeyboardButton{
				{Text: "Открыть", WebApp: &models.WebAppInfo{URL: webAppTarget}},
			})
		}

		confirmText := "💎 "
		if wasPremium {
			confirmText += "Тариф изменён!"
		} else {
			confirmText += "Premium активирован!"
		}
		confirmText += "\n\nТариф: " + tariffName
		if expiry != "" {
			confirmText += "\nДействует до: " + expiry
		}
		confirmText += "\n\nТеперь вам доступна 📊 Мой профиль - откройте её кнопкой ниже " +
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
			// Запоминаем id подтверждения Premium в выделенном premiumScreen-map
			// (как и прочие экраны): при выходе из Premium (кнопка «Назад»)
			// backToParent удаляет последний экран, и id переживает Reset.
			stateManager.SetPremiumScreenID(chatID, premiumMsgKey, strconv.Itoa(msgID))
		}
		return true
	}
}
