package handlers

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/service"
)

// StartBioscanFlow - начало опроса для Bioscan
func StartBioscanFlow(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64) {
	// Проверяем соглашение
	if stateManager.GetUserData(chatID, "agreement_accepted") != "yes" {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "📝 Пожалуйста, сначала примите пользовательское соглашение.",
			ReplyMarkup: keyboards.StartMenu(),
		})
		return
	}

	// Сбрасываем предыдущие данные Bioscan
	stateManager.SetUserData(chatID, "bioscan_photo_count", "0")
	stateManager.SetUserData(chatID, "bioscan_name", "")
	stateManager.SetUserData(chatID, "bioscan_age", "")
	stateManager.SetUserData(chatID, "bioscan_height", "")
	stateManager.SetUserData(chatID, "bioscan_weight", "")
	stateManager.SetUserData(chatID, "bioscan_goal", "")
	stateManager.SetUserData(chatID, "bioscan_photo1", "")
	stateManager.SetUserData(chatID, "bioscan_photo2", "")
	stateManager.SetUserData(chatID, "bioscan_photo3", "")
	stateManager.SetUserData(chatID, "bioscan_photo4", "")

	// Начинаем с вопроса об имени
	stateManager.SetState(chatID, states.StateWaitingBioscanName)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text: "📸 **Bioscan - комплексный анализ тела**\n\n" +
			"Я проведу детальный анализ вашей фигуры и дам персональные рекомендации.\n\n" +
			"📋 **Шаг 1 из 6: Введите ваше имя**",
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackMenu(), // Добавляем кнопку "Назад"
	})
}

// HandleBioscanName - обработка имени
func HandleBioscanName(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64, text string) {
	if text == "" || text == "⬅️ Назад" {
		return
	}

	stateManager.SetUserData(chatID, "bioscan_name", text)
	stateManager.SetState(chatID, states.StateWaitingBioscanAge)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text: fmt.Sprintf("👋 Приятно познакомиться, %s!\n\n"+
			"📋 **Шаг 2 из 6: Укажите ваш возраст**\n\n"+
			"Введите число от 10 до 120:", text),
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackMenu(), // Добавляем кнопку "Назад"
	})
}

// HandleBioscanAge - обработка возраста
func HandleBioscanAge(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64, text string) {
	if text == "" || text == "⬅️ Назад" {
		return
	}

	// Проверяем, что введено число
	var age int
	_, err := fmt.Sscanf(text, "%d", &age)
	if err != nil || age < 10 || age > 120 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "❌ Пожалуйста, введите корректный возраст (от 10 до 120 лет).",
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	stateManager.SetUserData(chatID, "bioscan_age", text)
	stateManager.SetState(chatID, states.StateWaitingBioscanHeight)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text: "📋 **Шаг 3 из 6: Укажите ваш рост**\n\n" +
			"Введите рост в сантиметрах (например: 175):",
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackMenu(), // Добавляем кнопку "Назад"
	})
}

// HandleBioscanHeight - обработка роста
func HandleBioscanHeight(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64, text string) {
	if text == "" || text == "⬅️ Назад" {
		return
	}

	var height int
	_, err := fmt.Sscanf(text, "%d", &height)
	if err != nil || height < 50 || height > 300 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "❌ Пожалуйста, введите корректный рост (от 50 до 300 см).",
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	stateManager.SetUserData(chatID, "bioscan_height", text)
	stateManager.SetState(chatID, states.StateWaitingBioscanWeight)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text: "📋 **Шаг 4 из 6: Укажите ваш вес**\n\n" +
			"Введите вес в килограммах (например: 70):",
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackMenu(), // Добавляем кнопку "Назад"
	})
}

