package router

import (
	"context"
	"log"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/bioscan"
	"github.com/theamornoir/analyzpro/internal/bot/handlers/userdata"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// handleQuestionnaireStates - обработка состояний сбора данных («Общая оценка
// здоровья»). Возвращает true, если обработано. Опросник сокращён до 7
// вопросов: имя, пол, возраст, рост+вес, цель, образ жизни, вредные привычки.
func (r *router) handleQuestionnaireStates(ctx context.Context, b *tgbot.Bot, chatID int64, text string) bool {
	collector := userdata.NewUserDataCollector(r.stateManager)
	state := r.stateManager.GetState(chatID)

	switch state {
	case states.StateWaitingName:
		log.Printf(locales.LogProcessingName, chatID)
		collector.HandleName(ctx, b, chatID, text)
		return true

	case states.StateWaitingGender:
		log.Printf(locales.LogProcessingGender, chatID)
		collector.HandleGender(ctx, b, chatID, text)
		return true

	case states.StateWaitingAge:
		log.Printf(locales.LogProcessingAge, chatID)
		collector.HandleAge(ctx, b, chatID, text)
		return true

	case states.StateWaitingHeightWeight:
		log.Printf(locales.LogProcessingHeightWeight, chatID)
		collector.HandleHeightWeight(ctx, b, chatID, text)
		return true

	case states.StateWaitingGoal:
		log.Printf(locales.LogProcessingGoal, chatID)
		collector.HandleGoal(ctx, b, chatID, text)
		return true

	case states.StateWaitingLifestyle:
		log.Printf(locales.LogProcessingLifestyle, chatID)
		collector.HandleLifestyle(ctx, b, chatID, text)
		return true

	case states.StateWaitingHabits:
		log.Printf(locales.LogProcessingHabits, chatID)
		collector.HandleHabits(ctx, b, chatID, text)
		return true
	}

	return false
}

// isBioscanState - проверяет, является ли состояние шагом Bioscan.
func isBioscanState(state states.State) bool {
	switch state {
	case states.StateWaitingBioscanName,
		states.StateWaitingBioscanAge,
		states.StateWaitingBioscanHeightWeight,
		states.StateWaitingBioscanGender,
		states.StateWaitingBioscanGoal,
		states.StateWaitingBioscanTrainingLevel,
		states.StateWaitingBioscanTrainingExp,
		states.StateWaitingBioscanTrainingFreq,
		states.StateWaitingBioscanTrainingType,
		states.StateWaitingBioscanInjuries,
		states.StateWaitingBioscanPostureIssues,
		states.StateWaitingBioscanImproveZones,
		states.StateWaitingBioscanMobility,
		states.StateWaitingBioscanRecovery,
		states.StateWaitingBioscanSleep,
		states.StateWaitingBioscanStress,
		states.StateWaitingBioscanNutrition,
		states.StateWaitingBioscanProtein,
		states.StateWaitingBioscanWater,
		states.StateWaitingBioscanSmoking,
		states.StateWaitingBioscanAlcohol,
		states.StateWaitingBioscanSedentary,
		states.StateWaitingBioscanBodyFatGoal,
		states.StateWaitingBioscanDietRestrictions,
		states.StateWaitingBioscanPhoto1,
		states.StateWaitingBioscanPhoto2,
		states.StateWaitingBioscanPhoto3,
		states.StateWaitingBioscanPhoto4,
		states.StateWaitingBioscanConfirm,
		states.StateWaitingBioscanBasicQ:
		return true
	}
	return false
}

// isQuestionnaireState - проверяет, является ли состояние шагом опросника
// «Общая оценка здоровья».
func isQuestionnaireState(state states.State) bool {
	switch state {
	case states.StateWaitingName,
		states.StateWaitingGender,
		states.StateWaitingAge,
		states.StateWaitingHeightWeight,
		states.StateWaitingGoal,
		states.StateWaitingLifestyle,
		states.StateWaitingHabits:
		return true
	}
	return false
}

// backQuestionnaire - шаг «Назад» внутри опросника «Общая оценка здоровья»:
// возврат к предыдущему вопросу без сброса уже собранных данных. Если вопрос
// первый - выход из опросника в хаб «Анализы».
func (r *router) backQuestionnaire(ctx context.Context, b *tgbot.Bot, chatID int64, state states.State) {
	prev := userdata.PrevStep(state)
	if prev == states.StateIdle {
		// Первый вопрос - выходим из опросника.
		r.stateManager.SetState(chatID, states.StateIdle)
		r.stateManager.SetUserData(chatID, "analysis_type", "")
		r.stateManager.SetUserData(chatID, "analysis_subtype", "")
		r.setCurrentSection(chatID, "analysis")
		r.deleteHubBlock(ctx, b, chatID)
		r.showMainMenuMessage(ctx, b, chatID, locales.MsgBackToAnalysisType)
		return
	}
	// Если предыдущий вопрос попадает в пропущенную демографию (имя/пол/
	// возраст/рост+вес, подставленную из профиля), возвращаться в него
	// бессмысленно - вместо этого снова показываем экран «Данные уже
	// известны? Использовать / Изменить», откуда можно заново начать с цели
	// либо перейти к полному опроснику.
	if skip := userdata.SkippedSteps(r.stateManager, chatID); skip > 0 {
		if idx := userdata.StepIndex(prev); idx >= 0 && idx < skip {
			if r.tryProfileConfirm(ctx, b, chatID, "health") {
				return
			}
		}
	}
	r.stateManager.SetState(chatID, prev)
	collector := userdata.NewUserDataCollector(r.stateManager)
	collector.SendStep(ctx, b, chatID, prev, userdata.PromptForState(prev))
}

// backBioscanQuestionnaire - шаг «Назад» внутри опросника Bioscan PRO: возврат
// к предыдущему вопросу. Если вопрос первый - выход из Bioscan в хаб «Анализы».
func (r *router) backBioscanQuestionnaire(ctx context.Context, b *tgbot.Bot, chatID int64, state states.State) {
	prev := bioscan.BioscanPrevQuestionState(state)
	if prev == states.StateIdle {
		// Первый вопрос опросника - выходим из Bioscan.
		bioscan.ResetBioscanData(r.stateManager, chatID)
		r.stateManager.SetState(chatID, states.StateIdle)
		r.setCurrentSection(chatID, "analysis")
		r.deleteHubBlock(ctx, b, chatID)
		r.showMainMenuMessage(ctx, b, chatID, locales.MsgBackToAnalysisType)
		return
	}
	r.stateManager.SetState(chatID, prev)
	bioscan.SendBioscanQuestion(ctx, b, chatID, prev)
}
