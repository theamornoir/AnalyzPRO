package userdata

import (
	"context"
	"fmt"
	"strconv"
	"strings"

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

// HealthSkipKey - ключ user-data, хранящий число ВЕДУЩИХ вопросов опросника
// «Общая оценка здоровья», уже отвеченных через профиль (Mini App / сохранённый
// профиль) и пропущенных при старте. Нужен прогресс-бару «Вопрос N из M»: если
// демография (имя/пол/возраст/рост+вес) подставлена из профиля, опросник
// начинается с цели (5-й вопрос из 7), и показ «Вопрос 5 из 7» обманчив - на
// самом деле это 1-й из 3 оставшихся (цель/образ жизни/привычки). Смещение
// вычитается из абсолютного индекса и из общего числа.
const HealthSkipKey = "health_quest_skip"

// SkippedSteps - число ведущих вопросов, уже отвеченных через профиль
// (прочитано из user-data по HealthSkipKey). Возвращает 0, если ключ не
// задан/невалиден.
func SkippedSteps(sm states.StateManager, chatID int64) int {
	v := strings.TrimSpace(sm.GetUserData(chatID, HealthSkipKey))
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

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

// progressHeader - формирует прогресс-бар «Вопрос N из M» с учётом шагов,
// уже отвеченных через профиль (HealthSkipKey). Если демография подставлена
// из профиля, опросник начинается не с 1-го, а с первого НЕотвеченного вопроса,
// поэтому считаем позицию ОТНОСИТЕЛЬНО оставшихся вопросов: цель, образ
// жизни, привычки -> «Вопрос 1 из 3», «2 из 3», «3 из 3» (а не «5 из 7»).
func (c *UserDataCollector) progressHeader(chatID int64, state states.State) string {
	absIdx := StepIndex(state)
	if absIdx < 0 {
		absIdx = 0
	}
	skip := SkippedSteps(c.stateManager, chatID)
	total := StepCount() - skip
	if total < 1 {
		total = 1
	}
	relIdx := absIdx - skip
	if relIdx < 0 {
		relIdx = 0
	}
	return fmt.Sprintf("📋 Вопрос %d из %d\n\n", relIdx+1, total)
}

// SendStep - отправляет вопрос опросника с прогресс-баром
// «Вопрос N из M» и клавиатурой [Назад / ❌ Отмена].
func (c *UserDataCollector) SendStep(ctx context.Context, b *tgbot.Bot, chatID int64, state states.State, text string) {
	header := c.progressHeader(chatID, state)
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
	header := c.progressHeader(chatID, state)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        header + text,
		ReplyMarkup: kb,
		ParseMode:   "Markdown",
	})
}

// SendGoalQuestion - повторно отправляет вопрос о цели (свободный текст).
// Используется, когда демографические данные уже известны из постоянного
// профиля и нужно пропустить вопросы имя/возраст/пол/рост/вес, перейдя
// сразу к уникальным вопросам опросника «Общая оценка здоровья».
func (c *UserDataCollector) SendGoalQuestion(ctx context.Context, b *tgbot.Bot, chatID int64) {
	c.SendStep(ctx, b, chatID, states.StateWaitingGoal, locales.MsgUserGoal)
}
