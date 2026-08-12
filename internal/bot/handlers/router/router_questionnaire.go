package router

import (
	"context"
	"log"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/userdata"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// handleQuestionnaireStates - обработка состояний сбора данных. Возвращает true, если обработано.
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

	case states.StateWaitingHeight:
		log.Printf(locales.LogProcessingHeight, chatID)
		collector.HandleHeight(ctx, b, chatID, text)
		return true

	case states.StateWaitingWeight:
		log.Printf(locales.LogProcessingWeight, chatID)
		collector.HandleWeight(ctx, b, chatID, text)
		return true

	case states.StateWaitingChronicDiseases:
		log.Printf(locales.LogProcessingChronicDiseases, chatID)
		collector.HandleChronicDiseases(ctx, b, chatID, text)
		return true

	case states.StateWaitingAllergies:
		log.Printf(locales.LogProcessingAllergies, chatID)
		collector.HandleAllergies(ctx, b, chatID, text)
		return true

	case states.StateWaitingMedications:
		log.Printf(locales.LogProcessingMedications, chatID)
		collector.HandleMedications(ctx, b, chatID, text)
		return true

	case states.StateWaitingSmoking:
		log.Printf(locales.LogProcessingSmoking, chatID)
		collector.HandleSmoking(ctx, b, chatID, text)
		return true

	case states.StateWaitingAlcohol:
		log.Printf(locales.LogProcessingAlcohol, chatID)
		collector.HandleAlcohol(ctx, b, chatID, text)
		return true

	case states.StateWaitingSportType:
		log.Printf(locales.LogProcessingSportType, chatID)
		collector.HandleSportType(ctx, b, chatID, text)
		return true

	case states.StateWaitingTrainingExperience:
		log.Printf(locales.LogProcessingTrainingExp, chatID)
		collector.HandleTrainingExperience(ctx, b, chatID, text)
		return true

	case states.StateWaitingGoal:
		log.Printf(locales.LogProcessingGoal, chatID)
		collector.HandleGoal(ctx, b, chatID, text)
		return true

	case states.StateWaitingCourseInfo:
		log.Printf(locales.LogProcessingCourseInfo, chatID)
		collector.HandleCourseInfo(ctx, b, chatID, text)
		return true

	case states.StateWaitingCourseTime:
		log.Printf(locales.LogProcessingCourseTime, chatID)
		collector.HandleCourseTime(ctx, b, chatID, text)
		return true
	}

	return false
}

// isBioscanState - проверяет, является ли состояние шагом Bioscan.
func isBioscanState(state states.State) bool {
	switch state {
	case states.StateWaitingBioscanName,
		states.StateWaitingBioscanAge,
		states.StateWaitingBioscanHeight,
		states.StateWaitingBioscanWeight,
		states.StateWaitingBioscanGoal,
		states.StateWaitingBioscanPhoto1,
		states.StateWaitingBioscanPhoto2,
		states.StateWaitingBioscanPhoto3,
		states.StateWaitingBioscanPhoto4,
		states.StateWaitingBioscanConfirm:
		return true
	}
	return false
}

// isQuestionnaireState - проверяет, является ли состояние шагом опросника.
func isQuestionnaireState(state states.State) bool {
	switch state {
	case states.StateWaitingName,
		states.StateWaitingGender,
		states.StateWaitingAge,
		states.StateWaitingHeight,
		states.StateWaitingWeight,
		states.StateWaitingChronicDiseases,
		states.StateWaitingAllergies,
		states.StateWaitingMedications,
		states.StateWaitingSmoking,
		states.StateWaitingAlcohol,
		states.StateWaitingSportType,
		states.StateWaitingTrainingExperience,
		states.StateWaitingGoal,
		states.StateWaitingCourseInfo,
		states.StateWaitingCourseTime:
		return true
	}
	return false
}
