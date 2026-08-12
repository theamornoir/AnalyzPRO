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

// HandleBioscanName - обработка имени.
func HandleBioscanName(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64, text string) {
	if text == "" || strings.TrimSpace(text) == locales.BtnBack {
		return
	}

	text = strings.TrimSpace(text)
	sm.SetUserData(chatID, "bioscan_name", text)
	sm.SetState(chatID, states.StateWaitingBioscanAge)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        fmt.Sprintf(locales.MsgBioscanWelcomeName, text),
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackMenu(),
	})
}

// HandleBioscanAge - обработка возраста.
func HandleBioscanAge(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64, text string) {
	if text == "" || strings.TrimSpace(text) == locales.BtnBack {
		return
	}

	var age int
	_, err := fmt.Sscanf(text, "%d", &age)
	if err != nil || age < 10 || age > 120 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanAgeInvalid,
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	sm.SetUserData(chatID, "bioscan_age", text)
	sm.SetState(chatID, states.StateWaitingBioscanHeight)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanStepAge,
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackMenu(),
	})
}

// HandleBioscanHeight - обработка роста.
func HandleBioscanHeight(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64, text string) {
	if text == "" || strings.TrimSpace(text) == locales.BtnBack {
		return
	}

	var height int
	_, err := fmt.Sscanf(text, "%d", &height)
	if err != nil || height < 50 || height > 300 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanHeightInvalid,
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	sm.SetUserData(chatID, "bioscan_height", text)
	sm.SetState(chatID, states.StateWaitingBioscanWeight)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanStepHeight,
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackMenu(),
	})
}

// HandleBioscanWeight - обработка веса.
func HandleBioscanWeight(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64, text string) {
	if text == "" || strings.TrimSpace(text) == locales.BtnBack {
		return
	}

	var weight int
	_, err := fmt.Sscanf(text, "%d", &weight)
	if err != nil || weight < 20 || weight > 500 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanWeightInvalid,
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	sm.SetUserData(chatID, "bioscan_weight", text)
	sm.SetState(chatID, states.StateWaitingBioscanGoal)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanStepWeight,
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackMenu(),
	})
}

// HandleBioscanGoal - обработка цели.
func HandleBioscanGoal(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64, text string) {
	if text == "" || strings.TrimSpace(text) == locales.BtnBack {
		return
	}

	goal := strings.TrimSpace(text)
	goal = strings.TrimPrefix(goal, "💪 ")
	goal = strings.TrimPrefix(goal, "🔥 ")
	goal = strings.TrimPrefix(goal, "⚖️ ")
	goal = strings.TrimPrefix(goal, "🏃 ")
	goal = strings.TrimPrefix(goal, "🧘 ")

	sm.SetUserData(chatID, "bioscan_goal", goal)
	sm.SetState(chatID, states.StateWaitingBioscanPhoto1)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanStepGoal,
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackMenu(),
	})
}
