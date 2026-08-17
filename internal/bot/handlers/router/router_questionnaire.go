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

	case states.StateWaitingSleep:
		log.Printf(locales.LogProcessingSleep, chatID)
		collector.HandleSleep(ctx, b, chatID, text)
		return true

	case states.StateWaitingStress:
		log.Printf(locales.LogProcessingStress, chatID)
		collector.HandleStress(ctx, b, chatID, text)
		return true

	case states.StateWaitingNutritionVeg:
		log.Printf(locales.LogProcessingNutritionVeg, chatID)
		collector.HandleNutritionVeg(ctx, b, chatID, text)
		return true

	case states.StateWaitingNutritionProcessed:
		log.Printf(locales.LogProcessingNutritionProcessed, chatID)
		collector.HandleNutritionProcessed(ctx, b, chatID, text)
		return true

	case states.StateWaitingWater:
		log.Printf(locales.LogProcessingWater, chatID)
		collector.HandleWater(ctx, b, chatID, text)
		return true

	case states.StateWaitingActivity:
		log.Printf(locales.LogProcessingActivity, chatID)
		collector.HandleActivity(ctx, b, chatID, text)
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

	case states.StateWaitingFamilyHistory:
		log.Printf(locales.LogProcessingFamilyHistory, chatID)
		collector.HandleFamilyHistory(ctx, b, chatID, text)
		return true

	case states.StateWaitingDigestion:
		log.Printf(locales.LogProcessingDigestion, chatID)
		collector.HandleDigestion(ctx, b, chatID, text)
		return true

	case states.StateWaitingSportType:
		log.Printf(locales.LogProcessingSportType, chatID)
		collector.HandleSportType(ctx, b, chatID, text)
		return true

	case states.StateWaitingGoal:
		log.Printf(locales.LogProcessingGoal, chatID)
		collector.HandleGoal(ctx, b, chatID, text)
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
		states.StateWaitingBioscanConfirm,
		states.StateWaitingBioscanBasicPhoto:
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
		states.StateWaitingSleep,
		states.StateWaitingStress,
		states.StateWaitingNutritionVeg,
		states.StateWaitingNutritionProcessed,
		states.StateWaitingWater,
		states.StateWaitingActivity,
		states.StateWaitingChronicDiseases,
		states.StateWaitingAllergies,
		states.StateWaitingMedications,
		states.StateWaitingSmoking,
		states.StateWaitingAlcohol,
		states.StateWaitingFamilyHistory,
		states.StateWaitingDigestion,
		states.StateWaitingSportType,
		states.StateWaitingGoal:
		return true
	}
	return false
}
