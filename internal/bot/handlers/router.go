package handlers

import (
	"context"
	"fmt"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/service"
)

func MessageRouter(
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	uploadDir string,
	stickerID string,
	adminChatID int64,
) func(context.Context, *tgbot.Bot, *models.Update) {

	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		chatID := update.Message.Chat.ID
		text := strings.TrimSpace(update.Message.Text)

		// ВРЕМЕННЫЙ ОБРАБОТЧИК СТИКЕРОВ
		if update.Message.Sticker != nil {
			stickerFileID := update.Message.Sticker.FileID
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:    chatID,
				Text:      fmt.Sprintf("✅ File ID стикера:\n\n`%s`", stickerFileID),
				ParseMode: "Markdown",
			})
			return
		}

		// ==========================================
		// ПРОВЕРКА: Обработка отзывов (ДО ВСЕГО!)
		// ==========================================

		// Если пользователь отправил текст и мы ожидаем отзыв
		if stateManager.GetUserData(chatID, "waiting_feedback") == "yes" {
			// Очищаем флаг ожидания
			stateManager.SetUserData(chatID, "waiting_feedback", "")

			// Обрабатываем отзыв
			FeedbackHandler(adminChatID)(ctx, b, update)
			return
		}

		state := stateManager.GetState(chatID)

		// ==========================================
		// ОБРАБОТКА СОСТОЯНИЙ
		// ==========================================

		if state == states.StateWaitingPhotoConfirm {
			handlePhotoConfirm(ctx, b, stateManager, analysisService, uploadDir, stickerID, chatID, text)
			return
		}

		if state == states.StateWaitingCourseInfo {
			handleCourseInfo(ctx, b, stateManager, chatID, text)
			return
		}

		if state == states.StateWaitingCourseTime {
			handleCourseTime(ctx, b, stateManager, chatID, text)
			return
		}

		if state == states.StateWaitingAnalysisFile {
			UploadHandler(stateManager, analysisService, uploadDir, stickerID)(ctx, b, update)
			return
		}

		// ==========================================
		// ОБРАБОТКА КНОПОК МЕНЮ
		// ==========================================

		if text == "" {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "Пожалуйста, отправьте текст или загрузите анализ.",
			})
			return
		}

		switch text {
		case "📤 Загрузить анализ":
			stateManager.SetState(chatID, states.StateWaitingCourseInfo)
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "🧬 Вы сейчас на курсе (спортивная фармакология / ПКТ)?\n\nОтветьте: Да / Нет",
			})
			return

		case "📊 История":
			HistoryHandler()(ctx, b, update)
			return

		case "💎 Premium":
			PremiumHandler()(ctx, b, update)
			return

		case "ℹ️ О сервисе":
			AboutHandler()(ctx, b, update)
			return

		case "📝 Отзывы и предложения":
			// Устанавливаем флаг, что ждем отзыв
			stateManager.SetUserData(chatID, "waiting_feedback", "yes")
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text: "✍️ Напишите ваш отзыв или предложение.\n\n" +
					"Вы можете поделиться:\n" +
					"• Мнением о работе бота\n" +
					"• Предложением по улучшению\n" +
					"• Сообщить о проблеме\n" +
					"• Задать вопрос\n\n" +
					"Я передам ваше сообщение разработчику.",
			})
			return
		}

		// Если обычный текст (не кнопка) - отправляем в AI
		result, err := analysisService.HandleAnalysis(ctx, text)
		if err != nil {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "⚠️ Не удалось обработать анализ. Попробуйте позже.",
			})
			return
		}

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   result,
		})
	}
}

