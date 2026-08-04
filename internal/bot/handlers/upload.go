package handlers

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

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
			log.Printf("📸 Обработка фото")

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
						ChatID: chatID,
						Text:   "📄 Понял! Тогда, пожалуйста, отправьте:\n\n• 📄 PDF-файл с анализами\n• 📸 Фотографию анализов\n• 📝 Текст с показателями\n\nЯ помогу вам с расшифровкой!",
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

		stateManager.Reset(chatID)
	}
}

// processAllPhotos - обрабатывает все накопленные фото
func processAllPhotos(
	ctx context.Context,
	b *tgbot.Bot,
	chatID int64,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	uploadDir string,
	stickerID string,
) {
	// Получаем список всех фото
	pendingPhotos := stateManager.GetUserData(chatID, "pending_photos")
	if pendingPhotos == "" {
		return
	}

	// Очищаем список и флаги
	stateManager.SetUserData(chatID, "pending_photos", "")
	stateManager.SetUserData(chatID, "waiting_photo_confirm", "")
	stateManager.SetUserData(chatID, "photo_processed", "yes")
	stateManager.SetUserData(chatID, "question_asked", "")
	stateManager.SetUserData(chatID, "processed_group_id", "")
	stateManager.SetUserData(chatID, "media_group_id", "")

	// Разбиваем ID
	photoIDs := strings.Split(pendingPhotos, ",")

	// Скачиваем все фото
	var allFileData [][]byte
	var allMimeTypes []string

	for _, id := range photoIDs {
		fileData, mimeType, err := downloadFileByID(ctx, b, id, uploadDir)
		if err == nil {
			allFileData = append(allFileData, fileData)
			allMimeTypes = append(allMimeTypes, mimeType)
		}
	}

	if len(allFileData) == 0 {
		sendError(ctx, b, chatID)
		stateManager.Reset(chatID)
		return
	}

	// Проверяем первое фото (базовая проверка)
	if !isPhotoLikelyAnalysis(allFileData[0]) {
		stateManager.SetState(chatID, states.StateWaitingAnalysisFile)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "📸 Я проверил фото, но оно не похоже на медицинские анализы.\n\nПожалуйста, отправьте:\n• 📄 PDF-файл с анализами\n• 📸 Фотографию анализов (с таблицей и цифрами)\n• 📝 Текст с показателями\n\nЯ не могу обработать это фото как медицинский анализ.",
		})
		return
	}

	// Проверяем данные пользователя
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

	// Отправляем стикер загрузки
	loadingMsg, textMsg := sendLoadingMessages(ctx, b, chatID, stickerID)
	analysisText := buildAnalysisText(userData)

	// ==========================================
	// ОТПРАВЛЯЕМ ВСЕ ФОТО В GEMINI
	// ==========================================

	var result string
	var err error

	if len(allFileData) == 1 {
		// Если одно фото - отправляем как есть
		result, err = analysisService.HandleAnalysisFromFileWithContext(ctx, allFileData[0], allMimeTypes[0], analysisText)
	} else {
		// Если несколько фото - отправляем все по очереди и объединяем ответы
		var allResults []string

		for i, fileData := range allFileData {
			resp, err := analysisService.HandleAnalysisFromFileWithContext(ctx, fileData, allMimeTypes[i], analysisText)
			if err != nil {
				continue
			}
			allResults = append(allResults, resp)
		}

		if len(allResults) == 0 {
			deleteMessage(ctx, b, chatID, loadingMsg.ID)
			deleteMessage(ctx, b, chatID, textMsg.ID)
			sendError(ctx, b, chatID)
			stateManager.Reset(chatID)
			return
		}

		// Объединяем ответы
		result = strings.Join(allResults, "\n\n---\n\n")

		// Если ответ слишком длинный - сокращаем
		if len(result) > 4000 {
			result = result[:4000] + "\n\n... (ответ сокращён)"
		}
	}

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

// processPDF - обрабатывает PDF файл
func processPDF(
	ctx context.Context,
	b *tgbot.Bot,
	chatID int64,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	uploadDir string,
	stickerID string,
	fileData []byte,
) {
	loadingMsg, textMsg := sendLoadingMessages(ctx, b, chatID, stickerID)

	userData := stateManager.GetAllUserData(chatID)
	analysisText := buildAnalysisText(userData)

	result, err := analysisService.HandleAnalysisFromFileWithContext(ctx, fileData, "application/pdf", analysisText)
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

	stateManager.Reset(chatID)
}

