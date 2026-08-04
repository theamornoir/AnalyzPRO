package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/theamornoir/analyzpro/internal/bot/states"
)

// UserDataCollector - собирает данные о пользователе
type UserDataCollector struct {
	stateManager states.StateManager
}

func NewUserDataCollector(stateManager states.StateManager) *UserDataCollector {
	return &UserDataCollector{
		stateManager: stateManager,
	}
}

// StartCollection - начинает сбор данных
func (c *UserDataCollector) StartCollection(ctx context.Context, b *tgbot.Bot, chatID int64) {
	c.stateManager.SetState(chatID, states.StateWaitingAge)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text: "📋 Для более точного анализа мне нужно знать немного о вас.\n\n" +
			"1️⃣ **Сколько вам лет?**\n" +
			"Напишите число (например: 25)",
		ParseMode: "Markdown",
	})
}

// HandleAge - обрабатывает возраст
func (c *UserDataCollector) HandleAge(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	age, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || age < 5 || age > 90 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Пожалуйста, укажите корректный возраст (от 5 до 90 лет).\n\nПопробуйте ещё раз:",
		})
		return
	}

	c.stateManager.SetUserData(chatID, "age", fmt.Sprintf("%d", age))
	c.stateManager.SetState(chatID, states.StateWaitingHeight)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    chatID,
		Text:      "2️⃣ **Какой у вас рост?**\nНапишите в сантиметрах (например: 178)",
		ParseMode: "Markdown",
	})
}

// HandleHeight - обрабатывает рост
func (c *UserDataCollector) HandleHeight(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	height, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || height < 50 || height > 210 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Пожалуйста, укажите корректный рост (от 50 до 210 см).\n\nПопробуйте ещё раз:",
		})
		return
	}

	c.stateManager.SetUserData(chatID, "height", fmt.Sprintf("%d", height))
	c.stateManager.SetState(chatID, states.StateWaitingWeight)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    chatID,
		Text:      "3️⃣ **Какой у вас вес?**\nНапишите в килограммах (например: 82)",
		ParseMode: "Markdown",
	})
}

// HandleWeight - обрабатывает вес
func (c *UserDataCollector) HandleWeight(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	weight, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || weight < 30 || weight > 200 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Пожалуйста, укажите корректный вес (от 30 до 200 кг).\n\nПопробуйте ещё раз:",
		})
		return
	}

	c.stateManager.SetUserData(chatID, "weight", fmt.Sprintf("%d", weight))
	c.stateManager.SetState(chatID, states.StateWaitingChronicDiseases)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text: "4️⃣ **Есть ли у вас хронические заболевания?**\n" +
			"Например: гипертония, диабет, проблемы с печенью и т.д.\n" +
			"Если нет - напишите **Нет**",
		ParseMode: "Markdown",
	})
}

// HandleChronicDiseases - обрабатывает хронические заболевания
func (c *UserDataCollector) HandleChronicDiseases(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "chronic_diseases", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingAllergies)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text: "5️⃣ **Есть ли у вас аллергии?**\n" +
			"На лекарства, продукты или что-то ещё.\n" +
			"Если нет - напишите **Нет**",
		ParseMode: "Markdown",
	})
}

// HandleAllergies - обрабатывает аллергии
func (c *UserDataCollector) HandleAllergies(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "allergies", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingMedications)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text: "6️⃣ **Принимаете ли вы какие-либо лекарства постоянно?**\n" +
			"Если да - напишите какие.\n" +
			"Если нет - напишите **Нет**",
		ParseMode: "Markdown",
	})
}

// HandleMedications - обрабатывает лекарства
func (c *UserDataCollector) HandleMedications(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "medications", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingSmoking)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text: "7️⃣ **Курите ли вы?**\n" +
			"Ответьте: Да / Нет\n" +
			"Если Да - укажите сколько лет и сколько в день.",
		ParseMode: "Markdown",
	})
}

// HandleSmoking - обрабатывает курение
func (c *UserDataCollector) HandleSmoking(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "smoking", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingAlcohol)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text: "8️⃣ **Как часто вы употребляете алкоголь?**\n" +
			"Например: не употребляю, редко, раз в неделю, каждый день",
		ParseMode: "Markdown",
	})
}

// HandleAlcohol - обрабатывает алкоголь
func (c *UserDataCollector) HandleAlcohol(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "alcohol", strings.TrimSpace(text))

	// Проверяем, спортсмен ли пользователь
	onCourse := c.stateManager.GetUserData(chatID, "on_course")

	if onCourse == "yes" {
		// Если спортсмен - спрашиваем про вид спорта
		c.stateManager.SetState(chatID, states.StateWaitingSportType)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text: "9️⃣ **Каким видом спорта вы занимаетесь?**\n" +
				"Например: бодибилдинг, кроссфит, тяжелая атлетика и т.д.",
			ParseMode: "Markdown",
		})
		return
	}

	// Если не спортсмен - завершаем сбор
	c.finishCollection(ctx, b, chatID)
}

