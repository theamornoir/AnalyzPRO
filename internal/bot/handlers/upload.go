package handlers

import (
	"context"
	"log"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/service"
)

func UploadHandler(
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	uploadDir string,
	stickerID string,
) func(context.Context, *tgbot.Bot, *models.Update) {

	return func(
		ctx context.Context,
		b *tgbot.Bot,
		update *models.Update,
	) {

		if update.Message == nil {
			return
		}

		chatID := update.Message.Chat.ID
		log.Printf("📥 Получено сообщение: chatID=%d, text=%q, hasPhoto=%v, hasDocument=%v",
			chatID, update.Message.Text, update.Message.Photo != nil, update.Message.Document != nil)

		state := stateManager.GetState(chatID)
		log.Printf("📊 Текущее состояние: %s", state)

		// Если мы уже ждём ответ на вопрос о фото - игнорируем новые сообщения
		if state == states.StateWaitingPhotoConfirm {
			log.Printf("⏭️ Уже ждём ответ на вопрос, игнорируем")
			return
		}

		if state != states.StateWaitingAnalysisFile {
			log.Printf("⏭️ Состояние не StateWaitingAnalysisFile, отправляем сообщение")
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "📄 Отправьте PDF-файл или фотографию анализов.",
			})
			return
		}

		// ==========================================
		// ПРОВЕРКА: что отправляет пользователь
		// ==========================================

		// 1. Если это не документ и не фото и не текст
		if update.Message.Document == nil && update.Message.Photo == nil && update.Message.Text == "" {
			log.Printf("❌ Пустое сообщение")
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "❌ Я могу обрабатывать только:\n\n• 📄 PDF-файлы с анализами\n• 📸 Фотографии анализов\n• 📝 Текст с показателями\n\nПожалуйста, отправьте один из этих форматов.",
			})
			return
		}

		// 2. Если это документ - проверяем PDF
		if update.Message.Document != nil {
			log.Printf("📄 Обработка документа")

			// Уведомление о начале обработки
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "⏳ Принял файл, обрабатываю...\n\nЭто может занять несколько секунд.",
			})

			doc := update.Message.Document
			mimeType := doc.MimeType
			fileName := strings.ToLower(doc.FileName)

			isPDF := mimeType == "application/pdf" || strings.HasSuffix(fileName, ".pdf")

			if !isPDF {
				log.Printf("❌ Не PDF файл: %s", fileName)
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: chatID,
					Text:   "❌ Поддерживаются только PDF-файлы.\n\nПожалуйста, отправьте:\n• PDF с анализами\n• Или фотографию анализов\n• Или текст с показателями",
				})
				return
			}

			// Проверяем содержимое PDF
			fileData, _, err := downloadUploadedFile(ctx, b, update, uploadDir)
			if err != nil {
				sendError(ctx, b, chatID)
				return
			}

			if !isPDFLikelyAnalysis(fileData) {
				log.Printf("❌ PDF не похож на анализы")
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: chatID,
					Text:   "📄 Я получил PDF-файл, но он не похож на медицинские анализы.\n\nПожалуйста, убедитесь, что файл содержит:\n• 📊 Таблицы с показателями\n• 📝 Названия анализов (гемоглобин, холестерин и т.д.)\n• 🔢 Числовые значения и референсные нормы\n\nИли отправьте фотографию анализов / текст с показателями.",
				})
				return
			}

			log.Printf("✅ PDF проходит проверку, обрабатываем")
			processPDF(ctx, b, chatID, stateManager, analysisService, uploadDir, stickerID, fileData)
			return
		}

		// 3. Если это фото - проверяем
		if update.Message.Photo != nil {
			// ==========================================
			// ПРОВЕРКА: Bioscan
			// ==========================================
			analysisType := stateManager.GetUserData(chatID, "analysis_type")
			if analysisType == "bioscan" {
				log.Printf("📸 Обработка Bioscan")
				BioscanHandler(stateManager, analysisService, uploadDir, stickerID)(ctx, b, update)
				return
			}

			// ... остальной код

			log.Printf("📸 Обработка фото")

			// Уведомление о начале обработки
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "⏳ Принял фото, обрабатываю...\n\nЭто может занять несколько секунд.",
			})

			// Проверяем, не обработаны ли уже фото
			photoProcessed := stateManager.GetUserData(chatID, "photo_processed")
			log.Printf("📸 photoProcessed: %q", photoProcessed)
			if photoProcessed == "yes" {
				log.Printf("⏭️ Фото уже обработаны, игнорируем")
				return
			}

			// Проверяем, ждём ли мы уже ответа на вопрос
			waitingConfirm := stateManager.GetUserData(chatID, "waiting_photo_confirm")
			log.Printf("📸 waitingConfirm: %q", waitingConfirm)
			if waitingConfirm == "yes" {
				log.Printf("📸 Уже ждём ответа, сохраняем фото")
				// Уже ждём ответа - просто сохраняем новые фото
				photo := update.Message.Photo[len(update.Message.Photo)-1]
				existingPhotos := stateManager.GetUserData(chatID, "pending_photos")
				if existingPhotos == "" {
					stateManager.SetUserData(chatID, "pending_photos", photo.FileID)
				} else {
					stateManager.SetUserData(chatID, "pending_photos", existingPhotos+","+photo.FileID)
				}
				return
			}

			// Проверяем, задавали ли уже вопрос
			questionAsked := stateManager.GetUserData(chatID, "question_asked")
			log.Printf("📸 questionAsked: %q", questionAsked)
			if questionAsked == "yes" {
				log.Printf("📸 Вопрос уже задан, сохраняем фото")
				// Вопрос уже задан - просто сохраняем фото
				photo := update.Message.Photo[len(update.Message.Photo)-1]
				existingPhotos := stateManager.GetUserData(chatID, "pending_photos")
				if existingPhotos == "" {
					stateManager.SetUserData(chatID, "pending_photos", photo.FileID)
				} else {
					stateManager.SetUserData(chatID, "pending_photos", existingPhotos+","+photo.FileID)
				}
				return
			}

			// ==========================================
			// НОВАЯ ПРОВЕРКА: MediaGroupID
			// ==========================================
			mediaGroupID := update.Message.MediaGroupID
			if mediaGroupID != "" {
				// Проверяем, не обрабатываем ли уже эту группу
				processedGroup := stateManager.GetUserData(chatID, "processed_group_id")
				if processedGroup == mediaGroupID {
					log.Printf("📸 Медиа-группа %s уже обрабатывается, игнорируем", mediaGroupID)
					// Но сохраняем фото в список
					photo := update.Message.Photo[len(update.Message.Photo)-1]
					existingPhotos := stateManager.GetUserData(chatID, "pending_photos")
					if existingPhotos == "" {
						stateManager.SetUserData(chatID, "pending_photos", photo.FileID)
					} else {
						stateManager.SetUserData(chatID, "pending_photos", existingPhotos+","+photo.FileID)
					}
					return
				}
				// Сохраняем ID группы
				stateManager.SetUserData(chatID, "processed_group_id", mediaGroupID)
				log.Printf("📸 Начинаем обработку медиа-группы: %s", mediaGroupID)
			}

			// Сохраняем фото
			photo := update.Message.Photo[len(update.Message.Photo)-1]
			existingPhotos := stateManager.GetUserData(chatID, "pending_photos")

			// Добавляем фото в список
			if existingPhotos == "" {
				stateManager.SetUserData(chatID, "pending_photos", photo.FileID)
			} else {
				stateManager.SetUserData(chatID, "pending_photos", existingPhotos+","+photo.FileID)
			}

			if mediaGroupID != "" {
				stateManager.SetUserData(chatID, "media_group_id", mediaGroupID)
			}

			// Проверяем подпись
			caption := strings.ToLower(update.Message.Caption)
			keywords := []string{
				"анализ", "кровь", "биохимия", "результат", "показатель",
				"гемоглобин", "холестерин", "глюкоза", "билирубин",
			}
			hasKeyword := false
			for _, kw := range keywords {
				if strings.Contains(caption, kw) {
					hasKeyword = true
					break
				}
			}

			if hasKeyword {
				log.Printf("📸 Есть ключевые слова в подписи, обрабатываем сразу")
				// Если есть ключевые слова - обрабатываем сразу
				processAllPhotos(ctx, b, chatID, stateManager, analysisService, uploadDir, stickerID)
				return
			}

			// Устанавливаем флаг, что вопрос задан
			log.Printf("📸 Устанавливаем флаги и отправляем вопрос")
			stateManager.SetUserData(chatID, "question_asked", "yes")
			stateManager.SetUserData(chatID, "waiting_photo_confirm", "yes")
			stateManager.SetState(chatID, states.StateWaitingPhotoConfirm)

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "📸 Это медицинские анализы?\n\nОтветьте: Да / Нет",
			})

			log.Printf("✅ Вопрос отправлен, завершаем обработку")
			return
		}

		// 4. Если это текст - проверяем ответ на вопрос про фото
		if update.Message.Text != "" {
			log.Printf("📝 Обработка текста: %q", update.Message.Text)
			text := strings.TrimSpace(strings.ToLower(update.Message.Text))

			// Проверяем, ждём ли ответа
			waitingConfirm := stateManager.GetUserData(chatID, "waiting_photo_confirm")
			log.Printf("📝 waitingConfirm: %q", waitingConfirm)

			if waitingConfirm == "yes" {
				log.Printf("📝 Это ответ на вопрос")
				// Это ответ на вопрос
				if text == "да" || text == "да." || text == "ага" || text == "yes" {
					log.Printf("📝 Ответ: ДА")
					// Очищаем флаги
					stateManager.SetUserData(chatID, "waiting_photo_confirm", "")
					stateManager.SetUserData(chatID, "question_asked", "")

					processAllPhotos(ctx, b, chatID, stateManager, analysisService, uploadDir, stickerID)
					return

				} else if text == "нет" || text == "нет." || text == "неа" || text == "no" {
					log.Printf("📝 Ответ: НЕТ")
					stateManager.SetUserData(chatID, "waiting_photo_confirm", "")
					stateManager.SetUserData(chatID, "question_asked", "")
					stateManager.SetUserData(chatID, "pending_photos", "")
					stateManager.SetUserData(chatID, "photo_processed", "yes")
					stateManager.SetUserData(chatID, "processed_group_id", "")
					stateManager.SetUserData(chatID, "media_group_id", "")
					stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

					_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
						ChatID:      chatID,
						Text:        "📄 Понял! Тогда, пожалуйста, отправьте:\n\n• 📄 PDF-файл с анализами\n• 📸 Фотографию анализов\n• 📝 Текст с показателями\n\nЯ помогу вам с расшифровкой!",
						ReplyMarkup: keyboards.BackMenu(),
					})
					return
				} else {
					log.Printf("📝 Непонятный ответ: %q", text)
					_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
						ChatID: chatID,
						Text:   "Пожалуйста, ответьте 'Да' или 'Нет'.\n\nЭто медицинские анализы?",
					})
					return
				}
			}

			// Обычная проверка текста на анализы
			log.Printf("📝 Обычная проверка текста на анализы")
			keywords := []string{
				"гемоглобин", "эритроциты", "лейкоциты", "тромбоциты",
				"холестерин", "глюкоза", "билирубин", "креатинин",
				"алт", "аст", "ферритин", "пролактин", "эстрадиол",
				"тестостерон", "ммоль", "мкмоль", "ед/л", "нг/л",
				"мкг/л", "г/л", "норма", "референс", "показатель",
				"анализ", "кровь", "биохимия", "липидограмма",
			}

			hasKeyword := false
			lowerText := strings.ToLower(text)
			for _, kw := range keywords {
				if strings.Contains(lowerText, kw) {
					hasKeyword = true
					break
				}
			}

			hasNumbers := strings.ContainsAny(text, "0123456789")
			hasUnits := strings.Contains(text, "мкмоль") || strings.Contains(text, "ммоль") ||
				strings.Contains(text, "ед/л") || strings.Contains(text, "нг/л") ||
				strings.Contains(text, "мкг/л") || strings.Contains(text, "г/л")

			if !hasKeyword && !hasNumbers {
				log.Printf("❌ Текст не похож на анализы (нет ключевых слов и чисел)")
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: chatID,
					Text:   "❓ Это не похоже на медицинские анализы.\n\nПожалуйста, отправьте:\n• 📄 PDF-файл с анализами\n• 📸 Фотографию анализов\n• 📝 Текст с показателями\n\nПример: \"Гемоглобин 110, норма 130-160\"",
				})
				return
			}

			if hasKeyword && !hasNumbers && !hasUnits {
				log.Printf("❌ Текст похож на анализы, но нет чисел")
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: chatID,
					Text:   "📝 Я вижу упоминание анализов, но не вижу числовых значений.\n\nПожалуйста, добавьте конкретные показатели с числами.\n\nПример: \"Гемоглобин 110, норма 130-160\"",
				})
				return
			}
		}

		// ==========================================
		// ОБРАБОТКА ТЕКСТА
		// ==========================================

		payload := strings.TrimSpace(update.Message.Text)

		if payload == "" {
			log.Printf("⚠️ Пустой текст, игнорируем")
			return
		}

		log.Printf("📤 Отправляем текст в AI: %q", payload[:min(len(payload), 50)])
		loadingMsg, textMsg := sendLoadingMessages(ctx, b, chatID, stickerID)

		userData := stateManager.GetAllUserData(chatID)
		analysisText := buildAnalysisText(userData)

		result, err := analysisService.HandleAnalysisWithContext(ctx, payload, analysisText)
		if err != nil {
			deleteMessage(ctx, b, chatID, loadingMsg.ID)
			deleteMessage(ctx, b, chatID, textMsg.ID)
			sendError(ctx, b, chatID)
			return
		}

		deleteMessage(ctx, b, chatID, loadingMsg.ID)
		deleteMessage(ctx, b, chatID, textMsg.ID)

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   result,
		})

		// Возвращаемся в состояние ожидания файла с кнопкой "Назад"
		stateManager.SetState(chatID, states.StateWaitingAnalysisFile)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "📄 Файл обработан! ✅\n\n📎 Вы можете отправить ещё один файл (PDF или фото) или нажать ⬅️ Назад для возврата в меню.\n\n⚠️ Отправляйте по одному файлу за раз.",
			ReplyMarkup: keyboards.BackMenu(),
		})
	}
}

