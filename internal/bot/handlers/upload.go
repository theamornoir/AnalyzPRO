package handlers

import (
	"context"
	"fmt"
	"io"
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

		state := stateManager.GetState(chatID)

		if state != states.StateWaitingAnalysisFile {
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
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "❌ Я могу обрабатывать только:\n\n• 📄 PDF-файлы с анализами\n• 📸 Фотографии анализов\n• 📝 Текст с показателями\n\nПожалуйста, отправьте один из этих форматов.",
			})
			return
		}

		// 2. Если это документ - проверяем PDF
		if update.Message.Document != nil {
			doc := update.Message.Document
			mimeType := doc.MimeType
			fileName := strings.ToLower(doc.FileName)

			isPDF := mimeType == "application/pdf" || strings.HasSuffix(fileName, ".pdf")

			if !isPDF {
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
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: chatID,
					Text:   "📄 Я получил PDF-файл, но он не похож на медицинские анализы.\n\nПожалуйста, убедитесь, что файл содержит:\n• 📊 Таблицы с показателями\n• 📝 Названия анализов (гемоглобин, холестерин и т.д.)\n• 🔢 Числовые значения и референсные нормы\n\nИли отправьте фотографию анализов / текст с показателями.",
				})
				return
			}

			processPDF(ctx, b, chatID, stateManager, analysisService, uploadDir, stickerID, fileData)
			return
		}

		// 3. Если это фото - проверяем
		if update.Message.Photo != nil {
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

			if !hasKeyword {
				photo := update.Message.Photo[len(update.Message.Photo)-1]
				stateManager.SetUserData(chatID, "pending_photo_id", photo.FileID)
				stateManager.SetState(chatID, states.StateWaitingPhotoConfirm)

				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: chatID,
					Text:   "📸 Это медицинские анализы?\n\nОтветьте: Да / Нет",
				})
				return
			}
		}

		// 4. Если это текст - проверяем ответ на вопрос про фото
		if update.Message.Text != "" {
			text := strings.TrimSpace(strings.ToLower(update.Message.Text))

			pendingPhotoID := stateManager.GetUserData(chatID, "pending_photo_id")

			if pendingPhotoID != "" {
				if text == "да" || text == "да." || text == "ага" || text == "yes" {
					stateManager.SetUserData(chatID, "pending_photo_id", "")

					// Скачиваем фото для проверки
					fileData, mimeType, err := downloadFileByID(ctx, b, pendingPhotoID, uploadDir)
					if err != nil {
						sendError(ctx, b, chatID)
						return
					}

					// Проверяем содержимое фото
					if !isPhotoLikelyAnalysis(fileData) {
						stateManager.SetUserData(chatID, "pending_photo_id", "")
						stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

						_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
							ChatID: chatID,
							Text:   "📸 Я проверил фото, но оно не похоже на медицинские анализы.\n\nПожалуйста, убедитесь, что на фото видно:\n• 📊 Таблицу с показателями\n• 📝 Названия анализов (гемоглобин, холестерин и т.д.)\n• 🔢 Числовые значения\n\nИли отправьте:\n• 📄 PDF-файл с анализами\n• 📝 Текст с показателями",
						})
						return
					}

					// Это действительно анализы - обрабатываем
					stateManager.SetUserData(chatID, "pending_photo_id", "")

					// Проверяем, есть ли информация о курсе
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

					// Проверяем, есть ли данные пользователя (возраст, рост и т.д.)
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

				} else if text == "нет" || text == "нет." || text == "неа" || text == "no" {
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
					return
				}
			}

			// Обычная проверка текста на анализы
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
				_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
					ChatID: chatID,
					Text:   "❓ Это не похоже на медицинские анализы.\n\nПожалуйста, отправьте:\n• 📄 PDF-файл с анализами\n• 📸 Фотографию анализов\n• 📝 Текст с показателями\n\nПример: \"Гемоглобин 110, норма 130-160\"",
				})
				return
			}

			if hasKeyword && !hasNumbers && !hasUnits {
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
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "Пожалуйста, отправьте PDF или фотографию анализов.",
			})
			return
		}

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

