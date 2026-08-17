package menu

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/analytics"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/onboarding"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/storage"
)

func StartHandler(
	stateManager states.StateManager,
	agreementStorage *storage.AgreementStorage,
	appStorage *storage.Storage,
) func(context.Context, *tgbot.Bot, *models.Update) {
	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		chatID := update.Message.Chat.ID

		// Персистим пользователя (и дефолтные предпочтения) при первом /start,
		// чтобы последующие анализы/биосканы привязывались к реальному User.
		if appStorage != nil {
			if _, err := appStorage.EnsureUser(ctx, chatID); err != nil {
				// Не фатально - онбординг продолжается.
				_ = err
			}
		}
		analytics.EmitEvent(ctx, analytics.Event{
			Type:       analytics.EventStart,
			TelegramID: chatID,
		})

		// /start всегда освобождает «зависшее» состояние от прошлых сессий
		// (оно персистится в states.json между перезапусками бота), чтобы
		// пользователь начинал с чистого главного меню, а не из середины
		// старого потока bioscan/анкеты.
		stateManager.Reset(chatID)

		// Онбординг: новые пользователи проходят 4 шага + соглашение.
		// Уже прошедшие (OnboardingCompleted) попадают сразу в главное меню.
		onboarded := false
		if appStorage != nil {
			onboarded = appStorage.IsOnboardingCompleted(ctx, chatID)
		}
		if !onboarded && agreementStorage.IsAgreed(chatID) {
			// Миграция существующих пользователей: они уже приняли
			// соглашение до появления онбординга - помечаем пройденным,
			// чтобы не гонять их по слайдеру повторно.
			if appStorage != nil {
				_ = appStorage.SetOnboardingCompleted(ctx, chatID, true)
			}
			onboarded = true
		}

		if onboarded {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        locales.MsgStartWelcomeBack,
				ReplyMarkup: keyboards.MainMenu(),
				ParseMode:   "Markdown",
			})
			return
		}

		// Новый пользователь - запускаем онбординг с первого шага.
		onboarding.SendStep(ctx, b, chatID, 1)
	}
}