// isPDFLikelyAnalysis - проверяет, похож ли PDF на медицинские анализы
func isPDFLikelyAnalysis(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	if len(data) < 5000 {
		return false
	}

	content := string(data)
	if len(content) > 10000 {
		content = content[:10000]
	}

	content = strings.ToLower(content)

	analysisKeywords := []string{
		"гемоглобин", "эритроциты", "лейкоциты", "тромбоциты",
		"нейтрофилы", "лимфоциты", "моноциты", "эозинофилы", "базофилы",
		"гематокрит", "цветовой", "показатель", "средний", "объем",
		"холестерин", "глюкоза", "билирубин", "креатинин", "мочевина",
		"алт", "аст", "ферритин", "пролактин", "эстрадиол",
		"тестостерон", "прогестерон", "кортизол", "тироксин", "т3", "т4",
		"триглицериды", "липопротеины", "лпвп", "лпнп", "калий", "натрий",
		"кальций", "магний", "фосфор", "железо", "трансферрин",
		"ммоль", "мкмоль", "ед/л", "нг/л", "мкг/л", "г/л", "мг/л", "мг/дл",
		"нмоль", "пмоль", "мме", "ме/л", "мкме", "ммоль/л", "мкмоль/л",
		"норма", "референс", "показатель", "анализ", "кровь", "биохимия",
		"липидограмма", "коагулограмма", "гормон", "витамин", "минерал",
		"общий", "свободный", "прямой", "непрямой", "белок",
		"альбумин", "глобулин", "щелочная", "фосфатаза", "гамма-гт",
		"мочевая", "кислота", "лактат", "дегидрогеназа", "креатинкиназа",
		"миоглобин", "тропонин", "гликированный", "инсулин",
		"с-пептид", "глюкагон", "паратгормон", "кальцитонин", "альдостерон",
		"ренин", "ангиотензин", "вазопрессин", "адреналин", "норадреналин",
		"дофамин", "серотонин", "гистамин", "брадикинин", "простагландин",
		"лаб", "лаборатория", "исследование", "диагностика", "скрининг",
		"биоматериал", "сыворотка", "плазма", "цельная", "моча",
		"кал", "ликвор", "экссудат", "транссудат", "биопсия", "пункция",
		"дата", "время", "взятия", "материала", "доставки", "готовности",
		"янв", "фев", "мар", "апр", "май", "июн", "июл", "авг", "сен", "окт", "ноя", "дек",
		"2024", "2025", "2026", "2027", "2028",
		"ф.и.о", "фио", "пациент", "возраст", "пол", "дата рождения",
		"направление", "врач", "клиника", "стационар", "амбулаторный",
		"номер", "заказа", "образца", "пробирка", "штрих-код",
	}

	foundCount := 0
	for _, kw := range analysisKeywords {
		if strings.Contains(content, kw) {
			foundCount++
		}
	}

	hasNumbers := strings.ContainsAny(content, "0123456789")
	hasUnits := strings.Contains(content, "ммоль") || strings.Contains(content, "мкмоль") ||
		strings.Contains(content, "ед/л") || strings.Contains(content, "нг/л") ||
		strings.Contains(content, "мкг/л") || strings.Contains(content, "г/л") ||
		strings.Contains(content, "мг/л") || strings.Contains(content, "мг/дл") ||
		strings.Contains(content, "нмоль") || strings.Contains(content, "пмоль") ||
		strings.Contains(content, "мме") || strings.Contains(content, "ме/л")

	hasArrows := strings.Contains(content, "↑") || strings.Contains(content, "↓")
	hasTable := strings.Contains(content, "|") && strings.Contains(content, "-")
	hasSpaces := strings.Contains(content, "  ") && strings.Contains(content, "\t")
	hasDate := strings.Contains(content, "202") || strings.Contains(content, "2026") ||
		strings.ContainsAny(content, "0123456789")

	if foundCount >= 2 {
		return true
	}

	if hasNumbers && hasUnits && foundCount >= 1 {
		return true
	}

	if hasArrows && hasNumbers {
		return true
	}

	if (hasTable || hasSpaces) && hasNumbers && foundCount >= 1 {
		return true
	}

	if hasDate && foundCount >= 1 {
		return true
	}

	if hasNumbers && hasUnits {
		hasStructure := strings.Contains(content, "|") ||
			strings.Contains(content, "  ") ||
			strings.Contains(content, "\t") ||
			strings.Contains(content, "норм") ||
			strings.Contains(content, "референс")

		if hasStructure {
			return true
		}
	}

	if hasNumbers && (strings.Contains(content, "анализ") || strings.Contains(content, "кровь") ||
		strings.Contains(content, "биохимия") || strings.Contains(content, "лаборатор")) {
		return true
	}

	return false
}

