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
	"github.com/theamornoir/analyzpro/internal/storage"
)

func MessageRouter(
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	uploadDir string,
	stickerID string,
	adminChatID int64,
	agreementStorage *storage.AgreementStorage,
) func(context.Context, *tgbot.Bot, *models.Update) {

	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		chatID := update.Message.Chat.ID
		text := strings.TrimSpace(update.Message.Text)

		// ==========================================
		// ДИАГНОСТИКА - ЛОГИРУЕМ ВСЕ ДАННЫЕ
		// ==========================================
		currentState := stateManager.GetState(chatID)
		agreed := agreementStorage.IsAgreed(chatID)
		analysisType := stateManager.GetUserData(chatID, "analysis_type")
		analysisSubtype := stateManager.GetUserData(chatID, "analysis_subtype")

		log.Printf("🔍 [ДИАГНОСТИКА] chatID=%d, text=%q, state=%s, agreed=%v, analysis_type=%q, analysis_subtype=%q",
			chatID, text, currentState, agreed, analysisType, analysisSubtype)

		// ==========================================
		// ПРОВЕРКА СОГЛАШЕНИЯ
		// ==========================================
		agreementAccepted := agreementStorage.IsAgreed(chatID)

		if !agreementAccepted {
			log.Printf("📝 Соглашение НЕ принято для chatID=%d", chatID)

			if text == "📝 Пользовательское соглашение" {
				log.Printf("📝 Показываем текст соглашения")
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:      chatID,
					Text:        keyboards.UserAgreementText(),
					ReplyMarkup: keyboards.AgreementMenu(),
					ParseMode:   "Markdown",
				})
				return
			}

			if text == "✅ Принять соглашение" {
				log.Printf("✅ Пользователь принимает соглашение для chatID=%d", chatID)
				agreementStorage.SetAgreed(chatID)
				stateManager.SetState(chatID, states.StateIdle)

				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:      chatID,
					Text:        "✅ **Спасибо! Вы приняли пользовательское соглашение.**\n\nТеперь вы можете пользоваться ботом. Выберите тип анализа:",
					ReplyMarkup: keyboards.MainMenu(),
					ParseMode:   "Markdown",
				})
				return
			}

			if text != "" && text != "/start" {
				log.Printf("⏳ Просим принять соглашение для chatID=%d", chatID)
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: chatID,
					Text: `📝 Для начала работы примите пользовательское соглашение.

Нажмите кнопку 📝 Пользовательское соглашение, чтобы ознакомиться и принять.`,
					ReplyMarkup: keyboards.StartMenu(),
					ParseMode:   "Markdown",
				})
				return
			}
		}

		state := stateManager.GetState(chatID)
		log.Printf("📊 Текущее состояние после проверки соглашения: %s", state)

		// ==========================================
		// ОБРАБОТКА СОСТОЯНИЙ ДЛЯ BIOSCAN (ПЕРВОЙ!)
		// ==========================================

		if state == states.StateWaitingBioscanName {
			log.Printf("📸 Обработка StateWaitingBioscanName для chatID=%d", chatID)
			HandleBioscanName(ctx, b, stateManager, chatID, text)
			return
		}

		if state == states.StateWaitingBioscanAge {
			log.Printf("📸 Обработка StateWaitingBioscanAge для chatID=%d", chatID)
			HandleBioscanAge(ctx, b, stateManager, chatID, text)
			return
		}

		if state == states.StateWaitingBioscanHeight {
			log.Printf("📸 Обработка StateWaitingBioscanHeight для chatID=%d", chatID)
			HandleBioscanHeight(ctx, b, stateManager, chatID, text)
			return
		}

		if state == states.StateWaitingBioscanWeight {
			log.Printf("📸 Обработка StateWaitingBioscanWeight для chatID=%d", chatID)
			HandleBioscanWeight(ctx, b, stateManager, chatID, text)
			return
		}

		if state == states.StateWaitingBioscanGoal {
			log.Printf("📸 Обработка StateWaitingBioscanGoal для chatID=%d", chatID)
			HandleBioscanGoal(ctx, b, stateManager, chatID, text)
			return
		}

		if state == states.StateWaitingBioscanPhoto1 ||
			state == states.StateWaitingBioscanPhoto2 ||
			state == states.StateWaitingBioscanPhoto3 ||
			state == states.StateWaitingBioscanPhoto4 {

			log.Printf("📸 Обработка фото Bioscan (state=%s) для chatID=%d", state, chatID)
			if len(update.Message.Photo) > 0 {
				HandleBioscanPhoto(ctx, b, stateManager, chatID, update.Message.Photo)
			} else {
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: chatID,
					Text:   "📸 Пожалуйста, отправьте фотографию.",
				})
			}
			return
		}

		if state == states.StateWaitingBioscanConfirm {
			log.Printf("📸 Обработка подтверждения Bioscan для chatID=%d, text=%q", chatID, text)
			if text == "✅ Подтвердить и проанализировать" {
				ProcessBioscanWithPhotos(ctx, b, stateManager, analysisService, uploadDir, stickerID, chatID)
			} else if text == "🔄 Начать заново" {
				StartBioscanFlow(ctx, b, stateManager, chatID)
			} else {
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: chatID,
					Text:   "Пожалуйста, нажмите 'Подтвердить' для начала анализа или 'Начать заново'.",
				})
			}
			return
		}

		// ==========================================
		// ОБРАБОТКА КНОПКИ "НАЗАД"
		// ==========================================
		if text == "⬅️ Назад" {
			currentState := stateManager.GetState(chatID)
			currentAnalysisType := stateManager.GetUserData(chatID, "analysis_type")
			log.Printf("⬅️ Обработка 'Назад' для chatID=%d, state=%s", chatID, currentState)

			// Если мы в BIOSCAN - возвращаем в главное меню
			if currentState == states.StateWaitingBioscanName ||
				currentState == states.StateWaitingBioscanAge ||
				currentState == states.StateWaitingBioscanHeight ||
				currentState == states.StateWaitingBioscanWeight ||
				currentState == states.StateWaitingBioscanGoal ||
				currentState == states.StateWaitingBioscanPhoto1 ||
				currentState == states.StateWaitingBioscanPhoto2 ||
				currentState == states.StateWaitingBioscanPhoto3 ||
				currentState == states.StateWaitingBioscanPhoto4 ||
				currentState == states.StateWaitingBioscanConfirm {

				log.Printf("⬅️ Возврат из BIOSCAN в главное меню для chatID=%d", chatID)
				stateManager.SetState(chatID, states.StateIdle)
				// Очищаем все данные bioscan
				stateManager.SetUserData(chatID, "bioscan_name", "")
				stateManager.SetUserData(chatID, "bioscan_age", "")
				stateManager.SetUserData(chatID, "bioscan_height", "")
				stateManager.SetUserData(chatID, "bioscan_weight", "")
				stateManager.SetUserData(chatID, "bioscan_goal", "")
				stateManager.SetUserData(chatID, "bioscan_photo1", "")
				stateManager.SetUserData(chatID, "bioscan_photo2", "")
				stateManager.SetUserData(chatID, "bioscan_photo3", "")
				stateManager.SetUserData(chatID, "bioscan_photo4", "")
				stateManager.SetUserData(chatID, "bioscan_photo_count", "0")
				stateManager.SetUserData(chatID, "analysis_type", "")

				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:      chatID,
					Text:        "🔙 Вы вернулись в главное меню.\n\nВыберите действие:",
					ReplyMarkup: keyboards.MainMenu(),
				})
				return
			}

			// Остальные обработки "Назад"...
			if currentState == states.StateWaitingName ||
				currentState == states.StateWaitingGender ||
				currentState == states.StateWaitingAge ||
				currentState == states.StateWaitingHeight ||
				currentState == states.StateWaitingWeight ||
				currentState == states.StateWaitingChronicDiseases ||
				currentState == states.StateWaitingAllergies ||
				currentState == states.StateWaitingMedications ||
				currentState == states.StateWaitingSmoking ||
				currentState == states.StateWaitingAlcohol ||
				currentState == states.StateWaitingSportType ||
				currentState == states.StateWaitingTrainingExperience ||
				currentState == states.StateWaitingGoal ||
				currentState == states.StateWaitingCourseInfo ||
				currentState == states.StateWaitingCourseTime {

				log.Printf("⬅️ Возврат из опросника в меню выбора типа анализа для chatID=%d", chatID)
				stateManager.SetState(chatID, states.StateIdle)
				stateManager.SetUserData(chatID, "analysis_type", "")
				stateManager.SetUserData(chatID, "analysis_subtype", "")

				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:      chatID,
					Text:        "🔙 Возврат к выбору типа анализа.\n\nВыберите тип анализа:",
					ReplyMarkup: keyboards.AnalysisTypeMenu(),
					ParseMode:   "Markdown",
				})
				return
			}

			if currentState == states.StateIdle && (currentAnalysisType == "regular" || currentAnalysisType == "extended") {
				log.Printf("⬅️ Возврат из меню выбора анализа в главное меню для chatID=%d", chatID)
				stateManager.SetState(chatID, states.StateIdle)
				stateManager.SetUserData(chatID, "analysis_type", "")
				stateManager.SetUserData(chatID, "analysis_subtype", "")
				stateManager.SetUserData(chatID, "photo_processed", "")
				stateManager.SetUserData(chatID, "waiting_photo_confirm", "")
				stateManager.SetUserData(chatID, "pending_photos", "")
				stateManager.SetUserData(chatID, "pending_docs", "")
				stateManager.SetUserData(chatID, "question_asked", "")
				stateManager.SetUserData(chatID, "processed_group_id", "")
				stateManager.SetUserData(chatID, "media_group_id", "")

				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:      chatID,
					Text:        "🔙 Вы вернулись в главное меню.\n\nВыберите действие:",
					ReplyMarkup: keyboards.MainMenu(),
				})
				return
			}

			if currentState == states.StateWaitingAnalysisFile || currentState == states.StateWaitingUploadConfirm || currentState == states.StateWaitingPhotoConfirm {
				log.Printf("⬅️ Возврат из ожидания файлов в меню выбора типа анализа для chatID=%d", chatID)
				stateManager.SetState(chatID, states.StateIdle)
				stateManager.SetUserData(chatID, "analysis_type", "")
				stateManager.SetUserData(chatID, "analysis_subtype", "")
				stateManager.SetUserData(chatID, "photo_processed", "")
				stateManager.SetUserData(chatID, "waiting_photo_confirm", "")
				stateManager.SetUserData(chatID, "pending_photos", "")
				stateManager.SetUserData(chatID, "pending_docs", "")
				stateManager.SetUserData(chatID, "question_asked", "")
				stateManager.SetUserData(chatID, "processed_group_id", "")
				stateManager.SetUserData(chatID, "media_group_id", "")

				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:      chatID,
					Text:        "🔙 Возврат к выбору типа анализа.\n\nВыберите тип анализа:",
					ReplyMarkup: keyboards.AnalysisTypeMenu(),
					ParseMode:   "Markdown",
				})
				return
			}

			log.Printf("⬅️ Обычный возврат в главное меню для chatID=%d", chatID)
			stateManager.SetState(chatID, states.StateIdle)
			stateManager.SetUserData(chatID, "photo_processed", "")
			stateManager.SetUserData(chatID, "waiting_photo_confirm", "")
			stateManager.SetUserData(chatID, "pending_photos", "")
			stateManager.SetUserData(chatID, "pending_docs", "")
			stateManager.SetUserData(chatID, "question_asked", "")
			stateManager.SetUserData(chatID, "processed_group_id", "")
			stateManager.SetUserData(chatID, "media_group_id", "")
			stateManager.SetUserData(chatID, "analysis_type", "")
			stateManager.SetUserData(chatID, "analysis_subtype", "")

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        "🔙 Вы вернулись в главное меню.\n\nВыберите действие:",
				ReplyMarkup: keyboards.MainMenu(),
			})
			return
		}

		// ==========================================
		// ОБРАБОТКА СОСТОЯНИЙ ДЛЯ СБОРА ДАННЫХ (ОПРОСНИК)
		// ==========================================

		collector := NewUserDataCollector(stateManager)

		if state == states.StateWaitingName {
			log.Printf("📝 Обработка StateWaitingName для chatID=%d", chatID)
			collector.HandleName(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingGender {
			log.Printf("📝 Обработка StateWaitingGender для chatID=%d", chatID)
			collector.HandleGender(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingAge {
			log.Printf("📝 Обработка StateWaitingAge для chatID=%d", chatID)
			collector.HandleAge(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingHeight {
			log.Printf("📝 Обработка StateWaitingHeight для chatID=%d", chatID)
			collector.HandleHeight(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingWeight {
			log.Printf("📝 Обработка StateWaitingWeight для chatID=%d", chatID)
			collector.HandleWeight(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingChronicDiseases {
			log.Printf("📝 Обработка StateWaitingChronicDiseases для chatID=%d", chatID)
			collector.HandleChronicDiseases(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingAllergies {
			log.Printf("📝 Обработка StateWaitingAllergies для chatID=%d", chatID)
			collector.HandleAllergies(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingMedications {
			log.Printf("📝 Обработка StateWaitingMedications для chatID=%d", chatID)
			collector.HandleMedications(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingSmoking {
			log.Printf("📝 Обработка StateWaitingSmoking для chatID=%d", chatID)
			collector.HandleSmoking(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingAlcohol {
			log.Printf("📝 Обработка StateWaitingAlcohol для chatID=%d", chatID)
			collector.HandleAlcohol(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingSportType {
			log.Printf("📝 Обработка StateWaitingSportType для chatID=%d", chatID)
			collector.HandleSportType(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingTrainingExperience {
			log.Printf("📝 Обработка StateWaitingTrainingExperience для chatID=%d", chatID)
			collector.HandleTrainingExperience(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingGoal {
			log.Printf("📝 Обработка StateWaitingGoal для chatID=%d", chatID)
			collector.HandleGoal(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingCourseInfo {
			log.Printf("📝 Обработка StateWaitingCourseInfo для chatID=%d", chatID)
			collector.HandleCourseInfo(ctx, b, chatID, text)
			return
		}

		if state == states.StateWaitingCourseTime {
			log.Printf("📝 Обработка StateWaitingCourseTime для chatID=%d", chatID)
			collector.HandleCourseTime(ctx, b, chatID, text)
			return
		}

		// ==========================================
		// ОБРАБОТКА ЗАГРУЗКИ И ПОДТВЕРЖДЕНИЯ АНАЛИЗОВ
		// ==========================================

		if text == "✅ Обработать анализы" {
			log.Printf("🚀 Нажата кнопка '✅ Обработать анализы' для chatID=%d", chatID)
			UploadHandler(
				stateManager,
				analysisService,
				reportRenderer,
				uploadDir,
				stickerID,
			)(ctx, b, update)
			return
		}

		if state == states.StateWaitingPhotoConfirm {
			log.Printf("📸 Обработка StateWaitingPhotoConfirm для chatID=%d, text=%q", chatID, text)
			handlePhotoConfirm(ctx, b, stateManager, analysisService, uploadDir, stickerID, chatID, text)
			return
		}

		if state == states.StateWaitingAnalysisFile || state == states.StateWaitingUploadConfirm || len(update.Message.Photo) > 0 || update.Message.Document != nil {
			log.Printf("📄 Обработка файла/фото для chatID=%d, state=%s", chatID, state)
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

		log.Printf("🔍 Обработка кнопки меню: %q для chatID=%d", text, chatID)

		switch text {
		case "📝 Пользовательское соглашение":
			log.Printf("📝 Обработка: Пользовательское соглашение для chatID=%d", chatID)
			if agreementStorage.IsAgreed(chatID) {
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
			log.Printf("✅ Обработка: Принять соглашение для chatID=%d", chatID)
			if agreementStorage.IsAgreed(chatID) {
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:      chatID,
					Text:        "✅ Вы уже приняли пользовательское соглашение.\n\nВыберите действие в главном меню:",
					ReplyMarkup: keyboards.MainMenu(),
				})
				return
			}

			agreementStorage.SetAgreed(chatID)
			stateManager.SetState(chatID, states.StateIdle)

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        "✅ **Спасибо! Вы приняли пользовательское соглашение.**\n\nТеперь вы можете пользоваться ботом. Выберите тип анализа:",
				ReplyMarkup: keyboards.MainMenu(),
				ParseMode:   "Markdown",
			})
			return

		case "ℹ️ О сервисе":
			log.Printf("ℹ️ Обработка: О сервисе для chatID=%d", chatID)
			AboutHandler()(ctx, b, update)
			return

		case "🏥 Диагностика анализов":
			log.Printf("🏥 Обработка: Диагностика анализов для chatID=%d", chatID)
			if !agreementStorage.IsAgreed(chatID) {
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:      chatID,
					Text:        "📝 Пожалуйста, сначала примите пользовательское соглашение.",
					ReplyMarkup: keyboards.StartMenu(),
				})
				return
			}

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text: "🏥 **Диагностика анализов**\n\n" +
					"Выберите тип анализа:\n\n" +
					"📊 **Обычный анализ** - базовая расшифровка\n" +
					"🔬 **Расширенный анализ** - детальная расшифровка с учетом вашего анкетного профиля и рекомендациями\n\n" +
					"Вы можете отправить один или несколько файлов (PDF или фото).",
				ReplyMarkup: keyboards.AnalysisTypeMenu(),
				ParseMode:   "Markdown",
			})
			return

		case "📊 Обычный анализ":
			log.Printf("📊 Обработка: Обычный анализ для chatID=%d", chatID)
			if !agreementStorage.IsAgreed(chatID) {
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:      chatID,
					Text:        "📝 Пожалуйста, сначала примите пользовательское соглашение.",
					ReplyMarkup: keyboards.StartMenu(),
				})
				return
			}

			stateManager.SetUserData(chatID, "analysis_type", "regular")
			stateManager.SetUserData(chatID, "analysis_subtype", "regular")
			stateManager.SetUserData(chatID, "pending_photos", "")
			stateManager.SetUserData(chatID, "pending_docs", "")
			stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text: "📊 **Обычный анализ**\n\n" +
					"⚠️ **ВАЖНО:** Пожалуйста, отправляйте PDF-файлы или фотографии **строго по одному сообщению** (не выделяйте сразу несколько фото для отправки альбомом).\n\n" +
					"После отправки всех нужных файлов нажмите **«✅ Обработать анализы»**.\n\n" +
					"📎 Поддерживаются: PDF, JPG, PNG",
				ReplyMarkup: keyboards.ProcessAnalysisMenu(),
				ParseMode:   "Markdown",
			})
			return

		case "🔬 Расширенный анализ":
			log.Printf("🔬 Обработка: Расширенный анализ для chatID=%d", chatID)
			if !agreementStorage.IsAgreed(chatID) {
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:      chatID,
					Text:        "📝 Пожалуйста, сначала примите пользовательское соглашение.",
					ReplyMarkup: keyboards.StartMenu(),
				})
				return
			}

			stateManager.SetUserData(chatID, "analysis_type", "extended")
			stateManager.SetUserData(chatID, "analysis_subtype", "extended")
			stateManager.SetUserData(chatID, "pending_photos", "")
			stateManager.SetUserData(chatID, "pending_docs", "")

			stateManager.SetState(chatID, states.StateWaitingName)

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text: "🔬 **Расширенный анализ**\n\n" +
					"Для более точной расшифровки с учетом особенностей вашего организма ответьте на несколько вопросов.\n\n" +
					"👤 **Введите ваше имя:**",
				ReplyMarkup: keyboards.BackMenu(),
				ParseMode:   "Markdown",
			})
			return

		case "📸 Bioscan":
			log.Printf("📸 Обработка: Bioscan для chatID=%d", chatID)

			if !agreementStorage.IsAgreed(chatID) {
				log.Printf("⚠️ Соглашение не принято для chatID=%d", chatID)
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID:      chatID,
					Text:        "📝 Пожалуйста, сначала примите пользовательское соглашение.",
					ReplyMarkup: keyboards.StartMenu(),
				})
				return
			}

			currentState := stateManager.GetState(chatID)
			if currentState != states.StateIdle {
				log.Printf("⚠️ Пользователь chatID=%d занят другим процессом: %s", chatID, currentState)
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: chatID,
					Text:   "⏳ Пожалуйста, завершите текущее действие или нажмите 'Назад'.",
				})
				return
			}

			// ПРИНУДИТЕЛЬНО УСТАНАВЛИВАЕМ СОСТОЯНИЕ ЗДЕСЬ
			stateManager.SetState(chatID, states.StateWaitingBioscanName)
			log.Printf("📌 Принудительно установлено состояние StateWaitingBioscanName для чата %d", chatID)

			// Проверяем, что состояние установилось
			if stateManager.GetState(chatID) != states.StateWaitingBioscanName {
				log.Printf("❌ НЕ УДАЛОСЬ установить состояние! Текущее состояние: %s", stateManager.GetState(chatID))
			}

			// Сбрасываем данные
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

			log.Printf("✅ Запускаем Bioscan для chatID=%d", chatID)

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text: "📸 **Bioscan - комплексный анализ тела**\n\n" +
					"Я проведу детальный анализ вашей фигуры и дам персональные рекомендации.\n\n" +
					"📋 **Шаг 1 из 6: Введите ваше имя**",
				ParseMode:   "Markdown",
				ReplyMarkup: keyboards.BackMenu(),
			})
			return

		case "📤 Загрузить анализ":
			log.Printf("📤 Обработка: Загрузить анализ для chatID=%d", chatID)
			analysisType := stateManager.GetUserData(chatID, "analysis_type")
			analysisSubtype := stateManager.GetUserData(chatID, "analysis_subtype")

			if analysisType == "" {
				analysisType = "regular"
			}

			stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

			if analysisSubtype == "extended" {
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: chatID,
					Text: "🔬 **Загрузка анализов**\n\n" +
						"⚠️ **ВАЖНО:** Пожалуйста, отправляйте PDF-файлы или фотографии **строго по одному сообщению** (не альбомом).\n\n" +
						"После отправки каждого файла вы можете отправить следующий, а затем нажать **«✅ Обработать анализы»**.",
					ReplyMarkup: keyboards.ProcessAnalysisMenu(),
					ParseMode:   "Markdown",
				})
			} else {
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: chatID,
					Text: "📄 **Загрузка анализов**\n\n" +
						"⚠️ **ВАЖНО:** Пожалуйста, отправляйте PDF-файлы или фотографии **строго по одному сообщению** (не альбомом).\n\n" +
						"После отправки всех нужных файлов нажмите **«✅ Обработать анализы»**.",
					ReplyMarkup: keyboards.ProcessAnalysisMenu(),
					ParseMode:   "Markdown",
				})
			}
			return

		case "📝 Отзывы и предложения":
			log.Printf("📝 Обработка: Отзывы и предложения для chatID=%d", chatID)
			FeedbackHandler(adminChatID)(ctx, b, update)
			return

		case "💎 Premium":
			log.Printf("💎 Обработка: Premium для chatID=%d", chatID)
			PremiumHandler()(ctx, b, update)
			return

		case "⬅️ Назад":
			log.Printf("⬅️ Обработка: Назад для chatID=%d", chatID)
			return
		}

		// ==========================================
		// ОБРАБОТКА ТЕКСТА (обычный ответ от AI)
		// ==========================================

		log.Printf("💬 Обработка обычного текста для chatID=%d", chatID)
		result, err := analysisService.HandleAnalysis(ctx, text)
		if err != nil {
			log.Printf("❌ Ошибка обработки текста для chatID=%d: %v", chatID, err)
			stateManager.SetState(chatID, states.StateIdle)

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        "⚠️ Не удалось обработать анализ. Попробуйте позже.",
				ReplyMarkup: keyboards.MainMenu(),
			})
			return
		}

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   result,
		})
	}
}

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

	analysisType := stateManager.GetUserData(chatID, "analysis_type")
	analysisSubtype := stateManager.GetUserData(chatID, "analysis_subtype")
	isExtended := analysisSubtype == "extended" || analysisType == "extended"

	log.Printf("📸 handlePhotoConfirm: chatID=%d, text=%q, isExtended=%v", chatID, text, isExtended)

	if text == "да" || text == "да." || text == "ага" || text == "yes" || text == "д" {
		log.Printf("📸 Пользователь подтвердил, что это анализы для chatID=%d", chatID)
		stateManager.SetUserData(chatID, "waiting_photo_confirm", "")
		stateManager.SetUserData(chatID, "pending_photo_id", "")
		stateManager.SetUserData(chatID, "photo_processed", "yes")
		stateManager.SetUserData(chatID, "question_asked", "")
		stateManager.SetUserData(chatID, "processed_group_id", "")
		stateManager.SetUserData(chatID, "media_group_id", "")

		fileData, mimeType, err := downloadFileByID(ctx, b, pendingPhotoID, uploadDir)
		if err != nil {
			log.Printf("❌ Ошибка скачивания файла для chatID=%d: %v", chatID, err)
			sendErrorWithMenu(ctx, b, stateManager, chatID)
			return
		}

		stateManager.SetUserData(chatID, "pending_photo_id", "")

		userData := stateManager.GetAllUserData(chatID)

		loadingMsg, textMsg := sendLoadingMessages(ctx, b, chatID, stickerID)

		var analysisText string
		if isExtended {
			analysisText = buildAnalysisText(userData)
		} else {
			analysisText = "Обычный анализ без персональных данных."
		}

		result, err := analysisService.HandleAnalysisFromFileWithContext(ctx, fileData, mimeType, analysisText)
		if err != nil {
			log.Printf("❌ Ошибка анализа файла для chatID=%d: %v", chatID, err)
			deleteMessage(ctx, b, chatID, loadingMsg.ID)
			deleteMessage(ctx, b, chatID, textMsg.ID)
			sendErrorWithMenu(ctx, b, stateManager, chatID)
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
		log.Printf("📸 Пользователь НЕ подтвердил, что это анализы для chatID=%d", chatID)
		stateManager.SetUserData(chatID, "waiting_photo_confirm", "")
		stateManager.SetUserData(chatID, "pending_photo_id", "")
		stateManager.SetUserData(chatID, "photo_processed", "yes")
		stateManager.SetUserData(chatID, "question_asked", "")
		stateManager.SetUserData(chatID, "processed_group_id", "")
		stateManager.SetUserData(chatID, "media_group_id", "")
		stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "📄 Понял! Тогда, пожалуйста, отправьте:\n\n• 📄 PDF-файл с анализами\n• 📸 Фотографию анализов\n• 📝 Текст с показателями\n\nЯ помогу вам с расшифровкой!",
			ReplyMarkup: keyboards.ProcessAnalysisMenu(),
		})
		return
	} else {
		log.Printf("📸 Нераспознанный ответ на вопрос для chatID=%d: %q", chatID, text)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "Пожалуйста, ответьте 'Да' или 'Нет'.\n\nЭто медицинские анализы?",
		})
	}
}

func sendErrorWithMenu(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64) {
	log.Printf("⚠️ sendErrorWithMenu для chatID=%d", chatID)
	stateManager.Reset(chatID)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "⚠️ Произошла ошибка при обработке файла или запроса. Попробуйте еще раз.",
		ReplyMarkup: keyboards.MainMenu(),
	})
}
