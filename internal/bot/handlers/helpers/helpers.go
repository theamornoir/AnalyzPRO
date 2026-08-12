package helpers

import (
	"context"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/locales"
)

// SendError - отправляет сообщение об ошибке.
func SendError(ctx context.Context, b *tgbot.Bot, chatID int64) {
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   locales.MsgTextProcessingError,
	})
}

// DeleteMessage - удаляет сообщение.
func DeleteMessage(ctx context.Context, b *tgbot.Bot, chatID int64, messageID int) {
	if messageID == 0 {
		return
	}
	_, _ = b.DeleteMessage(ctx, &tgbot.DeleteMessageParams{
		ChatID:    chatID,
		MessageID: messageID,
	})
}

// SafeDeleteLoadingMsgs - безопасно удаляет сообщения о загрузке.
func SafeDeleteLoadingMsgs(ctx context.Context, b *tgbot.Bot, chatID int64, stickerMsg, textMsg *models.Message) {
	if stickerMsg != nil {
		DeleteMessage(ctx, b, chatID, stickerMsg.ID)
	}
	if textMsg != nil {
		DeleteMessage(ctx, b, chatID, textMsg.ID)
	}
}

// SendLoadingMessages - отправляет стикер и текстовое сообщение о загрузке.
func SendLoadingMessages(
	ctx context.Context,
	b *tgbot.Bot,
	chatID int64,
	stickerID string,
) (*models.Message, *models.Message) {

	var stickerMsg *models.Message

	if stickerID != "" {
		stickerMsg, _ = b.SendSticker(ctx, &tgbot.SendStickerParams{
			ChatID: chatID,
			Sticker: &models.InputFileString{
				Data: stickerID,
			},
		})
	}

	var textMsg *models.Message
	if stickerMsg != nil {
		textMsg, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgLoadingProcessing,
		})
	} else {
		stickerMsg, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: chatID,
			Text:   locales.MsgLoadingProcessing2,
		})
	}

	return stickerMsg, textMsg
}
