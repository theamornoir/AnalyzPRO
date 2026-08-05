// package handlers

// import (
// 	"bytes"
// 	"context"
// 	"encoding/json"
// 	"log"
// 	"time"

// 	tgbot "github.com/go-telegram/bot"
// 	"github.com/go-telegram/bot/models"

// 	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
// 	"github.com/theamornoir/analyzpro/internal/bot/states"
// 	"github.com/theamornoir/analyzpro/internal/report"
// 	"github.com/theamornoir/analyzpro/internal/service"
// )

// func BioscanHandler(
// 	stateManager states.StateManager,
// 	analysisService service.AnalysisService,
// 	reportRenderer *report.Renderer,
// 	uploadDir string,
// 	stickerID string,
// ) func(context.Context, *tgbot.Bot, *models.Update) {

// 	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {

// 		if update.Message == nil {
// 			return
// 		}

// 		chatID := update.Message.Chat.ID

// 		if update.Message.Photo == nil {

// 			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
// 				ChatID: chatID,
// 				Text: "📸 **Bioscan — анализ фигуры**\n\n" +
// 					"Отправьте фото тела:\n\n" +
// 					"• полный рост\n" +
// 					"• хорошее освещение\n" +
// 					"• стоять прямо\n" +
// 					"• чёткое изображение",
// 				ReplyMarkup: keyboards.BackMenu(),
// 				ParseMode:   "Markdown",
// 			})

// 			return
// 		}

// 		agreement := stateManager.GetUserData(
// 			chatID,
// 			"agreement_accepted",
// 		)

// 		if agreement != "yes" {

// 			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
// 				ChatID:      chatID,
// 				Text:        "📝 Сначала примите пользовательское соглашение.",
// 				ReplyMarkup: keyboards.StartMenu(),
// 			})

// 			return
// 		}

// 		loadingMsg, statusMsg := sendLoadingMessages(
// 			ctx,
// 			b,
// 			chatID,
// 			stickerID,
// 		)

// 		go animateBioscanStatus(
// 			ctx,
// 			b,
// 			chatID,
// 			statusMsg.ID,
// 		)

// 		photo := update.Message.Photo[len(update.Message.Photo)-1]

// 		fileData, mimeType, err := downloadFileByID(
// 			ctx,
// 			b,
// 			photo.FileID,
// 			uploadDir,
// 		)

// 		if err != nil {

// 			deleteMessage(ctx, b, chatID, loadingMsg.ID)
// 			deleteMessage(ctx, b, chatID, statusMsg.ID)

// 			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
// 				ChatID: chatID,
// 				Text:   "❌ Не удалось загрузить фото.",
// 			})

// 			return
// 		}

// 		// Gemini анализ

// 		result, err := analysisService.HandleBioscan(
// 			ctx,
// 			fileData,
// 			mimeType,
// 			"",
// 		)

// 		if err != nil {

// 			deleteMessage(ctx, b, chatID, loadingMsg.ID)
// 			deleteMessage(ctx, b, chatID, statusMsg.ID)

// 			log.Printf(
// 				"Bioscan error: %v",
// 				err,
// 			)

// 			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
// 				ChatID: chatID,
// 				Text:   "⚠️ Не удалось обработать фото.",
// 			})

// 			return
// 		}

// 		deleteMessage(ctx, b, chatID, loadingMsg.ID)
// 		deleteMessage(ctx, b, chatID, statusMsg.ID)

// 		log.Printf(
// 			"Gemini response length: %d",
// 			len(result),
// 		)

// 		// JSON -> Report

// 		var bioscanReport report.Report

// 		err = json.Unmarshal(
// 			[]byte(result),
// 			&bioscanReport,
// 		)

// 		if err != nil {

// 			log.Printf(
// 				"JSON parse error: %v",
// 				err,
// 			)

// 			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
// 				ChatID: chatID,
// 				Text:   "⚠️ Бот вернул неправильный формат отчёта.",
// 			})

// 			return
// 		}

// 		creatingMsg, _ := b.SendMessage(
// 			ctx,
// 			&tgbot.SendMessageParams{
// 				ChatID: chatID,
// 				Text:   "📄 Создаю HTML отчёт...",
// 			},
// 		)

// 		htmlReport, err := reportRenderer.Render(
// 			bioscanReport,
// 		)

// 		if err != nil {

// 			log.Printf(
// 				"HTML render error: %v",
// 				err,
// 			)

// 			deleteMessage(
// 				ctx,
// 				b,
// 				chatID,
// 				creatingMsg.ID,
// 			)

// 			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
// 				ChatID: chatID,
// 				Text:   "❌ Ошибка создания отчёта.",
// 			})

// 			return
// 		}

// 		deleteMessage(
// 			ctx,
// 			b,
// 			chatID,
// 			creatingMsg.ID,
// 		)

// 		// Отправляем HTML

// 		_, err = b.SendDocument(
// 			ctx,
// 			&tgbot.SendDocumentParams{

// 				ChatID: chatID,

// 				Document: &models.InputFileUpload{

// 					Filename: "Bioscan_report.html",

// 					Data: bytes.NewReader(
// 						[]byte(htmlReport),
// 					),
// 				},

// 				Caption: "📄 Ваш Bioscan отчёт\n\n" +
// 					"💪 Анализ тела\n" +
// 					"📊 Оценка зон\n" +
// 					"🏋️ Рекомендации",
// 			},
// 		)

// 		if err != nil {

// 			log.Printf(
// 				"Send document error: %v",
// 				err,
// 			)
// 		}

