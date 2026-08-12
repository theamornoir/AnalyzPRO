package userdata

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// HandleName - обрабатывает имя.
func (c *UserDataCollector) HandleName(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	name := strings.TrimSpace(text)
	if len(name) < 2 || len(name) > 50 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgUserNameInvalid,
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	c.stateManager.SetUserData(chatID, "name", name)
	c.stateManager.SetState(chatID, states.StateWaitingGender)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgUserGender,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleGender - обрабатывает пол.
func (c *UserDataCollector) HandleGender(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	gender := strings.ToLower(strings.TrimSpace(text))

	if gender == "мужской" || gender == "м" || gender == "male" {
		c.stateManager.SetUserData(chatID, "gender", "Мужской")
	} else if gender == "женский" || gender == "ж" || gender == "female" {
		c.stateManager.SetUserData(chatID, "gender", "Женский")
	} else {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgUserGenderInvalid,
			ReplyMarkup: keyboards.BackMenu(),
			ParseMode:   "Markdown",
		})
		return
	}

	c.stateManager.SetState(chatID, states.StateWaitingAge)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgUserAge,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleAge - обрабатывает возраст.
func (c *UserDataCollector) HandleAge(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	age, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || age < 5 || age > 90 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgUserAgeInvalid,
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	c.stateManager.SetUserData(chatID, "age", fmt.Sprintf("%d", age))
	c.stateManager.SetState(chatID, states.StateWaitingHeight)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgUserHeight,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleHeight - обрабатывает рост.
func (c *UserDataCollector) HandleHeight(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	height, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || height < 50 || height > 210 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgUserHeightInvalid,
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	c.stateManager.SetUserData(chatID, "height", fmt.Sprintf("%d", height))
	c.stateManager.SetState(chatID, states.StateWaitingWeight)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgUserWeight,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleWeight - обрабатывает вес.
func (c *UserDataCollector) HandleWeight(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	weight, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || weight < 30 || weight > 200 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgUserWeightInvalid,
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	c.stateManager.SetUserData(chatID, "weight", fmt.Sprintf("%d", weight))
	c.stateManager.SetState(chatID, states.StateWaitingChronicDiseases)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgUserChronicDiseases,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}
