package router

import (
	"context"
	"log"
	"strconv"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/bioscan"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/menu"
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
// и reply-кнопкой «⬅️ Назад», и inline-кнопками hub_back/msg_back). Вся
// навигация идёт через ОДНО сообщение (main_menu_msg_id), которое
// перерисовывается «на месте» (editMessage), поэтому «Назад» не плодит новые
// сообщения в чате. Уровень (main/hub/action) отслеживается в nav_level.
func (r *router) backToParent(ctx context.Context, b *tgbot.Bot, chatID int64) {
	currentState := r.stateManager.GetState(chatID)
	navLevel := r.navLevel(chatID)

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
	// возвращает в Главное меню, полностью удаляя экран Premium.
	if r.currentSection(chatID) == "premium" || r.premiumScreenTracked(chatID) {
		menu.ClearPremiumScreen(ctx, b, r.stateManager, chatID)
		r.setCurrentSection(chatID, "analysis")
		r.setNavMsgID(chatID, 0)
		r.setNavLevel(chatID, "main")
		r.showMainMenuMessage(ctx, b, chatID, locales.MsgBackToMainMenu)
		return
	}

	navID := r.navMsgID(chatID)
	if navID == 0 {
		// Нет навигационного сообщения - просто показываем главное меню.
		r.setNavLevel(chatID, "main")
		r.showMainMenuMessage(ctx, b, chatID, locales.MsgBackToMainMenu)
		return
	}

	if navLevel == "hub" {
		// Были в хабе раздела - возврат в Главное меню (edit на месте).
		r.setNavLevel(chatID, "main")
		r.showMainMenuMessage(ctx, b, chatID, locales.MsgBackToMainMenu)
		return
	}

	// Были в под-действии (action) или неизвестно - возврат в хаб текущего
	// раздела (edit на месте).
	r.setNavLevel(chatID, "hub")
	r.renderHub(ctx, b, chatID, r.currentSection(chatID))
}
