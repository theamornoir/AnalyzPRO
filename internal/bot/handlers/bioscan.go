package handlers

import (
	"bytes"
	"context"
	"log"
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

		// Отправляем запрос в Gemini (без данных пользователя)
		result, err := analysisService.HandleBioscan(ctx, fileData, mimeType, "")
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
		// ЛОГИРУЕМ РЕЗУЛЬТАТ
		// ==========================================
		log.Printf("📊 Получен результат от Gemini: %d символов", len(result))
		if len(result) > 100 {
			log.Printf("📊 Первые 100 символов: %q", result[:100])
		} else {
			log.Printf("📊 Полный текст: %q", result)
		}

		// ==========================================
		// СОЗДАЁМ HTML ОТЧЁТ (без лишнего сообщения)
		// ==========================================

		// Отправляем сообщение о создании отчёта
		creatingMsg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "📄 Создаю отчёт...",
		})

		// Создаём HTML отчёт
		htmlData, err := GenerateSimpleBioscanPDF(result)
		if err != nil {
			log.Printf("❌ Ошибка создания HTML: %v", err)
			// Удаляем сообщение "Создаю отчёт..."
			deleteMessage(ctx, b, chatID, creatingMsg.ID)
			// Отправляем текст как fallback
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:    chatID,
				Text:      result,
				ParseMode: "Markdown",
			})
		} else {
			// Удаляем сообщение "Создаю отчёт..."
			deleteMessage(ctx, b, chatID, creatingMsg.ID)

			// Отправляем HTML файл с названием "Bioscan_отчет.html"
			_, _ = b.SendDocument(ctx, &tgbot.SendDocumentParams{
				ChatID: chatID,
				Document: &models.InputFileUpload{
					Filename: "Bioscan_отчет.html",
					Data:     bytes.NewReader(htmlData),
				},
				Caption: "📄 **Ваш Bioscan-отчёт**\n\n" +
					"🔍 Отчёт содержит детальный анализ фигуры и рекомендации.\n\n",
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

		// Ждём между обновлениями
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
