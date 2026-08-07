package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/service"
)

func UploadHandler(
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
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

		if state == states.StateWaitingPhotoConfirm {
			log.Printf("⏭️ Уже ждём ответ на вопрос, игнорируем")
			return
		}

		if state == states.StateWaitingUploadConfirm {
			log.Printf("⏭️ Ждём подтверждение загрузки")
			handleUploadConfirm(ctx, b, stateManager, analysisService, reportRenderer, uploadDir, stickerID, chatID, update.Message)
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

		if update.Message.Document != nil {
			log.Printf("📄 Обработка документа")
			handleFileUpload(ctx, b, stateManager, uploadDir, chatID, update.Message.Document)
			return
		}

		if update.Message.Photo != nil {
			log.Printf("📸 Обработка фото")
			handlePhotoUpload(ctx, b, stateManager, uploadDir, chatID, update.Message.Photo)
			return
		}

		if update.Message.Text != "" {
			log.Printf("📝 Обработка текста: %q", update.Message.Text)
			handleTextUpload(ctx, b, stateManager, analysisService, reportRenderer, uploadDir, stickerID, chatID, update.Message.Text)
			return
		}

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Пожалуйста, отправьте:\n\n• 📄 PDF-файл\n• 📸 Фотографию\n• 📝 Текст с показателями",
		})
	}
}

func handleFileUpload(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	uploadDir string,
	chatID int64,
	document *models.Document,
) {
	fileName := document.FileName
	ext := strings.ToLower(filepath.Ext(fileName))

	if ext != ".pdf" && ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" && ext != ".bmp" {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Пожалуйста, отправьте файл в формате PDF, JPG или PNG.",
		})
		return
	}

	fileData, mimeType, err := downloadFileByID(ctx, b, document.FileID, uploadDir)
	if err != nil {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Не удалось загрузить файл. Попробуйте ещё раз.",
		})
		return
	}

	files := stateManager.GetUserData(chatID, "uploaded_files")
	var uploadedFiles []UploadedFile
	if files != "" {
		if err := json.Unmarshal([]byte(files), &uploadedFiles); err != nil {
			uploadedFiles = []UploadedFile{}
		}
	}

	uploadedFiles = append(uploadedFiles, UploadedFile{
		Data:     fileData,
		MimeType: mimeType,
		FileName: fileName,
	})

	filesJSON, _ := json.Marshal(uploadedFiles)
	stateManager.SetUserData(chatID, "uploaded_files", string(filesJSON))
	stateManager.SetUserData(chatID, "file_count", fmt.Sprintf("%d", len(uploadedFiles)))

	safeFileName := strings.NewReplacer("<", "&lt;", ">", "&gt;", "&", "&amp;").Replace(fileName)

	messageText := fmt.Sprintf("✅ Файл <b>%s</b> добавлен!\n\n"+
		"📁 Всего загружено: <b>%d</b> файл(ов)\n\n"+
		"⚠️ <i>Пожалуйста, присылайте в чат по одному сообщению/файлу и нажмите <b>«✅ Обработать анализы»</b>, как будете готовы.</i>\n\n"+
		"Хотите добавить ещё один файл?\n"+
		"• Отправьте ещё один файл\n"+
		"• Или нажмите кнопку ниже:",
		safeFileName, len(uploadedFiles))

	_, err = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        messageText,
		ReplyMarkup: keyboards.UploadConfirm(),
		ParseMode:   "HTML",
	})
	if err != nil {
		log.Printf("⚠️ Ошибка отправки сообщения подтверждения файла: %v", err)
	}

	stateManager.SetState(chatID, states.StateWaitingUploadConfirm)
}

func handlePhotoUpload(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	uploadDir string,
	chatID int64,
	photos []models.PhotoSize,
) {
	photo := photos[len(photos)-1]

	fileData, mimeType, err := downloadFileByID(ctx, b, photo.FileID, uploadDir)
	if err != nil {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Не удалось загрузить фото. Попробуйте ещё раз.",
		})
		return
	}

	files := stateManager.GetUserData(chatID, "uploaded_files")
	var uploadedFiles []UploadedFile
	if files != "" {
		if err := json.Unmarshal([]byte(files), &uploadedFiles); err != nil {
			uploadedFiles = []UploadedFile{}
		}
	}

	uploadedFiles = append(uploadedFiles, UploadedFile{
		Data:     fileData,
		MimeType: mimeType,
		FileName: fmt.Sprintf("photo_%d.jpg", len(uploadedFiles)+1),
	})

	filesJSON, _ := json.Marshal(uploadedFiles)
	stateManager.SetUserData(chatID, "uploaded_files", string(filesJSON))
	stateManager.SetUserData(chatID, "file_count", fmt.Sprintf("%d", len(uploadedFiles)))

	messageText := fmt.Sprintf("✅ Фото <b>%d</b> добавлено!\n\n"+
		"📁 Всего загружено: <b>%d</b> файл(ов)\n\n"+
		"⚠️ <i>Пожалуйста, присылайте в чат по одному сообщению/файлу и нажмите <b>«✅ Обработать анализы»</b>, как будете готовы.</i>\n\n"+
		"Хотите добавить ещё один файл?\n"+
		"• Отправьте ещё один файл\n"+
		"• Или нажмите кнопку ниже:",
		len(uploadedFiles), len(uploadedFiles))

	_, err = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        messageText,
		ReplyMarkup: keyboards.UploadConfirm(),
		ParseMode:   "HTML",
	})
	if err != nil {
		log.Printf("⚠️ Ошибка отправки сообщения подтверждения фото: %v", err)
	}

	stateManager.SetState(chatID, states.StateWaitingUploadConfirm)
}

