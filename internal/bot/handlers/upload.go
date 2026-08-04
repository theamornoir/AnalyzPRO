package handlers

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

		if update.Message.Document != nil || update.Message.Photo != nil {
			// Отправляем стикер и текстовое сообщение
			loadingMsg, textMsg := sendLoadingMessages(ctx, b, chatID)

			fileData, mimeType, err := downloadUploadedFile(ctx, b, update, uploadDir)
			if err != nil {
				deleteMessage(ctx, b, chatID, loadingMsg.ID)
				deleteMessage(ctx, b, chatID, textMsg.ID)
				sendError(ctx, b, chatID)
				stateManager.Reset(chatID)
				return
			}

			result, err := analysisService.HandleAnalysisFromFile(ctx, fileData, mimeType)
			if err != nil {
				deleteMessage(ctx, b, chatID, loadingMsg.ID)
				deleteMessage(ctx, b, chatID, textMsg.ID)
				sendError(ctx, b, chatID)
				stateManager.Reset(chatID)
				return
			}

			// Удаляем стикер и текстовое сообщение
			deleteMessage(ctx, b, chatID, loadingMsg.ID)
			deleteMessage(ctx, b, chatID, textMsg.ID)

			// Отправляем результат
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   result,
			})

			stateManager.Reset(chatID)
			return
		}

		// Текстовый анализ
		payload := strings.TrimSpace(update.Message.Text)

		if payload == "" {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "Пожалуйста, отправьте PDF или фотографию анализов.",
			})
			return
		}

		loadingMsg, textMsg := sendLoadingMessages(ctx, b, chatID)

		result, err := analysisService.HandleAnalysis(ctx, payload)
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
}

// sendLoadingMessages - отправляет стикер и текстовое сообщение
func sendLoadingMessages(ctx context.Context, b *tgbot.Bot, chatID int64) (*models.Message, *models.Message) {
	// ВАШ FILE_ID СТИКЕРА
	stickerFileID := "CAACAgIAAxkBAAN3anIN2ra-k9IPOjpSwnAcKKu5ZQcAAmVzAAJK3mBLnf-jLZbQHmM9BA"

	// Отправляем стикер
	stickerMsg, err := b.SendSticker(ctx, &tgbot.SendStickerParams{
		ChatID: chatID,
		Sticker: &models.InputFileString{
			Data: stickerFileID,
		},
	})

	// Если стикер не отправился - отправляем текстовое сообщение вместо него
	if err != nil {
		stickerMsg, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "⏳ Обрабатываю...",
		})
	}

	// Отправляем текстовое сообщение под стикером
	textMsg, _ := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   "📊 Обрабатываю результаты...",
	})

	return stickerMsg, textMsg
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
