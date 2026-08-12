package router

import (
	"context"
	"log"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/bioscan"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// handleBack - обработка кнопки "⬅️ Назад". Возвращает true, если обработано.
func (r *router) handleBack(ctx context.Context, b *tgbot.Bot, chatID int64, text string) bool {
	if text != locales.BtnBack {
		return false
	}

	currentState := r.stateManager.GetState(chatID)
	currentAnalysisType := r.stateManager.GetUserData(chatID, "analysis_type")
	log.Printf(locales.LogRouterBack, chatID, currentState)

	// Если мы в BIOSCAN - возвращаем в главное меню
	if isBioscanState(currentState) {
		log.Printf(locales.LogRouterBackBioscan, chatID)
		r.stateManager.SetState(chatID, states.StateIdle)
		// Очищаем все данные bioscan
		bioscan.ResetBioscanData(r.stateManager, chatID)
		r.stateManager.SetUserData(chatID, "analysis_type", "")

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBackToMainMenu,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return true
	}

	// Остальные обработки "Назад"...
	if isQuestionnaireState(currentState) {
		log.Printf(locales.LogRouterBackQuestion, chatID)
		r.stateManager.SetState(chatID, states.StateIdle)
		r.stateManager.SetUserData(chatID, "analysis_type", "")
		r.stateManager.SetUserData(chatID, "analysis_subtype", "")

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBackToAnalysisType,
			ReplyMarkup: keyboards.AnalysisTypeMenu(),
			ParseMode:   "Markdown",
		})
		return true
	}

	if currentState == states.StateIdle && (currentAnalysisType == "regular" || currentAnalysisType == "extended") {
		log.Printf(locales.LogRouterBackTypeMenu, chatID)
		r.stateManager.SetState(chatID, states.StateIdle)
		r.stateManager.SetUserData(chatID, "analysis_type", "")
		r.stateManager.SetUserData(chatID, "analysis_subtype", "")

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBackToMainMenu,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return true
	}

	if currentState == states.StateWaitingAnalysisFile || currentState == states.StateWaitingUploadConfirm {
		log.Printf(locales.LogRouterBackFiles, chatID)
		r.stateManager.SetState(chatID, states.StateIdle)
		r.stateManager.SetUserData(chatID, "analysis_type", "")
		r.stateManager.SetUserData(chatID, "analysis_subtype", "")

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBackToAnalysisType,
			ReplyMarkup: keyboards.AnalysisTypeMenu(),
			ParseMode:   "Markdown",
		})
		return true
	}

	log.Printf(locales.LogRouterBackDefault, chatID)
	r.stateManager.SetState(chatID, states.StateIdle)
	r.stateManager.SetUserData(chatID, "analysis_type", "")
	r.stateManager.SetUserData(chatID, "analysis_subtype", "")

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgAgreementAlreadyAccepted,
		ReplyMarkup: keyboards.MainMenu(),
	})
	return true
}
