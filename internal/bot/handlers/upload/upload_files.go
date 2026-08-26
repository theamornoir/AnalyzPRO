package upload

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
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

// uploadConfirmKeyboard - inline-кнопки подтверждения загрузки. Reply-
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
	msgID int,
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

	fileData, mimeType, err := helpers.DownloadFileByID(ctx, b, document.FileID)
	if err != nil {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgUploadFileError,
		})
		return
	}

	// Дополнительная защита: отклоняем типы, не являющиеся
	// PDF/изображением (исполняемые файлы, архивы, HTML), даже если
	// пользователь переименовал расширение.
	if isDangerousMime(mimeType) {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgUploadFileInvalid,
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

	// Трекаем ID исходного сообщения пользователя, чтобы удалить его из
	// чата после успешной обработки анализа (приватность: исходные
	// материалы не остаются в истории).
	appendUploadedMsgID(stateManager, chatID, msgID)

	// Экранируем имя файла перед вставкой в HTML-сообщение (ParseMode: HTML),
	// чтобы имена вроде «<b>x</b>» или «<a href=...>» не интерпретировались
	// Telegram как разметка (бесполезный identity-replacer раньше этого не
	// делал - форматирование ломалось на спецсимволах).
	safeFileName := html.EscapeString(fileName)

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
	msgID int,
	photos []models.PhotoSize,
) {
	photo := photos[len(photos)-1]

	fileData, mimeType, err := helpers.DownloadFileByID(ctx, b, photo.FileID)
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

	// Трекаем ID исходного сообщения пользователя, чтобы удалить его из
	// чата после успешной обработки анализа (приватность: исходные
	// материалы не остаются в истории).
	appendUploadedMsgID(stateManager, chatID, msgID)

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

// dangerousMimes - типы, которые категорически не принимаем (исполняемые
// файлы, арх материалы, скрипты, HTML). Защита от загрузки вредоноса под
// видом медицинского анализа (например, rename evil.exe → evil.pdf).
var dangerousMimes = map[string]bool{
	"application/x-msdownload":                         true,
	"application/x-dosexec":                           true,
	"application/x-executable":                        true,
	"application/vnd.microsoft.portable-executable":   true,
	"application/x-msdos-program":                     true,
	"application/x-sh":                               true,
	"application/x-bat":                              true,
	"application/java-archive":                        true,
	"application/zip":                                true,
	"application/x-rar-compressed":                   true,
	"application/x-7z-compressed":                    true,
	"application/x-tar":                              true,
	"application/gzip":                              true,
	"text/html":                                      true,
	"text/javascript":                               true,
	"application/javascript":                         true,
	"application/x-python":                           true,
}

// isDangerousMime возвращает true для типов, которые не являются
// PDF/изображением и потенциально опасны при дальнейшей обработке.
func isDangerousMime(mime string) bool {
	return dangerousMimes[mime]
}