// handleFilesConfirm - обрабатывает подтверждение для группы файлов
func handleFilesConfirm(
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

	if text == "да" || text == "да." || text == "ага" || text == "yes" {
		// Проверяем, есть ли невалидные файлы
		hasInvalid := stateManager.GetUserData(chatID, "has_invalid_file")

		if hasInvalid == "yes" {
			stateManager.SetUserData(chatID, "has_invalid_file", "")
			stateManager.SetUserData(chatID, "pending_files", "")
			stateManager.SetUserData(chatID, "media_group_id", "")
			stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "📄 Я проверил все файлы. Один или несколько файлов не похожи на медицинские анализы.\n\nПожалуйста, убедитесь, что все файлы содержат:\n• 📊 Таблицы с показателями\n• 📝 Названия анализов (гемоглобин, холестерин и т.д.)\n• 🔢 Числовые значения и референсные нормы\n\nОтправьте только файлы с анализами.",
			})
			return
		}

		// Все файлы валидны - обрабатываем их все
		// Пока обрабатываем только первый файл
		// TODO: обработать все файлы из группы
		stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "✅ Все файлы прошли проверку! Обрабатываю...",
		})
		return

	} else if text == "нет" || text == "нет." || text == "неа" || text == "no" {
		stateManager.SetUserData(chatID, "pending_files", "")
		stateManager.SetUserData(chatID, "media_group_id", "")
		stateManager.SetUserData(chatID, "has_invalid_file", "")
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
