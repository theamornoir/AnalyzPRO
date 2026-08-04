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
