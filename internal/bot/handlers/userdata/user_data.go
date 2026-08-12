package userdata

import (
	"context"

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

// StartCollection - начинает сбор данных.
func (c *UserDataCollector) StartCollection(ctx context.Context, b *tgbot.Bot, chatID int64) {
	c.stateManager.SetState(chatID, states.StateWaitingName)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgUserDataStart,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}
