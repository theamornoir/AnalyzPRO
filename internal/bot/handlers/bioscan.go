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

// BioscanHandler - обработчик для Bioscan
func BioscanHandler(
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	uploadDir string,
	stickerID string,
) func(context.Context, *tgbot.Bot, *models.Update) {

	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		chatID := update.Message.Chat.ID

		// Проверяем, что это фото
		if update.Message.Photo == nil {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text: "📸 **Bioscan - анализ фигуры**\n\n" +
					"Пожалуйста, отправьте **фото** вашего тела для анализа.\n\n" +
					"📌 **Рекомендации для лучшего результата:**\n" +
					"• 📸 Фото в **полный рост** (анфас)\n" +
					"• 💡 Хорошее **освещение**\n" +
					"• 👕 В **обтягивающей одежде** или без неё\n" +
					"• 🧍 Стоять **прямо**, руки вдоль тела\n" +
					"• 📱 Фото должно быть **чётким**, не размытым",
				ReplyMarkup: keyboards.BackMenu(),
				ParseMode:   "Markdown",
			})
			return
		}

		// Проверяем, что пользователь согласился на обработку
		agreementAccepted := stateManager.GetUserData(chatID, "agreement_accepted")
		if agreementAccepted != "yes" {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        "📝 Пожалуйста, сначала примите пользовательское соглашение.",
				ReplyMarkup: keyboards.StartMenu(),
			})
			return
		}

		// ==========================================
		// ОТПРАВЛЯЕМ СТИКЕР И АНИМИРОВАННЫЙ СТАТУС
		// ==========================================
		loadingMsg, statusMsg := sendLoadingMessages(ctx, b, chatID, stickerID)

		// Анимируем статус
		go animateBioscanStatus(ctx, b, chatID, statusMsg.ID)

		// Скачиваем фото
		photo := update.Message.Photo[len(update.Message.Photo)-1]
		fileData, mimeType, err := downloadFileByID(ctx, b, photo.FileID, uploadDir)
		if err != nil {
			deleteMessage(ctx, b, chatID, loadingMsg.ID)
			deleteMessage(ctx, b, chatID, statusMsg.ID)
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        "❌ Не удалось загрузить фото. Попробуйте ещё раз.",
				ReplyMarkup: keyboards.BackMenu(),
			})
			return
		}

		// Получаем данные пользователя
		userData := stateManager.GetAllUserData(chatID)

		userName := userData["name"]
		if userName == "" {
			userName = "Пользователь"
		}

		age := userData["age"]
		if age == "" {
			age = "—"
		}

		gender := userData["gender"]
		if gender == "" {
			gender = "—"
		}

		height := userData["height"]
		if height == "" {
			height = "—"
		}

		weight := userData["weight"]
		if weight == "" {
			weight = "—"
		}

		sportType := userData["sport_type"]
		goal := userData["goal"]

		userContext := buildBioscanContext(userData)

		// Отправляем запрос в Gemini
		result, err := analysisService.HandleBioscan(ctx, fileData, mimeType, userContext)
		if err != nil {
			deleteMessage(ctx, b, chatID, loadingMsg.ID)
			deleteMessage(ctx, b, chatID, statusMsg.ID)
			log.Printf("❌ Bioscan error: %v", err)
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        "⚠️ **Не удалось обработать фото.**\n\nПопробуйте отправить другое фото с лучшим качеством.",
				ReplyMarkup: keyboards.BackMenu(),
				ParseMode:   "Markdown",
			})
			return
		}

		// Удаляем стикер и статус
		deleteMessage(ctx, b, chatID, loadingMsg.ID)
		deleteMessage(ctx, b, chatID, statusMsg.ID)

		// ==========================================
		// ОТПРАВЛЯЕМ РЕЗУЛЬТАТ В PDF
		// ==========================================

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "📄 Создаю профессиональный PDF-отчёт...\n\n⏳ Это может занять несколько секунд.",
		})

		pdfData, err := GenerateBioscanPDF(result, userName, age, gender, height, weight, sportType, goal)
		if err != nil {
			log.Printf("❌ PDF generation error: %v", err)
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:    chatID,
				Text:      result,
				ParseMode: "Markdown",
			})
		} else {
			_, _ = b.SendDocument(ctx, &tgbot.SendDocumentParams{
				ChatID: chatID,
				Document: &models.InputFileUpload{
					Filename: fmt.Sprintf("Bioscan_%s.pdf", time.Now().Format("2006-01-02_15-04")),
					Data:     bytes.NewReader(pdfData),
				},
				Caption: "📸 **Ваш профессиональный Bioscan-анализ**\n\n" +
					"📄 **Отчёт содержит:**\n" +
					"• 📊 Оценку телосложения\n" +
					"• 💪 Детальный разбор каждой мышцы\n" +
					"• 🦴 Анализ осанки\n" +
					"• ✅ Персональные рекомендации по тренировкам\n\n" +
					"📅 **Сохраните этот файл** для отслеживания прогресса!\n" +
					"🔄 Рекомендуется повторить анализ через 4-6 недель.",
				ParseMode: "Markdown",
			})
		}

		// Возвращаемся в меню
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "📸 **Хотите сделать ещё один Bioscan?**\n\nОтправьте новое фото или нажмите ⬅️ Назад.",
			ReplyMarkup: keyboards.BackMenu(),
			ParseMode:   "Markdown",
		})
	}
}

