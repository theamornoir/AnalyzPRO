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

// HandleSportType - обрабатывает вид спорта.
func (c *UserDataCollector) HandleSportType(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "sport_type", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingTrainingExperience)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgUserTrainingExp,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleTrainingExperience - обрабатывает стаж тренировок.
func (c *UserDataCollector) HandleTrainingExperience(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	exp, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || exp < 0 || exp > 80 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgUserTrainingExpInvalid,
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	c.stateManager.SetUserData(chatID, "training_experience", fmt.Sprintf("%d", exp))
	c.stateManager.SetState(chatID, states.StateWaitingGoal)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgUserGoal,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleGoal - обрабатывает цель.
func (c *UserDataCollector) HandleGoal(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "goal", strings.TrimSpace(text))

	// Вопрос о препаратах
	c.stateManager.SetState(chatID, states.StateWaitingCourseInfo)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgUserCourseInfo,
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleCourseInfo - обрабатывает ответ про препараты.
func (c *UserDataCollector) HandleCourseInfo(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	text = strings.ToLower(strings.TrimSpace(text))

	if text == "да" || text == "да." || text == "ага" || text == "yes" || text == "д" || text == "+" {
		c.stateManager.SetUserData(chatID, "on_course", "yes")
		c.stateManager.SetState(chatID, states.StateWaitingCourseTime)

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgUserCourseSubstance,
			ReplyMarkup: keyboards.BackMenu(),
			ParseMode:   "Markdown",
		})
		return
	}

	if text == "нет" || text == "нет." || text == "неа" || text == "no" || text == "н" || text == "-" {
		c.stateManager.SetUserData(chatID, "on_course", "no")
		c.finishCollection(ctx, b, chatID)
		return
	}

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgUserCourseInfoInvalid,
		ReplyMarkup: keyboards.BackMenu(),
	})
}

// HandleCourseTime - обрабатывает ответ про время курса.
func (c *UserDataCollector) HandleCourseTime(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	if strings.TrimSpace(text) == "" {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgUserCourseTimeEmpty,
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	c.stateManager.SetUserData(chatID, "course_info", strings.TrimSpace(text))
	c.finishCollection(ctx, b, chatID)
}