// handlePhotoConfirm - обрабатывает подтверждение "Это анализы?"
func handlePhotoConfirm(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	uploadDir string,
	stickerID string,
	chatID int64,
	text string,
) {
	text = strings.ToLower(text)

	pendingPhotoID := stateManager.GetUserData(chatID, "pending_photo_id")

	if text == "да" || text == "да." || text == "ага" || text == "yes" || text == "д" {
		stateManager.SetUserData(chatID, "pending_photo_id", "")

		fileData, mimeType, err := downloadFileByID(ctx, b, pendingPhotoID, uploadDir)
		if err != nil {
			sendError(ctx, b, chatID)
			stateManager.Reset(chatID)
			return
		}

		if !isPhotoLikelyAnalysis(fileData) {
			stateManager.SetUserData(chatID, "pending_photo_id", "")
			stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "📸 Я проверил фото, но оно НЕ похоже на медицинские анализы.\n\nПожалуйста, отправьте:\n• 📄 PDF-файл с анализами\n• 📸 Фотографию анализов (с таблицей и цифрами)\n• 📝 Текст с показателями\n\nЯ не могу обработать это фото как медицинский анализ.",
			})
			return
		}

		userData := stateManager.GetAllUserData(chatID)
		onCourse := userData["on_course"]

		if onCourse == "" {
			stateManager.SetState(chatID, states.StateWaitingCourseInfo)
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "🧬 Вы сейчас на курсе (спортивная фармакология / ПКТ)?\n\nОтветьте: Да / Нет",
			})
			return
		}

		loadingMsg, textMsg := sendLoadingMessages(ctx, b, chatID, stickerID)

		analysisText := buildAnalysisText(userData)

		result, err := analysisService.HandleAnalysisFromFileWithContext(ctx, fileData, mimeType, analysisText)
		if err != nil {
			deleteMessage(ctx, b, chatID, loadingMsg.ID)
			deleteMessage(ctx, b, chatID, textMsg.ID)
			sendError(ctx, b, chatID)
			stateManager.Reset(chatID)
			return
		}

		deleteMessage(ctx, b, chatID, loadingMsg.ID)
		deleteMessage(ctx, b, chatID, textMsg.ID)

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   result,
		})

		stateManager.Reset(chatID)
		return

	} else if text == "нет" || text == "нет." || text == "неа" || text == "no" || text == "н" {
		stateManager.SetUserData(chatID, "pending_photo_id", "")
		stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "📄 Понял! Тогда, пожалуйста, отправьте:\n\n• 📄 PDF-файл с анализами\n• 📸 Фотографию анализов\n• 📝 Текст с показателями\n\nЯ помогу вам с расшифровкой!",
		})
		return
	} else {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Пожалуйста, ответьте 'Да' или 'Нет'.\n\nЭто медицинские анализы?",
		})
	}
}

// handleCourseInfo - обрабатывает ответ про курс
func handleCourseInfo(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	chatID int64,
	text string,
) {
	text = strings.ToLower(text)

	if text == "да" || text == "да." || text == "ага" || text == "yes" || text == "д" {
		stateManager.SetUserData(chatID, "on_course", "yes")
		stateManager.SetState(chatID, states.StateWaitingCourseTime)

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "💉 На каком вы курсе и сколько по времени?\n\nНапример: \n- Туринабол, 6 неделя\n- ПКТ (Кломид + Тамоксифен), 2 неделя\n- Тестостерон + Примоболан, месяц",
		})
		return
	}

	if text == "нет" || text == "нет." || text == "неа" || text == "no" || text == "н" {
		stateManager.SetUserData(chatID, "on_course", "no")
		stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "📄 Понял! Тогда просто отправьте PDF-файл или фотографию анализов.\n\nЯ сделаю расшифровку как для обычного пациента.",
		})
		return
	}

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "Пожалуйста, ответьте 'Да' или 'Нет'.\n\nВы сейчас на курсе (спортивная фармакология / ПКТ)?",
	})
}

// handleCourseTime - обрабатывает ответ про время курса
func handleCourseTime(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	chatID int64,
	text string,
) {
	if strings.TrimSpace(text) == "" {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Пожалуйста, напишите на каком вы курсе и сколько по времени.\n\nНапример: Туринабол, 6 неделя",
		})
		return
	}

	stateManager.SetUserData(chatID, "course_info", text)
	stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "📄 Спасибо! Теперь отправьте PDF-файл или фотографию анализов.\n\nЯ учту вашу информацию о курсе при расшифровке.",
	})
}
