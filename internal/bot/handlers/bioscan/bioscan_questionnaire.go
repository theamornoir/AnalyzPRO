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

// bioscanQuestionnaire - последовательность вопросов опросника Bioscan PRO,
// идущих ПОСЛЕ цели и ДО загрузки 4 фотографий. Опросник сокращён до 7
// ёмких вопросов: имя, возраст, пол, рост+вес, цель, уровень тренированности
// и стаж тренировок. Уровень тренированности и цель выбираются inline-кнопками,
// стаж - свободным текстом. Этот срез содержит только вопросы ПОСЛЕ цели
// (пол/цель уже обработаны в bioscan_question.go), то есть здесь осталось
// два вопроса: уровень (кнопки) и стаж (текст), после чего - загрузка фото.
var bioscanQuestionnaire = []bioscanQuestion{
	{states.StateWaitingBioscanTrainingLevel, "bioscan_training_level", locales.MsgBioscanQTrainingLevel, states.StateWaitingBioscanTrainingExp},
	{states.StateWaitingBioscanTrainingExp, "bioscan_training_exp", locales.MsgBioscanQTrainingExp, states.StateWaitingBioscanPhoto1},
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
		ReplyMarkup: keyboards.BackCancelQuestionInlineBioscan(),
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
