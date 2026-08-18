package bioscan

import (
	"context"
	"fmt"
	"strings"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// bioscanQuestion - один вопрос опросника расширенного Bioscan PRO.
type bioscanQuestion struct {
	state  states.State
	key    string
	prompt string
	next   states.State
}

// bioscanQuestionnaire - последовательность вопросов, идущих ПОСЛЕ цели и ДО
// загрузки 4 фотографий. Это отдельный от анализа блок: здесь спрашиваем про
// образ жизни, тренировки, травмы и здоровье - те сферы, которые нужны для
// детального отчёта Body Intelligence (программа тренировок под травмы и опыт,
// разбор осанки с учётом жалоб, рекомендации под питание и сон).
var bioscanQuestionnaire = []bioscanQuestion{
	{states.StateWaitingBioscanTrainingExp, "bioscan_training_exp", locales.MsgBioscanQTrainingExp, states.StateWaitingBioscanTrainingFreq},
	{states.StateWaitingBioscanTrainingFreq, "bioscan_training_freq", locales.MsgBioscanQTrainingFreq, states.StateWaitingBioscanTrainingType},
	{states.StateWaitingBioscanTrainingType, "bioscan_training_type", locales.MsgBioscanQTrainingType, states.StateWaitingBioscanInjuries},
	{states.StateWaitingBioscanInjuries, "bioscan_injuries", locales.MsgBioscanQInjuries, states.StateWaitingBioscanPostureIssues},
	{states.StateWaitingBioscanPostureIssues, "bioscan_posture_issues", locales.MsgBioscanQPostureIssues, states.StateWaitingBioscanImproveZones},
	{states.StateWaitingBioscanImproveZones, "bioscan_improve_zones", locales.MsgBioscanQImproveZones, states.StateWaitingBioscanMobility},
	{states.StateWaitingBioscanMobility, "bioscan_mobility", locales.MsgBioscanQMobility, states.StateWaitingBioscanRecovery},
	{states.StateWaitingBioscanRecovery, "bioscan_recovery", locales.MsgBioscanQRecovery, states.StateWaitingBioscanSleep},
	{states.StateWaitingBioscanSleep, "bioscan_sleep", locales.MsgBioscanQSleep, states.StateWaitingBioscanStress},
	{states.StateWaitingBioscanStress, "bioscan_stress", locales.MsgBioscanQStress, states.StateWaitingBioscanNutrition},
	{states.StateWaitingBioscanNutrition, "bioscan_nutrition", locales.MsgBioscanQNutrition, states.StateWaitingBioscanProtein},
	{states.StateWaitingBioscanProtein, "bioscan_protein", locales.MsgBioscanQProtein, states.StateWaitingBioscanWater},
	{states.StateWaitingBioscanWater, "bioscan_water", locales.MsgBioscanQWater, states.StateWaitingBioscanSmoking},
	{states.StateWaitingBioscanSmoking, "bioscan_smoking", locales.MsgBioscanQSmoking, states.StateWaitingBioscanAlcohol},
	{states.StateWaitingBioscanAlcohol, "bioscan_alcohol", locales.MsgBioscanQAlcohol, states.StateWaitingBioscanSedentary},
	{states.StateWaitingBioscanSedentary, "bioscan_sedentary", locales.MsgBioscanQSedentary, states.StateWaitingBioscanBodyFatGoal},
	{states.StateWaitingBioscanBodyFatGoal, "bioscan_body_fat_goal", locales.MsgBioscanQBodyFatGoal, states.StateWaitingBioscanDietRestrictions},
	{states.StateWaitingBioscanDietRestrictions, "bioscan_diet_restrictions", locales.MsgBioscanQDietRestrictions, states.StateWaitingBioscanPhoto1},
}

// HandleBioscanQuestionnaireState - обрабатывает текущий вопрос опросника
// Bioscan PRO. Возвращает true, если состояние относится к опроснику и
// обработано (в т.ч. при повторном запросе того же вопроса на пустом вводе).
func HandleBioscanQuestionnaireState(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64, text string) bool {
	state := sm.GetState(chatID)
	idx := -1
	for i, q := range bioscanQuestionnaire {
		if q.state == state {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}

	q := bioscanQuestionnaire[idx]

	t := strings.TrimSpace(text)
	// Кнопки навигации не должны трактоваться как ответ на вопрос.
	if t == locales.BtnBack || t == locales.BtnCancel {
		return false
	}
	if t == "" {
		// Пустой ввод - повторяем тот же вопрос, ничего не сохраняем.
		sendBioscanQuestionText(ctx, b, chatID, idx, q.prompt)
		return true
	}

	sm.SetUserData(chatID, q.key, t)
	sm.SetState(chatID, q.next)

	if q.next == states.StateWaitingBioscanPhoto1 {
		// Последний вопрос опросника - переходим к загрузке 4 фото.
		sendPhotoPrompt(ctx, b, sm, chatID, states.StateWaitingBioscanPhoto1)
		return true
	}

	// Находим промпт следующего вопроса и отправляем его.
	for i, nq := range bioscanQuestionnaire {
		if nq.state == q.next {
			sendBioscanQuestionText(ctx, b, chatID, i, nq.prompt)
			return true
		}
	}
	return true
}

// sendBioscanQuestionText - отправляет вопрос опросника Bioscan PRO с
// прогресс-баром «Вопрос N из M» и клавиатурой [Назад / ❌ Отмена].
func sendBioscanQuestionText(ctx context.Context, b *tgbot.Bot, chatID int64, idx int, prompt string) {
	header := fmt.Sprintf("📋 Вопрос %d из %d\n\n", idx+1, len(bioscanQuestionnaire))
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        header + prompt,
		ReplyMarkup: keyboards.BackCancelMenu(),
		ParseMode:   "Markdown",
	})
}

// bioscanQuestionnaireKeys - ключи user-data всех вопросов опросника
// Bioscan PRO. Используется для очистки собранных данных при старте/сбросе.
func bioscanQuestionnaireKeys() []string {
	keys := make([]string, 0, len(bioscanQuestionnaire))
	for _, q := range bioscanQuestionnaire {
		keys = append(keys, q.key)
	}
	return keys
}

// IsBioscanQuestionnaireState - true, если состояние относится к опроснику
// Bioscan PRO (вопросы про образ жизни/здоровье, НЕ фото и НЕ подтверждение).
func IsBioscanQuestionnaireState(s states.State) bool {
	for _, q := range bioscanQuestionnaire {
		if q.state == s {
			return true
		}
	}
	return false
}

// BioscanPrevQuestionState - состояние предыдущего вопроса опросника
// (StateIdle, если вопрос первый - сигнал к выходу из опросника).
func BioscanPrevQuestionState(s states.State) states.State {
	for i, q := range bioscanQuestionnaire {
		if q.state == s {
			if i == 0 {
				return states.StateIdle
			}
			return bioscanQuestionnaire[i-1].state
		}
	}
	return states.StateIdle
}

// BioscanQuestionPrompt - текст вопроса опросника по состоянию.
func BioscanQuestionPrompt(s states.State) string {
	for _, q := range bioscanQuestionnaire {
		if q.state == s {
			return q.prompt
		}
	}
	return ""
}

// SendBioscanQuestion - повторно отправляет вопрос опросника по состоянию
// (используется при шаге «Назад» внутри опросника).
func SendBioscanQuestion(ctx context.Context, b *tgbot.Bot, chatID int64, state states.State) {
	idx := -1
	for i, q := range bioscanQuestionnaire {
		if q.state == state {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	sendBioscanQuestionText(ctx, b, chatID, idx, bioscanQuestionnaire[idx].prompt)
}
