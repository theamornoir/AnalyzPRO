package userdata

import (
	"context"
	"strings"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// HandleChronicDiseases - обрабатывает хронические заболевания.
func (c *UserDataCollector) HandleChronicDiseases(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "chronic_diseases", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingAllergies)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgUserAllergies,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleAllergies - обрабатывает аллергии.
func (c *UserDataCollector) HandleAllergies(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "allergies", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingMedications)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgUserMedications,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleMedications - обрабатывает лекарства.
func (c *UserDataCollector) HandleMedications(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "medications", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingSmoking)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgUserSmoking,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleSmoking - обрабатывает курение.
func (c *UserDataCollector) HandleSmoking(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "smoking", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingAlcohol)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgUserAlcohol,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleAlcohol - обрабатывает алкоголь.
func (c *UserDataCollector) HandleAlcohol(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "alcohol", strings.TrimSpace(text))

	// Проверяем, спортсмен ли пользователь
	onCourse := c.stateManager.GetUserData(chatID, "on_course")

	if onCourse == "yes" {
		// Если спортсмен - спрашиваем про вид спорта
		c.stateManager.SetState(chatID, states.StateWaitingSportType)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgUserSportType,
			ReplyMarkup: keyboards.BackMenu(),
			ParseMode:   "Markdown",
		})
		return
	}

	// Если не спортсмен - завершаем сбор
	c.finishCollection(ctx, b, chatID)
}