func handleUploadConfirm(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	uploadDir string,
	stickerID string,
	chatID int64,
	message *models.Message,
) {
	text := strings.TrimSpace(strings.ToLower(message.Text))

	if message.Document != nil {
		log.Printf("📄 Добавление ещё одного документа")
		handleFileUpload(ctx, b, stateManager, uploadDir, chatID, message.Document)
		return
	}

	if message.Photo != nil {
		log.Printf("📸 Добавление ещё одного фото")
		handlePhotoUpload(ctx, b, stateManager, uploadDir, chatID, message.Photo)
		return
	}

	if text == "✅ обработать анализы" || text == "обработать анализы" {
		log.Printf("🚀 Нажата кнопка '✅ Обработать анализы'. Запуск обработки всех сохраненных файлов.")
		startAnalysis(ctx, b, stateManager, analysisService, reportRenderer, uploadDir, stickerID, chatID)
		return
	}

	if text == "❌ отмена" || text == "отмена" {
		log.Printf("❌ Отмена загрузки")
		stateManager.SetUserData(chatID, "uploaded_files", "")
		stateManager.SetUserData(chatID, "file_count", "")
		stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "❌ Загрузка отменена.\n\nВы можете начать заново в главном меню:",
			ReplyMarkup: keyboards.MainMenu(),
		})
		return
	}

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text: "⚠️ Пожалуйста, присылайте по одному файлу в сообщении.\n\n" +
			"Когда добавите все файлы, нажмите кнопку <b>«✅ Обработать анализы»</b>.",
		ReplyMarkup: keyboards.UploadConfirm(),
		ParseMode:   "HTML",
	})
}

func startAnalysis(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	uploadDir string,
	stickerID string,
	chatID int64,
) {
	filesJSON := stateManager.GetUserData(chatID, "uploaded_files")
	if filesJSON == "" {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Нет загруженных файлов для анализа.",
		})
		stateManager.SetState(chatID, states.StateWaitingAnalysisFile)
		return
	}

	var uploadedFiles []UploadedFile
	if err := json.Unmarshal([]byte(filesJSON), &uploadedFiles); err != nil || len(uploadedFiles) == 0 {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка загрузки файлов. Попробуйте ещё раз.",
		})
		stateManager.SetState(chatID, states.StateWaitingAnalysisFile)
		return
	}

	stateManager.SetUserData(chatID, "uploaded_files", "")
	stateManager.SetUserData(chatID, "file_count", "")
	stateManager.SetState(chatID, states.StateWaitingAnalysisFile)

	analysisType := stateManager.GetUserData(chatID, "analysis_type")
	analysisSubtype := stateManager.GetUserData(chatID, "analysis_subtype")
	isExtended := analysisSubtype == "extended" || analysisType == "extended"

	userData := stateManager.GetAllUserData(chatID)
	contextInfo := buildAnalysisText(userData)

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text: fmt.Sprintf("📊 Начинаю анализ <b>%d</b> загруженного файла(ов)...\n\n"+
			"⏳ Это может занять несколько секунд.",
			len(uploadedFiles)),
		ParseMode: "HTML",
	})

	loadingMsg, textMsg := sendLoadingMessages(ctx, b, chatID, stickerID)

	if len(uploadedFiles) == 1 {
		file := uploadedFiles[0]
		processSingleFile(ctx, b, stateManager, analysisService, reportRenderer, chatID, loadingMsg, textMsg, file, isExtended, contextInfo)
	} else {
		processMultipleFiles(ctx, b, stateManager, analysisService, reportRenderer, chatID, loadingMsg, textMsg, uploadedFiles, isExtended, contextInfo)
	}
}

