package menu

import (
	"context"
	"log"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/payment"
	"github.com/theamornoir/analyzpro/internal/storage"
)

// ResetHandler - админ-команда сброса статуса Premium и онбординга
// (для тестирования полного цикла: онбординг → соглашение → меню → покупка).
// Доступна только пользователю с Telegram ID == adminChatID. Для всех
// прочих вызов молча игнорируется.
//
// Кроме Premium/онбординга/соглашения/состояния сбрасывается и анкета-профиль
// в истории дашборда (type="questionnaire"): иначе после /resetme бот
// забывал профиль (user_profiles очищалось), а дашборд продолжал «помнить»
// анкету - и форма заполнения профиля в Mini App пропадала, хотя бот всё ещё
// переспрашивал имя/данные. Теперь сброс консистентен на обеих сторонах:
// пользователь становится «как новый» и заново заполняет профиль один раз.
func ResetHandler(
	adminChatID int64,
	stateManager states.StateManager,
	agreementStorage *storage.AgreementStorage,
	paymentService *payment.PaymentService,
	appStorage *storage.Storage,
	monitorRepo monitoring.Repository,
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

		// 6. Сбрасываем анкету-профиль в истории дашборда (type="questionnaire"),
		// чтобы бот и Mini App снова были синхронны. Сами замеры здоровья
		// (анализы/биосканы) НЕ удаляем - очищается только профиль.
		if monitorRepo != nil {
			if qEntries, _, qErr := monitorRepo.ListHistory(ctx, chatID, "questionnaire", 1, 0); qErr == nil {
				for _, e := range qEntries {
					if dErr := monitorRepo.DeleteHistoryEntry(ctx, e.ID); dErr != nil {
						log.Printf("[RESET] не удалось удалить анкету id=%d user=%d: %v", e.ID, chatID, dErr)
					} else {
						log.Printf("[RESET] удалена анкета-профиль id=%d user=%d (сброс /resetme)", e.ID, chatID)
					}
				}
			} else {
				log.Printf("[RESET] не удалось получить анкеты user=%d: %v", chatID, qErr)
			}
		}

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgResetDone,
		})
	}
}