// isPhotoLikelyAnalysis - проверяет, похоже ли фото на медицинские анализы (улучшенная версия)
func isPhotoLikelyAnalysis(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// Проверяем размер фото - слишком маленькие файлы (меньше 30KB) - точно не анализы
	if len(data) < 30000 {
		return false
	}

	// Проверяем тип файла по магическим числам (сигнатурам)
	if len(data) >= 4 {
		// JPEG: FF D8 FF
		if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
			// JPEG - проверяем размер
			// Для анализов нужны качественные фото, обычно > 200KB
			if len(data) > 200000 { // качественное фото документа
				return true
			}
			// Если фото 30-200KB - это может быть иконка, стикер или маленькое фото
			// Не доверяем, даже если пользователь сказал "Да"
			return false
		}

		// PNG: 89 50 4E 47
		if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
			if len(data) > 150000 {
				return true
			}
			return false
		}

		// WEBP: 52 49 46 46
		if data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 {
			if len(data) > 150000 {
				return true
			}
			return false
		}
	}

	// Если формат неизвестен - не доверяем
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

	// Добавляем основную информацию
	parts = append(parts, "❗ ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:")

	// Возраст
	if age := userData["age"]; age != "" {
		parts = append(parts, fmt.Sprintf("• Возраст: %s лет", age))
	}

	// Рост и вес
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

	// Хронические заболевания
	if chronic := userData["chronic_diseases"]; chronic != "" && strings.ToLower(chronic) != "нет" {
		parts = append(parts, fmt.Sprintf("• Хронические заболевания: %s", chronic))
	}

	// Аллергии
	if allergies := userData["allergies"]; allergies != "" && strings.ToLower(allergies) != "нет" {
		parts = append(parts, fmt.Sprintf("• Аллергии: %s", allergies))
	}

	// Лекарства
	if medications := userData["medications"]; medications != "" && strings.ToLower(medications) != "нет" {
		parts = append(parts, fmt.Sprintf("• Принимаемые лекарства: %s", medications))
	}

	// Образ жизни
	if smoking := userData["smoking"]; smoking != "" && strings.ToLower(smoking) != "нет" {
		parts = append(parts, fmt.Sprintf("• Курение: %s", smoking))
	}
	if alcohol := userData["alcohol"]; alcohol != "" {
		parts = append(parts, fmt.Sprintf("• Алкоголь: %s", alcohol))
	}

	// Спортивные данные
	if sport := userData["sport_type"]; sport != "" {
		parts = append(parts, fmt.Sprintf("• Вид спорта: %s", sport))
	}
	if exp := userData["training_experience"]; exp != "" {
		parts = append(parts, fmt.Sprintf("• Стаж тренировок: %s лет", exp))
	}
	if goal := userData["goal"]; goal != "" {
		parts = append(parts, fmt.Sprintf("• Цель: %s", goal))
	}

	// Информация о курсе
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

// deleteMessage - удаляет сообщение
func deleteMessage(ctx context.Context, b *tgbot.Bot, chatID int64, messageID int) {
	if messageID == 0 {
		return
	}
	_, err := b.DeleteMessage(ctx, &tgbot.DeleteMessageParams{
		ChatID:    chatID,
		MessageID: messageID,
	})
	if err != nil {
		return
	}
}

// downloadUploadedFile - скачивает файл из Telegram
func downloadUploadedFile(
	ctx context.Context,
	b *tgbot.Bot,
	update *models.Update,
	uploadDir string,
) ([]byte, string, error) {
	var fileID string
	var fileName string
	var mimeType string

	if doc := update.Message.Document; doc != nil {
		fileID = doc.FileID
		fileName = doc.FileName
		mimeType = doc.MimeType
	} else if photos := update.Message.Photo; len(photos) > 0 {
		photo := photos[len(photos)-1]
		fileID = photo.FileID
		fileName = "photo.jpg"
		mimeType = "image/jpeg"
	}

	if fileID == "" {
		return nil, "", io.EOF
	}

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

	if mimeType == "" {
		mimeType = detectMimeType(fileName)
	}

	if uploadDir != "" {
		_ = os.MkdirAll(uploadDir, 0o755)
		_ = os.WriteFile(filepath.Join(uploadDir, fileName), data, 0o644)
	}

	return data, mimeType, nil
}

// detectMimeType - определяет MIME тип файла
func detectMimeType(fileName string) string {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// sendError - отправляет сообщение об ошибке
func sendError(
	ctx context.Context,
	b *tgbot.Bot,
	chatID int64,
) {
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "⚠️ Не удалось обработать анализ. Попробуйте позже.",
	})
}
