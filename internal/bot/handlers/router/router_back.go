package router

import (
	"context"
	"log"
	"strconv"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/bioscan"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// currentSectionKey - ключ в user-data для текущего раздела (analysis/health/
// service). Используется обработчиком «Назад» для возврата на уровень выше
// (в хаб раздела), а не сразу в Главное меню.
const currentSectionKey = "current_section"

// currentSection - текущий раздел пользователя (с безопасным дефолтом).
func (r *router) currentSection(chatID int64) string {
	if s := r.stateManager.GetUserData(chatID, currentSectionKey); s == "analysis" || s == "health" || s == "service" || s == "premium" {
		return s
	}
	return "analysis"
}

// setCurrentSection - запоминает текущий раздел (при входе в хаб или флоу).
func (r *router) setCurrentSection(chatID int64, section string) {
	r.stateManager.SetUserData(chatID, currentSectionKey, section)
}

// handleBack - обработка кнопки "⬅️ Назад". Возвращает true, если обработано.
func (r *router) handleBack(ctx context.Context, b *tgbot.Bot, chatID int64, text string) bool {
	if text != locales.BtnBack {
		return false
	}

	log.Printf(locales.LogRouterBack, chatID, r.stateManager.GetState(chatID))
	r.backToParent(ctx, b, chatID)
	return true
}

// backToParent - унифицированный иерархический возврат «назад» (используется
// и reply-кнопкой «⬅️ Назад», и inline-кнопками hub_back/msg_back).
// Логика:
//   - под-шаг раздела (состояние флоу НЕ idle) ИЛИ листовое сообщение раздела
//     (Сводка/Мониторинг/О сервисе - блок-хаб уже удалён) -> возврат в ХАБ
//     ЭТОГО раздела (на уровень выше, с единой клавиатурой [Назад]);
//   - нахождение прямо в хабе раздела (state=idle и блок-хаб на месте) ->
//     возврат в ГЛАВНОЕ меню.
func (r *router) backToParent(ctx context.Context, b *tgbot.Bot, chatID int64) {
	currentState := r.stateManager.GetState(chatID)
	isHubLevel := currentState == states.StateIdle && r.hubMessageID(chatID) > 0

	// Если мы в BIOSCAN - очищаем собранные данные.
	if isBioscanState(currentState) {
		log.Printf(locales.LogRouterBackBioscan, chatID)
		bioscan.ResetBioscanData(r.stateManager, chatID)
	}

	// Сбрасываем состояние/данные любого флоу (анализ/опросник/bioscan).
	r.stateManager.SetState(chatID, states.StateIdle)
	r.stateManager.SetUserData(chatID, "analysis_type", "")
	r.stateManager.SetUserData(chatID, "analysis_subtype", "")

	// Premium - верхнеуровневый вход без родительского хаба: «Назад» всегда
	// возвращает в Главное меню, полностью удаляя экран Premium (якорь +
	// список тарифов) из чата.
	if r.currentSection(chatID) == "premium" {
		for _, key := range []string{"premium_anchor_id", "premium_msg_id"} {
			if id, err := strconv.Atoi(r.stateManager.GetUserData(chatID, key)); err == nil && id > 0 {
				helpers.DeleteMessage(ctx, b, chatID, id)
				r.stateManager.SetUserData(chatID, key, "0")
			}
		}
		r.setLastMsg(chatID, 0)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBackToMainMenu,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return
	}

	// Безопасность: нет ни хаба, ни последнего сообщения - пользователь уже
	// в Главном меню (кнопки «Назад» там нет, но на всякий случай - меню).
	if currentState == states.StateIdle && r.hubMessageID(chatID) == 0 && r.lastMsgID(chatID) == 0 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBackToMainMenu,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return
	}

	if isHubLevel {
		// Прямо в Главное меню: хаб раздела больше не нужен.
		r.deleteHubBlock(ctx, b, chatID)
		if id := r.lastMsgID(chatID); id > 0 {
			helpers.DeleteMessage(ctx, b, chatID, id)
			r.setLastMsg(chatID, 0)
		}
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBackToMainMenu,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return
	}

	// Возврат на уровень выше - в хаб текущего раздела. Удаляем текущее
	// сообщение под-действия и (на всякий случай) блок-хаб, затем показываем
	// хаб раздела с единой клавиатурой [Назад] внизу.
	section := r.currentSection(chatID)
	if id := r.lastMsgID(chatID); id > 0 {
		helpers.DeleteMessage(ctx, b, chatID, id)
		r.setLastMsg(chatID, 0)
	}
	r.deleteHubBlock(ctx, b, chatID)
	r.renderHub(ctx, b, chatID, section)
}