// HandleBioscanWeight - обработка веса
func HandleBioscanWeight(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64, text string) {
	if text == "" || text == "⬅️ Назад" {
		return
	}

	var weight int
	_, err := fmt.Sscanf(text, "%d", &weight)
	if err != nil || weight < 20 || weight > 500 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "❌ Пожалуйста, введите корректный вес (от 20 до 500 кг).",
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	stateManager.SetUserData(chatID, "bioscan_weight", text)
	stateManager.SetState(chatID, states.StateWaitingBioscanGoal)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text: "📋 **Шаг 5 из 6: Какова ваша цель?**\n\n" +
			"Выберите или напишите свою цель:\n\n" +
			"💪 **Набор мышечной массы**\n" +
			"🔥 **Снижение веса**\n" +
			"⚖️ **Поддержание формы**\n" +
			"🏃 **Улучшение выносливости**\n" +
			"🧘 **Улучшение гибкости**\n\n" +
			"Или напишите свою цель:",
		ParseMode: "Markdown",
		ReplyMarkup: &models.ReplyKeyboardMarkup{
			Keyboard: [][]models.KeyboardButton{
				{
					{Text: "💪 Набор мышечной массы"},
					{Text: "🔥 Снижение веса"},
				},
				{
					{Text: "⚖️ Поддержание формы"},
					{Text: "🏃 Улучшение выносливости"},
				},
				{
					{Text: "🧘 Улучшение гибкости"},
				},
				{
					{Text: "⬅️ Назад"},
				},
			},
			ResizeKeyboard: true,
		},
	})
}

// HandleBioscanGoal - обработка цели
func HandleBioscanGoal(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64, text string) {
	if text == "" || text == "⬅️ Назад" {
		return
	}

	// Очищаем текст от эмодзи
	goal := strings.TrimSpace(text)
	goal = strings.ReplaceAll(goal, "💪 ", "")
	goal = strings.ReplaceAll(goal, "🔥 ", "")
	goal = strings.ReplaceAll(goal, "⚖️ ", "")
	goal = strings.ReplaceAll(goal, "🏃 ", "")
	goal = strings.ReplaceAll(goal, "🧘 ", "")

	stateManager.SetUserData(chatID, "bioscan_goal", goal)
	stateManager.SetState(chatID, states.StateWaitingBioscanPhoto1)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text: "📸 **Шаг 6 из 6: Отправьте 4 фотографии**\n\n" +
			"Для комплексного анализа мне нужно получить 4 фотографии:\n\n" +
			"📷 **Фото 1/4 - Анфас**\n" +
			"• Стойте прямо, руки вдоль тела\n" +
			"• В обтягивающей одежде или без неё\n" +
			"• Хорошее освещение\n" +
			"• Фото в полный рост\n\n" +
			"📤 **Отправьте первое фото:**",
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackMenu(),
	})
}

