package handlers

import (
	"bytes"
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
			sendLoading(ctx, b, chatID)

			fileData, mimeType, err := downloadUploadedFile(ctx, b, update, uploadDir)
			if err != nil {
				sendError(ctx, b, chatID)
				stateManager.Reset(chatID)
				return
			}

			result, err := analysisService.HandleAnalysisFromFile(ctx, fileData, mimeType)
			if err != nil {
				sendError(ctx, b, chatID)
				stateManager.Reset(chatID)
				return
			}

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   result,
			})

			stateManager.Reset(chatID)
			return
		}

		payload := strings.TrimSpace(update.Message.Text)

		if payload == "" {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "Пожалуйста, отправьте PDF или фотографию анализов.",
			})
			return
		}

		result, err := analysisService.HandleAnalysis(ctx, payload)
		if err != nil {
			sendError(ctx, b, chatID)
			stateManager.Reset(chatID)
			return
		}

		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   result,
		})

		stateManager.Reset(chatID)
	}
}

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

func sendLoading(
	ctx context.Context,
	b *tgbot.Bot,
	chatID int64,
) {

	path := "assets/loading.mp4"

	data, err := os.ReadFile(path)

	if err == nil && len(data) > 0 {

		_, _ = b.SendAnimation(ctx,
			&tgbot.SendAnimationParams{
				ChatID: chatID,
				Animation: &models.InputFileUpload{
					Filename: "loading.mp4",
					Data:     bytes.NewReader(data),
				},
				Caption: "⏳ Обработка анализа…\n\n" +
					"1/3 Сохраняем файл\n" +
					"2/3 Проверяем документ\n" +
					"3/3 Анализируем показатели",
			},
		)

		return
	}

	_, _ = b.SendMessage(ctx,
		&tgbot.SendMessageParams{
			ChatID: chatID,
			Text: "⏳ Обработка анализа…\n\n" +
				"1/3 Сохраняем файл\n" +
				"2/3 Проверяем документ\n" +
				"3/3 Анализируем показатели",
		},
	)
}

func sendError(
	ctx context.Context,
	b *tgbot.Bot,
	chatID int64,
) {

	_, _ = b.SendMessage(ctx,
		&tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   "⚠️ Не удалось обработать анализ. Попробуйте позже.",
		},
	)
}