// isPhotoLikelyAnalysis - проверяет, похоже ли фото на медицинские анализы
func isPhotoLikelyAnalysis(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	if len(data) < 30000 || len(data) > 5000000 {
		return false
	}

	if len(data) >= 4 {
		if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
			if len(data) > 200000 {
				return true
			}
			return false
		}

		if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
			if len(data) > 150000 {
				return true
			}
			return false
		}

		if data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 {
			if len(data) > 150000 {
				return true
			}
			return false
		}

		if len(data) > 12 && data[4] == 0x66 && data[5] == 0x74 && data[6] == 0x79 && data[7] == 0x70 {
			if len(data) > 200000 {
				return true
			}
			return false
		}
	}

	return false
}

// downloadFileByID - скачивает файл по file_id
func downloadFileByID(
	ctx context.Context,
	b *tgbot.Bot,
	fileID string,
	uploadDir string,
) ([]byte, string, error) {
	file, err := b.GetFile(ctx, &tgbot.GetFileParams{FileID: fileID})
	if err != nil {
		return nil, "", err
	}

	resp, err := http.Get(b.FileDownloadLink(file))
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	if uploadDir != "" {
		_ = os.MkdirAll(uploadDir, 0o755)
		_ = os.WriteFile(filepath.Join(uploadDir, "photo.jpg"), data, 0o644)
	}

	return data, "image/jpeg", nil
}

// buildAnalysisText - формирует текст с учетом данных пользователя
func buildAnalysisText(userData map[string]string) string {
	var parts []string

	parts = append(parts, "❗ ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:")

	if age := userData["age"]; age != "" {
		parts = append(parts, fmt.Sprintf("• Возраст: %s лет", age))
	}

	height := userData["height"]
	weight := userData["weight"]

	if height != "" {
		parts = append(parts, fmt.Sprintf("• Рост: %s см", height))
	}
	if weight != "" {
		parts = append(parts, fmt.Sprintf("• Вес: %s кг", weight))
		if height != "" && weight != "" {
			h, err := strconv.ParseFloat(height, 64)
			if err == nil {
				w, err := strconv.ParseFloat(weight, 64)
				if err == nil && h > 0 && w > 0 {
					bmi := w / ((h / 100) * (h / 100))
					parts = append(parts, fmt.Sprintf("• ИМТ: %.1f", bmi))
				}
			}
		}
	}

	if chronic := userData["chronic_diseases"]; chronic != "" && strings.ToLower(chronic) != "нет" {
		parts = append(parts, fmt.Sprintf("• Хронические заболевания: %s", chronic))
	}

	if allergies := userData["allergies"]; allergies != "" && strings.ToLower(allergies) != "нет" {
		parts = append(parts, fmt.Sprintf("• Аллергии: %s", allergies))
	}

	if medications := userData["medications"]; medications != "" && strings.ToLower(medications) != "нет" {
		parts = append(parts, fmt.Sprintf("• Принимаемые лекарства: %s", medications))
	}

	if smoking := userData["smoking"]; smoking != "" && strings.ToLower(smoking) != "нет" {
		parts = append(parts, fmt.Sprintf("• Курение: %s", smoking))
	}
	if alcohol := userData["alcohol"]; alcohol != "" {
		parts = append(parts, fmt.Sprintf("• Алкоголь: %s", alcohol))
	}

	if sport := userData["sport_type"]; sport != "" {
		parts = append(parts, fmt.Sprintf("• Вид спорта: %s", sport))
	}
	if exp := userData["training_experience"]; exp != "" {
		parts = append(parts, fmt.Sprintf("• Стаж тренировок: %s лет", exp))
	}
	if goal := userData["goal"]; goal != "" {
		parts = append(parts, fmt.Sprintf("• Цель: %s", goal))
	}

	onCourse := userData["on_course"]
	if onCourse == "yes" {
		courseInfo := userData["course_info"]
		if courseInfo != "" {
			parts = append(parts, fmt.Sprintf("• КУРС: %s", courseInfo))
		}
		parts = append(parts, "• Требуется интерпретация с учетом фармакологии")
	} else if onCourse == "no" {
		parts = append(parts, "• Без курса (естественный фон)")
	}

	return strings.Join(parts, "\n")
}

// sendLoadingMessages - отправляет стикер и текстовое сообщение
func sendLoadingMessages(
	ctx context.Context,
	b *tgbot.Bot,
	chatID int64,
	stickerID string,
) (*models.Message, *models.Message) {

	if stickerID != "" {
		stickerMsg, err := b.SendSticker(ctx, &tgbot.SendStickerParams{
			ChatID: chatID,
			Sticker: &models.InputFileString{
				Data: stickerID,
			},
		})

		if err == nil && stickerMsg != nil {
			textMsg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "📊 Обрабатываю результаты...",
			})
			return stickerMsg, textMsg
		}
	}

	stickerMsg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "⏳ Обрабатываю результаты...",
	})

	return stickerMsg, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