// HandleBioscanPhoto - обработка фотографий
func HandleBioscanPhoto(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64, photos []models.PhotoSize) {
	// Получаем текущее состояние
	state := stateManager.GetState(chatID)

	// Берем фото с максимальным разрешением
	photo := photos[len(photos)-1]

	// Обрабатываем в зависимости от состояния
	switch state {
	case states.StateWaitingBioscanPhoto1:
		stateManager.SetUserData(chatID, "bioscan_photo1", photo.FileID)
		stateManager.SetState(chatID, states.StateWaitingBioscanPhoto2)
		stateManager.SetUserData(chatID, "bioscan_photo_count", "1")

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text: "✅ **Фото 1/4 получено!**\n\n" +
				"📷 **Фото 2/4 - Профиль**\n" +
				"• Повернитесь боком\n" +
				"• Стойте прямо\n" +
				"• Руки вдоль тела\n\n" +
				"📤 **Отправьте второе фото:**",
			ParseMode:   "Markdown",
			ReplyMarkup: keyboards.BackMenu(),
		})

	case states.StateWaitingBioscanPhoto2:
		stateManager.SetUserData(chatID, "bioscan_photo2", photo.FileID)
		stateManager.SetState(chatID, states.StateWaitingBioscanPhoto3)
		stateManager.SetUserData(chatID, "bioscan_photo_count", "2")

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text: "✅ **Фото 2/4 получено!**\n\n" +
				"📷 **Фото 3/4 - Сзади**\n" +
				"• Повернитесь спиной\n" +
				"• Стойте прямо\n" +
				"• Руки вдоль тела\n\n" +
				"📤 **Отправьте третье фото:**",
			ParseMode:   "Markdown",
			ReplyMarkup: keyboards.BackMenu(),
		})

	case states.StateWaitingBioscanPhoto3:
		stateManager.SetUserData(chatID, "bioscan_photo3", photo.FileID)
		stateManager.SetState(chatID, states.StateWaitingBioscanPhoto4)
		stateManager.SetUserData(chatID, "bioscan_photo_count", "3")

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text: "✅ **Фото 3/4 получено!**\n\n" +
				"📷 **Фото 4/4 - В движении**\n" +
				"• Сделайте фото во время выполнения упражнения\n" +
				"• Или в естественной позе\n\n" +
				"📤 **Отправьте четвёртое фото:**",
			ParseMode:   "Markdown",
			ReplyMarkup: keyboards.BackMenu(),
		})

	case states.StateWaitingBioscanPhoto4:
		stateManager.SetUserData(chatID, "bioscan_photo4", photo.FileID)
		stateManager.SetState(chatID, states.StateWaitingBioscanConfirm)
		stateManager.SetUserData(chatID, "bioscan_photo_count", "4")

		// Показываем собранные данные для подтверждения
		name := stateManager.GetUserData(chatID, "bioscan_name")
		age := stateManager.GetUserData(chatID, "bioscan_age")
		height := stateManager.GetUserData(chatID, "bioscan_height")
		weight := stateManager.GetUserData(chatID, "bioscan_weight")
		goal := stateManager.GetUserData(chatID, "bioscan_goal")

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text: fmt.Sprintf("✅ **Все 4 фото получены!**\n\n"+
				"📋 **Проверьте данные:**\n"+
				"👤 Имя: %s\n"+
				"📅 Возраст: %s лет\n"+
				"📏 Рост: %s см\n"+
				"⚖️ Вес: %s кг\n"+
				"🎯 Цель: %s\n\n"+
				"✅ Всё верно? Нажмите 'Подтвердить' для начала анализа.\n\n"+
				"⏳ Анализ займёт 30-60 секунд.",
				name, age, height, weight, goal),
			ParseMode: "Markdown",
			ReplyMarkup: &models.ReplyKeyboardMarkup{
				Keyboard: [][]models.KeyboardButton{
					{
						{Text: "✅ Подтвердить и проанализировать"},
					},
					{
						{Text: "🔄 Начать заново"},
						{Text: "⬅️ Назад"},
					},
				},
				ResizeKeyboard: true,
			},
		})

	default:
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "❌ Что-то пошло не так. Пожалуйста, начните заново с кнопки '📸 Bioscan'.",
			ReplyMarkup: keyboards.BackMenu(),
		})
	}
}

