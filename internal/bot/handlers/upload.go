package handlers

import (
	"bytes"
	"context"
	"os"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/service"
)

func UploadHandler(
	stateManager states.StateManager,
	analysisService service.AnalysisService,
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

		// Обработка файлов
		if update.Message.Document != nil ||
			update.Message.Photo != nil {

			sendLoading(ctx, b, chatID)

			payload := buildPayload(update)

			result, err := analysisService.HandleAnalysis(
				ctx,
				payload,
			)

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

		// Если пришел текст
		payload := strings.TrimSpace(
			update.Message.Text,
		)

		if payload == "" {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "Пожалуйста, отправьте PDF или фотографию анализов.",
			})
			return
		}

		result, err := analysisService.HandleAnalysis(
			ctx,
			payload,
		)

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

func buildPayload(
	update *models.Update,
) string {

	if update.Message.Document != nil {

		payload :=
			"Пользователь загрузил PDF-анализ. " +
				"Документ принят в систему."

		if update.Message.Document.FileName != "" {
			payload +=
				" Имя файла: " +
					update.Message.Document.FileName
		}

		return payload
	}

	if update.Message.Photo != nil {

		return "Пользователь загрузил фотографию анализов. Изображение принято в систему."
	}

	return ""
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
