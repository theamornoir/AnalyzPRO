package menu

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/payment"
	"github.com/theamornoir/analyzpro/internal/storage"
)

// ResetHandler - админ-команда сброса статуса Premium и онбординга
// (для тестирования полного цикла: онбординг → соглашение → меню → покупка).
// Доступна только пользователю с Telegram ID == adminChatID. Для всех
// прочих вызов молча игнорируется.
func ResetHandler(
	adminChatID int64,
	stateManager states.StateManager,
	agreementStorage *storage.AgreementStorage,
	paymentService *payment.PaymentService,
	appStorage *storage.Storage,
) func(context.Context, *tgbot.Bot, *models.Update) {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}
		chatID := update.Message.Chat.ID

		// Только для администратора.
		if chatID != adminChatID {
			return
		}

		// 1. Сбрасываем Premium.
		paymentService.ResetPremium(chatID)

		// 2. Сбрасываем онбординг.
		if appStorage != nil {
			_ = appStorage.SetOnboardingCompleted(ctx, chatID, false)
		}

		// 3. Сбрасываем соглашение (чтобы можно было пройти онбординг
		// полностью, включая повторное принятие соглашения).
		agreementStorage.Reset(chatID)

		// 4. Подчищаем «висящий» экран Premium (якорь + список), ПОКА его
		// id ещё в user-data - иначе Reset ниже собьёт их и экран останется
		// висеть. Безопасно при отсутствии экрана.
		clearPremiumScreen(ctx, b, stateManager, chatID)

		// 5. Снимаем «зависшее» состояние.
		stateManager.Reset(chatID)

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgResetDone,
		})
	}
}
