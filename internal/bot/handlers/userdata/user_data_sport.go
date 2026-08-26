package userdata

import (
	"context"
	"strings"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// HandleSportType - обрабатывает вид спорта.
func (c *UserDataCollector) HandleSportType(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "sport_type", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingGoal)

	c.SendStep(ctx, b, chatID, states.StateWaitingGoal, locales.MsgUserGoal)
}

// HandleGoal - обрабатывает цель и переводит на первый из новых вопросов
// «Общей оценки здоровья» (расширение опросника 20 -> 28 вопросов).
func (c *UserDataCollector) HandleGoal(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "goal", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingEnergy)

	c.SendStep(ctx, b, chatID, states.StateWaitingEnergy, locales.MsgUserEnergy)
}