// HandleSportType - обрабатывает вид спорта
func (c *UserDataCollector) HandleSportType(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "sport_type", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingTrainingExperience)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text: "🔟 **Какой у вас стаж тренировок?**\n" +
			"Напишите количество лет (например: 5)",
		ParseMode: "Markdown",
	})
}

// HandleTrainingExperience - обрабатывает стаж тренировок
func (c *UserDataCollector) HandleTrainingExperience(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	exp, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || exp < 0 || exp > 80 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Пожалуйста, укажите корректный стаж тренировок (от 0 до 80 лет).\n\nПопробуйте ещё раз:",
		})
		return
	}

	c.stateManager.SetUserData(chatID, "training_experience", fmt.Sprintf("%d", exp))
	c.stateManager.SetState(chatID, states.StateWaitingGoal)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text: "1️⃣1️⃣ **Какова ваша цель?**\n" +
			"Например: набор мышечной массы, сушка, поддержка формы, улучшение силы",
		ParseMode: "Markdown",
	})
}

// HandleGoal - обрабатывает цель
func (c *UserDataCollector) HandleGoal(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "goal", strings.TrimSpace(text))
	c.finishCollection(ctx, b, chatID)
}

// finishCollection - завершает сбор данных
func (c *UserDataCollector) finishCollection(ctx context.Context, b *tgbot.Bot, chatID int64) {
	c.stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

	// Показываем собранные данные
	userData := c.stateManager.GetAllUserData(chatID)
	summary := formatUserData(userData)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text: "✅ **Данные сохранены!**\n\n" + summary + "\n\n" +
			"📄 Теперь отправьте PDF-файл или фотографию анализов.\n\n" +
			"Я учту все ваши данные при расшифровке!",
		ParseMode: "Markdown",
	})
}

// formatUserData - форматирует данные пользователя
func formatUserData(data map[string]string) string {
	var parts []string
	parts = append(parts, "📋 **Ваши данные:**")
	parts = append(parts, "")

	if age := data["age"]; age != "" {
		parts = append(parts, fmt.Sprintf("• **Возраст:** %s лет", age))
	}
	if height := data["height"]; height != "" {
		parts = append(parts, fmt.Sprintf("• **Рост:** %s см", height))
	}
	if weight := data["weight"]; weight != "" {
		parts = append(parts, fmt.Sprintf("• **Вес:** %s кг", weight))

		// Рассчитываем ИМТ если есть рост и вес
		if height := data["height"]; height != "" {
			if weight := data["weight"]; weight != "" {
				h, _ := strconv.ParseFloat(height, 64)
				w, _ := strconv.ParseFloat(weight, 64)
				if h > 0 && w > 0 {
					bmi := w / ((h / 100) * (h / 100))
					parts = append(parts, fmt.Sprintf("• **ИМТ:** %.1f", bmi))
				}
			}
		}
	}
	if chronic := data["chronic_diseases"]; chronic != "" && strings.ToLower(chronic) != "нет" {
		parts = append(parts, fmt.Sprintf("• **Хронические заболевания:** %s", chronic))
	}
	if allergies := data["allergies"]; allergies != "" && strings.ToLower(allergies) != "нет" {
		parts = append(parts, fmt.Sprintf("• **Аллергии:** %s", allergies))
	}
	if medications := data["medications"]; medications != "" && strings.ToLower(medications) != "нет" {
		parts = append(parts, fmt.Sprintf("• **Лекарства:** %s", medications))
	}
	if smoking := data["smoking"]; smoking != "" && strings.ToLower(smoking) != "нет" {
		parts = append(parts, fmt.Sprintf("• **Курение:** %s", smoking))
	}
	if alcohol := data["alcohol"]; alcohol != "" {
		parts = append(parts, fmt.Sprintf("• **Алкоголь:** %s", alcohol))
	}
	if sport := data["sport_type"]; sport != "" {
		parts = append(parts, fmt.Sprintf("• **Вид спорта:** %s", sport))
	}
	if exp := data["training_experience"]; exp != "" {
		parts = append(parts, fmt.Sprintf("• **Стаж тренировок:** %s лет", exp))
	}
	if goal := data["goal"]; goal != "" {
		parts = append(parts, fmt.Sprintf("• **Цель:** %s", goal))
	}
	if course := data["course_info"]; course != "" {
		parts = append(parts, fmt.Sprintf("• **Курс:** %s", course))
	}

	return strings.Join(parts, "\n")
}
