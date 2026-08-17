package userdata

import (
	"context"
	"strings"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// HandleSportType - обрабатывает вид спорта.
func (c *UserDataCollector) HandleSportType(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "sport_type", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingGoal)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgUserGoal,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleGoal - обрабатывает цель и завершает сбор (20 вопросов).
func (c *UserDataCollector) HandleGoal(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "goal", strings.TrimSpace(text))
	c.finishCollection(ctx, b, chatID)
}
