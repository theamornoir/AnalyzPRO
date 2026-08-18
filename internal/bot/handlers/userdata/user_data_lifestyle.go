package userdata

import (
	"context"
	"strings"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// HandleSleep - обрабатывает вопрос про сон (часы + качество).
func (c *UserDataCollector) HandleSleep(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "sleep", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingStress)

	c.SendStep(ctx, b, chatID, states.StateWaitingStress, locales.MsgUserStress)
}

// HandleStress - обрабатывает уровень стресса.
func (c *UserDataCollector) HandleStress(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "stress", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingNutritionVeg)

	c.SendStep(ctx, b, chatID, states.StateWaitingNutritionVeg, locales.MsgUserNutritionVeg)
}

// HandleNutritionVeg - обрабатывает частоту овощей/фруктов и т.п.
func (c *UserDataCollector) HandleNutritionVeg(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "nutrition_veg", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingNutritionProcessed)

	c.SendStep(ctx, b, chatID, states.StateWaitingNutritionProcessed, locales.MsgUserNutritionProcessed)
}

// HandleNutritionProcessed - обрабатывает частоту ультраобработанных продуктов.
func (c *UserDataCollector) HandleNutritionProcessed(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "nutrition_processed", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingWater)

	c.SendStep(ctx, b, chatID, states.StateWaitingWater, locales.MsgUserWater)
}

// HandleWater - обрабатывает питьевой режим.
func (c *UserDataCollector) HandleWater(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "water", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingActivity)

	c.SendStep(ctx, b, chatID, states.StateWaitingActivity, locales.MsgUserActivity)
}

// HandleActivity - обрабатывает физическую активность.
func (c *UserDataCollector) HandleActivity(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "activity", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingChronicDiseases)

	c.SendStep(ctx, b, chatID, states.StateWaitingChronicDiseases, locales.MsgUserChronicDiseases)
}

// HandleFamilyHistory - обрабатывает семейный анамнез.
func (c *UserDataCollector) HandleFamilyHistory(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "family_history", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingDigestion)

	c.SendStep(ctx, b, chatID, states.StateWaitingDigestion, locales.MsgUserDigestion)
}

// HandleDigestion - обрабатывает состояние ЖКТ / пищеварения.
func (c *UserDataCollector) HandleDigestion(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "digestion", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingSportType)

	c.SendStep(ctx, b, chatID, states.StateWaitingSportType, locales.MsgUserSportType)
}
