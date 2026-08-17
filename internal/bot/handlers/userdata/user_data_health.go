package userdata

import (
	"context"
	"strings"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// HandleChronicDiseases - обрабатывает хронические заболевания.
func (c *UserDataCollector) HandleChronicDiseases(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "chronic_diseases", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingAllergies)

	c.SendStep(ctx, b, chatID, states.StateWaitingAllergies, locales.MsgUserAllergies)
}

// HandleAllergies - обрабатывает аллергии.
func (c *UserDataCollector) HandleAllergies(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "allergies", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingMedications)

	c.SendStep(ctx, b, chatID, states.StateWaitingMedications, locales.MsgUserMedications)
}

// HandleMedications - обрабатывает лекарства.
func (c *UserDataCollector) HandleMedications(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "medications", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingSmoking)

	c.SendStep(ctx, b, chatID, states.StateWaitingSmoking, locales.MsgUserSmoking)
}

// HandleSmoking - обрабатывает курение.
func (c *UserDataCollector) HandleSmoking(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "smoking", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingAlcohol)

	c.SendStep(ctx, b, chatID, states.StateWaitingAlcohol, locales.MsgUserAlcohol)
}

// HandleAlcohol - обрабатывает алкоголь. После него переходим к семейному
// анамнезу, затем к ЖКТ, виду спорта и цели (20-вопросный опросник).
func (c *UserDataCollector) HandleAlcohol(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "alcohol", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingFamilyHistory)

	c.SendStep(ctx, b, chatID, states.StateWaitingFamilyHistory, locales.MsgUserFamilyHistory)
}
