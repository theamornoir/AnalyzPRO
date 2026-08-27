package bioscan

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

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
		ReplyMarkup: keyboards.BackQuestionInline(),
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
			ReplyMarkup: keyboards.BackQuestionInline(),
		})
		return
	}

	sm.SetUserData(chatID, "bioscan_age", text)
	sm.SetState(chatID, states.StateWaitingBioscanHeightWeight)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanStepHeightWeight,
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackQuestionInline(),
	})
}

// HandleBioscanHeightWeight - обработка РОСТА И ВЕСА, введённых одним
// сообщением (например «180 78»). Сохраняет оба значения в отдельные ключи.
func HandleBioscanHeightWeight(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64, text string) {
	if text == "" || strings.TrimSpace(text) == locales.BtnBack {
		return
	}

	height, weight, ok := parseBioscanHeightWeight(text)
	if !ok {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanHeightWeightInvalid,
			ReplyMarkup: keyboards.BackQuestionInline(),
		})
		return
	}

	sm.SetUserData(chatID, "bioscan_height", strconv.Itoa(height))
	sm.SetUserData(chatID, "bioscan_weight", strconv.Itoa(weight))
	sm.SetState(chatID, states.StateWaitingBioscanGender)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanQGender,
		ReplyMarkup: bioscanProGenderKeyboard(),
	})
}

// HandleBioscanGender - выбор пола (inline-кнопка) в опроснике Bioscan PRO.
func HandleBioscanGender(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64, gender string) {
	gender = normalizeGender(gender)
	sm.SetUserData(chatID, "bioscan_gender", gender)
	sm.SetState(chatID, states.StateWaitingBioscanGoal)
	askBioscanProGoal(ctx, b, chatID)
}

// HandleBioscanGoal - выбор цели (inline-кнопка) в опроснике Bioscan PRO.
// После цели идёт вопрос об уровне тренированности.
func HandleBioscanGoal(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64, goal string) {
	goal = cleanBioscanProGoal(goal)
	sm.SetUserData(chatID, "bioscan_goal", goal)
	sm.SetState(chatID, states.StateWaitingBioscanTrainingLevel)
	askBioscanProTrainingLevel(ctx, b, chatID)
}

// HandleBioscanTrainingLevel - выбор уровня тренированности (inline-кнопка)
// в опроснике Bioscan PRO. Это последний вопрос до загрузки 4 фото.
func HandleBioscanTrainingLevel(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64, level string) {
	level = cleanBioscanProLevel(level)
	sm.SetUserData(chatID, "bioscan_training_level", level)
	sm.SetState(chatID, states.StateWaitingBioscanTrainingExp)

	// Переходим к следующему вопросу среза опросника (стаж тренировок, idx 1).
	// Он обрабатывается универсальным HandleBioscanQuestionnaireState.
	sendBioscanQuestionText(ctx, b, chatID, 1, locales.MsgBioscanQTrainingExp)
}

// parseBioscanHeightWeight парсит рост и вес из одной строки (пробел/запятая/
// слэш). Возвращает (рост, вес, ok).
func parseBioscanHeightWeight(text string) (int, int, bool) {
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
	if height < 50 || height > 300 || weight < 20 || weight > 500 {
		return 0, 0, false
	}
	return height, weight, true
}

// cleanBioscanProGoal приводит текст/кнопку цели Bioscan PRO к чистому значению.
func cleanBioscanProGoal(btn string) string {
	switch strings.TrimSpace(btn) {
	case locales.BtnBioscanProGoalMass, "Набор", "💪 Набор":
		return "набор мышечной массы"
	case locales.BtnBioscanProGoalCut, "Сушка", "🔥 Сушка":
		return "снижение веса"
	case locales.BtnBioscanProGoalKeep, "Поддержание", "⚖️ Поддержание":
		return "поддержание формы"
	default:
		return strings.TrimSpace(btn)
	}
}

// cleanBioscanProLevel приводит текст/кнопку уровня тренированности к чистому
// значению.
func cleanBioscanProLevel(btn string) string {
	switch strings.TrimSpace(btn) {
	case locales.BtnBioscanProLevelNovice, "Новичок":
		return "новичок"
	case locales.BtnBioscanProLevelAmateur, "Любитель":
		return "любитель"
	case locales.BtnBioscanProLevelPro, "Профи":
		return "профи"
	default:
		return strings.TrimSpace(btn)
	}
}

// bioscanProGenderKeyboard - inline-кнопки выбора пола (Bioscan PRO).
func bioscanProGenderKeyboard() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: locales.BtnBioscanBasicMale, CallbackData: "bioscan_pro_gender_m"}},
			{{Text: locales.BtnBioscanBasicFemale, CallbackData: "bioscan_pro_gender_f"}},
			{{Text: locales.BtnBack, CallbackData: "bioscan_question_back"}},
		},
	}
}

// askBioscanProGoal - вопрос о цели (Bioscan PRO) с inline-кнопками.
func askBioscanProGoal(ctx context.Context, b *tgbot.Bot, chatID int64) {
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanStepGoal,
		ReplyMarkup: bioscanProGoalKeyboard(),
	})
}

// AskBioscanProGoal - экспортируемая обёртка askBioscanProGoal (вопрос о
// цели Bioscan PRO). Используется роутером, когда демографические данные
// уже известны из профиля и нужно пропустить вопросы имя/возраст/пол/рост/вес.
func AskBioscanProGoal(ctx context.Context, b *tgbot.Bot, chatID int64) {
	askBioscanProGoal(ctx, b, chatID)
}

// bioscanProGoalKeyboard - inline-кнопки выбора цели (Bioscan PRO).
func bioscanProGoalKeyboard() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: locales.BtnBioscanProGoalMass, CallbackData: "bioscan_pro_goal_mass"},
				{Text: locales.BtnBioscanProGoalCut, CallbackData: "bioscan_pro_goal_cut"},
				{Text: locales.BtnBioscanProGoalKeep, CallbackData: "bioscan_pro_goal_keep"},
			},
			{{Text: locales.BtnBack, CallbackData: "bioscan_question_back"}},
		},
	}
}

// askBioscanProTrainingLevel - вопрос об уровне тренированности (Bioscan PRO)
// с inline-кнопками.
func askBioscanProTrainingLevel(ctx context.Context, b *tgbot.Bot, chatID int64) {
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgBioscanQTrainingLevel,
		ReplyMarkup: bioscanProLevelKeyboard(),
	})
}

// bioscanProLevelKeyboard - inline-кнопки выбора уровня тренированности.
func bioscanProLevelKeyboard() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: locales.BtnBioscanProLevelNovice, CallbackData: "bioscan_pro_level_novice"},
				{Text: locales.BtnBioscanProLevelAmateur, CallbackData: "bioscan_pro_level_amateur"},
				{Text: locales.BtnBioscanProLevelPro, CallbackData: "bioscan_pro_level_pro"},
			},
			{{Text: locales.BtnBack, CallbackData: "bioscan_question_back"}},
		},
	}
}