func processSingleFile(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	chatID int64,
	loadingMsg *models.Message,
	textMsg *models.Message,
	file UploadedFile,
	isExtended bool,
	contextInfo string,
) {
	if isExtended {
		jsonResult, err := analysisService.HandleAnalysisFromFileJSON(
			ctx,
			file.Data,
			file.MimeType,
			contextInfo,
		)

		if err == nil && jsonResult != "" {
			renderAndSendReport(ctx, b, stateManager, reportRenderer, chatID, loadingMsg, textMsg, jsonResult)
			return
		}

		if loadingMsg != nil {
			deleteMessage(ctx, b, chatID, loadingMsg.ID)
		}
		if textMsg != nil {
			deleteMessage(ctx, b, chatID, textMsg.ID)
		}
		sendError(ctx, b, chatID)
		return
	}

	result, err := analysisService.HandleAnalysisFromFileWithContext(
		ctx,
		file.Data,
		file.MimeType,
		contextInfo,
	)

	if loadingMsg != nil {
		deleteMessage(ctx, b, chatID, loadingMsg.ID)
	}
	if textMsg != nil {
		deleteMessage(ctx, b, chatID, textMsg.ID)
	}

	if err != nil {
		sendError(ctx, b, chatID)
		return
	}

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   result,
	})

	stateManager.Reset(chatID)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "✅ <b>Анализ завершён!</b>\n\nВыберите действие в главном меню:",
		ReplyMarkup: keyboards.MainMenu(),
		ParseMode:   "HTML",
	})
}

func processMultipleFiles(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	chatID int64,
	loadingMsg *models.Message,
	textMsg *models.Message,
	files []UploadedFile,
	isExtended bool,
	contextInfo string,
) {
	var collectedTexts []string

	for i, file := range files {
		res, err := analysisService.HandleAnalysisFromFileWithContext(
			ctx,
			file.Data,
			file.MimeType,
			contextInfo,
		)
		if err == nil && res != "" {
			collectedTexts = append(collectedTexts, fmt.Sprintf("=== Данные из файла %d (%s) ===\n%s", i+1, file.FileName, res))
		}
	}

	if len(collectedTexts) == 0 {
		if loadingMsg != nil {
			deleteMessage(ctx, b, chatID, loadingMsg.ID)
		}
		if textMsg != nil {
			deleteMessage(ctx, b, chatID, textMsg.ID)
		}
		sendError(ctx, b, chatID)
		return
	}

	combinedPayload := strings.Join(collectedTexts, "\n\n")

	if isExtended {
		jsonResult, err := analysisService.HandleAnalysisJSON(ctx, combinedPayload)
		if err == nil && jsonResult != "" {
			renderAndSendReport(ctx, b, stateManager, reportRenderer, chatID, loadingMsg, textMsg, jsonResult)
			return
		}

		if loadingMsg != nil {
			deleteMessage(ctx, b, chatID, loadingMsg.ID)
		}
		if textMsg != nil {
			deleteMessage(ctx, b, chatID, textMsg.ID)
		}
		sendError(ctx, b, chatID)
		return
	}

	if loadingMsg != nil {
		deleteMessage(ctx, b, chatID, loadingMsg.ID)
	}
	if textMsg != nil {
		deleteMessage(ctx, b, chatID, textMsg.ID)
	}

	finalResult := fmt.Sprintf("📊 <b>Результаты анализа %d файлов</b>\n\n%s", len(files), combinedPayload)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    chatID,
		Text:      finalResult,
		ParseMode: "HTML",
	})

	stateManager.Reset(chatID)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "✅ <b>Анализ завершён!</b>\n\nВыберите действие в главном меню:",
		ReplyMarkup: keyboards.MainMenu(),
		ParseMode:   "HTML",
	})
}

