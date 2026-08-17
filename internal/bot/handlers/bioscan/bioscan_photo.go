package bioscan

import (
	"context"
	"fmt"
	"strconv"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
)

// HandleBioscanPhoto - обработка фотографий.
func HandleBioscanPhoto(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64, photos []models.PhotoSize) {
	if len(photos) == 0 {
		return
	}

	state := sm.GetState(chatID)
	photo := photos[len(photos)-1]

	switch state {
	case states.StateWaitingBioscanPhoto1:
		sm.SetUserData(chatID, "bioscan_photo1", photo.FileID)
		sm.SetState(chatID, states.StateWaitingBioscanPhoto2)
		sm.SetUserData(chatID, "bioscan_photo_count", "1")

		sendPhotoPrompt(ctx, b, sm, chatID, states.StateWaitingBioscanPhoto2)

	case states.StateWaitingBioscanPhoto2:
		sm.SetUserData(chatID, "bioscan_photo2", photo.FileID)
		sm.SetState(chatID, states.StateWaitingBioscanPhoto3)
		sm.SetUserData(chatID, "bioscan_photo_count", "2")

		sendPhotoPrompt(ctx, b, sm, chatID, states.StateWaitingBioscanPhoto3)

	case states.StateWaitingBioscanPhoto3:
		sm.SetUserData(chatID, "bioscan_photo3", photo.FileID)
		sm.SetState(chatID, states.StateWaitingBioscanPhoto4)
		sm.SetUserData(chatID, "bioscan_photo_count", "3")

		sendPhotoPrompt(ctx, b, sm, chatID, states.StateWaitingBioscanPhoto4)

	case states.StateWaitingBioscanPhoto4:
		sm.SetUserData(chatID, "bioscan_photo4", photo.FileID)
		sm.SetState(chatID, states.StateWaitingBioscanConfirm)
		sm.SetUserData(chatID, "bioscan_photo_count", "4")

		name := sm.GetUserData(chatID, "bioscan_name")
		age := sm.GetUserData(chatID, "bioscan_age")
		height := sm.GetUserData(chatID, "bioscan_height")
		weight := sm.GetUserData(chatID, "bioscan_weight")
		goal := sm.GetUserData(chatID, "bioscan_goal")

		// Подтверждение - inline-кнопки, чтобы единая Reply-клавиатура
		// [Назад] (установлена на предыдущем шаге) оставалась внизу.
		msg, err := b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:    chatID,
			Text:      fmt.Sprintf(locales.MsgBioscanAllPhotosReceived, name, age, height, weight, goal),
			ParseMode: "Markdown",
			ReplyMarkup: models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{
					{{Text: locales.BtnBioscanConfirm, CallbackData: "bioscan_confirm"}},
					{{Text: locales.BtnBioscanRestart, CallbackData: "bioscan_restart"}},
				},
			},
		})
		if err == nil && msg != nil {
			sm.SetUserData(chatID, "last_msg_id", strconv.Itoa(msg.ID))
		}

	default:
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:      chatID,
			Text:        locales.MsgBioscanError,
			ReplyMarkup: keyboards.BackMenu(),
		})
	}
}

// sendPhotoPrompt отправляет промпт для следующего фото.
func sendPhotoPrompt(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64, nextState states.State) {
	var prompt string

	switch nextState {
	case states.StateWaitingBioscanPhoto1:
		prompt = locales.MsgBioscanPhotoPrompt1
	case states.StateWaitingBioscanPhoto2:
		prompt = locales.MsgBioscanPhotoPrompt2
	case states.StateWaitingBioscanPhoto3:
		prompt = locales.MsgBioscanPhotoPrompt3
	case states.StateWaitingBioscanPhoto4:
		prompt = locales.MsgBioscanPhotoPrompt4
	default:
		prompt = locales.MsgBioscanDefaultPhotoPrompt
	}

	msg, err := b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        prompt,
		ParseMode:   "Markdown",
		ReplyMarkup: keyboards.BackMenu(),
	})
	if err == nil && msg != nil {
		sm.SetUserData(chatID, "last_msg_id", strconv.Itoa(msg.ID))
	}
}
