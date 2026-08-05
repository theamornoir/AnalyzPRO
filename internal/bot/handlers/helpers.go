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
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/service"
)

// sendError - отправляет сообщение об ошибке
func sendError(ctx context.Context, b *tgbot.Bot, chatID int64) {
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "⚠️ Не удалось обработать анализ. Попробуйте позже.",
	})
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

// buildAnalysisText - формирует текст с учетом данных пользователя
func buildAnalysisText(userData map[string]string) string {
	var parts []string

	// Имя пользователя
	name := userData["name"]
	if name == "" {
		name = "Пользователь"
	}
	parts = append(parts, fmt.Sprintf("👤 **Пациент:** %s", name))
	parts = append(parts, "")

	parts = append(parts, "❗ **ВАЖНАЯ ИНФОРМАЦИЯ ДЛЯ АНАЛИЗА:**")

	// Пол
	if gender := userData["gender"]; gender != "" {
		parts = append(parts, fmt.Sprintf("• Пол: %s", gender))
	}

	// Возраст
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
			h, _ := strconv.ParseFloat(height, 64)
			w, _ := strconv.ParseFloat(weight, 64)
			if h > 0 && w > 0 {
				bmi := w / ((h / 100) * (h / 100))
				parts = append(parts, fmt.Sprintf("• ИМТ: %.1f", bmi))
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

	// Информация о препаратах
	onCourse := userData["on_course"]
	if onCourse == "yes" {
		courseInfo := userData["course_info"]
		if courseInfo != "" {
			parts = append(parts, fmt.Sprintf("• ИСПОЛЬЗУЕТ ПРЕПАРАТЫ: %s", courseInfo))
		} else {
			parts = append(parts, "• ИСПОЛЬЗУЕТ ПРЕПАРАТЫ (информация не указана)")
		}
		parts = append(parts, "• Требуется интерпретация с учетом приема препаратов")
		parts = append(parts, "• Оценить влияние на гормональный фон и показатели")
	} else if onCourse == "no" {
		parts = append(parts, "• Без препаратов (естественный фон)")
	}

	return strings.Join(parts, "\n")
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
			ChatID:      chatID,
			Text:        "📸 Я проверил фото, но оно не похоже на медицинские анализы.\n\nПожалуйста, отправьте:\n• 📄 PDF-файл с анализами\n• 📸 Фотографию анализов (с таблицей и цифрами)\n• 📝 Текст с показателями\n\nЯ не могу обработать это фото как медицинский анализ.",
			ReplyMarkup: keyboards.BackMenu(),
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

	// Возвращаемся в состояние ожидания файла с кнопкой "Назад"
	stateManager.SetState(chatID, states.StateWaitingAnalysisFile)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "📄 Отправьте ещё один файл или нажмите ⬅️ Назад для возврата в меню.",
		ReplyMarkup: keyboards.BackMenu(),
	})
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

	// Возвращаемся в состояние ожидания файла с кнопкой "Назад"
	stateManager.SetState(chatID, states.StateWaitingAnalysisFile)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "📄 Отправьте ещё один файл или нажмите ⬅️ Назад для возврата в меню.",
		ReplyMarkup: keyboards.BackMenu(),
	})
}

// updateLoadingStatus - обновляет сообщение со статусом
func updateLoadingStatus(ctx context.Context, b *tgbot.Bot, chatID int64, messageID int, newStatus string) {
	_, _ = b.EditMessageText(ctx, &tgbot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text: newStatus + "\n\n" +
			"🔄 Пожалуйста, подождите...\n" +
			"⏳ Это может занять несколько секунд",
	})
}
