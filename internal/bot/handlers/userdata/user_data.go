package userdata

import (
	"context"
	"fmt"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// UserDataCollector - собирает данные о пользователе.
type UserDataCollector struct {
	stateManager states.StateManager
}

// NewUserDataCollector создаёт новый коллектор данных пользователя.
func NewUserDataCollector(stateManager states.StateManager) *UserDataCollector {
	return &UserDataCollector{
		stateManager: stateManager,
	}
}

// stepOrder - порядок вопросов 20-вопросного опросника анализа. Используется
// для прогресс-бара «Вопрос N из M» и навигации «Назад» между вопросами.
var stepOrder = []states.State{
	states.StateWaitingName,
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
	states.StateWaitingGoal,
}

// stepPrompt - текст вопроса для каждого состояния опросника.
var stepPrompt = map[states.State]string{
	states.StateWaitingName:               locales.MsgExtendedAnalysisIntro,
	states.StateWaitingGender:             locales.MsgUserGender,
	states.StateWaitingAge:                locales.MsgUserAge,
	states.StateWaitingHeight:             locales.MsgUserHeight,
	states.StateWaitingWeight:             locales.MsgUserWeight,
	states.StateWaitingSleep:              locales.MsgUserSleep,
	states.StateWaitingStress:             locales.MsgUserStress,
	states.StateWaitingNutritionVeg:       locales.MsgUserNutritionVeg,
	states.StateWaitingNutritionProcessed: locales.MsgUserNutritionProcessed,
	states.StateWaitingWater:              locales.MsgUserWater,
	states.StateWaitingActivity:           locales.MsgUserActivity,
	states.StateWaitingChronicDiseases:    locales.MsgUserChronicDiseases,
	states.StateWaitingAllergies:          locales.MsgUserAllergies,
	states.StateWaitingMedications:        locales.MsgUserMedications,
	states.StateWaitingSmoking:            locales.MsgUserSmoking,
	states.StateWaitingAlcohol:            locales.MsgUserAlcohol,
	states.StateWaitingFamilyHistory:      locales.MsgUserFamilyHistory,
	states.StateWaitingDigestion:          locales.MsgUserDigestion,
	states.StateWaitingSportType:          locales.MsgUserSportType,
	states.StateWaitingGoal:               locales.MsgUserGoal,
}

// StepCount - общее число вопросов опросника.
func StepCount() int { return len(stepOrder) }

// StepIndex - индекс вопроса (0-based) по состоянию, либо -1.
func StepIndex(s states.State) int {
	for i, st := range stepOrder {
		if st == s {
			return i
		}
	}
	return -1
}

// PrevStep - состояние предыдущего вопроса (StateIdle, если вопрос первый).
func PrevStep(s states.State) states.State {
	idx := StepIndex(s)
	if idx <= 0 {
		return states.StateIdle
	}
	return stepOrder[idx-1]
}

// PromptForState - текст вопроса по состоянию.
func PromptForState(s states.State) string {
	if p, ok := stepPrompt[s]; ok {
		return p
	}
	return ""
}

// SendStep - отправляет вопрос опросника с прогресс-баром
// «Вопрос N из M» и клавиатурой [Назад / ❌ Отмена].
func (c *UserDataCollector) SendStep(ctx context.Context, b *tgbot.Bot, chatID int64, state states.State, text string) {
	idx := StepIndex(state)
	if idx < 0 {
		idx = 0
	}
	header := fmt.Sprintf("📋 Вопрос %d из %d\n\n", idx+1, len(stepOrder))
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        header + text,
		ReplyMarkup: keyboards.BackCancelMenu(),
		ParseMode:   "Markdown",
	})
}
