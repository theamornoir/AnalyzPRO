package router

import (
	"context"
	"log"
	"strconv"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/bioscan"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
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

// premiumScreenTracked - true, если за пользователем в user-data до сих пор
// отслеживаются сообщения экрана Premium (якорь и/или список тарифов/экран
// оплаты/подтверждения). Используется обработчиком «Назад» как
// НЕЗАВИСИМЫЙ от current_section триггер: даже если раздел по какой-то
// причине не был выставлен (вход мимо роутера, восстановление из states.json,
// рассинхронизация), нажатие «Назад» всё равно корректно убирает экран
// Premium и показывает Главное меню - без «висящих» тарифов и без
// исчезнувшего меню.
func (r *router) premiumScreenTracked(chatID int64) bool {
	for _, key := range []string{"premium_anchor_id", "premium_msg_id"} {
		if id, err := strconv.Atoi(r.stateManager.GetUserData(chatID, key)); err == nil && id > 0 {
			return true
		}
		// Fallback на выделенный трекер (переживает Reset/перезапуск).
		if id, err := strconv.Atoi(r.stateManager.GetPremiumScreenID(chatID, key)); err == nil && id > 0 {
			return true
		}
	}
	return false
}

// handleBack - обработка кнопки "⬅️ Назад". Возвращает true, если обработано.
func (r *router) handleBack(ctx context.Context, b *tgbot.Bot, chatID int64, text string) bool {
	if text != locales.BtnBack {
		return false
	}

	state := r.stateManager.GetState(chatID)

	// Шаг «Назад» внутри опросника анализа - возврат к предыдущему вопросу
	// (без сброса уже собранных данных).
	if isQuestionnaireState(state) {
		log.Printf(locales.LogRouterBack, chatID, state)
		r.backQuestionnaire(ctx, b, chatID, state)
		return true
	}
	// Шаг «Назад» внутри опросника Bioscan PRO - возврат к предыдущему вопросу.
	if bioscan.IsBioscanQuestionnaireState(state) {
		log.Printf(locales.LogRouterBack, chatID, state)
		r.backBioscanQuestionnaire(ctx, b, chatID, state)
		return true
	}

	log.Printf(locales.LogRouterBack, chatID, r.stateManager.GetState(chatID))
	r.backToParent(ctx, b, chatID)
	return true
}

// handleCancel - обработка кнопки «❌ Отмена» внутри анкеты/опросника:
// полный выход из сбора данных без сохранения, возврат в хаб «Анализы».
// Возвращает true, если обработано.
func (r *router) handleCancel(ctx context.Context, b *tgbot.Bot, chatID int64, text string) bool {
	if text != locales.BtnCancel {
		return false
	}
	state := r.stateManager.GetState(chatID)
	if !isQuestionnaireState(state) && !bioscan.IsBioscanQuestionnaireState(state) {
		return false
	}
	bioscan.ResetBioscanData(r.stateManager, chatID)
	r.stateManager.SetState(chatID, states.StateIdle)
	r.stateManager.SetUserData(chatID, "analysis_type", "")
	r.stateManager.SetUserData(chatID, "analysis_subtype", "")
	r.setCurrentSection(chatID, "analysis")
	r.deleteHubBlock(ctx, b, chatID)
	r.showMainMenuMessage(ctx, b, chatID, locales.MsgBackToAnalysisType)
	return true
}

// backToParent - унифицированный иерархический возврат «назад» (используется
// и reply-кнопкой «⬅️ Назад», и inline-кнопками hub_back/msg_back).
// Логика:
//   - под-шаг раздела (состояние флоу НЕ idle) ИЛИ листовое сообщение раздела
//     (Мой профиль/Мониторинг/О сервисе - блок-хаб уже удалён) -> возврат в ХАБ
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
	if r.currentSection(chatID) == "premium" || r.premiumScreenTracked(chatID) {
		for _, key := range []string{"premium_anchor_id", "premium_msg_id"} {
			if id, err := strconv.Atoi(r.stateManager.GetUserData(chatID, key)); err == nil && id > 0 {
				helpers.DeleteMessage(ctx, b, chatID, id)
				r.stateManager.SetUserData(chatID, key, "0")
			}
			// Fallback на выделенный трекер (переживает Reset/перезапуск).
			if id, err := strconv.Atoi(r.stateManager.GetPremiumScreenID(chatID, key)); err == nil && id > 0 {
				helpers.DeleteMessage(ctx, b, chatID, id)
			}
		}
		// Очищаем выделенный трекер (гарантированно удаляет id, даже если
		// они пережили Reset при /start или перезапуск бота).
		r.stateManager.ClearPremiumScreenIDs(chatID)
		// Выходим в главное меню - текущий раздел больше не Premium
		// («analysis» - нейтральный дефолт, совпадает с поведением при
		// отсутствии сохранённого раздела). Иначе «залипшее» значение
		// «premium» ломало бы последующую иерархию «Назад».
		r.setCurrentSection(chatID, "analysis")
		r.setLastMsg(chatID, 0)
		// Подстраховка: убираем возможный «висячий» блок-хаб (на случай
		// рассинхронизации состояния) и показываем персистентное меню -
		// иначе после удаления экрана Premium внизу могло бы образоваться
		// «пустое дно» (исчезнувшее меню без кнопок).
		r.deleteHubBlock(ctx, b, chatID)
		r.showMainMenuMessage(ctx, b, chatID, locales.MsgBackToMainMenu)
		return
	}

	// Безопасность: нет ни хаба, ни последнего сообщения - пользователь уже
	// в Главном меню (кнопки «Назад» там нет, но на всякий случай - меню).
	if currentState == states.StateIdle && r.hubMessageID(chatID) == 0 && r.lastMsgID(chatID) == 0 {
		r.showMainMenuMessage(ctx, b, chatID, locales.MsgBackToMainMenu)
		return
	}

	if isHubLevel {
		// Прямо в Главное меню: хаб раздела больше не нужен.
		r.deleteHubBlock(ctx, b, chatID)
		if id := r.lastMsgID(chatID); id > 0 {
			helpers.DeleteMessage(ctx, b, chatID, id)
			r.setLastMsg(chatID, 0)
		}
		r.showMainMenuMessage(ctx, b, chatID, locales.MsgBackToMainMenu)
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
