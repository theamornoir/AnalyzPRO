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

// handleAgreementButton - показывает текст соглашения, если оно ещё не принято.
func (r *router) handleAgreementButton(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf(locales.LogProcessingAgreement, chatID)
	if r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgAgreementAlreadyAcceptedShort,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return true
	}

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        keyboards.UserAgreementText(),
		ReplyMarkup: keyboards.AgreementMenu(),
		ParseMode:   "Markdown",
	})
	return true
}

// handleAcceptAgreement - принимает соглашение.
func (r *router) handleAcceptAgreement(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf(locales.LogRouterAcceptAgreement, chatID)
	if r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgAgreementAlreadyAcceptedShort,
			ReplyMarkup: keyboards.MainMenu(),
		})
		return true
	}

	r.agreementStorage.SetAgreed(chatID)
	r.stateManager.SetState(chatID, states.StateIdle)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgAgreementAccepted,
		ReplyMarkup: keyboards.MainMenu(),
		ParseMode:   "Markdown",
	})
	return true
}

// handleDiagnostics - показывает меню выбора типа анализа.
func (r *router) handleDiagnostics(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf(locales.LogRouterDiagnostics, chatID)
	if !r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: keyboards.StartMenu(),
		})
		return true
	}

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgDiagnosticsIntro,
		ReplyMarkup: keyboards.AnalysisTypeMenu(),
		ParseMode:   "Markdown",
	})
	return true
}

// handleRegularAnalysis - запускает обычный анализ.
func (r *router) handleRegularAnalysis(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf(locales.LogRouterRegular, chatID)
	if !r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: keyboards.StartMenu(),
		})
		return true
	}

	r.stateManager.SetUserData(chatID, "analysis_type", "regular")
	r.stateManager.SetUserData(chatID, "analysis_subtype", "regular")
	r.stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgRegularAnalysisIntro,
		ReplyMarkup: keyboards.ProcessAnalysisMenu(),
		ParseMode:   "Markdown",
	})
	return true
}

// handleExtendedAnalysis - запускает расширенный анализ (с опросником).
func (r *router) handleExtendedAnalysis(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf(locales.LogRouterExtended, chatID)
	if !r.agreementStorage.IsAgreed(chatID) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: keyboards.StartMenu(),
		})
		return true
	}

	r.stateManager.SetUserData(chatID, "analysis_type", "extended")
	r.stateManager.SetUserData(chatID, "analysis_subtype", "extended")

	r.stateManager.SetState(chatID, states.StateWaitingName)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgExtendedAnalysisIntro,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
	return true
}

// handleBioscanStart - запускает bioscan.
func (r *router) handleBioscanStart(ctx context.Context, b *tgbot.Bot, chatID int64) bool {
	log.Printf(locales.LogRouterBioscanStart, chatID)

	if !r.agreementStorage.IsAgreed(chatID) {
		log.Printf(locales.LogRouterAgreeNotDone, chatID)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgreementRequired,
			ReplyMarkup: keyboards.StartMenu(),
		})
		return true
	}

	currentState := r.stateManager.GetState(chatID)
	if currentState != states.StateIdle {
		log.Printf(locales.LogRouterUserBusy, chatID, currentState)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgUserBusy,
		})
		return true
	}

	// Принудительно устанавливаем состояние
	r.stateManager.SetState(chatID, states.StateWaitingBioscanName)
	log.Printf(locales.LogRouterForceBioscan, chatID)

	// Проверяем, что состояние установилось
	if r.stateManager.GetState(chatID) != states.StateWaitingBioscanName {
		log.Printf(locales.LogRouterSetStateFail, r.stateManager.GetState(chatID))
	}

	// Сбрасываем данные bioscan
	bioscan.ResetBioscanData(r.stateManager, chatID)
	r.stateManager.SetUserData(chatID, "analysis_type", "")

	log.Printf(locales.LogRouterBioscanLaunch, chatID)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanIntro,
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackMenu(),
	})
	return true
}
