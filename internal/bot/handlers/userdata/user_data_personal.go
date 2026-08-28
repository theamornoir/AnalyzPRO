package userdata

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

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

	c.SendChoiceStep(ctx, b, chatID, states.StateWaitingGender, locales.MsgUserGender, questionnaireGenderKeyboard())
}

// HandleGender - обрабатывает пол (текст или inline-кнопка).
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
	c.stateManager.SetState(chatID, states.StateWaitingHeightWeight)
	c.SendStep(ctx, b, chatID, states.StateWaitingHeightWeight, locales.MsgUserHeightWeight)
}

// HandleHeightWeight - обрабатывает РОСТ И ВЕС, введённые одним сообщением
// (например «180 70» или «180, 70»). Сохраняет оба значения в отдельные
// ключи user-data, чтобы они учитывались дальше (ИМТ и т.п.).
func (c *UserDataCollector) HandleHeightWeight(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	height, weight, ok := parseHeightWeight(text)
	if !ok {
		c.SendStep(ctx, b, chatID, states.StateWaitingHeightWeight, locales.MsgUserHeightWeightInvalid)
		return
	}

	c.stateManager.SetUserData(chatID, "height", strconv.Itoa(height))
	c.stateManager.SetUserData(chatID, "weight", strconv.Itoa(weight))
	c.stateManager.SetState(chatID, states.StateWaitingGoal)
	c.SendStep(ctx, b, chatID, states.StateWaitingGoal, locales.MsgUserGoal)
}

// HandleGoal - обрабатывает цель (свободный текст). После цели -
// вопрос об образе жизни (сон/стресс/питание/активность, 1 вопрос).
func (c *UserDataCollector) HandleGoal(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	goal := strings.TrimSpace(text)
	if len(goal) < 2 || len(goal) > 300 {
		c.SendStep(ctx, b, chatID, states.StateWaitingGoal, locales.MsgUserGoalInvalid)
		return
	}

	c.stateManager.SetUserData(chatID, "goal", goal)
	c.stateManager.SetState(chatID, states.StateWaitingLifestyle)
	c.SendStep(ctx, b, chatID, states.StateWaitingLifestyle, locales.MsgUserLifestyle)
}

// HandleLifestyle - обрабатывает объединённый вопрос об образе жизни
// (сон, стресс, питание, активность). Свободный текст - одно поле.
func (c *UserDataCollector) HandleLifestyle(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "lifestyle", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingHabits)
	c.SendChoiceStep(ctx, b, chatID, states.StateWaitingHabits, locales.MsgUserHabits, questionnaireHabitsKeyboard())
}

// HandleHabits - обрабатывает вредные привычки (inline-кнопка). Последний
// вопрос: завершает сбор.
func (c *UserDataCollector) HandleHabits(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	habits := normalizeHabits(text)
	if habits == "" {
		c.SendChoiceStep(ctx, b, chatID, states.StateWaitingHabits, locales.MsgUserHabitsInvalid, questionnaireHabitsKeyboard())
		return
	}
	c.stateManager.SetUserData(chatID, "habits", habits)
	c.finishCollection(ctx, b, chatID)
}

// parseHeightWeight парсит рост и вес из одной строки (через пробел/запятую/
// слэш). Возвращает (рост, вес, ok).
func parseHeightWeight(text string) (int, int, bool) {
	t := strings.ToLower(strings.TrimSpace(text))
	t = strings.ReplaceAll(t, ",", " ")
	t = strings.ReplaceAll(t, "/", " ")
	t = strings.ReplaceAll(t, ".", " ")
	fields := strings.Fields(t)
	if len(fields) < 2 {
		return 0, 0, false
	}

	height, herr := strconv.Atoi(fields[0])
	weight, werr := strconv.Atoi(fields[1])
	if herr != nil || werr != nil {
		return 0, 0, false
	}
	if height < 50 || height > 260 || weight < 20 || weight > 400 {
		return 0, 0, false
	}
	return height, weight, true
}

// normalizeHabits приводит текст/кнопку вредных привычек к чистому значению.
func normalizeHabits(btn string) string {
	switch strings.TrimSpace(btn) {
	case "Нет", locales.BtnHabitsNone:
		return "Нет"
	case "Курю", locales.BtnHabitsSmoke:
		return "Курю"
	case "Алкоголь", locales.BtnHabitsAlcohol:
		return "Алкоголь"
	case "Курю и алкоголь", locales.BtnHabitsBoth:
		return "Курю и алкоголь"
	default:
		g := strings.TrimSpace(btn)
		if g == "" {
			return ""
		}
		return g
	}
}

// questionnaireGenderKeyboard - inline-кнопки выбора пола.
func questionnaireGenderKeyboard() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: locales.BtnHealthGenderMale, CallbackData: "question_gender_m"}},
			{{Text: locales.BtnHealthGenderFemale, CallbackData: "question_gender_f"}},
			{{Text: locales.BtnBack, CallbackData: "questionnaire_back"}},
		},
	}
}

// questionnaireHabitsKeyboard - inline-кнопки выбора вредных привычек.
func questionnaireHabitsKeyboard() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: locales.BtnHabitsNone, CallbackData: "question_habits_none"},
				{Text: locales.BtnHabitsSmoke, CallbackData: "question_habits_smoke"},
			},
			{
				{Text: locales.BtnHabitsAlcohol, CallbackData: "question_habits_alcohol"},
				{Text: locales.BtnHabitsBoth, CallbackData: "question_habits_both"},
			},
			{{Text: locales.BtnBack, CallbackData: "questionnaire_back"}},
		},
	}
}
