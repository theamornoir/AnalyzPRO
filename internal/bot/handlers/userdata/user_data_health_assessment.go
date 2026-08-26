package userdata

import (
	"context"
	"strings"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// Обработчики новых вопросов «Общей оценки здоровья» (расширение опросника
// 20 -> 28 вопросов). Идут ПОСЛЕ цели (StateWaitingGoal). Последний вопрос
// (StateWaitingPainAreas) завершает сбор через finishCollection, который
// переводит в терминальное состояние StateWaitingHealthAssessment (маршрутизатор
// генерирует отчёт ИИ на основе только текста опросника, без загрузки файлов).

// HandleEnergy - уровень энергии.
func (c *UserDataCollector) HandleEnergy(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "energy", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingMood)

	c.SendStep(ctx, b, chatID, states.StateWaitingMood, locales.MsgUserMood)
}

// HandleMood - настроение / эмоциональное состояние.
func (c *UserDataCollector) HandleMood(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "mood", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingWorkRegimen)

	c.SendStep(ctx, b, chatID, states.StateWaitingWorkRegimen, locales.MsgUserWorkRegimen)
}

// HandleWorkRegimen - режим работы / учёбы, сидячий образ жизни.
func (c *UserDataCollector) HandleWorkRegimen(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "work_regimen", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingScreenTime)

	c.SendStep(ctx, b, chatID, states.StateWaitingScreenTime, locales.MsgUserScreenTime)
}

// HandleScreenTime - экранное время.
func (c *UserDataCollector) HandleScreenTime(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "screen_time", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingMealRegularity)

	c.SendStep(ctx, b, chatID, states.StateWaitingMealRegularity, locales.MsgUserMealRegularity)
}

// HandleMealRegularity - регулярность питания.
func (c *UserDataCollector) HandleMealRegularity(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "meal_regularity", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingCaffeine)

	c.SendStep(ctx, b, chatID, states.StateWaitingCaffeine, locales.MsgUserCaffeine)
}

// HandleCaffeine - кофеин.
func (c *UserDataCollector) HandleCaffeine(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "caffeine", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingIllnessFreq)

	c.SendStep(ctx, b, chatID, states.StateWaitingIllnessFreq, locales.MsgUserIllnessFreq)
}

// HandleIllnessFreq - частота болезней / восстановление.
func (c *UserDataCollector) HandleIllnessFreq(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "illness_freq", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingPainAreas)

	c.SendStep(ctx, b, chatID, states.StateWaitingPainAreas, locales.MsgUserPainAreas)
}

// HandlePainAreas - боли / дискомфорт. Последний вопрос: завершает сбор.
func (c *UserDataCollector) HandlePainAreas(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "pain_areas", strings.TrimSpace(text))
	c.finishCollection(ctx, b, chatID)
}
