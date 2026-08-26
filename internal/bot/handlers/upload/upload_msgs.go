package upload

import (
	"context"
	"encoding/json"

	tgbot "github.com/go-telegram/bot"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/states"
)

// uploadedMsgIDsKey - ключ состояния со списком ID исходных сообщений
// пользователя (файлы/фото/текст анализа), присланных для обработки. После
// успешной обработки ИИ эти сообщения удаляются из чата (приватность:
// исходные материалы не остаются в истории и не хранятся ботом).
const uploadedMsgIDsKey = "uploaded_msg_ids"

// appendUploadedMsgID добавляет ID исходного сообщения пользователя в список
// для последующего удаления из чата после обработки анализа.
func appendUploadedMsgID(sm states.StateManager, chatID int64, msgID int) {
	if msgID == 0 {
		return
	}
	ids := getUploadedMsgIDs(sm, chatID)
	ids = append(ids, msgID)
	data, _ := json.Marshal(ids)
	sm.SetUserData(chatID, uploadedMsgIDsKey, string(data))
}

// getUploadedMsgIDs возвращает список ID исходных сообщений пользователя.
func getUploadedMsgIDs(sm states.StateManager, chatID int64) []int {
	raw := sm.GetUserData(chatID, uploadedMsgIDsKey)
	if raw == "" {
		return nil
	}
	var ids []int
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil
	}
	return ids
}

// clearUploadedMsgIDs очищает список ID исходных сообщений (без удаления из чата).
func clearUploadedMsgIDs(sm states.StateManager, chatID int64) {
	sm.SetUserData(chatID, uploadedMsgIDsKey, "")
}

// deleteSubmittedMessages удаляет из чата исходные сообщения пользователя
// (присланные файлы/фото/текст), которые были отправлены для анализа.
// Вызывается ПОСЛЕ успешной обработки ИИ, чтобы исходные материалы не
// оставались в истории чата и не хранились ботом. Удаление наилучшего
// усилия: ошибки (сообщение уже удалено и т.п.) игнорируются. Список ID
// очищается в любом случае.
//
// В приватном чате (1-на-1) бот может удалять сообщения пользователя без
// ограничений по возрасту сообщения.
func deleteSubmittedMessages(ctx context.Context, b *tgbot.Bot, sm states.StateManager, chatID int64) {
	for _, id := range getUploadedMsgIDs(sm, chatID) {
		helpers.DeleteMessage(ctx, b, chatID, id)
	}
	clearUploadedMsgIDs(sm, chatID)
}
