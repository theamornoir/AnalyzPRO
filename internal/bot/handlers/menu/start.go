package menu

import (
	"context"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/analytics"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/onboarding"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/storage"
)

// consultFinishKey / consultResultKey - ключи трекинга сообщений флоу
// консультации. Должны СОВПАДАТЬ с router.consultFinishMsgKey и
// router.consultResultMsgKey (дублируем намеренно: StartHandler живёт в
// пакете menu и не имеет доступа к приватным константам роутера). Если
// меняете имена ключей в роутере - синхронизируйте и здесь.
const (
	consultFinishKey = "consult_finish_msg_id"
	consultResultKey = "consult_result_msg_ids"
)

// clearConsultationMessages - удаляет «висящие» сообщения флоу консультации
// (reply-клавиатуру «Закончить консультацию» и сообщение(я)-ответ ИИ), если
// пользователь нажал /start прямо во время финиша консультации. Вызывать ДО
// stateManager.Reset (Reset сбросит user-data, и id сообщений потеряются, а
// сами сообщения останутся висеть в чате с нижней клавиатурой). Безопасно
// при отсутствии сохранённых id (просто ничего не делает).
func clearConsultationMessages(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64) {
	ids := []string{stateManager.GetUserData(chatID, consultFinishKey)}
	if raw := stateManager.GetUserData(chatID, consultResultKey); raw != "" {
		for _, p := range strings.Split(raw, ",") {
			ids = append(ids, strings.TrimSpace(p))
		}
	}
	for _, idStr := range ids {
		if id, err := strconv.Atoi(idStr); err == nil && id > 0 {
			helpers.DeleteMessage(ctx, b, chatID, id)
		}
	}
}

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
			// Фиксируем активность (нужно системе напоминаний).
			_ = appStorage.TouchActivity(ctx, chatID)
		}
		analytics.EmitEvent(ctx, analytics.Event{
			Type:       analytics.EventStart,
			TelegramID: chatID,
		})

		// PostHog: первый/повторный запуск бота пользователем.
		analytics.Track(chatID, "user_started", map[string]interface{}{
			"source": "start_command",
		})

		// Если пользователь НАХОДИТСЯ в разделе Premium и его экран уже
		// показан - повторный /start (случайное нажатие кнопки «Старт» в
		// Telegram или выбор команды /start из меню бота во время Premium)
		// НЕ должен уничтожать экран Premium. Просто оставляем экран как
		// есть и ничего не сбрасываем - иначе весь премиум-флоу «исчезает»
		// (баг «премиум слетает после /start»). Висящий Premium чистим
		// ТОЛЬКО когда пользователь НЕ в Premium (экран - «хвост» прошлой
		// мёртвой сессии, который надо убрать).
		if stateManager.GetUserData(chatID, "current_section") == "premium" {
			anchor := stateManager.GetPremiumScreenID(chatID, premiumAnchorKey)
			msg := stateManager.GetPremiumScreenID(chatID, premiumMsgKey)
			if anchor != "" || msg != "" {
				analytics.Track(chatID, "start_ignored_premium", nil)
				return
			}
		}

		// ВАЖНО: сначала подчищаем «висящий» экран Premium (якорь +
		// список/оплата/подтверждение), ПОКА его message_id ещё лежат в
		// user-data. Иначе последующий Reset ниже целиком собьёт
		// premium_anchor_id/premium_msg_id, clearPremiumScreen прочитает
		// пустые ключи и не удалит сообщения - экран Premium останется
		// висеть в чате поверх приветствия /start (баг «Premium висит после
		// /start»). Безопасно при отсутствии экрана.
		clearPremiumScreen(ctx, b, stateManager, chatID)

		// Подчищаем «висящий» флоу консультации (reply-клавиатуру
		// «Закончить консультацию» и сообщение-ответ ИИ), если пользователь
		// нажал /start прямо во время финиша консультации. Читаем id ДО
		// Reset (ниже), иначе ключи исчезнут из user-data, а сами сообщения
		// останутся висеть в чате с нижней клавиатурой (баг «клавиатура
		// консультации остаётся после /start»). Ключи совпадают с
		// router.consultFinishMsgKey / router.consultResultMsgKey.
		clearConsultationMessages(ctx, b, stateManager, chatID)

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

		// Сообщение пользователя с командой /start НЕ удаляем: в Telegram
		// это штатная запись команды, а её скрытие выглядело «странно»
		// (история будто сама стиралась). Оставляем как есть - за ботом
		// придёт приветственное персистентное сообщение.

		if onboarded {
			// /start присылает «закреплённое» сообщение главного меню
			// (приветствие) под тем же ключом, что и showMainMenuMessage. Оно
			// ПЕРСИСТЕНТНО: висит внизу чата вместе с клавиатурой главного
			// меню, пока пользователь не выберет действие. При выборе любого
			// действия (хаб/Premium/под-действие) старое закреплённое
			// сообщение удаляется (deleteMainMenuMessage в renderHub /
			// handleMenuButtons / deleteHubBlock), а на его место приходит
			// новое - дублей нет. Само по себе приветствие НЕ исчезает: так
			// бот «не пустеет», а клавиатура главного меню остаётся доступной,
			// пока пользователь не начнёт действие. Повторный /start просто
			// перерисовывает его (ShowPersistentMessage сначала удаляет
			// предыдущее закреплённое сообщение).
			helpers.ShowPersistentMessage(ctx, b, stateManager, chatID, helpers.MainMenuMsgKey, tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        locales.MsgStartWelcomeBack,
				ReplyMarkup: keyboards.MainMenuInline(),
				ParseMode:   "Markdown",
			})
			return
		}

		// Новый пользователь - запускаем короткий онбординг (1 сообщение
		// с описанием функционала → согласие).
		onboarding.SendIntro(ctx, b, chatID)
	}
}