func renderAndSendReport(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	reportRenderer *report.Renderer,
	chatID int64,
	loadingMsg *models.Message,
	textMsg *models.Message,
	jsonResult string,
) {
	// --- ОЧИСТКА ТЕКСТА JSON ОТ MARKDOWN И СИМВОЛОВ ---
	cleanedJSON := strings.TrimSpace(jsonResult)
	cleanedJSON = strings.TrimPrefix(cleanedJSON, "```json")
	cleanedJSON = strings.TrimPrefix(cleanedJSON, "```")
	cleanedJSON = strings.TrimSuffix(cleanedJSON, "```")
	cleanedJSON = strings.TrimSpace(cleanedJSON)

	var reportData report.Report
	if err := json.Unmarshal([]byte(cleanedJSON), &reportData); err != nil {
		log.Printf("⚠️ Ошибка парсинга JSON: %v", err)
		log.Printf("📄 Полученный текст: %s", cleanedJSON)

		if loadingMsg != nil {
			deleteMessage(ctx, b, chatID, loadingMsg.ID)
		}
		if textMsg != nil {
			deleteMessage(ctx, b, chatID, textMsg.ID)
		}

		// Сбрасываем состояние и возвращаем главное меню
		stateManager.Reset(chatID)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "❌ Ответ от ИИ получился слишком объемным и не смог обработаться.\n\nПопробуйте отправить файлы по отдельности или выберите действие в меню:",
			ReplyMarkup: keyboards.MainMenu(),
		})
		return
	}

	htmlResult, err := reportRenderer.Render(reportData)
	if err != nil {
		log.Printf("⚠️ Ошибка генерации HTML: %v", err)
		if loadingMsg != nil {
			deleteMessage(ctx, b, chatID, loadingMsg.ID)
		}
		if textMsg != nil {
			deleteMessage(ctx, b, chatID, textMsg.ID)
		}

		// Сбрасываем состояние и возвращаем главное меню
		stateManager.Reset(chatID)
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        "❌ Произошла ошибка при формировании отчета.\n\nВыберите действие в меню:",
			ReplyMarkup: keyboards.MainMenu(),
		})
		return
	}

	pdfData, pdfErr := report.ConvertHTMLToPDF(htmlResult)

	if loadingMsg != nil {
		deleteMessage(ctx, b, chatID, loadingMsg.ID)
	}
	if textMsg != nil {
		deleteMessage(ctx, b, chatID, textMsg.ID)
	}

	if pdfErr == nil && len(pdfData) > 0 {
		_, _ = b.SendDocument(ctx, &tgbot.SendDocumentParams{
			ChatID: chatID,
			Document: &models.InputFileUpload{
				Filename: "Analysis_report.pdf",
				Data:     bytes.NewReader(pdfData),
			},
			Caption: "📄 <b>Ваш расширенный анализ</b>\n\n" +
				"🔬 Детальная расшифровка всех загруженных файлов\n" +
				"📊 Оценка всех показателей\n" +
				"💡 Персональные рекомендации",
			ParseMode: "HTML",
		})
	} else {
		_, _ = b.SendDocument(ctx, &tgbot.SendDocumentParams{
			ChatID: chatID,
			Document: &models.InputFileUpload{
				Filename: "Analysis_report.html",
				Data:     bytes.NewReader([]byte(htmlResult)),
			},
			Caption: "📄 <b>Ваш расширенный анализ</b>\n\n" +
				"🔬 Детальная расшифровка всех загруженных файлов\n" +
				"📊 Оценка всех показателей\n" +
				"💡 Персональные рекомендации\n\n" +
				"📎 Откройте файл в браузере для просмотра",
			ParseMode: "HTML",
		})
	}

	stateManager.Reset(chatID)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "✅ <b>Анализ завершён!</b>\n\nВыберите действие в главном меню:",
		ReplyMarkup: keyboards.MainMenu(),
		ParseMode:   "HTML",
	})
}

func handleTextUpload(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	uploadDir string,
	stickerID string,
	chatID int64,
	text string,
) {
	payload := strings.TrimSpace(text)
	if payload == "" {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Пожалуйста, отправьте текст с показателями анализов.",
		})
		return
	}

	userData := stateManager.GetAllUserData(chatID)
	analysisType := stateManager.GetUserData(chatID, "analysis_type")
	analysisSubtype := stateManager.GetUserData(chatID, "analysis_subtype")
	contextInfo := buildAnalysisText(userData)

	isExtended := analysisSubtype == "extended" || analysisType == "extended"

	loadingMsg, textMsg := sendLoadingMessages(ctx, b, chatID, stickerID)

	if isExtended {
		log.Printf("🔬 РАСШИРЕННЫЙ АНАЛИЗ ТЕКСТА - генерируем HTML из JSON...")

		jsonResult, err := analysisService.HandleAnalysisJSON(ctx, payload)
		if err == nil && jsonResult != "" {
			renderAndSendReport(ctx, b, stateManager, reportRenderer, chatID, loadingMsg, textMsg, jsonResult)
			return
		}
	}

	result, err := analysisService.HandleAnalysisWithContext(ctx, payload, contextInfo)

	if loadingMsg != nil {
		deleteMessage(ctx, b, chatID, loadingMsg.ID)
	}
	if textMsg != nil {
		deleteMessage(ctx, b, chatID, textMsg.ID)
	}

	if err != nil {
		sendError(ctx, b, chatID)
		return
	}

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   result,
	})

	stateManager.Reset(chatID)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        "✅ <b>Анализ завершён!</b>\n\nВыберите действие в главном меню:",
		ReplyMarkup: keyboards.MainMenu(),
		ParseMode:   "HTML",
	})
}

type UploadedFile struct {
	Data     []byte `json:"data"`
	MimeType string `json:"mime_type"`
	FileName string `json:"file_name"`
}