// 		_, _ = b.SendMessage(
// 			ctx,
// 			&tgbot.SendMessageParams{
// 				ChatID: chatID,
// 				Text: "📸 Хотите сделать новый Bioscan?\n\n" +
// 					"Отправьте новое фото.",
// 				ReplyMarkup: keyboards.BackMenu(),
// 			},
// 		)

// 	}

// }

// func animateBioscanStatus(
// 	ctx context.Context,
// 	b *tgbot.Bot,
// 	chatID int64,
// 	messageID int,
// ) {

// 	statuses := []string{

// 		"🔍 Анализирую пропорции тела...",

// 		"💪 Проверяю мышечный баланс...",

// 		"🦴 Анализирую осанку...",

// 		"📊 Оцениваю композицию тела...",

// 		"🧬 Формирую профиль развития...",

// 		"📝 Создаю рекомендации...",
// 	}

// 	for _, status := range statuses {

// 		select {

// 		case <-ctx.Done():
// 			return

// 		default:

// 		}

// 		time.Sleep(
// 			2 * time.Second,
// 		)

// 		_, _ = b.EditMessageText(
// 			ctx,
// 			&tgbot.EditMessageTextParams{
// 				ChatID:    chatID,
// 				MessageID: messageID,
// 				Text: status +
// 					"\n\n⏳ Подождите...",
// 			},
// 		)

// 	}

// }
package handlers

import (
	"bytes"
	"context"
	"log"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/service"
)

func BioscanHandler(
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	uploadDir string,
	stickerID string,
) func(context.Context, *tgbot.Bot, *models.Update) {

	return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {

		if update.Message == nil {
			return
		}

		chatID := update.Message.Chat.ID

		if update.Message.Photo == nil {

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text: "📸 **Bioscan — анализ фигуры**\n\n" +
					"Отправьте фото тела:\n\n" +
					"• полный рост\n" +
					"• хорошее освещение\n" +
					"• стоять прямо\n" +
					"• чёткое изображение",
				ReplyMarkup: keyboards.BackMenu(),
				ParseMode:   "Markdown",
			})

			return
		}

		agreement := stateManager.GetUserData(
			chatID,
			"agreement_accepted",
		)

		if agreement != "yes" {

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID:      chatID,
				Text:        "📝 Сначала примите пользовательское соглашение.",
				ReplyMarkup: keyboards.StartMenu(),
			})

			return
		}

		loadingMsg, statusMsg := sendLoadingMessages(
			ctx,
			b,
			chatID,
			stickerID,
		)

		go animateBioscanStatus(
			ctx,
			b,
			chatID,
			statusMsg.ID,
		)

		photo := update.Message.Photo[len(update.Message.Photo)-1]

		fileData, mimeType, err := downloadFileByID(
			ctx,
			b,
			photo.FileID,
			uploadDir,
		)

		if err != nil {

			deleteMessage(ctx, b, chatID, loadingMsg.ID)
			deleteMessage(ctx, b, chatID, statusMsg.ID)

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "❌ Не удалось загрузить фото.",
			})

			return
		}

		// Анализ + генерация HTML внутри сервиса

		htmlReport, err := analysisService.HandleBioscan(
			ctx,
			fileData,
			mimeType,
			"",
		)

		if err != nil {

			deleteMessage(ctx, b, chatID, loadingMsg.ID)
			deleteMessage(ctx, b, chatID, statusMsg.ID)

			log.Printf(
				"Bioscan error: %v",
				err,
			)

			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   "⚠️ Не удалось обработать фото.",
			})

			return
		}

		deleteMessage(ctx, b, chatID, loadingMsg.ID)
		deleteMessage(ctx, b, chatID, statusMsg.ID)

		// Отправляем готовый HTML

		_, err = b.SendDocument(
			ctx,
			&tgbot.SendDocumentParams{

				ChatID: chatID,

				Document: &models.InputFileUpload{

					Filename: "Bioscan_report.html",

					Data: bytes.NewReader(
						[]byte(htmlReport),
					),
				},

				Caption: "📄 Ваш Bioscan отчёт\n\n" +
					"💪 Анализ тела\n" +
					"📊 Оценка зон\n" +
					"🏋️ Рекомендации",
			},
		)

		if err != nil {

			log.Printf(
				"Send document error: %v",
				err,
			)
		}

		_, _ = b.SendMessage(
			ctx,
			&tgbot.SendMessageParams{
				ChatID: chatID,
				Text: "📸 Хотите сделать новый Bioscan?\n\n" +
					"Отправьте новое фото.",
				ReplyMarkup: keyboards.BackMenu(),
			},
		)

	}

}

func animateBioscanStatus(
	ctx context.Context,
	b *tgbot.Bot,
	chatID int64,
	messageID int,
) {

	statuses := []string{

		"🔍 Анализирую пропорции тела...",

		"💪 Проверяю мышечный баланс...",

		"🦴 Анализирую осанку...",

		"📊 Оцениваю композицию тела...",

		"🧬 Формирую профиль развития...",

		"📝 Создаю рекомендации...",
	}

	for _, status := range statuses {

		select {

		case <-ctx.Done():
			return

		default:

		}

		time.Sleep(
			2 * time.Second,
		)

		_, _ = b.EditMessageText(
			ctx,
			&tgbot.EditMessageTextParams{
				ChatID:    chatID,
				MessageID: messageID,
				Text: status +
					"\n\n⏳ Подождите...",
			},
		)

	}

}