// ProcessBioscanWithPhotos - обработка подтверждения и отправка в Gemini
func ProcessBioscanWithPhotos(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	uploadDir string,
	stickerID string,
	chatID int64,
) {
	// Собираем все данные
	name := stateManager.GetUserData(chatID, "bioscan_name")
	age := stateManager.GetUserData(chatID, "bioscan_age")
	height := stateManager.GetUserData(chatID, "bioscan_height")
	weight := stateManager.GetUserData(chatID, "bioscan_weight")
	goal := stateManager.GetUserData(chatID, "bioscan_goal")
	photo1ID := stateManager.GetUserData(chatID, "bioscan_photo1")
	photo2ID := stateManager.GetUserData(chatID, "bioscan_photo2")
	photo3ID := stateManager.GetUserData(chatID, "bioscan_photo3")
	photo4ID := stateManager.GetUserData(chatID, "bioscan_photo4")

	// Проверяем, что все данные есть
	if name == "" || age == "" || height == "" || weight == "" || goal == "" ||
		photo1ID == "" || photo2ID == "" || photo3ID == "" || photo4ID == "" {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "❌ Не все данные собраны. Пожалуйста, начните заново.",
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	// Формируем контекст с данными пользователя
	contextInfo := fmt.Sprintf(
		"Данные пользователя:\nИмя: %s\nВозраст: %s лет\nРост: %s см\nВес: %s кг\nЦель: %s",
		name, age, height, weight, goal,
	)

	// Отправляем сообщение о начале анализа
	loadingMsg, textMsg := sendLoadingMessages(ctx, b, chatID, stickerID)

	// Запускаем анимацию статуса
	go animateBioscanStatus(ctx, b, chatID, textMsg.ID)

	// Скачиваем фото 1 (анфас) для основного анализа
	photoData, _, err := downloadFileByID(ctx, b, photo1ID, uploadDir)
	if err != nil {
		deleteMessage(ctx, b, chatID, loadingMsg.ID)
		deleteMessage(ctx, b, chatID, textMsg.ID)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "❌ Не удалось загрузить фотографию. Попробуйте еще раз.",
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	// Выполняем анализ через существующий HandleBioscan
	htmlReport, err := analysisService.HandleBioscan(
		ctx,
		photoData,
		"image/jpeg",
		contextInfo,
	)

	if err != nil {
		deleteMessage(ctx, b, chatID, loadingMsg.ID)
		deleteMessage(ctx, b, chatID, textMsg.ID)
		log.Printf("Bioscan error: %v", err)

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        fmt.Sprintf("⚠️ Не удалось обработать фото. Ошибка: %v", err),
			ReplyMarkup: keyboards.BackMenu(),
		})
		return
	}

	// Удаляем сообщения о загрузке
	deleteMessage(ctx, b, chatID, loadingMsg.ID)
	deleteMessage(ctx, b, chatID, textMsg.ID)

	// Отправляем готовый HTML
	_, err = b.SendDocument(
		ctx,
		&tgbot.SendDocumentParams{
			ChatID: chatID,
			Document: &models.InputFileUpload{
				Filename: "Bioscan_report.html",
				Data:     bytes.NewReader([]byte(htmlReport)),
			},
			Caption: "📄 **Ваш Bioscan отчёт**\n\n" +
				"👤 " + name + ", " + age + " лет\n" +
				"📏 " + height + " см, ⚖️ " + weight + " кг\n" +
				"🎯 Цель: " + goal + "\n\n" +
				"💪 Анализ тела\n" +
				"📊 Оценка зон\n" +
				"🏋️ Рекомендации",
			ParseMode: "Markdown",
		},
	)

	if err != nil {
		log.Printf("Send document error: %v", err)
	}

	// Сбрасываем состояние
	stateManager.SetState(chatID, states.StateIdle)

	// Показываем главное меню
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "✅ **Анализ завершён!**\n\nХотите сделать новый Bioscan? Отправьте новое фото или выберите действие в меню.",
		ReplyMarkup: keyboards.MainMenu(),
		ParseMode:   "Markdown",
	})
}

// animateBioscanStatus - анимация статуса обработки
func animateBioscanStatus(ctx context.Context, b *tgbot.Bot, chatID int64, messageID int) {
	statuses := []string{
		"🔍 Анализирую пропорции тела...",
		"💪 Проверяю мышечный баланс...",
		"🦴 Анализирую осанку...",
		"📊 Оцениваю композицию тела...",
		"🧬 Формирую профиль развития...",
		"📝 Создаю рекомендации...",
	}

	for _, status := range statuses {
		select {
		case <-ctx.Done():
			return
		default:
		}

		time.Sleep(2 * time.Second)

		_, _ = b.EditMessageText(ctx, &tgbot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text:      status + "\n\n⏳ Подождите...",
		})
	}
}
