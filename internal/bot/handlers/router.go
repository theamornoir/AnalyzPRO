package handlers

import (
	"context"
	"log"
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

		state := stateManager.GetState(chatID)

		// ==========================================
		// ОБРАБОТКА СОСТОЯНИЙ ДЛЯ СБОРА ДАННЫХ
		// ==========================================

		collector := NewUserDataCollector(stateManager)

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

		// ==========================================
		// ОБРАБОТКА ОСТАЛЬНЫХ СОСТОЯНИЙ
		// ==========================================

		// StateWaitingPhotoConfirm - УДАЛЁН!
		// Вся логика обрабатывается в upload.go

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
			log.Printf("⚠️ router.go: пустой текст, игнорируем")
			return
		}

		switch text {
		case "📤 Загрузить анализ":
			// Сбрасываем все флаги
			stateManager.SetUserData(chatID, "photo_processed", "")
			stateManager.SetUserData(chatID, "waiting_photo_confirm", "")
			stateManager.SetUserData(chatID, "pending_photos", "")
			stateManager.SetUserData(chatID, "question_asked", "")
			stateManager.SetUserData(chatID, "processed_group_id", "")
			stateManager.SetUserData(chatID, "media_group_id", "")
			collector.StartCollection(ctx, b, chatID)
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
			FeedbackHandler(adminChatID)(ctx, b, update)
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

		userData := stateManager.GetAllUserData(chatID)
		if userData["age"] == "" {
			collector := NewUserDataCollector(stateManager)
			collector.StartCollection(ctx, b, chatID)
			return
		}

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

	userData := stateManager.GetAllUserData(chatID)
	if userData["age"] == "" {
		collector := NewUserDataCollector(stateManager)
		collector.StartCollection(ctx, b, chatID)
		return
	}

	stateManager.SetState(chatID, states.StateWaitingAnalysisFile)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "📄 Спасибо! Теперь отправьте PDF-файл или фотографию анализов.\n\nЯ учту вашу информацию о курсе при расшифровке.",
	})
}
