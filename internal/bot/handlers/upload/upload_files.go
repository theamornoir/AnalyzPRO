package upload

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// uploadConfirmKeyboard — inline-кнопки подтверждения загрузки. Reply-
// клавиатура при этом остаётся единой [Назад] (наследуется от предыдущего
// шага анализа), поэтому действия «Обработать/Отмена» вынесены в inline.
func uploadConfirmKeyboard() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: locales.BtnProcessAnalysis, CallbackData: "upload_process"}},
			{{Text: locales.BtnCancel, CallbackData: "upload_cancel"}},
		},
	}
}

// handleFileUpload - обрабатывает загрузку документа (PDF/изображение).
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
			Text:   locales.MsgUploadFileInvalid,
		})
		return
	}

	fileData, mimeType, err := helpers.DownloadFileByID(ctx, b, document.FileID, uploadDir)
	if err != nil {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgUploadFileError,
		})
		return
	}

	// Сохраняем файл на диск, в память кладём только путь
	path := saveUploadedFile(uploadDir, fileName, fileData)

	uploadedFiles := appendUploadedFile(stateManager, chatID, UploadedFile{
		Path:     path,
		MimeType: mimeType,
		FileName: fileName,
	})

	safeFileName := strings.NewReplacer("&", "&", "<", "<", ">", ">").Replace(fileName)

	messageText := fmt.Sprintf(locales.MsgUploadFileAdded,
		safeFileName, len(uploadedFiles))

	msg, err := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        messageText,
		ReplyMarkup: uploadConfirmKeyboard(),
		ParseMode:   "HTML",
	})
	if err != nil {
		log.Printf(locales.LogUploadConfirmDocErr, err)
	} else if msg != nil {
		stateManager.SetUserData(chatID, "last_msg_id", strconv.Itoa(msg.ID))
	}

	stateManager.SetState(chatID, states.StateWaitingUploadConfirm)
}

// handlePhotoUpload - обрабатывает загрузку фотографии.
func handlePhotoUpload(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	uploadDir string,
	chatID int64,
	photos []models.PhotoSize,
) {
	photo := photos[len(photos)-1]

	fileData, mimeType, err := helpers.DownloadFileByID(ctx, b, photo.FileID, uploadDir)
	if err != nil {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgUploadPhotoError,
		})
		return
	}

	// Определяем расширение по реальному MIME-типу
	ext := ".jpg"
	switch mimeType {
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	case "image/jpeg", "":
		ext = ".jpg"
	}

	existing := getUploadedFiles(stateManager, chatID)
	fileName := fmt.Sprintf("photo_%d%s", len(existing)+1, ext)

	// Сохраняем файл на диск, в память кладём только путь
	path := saveUploadedFile(uploadDir, fileName, fileData)

	uploadedFiles := appendUploadedFile(stateManager, chatID, UploadedFile{
		Path:     path,
		MimeType: mimeType,
		FileName: fileName,
	})

	messageText := fmt.Sprintf(locales.MsgUploadPhotoAdded,
		len(uploadedFiles), len(uploadedFiles))

	msg, err := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        messageText,
		ReplyMarkup: uploadConfirmKeyboard(),
		ParseMode:   "HTML",
	})
	if err != nil {
		log.Printf(locales.LogUploadConfirmPhotoErr, err)
	} else if msg != nil {
		stateManager.SetUserData(chatID, "last_msg_id", strconv.Itoa(msg.ID))
	}

	stateManager.SetState(chatID, states.StateWaitingUploadConfirm)
}

// getUploadedFiles - читает список загруженных файлов из состояния.
func getUploadedFiles(stateManager states.StateManager, chatID int64) []UploadedFile {
	files := stateManager.GetUserData(chatID, "uploaded_files")
	var uploadedFiles []UploadedFile
	if files != "" {
		if err := json.Unmarshal([]byte(files), &uploadedFiles); err != nil {
			uploadedFiles = []UploadedFile{}
		}
	}
	return uploadedFiles
}

// appendUploadedFile - добавляет файл в список и сохраняет обратно в состояние.
func appendUploadedFile(stateManager states.StateManager, chatID int64, file UploadedFile) []UploadedFile {
	uploadedFiles := getUploadedFiles(stateManager, chatID)
	uploadedFiles = append(uploadedFiles, file)

	filesJSON, _ := json.Marshal(uploadedFiles)
	stateManager.SetUserData(chatID, "uploaded_files", string(filesJSON))
	stateManager.SetUserData(chatID, "file_count", fmt.Sprintf("%d", len(uploadedFiles)))
	return uploadedFiles
}
