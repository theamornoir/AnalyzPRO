package userdata

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// HandleName - обрабатывает имя.
func (c *UserDataCollector) HandleName(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	name := strings.TrimSpace(text)
	if len(name) < 2 || len(name) > 50 {
		c.SendStep(ctx, b, chatID, states.StateWaitingName, locales.MsgUserNameInvalid)
		return
	}

	c.stateManager.SetUserData(chatID, "name", name)
	c.stateManager.SetState(chatID, states.StateWaitingGender)

	c.SendStep(ctx, b, chatID, states.StateWaitingGender, locales.MsgUserGender)
}

// HandleGender - обрабатывает пол.
func (c *UserDataCollector) HandleGender(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	gender := strings.ToLower(strings.TrimSpace(text))

	if gender == "мужской" || gender == "м" || gender == "male" {
		c.stateManager.SetUserData(chatID, "gender", "Мужской")
	} else if gender == "женский" || gender == "ж" || gender == "female" {
		c.stateManager.SetUserData(chatID, "gender", "Женский")
	} else {
		c.SendStep(ctx, b, chatID, states.StateWaitingGender, locales.MsgUserGenderInvalid)
		return
	}

	c.stateManager.SetState(chatID, states.StateWaitingAge)

	c.SendStep(ctx, b, chatID, states.StateWaitingAge, locales.MsgUserAge)
}

// HandleAge - обрабатывает возраст.
func (c *UserDataCollector) HandleAge(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	age, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || age < 5 || age > 90 {
		c.SendStep(ctx, b, chatID, states.StateWaitingAge, locales.MsgUserAgeInvalid)
		return
	}

	c.stateManager.SetUserData(chatID, "age", fmt.Sprintf("%d", age))
	c.stateManager.SetState(chatID, states.StateWaitingHeight)

	c.SendStep(ctx, b, chatID, states.StateWaitingHeight, locales.MsgUserHeight)
}

// HandleHeight - обрабатывает рост.
func (c *UserDataCollector) HandleHeight(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	height, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || height < 50 || height > 210 {
		c.SendStep(ctx, b, chatID, states.StateWaitingHeight, locales.MsgUserHeightInvalid)
		return
	}

	c.stateManager.SetUserData(chatID, "height", fmt.Sprintf("%d", height))
	c.stateManager.SetState(chatID, states.StateWaitingWeight)

	c.SendStep(ctx, b, chatID, states.StateWaitingWeight, locales.MsgUserWeight)
}

// HandleWeight - обрабатывает вес.
func (c *UserDataCollector) HandleWeight(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	weight, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || weight < 30 || weight > 200 {
		c.SendStep(ctx, b, chatID, states.StateWaitingWeight, locales.MsgUserWeightInvalid)
		return
	}

	c.stateManager.SetUserData(chatID, "weight", fmt.Sprintf("%d", weight))
	c.stateManager.SetState(chatID, states.StateWaitingSleep)

	c.SendStep(ctx, b, chatID, states.StateWaitingSleep, locales.MsgUserSleep)
}
