package userdata

import (
	"context"
	"fmt"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

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

// stepOrder - порядок вопросов опросника «Общая оценка здоровья». Сокращён
// до 7 ёмких вопросов (имя, пол, возраст, рост+вес, цель, образ жизни,
// вредные привычки). Пол/цель/привычки - варианты ответов (inline-кнопки),
// остальные - короткий текст. Используется для прогресс-бара «Вопрос N из M»
// и навигации «Назад» между вопросами.
var stepOrder = []states.State{
	states.StateWaitingName,
	states.StateWaitingGender,
	states.StateWaitingAge,
	states.StateWaitingHeightWeight,
	states.StateWaitingGoal,
	states.StateWaitingLifestyle,
	states.StateWaitingHabits,
}

// stepPrompt - текст вопроса для каждого состояния опросника.
var stepPrompt = map[states.State]string{
	states.StateWaitingName:         locales.MsgExtendedAnalysisIntro,
	states.StateWaitingGender:       locales.MsgUserGender,
	states.StateWaitingAge:          locales.MsgUserAge,
	states.StateWaitingHeightWeight: locales.MsgUserHeightWeight,
	states.StateWaitingGoal:         locales.MsgUserGoal,
	states.StateWaitingLifestyle:    locales.MsgUserLifestyle,
	states.StateWaitingHabits:       locales.MsgUserHabits,
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
		ReplyMarkup: keyboards.BackCancelQuestionInline(),
		ParseMode:   "Markdown",
	})
}

// SendChoiceStep - отправляет вопрос опросника с прогресс-баром
// «Вопрос N из M» и inline-клавиатурой вариантов ответа (кнопки). Используется
// для вопросов, где ответ выбирается из готовых вариантов (пол, цель, привычки).
func (c *UserDataCollector) SendChoiceStep(ctx context.Context, b *tgbot.Bot, chatID int64, state states.State, text string, kb models.InlineKeyboardMarkup) {
	idx := StepIndex(state)
	if idx < 0 {
		idx = 0
	}
	header := fmt.Sprintf("📋 Вопрос %d из %d\n\n", idx+1, len(stepOrder))
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        header + text,
		ReplyMarkup: kb,
		ParseMode:   "Markdown",
	})
}

// SendGoalQuestion - повторно отправляет вопрос о цели (inline-кнопки).
// Используется, когда демографические данные уже известны из постоянного
// профиля и нужно пропустить вопросы имя/возраст/пол/рост/вес, перейдя
// сразу к уникальным вопросам опросника «Общая оценка здоровья».
func (c *UserDataCollector) SendGoalQuestion(ctx context.Context, b *tgbot.Bot, chatID int64) {
	c.SendChoiceStep(ctx, b, chatID, states.StateWaitingGoal, locales.MsgUserGoal, questionnaireGoalKeyboard())
}