// animateBioscanStatus - анимирует статус обработки Bioscan
func animateBioscanStatus(ctx context.Context, b *tgbot.Bot, chatID int64, messageID int) {
	statuses := []string{
		"🔍 Анализирую пропорции тела...",
		"💪 Проверяю мышечный баланс...",
		"🧬 Оцениваю развитие мышц...",
		"🦴 Анализирую осанку...",
		"📊 Оцениваю процент жира...",
		"🔬 Детально изучаю каждую зону...",
		"📝 Формирую персональные рекомендации...",
	}

	for i, status := range statuses {
		// Проверяем, не отменён ли контекст
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Ждём между обновлениями (первое обновление через 1.5 сек, остальные через 1-2 сек)
		if i == 0 {
			time.Sleep(1500 * time.Millisecond)
		} else {
			time.Sleep(1800 * time.Millisecond)
		}

		// Обновляем сообщение
		_, _ = b.EditMessageText(ctx, &tgbot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: messageID,
			Text: status + "\n\n" +
				"⏳ Пожалуйста, подождите...\n" +
				"🔄 Процесс анализа может занять до 20 секунд",
		})
	}
}

// buildBioscanContext - формирует контекст для Bioscan
func buildBioscanContext(userData map[string]string) string {
	var parts []string

	name := userData["name"]
	if name == "" {
		name = "Пользователь"
	}
	parts = append(parts, fmt.Sprintf("👤 **Имя:** %s", name))

	if gender := userData["gender"]; gender != "" {
		parts = append(parts, fmt.Sprintf("• **Пол:** %s", gender))
	}
	if age := userData["age"]; age != "" {
		parts = append(parts, fmt.Sprintf("• **Возраст:** %s лет", age))
	}
	if height := userData["height"]; height != "" {
		parts = append(parts, fmt.Sprintf("• **Рост:** %s см", height))
	}
	if weight := userData["weight"]; weight != "" {
		parts = append(parts, fmt.Sprintf("• **Вес:** %s кг", weight))
	}
	if sport := userData["sport_type"]; sport != "" {
		parts = append(parts, fmt.Sprintf("• **Вид спорта:** %s", sport))
	}
	if goal := userData["goal"]; goal != "" {
		parts = append(parts, fmt.Sprintf("• **Цель:** %s", goal))
	}

	return strings.Join(parts, "\n")
}
