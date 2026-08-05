package handlers

import (
	"context"
	"log"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/service"
)

func MessageRouter(
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
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

		// ==========================================
		// ПРОВЕРКА СОГЛАШЕНИЯ (в самом начале)
		// ==========================================
		agreementAccepted := stateManager.GetUserData(chatID, "agreement_accepted")

		if agreementAccepted != "yes" {
			// Если это кнопка соглашения
			if text == "📝 Пользовательское соглашение" {
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:      chatID,
					Text:        keyboards.UserAgreementText(),
					ReplyMarkup: keyboards.AgreementMenu(),
					ParseMode:   "Markdown",
				})
				return
			}

			// Если это кнопка "Принять соглашение"
			if text == "✅ Принять соглашение" {
				stateManager.SetUserData(chatID, "agreement_accepted", "yes")
				stateManager.SetState(chatID, states.StateIdle)

				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:      chatID,
					Text:        "✅ **Спасибо! Вы приняли пользовательское соглашение.**\n\nТеперь вы можете пользоваться ботом. Выберите тип анализа:",
					ReplyMarkup: keyboards.MainMenu(),
					ParseMode:   "Markdown",
				})
				return
			}

			// Любое другое действие - просим принять соглашение
			if text != "" && text != "/start" {
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: chatID,
					Text: `📝 **Для начала работы примите пользовательское соглашение.**

Нажмите кнопку **📝 Пользовательское соглашение**, чтобы ознакомиться и принять.`,
					ReplyMarkup: keyboards.StartMenu(),
					ParseMode:   "Markdown",
				})
				return
			}
		}

		state := stateManager.GetState(chatID)

		// ==========================================
		// ОБРАБОТКА КНОПКИ "НАЗАД" (ДО ВСЕГО)
		// ==========================================
		if text == "⬅️ Назад" {
			stateManager.SetState(chatID, states.StateIdle)
			stateManager.SetUserData(chatID, "photo_processed", "")
			stateManager.SetUserData(chatID, "waiting_photo_confirm", "")
			stateManager.SetUserData(chatID, "pending_photos", "")
			stateManager.SetUserData(chatID, "question_asked", "")
			stateManager.SetUserData(chatID, "processed_group_id", "")
			stateManager.SetUserData(chatID, "media_group_id", "")
			stateManager.SetUserData(chatID, "analysis_type", "")

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        "🔙 Вы вернулись в главное меню.\n\nВыберите действие:",
				ReplyMarkup: keyboards.MainMenu(),
			})
			return
		}

		// ==========================================
		// ОБРАБОТКА СОСТОЯНИЙ ДЛЯ СБОРА ДАННЫХ
		// ==========================================

		collector := NewUserDataCollector(stateManager)

		if state == states.StateWaitingName {
			collector.HandleName(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingGender {
			collector.HandleGender(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingAge {
			collector.HandleAge(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingHeight {
			collector.HandleHeight(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingWeight {
			collector.HandleWeight(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingChronicDiseases {
			collector.HandleChronicDiseases(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingAllergies {
			collector.HandleAllergies(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingMedications {
			collector.HandleMedications(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingSmoking {
			collector.HandleSmoking(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingAlcohol {
			collector.HandleAlcohol(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingSportType {
			collector.HandleSportType(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingTrainingExperience {
			collector.HandleTrainingExperience(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingGoal {
			collector.HandleGoal(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingCourseInfo {
			collector.HandleCourseInfo(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingCourseTime {
			collector.HandleCourseTime(ctx, b, chatID, text)
			return
		}

		// ==========================================
		// ОБРАБОТКА ОСТАЛЬНЫХ СОСТОЯНИЙ
		// ==========================================

		if state == states.StateWaitingPhotoConfirm {
			handlePhotoConfirm(ctx, b, stateManager, analysisService, uploadDir, stickerID, chatID, text)
			return
		}

		if state == states.StateWaitingAnalysisFile {
			UploadHandler(
				stateManager,
				analysisService,
				reportRenderer,
				uploadDir,
				stickerID,
			)(ctx, b, update)
			return
		}

		// ==========================================
		// ОБРАБОТКА КНОПОК МЕНЮ
		// ==========================================

		if text == "" {
			return
		}

		log.Printf("🔍 Получен текст: %q", text)

		switch text {
		// ==========================================
		// СОГЛАШЕНИЕ (уже обработано выше, но оставляем для безопасности)
		// ==========================================
		case "📝 Пользовательское соглашение":
			log.Printf("📝 Обработка: Пользовательское соглашение")
			if stateManager.GetUserData(chatID, "agreement_accepted") == "yes" {
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:      chatID,
					Text:        "✅ Вы уже приняли пользовательское соглашение.\n\nВыберите действие в главном меню:",
					ReplyMarkup: keyboards.MainMenu(),
				})
				return
			}

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        keyboards.UserAgreementText(),
				ReplyMarkup: keyboards.AgreementMenu(),
				ParseMode:   "Markdown",
			})
			return

		case "✅ Принять соглашение":
			log.Printf("📝 Обработка: Принять соглашение")
			if stateManager.GetUserData(chatID, "agreement_accepted") == "yes" {
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:      chatID,
					Text:        "✅ Вы уже приняли пользовательское соглашение.\n\nВыберите действие в главном меню:",
					ReplyMarkup: keyboards.MainMenu(),
				})
				return
			}

			stateManager.SetUserData(chatID, "agreement_accepted", "yes")
			stateManager.SetState(chatID, states.StateIdle)

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        "✅ **Спасибо! Вы приняли пользовательское соглашение.**\n\nТеперь вы можете пользоваться ботом. Выберите тип анализа:",
				ReplyMarkup: keyboards.MainMenu(),
				ParseMode:   "Markdown",
			})
			return

		case "ℹ️ О сервисе":
			AboutHandler()(ctx, b, update)
			return

		// ==========================================
		// ОСНОВНОЕ МЕНЮ
		// ==========================================
		case "🏥 Обычный анализ":
			stateManager.SetUserData(chatID, "analysis_type", "regular")
			stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        "📄 Отправьте **один** PDF-файл или **одну** фотографию анализов.\n\nПосле обработки вы сможете отправить ещё один файл.\n\n📎 Поддерживаются: PDF, JPG, PNG",
				ReplyMarkup: keyboards.BackMenu(),
			})
			return

		case "🏋️ Анализ для спортсмена":
			stateManager.SetUserData(chatID, "analysis_type", "sportsman")

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        "📋 Сначала ответьте на несколько вопросов, затем вы сможете загрузить файл.\n\n📎 Файлы принимаются по одному: PDF, JPG, PNG",
				ReplyMarkup: keyboards.BackMenu(),
			})

			collector.StartCollection(ctx, b, chatID)
			return

		case "📸 Bioscan":
			if stateManager.GetUserData(chatID, "agreement_accepted") != "yes" {
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:      chatID,
					Text:        "📝 Пожалуйста, сначала примите пользовательское соглашение.",
					ReplyMarkup: keyboards.StartMenu(),
				})
				return
			}

			stateManager.SetUserData(chatID, "analysis_type", "bioscan")
			stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text: "📸 **Bioscan - анализ фигуры**\n\n" +
					"Отправьте **фото** вашего тела для анализа.\n\n" +
					"📌 Рекомендации:\n" +
					"• Фото в полный рост (анфас)\n" +
					"• Хорошее освещение\n" +
					"• В обтягивающей одежде или без неё\n" +
					"• Стоять прямо, руки вдоль тела\n\n" +
					"⏳ Анализ займёт 10-20 секунд.",
				ReplyMarkup: keyboards.BackMenu(),
				ParseMode:   "Markdown",
			})
			return

		case "📤 Загрузить анализ":
			analysisType := stateManager.GetUserData(chatID, "analysis_type")
			if analysisType == "" {
				analysisType = "regular"
			}

			stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

			if analysisType == "sportsman" {
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:      chatID,
					Text:        "📄 Отправьте PDF-файл или фотографию анализов.\n\nЯ учту ваши спортивные данные при расшифровке!",
					ReplyMarkup: keyboards.BackMenu(),
				})
			} else {
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:      chatID,
					Text:        "📄 Отправьте PDF-файл или фотографию анализов.\n\nЯ сделаю расшифровку как для обычного пациента.",
					ReplyMarkup: keyboards.BackMenu(),
				})
			}
			return

		case "📝 Отзывы и предложения":
			FeedbackHandler(adminChatID)(ctx, b, update)
			return

		case "💎 Premium":
			PremiumHandler()(ctx, b, update)
			return

		case "⬅️ Назад":
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
		stateManager.SetUserData(chatID, "waiting_photo_confirm", "")
		stateManager.SetUserData(chatID, "pending_photo_id", "")
		stateManager.SetUserData(chatID, "photo_processed", "yes")
		stateManager.SetUserData(chatID, "question_asked", "")
		stateManager.SetUserData(chatID, "processed_group_id", "")
		stateManager.SetUserData(chatID, "media_group_id", "")

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
				Text:   "📸 Я проверил фото, но оно не похоже на медицинские анализы.\n\nПожалуйста, отправьте:\n• 📄 PDF-файл с анализами\n• 📸 Фотографию анализов (с таблицей и цифрами)\n• 📝 Текст с показателями\n\nЯ не могу обработать это фото как медицинский анализ.",
			})
			return
		}

		stateManager.SetUserData(chatID, "pending_photo_id", "")

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

		if userData["age"] == "" {
			collector := NewUserDataCollector(stateManager)
			collector.StartCollection(ctx, b, chatID)
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
		stateManager.SetUserData(chatID, "waiting_photo_confirm", "")
		stateManager.SetUserData(chatID, "pending_photo_id", "")
		stateManager.SetUserData(chatID, "photo_processed", "yes")
		stateManager.SetUserData(chatID, "question_asked", "")
		stateManager.SetUserData(chatID, "processed_group_id", "")
		stateManager.SetUserData(chatID, "media_group_id", "")
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
