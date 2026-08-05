package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
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
	c.stateManager.SetState(chatID, states.StateWaitingName)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "📋 Для более точного анализа мне нужно знать немного о вас.\n\n1️⃣ **Как вас зовут?**\nНапишите ваше имя (например: Ирина)",
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleName - обрабатывает имя
func (c *UserDataCollector) HandleName(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	name := strings.TrimSpace(text)
	if len(name) < 2 || len(name) > 50 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "❌ Пожалуйста, укажите корректное имя (от 2 до 50 символов).\n\nПопробуйте ещё раз:",
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	c.stateManager.SetUserData(chatID, "name", name)
	c.stateManager.SetState(chatID, states.StateWaitingGender)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "2️⃣ **Ваш пол?**\nОтветьте: Мужской / Женский",
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleGender - обрабатывает пол
func (c *UserDataCollector) HandleGender(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	gender := strings.ToLower(strings.TrimSpace(text))

	if gender == "мужской" || gender == "м" || gender == "male" {
		c.stateManager.SetUserData(chatID, "gender", "Мужской")
	} else if gender == "женский" || gender == "ж" || gender == "female" {
		c.stateManager.SetUserData(chatID, "gender", "Женский")
	} else {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "❌ Пожалуйста, укажите ваш пол: **Мужской** или **Женский**.\n\nПопробуйте ещё раз:",
			ReplyMarkup: keyboards.BackMenu(),
			ParseMode:   "Markdown",
		})
		return
	}

	c.stateManager.SetState(chatID, states.StateWaitingAge)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "3️⃣ **Сколько вам лет?**\nНапишите число (например: 25)",
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleAge - обрабатывает возраст
func (c *UserDataCollector) HandleAge(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	age, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || age < 5 || age > 90 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "❌ Пожалуйста, укажите корректный возраст (от 5 до 90 лет).\n\nПопробуйте ещё раз:",
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	c.stateManager.SetUserData(chatID, "age", fmt.Sprintf("%d", age))
	c.stateManager.SetState(chatID, states.StateWaitingHeight)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "4️⃣ **Какой у вас рост?**\nНапишите в сантиметрах (например: 178)",
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleHeight - обрабатывает рост
func (c *UserDataCollector) HandleHeight(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	height, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || height < 50 || height > 210 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "❌ Пожалуйста, укажите корректный рост (от 50 до 210 см).\n\nПопробуйте ещё раз:",
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	c.stateManager.SetUserData(chatID, "height", fmt.Sprintf("%d", height))
	c.stateManager.SetState(chatID, states.StateWaitingWeight)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "5️⃣ **Какой у вас вес?**\nНапишите в килограммах (например: 82)",
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleWeight - обрабатывает вес
func (c *UserDataCollector) HandleWeight(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	weight, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || weight < 30 || weight > 200 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "❌ Пожалуйста, укажите корректный вес (от 30 до 200 кг).\n\nПопробуйте ещё раз:",
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	c.stateManager.SetUserData(chatID, "weight", fmt.Sprintf("%d", weight))
	c.stateManager.SetState(chatID, states.StateWaitingChronicDiseases)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "6️⃣ **Есть ли у вас хронические заболевания?**\nНапример: гипертония, диабет, проблемы с печенью и т.д.\nЕсли нет - напишите **Нет**",
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleChronicDiseases - обрабатывает хронические заболевания
func (c *UserDataCollector) HandleChronicDiseases(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "chronic_diseases", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingAllergies)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "7️⃣ **Есть ли у вас аллергии?**\nНа лекарства, продукты или что-то ещё.\nЕсли нет - напишите **Нет**",
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleAllergies - обрабатывает аллергии
func (c *UserDataCollector) HandleAllergies(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "allergies", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingMedications)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "8️⃣ **Принимаете ли вы какие-либо лекарства постоянно?**\nЕсли да - напишите какие.\nЕсли нет - напишите **Нет**",
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleMedications - обрабатывает лекарства
func (c *UserDataCollector) HandleMedications(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "medications", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingSmoking)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "9️⃣ **Курите ли вы?**\nОтветьте: Да / Нет\nЕсли Да - укажите сколько лет и сколько в день.",
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleSmoking - обрабатывает курение
func (c *UserDataCollector) HandleSmoking(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "smoking", strings.TrimSpace(text))
	c.stateManager.SetState(chatID, states.StateWaitingAlcohol)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "🔟 **Как часто вы употребляете алкоголь?**\nНапример: не употребляю, редко, раз в неделю, каждый день",
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
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
			ChatID:      chatID,
			Text:        "1️⃣1️⃣ **Каким видом спорта вы занимаетесь?**\nНапример: бодибилдинг, кроссфит, тяжелая атлетика и т.д.",
			ReplyMarkup: keyboards.BackMenu(),
			ParseMode:   "Markdown",
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
		ChatID:      chatID,
		Text:        "1️⃣2️⃣ **Какой у вас стаж тренировок?**\nНапишите количество лет (например: 5)",
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleTrainingExperience - обрабатывает стаж тренировок
func (c *UserDataCollector) HandleTrainingExperience(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	exp, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || exp < 0 || exp > 80 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "❌ Пожалуйста, укажите корректный стаж тренировок (от 0 до 80 лет).\n\nПопробуйте ещё раз:",
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	c.stateManager.SetUserData(chatID, "training_experience", fmt.Sprintf("%d", exp))
	c.stateManager.SetState(chatID, states.StateWaitingGoal)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "1️⃣3️⃣ **Какова ваша цель?**\nНапример: набор мышечной массы, сушка, поддержка формы, улучшение силы",
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleGoal - обрабатывает цель
func (c *UserDataCollector) HandleGoal(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	c.stateManager.SetUserData(chatID, "goal", strings.TrimSpace(text))

	// ==========================================
	// ВОПРОС О ПРЕПАРАТАХ
	// ==========================================
	c.stateManager.SetState(chatID, states.StateWaitingCourseInfo)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "1️⃣4️⃣ **Используете ли вы препараты для повышения тестостерона или другие гормональные препараты?**\n\nНапример: стероиды, SARMs, ПКТ, TRT и т.д.\n\nОтветьте: Да / Нет",
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// HandleCourseInfo - обрабатывает ответ про препараты (используется из router.go)
// Но мы добавим его сюда для полноты
func (c *UserDataCollector) HandleCourseInfo(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	text = strings.ToLower(strings.TrimSpace(text))

	if text == "да" || text == "да." || text == "ага" || text == "yes" || text == "д" || text == "+" {
		c.stateManager.SetUserData(chatID, "on_course", "yes")
		c.stateManager.SetState(chatID, states.StateWaitingCourseTime)

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "💉 **Какие препараты вы используете и сколько по времени?**\n\nНапишите подробно, например:\n• Тестостерон энантат, 8 недель, 250 мг/нед\n• Туринабол, 6 недель, 40 мг/день\n• ПКТ (Кломид + Тамоксифен), 4 недели",
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
		Text:        "❌ Пожалуйста, ответьте 'Да' или 'Нет'.\n\nИспользуете ли вы препараты для повышения тестостерона?",
		ReplyMarkup: keyboards.BackMenu(),
	})
}

// HandleCourseTime - обрабатывает ответ про время курса
func (c *UserDataCollector) HandleCourseTime(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	if strings.TrimSpace(text) == "" {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "❌ Пожалуйста, напишите какие препараты вы используете и сколько по времени.\n\nНапример: Тестостерон энантат, 8 недель",
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	c.stateManager.SetUserData(chatID, "course_info", strings.TrimSpace(text))
	c.finishCollection(ctx, b, chatID)
}

// finishCollection - завершает сбор данных
func (c *UserDataCollector) finishCollection(ctx context.Context, b *tgbot.Bot, chatID int64) {
	c.stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

	// Показываем собранные данные
	userData := c.stateManager.GetAllUserData(chatID)
	summary := formatUserData(userData)

	name := userData["name"]
	if name == "" {
		name = "Пользователь"
	}

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        fmt.Sprintf("✅ **%s, ваши данные сохранены!**\n\n%s\n\n📄 Теперь отправьте PDF-файл или фотографию анализов.\n\nЯ учту все ваши данные при расшифровке!", name, summary),
		ReplyMarkup: keyboards.BackMenu(),
		ParseMode:   "Markdown",
	})
}

// formatUserData - форматирует данные пользователя
func formatUserData(data map[string]string) string {
	var parts []string
	parts = append(parts, "📋 **Ваши данные:**")
	parts = append(parts, "")

	if name := data["name"]; name != "" {
		parts = append(parts, fmt.Sprintf("• **Имя:** %s", name))
	}
	if gender := data["gender"]; gender != "" {
		parts = append(parts, fmt.Sprintf("• **Пол:** %s", gender))
	}
	if age := data["age"]; age != "" {
		parts = append(parts, fmt.Sprintf("• **Возраст:** %s лет", age))
	}
	if height := data["height"]; height != "" {
		parts = append(parts, fmt.Sprintf("• **Рост:** %s см", height))
	}
	if weight := data["weight"]; weight != "" {
		parts = append(parts, fmt.Sprintf("• **Вес:** %s кг", weight))

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
		parts = append(parts, fmt.Sprintf("• **Препараты:** %s", course))
	}

	return strings.Join(parts, "\n")
}
